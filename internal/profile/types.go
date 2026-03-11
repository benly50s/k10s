package profile

import "github.com/benly/k10s/internal/config"

// Profile represents a resolved Kubernetes cluster profile
type Profile struct {
	Name          string
	FilePath      string // path to kubeconfig file
	Context       string // kubectl context name (empty = file-based profile)
	ServerURL     string
	DefaultAction string
	OIDC          bool
	Argocd        *config.ArgocdConfig
}
