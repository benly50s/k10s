package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	validActions := map[string]bool{"": true, "select": true, "k9s": true}
	if !validActions[cfg.Global.DefaultAction] {
		return fmt.Errorf("invalid global default_action %q: must be one of: select, k9s", cfg.Global.DefaultAction)
	}
	for name, p := range cfg.Profiles {
		if !validActions[p.DefaultAction] {
			return fmt.Errorf("invalid default_action %q in profile %q: must be one of: select, k9s", p.DefaultAction, name)
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

// IsFavorite returns true if the given cluster name is in favorites.
func (cfg *K10sConfig) IsFavorite(name string) bool {
	for _, f := range cfg.Global.Favorites {
		if f == name {
			return true
		}
	}
	return false
}

// ToggleFavorite adds or removes a cluster from favorites.
func (cfg *K10sConfig) ToggleFavorite(name string) {
	for i, f := range cfg.Global.Favorites {
		if f == name {
			cfg.Global.Favorites = append(cfg.Global.Favorites[:i], cfg.Global.Favorites[i+1:]...)
			return
		}
	}
	cfg.Global.Favorites = append(cfg.Global.Favorites, name)
}

// UpdateRecent records a cluster as recently used (max 10 entries).
func (cfg *K10sConfig) UpdateRecent(name string) {
	now := time.Now()
	// Remove existing entry if present
	filtered := make([]RecentEntry, 0, len(cfg.Global.Recents))
	for _, r := range cfg.Global.Recents {
		if r.Name != name {
			filtered = append(filtered, r)
		}
	}
	// Prepend new entry
	cfg.Global.Recents = append([]RecentEntry{{Name: name, LastUsed: now}}, filtered...)
	// Cap at 10
	if len(cfg.Global.Recents) > 10 {
		cfg.Global.Recents = cfg.Global.Recents[:10]
	}
}

// RecentIndex returns the position in recents (0-based), or -1 if not found.
func (cfg *K10sConfig) RecentIndex(name string) int {
	for i, r := range cfg.Global.Recents {
		if r.Name == name {
			return i
		}
	}
	return -1
}

// AddPreset adds a port-forward preset (replaces existing with same name).
func (cfg *K10sConfig) AddPreset(preset PortForwardPreset) {
	for i, p := range cfg.Global.PortForwardPresets {
		if p.Name == preset.Name {
			cfg.Global.PortForwardPresets[i] = preset
			return
		}
	}
	cfg.Global.PortForwardPresets = append(cfg.Global.PortForwardPresets, preset)
}

// RemovePreset removes a port-forward preset by name.
func (cfg *K10sConfig) RemovePreset(name string) {
	for i, p := range cfg.Global.PortForwardPresets {
		if p.Name == name {
			cfg.Global.PortForwardPresets = append(cfg.Global.PortForwardPresets[:i], cfg.Global.PortForwardPresets[i+1:]...)
			return
		}
	}
}

// GetPresetsForProfile returns presets matching the given profile name.
func (cfg *K10sConfig) GetPresetsForProfile(profileName string) []PortForwardPreset {
	var out []PortForwardPreset
	for _, p := range cfg.Global.PortForwardPresets {
		if p.Profile == profileName {
			out = append(out, p)
		}
	}
	return out
}

// AddPFHistory records a port-forward as recently used (max 20 entries).
// Deduplicates by matching all fields except LastUsed.
func (cfg *K10sConfig) AddPFHistory(entry PortForwardHistoryEntry) {
	entry.LastUsed = time.Now()

	// Remove existing duplicate if present
	filtered := make([]PortForwardHistoryEntry, 0, len(cfg.Global.PortForwardHistory))
	for _, h := range cfg.Global.PortForwardHistory {
		if h.Profile == entry.Profile &&
			h.Namespace == entry.Namespace &&
			h.ResourceType == entry.ResourceType &&
			h.ResourceName == entry.ResourceName &&
			h.LocalPort == entry.LocalPort &&
			h.RemotePort == entry.RemotePort {
			continue
		}
		filtered = append(filtered, h)
	}

	// Prepend new entry
	cfg.Global.PortForwardHistory = append([]PortForwardHistoryEntry{entry}, filtered...)

	// Cap at 20
	if len(cfg.Global.PortForwardHistory) > 20 {
		cfg.Global.PortForwardHistory = cfg.Global.PortForwardHistory[:20]
	}
}

// GetPFHistoryForProfile returns history entries matching the given profile name.
func (cfg *K10sConfig) GetPFHistoryForProfile(profileName string) []PortForwardHistoryEntry {
	var out []PortForwardHistoryEntry
	for _, h := range cfg.Global.PortForwardHistory {
		if h.Profile == profileName {
			out = append(out, h)
		}
	}
	return out
}
