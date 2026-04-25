# Codg

<p align="left">
    <a href="https://github.com/vcaesar/codg/releases"><img src="https://img.shields.io/github/release/vcaesar/codg" alt="最新版本"></a>
    <a href="https://github.com/vcaesar/codg/actions"><img src="https://github.com/vcaesar/codg/actions/workflows/go.yml/badge.svg" alt="建置狀態"></a>
    <a href="https://pkg.go.dev/github.com/vcaesar/codg?tab=doc"><img src="https://pkg.go.dev/badge/github.com/vcaesar/codg?status.svg" alt="GoDoc"></a>
    <a href="https://discord.gg/Dy5QZRbaND"><img src="https://img.shields.io/discord/1484658282777018551.svg?logo=discord&logoColor=white&label=Discord&color=5865F2" alt="加入 Discord 聊天 https://discord.gg/Dy5QZRbaND"></a>
</p>

下一代簡易的程式碼與工作 AI 智慧代理 Harness 系統，自動化、非同步、高並行、高效能，高效率且高精度。

[English](../README.md) | **繁體中文** | [简体中文](./README.zh.md) | [日本語](./README.ja.md) | [한국어](./README.ko.md) | [Français](./README.fr.md) | [Deutsch](./README.de.md) | [Español](./README.es.md) | [Português](./README.pt.md) | [Русский](./README.ru.md) | [العربية](./README.ar.md)

<p align="center">
<a href="https://atomai.cc" rel="nofollow">
<img width="800" alt="Codg 展示" src="https://github.com/vcaesar/codg/raw/main/demo/26-04.png" />
<img width="800" alt="Codg 展示" src="https://github.com/vcaesar/codg/raw/main/demo/26-04-1.png" />
</a>
</p>

## 安裝

Mac 與 Linux：

```bash
# Homebrew
brew install vcaesar/tap/codg

# NPM
# npm install -g @vcaesar/codg
```

Windows (PowerShell):

```powershell
# Winget
# winget install vcaesar.codg

# YOLO（原生 PowerShell 安裝腳本）
irm https://raw.githubusercontent.com/vcaesar/codg/main/demo/boot.ps1 | iex
```

全平台（macOS、Linux，或透過 Git Bash / MSYS2 / Cygwin / WSL 執行的 Windows）：

```bash
# YOLO
curl -fsSL https://raw.githubusercontent.com/vcaesar/codg/main/demo/boot.sh | bash
```

