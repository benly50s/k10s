# k10s — Kubernetes Cluster Manager

A terminal UI for managing multiple Kubernetes clusters. Select a cluster, pick an action (k9s or shell), choose a namespace, and go.

---

## Installation

### Homebrew (Recommended)

```bash
brew install benly50s/tap/k10s
```

### Go Install

```bash
go install github.com/benly50s/k10s@latest
```

### Manual Build

```bash
# Prerequisites: Go 1.21+, make
make build    # → ./bin/k10s
make install  # → /usr/local/bin/k10s
```

---

## Quick Start

1. **Check dependencies**: `k10s doctor` — verifies kubectl, k9s are installed (installs via brew if missing)
2. **Add a kubeconfig**: `k10s add ~/Downloads/my-cluster.yaml`
3. **List registered clusters**: `k10s list`
4. **Launch TUI**: `k10s` — select cluster → select action → select namespace → launch
5. **Edit config**: `k10s config init` then `k10s config edit`

---

## Commands

| Command | Description |
|---|---|
| `k10s` | Launch the TUI |
| `k10s add <file>` | Register a kubeconfig file |
| `k10s remove <name>` | Unregister a kubeconfig |
| `k10s list` | List all registered clusters |
| `k10s doctor` | Check dependencies and auto-install missing tools |
| `k10s onboard` | Bulk-import kubeconfigs from a directory |
| `k10s config init` | Create `~/.k10s/config.yaml` with defaults |
| `k10s config show` | Print the current config |
| `k10s config edit` | Open config in `$EDITOR` |

---

## TUI Keyboard Shortcuts

| Key | Action |
|---|---|
| `↑` / `↓` or `j` / `k` | Move cursor |
| `Enter` | Confirm selection |
| `/` | Filter / search |
| `1`–`2` | Direct action selection (on action screen) |
| `Ctrl+D` | Delete a profile (on cluster list) |
| `Esc` / `←` | Go back |
| `q` | Quit |

---

## Configuration (`~/.k10s/config.yaml`)

```yaml
global:
  configs_dir: "~/.kube/configs"       # Directory scanned for kubeconfig files
  default_action: "select"             # "select" (show action menu) or "k9s" (skip straight to namespace select)
  contexts_mode: false                 # true: read multiple contexts from a single kubeconfig
  kubeconfig_file: "~/.kube/config"   # Used when contexts_mode is true

profiles:
  my-cluster:
    default_action: "k9s"             # Override global default for this cluster
    oidc: true                        # Force OIDC badge (auto-detected otherwise)
```

### Field reference

| Field | Default | Description |
|---|---|---|
| `global.configs_dir` | `~/.kube/configs` | Directory k10s scans for kubeconfig files |
| `global.default_action` | `select` | `select` shows action menu; `k9s` skips to namespace select |
| `global.contexts_mode` | `false` | Read all contexts from a single file instead of one-file-per-cluster |
| `global.kubeconfig_file` | _(empty)_ | Kubeconfig used when `contexts_mode: true` |
| `profiles.<name>.default_action` | _(inherits global)_ | Per-cluster action override |
| `profiles.<name>.oidc` | `false` | Mark cluster as OIDC (badge in list, kubelogin triggered) |

---

## Contexts Mode

If all your clusters live in a single kubeconfig with multiple contexts, use Contexts Mode instead of managing one file per cluster:

```yaml
global:
  contexts_mode: true
  kubeconfig_file: "~/.kube/config"
```

k10s will enumerate every context in that file and present each as a selectable cluster.

---

## OIDC

k10s auto-detects OIDC if the kubeconfig user entry references `oidc` or `kubelogin`. Clusters with OIDC are marked with a badge in the list.

