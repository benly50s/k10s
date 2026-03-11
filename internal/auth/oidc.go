package auth

import (
	"fmt"
	"os"
	"os/exec"
)

// RefreshOIDC triggers an OIDC token refresh using kubelogin or kubectl.
// kubeconfigPath sets the KUBECONFIG env. context optionally sets --context flag.
func RefreshOIDC(kubeconfigPath, context string) error {
	env := append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))

	// Try kubelogin convert-kubeconfig (idempotent, rewrites exec credentials)
	if path, err := exec.LookPath("kubelogin"); err == nil {
		cmd := exec.Command(path, "convert-kubeconfig", "-l", "azurecli")
		cmd.Env = env
		if _, err := cmd.CombinedOutput(); err == nil {
			return nil
		}
		// Not fatal; fall through to kubectl
	}

	// Fall back to kubectl get nodes to trigger OIDC interactive flow
	args := []string{"get", "nodes", "--request-timeout=30s"}
	if context != "" {
		args = append(args, "--context", context)
	}
	cmd := exec.Command("kubectl", args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("OIDC refresh failed: %w\n%s", err, string(out))
	}

	return nil
}
