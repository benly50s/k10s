package k8s

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// DiscoveredPortForward represents a kubectl port-forward process found on the system.
type DiscoveredPortForward struct {
	PID            int
	KubeconfigPath string
	Context        string
	Namespace      string
	ResourceType   string
	ResourceName   string
	LocalPort      int
	RemotePort     int
	Handle         *PortForwardHandle
}

// DiscoverPortForwards scans running processes for kubectl port-forward commands
// and returns parsed entries. It skips processes owned by the current k10s session
// (identified by ownPIDs).
func DiscoverPortForwards(ownPIDs map[int]bool) []DiscoveredPortForward {
	lines, err := listKubectlPortForwardProcesses()
	if err != nil {
		return nil
	}

	myPID := os.Getpid()
	var results []DiscoveredPortForward

	for _, line := range lines {
		pid, args := parsePSLine(line)
		if pid <= 0 {
			continue
		}
		// Skip our own child processes and current process
		if ownPIDs[pid] || pid == myPID {
			continue
		}

		d := parseKubectlArgs(pid, args)
		if d == nil {
			continue
		}

		proc, err := os.FindProcess(d.PID)
		if err != nil {
			continue
		}
		d.Handle = &PortForwardHandle{
			Process:   proc,
			LocalPort: d.LocalPort,
		}

		results = append(results, *d)
	}

	return results
}

// listKubectlPortForwardProcesses returns ps output lines for kubectl port-forward processes.
func listKubectlPortForwardProcesses() ([]string, error) {
	// ps -eo pid,args: show PID and full command line for all processes
	cmd := exec.Command("ps", "-eo", "pid,args")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running ps: %w", err)
	}

	var matches []string
	for _, line := range strings.Split(string(out), "\n") {
		// Match lines containing both "kubectl" and "port-forward"
		if strings.Contains(line, "kubectl") && strings.Contains(line, "port-forward") {
			// Skip the grep/ps process itself
			if strings.Contains(line, "ps -eo") {
				continue
			}
			matches = append(matches, line)
		}
	}
	return matches, nil
}

// parsePSLine extracts PID and args string from a ps output line.
// Format: "  12345 kubectl --kubeconfig ..."
func parsePSLine(line string) (int, string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, ""
	}

	// Split into PID and rest
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		return 0, ""
	}

	pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, ""
	}

	return pid, strings.TrimSpace(parts[1])
}

// parseKubectlArgs parses kubectl port-forward command line arguments.
// Expected format: kubectl [--kubeconfig path] [--context ctx] port-forward [-n ns] type/name local:remote
func parseKubectlArgs(pid int, argsStr string) *DiscoveredPortForward {
	fields := strings.Fields(argsStr)
	if len(fields) < 2 {
		return nil
	}

	d := &DiscoveredPortForward{PID: pid}

	// Find the "port-forward" subcommand position
	pfIdx := -1
	for i, f := range fields {
		if f == "port-forward" {
			pfIdx = i
			break
		}
	}
	if pfIdx < 0 {
		return nil
	}

	// Parse flags before "port-forward"
	for i := 0; i < pfIdx; i++ {
		switch fields[i] {
		case "--kubeconfig":
			if i+1 < pfIdx {
				d.KubeconfigPath = fields[i+1]
				i++
			}
		case "--context":
			if i+1 < pfIdx {
				d.Context = fields[i+1]
				i++
			}
		}
		// Handle --kubeconfig=value format
		if strings.HasPrefix(fields[i], "--kubeconfig=") {
			d.KubeconfigPath = strings.TrimPrefix(fields[i], "--kubeconfig=")
		}
		if strings.HasPrefix(fields[i], "--context=") {
			d.Context = strings.TrimPrefix(fields[i], "--context=")
		}
	}

	// Parse args after "port-forward"
	afterPF := fields[pfIdx+1:]
	for i := 0; i < len(afterPF); i++ {
		arg := afterPF[i]

		// Namespace flag
		if arg == "-n" || arg == "--namespace" {
			if i+1 < len(afterPF) {
				d.Namespace = afterPF[i+1]
				i++
				continue
			}
		}
		if strings.HasPrefix(arg, "-n=") {
			d.Namespace = strings.TrimPrefix(arg, "-n=")
			continue
		}
		if strings.HasPrefix(arg, "--namespace=") {
			d.Namespace = strings.TrimPrefix(arg, "--namespace=")
			continue
		}

		// Skip other flags
		if strings.HasPrefix(arg, "-") {
			continue
		}

		// Resource type/name (e.g., svc/api, pod/web-abc123, deployment/app)
		if strings.Contains(arg, "/") && d.ResourceType == "" {
			parts := strings.SplitN(arg, "/", 2)
			d.ResourceType = parts[0]
			d.ResourceName = parts[1]
			continue
		}

		// Port mapping (e.g., 8080:80)
		if strings.Contains(arg, ":") && d.LocalPort == 0 {
			parts := strings.SplitN(arg, ":", 2)
			local, err1 := strconv.Atoi(parts[0])
			remote, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil {
				d.LocalPort = local
				d.RemotePort = remote
			}
			continue
		}
	}

	// Validate that we got the essential fields
	if d.ResourceType == "" || d.LocalPort == 0 {
		return nil
	}

	// Default namespace
	if d.Namespace == "" {
		d.Namespace = "default"
	}

	return d
}
