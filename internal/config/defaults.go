package config

const (
	DefaultConfigsDir    = "~/.kube/configs"
	DefaultDefaultAction = "select"
)

func DefaultK10sConfig() K10sConfig {
	return K10sConfig{
		Global: GlobalConfig{
			ConfigsDir:    DefaultConfigsDir,
			DefaultAction: DefaultDefaultAction,
		},
		Profiles: map[string]ProfileConfig{},
	}
}
