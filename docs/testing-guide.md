# Manual Testing Guide — k10s

## Prerequisites

- Binary built: `make build && ./bin/k10s --help` outputs usage
- kubectl installed and configured
- At least one kubeconfig file available

## Test Environment Setup

```bash
# 1. Build
cd /path/to/k10s && make build

# 2. Create configs directory
mkdir -p ~/.kube/configs

# 3. Copy sample kubeconfigs (fake, for scanner testing)
cp docs/sample-configs/*.yaml ~/.kube/configs/

# 4. Initialize config
./bin/k10s config init
```

---

## Scenario 1: Basic CLI Commands (no cluster required)

### k10s doctor

**Steps:**
1. Run `./bin/k10s doctor`
2. Observe the dependency report

**Expected output:**
```
Dependency check:

  ✓ kubectl       found
  ✓ k9s           found
  ✓ lsof          found
  ✗ kubelogin     not found (optional)
  ✗ argocd        not found (optional)

All required dependencies satisfied.
```
If a required dep is missing, it should offer: `Install <dep> via brew? [y/N]`

### k10s list

**Steps:**
1. Ensure `~/.kube/configs/` contains at least one `.yaml` file
2. Run `./bin/k10s list`

**Expected output:** A table with columns `NAME`, `SERVER`, `ACTION`, `OIDC`.
If no configs are found: `No profiles found.` with the configs directory path.

### k10s add

**Steps:**
1. Run `./bin/k10s add docs/sample-configs/dev-cluster.yaml`

**Expected output:**
```
Added docs/sample-configs/dev-cluster.yaml -> ~/.kube/configs/dev-cluster.yaml
```
Verify: `ls ~/.kube/configs/` includes `dev-cluster.yaml`.

### k10s remove

**Steps:**
1. Run `./bin/k10s remove dev-cluster`

**Expected output:**
```
Removed ~/.kube/configs/dev-cluster.yaml
```
Verify: `ls ~/.kube/configs/` no longer includes `dev-cluster.yaml`.

### k10s config show

**Steps:**
1. Run `./bin/k10s config show`

**Expected output:** Config file path, full YAML config dump, then a list of detected profiles with server URLs and tags.

### k10s config init

**Steps:**
1. Delete `~/.k10s/config.yaml` if it exists: `rm -f ~/.k10s/config.yaml`
2. Run `./bin/k10s config init`

**Expected output:**
```
Created config file: ~/.k10s/config.yaml
```
Verify: `cat ~/.k10s/config.yaml` shows global defaults and any auto-detected profiles.

### k10s config edit

**Steps:**
1. Run `./bin/k10s config edit`

**Expected:** Opens `~/.k10s/config.yaml` in `$EDITOR` (falls back to `vi`).
After saving and closing, the file should contain the edited content.

---

## Scenario 2: TUI Navigation (no cluster required)

**Steps:**
1. Ensure at least one kubeconfig exists in `~/.kube/configs/`
2. Run `./bin/k10s`

**Expected behavior:**

| Key | Expected behavior |
|-----|-------------------|
| Up / Down arrow | Highlight moves between cluster entries |
| `/` | Search filter input appears; typing narrows the list |
| Backspace | Removes last character from search filter |
| Enter | Confirms selection and proceeds to action menu |
| `q` | Quits the TUI and returns to shell |
| Ctrl+C | Exits the TUI cleanly without a panic or stack trace |

If no profiles are found, the TUI should not launch — instead a message like:
```
No kubeconfig profiles found.
Add kubeconfig files to ~/.kube/configs or run 'k10s add <file>'
```

---

## Scenario 3: Config Validation

### Invalid default_action value

**Steps:**
1. Edit `~/.k10s/config.yaml` and set:
   ```yaml
   global:
     default_action: invalid_value
   ```
2. Run `./bin/k10s list`

**Expected:** Clear error message such as `invalid default_action "invalid_value": must be one of select, k9s, argocd`. No panic.

### Missing ArgoCD password

**Steps:**
1. Configure a profile with ArgoCD but no password:
   ```yaml
   profiles:
     my-cluster:
       argocd:
         namespace: argocd
   ```
2. Select that profile in the TUI and choose the ArgoCD action.

