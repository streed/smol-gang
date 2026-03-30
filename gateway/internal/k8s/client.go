package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/streed/smol-gang/gateway/internal/config"
	"github.com/streed/smol-gang/gateway/internal/models"
)

type Client struct {
	clientset *kubernetes.Clientset
	namespace string
	config    *config.Config
}

// Clientset returns the underlying kubernetes clientset for direct API access.
func (c *Client) Clientset() kubernetes.Interface {
	return c.clientset
}

func NewClient(cfg *config.Config) (*Client, error) {
	var restConfig *rest.Config
	var err error

	if os.Getenv("K8S_IN_CLUSTER") == "true" {
		restConfig, err = rest.InClusterConfig()
	} else {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}
		restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s config: %w", err)
	}

	// Allow overriding the API server host (e.g. when running in Docker alongside Kind)
	if override := os.Getenv("K8S_API_SERVER"); override != "" {
		restConfig.Host = override
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s clientset: %w", err)
	}

	return &Client{
		clientset: clientset,
		namespace: cfg.K8sNamespace,
		config:    cfg,
	}, nil
}

func (c *Client) CreateAgentPod(ctx context.Context, ws models.Workstream, repo models.Repository, extraEnv map[string]string) (podName, serviceName string, err error) {
	podName = fmt.Sprintf("smol-agent-%s", ws.ID.String()[:8])
	serviceName = fmt.Sprintf("smol-svc-%s", ws.ID.String()[:8])

	pod := BuildPodSpec(podName, ws, repo, c.config, extraEnv)

	_, err = c.clientset.CoreV1().Pods(c.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to create pod: %w", err)
	}

	svc := BuildServiceSpec(serviceName, podName, ws)
	_, err = c.clientset.CoreV1().Services(c.namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		// Clean up pod if service creation fails
		_ = c.clientset.CoreV1().Pods(c.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
		return "", "", fmt.Errorf("failed to create service: %w", err)
	}

	return podName, serviceName, nil
}

func (c *Client) DeleteAgentPod(ctx context.Context, podName, serviceName string) error {
	if serviceName != "" {
		_ = c.clientset.CoreV1().Services(c.namespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
	}
	if podName != "" {
		return c.clientset.CoreV1().Pods(c.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	}
	return nil
}

func (c *Client) GetPodStatus(ctx context.Context, podName string) (string, error) {
	pod, err := c.clientset.CoreV1().Pods(c.namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(pod.Status.Phase), nil
}

func (c *Client) GetPodLogs(ctx context.Context, podName string, container string) (string, error) {
	if container == "" {
		container = "agent"
	}
	tailLines := int64(500)
	req := c.clientset.CoreV1().Pods(c.namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: container,
		TailLines: &tailLines,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, stream)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// GetServiceHost returns the in-cluster DNS address for a service port.
func (c *Client) GetServiceHost(serviceName string, port int) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", serviceName, c.namespace, port)
}

func (c *Client) GetServiceEndpoints(ctx context.Context, serviceName string) (map[string]string, error) {
	svc, err := c.clientset.CoreV1().Services(c.namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	endpoints := make(map[string]string)
	for _, port := range svc.Spec.Ports {
		endpoints[port.Name] = fmt.Sprintf("%s:%d", svc.Spec.ClusterIP, port.Port)
	}
	return endpoints, nil
}

// proxyPost sends an HTTP POST to a pod via the K8s API server proxy.
func (c *Client) proxyPost(ctx context.Context, podName, path string, reqBody []byte) ([]byte, error) {
	// Build the proxy URL: /api/v1/namespaces/{ns}/pods/{pod}:{port}/proxy/{path}
	proxyURL := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s:8022/proxy/%s", c.namespace, podName, path)

	result := c.clientset.CoreV1().RESTClient().
		Post().
		AbsPath(proxyURL).
		SetHeader("Content-Type", "application/json").
		Body(reqBody).
		Do(ctx)

	return result.Raw()
}

func (c *Client) ProxyGet(ctx context.Context, podName, path string) ([]byte, error) {
	proxyURL := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s:8022/proxy/%s", c.namespace, podName, path)
	result := c.clientset.CoreV1().RESTClient().
		Get().
		AbsPath(proxyURL).
		Do(ctx)
	return result.Raw()
}

func (c *Client) SendMessageToAgent(ctx context.Context, podName, content string) error {
	body := fmt.Sprintf(`{"content":%q}`, content)
	_, err := c.proxyPost(ctx, podName, "message", []byte(body))
	return err
}

func (c *Client) CompleteWorkstream(ctx context.Context, ws models.Workstream) (string, error) {
	respBody, err := c.proxyPost(ctx, ws.PodName, "complete", []byte("{}"))
	if err != nil {
		return "", err
	}

	var result struct {
		PRURL string `json:"pull_request_url"`
	}
	if len(respBody) > 0 {
		json.Unmarshal(respBody, &result)
	}
	return result.PRURL, nil
}
