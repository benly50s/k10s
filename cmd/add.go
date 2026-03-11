package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Sync()
}
