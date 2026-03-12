package profile

import (
	"github.com/benly/k10s/internal/config"
)

// Resolve merges profile config > global config > built-in defaults
// and returns a resolved Profile. context is the kubectl context name (empty for file-based profiles).
func Resolve(name, filePath, context, serverURL string, oidcDetected bool, cfg *config.K10sConfig) Profile {
	p := Profile{
		Name:      name,
		FilePath:  filePath,
		Context:   context,
		ServerURL: serverURL,
	}

	// Start from built-in defaults
	defaultAction := cfg.Global.DefaultAction
	if defaultAction == "" {
		defaultAction = config.DefaultDefaultAction
	}

	oidc := oidcDetected
	var argocdCfg *config.ArgocdConfig

	// Apply profile-specific config if present
	if profileCfg, ok := cfg.Profiles[name]; ok {
		if profileCfg.DefaultAction != "" {
			defaultAction = profileCfg.DefaultAction
		}
		// Profile OIDC setting OR auto-detection (either one being true wins)
		oidc = oidcDetected || profileCfg.OIDC

		if profileCfg.Argocd != nil {
			// Merge argocd config with defaults
			defaults := config.DefaultArgocdConfig()
			merged := *profileCfg.Argocd

			if merged.Namespace == "" {
				merged.Namespace = defaults.Namespace
			}
			if merged.Service == "" {
				merged.Service = defaults.Service
			}
			if merged.LocalPort == 0 {
				merged.LocalPort = defaults.LocalPort
			}
			if merged.RemotePort == 0 {
				merged.RemotePort = defaults.RemotePort
			}
			if merged.Username == "" {
				merged.Username = defaults.Username
			}
			// Validate port ranges
			if merged.LocalPort < 1 || merged.LocalPort > 65535 {
				merged.LocalPort = defaults.LocalPort
			}
			if merged.RemotePort < 1 || merged.RemotePort > 65535 {
				merged.RemotePort = defaults.RemotePort
			}
			argocdCfg = &merged
		}
	}

	p.DefaultAction = defaultAction
	p.OIDC = oidc
	p.Argocd = argocdCfg

	return p
}
