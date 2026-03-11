# k10s 로드맵 — 버그 수정 계획

## 요약 테이블

| 우선순위 | 파일 | 문제 | 수정 설명 | 공수 |
|----------|------|---------|----------------|--------|
| P0 | `internal/config/types.go` | `ArgocdConfig`에 `Password` 필드 누락 — 로그인이 항상 조용히 건너뛰어짐 | 구조체에 `Password string` yaml 필드 추가 | XS |
| P0 | `internal/config/loader.go` | `default_action` 값 검증 없음 — 잘못된 값도 조용히 허용됨 | `Validate()` 함수 추가; unmarshal 후 `Load()`에서 호출 | S |
| P0 | `internal/argocd/connect.go` | 비밀번호로 `""`가 하드코딩된 채 `Login` 호출 — 필드가 전혀 사용되지 않음 | `""` 대신 `argocdCfg.Password` 전달 | XS |
| P0 | `internal/argocd/login.go` | 빈 비밀번호일 때 메시지만 출력하고 `nil` 반환 — 에러가 묻힘 | 비밀번호가 비어있을 때 설명이 포함된 `fmt.Errorf` 반환 | XS |
| P1 | `internal/deps/checker.go` | 의존성 목록에 `lsof` 없음; 1.27+에서 `kubectl version --short` 지원 중단 | `lsof` 의존성 항목 추가; 버전 감지 제거 (도구마다 플래그가 다름) | S |
| P1 | `internal/k8s/portforward.go` | `lsof`를 사용할 수 없을 때 예외 처리 없음 — 없는 시스템에서는 조용히 실패 | `lsofAvailable()` 헬퍼 추가; `IsPortInUse` 및 `GetPIDsOnPort`에서 방어 로직 추가 | S |
| P1 | `internal/k8s/kubeconfig.go` | 불완전한 OIDC 감지 — `oidc-login`, `get-token`, `azure` exec/auth 패턴 누락 | `oidc-login` 명령, `get-token`/`azure` 인자, azure auth-provider를 찾도록 `isOIDCUser` 확장 | S |
| P1 | `internal/tui/app.go` | `StateError`는 도달할 수 없는 코드 — `Run()`에서 `err` 필드가 설정되지 않음 | `p.Run()`이 에러를 반환한 후, `AppModel{state: StateError, err: err}`를 생성하고 렌더링 | S |
| P2 | `internal/argocd/browser.go` | macOS 전용 — Linux의 `xdg-open`이나 Windows의 `start` 없음 | `runtime.GOOS`를 통해 OS별 올바른 열기 명령 선택 | S |
| P2 | `internal/profile/resolver.go` | 포트 값이 범위 검증되지 않음 — 범위를 벗어난 포트가 조용히 사용됨 | 병합 후, `LocalPort`/`RemotePort`를 1-65535로 제한; 범위 밖일 경우 기본값 폴백 | XS |

공수 기준: XS = < 30분, S = 30–60분, M = 1–3시간, L = 3시간+

---

## P0 — 치명적인 버그 (로그인이 항상 불가)

이 4가지 버그가 합쳐져 எந்த 설정에서도 ArgoCD 로그인이 **절대 동작하지 않습니다**.

### 1. 누락된 Password 필드 (`internal/config/types.go`)

`ArgocdConfig`에 `Password` 필드가 없어, `config.yaml`에 설정된 비밀번호가 파싱 때 조용히 무시됩니다. 다운스트림으로 전달되게 고치더라도 넘겨줄 값이 없습니다.

**수정:** `ArgocdConfig`에 `Password string \`yaml:"password,omitempty"\`` 추가.

### 2. default_action 검증 없음 (`internal/config/loader.go`)

`Load()`는 빈 필드에 기본값을 적용하지만 `default_action` 값이 허용된 문자열(`select`, `k9s`, `argocd`) 중 하나인지 결코 검증하지 않습니다. 잘못된 값이 조용히 전달되어 TUI에서 예기치 않은 동작을 유발합니다.

**수정:** 전역 설정과 프로필별 설정에서 알 수 없는 `default_action` 값을 거부하는 `Validate(*K10sConfig) error` 함수를 추가합니다. `yaml.Unmarshal` 이후 `Load()`에서 호출하세요.

### 3. 하드코딩된 빈 비밀번호 (`internal/argocd/connect.go`)

