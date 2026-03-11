package argocd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/benly/k10s/internal/auth"
	"github.com/benly/k10s/internal/config"
	"github.com/benly/k10s/internal/k8s"
)

// Connect orchestrates the full ArgoCD connection flow:
// OIDC auth → port conflict check/resolve → port-forward → argocd login → open browser
func Connect(kubeconfigPath, context string, oidc bool, argocdCfg *config.ArgocdConfig) error {
	// [1] OIDC authentication
	if oidc {
		fmt.Println("Running OIDC authentication...")
		if err := auth.RefreshOIDC(kubeconfigPath, context); err != nil {
			fmt.Printf("Warning: OIDC auth failed: %v\n", err)
		}
	}

	// [2] Resolve port (auto-allocate if preferred is busy)
	localPort, err := resolvePort(argocdCfg.LocalPort)
	if err != nil {
		return err
	}
	if localPort != argocdCfg.LocalPort {
		fmt.Printf("Port %d was in use, using %d instead.\n", argocdCfg.LocalPort, localPort)
	}

	// [3] Start port-forward
	fmt.Printf("Starting port-forward %d -> %s/%s:%d...\n",
		localPort, argocdCfg.Namespace, argocdCfg.Service, argocdCfg.RemotePort)

	handle, err := k8s.StartPortForward(
		kubeconfigPath,
		context,
		argocdCfg.Namespace,
		argocdCfg.Service,
		localPort,
		argocdCfg.RemotePort,
	)
	if err != nil {
		return fmt.Errorf("port-forward failed: %w", err)
	}
	defer handle.Stop()

	fmt.Printf("Port-forward established on localhost:%d\n", localPort)

	// [4] ArgoCD login
	fmt.Println("Logging in to ArgoCD...")
	if err := Login(localPort, argocdCfg.Username, argocdCfg.Password, argocdCfg.Insecure); err != nil {
		fmt.Printf("Warning: ArgoCD login failed: %v\n", err)
	}

	// [5] Open browser (update URL port if we auto-allocated a different port)
	browserURL := argocdCfg.URL
	if browserURL == "" {
		scheme := "http"
		if !argocdCfg.Insecure {
			scheme = "https"
		}
		browserURL = fmt.Sprintf("%s://localhost:%d", scheme, localPort)
	}
	fmt.Printf("Opening browser: %s\n", browserURL)
	if err := OpenBrowser(browserURL); err != nil {
		fmt.Printf("Warning: could not open browser: %v\n", err)
	}

	// [6] Wait for Ctrl+C, then clean up
	fmt.Println("\nPort-forward is running. Press Ctrl+C to stop.")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down port-forward...")
	return nil
}

// PortForwardOnly starts port-forward and waits for Ctrl+C
func PortForwardOnly(kubeconfigPath, context string, oidc bool, argocdCfg *config.ArgocdConfig) error {
	if oidc {
		fmt.Println("Running OIDC authentication...")
		if err := auth.RefreshOIDC(kubeconfigPath, context); err != nil {
			fmt.Printf("Warning: OIDC auth failed: %v\n", err)
		}
	}

	localPort, err := resolvePort(argocdCfg.LocalPort)
	if err != nil {
		return err
	}
	if localPort != argocdCfg.LocalPort {
		fmt.Printf("Port %d was in use, using %d instead.\n", argocdCfg.LocalPort, localPort)
	}

	fmt.Printf("Starting port-forward %d -> %s/%s:%d...\n",
		localPort, argocdCfg.Namespace, argocdCfg.Service, argocdCfg.RemotePort)

	handle, err := k8s.StartPortForward(
		kubeconfigPath,
		context,
		argocdCfg.Namespace,
		argocdCfg.Service,
		localPort,
		argocdCfg.RemotePort,
	)
	if err != nil {
		return fmt.Errorf("port-forward failed: %w", err)
	}
	defer handle.Stop()

	fmt.Printf("Port-forward running on localhost:%d\nPress Ctrl+C to stop.\n", localPort)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down port-forward...")
	return nil
}

// resolvePort checks if the preferred port is available.
// If it's in use by a kubectl port-forward process, offer to kill it.
// If it's in use by something else, auto-find the next available port.
func resolvePort(preferred int) (int, error) {
	if !k8s.IsPortInUse(preferred) {
		return preferred, nil
	}

	pids := k8s.GetPIDsOnPort(preferred)
	isOurPortForward := false
	for _, pid := range pids {
		if isKubectlPortForward(pid) {
			isOurPortForward = true
			break
		}
	}

	if isOurPortForward {
		// Orphan port-forward from a previous session — kill it
		fmt.Printf("Found orphan kubectl port-forward on port %d (pids: %v), killing...\n", preferred, pids)
		if err := k8s.KillProcessOnPort(preferred); err != nil {
			fmt.Printf("Warning: could not kill orphan process: %v\n", err)
		} else {
			fmt.Printf("Killed orphan port-forward on port %d.\n", preferred)
			return preferred, nil
		}
	}

	// Port taken by a non-port-forward process — find next available
	port, err := k8s.FindAvailablePort(preferred + 1)
	if err != nil {
		return 0, fmt.Errorf("port %d is in use and %w", preferred, err)
	}
	return port, nil
}

// isKubectlPortForward checks if a PID belongs to a kubectl port-forward process
func isKubectlPortForward(pid int) bool {
	// Read /proc/<pid>/comm on Linux or use ps on macOS
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err == nil {
		return strings.Contains(string(data), "kubectl")
	}
	// macOS fallback: use ps
	out, err := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "command=").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "kubectl") && strings.Contains(string(out), "port-forward")
}
