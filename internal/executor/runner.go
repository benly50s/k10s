package executor

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// RunWithEnv runs a command with additional environment variables injected.
// It waits for the command to complete.
func RunWithEnv(command string, args []string, env map[string]string) error {
	path, err := exec.LookPath(command)
	if err != nil {
		return fmt.Errorf("command not found: %s", command)
	}

	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Build environment
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	return cmd.Run()
}

// ExecReplace replaces the current process with the given command using syscall.Exec.
// This is used for k9s so it gets full terminal control.
func ExecReplace(command string, args []string, env map[string]string) error {
	path, err := exec.LookPath(command)
	if err != nil {
		return fmt.Errorf("command not found: %s", command)
	}

	// Build argv (command name + args)
	argv := append([]string{command}, args...)

	// Build environment
	environ := os.Environ()
	for k, v := range env {
		// Override or append
		found := false
		for i, e := range environ {
			if len(e) > len(k) && e[:len(k)] == k && e[len(k)] == '=' {
				environ[i] = fmt.Sprintf("%s=%s", k, v)
				found = true
				break
			}
		}
		if !found {
			environ = append(environ, fmt.Sprintf("%s=%s", k, v))
		}
	}

	return syscall.Exec(path, argv, environ)
}
