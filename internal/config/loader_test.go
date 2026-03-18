package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benly/k10s/internal/config"
)

func writeConfigFile(t *testing.T, dir, content string) {
	t.Helper()
	cfgDir := filepath.Join(dir, ".k10s")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Global.ConfigsDir != config.DefaultConfigsDir {
		t.Errorf("ConfigsDir = %q; want %q", cfg.Global.ConfigsDir, config.DefaultConfigsDir)
	}
	if cfg.Global.DefaultAction != "select" {
		t.Errorf("DefaultAction = %q; want %q", cfg.Global.DefaultAction, "select")
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	content := `
global:
  configs_dir: "/tmp/test-configs"
  default_action: "k9s"
profiles:
  my-cluster:
    oidc: true
    default_action: "select"
`
	writeConfigFile(t, tmpDir, content)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Global.ConfigsDir != "/tmp/test-configs" {
		t.Errorf("ConfigsDir = %q; want %q", cfg.Global.ConfigsDir, "/tmp/test-configs")
	}
	if cfg.Global.DefaultAction != "k9s" {
		t.Errorf("DefaultAction = %q; want %q", cfg.Global.DefaultAction, "k9s")
	}
	p, ok := cfg.Profiles["my-cluster"]
	if !ok {
		t.Fatal("profile 'my-cluster' not found")
	}
	if !p.OIDC {
		t.Error("expected my-cluster OIDC=true")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	writeConfigFile(t, tmpDir, "key: [invalid yaml")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoad_InvalidGlobalDefaultAction(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	writeConfigFile(t, tmpDir, `
global:
  default_action: "invalid"
`)

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid global default_action")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error %q should contain 'invalid'", err.Error())
	}
}

func TestLoad_InvalidProfileDefaultAction(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	writeConfigFile(t, tmpDir, `
profiles:
  my-cluster:
    default_action: "bad"
`)

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid profile default_action")
	}
}

func TestLoad_ValidActions(t *testing.T) {
	validActions := []string{"", "select", "k9s"}

	for _, action := range validActions {
		action := action
		t.Run("action="+action, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("HOME", tmpDir)

			var content string
			if action == "" {
				content = "global: {}\n"
			} else {
				content = "global:\n  default_action: " + action + "\n"
			}
			writeConfigFile(t, tmpDir, content)

			_, err := config.Load()
			if err != nil {
				t.Errorf("action=%q: unexpected error: %v", action, err)
			}
		})
	}
}

func TestSave_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	original := &config.K10sConfig{
		Global: config.GlobalConfig{
			ConfigsDir:    "/custom/configs",
			DefaultAction: "k9s",
		},
		Profiles: map[string]config.ProfileConfig{
			"test-profile": {
				DefaultAction: "select",
				OIDC:          true,
			},
		},
	}

	if err := config.Save(original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load after Save failed: %v", err)
	}

	if loaded.Global.ConfigsDir != original.Global.ConfigsDir {
		t.Errorf("ConfigsDir: got %q, want %q", loaded.Global.ConfigsDir, original.Global.ConfigsDir)
	}
	if loaded.Global.DefaultAction != original.Global.DefaultAction {
		t.Errorf("DefaultAction: got %q, want %q", loaded.Global.DefaultAction, original.Global.DefaultAction)
	}
	p, ok := loaded.Profiles["test-profile"]
	if !ok {
		t.Fatal("profile 'test-profile' not found after round-trip")
	}
	if p.DefaultAction != "select" {
		t.Errorf("profile DefaultAction: got %q, want %q", p.DefaultAction, "select")
	}
	if !p.OIDC {
		t.Error("profile OIDC: got false, want true")
	}
}

func TestConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	got := config.ConfigDir()
	want := filepath.Join(tmpDir, ".k10s")
	if got != want {
		t.Errorf("ConfigDir() = %q; want %q", got, want)
	}
}

func TestExpandPath_Tilde(t *testing.T) {
	t.Setenv("HOME", "/tmp/testhome")

	got := config.ExpandPath("~/foo")
	want := "/tmp/testhome/foo"
	if got != want {
		t.Errorf("ExpandPath(~/foo) = %q; want %q", got, want)
	}
}

func TestExpandPath_Absolute(t *testing.T) {
	got := config.ExpandPath("/abs/path")
	if got != "/abs/path" {
		t.Errorf("ExpandPath(/abs/path) = %q; want %q", got, "/abs/path")
	}
}
