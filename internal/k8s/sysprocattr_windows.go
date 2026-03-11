//go:build windows

package k8s

import (
	"os/exec"
)

func setSysProcAttr(cmd *exec.Cmd) {
	// Not supported or needed on Windows in the same way
}
