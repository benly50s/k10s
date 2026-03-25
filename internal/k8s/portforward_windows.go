//go:build windows

package k8s

// IsAlive checks whether the port-forward process is still running.
func (h *PortForwardHandle) IsAlive() bool {
	p := h.process()
	if p == nil {
		return false
	}
	if h.Cmd != nil && h.Cmd.ProcessState != nil {
		return false
	}
	return true
}

// Stop kills the port-forward process.
func (h *PortForwardHandle) Stop() {
	p := h.process()
	if p == nil {
		return
	}
	_ = p.Kill()
	if h.Cmd != nil {
		_ = h.Cmd.Wait()
	}
}