`Login`은 `Login(localPort, argocdCfg.Username, "", argocdCfg.Insecure)`와 같이 호출됩니다. 비밀번호가 항상 빈 문자열 리터럴이므로, 구조체 필드를 수정하더라도 `argocd login`에는 절대 값이 도달하지 않습니다.

**수정:** `Login(localPort, argocdCfg.Username, argocdCfg.Password, argocdCfg.Insecure)`를 호출하도록 변경합니다.

### 4. 빈 비밀번호시 에러 무시 (`internal/argocd/login.go`)

`password == ""` 일 때, 함수가 메시지만 띄우고 `nil`을 반환합니다. 이로 인해 설정이 잘못된 상황이 에러가 아닌 것으로 마스킹되어 디버깅이 매우 어려워집니다.

**수정:** `fmt.Errorf("argocd password not configured; set it in ~/.k10s/config.yaml under profiles.<name>.argocd.password")`를 반환하도록 변경합니다.

---

## P1 — 높은 우선순위 버그

### 5. lsof 의존성 누락 + 사용되지 않는 kubectl 플래그 (`internal/deps/checker.go`)

포트포워드 서브시스템에서 `lsof`를 직접 사용하지만 의존성에 등록되지 않아, `doctor`에서 사용자가 이를 알 수 없습니다. 또한 `kubectl version --short`는 Kubernetes 1.27에서 deprecated 되고 이후 제거되어, 버전을 구하는 데 항상 실패합니다.

**수정:** `k9s`와 `kubelogin` 사이에 `{Name: "lsof", Brew: "lsof", Required: true}`를 추가합니다. 도구별로 버전을 구하는 플래그가 다르며 확인 용도일 뿐이므로, 버전 감지 루프를 제거(무조건 `Version`을 `""`로 설정)합니다.

### 6. portforward에 lsof 예외 처리 없음 (`internal/k8s/portforward.go`)

`GetPIDsOnPort`는 무조건 `exec.Command("lsof", ...)`를 실행합니다. `lsof`가 없는 시스템(일부 리눅스 배포판, 컨테이너 등)에서는 조용히 빈 결과를 반환하므로 포트 충돌이 감지되지 않습니다.

**수정:** `exec.LookPath`를 이용한 `lsofAvailable() bool` 헬퍼를 추가합니다. `IsPortInUse`의 앞단에 헬퍼로 조건을 걸어 stderr 경고를 출력하고 `false`를 조기 반환하게 합니다. `GetPIDsOnPort`도 앞단에 조건을 걸고 `nil`을 조기 반환합니다.

### 7. 불완전한 OIDC 감지 (`internal/k8s/kubeconfig.go`)

`isOIDCUser`는 오직 auth-provider의 이름 `oidc` (대소문자 구별 안 함) 및 exec 명령어/인수에서 `kubelogin` 또는 `oidc`를 파악합니다. 다음을 놓치게 됩니다:
- `oidc-login`이 포함된 exec 명령 (int128/kubelogin 플러그인 등 지원)
- `get-token`이 포함된 exec 인수 (Azure AD kubelogin 하위 명령)
- `azure`가 포함된 exec 인수
- auth-provider 이름 `azure`

**수정:** 추가적인 4가지 패턴을 처리할 수 있도록 `isOIDCUser`에 설정된 조건을 확장합니다.

### 8. StateError 불용 코드 (`internal/tui/app.go`)

`StateError`가 `View()`에 정의 및 처리되나 정작 `err` 값이 `Run()` 함수에서 지정되지 않습니다. `p.Run()`이 오류를 뱉으면 에러 모델 구성 없이 코드 처리가 곧바로 종료됩니다. `View()` 내 `StateError`의 렌더링을 관장하는 분기는 사실상 도달이 불가능합니다.

**수정:** `finalModel, err := p.Run()` 부분 이후에 만약 `err != nil`일 시, `AppModel{state: StateError, err: err}`를 생성하고 `fmt.Fprintln`를 통해 `os.Stderr`로 렌더링한 다음에 에러를 반환하게 만듭니다. imports 구문에 `"os"`를 추가합니다.

---

## P2 — 중간 우선순위

### 9. macOS 한정의 브라우저 팝업 (`internal/argocd/browser.go`)

`OpenBrowser`에 하드코딩된 `exec.Command("open", url)` 방식은 macOS에서만 작동합니다. Linux나 Windows 기기에서 k10s 구동 시 이 때문에 ArgoCD UI 창 열기 단계가 조용히 실패합니다(또는 아예 열기가 불가).

