package deps

import (
	"fmt"
	"os/exec"
)

// Dep represents a dependency to check
type Dep struct {
	Name        string
	Brew        string // brew package name (if different from Name)
	Required    bool   // if true, doctor will offer to install when missing
	Found       bool
	Version     string
}

// CheckResult holds the result of dependency checking
type CheckResult struct {
	Deps []Dep
	OK   bool
}

// Check checks whether kubectl, k9s, kubelogin, and argocd are available in PATH
func Check() CheckResult {
	deps := []Dep{
		{Name: "kubectl",   Brew: "kubernetes-cli", Required: true},
		{Name: "k9s",       Brew: "k9s",            Required: true},
		{Name: "lsof",      Brew: "lsof",            Required: true},
		{Name: "kubectl-oidc_login", Brew: "kubelogin",       Required: false},
		{Name: "argocd",    Brew: "argocd",          Required: false},
	}

	allOK := true
	for i, d := range deps {
		_, err := exec.LookPath(d.Name)
		if err == nil {
			deps[i].Found = true
			// Version detection omitted intentionally (tool-specific flags vary)
		} else if d.Required {
			allOK = false
		}
	}

	return CheckResult{Deps: deps, OK: allOK}
}

// InstallViaBrew installs a package using brew
func InstallViaBrew(pkg string) error {
	fmt.Printf("Installing %s via brew...\n", pkg)
	out, err := exec.Command("brew", "install", pkg).CombinedOutput()
	if err != nil {
		return fmt.Errorf("brew install %s failed: %w\n%s", pkg, err, string(out))
	}
	fmt.Printf("Successfully installed %s\n", pkg)
	return nil
}

// PrintReport prints a human-readable dependency report
func PrintReport(result CheckResult) {
	fmt.Println("Dependency check:")
	fmt.Println()
	for _, d := range result.Deps {
		tag := ""
		if !d.Required {
			tag = " (optional)"
		}
		
		displayName := d.Brew
		if displayName == "" {
			displayName = d.Name
		}

		if d.Found {
			fmt.Printf("  ✓ %-16s found%s\n", displayName, tag)
		} else {
			fmt.Printf("  ✗ %-16s not found%s\n", displayName, tag)
		}
	}
	fmt.Println()
	if result.OK {
		fmt.Println("All required dependencies satisfied.")
	} else {
		fmt.Println("Some required dependencies are missing.")
	}
}
