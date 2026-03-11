package argocd

import (
	"fmt"
	"os/exec"
)

// Login runs argocd login against localhost:{port}
func Login(port int, username, password string, insecure bool) error {
	if password == "" {
		return fmt.Errorf("argocd password not configured; set it in ~/.k10s/config.yaml under profiles.<name>.argocd.password")
	}

	args := []string{
		"login",
		fmt.Sprintf("localhost:%d", port),
		"--username", username,
		"--password", password,
		"--grpc-web",
	}

	if insecure {
		args = append(args, "--insecure")
	}

	cmd := exec.Command("argocd", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("argocd login failed: %w\n%s", err, string(out))
	}

	return nil
}
