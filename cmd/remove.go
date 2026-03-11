package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/benly/k10s/internal/config"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a kubeconfig profile from the configs directory",
	Args:    cobra.ExactArgs(1),
	RunE:    runRemove,
}

func runRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	dir := config.ExpandPath(cfg.Global.ConfigsDir)

	// Try both .yaml and .yml
	removed := false
	for _, ext := range []string{".yaml", ".yml"} {
		candidate := filepath.Join(dir, name+ext)
		if _, err := os.Stat(candidate); err == nil {
			if err := os.Remove(candidate); err != nil {
				return fmt.Errorf("removing %s: %w", candidate, err)
			}
			fmt.Printf("Removed %s\n", candidate)
			removed = true
			break
		}
	}

	if !removed {
		return fmt.Errorf("profile %q not found in %s", name, dir)
	}

	return nil
}
