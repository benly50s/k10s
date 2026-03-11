# 수동 테스트 가이드 — k10s

## 사전 준비 (Prerequisites)

- 바이너리 빌드 완료: `make build && ./bin/k10s --help` 로 도움말이 출력되어야 함
- `kubectl` 설치 및 설정 완료 상태
- 최소 하나 이상의 `kubeconfig` 파일이 준비된 상태

## 테스트 환경 구성 (Setup)

```bash
# 1. 빌드하기
cd /path/to/k10s && make build

# 2. configs 디렉토리 생성
mkdir -p ~/.kube/configs

# 3. 샘플 kubeconfig 복사 (스캐너 테스트 용도의 가짜 파일)
cp docs/sample-configs/*.yaml ~/.kube/configs/

# 4. config 초기화
./bin/k10s config init
```

---

## 시나리오 1: 기본 CLI 커맨드 테스트 (클러스터 연결 불필요)

### k10s doctor

**테스트 단계:**
1. `./bin/k10s doctor` 실행
2. 의존성 체크(Dependency report) 결과를 확인합니다.

**기대 결과:**
```
Dependency check:

  ✓ kubectl       found
  ✓ k9s           found
  ✓ lsof          found
  ✗ kubelogin     not found (optional)
  ✗ argocd        not found (optional)

All required dependencies satisfied.
```
만약 필수 의존성(required dep)이 누락되었다면, 다음 프롬프트를 띄워야 합니다: `Install <dep> via brew? [y/N]`

### k10s list

**테스트 단계:**
1. `~/.kube/configs/` 경로에 최소 1개 이상의 `.yaml` 파일이 있는지 확인합니다.
2. `./bin/k10s list` 실행

**기대 결과:** `NAME`, `SERVER`, `ACTION`, `OIDC` 컬럼으로 구성된 테이블이 출력되어야 합니다.
만약 구성 파일이 아예 없다면, configs 디렉토리 경로와 함께 `No profiles found.`라는 안내 문구가 나와야 합니다.

### k10s add

**테스트 단계:**
1. `./bin/k10s add docs/sample-configs/dev-cluster.yaml` 실행

**기대 결과:**
```
Added docs/sample-configs/dev-cluster.yaml -> ~/.kube/configs/dev-cluster.yaml
```
확인: `ls ~/.kube/configs/` 상에 `dev-cluster.yaml` 이 추가되었는지 체크.

### k10s remove

**테스트 단계:**
1. `./bin/k10s remove dev-cluster` 실행

**기대 결과:**
```
Removed ~/.kube/configs/dev-cluster.yaml
```
확인: `ls ~/.kube/configs/` 에 더 이상 `dev-cluster.yaml` 이 남아있지 않은지 체크.

### k10s config show

**테스트 단계:**
1. `./bin/k10s config show` 실행

**기대 결과:** 설정 파일의 전체 경로(path)와, YAML 설정 덤프 원문, 그리고 감지된 프로필들(서버 URL, 각종 태그 포함)의 목록이 출력되어야 합니다.

### k10s config init

**테스트 단계:**
1. 만약 `~/.k10s/config.yaml` 파일이 있다면 삭제합니다: `rm -f ~/.k10s/config.yaml`
2. `./bin/k10s config init` 실행

**기대 결과:**
```
Created config file: ~/.k10s/config.yaml
```
확인: `cat ~/.k10s/config.yaml` 실행 시 전역(global) 기본값들과 자동 탐지된 대상 프로필들의 내용이 보여야 합니다.

### k10s config edit

**테스트 단계:**
1. `./bin/k10s config edit` 실행

**기대 결과:** `$EDITOR` (값이 없다면 `vi`로 대체)로 `~/.k10s/config.yaml` 파일이 열려야 합니다.
수정 후 저장하고 닫았을 때, 파일 내용이 정상적으로 반영되어야 합니다.

---

## 시나리오 2: TUI 내비게이션 (클러스터 연결 불필요)

**테스트 단계:**
1. `~/.kube/configs/` 내에 kubeconfig 파일이 최소 1개라도 존재하는지 확인합니다.
2. `./bin/k10s` 실행

**기대 동작:**

