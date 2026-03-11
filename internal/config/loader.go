package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// ConfigDir returns the k10s config directory (~/.k10s)
func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".k10s"
	}
	return filepath.Join(home, ".k10s")
}

// ConfigFilePath returns the path to ~/.k10s/config.yaml
func ConfigFilePath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

// ExpandPath expands ~ to the home directory in a path
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// Load reads ~/.k10s/config.yaml, returning defaults if not found
func Load() (*K10sConfig, error) {
	cfg := DefaultK10sConfig()

	data, err := os.ReadFile(ConfigFilePath())
	if os.IsNotExist(err) {
		return &cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Apply defaults for empty fields
	if cfg.Global.ConfigsDir == "" {
		cfg.Global.ConfigsDir = DefaultConfigsDir
	}
	if cfg.Global.DefaultAction == "" {
		cfg.Global.DefaultAction = DefaultDefaultAction
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]ProfileConfig{}
	}

	if err := Validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return &cfg, nil
}

// Validate checks that config values are within allowed ranges.
func Validate(cfg *K10sConfig) error {
	validActions := map[string]bool{"": true, "select": true, "k9s": true, "argocd": true}
	if !validActions[cfg.Global.DefaultAction] {
		return fmt.Errorf("invalid global default_action %q: must be one of: select, k9s, argocd", cfg.Global.DefaultAction)
	}
	for name, p := range cfg.Profiles {
		if !validActions[p.DefaultAction] {
			return fmt.Errorf("invalid default_action %q in profile %q: must be one of: select, k9s, argocd", p.DefaultAction, name)
		}
	}
	return nil
}

// Save writes the config to ~/.k10s/config.yaml
func Save(cfg *K10sConfig) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(ConfigFilePath(), data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}
