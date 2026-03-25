package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// FetchPodLogs returns the last N lines of logs from a pod container.
func FetchPodLogs(kubeconfigPath, kubeContext, namespace, pod, container string, lines int) (string, error) {
	if DemoMode {
		return "2026-03-25T14:30:00Z INFO [Demo] Application started\n2026-03-25T14:30:01Z DEBUG [Demo] Initializing components\n2026-03-25T14:30:05Z INFO [Demo] Listening on port 8080\n2026-03-25T14:35:12Z WARN [Demo] High memory usage detected\n2026-03-25T14:40:00Z ERROR [Demo] Failed to connect to database", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	args := buildKubectlArgs(kubeconfigPath, kubeContext)
	args = append(args, "logs", pod, "-n", namespace, fmt.Sprintf("--tail=%d", lines))
	if container != "" {
		args = append(args, "-c", container)
	}

	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("kubectl logs timed out after 15s")
		}
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("kubectl: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}

// StreamPodLogs starts kubectl logs -f and returns the command and a reader for its stdout.
// The caller must call cmd.Process.Kill() and cmd.Wait() when done.
func StreamPodLogs(kubeconfigPath, kubeContext, namespace, pod, container string) (*exec.Cmd, io.ReadCloser, error) {
	args := buildKubectlArgs(kubeconfigPath, kubeContext)
	args = append(args, "logs", pod, "-n", namespace, "-f", "--tail=0")
	if container != "" {
		args = append(args, "-c", container)
	}

	cmd := exec.Command("kubectl", args...)
	setSysProcAttr(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("getting stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("starting kubectl logs: %w", err)
	}

	return cmd, stdout, nil
}

// FetchPodContainers returns the container names for a pod.
func FetchPodContainers(kubeconfigPath, kubeContext, namespace, pod string) ([]string, error) {
	if DemoMode {
		return []string{"main-app", "sidecar-proxy"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := buildKubectlArgs(kubeconfigPath, kubeContext)
	args = append(args, "get", "pod", pod, "-n", namespace, "-o", "json")

	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("fetching pod containers: %w", err)
	}

	var podObj struct {
		Spec struct {
			Containers []struct {
				Name string `json:"name"`
			} `json:"containers"`
			InitContainers []struct {
				Name string `json:"name"`
			} `json:"initContainers"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(out, &podObj); err != nil {
		return nil, fmt.Errorf("parsing pod JSON: %w", err)
	}

	var names []string
	for _, c := range podObj.Spec.Containers {
		names = append(names, c.Name)
	}
	return names, nil
}
