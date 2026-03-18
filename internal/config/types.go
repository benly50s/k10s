package config

type GlobalConfig struct {
	ConfigsDir     string `yaml:"configs_dir"     json:"configs_dir"`
	DefaultAction  string `yaml:"default_action"  json:"default_action"`  // select | k9s
	KubeconfigFile string `yaml:"kubeconfig_file" json:"kubeconfig_file"` // single kubeconfig with multiple contexts
	ContextsMode   bool   `yaml:"contexts_mode"   json:"contexts_mode"`   // true: scan contexts from KubeconfigFile
}

type ProfileConfig struct {
	DefaultAction string `yaml:"default_action,omitempty" json:"default_action,omitempty"`
	OIDC          bool   `yaml:"oidc,omitempty"           json:"oidc,omitempty"`
}

type K10sConfig struct {
	Global   GlobalConfig             `yaml:"global"   json:"global"`
	Profiles map[string]ProfileConfig `yaml:"profiles" json:"profiles"`
}
