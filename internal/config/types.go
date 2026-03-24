package config

import "time"

type GlobalConfig struct {
	ConfigsDir     string `yaml:"configs_dir"     json:"configs_dir"`
	DefaultAction  string `yaml:"default_action"  json:"default_action"`  // select | k9s
	KubeconfigFile string `yaml:"kubeconfig_file" json:"kubeconfig_file"` // single kubeconfig with multiple contexts
	ContextsMode   bool   `yaml:"contexts_mode"   json:"contexts_mode"`   // true: scan contexts from KubeconfigFile

	Favorites          []string             `yaml:"favorites,omitempty"            json:"favorites,omitempty"`
	Recents            []RecentEntry        `yaml:"recents,omitempty"              json:"recents,omitempty"`
	PortForwardPresets []PortForwardPreset  `yaml:"port_forward_presets,omitempty" json:"port_forward_presets,omitempty"`
}

type RecentEntry struct {
	Name     string    `yaml:"name"      json:"name"`
	LastUsed time.Time `yaml:"last_used" json:"last_used"`
}

type PortForwardPreset struct {
	Name         string `yaml:"name"          json:"name"`
	Profile      string `yaml:"profile"       json:"profile"`
	Namespace    string `yaml:"namespace"     json:"namespace"`
	ResourceType string `yaml:"resource_type" json:"resource_type"`
	ResourceName string `yaml:"resource_name" json:"resource_name"`
	LocalPort    int    `yaml:"local_port"    json:"local_port"`
	RemotePort   int    `yaml:"remote_port"   json:"remote_port"`
}

type ProfileConfig struct {
	DefaultAction string `yaml:"default_action,omitempty" json:"default_action,omitempty"`
	OIDC          bool   `yaml:"oidc,omitempty"           json:"oidc,omitempty"`
}

type K10sConfig struct {
	Global   GlobalConfig             `yaml:"global"   json:"global"`
	Profiles map[string]ProfileConfig `yaml:"profiles" json:"profiles"`
}