或者直接點擊 [Releases](https://github.com/vcaesar/codg/releases) 下載並執行。

進入您的專案目錄，執行 `codg`。
使用 “/yolo” 切換自動模式與詢問模式，權限可透過 codg.toml 設定。

## 功能特色

- 自動化、非同步、高並行、高效能的智慧代理系統，記憶體佔用低
- 多模型提供商（40+ API 與 Pro 提供商、Custom URL API）及本地模型（透過 openai-compat 或 claude-compat），支援 Openrouter、Ollama、Nvidia 等免費模型，可透過 “/connect” “/models” 或 “codg auth” 使用
- 支援任何終端機與作業系統，同時支援 Web 終端機
- 易用性：TUI 隨處可用，體驗接近 GUI，桌面版與 Web 版處於 BETA 階段
- 點擊或輸入 “/xxx” 切換會話，TUI 中任意位置皆可點擊
- 點擊 “Modified Files” 或輸入 “/diff”、“/diff git” 在 TUI 中檢視差異檔案，體驗與 VSCode 相似
- 自動補全英文字母與短句

桌面應用（BETA）、Web（BETA）、Claw（BETA），部分功能仍需等待測試與修復 bug 後再發佈。

## 基準測試

### 記憶體使用

| 工具                   | 1 個活躍工作階段 | 10 個活躍工作階段  | 每新增工作階段額外 PSS      |
| ---------------------- | ---------------- | ------------------ | --------------------------- |
| **Codg**               | 65 MB            | 165 MB             | ~10 MB                      |
| **Codex CLI**          | 140.0 MB         | 334.8 MB           | ~21.6 MB                    |
| **Cursor Agent**       | 214.9 MB         | 1632.4 MB          | ~157.5 MB                   |
| **GitHub Copilot CLI** | 333.3 MB         | 1756.5 MB          | ~158.1 MB                   |
| **OpenCode**           | 371.5 MB         | 3237.2 MB          | ~318.4 MB                   |
| **Claude Code**        | 386.6 MB         | 2300.6 MB          | ~212.7 MB                   |

## 回報 Bug

請提交 [Github Issues](https://github.com/vcaesar/codg/issues)

## 我們如何使用您的資料

目前不蒐集任何資料與遙測資訊，並支援 100% 本地模型；使用 API 時請參閱對應服務商的隱私政策。

# CLI 指令

使用 `codg -h` 或在 TUI 中輸入 “/help”

```bash
codg auth/login               # 登入認證 (Atom、OpenAI、GitHub...)
codg web                      # 在 4096 埠啟動 Web UI
codg desktop                  # 啟動桌面應用 (Wails)
codg claw                     # 啟動訊息代理 (Telegram/Discord/Slack)
codg gateway --private-only   # 啟動受保護的閘道
codg models claude            # 列出符合 "claude" 的模型
codg runm start Qwen/Qwen3-8B-GGUF   # 啟動本地模型
codg runm download user/model # 下載 GGUF 模型
codg plugin install repo/name # 安裝外掛
codg plugin list              # 列出已安裝的外掛
codg install repo/name        # plugin install 的簡寫形式
codg mcp add myserver cmd     # 新增 MCP 伺服器
codg mcp list                 # 列出已設定的 MCP 伺服器
codg skill url add <url>      # 新增技能來源 URL
codg themes set catppuccin    # 切換佈景主題
# codg logs -f                # 追蹤應用程式日誌
codg toml                     # 顯示全部設定
codg stats/s                  # 顯示使用統計
codg dirs                     # 列印資料/設定目錄路徑
codg projects                 # 列出追蹤的專案目錄
codg lite 2                   # 設定精簡模式等級 (0-4)
codg merge origin main        # 含 v1/ 備份的安全 git 合併
codg migrate                  # 從 .claude/.opencode 移轉設定
codg vm build                 # 在遠端 VM 上建置
codg vm run -- make test      # 在 VM 上執行指令
codg sandbox run -- ./test.sh # 在沙盒中執行
codg sandbox status           # 檢視沙盒可用性
codg update                   # 更新服務商定義
```

## 使用範例

### 非互動模式 (`codg run`)

```bash
# 透過管線傳入另一個指令的輸出。
cat errors.log | codg run "這些錯誤的原因是什麼?"
# 詳細模式 (除錯資訊輸出至 stderr)。
codg run -v "除錯這個函式"
```

### Web UI

```bash
# 在預設埠 4096 啟動 Web UI;(等待測試完成後發佈)。
codg web
# 自訂埠。
codg web -p 8080

# 僅 API 模式 (無前端、無瀏覽器)。
codg web 0
```

### 外掛管理

```bash
# 從 Git 儲存庫安裝外掛。
codg install github.com/user/codg-xxx-auth
```

### 自訂代理與技能

將 xx_agent.md (.codg/agents/templates) 或 SKILL.md (.codg/skills) 複製到對應目錄

# 設定系統

在專案根目錄建立 `codg.toml`（或 `~/.codg/config/codg.toml` 用於全域設定）：

```toml
# codg.toml — 最小化專案設定。
[options]
lite_mode = 0          # 0 = 全部代理,2 = 預設精簡集,4 = 單一代理
locale    = "en"       # UI 語言:en、zh-CN、ja

[options.tui]
theme     = "catppuccin"
dark_mode = true
compact_mode = false
```

### 服務商設定

```toml
# 使用 API 金鑰 (支援 $ENV_VAR 展開)。
[providers.anthropic]
api_key = "$ANTHROPIC_API_KEY"

# 使用 OAuth (透過 `codg auth` 設定)。
[providers.openai]
oauth = true

# 自訂/自架服務商。
[providers.local]
name     = "My Local LLM"
type     = "openai-compat"
base_url = "http://localhost:8080/v1"
api_key  = "not-needed"
```

### 代理自訂

```toml
# 簡寫形式:指定模型類型。
agents.coder = "large"
agents.task  = "small"

# 完整形式:微調代理。
[agents.advisor]
model           = "large"
temperature     = 0.3
thinking_budget = 32000
```

### MCP 伺服器

```toml
# HTTP MCP 伺服器。
[mcp.websearch]
type = "http"
url  = "https://mcp.exa.ai/mcp?tools=web_search_exa"
```

### 技能

```toml
# 在 TUI 或 codg skill 中自動載入與下載
[option]
skill_urls = ["https://github.com/user/skills"]
```

### 本地模型 (llama.cpp)

```toml
[llama]
port     = 8090
host     = "127.0.0.1"
ctx_size = 32000
gpu      = "auto"          # auto、cuda、off
```

### 訊息通道

```toml
[channels.telegram]
enabled     = true
token       = "$TELEGRAM_BOT_TOKEN"
allowed_ids = ["123456789"]

[channels.discord]
enabled  = true
token    = "$DISCORD_BOT_TOKEN"
```

### 權限

```toml
[permissions]
allowed_tools = ["bash", "edit", "view", "glob", "grep"]
allowed_dirs = ["**x"] # 所有目錄
```
