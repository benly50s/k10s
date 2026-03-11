package k8s

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// PortForwardHandle holds the running port-forward process
type PortForwardHandle struct {
	Cmd       *exec.Cmd
	LocalPort int
}

// Stop kills the port-forward process
func (h *PortForwardHandle) Stop() {
	if h.Cmd != nil && h.Cmd.Process != nil {
		_ = h.Cmd.Process.Kill()
		_ = h.Cmd.Wait()
	}
}

// StartPortForward starts kubectl port-forward in the background.
// It waits up to 10 seconds for "Forwarding from" in stderr before returning.
// context is optional — if non-empty it is passed as --context to kubectl.
func StartPortForward(kubeconfigPath, context, namespace, service string, localPort, remotePort int) (*PortForwardHandle, error) {
	target := fmt.Sprintf("svc/%s", service)
	portArg := fmt.Sprintf("%d:%d", localPort, remotePort)

	args := []string{"port-forward", "-n", namespace, target, portArg}
	if context != "" {
		args = append([]string{"--context", context}, args...)
	}

	cmd := exec.Command("kubectl", args...)
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	// Ensure port-forward child process is killed when parent dies (Unix only)
	setSysProcAttr(cmd)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("getting stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting port-forward: %w", err)
	}

	handle := &PortForwardHandle{Cmd: cmd, LocalPort: localPort}

	// Wait for "Forwarding from" or error with a 10s timeout
	ready := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(stderrPipe)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					ready <- fmt.Errorf("port-forward process exited unexpectedly")
				} else {
					ready <- fmt.Errorf("reading port-forward output: %w", err)
				}
				return
			}
			if strings.Contains(line, "Forwarding from") {
				ready <- nil
				// Drain remaining stderr to prevent pipe blockage
				go func() {
					for {
						_, err := reader.ReadString('\n')
						if err != nil {
							return
						}
					}
				}()
				return
			}
			// Detect common errors early
			if strings.Contains(line, "error") || strings.Contains(line, "Error") {
				ready <- fmt.Errorf("port-forward error: %s", strings.TrimSpace(line))
				return
			}
		}
	}()

	select {
	case err := <-ready:
		if err != nil {
			handle.Stop()
			return nil, err
		}
		return handle, nil
	case <-time.After(10 * time.Second):
		handle.Stop()
		return nil, fmt.Errorf("port-forward timed out waiting for ready signal")
	}
}

// lsofAvailable returns true if lsof is present in PATH
func lsofAvailable() bool {
	_, err := exec.LookPath("lsof")
	return err == nil
}

// IsPortInUse checks if a local port is already in use
func IsPortInUse(port int) bool {
	if !lsofAvailable() {
		fmt.Fprintln(os.Stderr, "warning: lsof not found; cannot check port availability")
		return false
	}
	return len(GetPIDsOnPort(port)) > 0
}

// GetPIDsOnPort returns PIDs of processes listening on the given port
func GetPIDsOnPort(port int) []int {
	if !lsofAvailable() {
		return nil
	}
	cmd := exec.Command("lsof", "-i", fmt.Sprintf("TCP:%d", port), "-t", "-sTCP:LISTEN")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if pid, err := strconv.Atoi(line); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

// KillProcessOnPort kills all processes listening on the given port.
// Returns an error if killing fails.
func KillProcessOnPort(port int) error {
	pids := GetPIDsOnPort(port)
	if len(pids) == 0 {
		return nil
	}

	var errs []string
	for _, pid := range pids {
		proc, err := os.FindProcess(pid)
		if err != nil {
			errs = append(errs, fmt.Sprintf("find pid %d: %v", pid, err))
			continue
		}
		if err := proc.Kill(); err != nil {
			errs = append(errs, fmt.Sprintf("kill pid %d: %v", pid, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to kill processes on port %d: %s", port, strings.Join(errs, "; "))
	}
	return nil
}

// FindAvailablePort tries the preferred port first, then searches sequentially
// up to maxAttempts ports higher. Returns the first available port.
func FindAvailablePort(preferred int) (int, error) {
	const maxAttempts = 20
	for i := 0; i < maxAttempts; i++ {
		port := preferred + i
		if !IsPortInUse(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found in range %d-%d", preferred, preferred+maxAttempts-1)
}