**수정:** 플랫폼 전환 처리에 `runtime.GOOS`를 차용합니다: `"darwin"` → `open`, `"linux"` → `xdg-open`, `"windows"` → `cmd /c start`. 미지원 플랫폼에는 에러를 반환합니다.

### 10. 포트 값이 범위 검증되지 않음 (`internal/profile/resolver.go`)

프로필 설정 취합 후에, `LocalPort` 및 `RemotePort`가 유효 TCP 범위(1-65535) 내인지 확인하는 절차가 누락됐습니다. `0` 또는 `99999` 같이 세팅된 기입착의 값도 `kubectl port-forward`에 고스란히 이관되고 런타임 단계에 예기치 못한 에러를 빚어냅니다.

**수정:** merge 블록 처리가 만료된 뒤, 내장 설정된 기존 범위 폴백을 수용하여 포맷 둘 다 값을 1~65535로 제한 고정하도록 만듭니다.

---

## 테스트 인프라

현재 상태: **모든 28개 소스 파일에 걸쳐 테스트 커버리지 0%.** 레포지토리 어디에도 테스트 결과 파일이 없습니다.

### 권장 테스트 대상 (투자 대비 수익(ROI) 순)

| 패키지 | 필요한 핵심 테스트 |
|---------|-----------------|
| `internal/config` | 제대로 된/틀린 YAML에 대한 `Load()` 처리; 모든 틀린 `default_action` 값에 대한 `Validate()` 처리 |
| `internal/config` | 제대로 된 값에만 반응하는지 `Validate()` 시험; 글로벌 및 개별 프로필에서의 모르는 변수는 예외로 기각시키는지 확인 |
| `internal/k8s` | 모든 OIDC 패턴들(kubelogin, oidc-login, azure, get-token)을 포함한 `isOIDCUser`의 표-기반(Driven) 테스트 |
| `internal/profile` | `Resolve()` 병합의 선후관계 설정치; 범위 내 포트 한정(Clamp); 기본 설정 내재가치 상속 파급 |
| `internal/deps` | 목차(Mocked) PATH에 기반한 `Check()` |
| `internal/argocd` | 공란(Empty)인 비밀번호엔 `Login()`이 에러 토출하게끔 |
| `internal/tui` | 각 상태값마다 `AppModel.View()` 구동 확인 |

### 제안 접근법

1. kubeconfig 및 k10s 설정 YAML 규격 양식을 `testdata/` 폴더에 fixture로 편입 추가.
2. `~/.k10s`에 물리적인 개재 없이 임시 설정 파일 테스트를 기하도록 `t.TempDir()` 사용.
3. dep-checker 테스트로 `t.Setenv("PATH", ...)` 활용.
4. 모든 로직을 관통하는 가장 비대한 패키지들인 `config`, `profile`, `k8s` 부문에 80% 커버리지를 선 목표로 달성할 것.

---

## 추후 구상

- **난독 암호화 보관:** `config.yaml`에 비밀번호를 단순 플레인 텍스트로 보존하기보다는 OS별 운영체제 키체인 체계를 채택하여(macOS Keychain, Linux Secret Service) `argocd.password` 은닉 보관.
- **다중 kubeconfig 파일 감당:** kubeconfig 주소 경로들을 `global.kubeconfig_file`에 다수로 품게 두어 환경 요인들을 통합 편입.
- **플러그인 시스템:** 기존의 k9s나 ArgoCD외에도 맞춤형 명령어를 추가 지정하게 할 수 있는 외부 스크립트/바이너리 추가 확장 장력 제공.
- **TUI 테마 옵션:** 사용자가 `global` config 내에서 직접 색상 배합과 스킴 변환을 이룰 수 있게 해방.
- **쉘 자동완성:** cobra/completions 유틸리티를 통한 bash, zsh, fish 용 쉘 자동완성 기능 구비 적용.
- **Config 스키마 검증 도구:** 편집기 차원의 자동완성과 문법 점검을 촉발자원하기 위한 JSON Schema 공개 규격서 `config.yaml` 배포.
- **Windows 지원:** 온전한 최고등급 Windows 서비스 지원망 안착 (브라우저 오픈은 1단원일 뿐; 차제에 구동 경로 처리 방식이나 SysProcAttr 보강 등 순차적 도모).
