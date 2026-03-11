package config

const (
	DefaultConfigsDir    = "~/.kube/configs"
	DefaultDefaultAction = "select"
	DefaultArgocdNamespace  = "argocd"
	DefaultArgocdService    = "argocd-server"
	DefaultArgocdLocalPort  = 8080
	DefaultArgocdRemotePort = 443
	DefaultArgocdUsername   = "admin"
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

func DefaultArgocdConfig() ArgocdConfig {
	return ArgocdConfig{
		Namespace:  DefaultArgocdNamespace,
		Service:    DefaultArgocdService,
		LocalPort:  DefaultArgocdLocalPort,
		RemotePort: DefaultArgocdRemotePort,
		Username:   DefaultArgocdUsername,
		Insecure:   true,
	}
}
