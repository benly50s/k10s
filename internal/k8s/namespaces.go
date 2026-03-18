package k8s

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// FetchNamespaces returns the sorted list of namespace names visible from the
// given kubeconfig file and optional context.  It shells out to kubectl so that
// RBAC, OIDC tokens, and exec-plugins are all handled transparently.
func FetchNamespaces(kubeconfigPath, kubeContext string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{
		"--kubeconfig", kubeconfigPath,
		"get", "namespaces",
		"-o", "jsonpath={.items[*].metadata.name}",
	}
	if kubeContext != "" {
		args = append([]string{"--context", kubeContext}, args...)
	}

	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("kubectl timed out after 10s")
		}
		// Include stderr if available
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("kubectl: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}

	names := strings.Fields(raw)
	sort.Strings(names)
	return names, nil
}
