package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/benly/k10s/internal/config"
	"github.com/benly/k10s/internal/profile"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage k10s configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize ~/.k10s/config.yaml with detected profiles",
	RunE:  runConfigInit,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration and detected profiles",
	RunE:  runConfigShow,
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open config file in $EDITOR",
	RunE:  runConfigEdit,
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	cfgPath := config.ConfigFilePath()

	if _, err := os.Stat(cfgPath); err == nil {
		fmt.Printf("Config file already exists: %s\n", cfgPath)
		fmt.Print("Overwrite? [y/N] ")
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Load or create default config
	cfg := config.DefaultK10sConfig()

	// Scan for existing profiles to populate
	profiles, err := profile.Scan(&cfg)
	if err == nil && len(profiles) > 0 {
		for _, p := range profiles {
			profileCfg := config.ProfileConfig{
				OIDC: p.OIDC,
			}
			cfg.Profiles[p.Name] = profileCfg
		}
	}

	if err := config.Save(&cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Created config file: %s\n", cfgPath)
	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	fmt.Printf("Config file: %s\n\n", config.ConfigFilePath())

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	fmt.Println("=== Configuration ===")
	fmt.Println(string(data))

	// Show detected profiles
	profiles, err := profile.Scan(cfg)
	if err != nil {
		fmt.Printf("Warning: could not scan profiles: %v\n", err)
		return nil
	}

	fmt.Printf("=== Detected Profiles (%d) ===\n", len(profiles))
	for _, p := range profiles {
		oidcTag := ""
		if p.OIDC {
			oidcTag = " [OIDC]"
		}
		fmt.Printf("  %-30s %s%s\n", p.Name, p.ServerURL, oidcTag)
	}

	return nil
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	cfgPath := config.ConfigFilePath()

	// Ensure the file exists
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		cfg := config.DefaultK10sConfig()
		if err := config.Save(&cfg); err != nil {
			return fmt.Errorf("creating config: %w", err)
		}
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	editorCmd := exec.Command(editor, cfgPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	return nil
}
