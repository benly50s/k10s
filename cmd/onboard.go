package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/benly/k10s/internal/config"
	"github.com/benly/k10s/internal/deps"
	"github.com/benly/k10s/internal/k8s"
	"github.com/benly/k10s/internal/tui"
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

	// 2. Determine kubeconfig path or directory (via argument or prompt)
	var targetPath string
	if len(args) > 0 {
		targetPath = args[0]
	} else {
		targetPath = "./" // default to current directory if not specified
	}

	targetPath = config.ExpandPath(targetPath)
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("resolving absolute path: %w", err)
	}
	targetPath = absPath

	info, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("path not found: %s", targetPath)
	}

	var kubeconfigsToProcess []string

	if info.IsDir() {
		fmt.Printf("\nScanning directory %s for Kubeconfig files...\n", targetPath)
		foundFiles, err := k8s.ScanForKubeconfigs(targetPath)
		if err != nil {
			return fmt.Errorf("failed to scan directory: %w", err)
		}

		if len(foundFiles) == 0 {
			return fmt.Errorf("no valid kubeconfig files found in %s", targetPath)
		}

		// Run Multi-select TUI
		chosenFiles, err := tui.RunMultiSelect(foundFiles)
		if err != nil {
			return err
		}

		if len(chosenFiles) == 0 {
			fmt.Println("No files selected. Aborting.")
			return nil
		}
		
		kubeconfigsToProcess = chosenFiles
	} else {
		// Single file path provided
		if !k8s.IsValidKubeconfig(targetPath) {
			fmt.Printf("Warning: %s doesn't look like a standard Kubeconfig file, but proceeding anyway.\n", targetPath)
		}
		kubeconfigsToProcess = append(kubeconfigsToProcess, targetPath)
	}

	// 3. Load config and Setup
	fmt.Println("\n=== Step 3: Configure k10s ===")
	cfg, err := config.Load()
	if err != nil {
		defaultCfg := config.DefaultK10sConfig()
		cfg = &defaultCfg
	}

	configsDir := config.ExpandPath(cfg.Global.ConfigsDir)
	if err := os.MkdirAll(configsDir, 0755); err != nil {
		return fmt.Errorf("creating configs config dir: %w", err)
	}

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]config.ProfileConfig)
	}

	processedCount := 0

	for _, srcPath := range kubeconfigsToProcess {
		fileName := filepath.Base(srcPath)
		destPath := filepath.Join(configsDir, fileName)

		// Copy the file
		if err := copyFile(srcPath, destPath); err != nil {
			fmt.Printf("Error copying %s: %v\n", fileName, err)
			continue
		}
		fmt.Printf("Copied %s to %s\n", fileName, destPath)

		// 4. Update config.yaml with ArgoCD defaults
		profileName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		
		pCfg := cfg.Profiles[profileName]
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
		processedCount++
	}

	if processedCount > 0 {
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Printf("\nSuccessfully configured %d profile(s) with ArgoCD defaults.\n", processedCount)
		fmt.Println("Onboarding complete! Run 'k10s' to start.")
	} else {
		fmt.Println("\nNo profiles were successfully processed.")
	}

	return nil
}


