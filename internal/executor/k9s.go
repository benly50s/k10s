package executor

// LaunchK9s launches k9s with KUBECONFIG set to the given file path.
// If context is non-empty, --context is passed to k9s to target a specific context.
// It uses syscall.Exec to replace the current process so k9s gets full terminal control.
func LaunchK9s(kubeconfigPath, context string) error {
	env := map[string]string{
		"KUBECONFIG": kubeconfigPath,
	}
	args := []string{}
	if context != "" {
		args = append(args, "--context", context)
	}
	return ExecReplace("k9s", args, env)
}
