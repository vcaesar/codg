# Codg

<p align="left">
    <a href="https://github.com/vcaesar/codg/releases"><img src="https://img.shields.io/github/release/vcaesar/codg" alt="최신 릴리스"></a>
    <a href="https://github.com/vcaesar/codg/actions"><img src="https://github.com/vcaesar/codg/actions/workflows/go.yml/badge.svg" alt="빌드 상태"></a>
    <a href="https://pkg.go.dev/github.com/vcaesar/codg?tab=doc"><img src="https://pkg.go.dev/badge/github.com/vcaesar/codg?status.svg" alt="GoDoc"></a>
    <a href="https://discord.gg/Dy5QZRbaND"><img src="https://img.shields.io/discord/1484658282777018551.svg?logo=discord&logoColor=white&label=Discord&color=5865F2" alt="Discord 채팅 참여 https://discord.gg/Dy5QZRbaND"></a>
</p>

차세대의 손쉬운 코드 및 업무용 AI 에이전트 시스템. 자동·비동기, 높은 동시성과 성능, 효율성과 정확성을 모두 갖춘 솔루션입니다.

[English](../README.md) | [繁體中文](./README.zht.md) | [简体中文](./README.zh.md) | [日本語](./README.ja.md) | [한국어](./README.ko.md) | [Français](./README.fr.md) | [Deutsch](./README.de.md) | [Español](./README.es.md) | [Português](./README.pt.md) | [Русский](./README.ru.md) | [العربية](./README.ar.md)

<p align="center">
<a href="https://atomai.cc" rel="nofollow">
<img width="800" alt="Codg 데모" src="https://github.com/vcaesar/codg/raw/main/demo/26-04.png" />
<img width="800" alt="Codg 데모" src="https://github.com/vcaesar/codg/raw/main/demo/26-04-1.png" />
</a>
</p>

## 설치

Mac 및 Linux:

```bash
# Homebrew
brew install vcaesar/tap/codg

# NPM
# npm install -g @vcaesar/codg
```

Windows:

```bash
# Winget
# winget install vcaesar.codg
```

```bash
# YOLO
curl -fsSL https://raw.githubusercontent.com/vcaesar/codg/main/demo/boot.sh | bash
```

