//go:build !windows

package k8s

import (
	"os/exec"
	"syscall"
)

func setSysProcAttr(cmd *exec.Cmd) {
	// We keep the child in the same process group so that it receives
	// signals (like SIGHUP when the terminal closes) alongside the parent.
	cmd.SysProcAttr = &syscall.SysProcAttr{}
}
