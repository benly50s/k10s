# k10s Roadmap — Bug Fix Plan

## Summary Table

| Priority | File | Problem | Fix Description | Effort |
|----------|------|---------|----------------|--------|
| P0 | `internal/config/types.go` | `ArgocdConfig` missing `Password` field — login always silently skipped | Add `Password string` yaml field to struct | XS |
| P0 | `internal/config/loader.go` | No `default_action` value validation — invalid values silently accepted | Add `Validate()` function; call it in `Load()` after unmarshal | S |
| P0 | `internal/argocd/connect.go` | `Login` called with `""` hardcoded for password — field never used | Pass `argocdCfg.Password` instead of `""` | XS |
| P0 | `internal/argocd/login.go` | Empty password prints message and returns `nil` — error silently lost | Return descriptive `fmt.Errorf` when password is empty | XS |
| P1 | `internal/deps/checker.go` | `lsof` not in deps list; `kubectl version --short` deprecated in 1.27+ | Add `lsof` dep entry; remove version detection (flags vary per tool) | S |
| P1 | `internal/k8s/portforward.go` | No guard when `lsof` is unavailable — silent failure on systems without it | Add `lsofAvailable()` helper; guard `IsPortInUse` and `GetPIDsOnPort` | S |
| P1 | `internal/k8s/kubeconfig.go` | Incomplete OIDC detection — misses `oidc-login`, `get-token`, `azure` exec/auth patterns | Extend `isOIDCUser` to match `oidc-login` command, `get-token`/`azure` args, azure auth-provider | S |
| P1 | `internal/tui/app.go` | `StateError` is dead code — `err` field never set in `Run()` | After `p.Run()` returns error, create `AppModel{state: StateError, err: err}` and render it | S |
| P2 | `internal/argocd/browser.go` | macOS-only — no `xdg-open` for Linux or `start` for Windows | Switch on `runtime.GOOS` to select the correct open command per OS | S |
| P2 | `internal/profile/resolver.go` | Port values are not range-validated — out-of-range ports silently used | After merging, clamp `LocalPort`/`RemotePort` to 1–65535; fall back to defaults | XS |

Effort key: XS = < 30 min, S = 30–60 min, M = 1–3 h, L = 3+ h

---

## P0 — Critical Bugs (login always broken)

These four bugs collectively mean ArgoCD login **never works** in any configuration.

### 1. Missing Password field (`internal/config/types.go`)

`ArgocdConfig` has no `Password` field, so any password set in `config.yaml` is silently discarded at parse time. Even if the field were passed correctly downstream, there is nothing to pass.

**Fix:** Add `Password string \`yaml:"password,omitempty"\`` to `ArgocdConfig`.

### 2. No default_action validation (`internal/config/loader.go`)

`Load()` applies defaults for empty fields but never validates that `default_action` values are one of the permitted strings (`select`, `k9s`, `argocd`). An invalid value propagates silently and causes unexpected behaviour in the TUI.

**Fix:** Add a `Validate(*K10sConfig) error` function that rejects unknown `default_action` values in both global config and per-profile config. Call it in `Load()` after `yaml.Unmarshal`.

### 3. Hardcoded empty password (`internal/argocd/connect.go`)

`Login` is called as `Login(localPort, argocdCfg.Username, "", argocdCfg.Insecure)`. The password is always the empty string literal, so even after fixing the struct field the value would never reach `argocd login`.

**Fix:** Change the call to `Login(localPort, argocdCfg.Username, argocdCfg.Password, argocdCfg.Insecure)`.

### 4. Empty password swallows error (`internal/argocd/login.go`)

When `password == ""`, the function prints a message and returns `nil`. This masks the misconfiguration as a non-error, making debugging very difficult.

**Fix:** Return `fmt.Errorf("argocd password not configured; set it in ~/.k10s/config.yaml under profiles.<name>.argocd.password")`.

---

## P1 — High Priority Bugs

### 5. Missing lsof dep + deprecated kubectl flag (`internal/deps/checker.go`)

`lsof` is used directly by the port-forward subsystem but is not listed as a dependency, so `doctor` never warns users it is missing. Additionally, `kubectl version --short` was deprecated in Kubernetes 1.27 and removed in later releases, causing version detection to fail for all tools.

**Fix:** Add `{Name: "lsof", Brew: "lsof", Required: true}` between `k9s` and `kubelogin`. Remove the per-tool version detection loop (set `Version` to `""` unconditionally) since tool-specific version flags vary and this is display-only information.

