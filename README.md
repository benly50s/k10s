# k10s — English Quick Reference

> For full documentation see the Korean sections below.

## Build & Install

```bash
# Prerequisites: Go 1.21+, make
make build    # → ./bin/k10s
make install  # → /usr/local/bin/k10s

# Verify
./bin/k10s --help
./bin/k10s doctor
```

## Quick Start

1. **Check dependencies**: `k10s doctor` (installs missing kubectl/k9s via brew)
2. **Add a kubeconfig**: `k10s add ~/Downloads/my-cluster.yaml`
3. **List clusters**: `k10s list`
4. **Launch TUI**: `k10s` → arrow keys to select, Enter to confirm, `/` to search
5. **Setup config**: `k10s config init` then `k10s config edit`

## ArgoCD Password

To enable ArgoCD login, add the password to `~/.k10s/config.yaml`:

```yaml
profiles:
  my-cluster:
    argocd:
      password: "your-argocd-password"
```

## Context Mode (single kubeconfig, multiple contexts)

```yaml
# ~/.k10s/config.yaml
global:
  contexts_mode: true
  kubeconfig_file: "~/.kube/config"
```

---

