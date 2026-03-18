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
}

func TestResolve_ProfileOverridesGlobal(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Global.DefaultAction = "k9s"
	cfg.Profiles = map[string]config.ProfileConfig{
		"my-profile": {DefaultAction: "select"},
	}

	p := profile.Resolve("my-profile", "/path/to/kubeconfig", "", "", false, cfg)

	if p.DefaultAction != "select" {
		t.Errorf("DefaultAction = %q; want %q", p.DefaultAction, "select")
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

	p := profile.Resolve("my-profile", "/path/to/kubeconfig", "", "", false, cfg)

	if !p.OIDC {
		t.Error("expected OIDC=true from profile config override")
	}
}

func TestResolve_ContextPropagated(t *testing.T) {
	cfg := defaultTestConfig()

	p := profile.Resolve("my-profile", "/path/to/kubeconfig", "my-context", "", false, cfg)

	if p.Context != "my-context" {
		t.Errorf("Context = %q; want %q", p.Context, "my-context")
	}
}
