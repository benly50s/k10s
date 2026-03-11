package profile

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/benly/k10s/internal/config"
	"github.com/benly/k10s/internal/k8s"
)

// Scan returns a list of profiles based on the k10s configuration.
// If contexts_mode is enabled and kubeconfig_file is set, it scans contexts
// from a single kubeconfig. Otherwise it scans yaml files from configs_dir.
func Scan(cfg *config.K10sConfig) ([]Profile, error) {
	if cfg.Global.ContextsMode && cfg.Global.KubeconfigFile != "" {
		return scanFromContexts(cfg)
	}
	return scanFromFiles(cfg)
}

// scanFromFiles scans configs_dir for *.yaml / *.yml kubeconfig files.
// Each file (minus extension) becomes a profile name.
func scanFromFiles(cfg *config.K10sConfig) ([]Profile, error) {
	dir := config.ExpandPath(cfg.Global.ConfigsDir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Profile{}, nil
		}
		return nil, err
	}

	var profiles []Profile

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := filepath.Ext(name)
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		baseName := strings.TrimSuffix(name, ext)

		// Skip k10s config files
		if baseName == "k10s" || baseName == "config" {
			continue
		}

		filePath := filepath.Join(dir, name)

		serverURL, oidcDetected := k8s.ParseKubeconfig(filePath)

		profile := Resolve(baseName, filePath, "", serverURL, oidcDetected, cfg)
		profiles = append(profiles, profile)
	}

	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	return profiles, nil
}

// scanFromContexts reads all contexts from a single kubeconfig file.
// Each context becomes a profile, using the context name as the profile name.
func scanFromContexts(cfg *config.K10sConfig) ([]Profile, error) {
	kubeconfigPath := config.ExpandPath(cfg.Global.KubeconfigFile)

	contexts, err := k8s.ParseKubeconfigContexts(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	var profiles []Profile
	for _, ctx := range contexts {
		profile := Resolve(ctx.Name, kubeconfigPath, ctx.Name, ctx.ServerURL, ctx.OIDC, cfg)
		profiles = append(profiles, profile)
	}

	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	return profiles, nil
}
