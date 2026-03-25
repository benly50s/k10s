//go:build !windows

package k8s

import (
	"syscall"
	"time"
)

// IsAlive checks whether the port-forward process is still running.
func (h *PortForwardHandle) IsAlive() bool {
	p := h.process()
	if p == nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// Stop gracefully terminates the port-forward process.
// It sends SIGTERM first and falls back to SIGKILL after 2 seconds.
func (h *PortForwardHandle) Stop() {
	p := h.process()
	if p == nil {
		return
	}
	_ = p.Signal(syscall.SIGTERM)

	done := make(chan struct{}, 1)
	go func() {
		if h.Cmd != nil {
			_ = h.Cmd.Wait()
		} else {
			// External process: poll until dead
			for i := 0; i < 20; i++ {
				time.Sleep(100 * time.Millisecond)
				if p.Signal(syscall.Signal(0)) != nil {
					break
				}
			}
		}
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = p.Kill()
		if h.Cmd != nil {
			_ = h.Cmd.Wait()
		}
	}
}
