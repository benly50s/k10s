package executor

import (
	"fmt"
	"os"
	"os/exec"
)

// LaunchShell launches the user's default shell with KUBECONFIG set to the given file path.
// If context is non-empty, kubectl config use-context is run first so that
// kubectl commands inside the shell automatically use the correct context.
// It uses syscall.Exec to replace the current process so the new shell gets full terminal control.
func LaunchShell(kubeconfigPath, context string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	env := map[string]string{
		"KUBECONFIG": kubeconfigPath,
	}

	// Set the active context in the kubeconfig file so that plain `kubectl` commands
	// inside the shell automatically target the right cluster.
	if context != "" {
		cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "config", "use-context", context)
		cmd.Env = os.Environ()
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("Warning: could not set context '%s': %v\n%s\n", context, err, string(out))
		}
	}

	return ExecReplace(shell, []string{}, env)
}