### 6. No lsof guard in portforward (`internal/k8s/portforward.go`)

`GetPIDsOnPort` runs `exec.Command("lsof", ...)` unconditionally. On systems where `lsof` is absent (some Linux distros, containers) this silently returns an empty result, meaning port conflicts are never detected.

**Fix:** Add `lsofAvailable() bool` using `exec.LookPath`. Guard the start of `IsPortInUse` with a stderr warning and early `false` return. Guard the start of `GetPIDsOnPort` with an early `nil` return.

### 7. Incomplete OIDC detection (`internal/k8s/kubeconfig.go`)

`isOIDCUser` only checks for auth-provider name `oidc` (case-insensitive) and exec command/args containing `kubelogin` or `oidc`. It misses:
- exec command containing `oidc-login` (used by `int128/kubelogin` plugin)
- exec args containing `get-token` (Azure AD kubelogin subcommand)
- exec args containing `azure`
- auth-provider name `azure`

**Fix:** Extend the condition set in `isOIDCUser` to cover all four additional patterns.

### 8. StateError is dead code (`internal/tui/app.go`)

`StateError` is defined and handled in `View()` but `err` is never set in the `Run()` function — if `p.Run()` returns an error, the code returns immediately without constructing an error model. The `View()` branch for `StateError` is therefore unreachable.

**Fix:** After `finalModel, err := p.Run()`, when `err != nil`, create `AppModel{state: StateError, err: err}`, render it to `os.Stderr` via `fmt.Fprintln`, then return the error. Add `"os"` to imports.

---

## P2 — Medium Priority

### 9. macOS-only browser open (`internal/argocd/browser.go`)

`OpenBrowser` hardcodes `exec.Command("open", url)` which only works on macOS. Running k10s on Linux or Windows silently fails (or is not possible at all) when it tries to open the ArgoCD UI.

**Fix:** Switch on `runtime.GOOS`: `"darwin"` → `open`, `"linux"` → `xdg-open`, `"windows"` → `cmd /c start`. Return an error for unsupported platforms.

### 10. Port values not range-validated (`internal/profile/resolver.go`)

After merging profile config with defaults, `LocalPort` and `RemotePort` are used without checking they are in the valid TCP range (1–65535). A misconfigured value such as `0` or `99999` would be passed to `kubectl port-forward` and fail at runtime with an opaque error.

**Fix:** After the merge block, clamp both ports to 1–65535, falling back to the built-in defaults if out of range.

---

## Test Infrastructure

Current state: **0% test coverage across all 28 source files.** No test files exist anywhere in the repository.

### Recommended test targets (ordered by return on investment)

| Package | Key tests needed |
|---------|-----------------|
| `internal/config` | `Load()` with valid/invalid YAML; `Validate()` with all invalid `default_action` values |
| `internal/config` | `Validate()` accepts all valid values; rejects unknown values in global and per-profile |
| `internal/k8s` | `isOIDCUser` table-driven tests for all OIDC patterns (kubelogin, oidc-login, azure, get-token) |
| `internal/profile` | `Resolve()` merging priority; port clamping; defaults propagation |
| `internal/deps` | `Check()` with mocked PATH |
| `internal/argocd` | `Login()` returns error on empty password |
| `internal/tui` | `AppModel.View()` for each state |

### Suggested approach

1. Add `testdata/` fixtures for kubeconfig and k10s config YAML files.
2. Use `t.TempDir()` for config file tests to avoid touching `~/.k10s`.
3. Use `t.Setenv("PATH", ...)` for dep-checker tests.
4. Target 80% coverage on `config`, `profile`, and `k8s` packages first — these contain the most logic.

---

## Future

- **Encrypted password storage:** Store `argocd.password` via OS keychain (macOS Keychain, Linux Secret Service) rather than plain text in `config.yaml`.
- **Multiple kubeconfig file support:** Accept a list of kubeconfig paths in `global.kubeconfig_file` and merge contexts.
- **Plugin system:** Allow external scripts/binaries to define custom actions beyond k9s and ArgoCD.
- **TUI theming:** Expose colour scheme configuration in `global` config.
- **Shell completions:** Generate completions for bash, zsh, and fish via cobra/completions.
- **Config schema validation:** Publish a JSON Schema for `config.yaml` to enable editor autocompletion and validation.
- **Windows support:** Full first-class Windows support (browser open is the first step; also path handling, SysProcAttr).
