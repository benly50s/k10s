package executor

import (
	"os"
)

// LaunchShell launches the user's default shell with KUBECONFIG set to the given file path.
// If context is non-empty, KUBE_CONTEXT will also be set.
// It uses syscall.Exec to replace the current process so the new shell gets full terminal control.
func LaunchShell(kubeconfigPath, context string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		// Fallback for systems without SHELL
		shell = "/bin/sh"
	}

	env := map[string]string{
		"KUBECONFIG": kubeconfigPath,
	}
	if context != "" {
		env["KUBE_CONTEXT"] = context
	}

	// Run the shell with no extra arguments
	return ExecReplace(shell, []string{}, env)
}
