package k8s

import (
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// Scanner checks directories for valid Kubeconfig files
func ScanForKubeconfigs(dirPath string) ([]string, error) {
	var results []string

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		if IsValidKubeconfig(fullPath) {
			results = append(results, fullPath)
		}
	}

	return results, nil
}

// IsValidKubeconfig peeks into the file to see if it looks like a kubeconfig
func IsValidKubeconfig(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	// Clean up non-breaking spaces (U+00a0) that might break yaml parsing
	cleanStr := strings.ReplaceAll(string(data), "\u00a0", " ")

	var metadata struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
	}

	if err := yaml.Unmarshal([]byte(cleanStr), &metadata); err != nil {
		return false
	}

	return metadata.APIVersion == "v1" && metadata.Kind == "Config"
}
