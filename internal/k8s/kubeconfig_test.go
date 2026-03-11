package k8s_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/benly/k10s/internal/k8s"
)

func writeKubeconfigYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "kubeconfig-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

// kubeconfigTemplate is a single-context kubeconfig with configurable server and user fields.
// The userSection placeholder must be a valid YAML block under the user: key.
const kubeconfigTemplate = `apiVersion: v1
kind: Config
clusters:
- name: test-cluster
  cluster:
    server: %s
users:
- name: test-user
  user:
    %s
contexts:
- name: test-context
  context:
    cluster: test-cluster
    user: test-user
current-context: test-context
`

func TestParseKubeconfig_ServerURL(t *testing.T) {
	content := fmt.Sprintf(kubeconfigTemplate, "https://k8s.example.com:6443", "{}")
	path := writeKubeconfigYAML(t, content)

	serverURL, oidc := k8s.ParseKubeconfig(path)

	if serverURL != "https://k8s.example.com:6443" {
		t.Errorf("serverURL = %q; want %q", serverURL, "https://k8s.example.com:6443")
	}
	if oidc {
		t.Error("expected oidc=false for plain user")
	}
}

func TestParseKubeconfig_OIDCAuthProvider(t *testing.T) {
	userSection := "auth-provider:\n      name: oidc"
	content := fmt.Sprintf(kubeconfigTemplate, "https://k8s.example.com:6443", userSection)
	path := writeKubeconfigYAML(t, content)

	_, oidc := k8s.ParseKubeconfig(path)

	if !oidc {
		t.Error("expected oidc=true for auth-provider: oidc")
	}
}

func TestParseKubeconfig_AzureAuthProvider(t *testing.T) {
	userSection := "auth-provider:\n      name: azure"
	content := fmt.Sprintf(kubeconfigTemplate, "https://k8s.example.com:6443", userSection)
	path := writeKubeconfigYAML(t, content)

	_, oidc := k8s.ParseKubeconfig(path)

	if !oidc {
		t.Error("expected oidc=true for auth-provider: azure")
	}
}

func TestParseKubeconfig_KubeloginExec(t *testing.T) {
	userSection := "exec:\n      command: /usr/bin/kubelogin\n      args: []"
	content := fmt.Sprintf(kubeconfigTemplate, "https://k8s.example.com:6443", userSection)
	path := writeKubeconfigYAML(t, content)

	_, oidc := k8s.ParseKubeconfig(path)

	if !oidc {
		t.Error("expected oidc=true for exec command containing kubelogin")
	}
}

func TestParseKubeconfig_OIDCLoginExec(t *testing.T) {
	userSection := "exec:\n      command: oidc-login\n      args: []"
	content := fmt.Sprintf(kubeconfigTemplate, "https://k8s.example.com:6443", userSection)
	path := writeKubeconfigYAML(t, content)

	_, oidc := k8s.ParseKubeconfig(path)

	if !oidc {
		t.Error("expected oidc=true for exec command oidc-login")
	}
}

func TestParseKubeconfig_GetTokenArg(t *testing.T) {
	userSection := "exec:\n      command: kubectl\n      args:\n      - oidc-login\n      - get-token"
	content := fmt.Sprintf(kubeconfigTemplate, "https://k8s.example.com:6443", userSection)
	path := writeKubeconfigYAML(t, content)

	_, oidc := k8s.ParseKubeconfig(path)

	if !oidc {
		t.Error("expected oidc=true for exec with get-token arg")
	}
}

func TestParseKubeconfig_NonOIDC(t *testing.T) {
	// aws eks token-command does not use any oidc/kubelogin/get-token/azure keywords
	userSection := "exec:\n      command: aws\n      args:\n      - eks\n      - --region\n      - us-east-1"
	content := fmt.Sprintf(kubeconfigTemplate, "https://k8s.example.com:6443", userSection)
	path := writeKubeconfigYAML(t, content)

	_, oidc := k8s.ParseKubeconfig(path)

	if oidc {
		t.Error("expected oidc=false for aws exec with non-oidc args")
	}
}

func TestParseKubeconfig_NonexistentFile(t *testing.T) {
	serverURL, oidc := k8s.ParseKubeconfig("/nonexistent/file.yaml")

	if serverURL != "" {
		t.Errorf("serverURL = %q; want empty string", serverURL)
	}
	if oidc {
		t.Error("expected oidc=false for nonexistent file")
	}
}

func TestParseKubeconfigContexts_TwoContexts(t *testing.T) {
	content := `apiVersion: v1
kind: Config
clusters:
- name: cluster-a
  cluster:
    server: https://cluster-a.example.com:6443
- name: cluster-b
  cluster:
    server: https://cluster-b.example.com:6443
users:
- name: user-a
  user: {}
- name: user-b
  user: {}
contexts:
- name: ctx-a
  context:
    cluster: cluster-a
    user: user-a
- name: ctx-b
  context:
    cluster: cluster-b
    user: user-b
current-context: ctx-a
`
	path := writeKubeconfigYAML(t, content)

	contexts, err := k8s.ParseKubeconfigContexts(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contexts) != 2 {
		t.Fatalf("expected 2 contexts, got %d", len(contexts))
	}

	byName := make(map[string]k8s.ContextInfo)
	for _, c := range contexts {
		byName[c.Name] = c
	}

	ctxA, ok := byName["ctx-a"]
	if !ok {
		t.Fatal("ctx-a not found")
	}
	if ctxA.ServerURL != "https://cluster-a.example.com:6443" {
		t.Errorf("ctx-a ServerURL = %q; want %q", ctxA.ServerURL, "https://cluster-a.example.com:6443")
	}

	ctxB, ok := byName["ctx-b"]
	if !ok {
		t.Fatal("ctx-b not found")
	}
	if ctxB.ServerURL != "https://cluster-b.example.com:6443" {
		t.Errorf("ctx-b ServerURL = %q; want %q", ctxB.ServerURL, "https://cluster-b.example.com:6443")
	}
}

func TestParseKubeconfigContexts_OIDCPerContext(t *testing.T) {
	content := `apiVersion: v1
kind: Config
clusters:
- name: cluster-a
  cluster:
    server: https://cluster-a.example.com:6443
- name: cluster-b
  cluster:
    server: https://cluster-b.example.com:6443
users:
- name: oidc-user
  user:
    auth-provider:
      name: oidc
- name: plain-user
  user: {}
contexts:
- name: ctx-oidc
  context:
    cluster: cluster-a
    user: oidc-user
- name: ctx-plain
  context:
    cluster: cluster-b
    user: plain-user
current-context: ctx-oidc
`
	path := writeKubeconfigYAML(t, content)

	contexts, err := k8s.ParseKubeconfigContexts(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contexts) != 2 {
		t.Fatalf("expected 2 contexts, got %d", len(contexts))
	}

	byName := make(map[string]k8s.ContextInfo)
	for _, c := range contexts {
		byName[c.Name] = c
	}

	if !byName["ctx-oidc"].OIDC {
		t.Error("expected ctx-oidc OIDC=true")
	}
	if byName["ctx-plain"].OIDC {
		t.Error("expected ctx-plain OIDC=false")
	}
}

func TestParseKubeconfigContexts_EmptyFile(t *testing.T) {
	path := writeKubeconfigYAML(t, "{}")

	contexts, err := k8s.ParseKubeconfigContexts(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contexts) != 0 {
		t.Errorf("expected 0 contexts, got %d", len(contexts))
	}
}
