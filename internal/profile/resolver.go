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

	defaultAction := cfg.Global.DefaultAction
	if defaultAction == "" {
		defaultAction = config.DefaultDefaultAction
	}

	oidc := oidcDetected

	if profileCfg, ok := cfg.Profiles[name]; ok {
		if profileCfg.DefaultAction != "" {
			defaultAction = profileCfg.DefaultAction
		}
		oidc = oidcDetected || profileCfg.OIDC
	}

	p.DefaultAction = defaultAction
	p.OIDC = oidc

	return p
}
