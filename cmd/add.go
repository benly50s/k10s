package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/benly/k10s/internal/config"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <file>",
	Short: "Add a kubeconfig file to the configs directory",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdd,
}

func runAdd(cmd *cobra.Command, args []string) error {
	src := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	destDir := config.ExpandPath(cfg.Global.ConfigsDir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating configs dir: %w", err)
	}

	dest := filepath.Join(destDir, filepath.Base(src))

	if err := copyFile(src, dest); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}

	fmt.Printf("Added %s -> %s\n", src, dest)
	return nil
}

func copyFile(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Sanitize non-breaking spaces
	cleanStr := strings.ReplaceAll(string(data), "\u00a0", " ")

	// Write back with standard permissions
	return os.WriteFile(dest, []byte(cleanStr), 0644)
}
