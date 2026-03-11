package profile_test

import (
	"testing"

	"github.com/benly/k10s/internal/config"
	"github.com/benly/k10s/internal/profile"
)

func defaultTestConfig() *config.K10sConfig {
	cfg := config.DefaultK10sConfig()
	return &cfg
}

func TestResolve_Defaults(t *testing.T) {
	cfg := defaultTestConfig()

	p := profile.Resolve("my-profile", "/path/to/kubeconfig", "", "", false, cfg)

	if p.DefaultAction != "select" {
		t.Errorf("DefaultAction = %q; want %q", p.DefaultAction, "select")
	}
	if p.OIDC {
		t.Error("expected OIDC=false")
	}
	if p.Argocd != nil {
		t.Error("expected Argocd=nil")
	}
}

func TestResolve_ProfileOverridesGlobal(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Global.DefaultAction = "argocd"
	cfg.Profiles = map[string]config.ProfileConfig{
		"my-profile": {DefaultAction: "k9s"},
	}

	p := profile.Resolve("my-profile", "/path/to/kubeconfig", "", "", false, cfg)

	if p.DefaultAction != "k9s" {
		t.Errorf("DefaultAction = %q; want %q", p.DefaultAction, "k9s")
	}
}

func TestResolve_GlobalOverridesBuiltin(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Global.DefaultAction = "k9s"

	p := profile.Resolve("no-profile-entry", "/path/to/kubeconfig", "", "", false, cfg)

	if p.DefaultAction != "k9s" {
		t.Errorf("DefaultAction = %q; want %q", p.DefaultAction, "k9s")
	}
}

func TestResolve_OIDCOverridesDetected(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Profiles = map[string]config.ProfileConfig{
		"my-profile": {OIDC: true},
	}

	// oidcDetected=false but profile explicitly sets OIDC=true
	p := profile.Resolve("my-profile", "/path/to/kubeconfig", "", "", false, cfg)

	if !p.OIDC {
		t.Error("expected OIDC=true from profile config override")
	}
}

func TestResolve_ArgocdDefaults(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Profiles = map[string]config.ProfileConfig{
		"my-profile": {
			Argocd: &config.ArgocdConfig{
				Namespace: "my-namespace",
			},
		},
	}

	p := profile.Resolve("my-profile", "/path/to/kubeconfig", "", "", false, cfg)

	if p.Argocd == nil {
		t.Fatal("expected Argocd config to be set")
	}
	if p.Argocd.Service != config.DefaultArgocdService {
		t.Errorf("Service = %q; want %q", p.Argocd.Service, config.DefaultArgocdService)
	}
	if p.Argocd.LocalPort != config.DefaultArgocdLocalPort {
		t.Errorf("LocalPort = %d; want %d", p.Argocd.LocalPort, config.DefaultArgocdLocalPort)
	}
	if p.Argocd.Username != config.DefaultArgocdUsername {
		t.Errorf("Username = %q; want %q", p.Argocd.Username, config.DefaultArgocdUsername)
	}
}

func TestResolve_PortRangeValidation_Zero(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Profiles = map[string]config.ProfileConfig{
		"my-profile": {
			Argocd: &config.ArgocdConfig{
				LocalPort: 0,
			},
		},
	}

	p := profile.Resolve("my-profile", "/path/to/kubeconfig", "", "", false, cfg)

	if p.Argocd == nil {
		t.Fatal("expected Argocd config to be set")
	}
	if p.Argocd.LocalPort != config.DefaultArgocdLocalPort {
		t.Errorf("LocalPort = %d; want %d (default)", p.Argocd.LocalPort, config.DefaultArgocdLocalPort)
	}
}

func TestResolve_PortRangeValidation_TooHigh(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Profiles = map[string]config.ProfileConfig{
		"my-profile": {
			Argocd: &config.ArgocdConfig{
				LocalPort: 99999,
			},
		},
	}

	p := profile.Resolve("my-profile", "/path/to/kubeconfig", "", "", false, cfg)

	if p.Argocd == nil {
		t.Fatal("expected Argocd config to be set")
	}
	if p.Argocd.LocalPort != config.DefaultArgocdLocalPort {
		t.Errorf("LocalPort = %d; want %d (default)", p.Argocd.LocalPort, config.DefaultArgocdLocalPort)
	}
}

func TestResolve_ContextPropagated(t *testing.T) {
	cfg := defaultTestConfig()

	p := profile.Resolve("my-profile", "/path/to/kubeconfig", "my-context", "", false, cfg)

	if p.Context != "my-context" {
		t.Errorf("Context = %q; want %q", p.Context, "my-context")
	}
}
