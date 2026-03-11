package k8s

import (
	"os"
	"strings"

	"sigs.k8s.io/yaml"
)

// kubeconfigCluster represents a cluster entry in kubeconfig
type kubeconfigCluster struct {
	Name    string `yaml:"name"    json:"name"`
	Cluster struct {
		Server string `yaml:"server" json:"server"`
	} `yaml:"cluster" json:"cluster"`
}

// kubeconfigUser represents a user entry in kubeconfig
type kubeconfigUser struct {
	Name string `yaml:"name" json:"name"`
	User struct {
		AuthProvider *struct {
			Name   string            `yaml:"name"   json:"name"`
			Config map[string]string `yaml:"config" json:"config"`
		} `yaml:"auth-provider" json:"auth-provider"`
		Exec *struct {
			Command string   `yaml:"command" json:"command"`
			Args    []string `yaml:"args"    json:"args"`
		} `yaml:"exec" json:"exec"`
	} `yaml:"user" json:"user"`
}

// kubeconfigContext represents a context entry in kubeconfig
type kubeconfigContext struct {
	Name    string `yaml:"name" json:"name"`
	Context struct {
		Cluster   string `yaml:"cluster"   json:"cluster"`
		User      string `yaml:"user"       json:"user"`
		Namespace string `yaml:"namespace"  json:"namespace"`
	} `yaml:"context" json:"context"`
}

// kubeconfig is a minimal representation of a kubeconfig file
type kubeconfig struct {
	Clusters       []kubeconfigCluster `yaml:"clusters"        json:"clusters"`
	Users          []kubeconfigUser    `yaml:"users"           json:"users"`
	Contexts       []kubeconfigContext `yaml:"contexts"        json:"contexts"`
	CurrentContext string              `yaml:"current-context" json:"current-context"`
}

// ContextInfo holds info extracted from a kubeconfig context
type ContextInfo struct {
	Name      string
	ServerURL string
	OIDC      bool
}

// ParseKubeconfig parses a kubeconfig file and returns the first cluster server URL
// and whether OIDC authentication is configured.
func ParseKubeconfig(filePath string) (serverURL string, oidc bool) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", false
	}

	var kc kubeconfig
	if err := yaml.Unmarshal(data, &kc); err != nil {
		return "", false
	}

	if len(kc.Clusters) > 0 {
		serverURL = kc.Clusters[0].Cluster.Server
	}

	for _, user := range kc.Users {
		if isOIDCUser(user) {
			oidc = true
			break
		}
	}

	return serverURL, oidc
}

// ParseKubeconfigContexts parses all contexts from a kubeconfig file.
// Returns a list of ContextInfo for each context.
func ParseKubeconfigContexts(filePath string) ([]ContextInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var kc kubeconfig
	if err := yaml.Unmarshal(data, &kc); err != nil {
		return nil, err
	}

	// Build lookup maps
	clusterByName := make(map[string]string) // name -> server URL
	for _, c := range kc.Clusters {
		clusterByName[c.Name] = c.Cluster.Server
	}

	userByName := make(map[string]kubeconfigUser) // name -> user
	for _, u := range kc.Users {
		userByName[u.Name] = u
	}

	var contexts []ContextInfo
	for _, ctx := range kc.Contexts {
		info := ContextInfo{
			Name:      ctx.Name,
			ServerURL: clusterByName[ctx.Context.Cluster],
		}
		if user, ok := userByName[ctx.Context.User]; ok {
			info.OIDC = isOIDCUser(user)
		}
		contexts = append(contexts, info)
	}

	return contexts, nil
}

// isOIDCUser checks whether a kubeconfig user entry uses OIDC authentication
func isOIDCUser(user kubeconfigUser) bool {
	if user.User.AuthProvider != nil {
		name := strings.ToLower(user.User.AuthProvider.Name)
		if name == "oidc" || name == "azure" {
			return true
		}
	}
	if user.User.Exec != nil {
		cmd := user.User.Exec.Command
		if strings.Contains(cmd, "kubelogin") || strings.Contains(cmd, "oidc-login") {
			return true
		}
		for _, arg := range user.User.Exec.Args {
			if strings.Contains(arg, "oidc") || strings.Contains(arg, "kubelogin") ||
				strings.Contains(arg, "get-token") || strings.Contains(arg, "azure") {
				return true
			}
		}
	}
	return false
}
