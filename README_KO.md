# k10s — 빠른 참조 가이드

> 전체 문서는 아래의 한국어 섹션을 참조하세요. (역자 주: 기존 문서 구조를 반영한 번역)

## 빌드 및 설치

```bash
# 사전 준비: Go 1.21+, make
make build    # → ./bin/k10s
make install  # → /usr/local/bin/k10s

# 확인
./bin/k10s --help
./bin/k10s doctor
```

## 빠른 시작

1. **의존성 확인**: `k10s doctor` (brew를 통해 누락된 kubectl/k9s 설치)
2. **kubeconfig 추가**: `k10s add ~/Downloads/my-cluster.yaml`
3. **클러스터 목록 조회**: `k10s list`
4. **TUI 실행**: `k10s` → 화살표 키로 선택, Enter로 확인, `/`로 검색
5. **설정 세팅**: `k10s config init` 후 `k10s config edit`

## ArgoCD 비밀번호

ArgoCD 로그인을 활성화하려면 `~/.k10s/config.yaml`에 비밀번호를 추가하세요:

```yaml
profiles:
  my-cluster:
    argocd:
      password: "your-argocd-password"
```

## Context 모드 (단일 kubeconfig, 다중 컨텍스트)

```yaml
# ~/.k10s/config.yaml
global:
  contexts_mode: true
  kubeconfig_file: "~/.kube/config"
```

---
