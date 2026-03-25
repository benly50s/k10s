package cmd

import (
	"fmt"
	"os"

	"github.com/benly/k10s/internal/config"
	"github.com/benly/k10s/internal/deps"
	"github.com/benly/k10s/internal/executor"
	"github.com/benly/k10s/internal/k8s"
	"github.com/benly/k10s/internal/portforward"
	"github.com/benly/k10s/internal/profile"
	"github.com/benly/k10s/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "k10s",
	Short: "Benly's Kubernetes Cluster Manager with TUI",
	Long: `k10s is a CLI tool for managing multiple Kubernetes clusters.
It provides a TUI for selecting clusters and launching k9s or a shell.`,
	RunE: runRoot,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(onboardCmd)
	rootCmd.AddCommand(completionCmd)

	rootCmd.Flags().BoolVar(&k8s.DemoMode, "demo", false, "Run in demo mode with fake data")
}

func runRoot(cmd *cobra.Command, args []string) error {
	// [1] Auto doctor check (warn if deps missing, don't block)
	result := deps.Check()
	if !result.OK {
		fmt.Fprintln(os.Stderr, "Warning: some dependencies are missing. Run 'k10s doctor' for details.")
	}

	// [2] Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Port-forward manager lives across TUI loops
	pfManager := portforward.NewManager()
	defer pfManager.StopAll()

	for {
		var profiles []profile.Profile
		var err error

		if k8s.DemoMode {
			profiles = []profile.Profile{
				{Name: "dev-cluster", FilePath: "(demo)", Context: "dev"},
				{Name: "staging-cluster", FilePath: "(demo)", Context: "staging"},
				{Name: "prod-cluster", FilePath: "(demo)", Context: "prod"},
			}
		} else {
			// [3] Scan profiles (re-scan each loop to reflect deletions/additions)
			profiles, err = profile.Scan(cfg)
			if err != nil {
				return fmt.Errorf("scanning profiles: %w", err)
			}
		}

		if len(profiles) == 0 {
			fmt.Println("No kubeconfig profiles found.")
			fmt.Printf("Add kubeconfig files to %s or run 'k10s add <file>'\n",
				config.ExpandPath(cfg.Global.ConfigsDir))
			return nil
		}

		// [4] Run TUI
		executeMsg, err := tui.Run(profiles, pfManager, cfg)
		if err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}

		if executeMsg == nil {
			// User quit with q
			return nil
		}

		// [5] Record recent usage and execute
		cfg.UpdateRecent(executeMsg.Profile.Name)
		_ = config.Save(cfg)

		if err := executeAction(executeMsg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}

func executeAction(msg *tui.ExecuteMsg) error {
	p := msg.Profile

	switch msg.Action {
	case tui.ActionK9s:
		fmt.Printf("Launching k9s with KUBECONFIG=%s\n", p.FilePath)
		return executor.LaunchK9s(p.FilePath, p.Context, "")

	case tui.ActionShell:
		fmt.Printf("Dropping into %s shell with KUBECONFIG=%s\n", os.Getenv("SHELL"), p.FilePath)
		return executor.LaunchShell(p.FilePath, p.Context)

	default:
		return fmt.Errorf("unknown action: %v", msg.Action)
	}
}
