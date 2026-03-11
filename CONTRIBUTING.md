# Contributing to k10s

## Development Environment

**Required:**
- Go 1.21+
- kubectl
- k9s
- lsof (ships with macOS; install on Linux: `apt install lsof`)
- make

**Optional (for full feature testing):**
- kubelogin — OIDC authentication
- argocd CLI — ArgoCD login flow
- Access to a Kubernetes cluster

## Building

```bash
make build          # ./bin/k10s
make install        # /usr/local/bin/k10s (requires write permission)
make clean          # remove ./bin/
go build -o ./bin/k10s .   # same as make build
```

## Running Tests

```bash
go test ./...                           # all unit tests
go test ./internal/config/... -v        # config package tests
go test ./internal/profile/... -v       # profile package tests
go test ./internal/k8s/... -v           # k8s package tests
go test ./internal/deps/... -v          # deps package tests

# Integration tests (require live tools):
go test -tags integration ./...
```

## Code Quality

```bash
go vet ./...        # static analysis (run before every commit)
```

## Project Layout

```
k10s/
├── main.go              Entry point
├── cmd/                 cobra CLI commands
│   ├── root.go          Default command → TUI
│   ├── list.go          k10s list
│   ├── add.go           k10s add <file>
│   ├── remove.go        k10s remove <name>
│   ├── doctor.go        k10s doctor
│   └── config.go        k10s config init/show/edit
└── internal/
    ├── config/          Config file types, loader, defaults
    ├── profile/         Profile scanning (file-based & context-based)
    ├── tui/             Bubble Tea TUI (state machine)
    ├── k8s/             kubeconfig parsing, port-forward management
    ├── argocd/          ArgoCD connection orchestration
    ├── auth/            OIDC refresh via kubelogin
    ├── executor/        k9s process launcher (syscall.Exec)
    └── deps/            Dependency checker (brew install)
```

## Adding a New Feature

1. Identify the relevant package in `internal/`
2. Write tests first (see existing `*_test.go` files for patterns)
3. Implement the feature
4. Run `go vet ./...` and `go test ./...`
5. Update `README.md` if the feature changes user-facing behavior

## Config Schema Reference

See `internal/config/types.go` for the full schema.
Key types: `K10sConfig`, `GlobalConfig`, `ProfileConfig`, `ArgocdConfig`.