| 입력 키 | 기대 동작 |
|-----|-------------------|
| 위 / 아래 방향키 | 클러스터 목록 대상자 위아래로 하이라이트 칸 이동 |
| `/` | 검색을 위한 필터 입력란이 뜹니다; 타이핑 시 목록이 그에 맞게 줄어듭니다(필터링) |
| Backspace | 검색 필터에 입력한 글자를 한 글자 지웁니다 |
| Enter | 선택 항목으로 실행 확정 후, 액션 메뉴 창으로 넘어갑니다 |
| `q` | TUI를 완전히 빠져나가며 기존 쉘 환경으로 복귀합니다 |
| Ctrl+C | TUI 로직 패닉 혹은 스택 추적 에러를 뿜지 않고 깔끔하게 빠져나갑니다(종료) |

프로필이 아무것도 없다면 화면상에 TUI가 아예 출력되지 않아야 하며, 대신 아래와 비슷한 포맷의 에러 안내문이 출력되어야 합니다:
```
No kubeconfig profiles found.
Add kubeconfig files to ~/.kube/configs or run 'k10s add <file>'
```

---

## 시나리오 3: 설정(Config) 데이터 검증

### default_action 에 잘못된 값 기입

**테스트 단계:**
1. `~/.k10s/config.yaml` 에 진입해 편집합니다:
   ```yaml
   global:
     default_action: invalid_value
   ```
2. `./bin/k10s list` 실행

**기대 동작:** 코드가 Panic 으로 죽지 않아야 하며, 명확한 에러문인 `invalid default_action "invalid_value": must be one of select, k9s, argocd` 이 출력되어야 합니다.

### 누락된 ArgoCD 비밀번호

**테스트 단계:**
1. 어떤 프로필에 ArgoCD 관련 설정을 구성하되, 비밀번호는 빼둡니다:
   ```yaml
   profiles:
     my-cluster:
       argocd:
         namespace: argocd
   ```
2. TUI 에서 해당 프로필 클러스터를 고르고 ArgoCD 액션을 선택합니다.

**기대 동작:** 에러 메시지가 제대로 표출되어야 합니다: `argocd password not configured; set it in ~/.k10s/config.yaml under profiles.<name>.argocd.password`. 묵묵부답으로 조용히 실패하지 않아야 합니다.

### 비정상적인 범주의 포트 숫자

**테스트 단계:**
1. 설정에 범주를 벗어난 포트 숫자를 기입합니다:
   ```yaml
   profiles:
     my-cluster:
       argocd:
         local_port: 99999
   ```
2. `./bin/k10s list` 실행

**기대 동작:** 값을 읽어들이는(Load) 시점에서 유효성 제한 에러를 발생시키거나, 해당 포트를 아예 무시하고 기본값(8080)으로 몰래 조용히 범위를 잘라내 덮어씌워야 합니다(Clamped). 절대로 프로그램 충돌이 발생하거나 포트 번호 99999로 바인딩 하려고 시도해선 안 됩니다.

---

## 시나리오 4: k9s 통합 (실제 클러스터 필요)

**사전 준비:**
- `kubectl`이 적합한 클러스터 컨텍스트에 제대로 세팅됨
- k9s 설치 완료 (`k10s doctor` 보고서에서 found로 확인됨)
- 최소 1개 이상의 도달 가능한 올바른 `~/.kube/configs/` kubeconfig 내재

**테스트 단계:**
1. `./bin/k10s` 실행
2. 리스트에서 접속하고자 하는 특정 클러스터 한 곳을 선택
3. `k9s` 액션 선택

**기대 동작:** 타기팅한 파일과 올바른 컨텍스트를 품은 `KUBECONFIG` 환경변수 세팅과 함께 k9s 화면이 정상적으로 열립니다. k9s UI가 나타납니다. k9s를 끄면 쉘로 원상 복귀합니다.

---

## 시나리오 5: ArgoCD 연결 (ArgoCD 배포 필요)

**사전 준비:**
- `argocd` 네임스페이스 아래에 ArgoCD가 얹혀서 동작 중인 실제 Kubernetes 클러스터
- `argocd` CLI 도구 설치 완료 (`k10s doctor` 가 found로 인식함)
- `~/.k10s/config.yaml` 에 설정된 비밀번호가 있어야 합니다:
  ```yaml
  profiles:
    my-cluster:
      argocd:
        password: "your-argocd-admin-password"
  ```

**테스트 단계:**
1. `./bin/k10s` 커맨드로 실행
2. ArgoCD 설정이 되어있는 해당 클러스터를 선택
3. 그 다음 `argocd` 액션 선택
4. 터미널 창(TUI 밖)으로 나온 상태 피드백 텍스트 관찰
5. 열렸던 브라우저가 다 열리고 나면, 콘솔에서 Ctrl+C 를 눌러 강제 정지

