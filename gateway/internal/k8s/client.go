package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/streed/smol-cluster/gateway/internal/config"
	"github.com/streed/smol-cluster/gateway/internal/models"
)

type Client struct {
	clientset *kubernetes.Clientset
	namespace string
	config    *config.Config
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

func (c *Client) CreateAgentPod(ctx context.Context, ws models.Workstream, repo models.Repository) (podName, serviceName string, err error) {
	podName = fmt.Sprintf("smol-agent-%s", ws.ID.String()[:8])
	serviceName = fmt.Sprintf("smol-svc-%s", ws.ID.String()[:8])

	pod := BuildPodSpec(podName, ws, repo, c.config)

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

func (c *Client) GetPodLogs(ctx context.Context, podName string) (string, error) {
	tailLines := int64(500)
	req := c.clientset.CoreV1().Pods(c.namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: "agent",
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

func (c *Client) SendMessageToAgent(ctx context.Context, serviceName, content string) error {
	url := fmt.Sprintf("http://%s.%s.svc.cluster.local:8022/message", serviceName, c.namespace)
	body := fmt.Sprintf(`{"content":%q}`, content)
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) CompleteWorkstream(ctx context.Context, ws models.Workstream) (string, error) {
	url := fmt.Sprintf("http://%s.%s.svc.cluster.local:8022/complete", ws.ServiceName, c.namespace)
	resp, err := http.Post(url, "application/json", bytes.NewBufferString("{}"))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		PRURL string `json:"pull_request_url"`
	}
	if err := io.ReadAll(resp.Body); err == nil {
		// parse the response for PR URL
	}
	_ = result
	return result.PRURL, nil
}
