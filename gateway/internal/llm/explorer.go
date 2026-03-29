package llm

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/streed/smol-gang/gateway/internal/config"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const explorationPrompt = `Analyze this repository and provide a detailed summary. Include:

1. **Purpose**: What does this project do? What problem does it solve?
2. **Architecture**: High-level architecture, key modules/packages, and how they relate
3. **Tech Stack**: Languages, frameworks, databases, tools
4. **Key Files**: The most important files and their roles
5. **Build & Run**: How to build, test, and run the project
6. **Code Patterns**: Notable patterns, conventions, or architectural decisions
7. **Areas of Complexity**: Parts of the codebase that are complex or have many dependencies

Be specific — reference actual file paths and module names. This summary will be used to plan development tasks.`

// basicExploreScript generates file tree + key files without needing smol-agent
const basicExploreScript = `#!/bin/bash
set -e
git clone --depth=1 "%s" /workspace/repo 2>&1
cd /workspace/repo

echo "## File Tree"
echo '` + "```" + `'
find . -not -path './.git/*' -not -path '*/node_modules/*' -not -path '*/vendor/*' -not -path '*/.venv/*' -not -path '*/__pycache__/*' | head -300 | sort
echo '` + "```" + `'
echo ""

for f in README.md package.json go.mod Cargo.toml pyproject.toml requirements.txt Makefile docker-compose.yml Dockerfile .smol-gang.yaml; do
  if [ -f "$f" ]; then
    echo "## $f"
    echo '` + "```" + `'
    head -100 "$f"
    echo '` + "```" + `'
    echo ""
  fi
done
`

// ExploreRepoBasic runs a lightweight K8s pod that clones the repo and outputs
// file tree + key files. Fast (~30s) and doesn't need smol-agent.
func ExploreRepoBasic(ctx context.Context, k8s kubernetes.Interface, gitURL, token, branch, namespace string) (string, error) {
	if k8s == nil {
		return "", fmt.Errorf("k8s client not available")
	}

	podName := fmt.Sprintf("smol-scan-%d", time.Now().Unix())

	authURL := gitURL
	if token != "" {
		authURL = strings.Replace(gitURL, "https://", "https://x-access-token:"+token+"@", 1)
	}

	script := fmt.Sprintf(basicExploreScript, authURL)

	scanDeadline := int64(300) // 5 minutes
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    map[string]string{"app": "smol-scan", "type": "scan"},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:         corev1.RestartPolicyNever,
			ActiveDeadlineSeconds: &scanDeadline,
			Containers: []corev1.Container{
				{
					Name:            "scanner",
					Image:           "alpine/git:latest",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"/bin/sh", "-c"},
					Args:            []string{script},
				},
			},
		},
	}

	_, err := k8s.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("create scan pod: %w", err)
	}

	// Wait with hard timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			k8s.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
			return "", fmt.Errorf("scan timed out")
		case <-time.After(3 * time.Second):
			p, err := k8s.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				k8s.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
				return "", fmt.Errorf("scan pod disappeared")
			}
			if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
				goto scanDone
			}
		}
	}
scanDone:

	// Read logs
	result := readPodLogs(ctx, k8s, podName, "scanner", namespace)

	// Always clean up
	k8s.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})

	if result == "" {
		return "", fmt.Errorf("scan pod produced no output")
	}

	return result, nil
}

// ExploreWithAgent runs smol-agent in a K8s pod to do a deep analysis of the repo.
// Returns the agent's analysis as a string. Takes longer (~2-5min) but gives better context.
func ExploreWithAgent(ctx context.Context, k8s kubernetes.Interface, cfg *config.Config, gitURL, token, branch, namespace string) (string, error) {
	if k8s == nil {
		return "", fmt.Errorf("k8s client not available")
	}

	podName := fmt.Sprintf("smol-explore-%d", time.Now().Unix())

	authURL := gitURL
	if token != "" {
		authURL = strings.Replace(gitURL, "https://", "https://x-access-token:"+token+"@", 1)
	}

	script := fmt.Sprintf(`#!/bin/bash
set -e
git clone --depth=50 "%s" /workspace/repo 2>&1
cd /workspace/repo
git config user.email "explorer@smol-gang.local"
git config user.name "smol-gang-explorer"

smol-agent --yolo --provider %s --model %s %s %s -d /workspace/repo "%s" 2>/dev/null || echo "Agent exploration completed with errors"
`,
		authURL,
		cfg.LLMProvider,
		cfg.LLMModel,
		func() string {
			if cfg.LLMApiKey != "" {
				return "--api-key " + cfg.LLMApiKey
			}
			return ""
		}(),
		func() string {
			if cfg.LLMApiURL != "" {
				return "--host " + cfg.LLMApiURL
			}
			return ""
		}(),
		explorationPrompt,
	)

	activeDeadline := int64(1200) // 20 minutes
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    map[string]string{"app": "smol-explore", "type": "exploration"},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:         corev1.RestartPolicyNever,
			ActiveDeadlineSeconds: &activeDeadline,
			Containers: []corev1.Container{
				{
					Name:            "explorer",
					Image:           cfg.AgentImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"/bin/bash", "-c"},
					Args:            []string{script},
					Env: []corev1.EnvVar{
						{Name: "OLLAMA_API_KEY", Value: cfg.LLMApiKey},
					},
				},
			},
		},
	}

	_, err := k8s.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("create exploration pod: %w", err)
	}

	// Wait for completion with hard 20-minute timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			log.Printf("explorer: timeout waiting for pod %s, cleaning up", podName)
			k8s.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
			return "", fmt.Errorf("exploration timed out after 20 minutes")
		case <-time.After(5 * time.Second):
			p, err := k8s.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				k8s.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
				return "", fmt.Errorf("pod disappeared: %w", err)
			}
			if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
				goto done
			}
		}
	}
done:

	result := readPodLogs(ctx, k8s, podName, "explorer", namespace)

	// Always clean up
	k8s.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})

	if result == "" {
		return "", fmt.Errorf("exploration pod produced no output")
	}

	return result, nil
}

func readPodLogs(ctx context.Context, k8s kubernetes.Interface, podName, container, namespace string) string {
	tailLines := int64(500)
	logReq := k8s.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: container,
		TailLines: &tailLines,
	})
	stream, err := logReq.Stream(ctx)
	if err != nil {
		log.Printf("explorer: failed to read logs from %s: %v", podName, err)
		return ""
	}
	defer stream.Close()

	var buf strings.Builder
	b := make([]byte, 4096)
	for {
		n, err := stream.Read(b)
		if n > 0 {
			buf.Write(b[:n])
		}
		if err != nil {
			break
		}
	}
	return buf.String()
}

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