**기대 동작:**
- `kubectl port-forward` 가 개시됩니다 (예: `Forwarding from 127.0.0.1:8080 -> 443` 출력 문구 뜸)
- `argocd login` 이 설정에서 받은 인증번호 단서를 달고 `localhost:8080` 으로 로그인에 성공합니다.
- 브라우저가 ArgoCD UI URL로 이동하며 열립니다.
- Ctrl+C 입력 시: 곧바로 포트포워드 프로세스가 깔끔히 정지 및 강제 종료되며 어떤 무법(zombie) 프로세스 찌꺼기도 남기지 않아야 합니다.

---

## 시나리오 6: 포트 충돌 복구 시나리오

### a) 남겨진 kubectl 포트-포워드 찌꺼기 프로세스 (k10s가 강제로 이를 Kill시킨 뒤 해당 포트를 점거해야 함)

**테스트 단계:**
1. 쉘에서 수동으로 포트-포워드를 백그라운드로 엽니다:
   ```bash
   kubectl port-forward svc/argocd-server -n argocd 8080:443 &
   ORPHAN_PID=$!
   ```
2. `./bin/k10s` 를 실행한 후 방금 포트 번호가 8080인 ArgoCD 액션이 들어있는 클러스터를 클릭합니다.

**기대 동작:** 해당 8080 포트가 이미 기존 `kubectl` 에 의해 독점되어 있다는 사실을 인지하고 기존 `kubectl` 프로세스를 강제 사살한 후 당당히 새로운 포트포워드 작업을 8080으로 실행합니다. 에러 창을 내뿜지 않습니다.

### b) 타 프로세스가 해당 포트를 사용 중일 경우 (k10s가 다음 후보군 포트 번호인 빈 자리를 자동으로 섭외해야 함)

**테스트 단계:**
1. 이번에는 아예 kubectl 과 상관이 없는 다른 무관한 프로세스로 8080을 미리 차지합니다:
   ```bash
   python3 -m http.server 8080 &
   HTTP_PID=$!
   ```
2. 다시 `./bin/k10s` 를 돌려서 포트 8080번지에 지정된 ArgoCD 액션을 선택합니다.

**기대 동작:** 포트 8080 번지가 이미 다른 류의 프로세스의 몫으로 할당되어 있음을 파생 감지하고 그 다음 빈자리 대체 포트(예: 8081)를 자동으로 파보고 선정해냅니다. 그리고 해당 포트에 대고 포트포워딩을 개시합니다. 응답 없음(Hang)이나 충돌이 발생하지 않아야 합니다.

**청소 작업:** 테스트 완료 후 마무리에 `kill $HTTP_PID` 실행.

---

## 시나리오 7: OIDC 접근 인증 (Authentication)

**사전 준비:**
- (auth-provider `oidc`, 또는 `kubelogin` 및 `oidc-login`이 exec로 지정된) OIDC 인증을 쓰는 임의의 kubeconfig 파일
- `kubelogin` 바이너리 필수 내장 (`k10s doctor` 결과가 found 표시)

**테스트 단계:**
1. `~/.kube/configs/` 안에 OIDC 관련 설정이 담긴 이 kubeconfig를 집어넣습니다.
2. `./bin/k10s list` 커맨드 확인 — `OIDC` 항목 컬럼 표시에 `true` 상태로 떠있는지 봅니다.
3. 곧장 `./bin/k10s` 를 구동해 해당 클러스터의 `k9s` 액션란까지 들어갑니다.

**기대 동작:** k9s 명령어를 치기 직전에, 당장 k10s가 먼저 알아서 OIDC 토큰의 갱신을 독촉하는 `kubelogin` 작업을 밟습니다. 그런 후에야 브라우저 위주 로그인 세션 창이 호출될 수 있습니다. 최종 갱신작업을 성공리에 마치고 나야 곧이어 새로 충천받은 세션을 물고 k9s가 점화되는 흐름이 완성됩니다.

---

## 시나리오 8: 컨텍스트 모드 (Context Mode)

**사전 준비:**
- 다수의 컨텍스트를 전부 담고 있는 단일체 kubeconfig 파일 규격 구비 (예: `docs/sample-configs/prod-cluster.yaml`)

**테스트 단계:**
1. 우선 설정 파일 `~/.k10s/config.yaml` 의 전역 파트를 다음과 같이 최신화합니다:
   ```yaml
   global:
     contexts_mode: true
     kubeconfig_file: "~/.kube/config"
   ```
2. `./bin/k10s list` 를 실행합니다.