**Expected:** Error message: `argocd password not configured; set it in ~/.k10s/config.yaml under profiles.<name>.argocd.password`. Not a silent no-op.

### Invalid port numbers

**Steps:**
1. Set an out-of-range port in config:
   ```yaml
   profiles:
     my-cluster:
       argocd:
         local_port: 99999
   ```
2. Run `./bin/k10s list`

**Expected:** Either a validation error at load time, or the port is silently clamped to a valid default (8080). No crash and no attempt to bind port 99999.

---

## Scenario 4: k9s Integration (requires real cluster)

**Prerequisites:**
- kubectl configured with a valid cluster context
- k9s installed (`k10s doctor` reports it found)
- At least one kubeconfig in `~/.kube/configs/` pointing to a reachable cluster

**Steps:**
1. Run `./bin/k10s`
2. Select a cluster from the list
3. Choose the `k9s` action

**Expected:** k9s launches with `KUBECONFIG` set to the selected file and the correct context. The k9s UI appears. After quitting k9s, control returns to the shell.

---

## Scenario 5: ArgoCD Connection (requires ArgoCD deployment)

**Prerequisites:**
- A running Kubernetes cluster with ArgoCD installed in the `argocd` namespace
- The `argocd` CLI installed (`k10s doctor` reports it found)
- Password configured in `~/.k10s/config.yaml`:
  ```yaml
  profiles:
    my-cluster:
      argocd:
        password: "your-argocd-admin-password"
  ```

**Steps:**
1. Run `./bin/k10s`
2. Select the cluster with ArgoCD configured
3. Choose the `argocd` action
4. Observe the output in the terminal
5. After the browser opens, press Ctrl+C to stop

**Expected:**
- `kubectl port-forward` starts (a line like `Forwarding from 127.0.0.1:8080 -> 443`)
- `argocd login` runs against `localhost:8080` with the configured credentials
- Browser opens to the ArgoCD UI URL
- On Ctrl+C: port-forward process is terminated cleanly, no zombie processes left

---

## Scenario 6: Port Conflict Recovery

### a) Orphan kubectl port-forward (k10s should kill it and reuse port)

**Steps:**
1. Start a stale port-forward manually:
   ```bash
   kubectl port-forward svc/argocd-server -n argocd 8080:443 &
   ORPHAN_PID=$!
   ```
2. Run `./bin/k10s` and select the ArgoCD action for a cluster using port 8080

**Expected:** k10s detects the existing `kubectl` process on port 8080, kills it, then starts a fresh port-forward on port 8080. No error about the port being already in use.

### b) Other process on port (k10s should auto-allocate next port)

**Steps:**
1. Occupy port 8080 with a non-kubectl process:
   ```bash
   python3 -m http.server 8080 &
   HTTP_PID=$!
   ```
2. Run `./bin/k10s` and select the ArgoCD action for a cluster using port 8080

**Expected:** k10s detects the port is occupied by a non-kubectl process, selects the next available port (e.g. 8081), and starts the port-forward on that port. No hang or crash.

**Cleanup:** `kill $HTTP_PID`

---

## Scenario 7: OIDC Authentication

**Prerequisites:**
- A kubeconfig that uses OIDC auth (auth-provider `oidc`, or exec referencing `kubelogin` / `oidc-login`)
- `kubelogin` installed (`k10s doctor` reports it found)

**Steps:**
1. Place the OIDC kubeconfig in `~/.kube/configs/`
2. Run `./bin/k10s list` — verify the `OIDC` column shows `true` for the cluster
3. Run `./bin/k10s` and select that cluster with the `k9s` action

**Expected:** Before launching k9s, k10s runs `kubelogin` to refresh the OIDC token. A browser-based login flow may open. After authentication completes, k9s launches with the refreshed credentials.

---

## Scenario 8: Context Mode

**Prerequisites:**
- A kubeconfig file with multiple contexts (e.g. `docs/sample-configs/prod-cluster.yaml`)

**Steps:**
1. Update `~/.k10s/config.yaml`:
   ```yaml
   global:
     contexts_mode: true
     kubeconfig_file: "~/.kube/config"
   ```
2. Run `./bin/k10s list`

