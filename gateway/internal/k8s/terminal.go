package k8s

import (
	"context"
	"fmt"
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/gorilla/websocket"
)

// TerminalSession bridges a WebSocket connection to a pod exec session,
// giving users an interactive terminal into the app container.
type TerminalSession struct {
	conn     *websocket.Conn
	sizeChan chan remotecommand.TerminalSize
}

func (t *TerminalSession) Read(p []byte) (int, error) {
	_, message, err := t.conn.ReadMessage()
	if err != nil {
		return 0, err
	}
	return copy(p, message), nil
}

func (t *TerminalSession) Write(p []byte) (int, error) {
	err := t.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (t *TerminalSession) Next() *remotecommand.TerminalSize {
	size, ok := <-t.sizeChan
	if !ok {
		return nil
	}
	return &size
}

// HandleTerminal upgrades an HTTP request to WebSocket and connects to a
// pod container's shell. Defaults to the "app" container so users can
// interact with the running application without affecting the agent.
func (c *Client) HandleTerminal(w http.ResponseWriter, r *http.Request, podName, container string) error {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return fmt.Errorf("websocket upgrade failed: %w", err)
	}
	defer conn.Close()

	session := &TerminalSession{
		conn:     conn,
		sizeChan: make(chan remotecommand.TerminalSize),
	}

	if container == "" {
		container = "app"
	}

	// Use /bin/sh as universal fallback (Alpine containers like dind don't have bash)
	shell := []string{"/bin/sh", "-l"}
	if container == "app" {
		shell = []string{"/bin/bash", "-l"}
	}

	return c.execInPod(r.Context(), podName, container, shell, session)
}

func (c *Client) execInPod(ctx context.Context, podName, container string, command []string, session *TerminalSession) error {
	restConfig, err := getRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to get k8s config: %w", err)
	}

	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(c.namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             session,
		Stdout:            session,
		Stderr:            session,
		Tty:               true,
		TerminalSizeQueue: session,
	})
}

func getRESTConfig() (*rest.Config, error) {
	if os.Getenv("K8S_IN_CLUSTER") == "true" {
		return rest.InClusterConfig()
	}
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}