**기대 동작:** 방금 타깃을 정해주었던 단일 kubeconfig 안쪽의 수많은 컨텍스트들이 각기 개별적인 엔트리 항목으로서 깔끔히 리스트 창을 수놓게 되며, 제각각의 서버 URL 도 모두 정직하게 파싱되어 나와주어야 합니다. 특히 `NAME` 컬럼에선 그 당시의 각 컨테스트 명칭(가령 `prod-us` 혹은 `prod-eu` 등)이 올바로 표시되어야 합니다.

---

## 통과여부 진단 테스트 셀프 체크리스트(Checklist)

### 시나리오 1: 기본 CLI 커맨드

- [ ] `k10s doctor` 이 found/not-found 표기 현황을 올바르게 잘 표시함
- [ ] `k10s doctor` 의존성이 부족하면 brew 다운로드를 진행하라는 화면 창이 뜸
- [ ] 여러 프로필들이 존재 시 테이블 포맷으로 `k10s list` 가 예쁘게 나열됨
- [ ] 만약에 아무 프로필이 없으면 적당한 안내 알림말을 `k10s list` 가 잘 토설함
- [ ] `k10s add <file>` 명령이 수행되면 configs 디렉토리 구역으로 해당 파일이 잘 복사됨
- [ ] `k10s remove <name>` 명령이 들어가면 config 디렉토리에서 무사히 제거 및 소멸됨
- [ ] Config 파일과 연관 프로필 항목들이 `k10s config show` 명령으로 정상 표절됨
- [ ] `k10s config init` 명령 후 `~/.k10s/config.yaml` 이 기본 세트로 생성됨
- [ ] 이미 파일 구성물이 있다면 `k10s config init` 프롬프트 단에서 덮어쓰기 유무를 먼저 질문받음
- [ ] `$EDITOR` 세션상에서 `k10s config edit` 를 통해 깔끔히 설정 편집 창이 오소리 뜸

### 시나리오 2: TUI 내비게이션

- [ ] 여러 프로필이 진열돼있는 상황을 맞이했을 때 TUI 가 무사히 잘 점화됨
- [ ] 방향키가 선택 항목 위치 하이라이트바 이동 역할을 여실히 반영함
- [ ] `/` 키를 치면 검색용 텍스트 필터 구역이 별도 생성됨
- [ ] 검색란에 글자 타이핑 압력이 들어가면 즉각 대조해서 알맞게 출력 항목들을 좁히고 필터링됨
- [ ] Backspace 키에 대고 이미 친 글자를 일일이 지우는 백업 무르기가 통용됨
- [ ] Enter 로써 원했던 선택 확정 기능을 정상 소화함
- [ ] `q` 글자를 갈기면 흠집 및 에러 없이 TUI를 깔끔히 나옴
- [ ] Ctrl+C 를 치명적인 패닉(panic) 충돌 없이 유연하게 수용하고 정상 구동 정지를 달성함
- [ ] 아무 연관 프로필을 찾을 수 없었을 때 TUI 로 무리하게 접속 시켜버리는 대참사가 없으며 도움말 문구로 차치됨

### 시나리오 3: 설정(Config) 데이터 검증

- [ ] 옳지 못한 `default_action` 의 반입이 확인되면 프로그램 패닉에 빠지지 않고 정직하게 에러 원인을 적시해 줌
- [ ] ArgoCD 구성에 비밀번호 설정이 되지 않은 위중 시 조용히 넘기지 않고 대놓고 원인 파악을 유도하는 에러를 띄움
- [ ] 포트 번호 수용 범위를 까마득히 이탈한 막무가내 숫자 유입에도 이탈이나 참사 없이 슬기롭게 극복 차폐함

### 시나리오 4: k9s 통합

- [ ] 타깃으로 잘 정제된 `KUBECONFIG` 매개체 변수가 k9s 프로세스에 무리 없이 전달 인입됨
- [ ] k9s 화면 내 상단에서도 자신이 원했던 올바른 환경 컨텍스트 구역의 라벨이 지명되어 뜸
- [ ] k9s에서 빠져나왔을 때 정상 쉘 환경으로 곧바로 귀환 및 복원됨

### 시나리오 5: ArgoCD 연결

- [ ] 성공적으로 포트-포워딩이 스타트를 끊고 중계 포워딩 과정이 문구로 뜨는 것이 관찰됨
- [ ] 뒤따라 `argocd login` 이 차질 없이 성사됨
- [ ] 무사히 개시된 브라우저가 타깃된 웹상의 URL(ArgoCD UI)로 조향 성공함
- [ ] 차후 Ctrl+C를 통한 포트 포워드 정지 요청을 깔끔하게 받아먹음