When a profile has `oidc: true` (or is auto-detected), k10s expects [`kubelogin`](https://github.com/int128/kubelogin) to be available and will trigger token refresh before launching k9s or the shell.

---

## Dependencies

| Tool | Required | Notes |
|---|---|---|
| `kubectl` | Yes | Used for namespace listing and context switching |
| `k9s` | Yes | Launched by the `k9s` action |
| `kubelogin` | No | Required only for OIDC clusters |

Run `k10s doctor` to check and auto-install missing required tools via Homebrew.

---

---

# k10s — 쿠버네티스 클러스터 매니저

여러 쿠버네티스 클러스터를 관리하는 터미널 UI 도구입니다. 클러스터를 선택하고, 액션(k9s 또는 쉘)을 선택하고, 네임스페이스를 선택한 뒤 바로 시작할 수 있습니다.

---

## 설치

### Homebrew (권장)

```bash
brew install benly50s/tap/k10s
```

### Go Install

```bash
go install github.com/benly50s/k10s@latest
```

### 수동 빌드

```bash
# 필요 조건: Go 1.21+, make
make build    # → ./bin/k10s
make install  # → /usr/local/bin/k10s
```

---

## 빠른 시작

1. **의존성 확인**: `k10s doctor` — kubectl, k9s 설치 여부 확인 (미설치 시 brew로 자동 설치)
2. **kubeconfig 추가**: `k10s add ~/Downloads/my-cluster.yaml`
3. **등록된 클러스터 목록**: `k10s list`
4. **TUI 실행**: `k10s` — 클러스터 선택 → 액션 선택 → 네임스페이스 선택 → 실행
5. **설정 편집**: `k10s config init` 후 `k10s config edit`

---

## 명령어

| 명령어 | 설명 |
|---|---|
| `k10s` | TUI 실행 |
| `k10s add <file>` | kubeconfig 파일 등록 |
| `k10s remove <name>` | kubeconfig 등록 해제 |
| `k10s list` | 등록된 클러스터 목록 출력 |
| `k10s doctor` | 의존성 체크 및 미설치 도구 자동 설치 |
| `k10s onboard` | 디렉토리에서 kubeconfig 일괄 가져오기 |
| `k10s config init` | `~/.k10s/config.yaml` 기본값으로 생성 |
| `k10s config show` | 현재 설정 출력 |
| `k10s config edit` | `$EDITOR`로 설정 파일 열기 |

---

## TUI 키보드 단축키

| 키 | 동작 |
|---|---|
| `↑` / `↓` 또는 `j` / `k` | 커서 이동 |
| `Enter` | 선택 확인 |
| `/` | 필터 / 검색 |
| `1`–`2` | 액션 화면에서 직접 선택 |
| `Ctrl+D` | 프로필 삭제 (클러스터 목록에서) |
| `Esc` / `←` | 뒤로 가기 |
| `q` | 종료 |

---

## 설정 (`~/.k10s/config.yaml`)

```yaml
global:
  configs_dir: "~/.kube/configs"       # kubeconfig 파일을 스캔할 디렉토리
  default_action: "select"             # "select" (액션 메뉴 표시) 또는 "k9s" (바로 네임스페이스 선택으로)
  contexts_mode: false                 # true: 단일 kubeconfig에서 여러 컨텍스트 읽기
  kubeconfig_file: "~/.kube/config"   # contexts_mode가 true일 때 사용

profiles:
  my-cluster:
    default_action: "k9s"             # 이 클러스터의 기본 액션 재정의
    oidc: true                        # OIDC 배지 강제 (자동 감지되지 않을 때)
```

### 필드 설명

| 필드 | 기본값 | 설명 |
|---|---|---|
| `global.configs_dir` | `~/.kube/configs` | k10s가 kubeconfig 파일을 스캔하는 디렉토리 |
| `global.default_action` | `select` | `select`는 액션 메뉴 표시, `k9s`는 네임스페이스 선택으로 바로 이동 |
| `global.contexts_mode` | `false` | 클러스터별 파일 대신 단일 파일의 모든 컨텍스트를 사용 |
| `global.kubeconfig_file` | _(없음)_ | `contexts_mode: true`일 때 사용하는 kubeconfig |
| `profiles.<name>.default_action` | _(글로벌 상속)_ | 클러스터별 액션 재정의 |
| `profiles.<name>.oidc` | `false` | OIDC 클러스터로 표시 (목록에 배지 표시, kubelogin 연동) |

---

## Contexts 모드

모든 클러스터가 여러 컨텍스트를 가진 단일 kubeconfig에 있다면, 클러스터별 파일 관리 대신 Contexts 모드를 사용하세요:

```yaml
global:
  contexts_mode: true
  kubeconfig_file: "~/.kube/config"
```

k10s는 해당 파일의 모든 컨텍스트를 열거하여 각각을 선택 가능한 클러스터로 표시합니다.

---

## OIDC

kubeconfig 사용자 항목이 `oidc` 또는 `kubelogin`을 참조하는 경우 k10s가 자동으로 감지합니다. OIDC가 있는 클러스터는 목록에 배지로 표시됩니다.

프로필에 `oidc: true`가 설정되거나 자동 감지된 경우, k10s는 [`kubelogin`](https://github.com/int128/kubelogin)이 설치되어 있다고 가정하고 k9s 또는 쉘 실행 전에 토큰 갱신을 수행합니다.

---

## 의존성

| 도구 | 필수 여부 | 비고 |
|---|---|---|
| `kubectl` | 필수 | 네임스페이스 목록 조회 및 컨텍스트 전환에 사용 |
| `k9s` | 필수 | k9s 액션 실행 시 사용 |
| `kubelogin` | 선택 | OIDC 클러스터에서만 필요 |

`k10s doctor`를 실행하면 필수 도구를 체크하고 Homebrew로 자동 설치할 수 있습니다.
