package argocd

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser opens the given URL in the default browser.
// Supports macOS, Linux (xdg-open), and Windows.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported OS for browser open: %s", runtime.GOOS)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("opening browser: %w", err)
	}
	return nil
}
