package executor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// LaunchShell launches the user's default shell with KUBECONFIG set to the given file path.
// If context is non-empty, kubectl config use-context is run first so that
// kubectl commands inside the shell automatically use the correct context.
func LaunchShell(kubeconfigPath, context string) error {
	shell := resolveShell()

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

	return RunWithEnv(shell, []string{}, env)
}

// resolveShell picks a shell binary appropriate for the current OS. The SHELL
// environment variable always wins if set. On Windows, prefer pwsh then
// powershell then cmd.exe (via COMSPEC). On Unix, fall back to /bin/sh.
func resolveShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	if runtime.GOOS == "windows" {
		for _, cand := range []string{"pwsh.exe", "powershell.exe"} {
			if p, err := exec.LookPath(cand); err == nil {
				return p
			}
		}
		if s := os.Getenv("COMSPEC"); s != "" {
			return s
		}
		return "cmd.exe"
	}
	return "/bin/sh"
}
