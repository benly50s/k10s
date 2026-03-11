# k10s 기여 가이드

## 개발 환경

**필수 요구사항:**
- Go 1.21+
- kubectl
- k9s
- lsof (macOS 기본 내장; Linux 설치 시: `apt install lsof`)
- make

**선택 요구사항 (모든 기능 테스트용):**
- kubelogin — OIDC 인증용
- argocd CLI — ArgoCD 로그인 플로우용
- Kubernetes 클러스터 접근 권한

## 빌드 방법

```bash
make build          # ./bin/k10s
make install        # /usr/local/bin/k10s (쓰기 권한 필요)
make clean          # ./bin/ 디렉토리 삭제
go build -o ./bin/k10s .   # make build와 동일함
```

## 테스트 실행

```bash
go test ./...                           # 전체 단위 테스트 실행
go test ./internal/config/... -v        # config 패키지 테스트
go test ./internal/profile/... -v       # profile 패키지 테스트
go test ./internal/k8s/... -v           # k8s 패키지 테스트
go test ./internal/deps/... -v          # deps 패키지 테스트

# 통합 테스트 (실제 라이브 툴 필요):
go test -tags integration ./...
```

## 코드 품질

```bash
go vet ./...        # 정적 분석 (모든 커밋 전에 실행할 것)
```

## 프로젝트 구조

```
k10s/
├── main.go              진입점 (Entry point)
├── cmd/                 cobra CLI 커맨드
│   ├── root.go          기본 커맨드 → TUI 연결
│   ├── list.go          k10s list
│   ├── add.go           k10s add <file>
│   ├── remove.go        k10s remove <name>
│   ├── doctor.go        k10s doctor
│   └── config.go        k10s config init/show/edit
└── internal/
    ├── config/          설정 파일 타입, 로더, 기본값 정의
    ├── profile/         프로필 스캐닝 (파일 기반 & 컨텍스트 기반)
    ├── tui/             Bubble Tea TUI 구현체 (상태 머신)
    ├── k8s/             kubeconfig 파싱, 포트포워드 관리
    ├── argocd/          ArgoCD 커넥션 오케스트레이션 수행
    ├── auth/            kubelogin을 경유한 OIDC 새로고침
    ├── executor/        k9s 프로세스 실행자 (syscall.Exec)
    └── deps/            의존성 감식기 (brew install 수행)
```

## 새로운 기능 추가하기

1. `internal/` 폴더 내 관련이 깊은 소속 패키지를 파악합니다.
2. 기존의 `*_test.go` 파일들의 작성 패턴을 참조하여 테스트 코드를 먼저 작성합니다.
3. 기능을 구현합니다.
4. `go vet ./...` 와 `go test ./...` 를 통과하는지 검증합니다.
5. 유저와 직접 대면하는 외형/외관의 작동 요소가 변환됐다면 반드시 `README.md`도 최신화합니다.

## Config 스키마 참조

전체 스키마에 대해서는 `internal/config/types.go` 문서를 살핍니다.
주요 타입 종류: `K10sConfig`, `GlobalConfig`, `ProfileConfig`, `ArgocdConfig`.
