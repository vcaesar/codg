# Codg

<p align="left">
    <a href="https://github.com/vcaesar/codg/releases"><img src="https://img.shields.io/github/release/vcaesar/codg" alt="最新版本"></a>
    <a href="https://github.com/vcaesar/codg/actions"><img src="https://github.com/vcaesar/codg/actions/workflows/go.yml/badge.svg" alt="构建状态"></a>
    <a href="https://pkg.go.dev/github.com/vcaesar/codg?tab=doc"><img src="https://pkg.go.dev/badge/github.com/vcaesar/codg?status.svg" alt="GoDoc"></a>
    <a href="https://discord.gg/Dy5QZRbaND"><img src="https://img.shields.io/discord/1484658282777018551.svg?logo=discord&logoColor=white&label=Discord&color=5865F2" alt="加入 Discord 聊天 https://discord.gg/Dy5QZRbaND"></a>
</p>

下一代简易的代码与工作 AI 智能体系统,自动化、异步化、高并发、高性能,高效且高精度。

[English](../README.md) | [繁體中文](./README.zht.md) | [简体中文](./README.zh.md) | [日本語](./README.ja.md) | [한국어](./README.ko.md) | [Français](./README.fr.md) | [Deutsch](./README.de.md) | [Español](./README.es.md) | [Português](./README.pt.md) | [Русский](./README.ru.md) | [العربية](./README.ar.md)

<p align="center">
<a href="https://atomai.cc" rel="nofollow">
<img width="800" alt="Codg 演示" src="https://github.com/vcaesar/codg/raw/main/demo/26-04.png" />
<img width="800" alt="Codg 演示" src="https://github.com/vcaesar/codg/raw/main/demo/26-04-1.png" />
</a>
</p>

## 安装

Mac 和 Linux:

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

或者直接点击 [Releases](https://github.com/vcaesar/codg/releases) 下载并运行。

进入你的项目目录,运行 `codg`。

# 功能特性

- 自动化、异步化、高并发、高性能的智能体系统,内存占用低
- 多模型提供商(API 与 Pro)及本地模型(通过 openai-compat 或 claude-compat),支持 Openrouter 免费模型
- 支持任何终端和操作系统,同时支持 Web 终端
- 易用性:TUI 随处可用,体验接近 GUI,桌面版和 Web 版处于 BETA 阶段
- 点击或输入 "/xxx" 切换会话,TUI 中任意位置可点击
- 点击 "Modified Files" 或输入 "/diff"、"/diff git" 在 TUI 中查看差异文件,体验与 VSCode 相似
- 自动补全英文字母和短句

桌面应用(BETA)、Web(BETA)、Claw(BETA),部分功能仍需等待测试与修复 bug

## 报告 Bug:

请提交 [Github Issues](https://github.com/vcaesar/codg/issues)

## 我们如何使用你的数据:

目前不收集任何数据和遥测信息,并支持 100% 本地模型;使用 API 时请参考对应服务商的隐私政策。

# CLI 命令

使用 `codg -h` 或在 TUI 中输入 "/help"

```bash
codg auth/login               # 登录认证 (Atom、OpenAI、GitHub...)
codg web                      # 在 4096 端口启动 Web UI
codg desktop                  # 启动桌面应用 (Wails)
codg claw                     # 启动消息智能体 (Telegram/Discord/Slack)
codg gateway --private-only   # 启动受保护的网关
codg models claude            # 列出匹配 "claude" 的模型
codg runm start Qwen/Qwen3-8B-GGUF   # 启动本地模型
codg runm download user/model # 下载 GGUF 模型
codg plugin install repo/name # 安装插件
codg plugin list              # 列出已安装插件
codg install repo/name        # plugin install 的简写形式
codg mcp add myserver cmd     # 添加 MCP 服务器
codg mcp list                 # 列出已配置的 MCP 服务器
codg skill url add <url>      # 添加技能源 URL
codg themes set catppuccin    # 切换主题
# codg logs -f                # 查看应用日志
codg toml                     # 显示全部配置
codg stats/s                  # 显示使用统计
codg dirs                     # 打印数据/配置目录路径
codg projects                 # 列出跟踪的项目目录
codg lite 2                   # 设置精简模式等级 (0-4)
codg merge origin main        # 带 v1/ 备份的安全 git 合并
codg migrate                  # 从 .claude/.opencode 迁移配置
codg vm build                 # 在远程 VM 上构建
codg vm run -- make test      # 在 VM 上执行命令
codg sandbox run -- ./test.sh # 在沙箱中运行
codg sandbox status           # 查看沙箱可用性
codg update                   # 更新服务商定义
```

## 使用示例

### 非交互模式 (`codg run`)

```bash
# 通过管道传入另一个命令的输出。
cat errors.log | codg run "这些错误是什么原因?"
# 详细模式 (调试信息输出到 stderr)。
codg run -v "调试这个函数"
```

### Web UI

```bash
# 在默认端口 4096 启动 Web UI;(等待测试完成后构建)。
codg web
# 自定义端口。
codg web -p 8080

# 仅 API 模式 (无前端、无浏览器)。
codg web 0
```

### 插件管理

```bash
# 从 Git 仓库安装插件。
codg install github.com/user/codg-xxx-auth
```

### 自定义智能体和技能:

将 xx_agent.md (.codg/agents/templates) 或 SKILL.md (.codg/skills) 复制到对应目录

# 配置系统

在项目根目录创建 `codg.toml`(或 `~/.codg/config/codg.toml` 用于全局设置):

```toml
# codg.toml — 最小化项目配置。
[options]
lite_mode = 0          # 0 = 全部智能体,2 = 默认精简集,4 = 单智能体
locale    = "en"       # UI 语言:en、zh-CN、ja

[options.tui]
theme     = "catppuccin"
dark_mode = true
compact_mode = false
```

### 服务商设置

```toml
# 使用 API 密钥 (支持 $ENV_VAR 展开)。
[providers.anthropic]
api_key = "$ANTHROPIC_API_KEY"

# 使用 OAuth (通过 `codg auth` 设置)。
[providers.openai]
oauth = true

# 自定义/自部署服务商。
[providers.local]
name     = "My Local LLM"
type     = "openai-compat"
base_url = "http://localhost:8080/v1"
api_key  = "not-needed"
```

### 智能体自定义

```toml
# 简写形式:指定模型类型。
agents.coder = "large"
agents.task  = "small"

# 完整形式:精细调整智能体。
[agents.advisor]
model           = "large"
temperature     = 0.3
thinking_budget = 32000
```

### MCP 服务器

```toml
# HTTP MCP 服务器。
[mcp.websearch]
type = "http"
url  = "https://mcp.exa.ai/mcp?tools=web_search_exa"
```

### 技能

```toml
# 在 TUI 或 codg skill 中自动加载与下载
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

### 消息渠道

```toml
[channels.telegram]
enabled     = true
token       = "$TELEGRAM_BOT_TOKEN"
allowed_ids = ["123456789"]

[channels.discord]
enabled  = true
token    = "$DISCORD_BOT_TOKEN"
```

### 权限

```toml
[permissions]
allowed_tools = ["bash", "edit", "view", "glob", "grep"]
allowed_dirs = ["**x"] # 所有目录
```
