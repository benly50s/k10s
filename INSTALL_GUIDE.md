# k10s macOS (Apple Silicon) 빠른 설치 명령어

터미널을 열고 아래 명령어 한 줄 전체를 복사해서 붙여넣고 엔터를 치시면 설치가 끝납니다.
(중간에 Mac 로그인 비밀번호를 한 번 요구할 수 있습니다.)

```bash
curl -L -o k10s.tar.gz https://github.com/benly50s/k10s/releases/download/v0.1.5/k10s_Darwin_arm64.tar.gz && tar -xzf k10s.tar.gz && sudo mv k10s /usr/local/bin/
```
