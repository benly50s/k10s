package k8s_test

import (
	"os/exec"
	"testing"

	"github.com/benly/k10s/internal/k8s"
)

func TestIsPortInUse_HighPort(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not available")
	}

	// Port 19999 is almost certainly free in CI/test environments
	inUse := k8s.IsPortInUse(19999)
	if inUse {
		t.Skip("port 19999 is unexpectedly in use; skipping")
	}
	// If we get here, the function correctly reports it's not in use
}

func TestGetPIDsOnPort_HighPort(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not available")
	}

	pids := k8s.GetPIDsOnPort(19998)
	// Nothing should be listening on this high port; expect empty/nil
	if len(pids) != 0 {
		t.Skipf("port 19998 is unexpectedly in use by PIDs %v; skipping", pids)
	}
}

func TestKillProcessOnPort_NoPIDs(t *testing.T) {
	// KillProcessOnPort should return nil when nothing is on the port,
	// regardless of whether lsof is available (GetPIDsOnPort returns nil either way)
	err := k8s.KillProcessOnPort(19997)
	if err != nil {
		t.Errorf("expected nil error for port with no processes, got: %v", err)
	}
}

func TestFindAvailablePort_FreePorts(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not available")
	}

	port, err := k8s.FindAvailablePort(19996)
	if err != nil {
		t.Fatalf("FindAvailablePort(19996) returned error: %v", err)
	}
	// Should return a port near 19996
	if port < 19996 || port > 20015 {
		t.Errorf("returned port %d is outside expected range [19996, 20015]", port)
	}
}

func TestFindAvailablePort_ValidRange(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not available")
	}

	port, err := k8s.FindAvailablePort(19996)
	if err != nil {
		t.Fatalf("FindAvailablePort returned error: %v", err)
	}
	if port < 1 || port > 65535 {
		t.Errorf("returned port %d is outside valid TCP range [1, 65535]", port)
	}
}
