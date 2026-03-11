package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/benly/k10s/internal/config"
	"github.com/benly/k10s/internal/deps"
	"github.com/spf13/cobra"
)

var onboardCmd = &cobra.Command{
	Use:   "onboard [kubeconfig-file]",
	Short: "Onboard a new kubeconfig and auto-configure k10s with ArgoCD defaults",
	RunE:  runOnboard,
}

func runOnboard(cmd *cobra.Command, args []string) error {
	// 1. Dependency Check and Auto-Install Prompt
	fmt.Println("=== Step 1: Checking Dependencies ===")
	result := deps.Check()
	deps.PrintReport(result)

	reader := bufio.NewReader(os.Stdin)
	if !result.OK {
		fmt.Println("\nMissing required dependencies. Let's fix them.")
		for _, d := range result.Deps {
			if !d.Found && d.Required {
					brewPkg := d.Brew
					if brewPkg == "" {
						brewPkg = d.Name
					}
				fmt.Printf("Install %s via brew? [y/N] ", brewPkg)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer == "y" || answer == "yes" {
					if err := deps.InstallViaBrew(brewPkg); err != nil {
						fmt.Fprintf(os.Stderr, "Error installing %s: %v\n", brewPkg, err)
					}
				}
			}
		}
	}

	// 2. Determine kubeconfig path (via argument or prompt)
	var srcPath string
	if len(args) > 0 {
		srcPath = args[0]
	} else {
		fmt.Println("\n=== Step 2: Add Kubeconfig ===")
		fmt.Print("Enter the path to your kubeconfig file (e.g., ~/Downloads/my-cluster.yaml): ")
		filePath, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		srcPath = strings.TrimSpace(filePath)
	}

	if srcPath == "" {
		return fmt.Errorf("no path provided")
	}

	// Resolve ~ and relative paths
	srcPath = config.ExpandPath(srcPath)
	absPath, err := filepath.Abs(srcPath)
	if err != nil {
		return fmt.Errorf("resolving absolute path: %w", err)
	}
	srcPath = absPath

	// Verify file exists
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", srcPath)
	}

	// 3. Load config and Setup
	fmt.Println("\n=== Step 3: Configure k10s ===")
	cfg, err := config.Load()
	if err != nil {
		// If fails to load, create default
		defaultCfg := config.DefaultK10sConfig()
		cfg = &defaultCfg
	}

	configsDir := config.ExpandPath(cfg.Global.ConfigsDir)
	if err := os.MkdirAll(configsDir, 0755); err != nil {
		return fmt.Errorf("creating configs config dir: %w", err)
	}

	fileName := filepath.Base(srcPath)
	destPath := filepath.Join(configsDir, fileName)

	// Copy the file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}
	fmt.Printf("Copied %s to %s\n", srcPath, destPath)

	// 4. Update config.yaml with ArgoCD defaults
	profileName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]config.ProfileConfig)
	}

	// Fetch existing or create new
	pCfg := cfg.Profiles[profileName]
	
	// Pre-fill ArgoCD defaults (password empty for dynamic fetch)
	pCfg.Argocd = &config.ArgocdConfig{
		Namespace:  "argocd",
		Service:    "argocd-server",
		LocalPort:  8080,
		RemotePort: 443,
		URL:        "https://localhost:8080",
		Username:   "admin",
		Password:   "", // empty triggers dynamic fetch
		Insecure:   true,
	}

	cfg.Profiles[profileName] = pCfg

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Successfully configured profile '%s' with ArgoCD defaults.\n", profileName)
	fmt.Println("\nOnboarding complete! Run 'k10s' to start.")

	return nil
}