또는 [Releases](https://github.com/vcaesar/codg/releases)를 직접 클릭하여 다운로드한 뒤 실행하세요.

프로젝트 디렉터리로 이동해 `codg`를 실행하세요.

# 주요 기능

- 자동·비동기, 높은 동시성과 성능을 지닌 에이전트 시스템이며 메모리 사용량도 적음
- openai-compat 또는 claude-compat을 통한 다중 모델 및 로컬 모델 지원. 모든 터미널과 OS에서 동작
- 사용 편의성: TUI는 어디서나 GUI처럼 사용 가능하며, 데스크톱/웹 버전은 BETA 제공 중
- 클릭 또는 "/xxx" 로 세션 전환, TUI 어디서나 클릭 가능
- "Modified Files" 클릭 또는 "/diff", "/diff git" 으로 VSCode처럼 TUI 내에서 diff 파일 확인
- 영문 글자 및 짧은 문장 자동 완성

데스크톱 앱(BETA), 웹(BETA), Claw(BETA)

## 버그 신고:

[Github Issues](https://github.com/vcaesar/codg/issues) 를 열어주세요

## 데이터 사용 방침:

현재 어떠한 데이터나 텔레메트리도 수집하지 않으며, 100% 로컬 모델을 지원합니다. API 사용 시 해당 제공자의 정책을 참고하세요.

# CLI 명령어

`codg -h` 또는 TUI에서 "/help" 사용:

```bash
codg auth/login               # 인증 (Atom, OpenAI, GitHub...)
codg web                      # 포트 4096에서 웹 UI 시작
codg desktop                  # 데스크톱 앱 실행 (Wails)
codg claw                     # 메시징 에이전트 시작 (Telegram/Discord/Slack)
codg gateway --private-only   # 보안 게이트웨이 시작
codg models claude            # "claude"와 일치하는 모델 목록
codg runm start Qwen/Qwen3-8B-GGUF   # 로컬 모델 시작
codg runm download user/model # GGUF 모델 다운로드
codg plugin install repo/name # 플러그인 설치
codg plugin list              # 설치된 플러그인 목록
codg install repo/name        # plugin install의 축약형
codg mcp add myserver cmd     # MCP 서버 추가
codg mcp list                 # 구성된 MCP 서버 목록
codg skill url add <url>      # 스킬 소스 URL 추가
codg themes set catppuccin    # 테마 전환
# codg logs -f                # 애플리케이션 로그 추적
codg toml                     # 모든 설정 표시
codg stats/s                  # 사용 통계 표시
codg dirs                     # 데이터/설정 디렉터리 경로 출력
codg projects                 # 추적 중인 프로젝트 디렉터리 목록
codg lite 2                   # 라이트 모드 레벨 설정 (0-4)
codg merge origin main        # v1/ 백업이 포함된 안전한 git 병합
codg migrate                  # .claude/.opencode에서 설정 마이그레이션
codg vm build                 # 원격 VM에서 빌드
codg vm run -- make test      # VM에서 명령 실행
codg sandbox run -- ./test.sh # 샌드박스에서 실행
codg sandbox status           # 샌드박스 가용성 확인
codg update                   # 제공자 정의 업데이트
```

## 사용 예시

### 비대화식 모드 (`codg run`)

```bash
# 다른 명령의 출력을 파이프로 전달.
cat errors.log | codg run "이 오류들의 원인은?"
# 상세 모드 (디버그 출력을 stderr로).
codg run -v "이 함수 디버깅"
```

### 웹 UI

```bash
# 기본 포트 4096 에서 웹 UI 시작; (테스트 완료 후 빌드).
codg web
# 사용자 지정 포트.
codg web -p 8080

# API 전용 모드 (프런트엔드 및 브라우저 없음).
codg web 0
```

### 플러그인 관리

```bash
# Git 저장소에서 플러그인 설치.
codg install github.com/user/codg-xxx-auth
```

### 커스텀 에이전트 및 스킬:

xx_agent.md (.codg/agents/templates) 또는 SKILL.md (.codg/skills)를 해당 디렉터리에 복사

# 설정 시스템

프로젝트 루트에 `codg.toml`을 생성하거나(전역 설정은 `~/.codg/config/codg.toml`):

```toml
# codg.toml — 최소 프로젝트 설정.
[options]
lite_mode = 0          # 0 = 모든 에이전트, 2 = 기본 경량 세트, 4 = 단일 에이전트
locale    = "en"       # UI 언어: en, zh-CN, ja

[options.tui]
theme     = "catppuccin"
dark_mode = true
compact_mode = false
```

### 제공자 설정

```toml
# API 키 사용 ($ENV_VAR 확장 지원).
[providers.anthropic]
api_key = "$ANTHROPIC_API_KEY"

# OAuth 사용 (`codg auth`로 설정).
[providers.openai]
oauth = true

# 커스텀/자체 호스팅 제공자.
[providers.local]
name     = "My Local LLM"
type     = "openai-compat"
base_url = "http://localhost:8080/v1"
api_key  = "not-needed"
```

### 에이전트 커스터마이징

```toml
# 축약형: 모델 타입 지정.
agents.coder = "large"
agents.task  = "small"

# 완전형: 에이전트 세부 조정.
[agents.advisor]
model           = "large"
temperature     = 0.3
thinking_budget = 32000
```

### MCP 서버

```toml
# HTTP MCP 서버.
[mcp.websearch]
type = "http"
url  = "https://mcp.exa.ai/mcp?tools=web_search_exa"
```

### 스킬

```toml
# TUI 또는 codg skill에서 자동 로드 및 다운로드
[option]
skill_urls = ["https://github.com/user/skills"]
```

### 로컬 모델 (llama.cpp)

```toml
[llama]
port     = 8090
host     = "127.0.0.1"
ctx_size = 32000
gpu      = "auto"          # auto, cuda, off
```

### 메시징 채널

```toml
[channels.telegram]
enabled     = true
token       = "$TELEGRAM_BOT_TOKEN"
allowed_ids = ["123456789"]

[channels.discord]
enabled  = true
token    = "$DISCORD_BOT_TOKEN"
```

### 권한

```toml
[permissions]
allowed_tools = ["bash", "edit", "view", "glob", "grep"]
allowed_dirs = ["**x"] # 모든 디렉터리
```