### 시나리오 6: 포트 충돌 복구 시나리오

- [ ] 갈길을 잃은 kubectl 백그라운드 프로세스의 잔재들이 무참히 색출 진압되고 빈 포트 라인은 즉시 포획 점유됨
- [ ] 전혀 상관없는 민간 프로세스가 포워드 대기 중인 라인 번지에 터를 잡고 있으면 융통성 있게 즉각 다이렉트로 빈 곳 다음 번호 차석 포트로 자동 갈아타고 자리 잡음

### 시나리오 7: OIDC 접근 인증 계열

- [ ] `k10s list` 출력표 상에서 `OIDC` 에 해당하는 OIDC 진영들의 란에 어김 없이 `true` 지표기가 활성화되어 뜸
- [ ] k9s 에 들어가려는 오프닝 세레모니 이전에 `kubelogin` 징검다리 스텝이 먼저 선점적으로 잘 불려 나타남
- [ ] OIDC 토큰 재정비 인증이 끝마침을 고하면 성공적으로 대망의 k9s가 등판함

### 시나리오 8: 컨텍스트 모드

- [ ] 뭉쳐있는 모든 컨텍스트 분가 식구들이 하나도 빠짐없이 `k10s list` 테이블 파티장에 초대되어 각각 줄서있음
- [ ] 컨텍스트 고유 이름들이 그들의 어엿한 프로필 명칭들로 무난하게 간판 교체되어 매핑됨
- [ ] 각 서브 컨텍스트들이 원래 쥐고 있었던 서버 URL 출구 본위들이 명징하게 해독 및 노출됨

---

## 트러블슈팅(Troubleshooting)

### "lsof not found" 에러

포트 충돌 색출 및 단선 차단 처리를 위해 `lsof` 바이너리 설치가 시급합니다. 리눅스에서는 다음과 같습니다:
```bash
apt install lsof       # Debian/Ubuntu 계열
yum install lsof       # RHEL/CentOS 계열
brew install lsof      # macOS (보통은 기본 탑재임)
```
`lsof` 수배가 불발에 그치면 프로그램이 어쩔 수 없이 포트 충돌 여부 감지 과정을 스킵 넘겨야 하고 정작 포트 번지가 타깃에 의해 사용 중일 경우 오동작할 위험을 내포하게 됩니다.

### "no kubeconfig profiles found" 문구

보통의 경우, k10s는 우선 `~/.kube/configs/` 주소 안쪽에 모여있을 kubeconfig 파일들을 사냥합니다. 이런 경우에는:
- 그냥 그 주소 안에다 복붙을 해주거나: `k10s add ~/Downloads/my-cluster.yaml`
- 수색 허용지 주소를 바꾸자: `k10s config edit` → `global.configs_dir` 주소란 경로 수정
- 아니면 단일 파일 내재의 다중 컨텍스트 수납함 문호를 개방 모드로 열어버리고 기존의 파일 경로를 명시하기: `global.contexts_mode: true` 및 `global.kubeconfig_file` 설정 기입

### ArgoCD password not configured 에러가 터졌을 때

명명된 타깃 클러스터 프로필 라인의 품 안 쪽에, `~/.k10s/config.yaml` 에서 직접 수동으로 해당 필드를 기재하십시오:
```yaml
profiles:
  <profile-name>:
    argocd:
      password: "여기에_설정하려는_비밀번호_입력"
```
먼저 `k10s list` 로 해당 클러스터 프로필 본인 당사자 명칭을 눈으로 재차 점검한 후에, `k10s config edit` 화면에서 누락된 부분을 텍스트로 박아두시면 됩니다.

### Port-forward 연결이 타임아웃 지연 시에

시간이 야속하게 흐르고 포워딩 끈끈이가 먹통으로 프리징 타임아웃 났을 때:
1. 배 밖의 클러스터 접근은 살아있는지 진단 여부 파악: `kubectl cluster-info --kubeconfig <file>`
2. ArgoCD 본진 서비스 자체는 배포되었는지가 문제의 핵심: `kubectl get svc -n argocd argocd-server`
3. 이미 전에 열렸던 지들끼리의 뒷단 닫힘 포트포워드 뒷문 좀비 잔재 세션 찾기: `lsof -i :<포트번호>` 이후 수동으로 도륙 내서 kill 수행
4. 당초 ArgoCD 기입 설정의 `namespace` 및 `service` 명칭이 기존 클러스터 배포 실제 현황 명칭과 제대로 이빨이 들어맞아 호환 일치되는지 더블 체크 수행
