# k10s 빠른 설치 가이드

> 최신 릴리스 버전은 https://github.com/benly50s/k10s/releases 에서 확인하세요.
> 아래 예시의 `<VERSION>` 은 실제 태그(예: `v0.1.6`)로 바꿔 사용합니다.

---

## macOS (Apple Silicon)

터미널에서 한 줄 실행으로 끝납니다. (중간에 Mac 로그인 비밀번호를 한 번 요구할 수 있습니다.)

```bash
curl -L -o k10s.tar.gz https://github.com/benly50s/k10s/releases/download/v0.1.5/k10s_Darwin_arm64.tar.gz && tar -xzf k10s.tar.gz && sudo mv k10s /usr/local/bin/
```

Homebrew 를 쓸 수 있다면:

```bash
brew install benly50s/tap/k10s
```

---

## Windows (amd64) — 오프라인 반입 설치

Windows 설치파일은 `k10s.exe` + `kubectl.exe` + `k9s.exe` + `kubelogin.exe` 를 하나의 `.exe` 에 담은 번들입니다. 관리자 권한이 필요 없고, 인터넷이 차단된 환경에서도 USB 로 반입해 더블클릭 한 번으로 설치됩니다.

### 1. 반입할 파일 받기 (인터넷 되는 곳에서)

[Releases 페이지](https://github.com/benly50s/k10s/releases) 에서 아래 2개를 내려받습니다.

- `k10s-setup-<VERSION>.exe`
- `k10s-setup-<VERSION>.exe.sha256`

두 파일을 USB 등에 복사해 대상 Windows 머신으로 옮깁니다.

### 2. 무결성 확인 (권장)

대상 머신에서 PowerShell 을 열고 실행합니다. 출력된 해시가 `.sha256` 파일의 해시와 **일치해야 합니다**.

```powershell
Get-FileHash .\k10s-setup-<VERSION>.exe -Algorithm SHA256
Get-Content  .\k10s-setup-<VERSION>.exe.sha256
```

### 3. 설치

`k10s-setup-<VERSION>.exe` 를 더블클릭합니다.

> **SmartScreen 경고**가 뜨면 "추가 정보" → "실행" 을 눌러 진행합니다. 코드 서명이 없는 설치파일이라 Windows 가 경고를 띄우는 것으로, `.sha256` 으로 무결성을 확인한 상태라면 정상 진행할 수 있습니다.

설치 마법사가 `%LocalAppData%\Programs\k10s\` 에 번들을 풀고, 해당 폴더를 사용자 `PATH` 에 추가합니다.

### 4. 동작 확인

**새** PowerShell 창을 열고 (기존 창은 PATH 가 아직 반영 안 됨):

```powershell
k10s --help
k10s doctor     # kubectl / k9s / kubelogin 모두 found 로 표시되어야 함
kubectl version --client
k9s version
```

### 5. 제거

제어판 → "프로그램 추가/제거" → `k10s` 선택 → 제거. 설치 폴더와 PATH 항목이 함께 정리됩니다.

---

## 공통: 최초 사용

1. kubeconfig 추가: `k10s add <kubeconfig 경로>`
2. 클러스터 목록 확인: `k10s list`
3. TUI 실행: `k10s`

자세한 사용법은 [README](./README.md) 참고.
