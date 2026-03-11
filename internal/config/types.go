package config

type GlobalConfig struct {
	ConfigsDir     string `yaml:"configs_dir"     json:"configs_dir"`
	DefaultAction  string `yaml:"default_action"  json:"default_action"`  // select | k9s | argocd
	KubeconfigFile string `yaml:"kubeconfig_file" json:"kubeconfig_file"` // single kubeconfig with multiple contexts
	ContextsMode   bool   `yaml:"contexts_mode"   json:"contexts_mode"`   // true: scan contexts from KubeconfigFile
}

type ArgocdConfig struct {
	Namespace  string `yaml:"namespace"           json:"namespace"`
	Service    string `yaml:"service"             json:"service"`
	LocalPort  int    `yaml:"local_port"          json:"local_port"`
	RemotePort int    `yaml:"remote_port"         json:"remote_port"`
	URL        string `yaml:"url"                 json:"url"`
	Username   string `yaml:"username"            json:"username"`
	Password   string `yaml:"password,omitempty"  json:"password,omitempty"`
	Insecure   bool   `yaml:"insecure"            json:"insecure"`
}

type ProfileConfig struct {
	DefaultAction string        `yaml:"default_action,omitempty" json:"default_action,omitempty"`
	OIDC          bool          `yaml:"oidc,omitempty"           json:"oidc,omitempty"`
	Argocd        *ArgocdConfig `yaml:"argocd,omitempty"         json:"argocd,omitempty"`
}

type K10sConfig struct {
	Global   GlobalConfig             `yaml:"global"   json:"global"`
	Profiles map[string]ProfileConfig `yaml:"profiles" json:"profiles"`
}
