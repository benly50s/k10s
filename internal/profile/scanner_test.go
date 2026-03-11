package profile_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/benly/k10s/internal/config"
	"github.com/benly/k10s/internal/profile"
)

func writeKubeconfig(t *testing.T, dir, name, server string) string {
	t.Helper()
	content := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: cluster-1
  cluster:
    server: %s
users:
- name: user-1
  user: {}
contexts:
- name: ctx-1
  context:
    cluster: cluster-1
    user: user-1
current-context: ctx-1
`, server)
	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestScanFromFiles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := config.DefaultK10sConfig()
	cfg.Global.ConfigsDir = tmpDir

	profiles, err := profile.Scan(&cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(profiles))
	}
}

func TestScanFromFiles_BasicScan(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files that should be scanned
	writeKubeconfig(t, tmpDir, "alpha", "https://alpha.example.com")
	writeKubeconfig(t, tmpDir, "beta", "https://beta.example.com")

	// Create files that should be skipped
	writeKubeconfig(t, tmpDir, "config", "https://config.example.com")
	writeKubeconfig(t, tmpDir, "k10s", "https://k10s.example.com")

	cfg := config.DefaultK10sConfig()
	cfg.Global.ConfigsDir = tmpDir

	profiles, err := profile.Scan(&cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}
	// Verify alphabetical order
	if profiles[0].Name != "alpha" {
		t.Errorf("profiles[0].Name = %q; want %q", profiles[0].Name, "alpha")
	}
	if profiles[1].Name != "beta" {
		t.Errorf("profiles[1].Name = %q; want %q", profiles[1].Name, "beta")
	}
}

func TestScanFromFiles_NonexistentDir(t *testing.T) {
	cfg := config.DefaultK10sConfig()
	cfg.Global.ConfigsDir = "/nonexistent/path/xyz"

	profiles, err := profile.Scan(&cfg)
	if err != nil {
		t.Fatalf("unexpected error for nonexistent dir: %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(profiles))
	}
}

func TestScanFromFiles_SkipsDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory named with .yaml extension
	subdir := filepath.Join(tmpDir, "subdir.yaml")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create one real profile so we can verify the dir was scanned
	writeKubeconfig(t, tmpDir, "realprofile", "https://real.example.com")

	cfg := config.DefaultK10sConfig()
	cfg.Global.ConfigsDir = tmpDir

	profiles, err := profile.Scan(&cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, p := range profiles {
		if p.Name == "subdir" {
			t.Error("subdir.yaml directory should not appear as a profile")
		}
	}
}

func TestScanFromContexts_MultiContext(t *testing.T) {
	tmpDir := t.TempDir()

	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- name: dev-cluster
  cluster:
    server: https://dev.example.com:6443
- name: prd-cluster
  cluster:
    server: https://prd.example.com:6443
users:
- name: dev-user
  user: {}
- name: prd-user
  user: {}
contexts:
- name: dev-ctx
  context:
    cluster: dev-cluster
    user: dev-user
- name: prd-ctx
  context:
    cluster: prd-cluster
    user: prd-user
current-context: dev-ctx
`
	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig.yaml")
	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultK10sConfig()
	cfg.Global.ContextsMode = true
	cfg.Global.KubeconfigFile = kubeconfigPath

	profiles, err := profile.Scan(&cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}

	// Profiles should be sorted alphabetically
	if profiles[0].Name != "dev-ctx" {
		t.Errorf("profiles[0].Name = %q; want %q", profiles[0].Name, "dev-ctx")
	}
	if profiles[1].Name != "prd-ctx" {
		t.Errorf("profiles[1].Name = %q; want %q", profiles[1].Name, "prd-ctx")
	}

	// Verify ServerURL for dev-ctx
	if profiles[0].ServerURL != "https://dev.example.com:6443" {
		t.Errorf("dev-ctx ServerURL = %q; want %q", profiles[0].ServerURL, "https://dev.example.com:6443")
	}
}

func TestScanFromContexts_NonexistentFile(t *testing.T) {
	cfg := config.DefaultK10sConfig()
	cfg.Global.ContextsMode = true
	cfg.Global.KubeconfigFile = "/nonexistent/file.yaml"

	_, err := profile.Scan(&cfg)
	if err == nil {
		t.Fatal("expected error for nonexistent kubeconfig file, got nil")
	}
}
