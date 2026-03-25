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

// PortForwardHandle holds the running port-forward process.
// For processes started by k10s, Cmd is set.
// For externally discovered processes, Process is set instead.
type PortForwardHandle struct {
	Cmd       *exec.Cmd
	Process   *os.Process // used for externally discovered processes
	LocalPort int
}

// process returns the underlying os.Process, preferring Cmd.Process.
func (h *PortForwardHandle) process() *os.Process {
	if h.Cmd != nil && h.Cmd.Process != nil {
		return h.Cmd.Process
	}
	return h.Process
}

// IsAlive checks whether the port-forward process is still running.
// Platform-specific implementation in portforward_unix.go / portforward_windows.go.

// Stop gracefully terminates the port-forward process.
// Platform-specific implementation in portforward_unix.go / portforward_windows.go.

// StartPortForward starts kubectl port-forward in the background.
// It waits up to 10 seconds for "Forwarding from" in stderr before returning.
// context is optional — if non-empty it is passed as --context to kubectl.
func StartPortForward(kubeconfigPath, kubeContext, namespace, resourceType, resourceName string, localPort, remotePort int) (*PortForwardHandle, error) {
	if DemoMode {
		return &PortForwardHandle{
			Cmd:       nil,
			Process:   nil,
			LocalPort: localPort,
		}, nil
	}

	target := fmt.Sprintf("%s/%s", resourceType, resourceName)
	portArg := fmt.Sprintf("%d:%d", localPort, remotePort)

	args := []string{"--kubeconfig", kubeconfigPath}
	if kubeContext != "" {
		args = append(args, "--context", kubeContext)
	}
	args = append(args, "port-forward", "-n", namespace, target, portArg)

	cmd := exec.Command("kubectl", args...)
	// Ensure port-forward child process is killed when parent dies (Unix only)
	setSysProcAttr(cmd)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("getting stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("getting stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting port-forward: %w", err)
	}

	handle := &PortForwardHandle{Cmd: cmd, LocalPort: localPort}

	// Wait for "Forwarding from" on stdout/stderr, or error with a 10s timeout.
	// kubectl may write the ready message to either stream depending on version.
	ready := make(chan error, 2)

	watchPipe := func(pipe io.Reader, isStderr bool) {
		reader := bufio.NewReader(pipe)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					if isStderr {
						// stderr EOF is normal once stdout signals ready
						return
					}
					ready <- fmt.Errorf("port-forward process exited unexpectedly")
				} else {
					ready <- fmt.Errorf("reading port-forward output: %w", err)
				}
				return
			}
			if strings.Contains(line, "Forwarding from") {
				ready <- nil
				// Drain remaining output to prevent pipe blockage
				go func() {
					for {
						if _, err := reader.ReadString('\n'); err != nil {
							return
						}
					}
				}()
				return
			}
			// Detect common errors early (typically on stderr)
			if isStderr && (strings.Contains(line, "error") || strings.Contains(line, "Error")) {
				ready <- fmt.Errorf("port-forward error: %s", strings.TrimSpace(line))
				return
			}
		}
	}

	go watchPipe(stdoutPipe, false)
	go watchPipe(stderrPipe, true)

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