**Expected:** All contexts from the specified kubeconfig file appear as separate entries in the list, each showing its server URL. The `NAME` column uses the context name (e.g. `prod-us`, `prod-eu`).

---

## Pass/Fail Checklist

### Scenario 1: Basic CLI Commands

- [ ] `k10s doctor` prints a dependency table with correct found/not-found status
- [ ] `k10s doctor` offers brew install for missing required deps
- [ ] `k10s list` prints a table when profiles exist
- [ ] `k10s list` prints a helpful message when no profiles exist
- [ ] `k10s add <file>` copies the file to the configs directory
- [ ] `k10s remove <name>` deletes the file from the configs directory
- [ ] `k10s config show` prints config YAML and detected profiles
- [ ] `k10s config init` creates `~/.k10s/config.yaml` with defaults
- [ ] `k10s config init` prompts before overwriting an existing file
- [ ] `k10s config edit` opens the config file in `$EDITOR`

### Scenario 2: TUI Navigation

- [ ] TUI launches when profiles are available
- [ ] Arrow keys move the selection highlight
- [ ] `/` opens search filter
- [ ] Typing in search filter narrows the list
- [ ] Backspace removes characters from the search filter
- [ ] Enter confirms selection
- [ ] `q` exits cleanly
- [ ] Ctrl+C exits cleanly without panic
- [ ] Helpful message shown when no profiles found (TUI does not launch)

### Scenario 3: Config Validation

- [ ] Invalid `default_action` produces a clear error message, not a panic
- [ ] Missing ArgoCD password produces a descriptive error, not a silent no-op
- [ ] Out-of-range port numbers do not cause a crash

### Scenario 4: k9s Integration

- [ ] k9s launches with the correct `KUBECONFIG` environment variable
- [ ] The correct context is selected in k9s
- [ ] Shell is restored after quitting k9s

### Scenario 5: ArgoCD Connection

- [ ] Port-forward starts and shows forwarding message
- [ ] `argocd login` completes successfully
- [ ] Browser opens to the ArgoCD UI
- [ ] Ctrl+C terminates the port-forward cleanly

### Scenario 6: Port Conflict Recovery

- [ ] Orphan kubectl port-forward is killed and the port is reused
- [ ] Non-kubectl process on port causes k10s to select the next available port

### Scenario 7: OIDC Authentication

- [ ] OIDC kubeconfigs show `true` in the `OIDC` column of `k10s list`
- [ ] `kubelogin` is invoked before launching k9s for OIDC profiles
- [ ] k9s launches successfully after OIDC token refresh

### Scenario 8: Context Mode

- [ ] All contexts appear as separate entries in `k10s list`
- [ ] Context names are used as profile names
- [ ] Server URLs are correctly resolved per context

---

## Troubleshooting

### "lsof not found" warning

`lsof` is required for port conflict detection. On Linux, install it:
```bash
apt install lsof       # Debian/Ubuntu
yum install lsof       # RHEL/CentOS
brew install lsof      # macOS (should already be present)
```
Without `lsof`, k10s will skip port conflict checks and may fail if a port is already in use.

### "no kubeconfig profiles found"

k10s looks for kubeconfig files in `~/.kube/configs/` by default. Either:
- Copy your kubeconfig there: `k10s add ~/Downloads/my-cluster.yaml`
- Or change the directory in config: `k10s config edit` → set `global.configs_dir`
- Or enable contexts mode and point to an existing kubeconfig: set `global.contexts_mode: true` and `global.kubeconfig_file`

### ArgoCD password not configured error

Add the password under the correct profile name in `~/.k10s/config.yaml`:
```yaml
profiles:
  <profile-name>:
    argocd:
      password: "your-password"
```
Run `k10s list` to confirm the profile name, then re-run `k10s config edit` to add the password.

### Port-forward timeout

If `kubectl port-forward` hangs or times out:
1. Verify the cluster is reachable: `kubectl cluster-info --kubeconfig <file>`
2. Verify the ArgoCD service exists: `kubectl get svc -n argocd argocd-server`
3. Check for leftover port-forward processes: `lsof -i :<port>` and kill them manually
4. Confirm the `namespace` and `service` fields in the ArgoCD config match the actual deployment
