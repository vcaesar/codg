# Codg

<p align="left">
    <a href="https://github.com/vcaesar/codg/releases"><img src="https://img.shields.io/github/release/vcaesar/codg" alt="Latest Release"></a>
    <a href="https://github.com/vcaesar/codg/actions"><img src="https://github.com/vcaesar/codg/actions/workflows/go.yml/badge.svg" alt="Build Status"></a>
    <a href="https://pkg.go.dev/github.com/vcaesar/codg?tab=doc"><img src="https://pkg.go.dev/badge/github.com/vcaesar/codg?status.svg" alt="GoDoc"></a>
   <Join href ="https://discord.gg/codg"><img src ="https://img.shields.io/discord/1484658282777018551.svg?logo=discord&logoColor=white&label=Discord&color=5865F2" alt="Join the Discord chat at https://discord.gg/codg")
</p>

The next work and code AI agent, auto and asynchronous, concurrency and high performance.

[Englsih](README.md) | [繁體中文](./lang/README.zh-tw.md) | [简体中文](./lang/README.zh-cn.md)

<p align="center">
<a href="https://atomai.cc" rel="nofollow">
<img width="800" alt="Codg Demo" src="https://github.com/vcaesar/codg/raw/main/demo/26-04.png" />
<img width="800" alt="Codg Demo" src="https://github.com/vcaesar/codg/raw/main/demo/26-04-1.png" />
</a>
</p>

## Installation

Mac and Linux:

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

Go to your project directory, run `codg`.

# Features

- Auto and asynchronous, concurrency and high performance agent system, and low memory use
- Multi models and local models via by openai-compat or claude-compat, any terminal and OS support
- Easy use: The TUI in everywhere like GUI and Easy, Desktop and Web in the BETA
- Click or "/xxx" to switch sessions, clcik or "/diff" to view the diff files same the vscode

Desktop App (BETA), Web (BETA), Claw (BETA)

## Reporting Bugs:

Open a [Github Issues](https://github.com/vcaesar/codg/issues)

## How we use your data:

Currently no any data and telemetry is collected here, and 100% local model supported, use the API you can see they providers' policies.

# CLI Commands

Use: codg -h or "/help" in TUI

```bash
codg auth/login               # Authenticate (Atom, OpenAI, GitHub...)
codg web                      # Start web UI on port 4096
codg desktop                  # Launch the desktop app (Wails)
codg claw                     # Start messaging agent (Telegram/Discord/Slack)
codg gateway --private-only   # Start secured gateway
codg models claude            # List models matching "claude"
codg runm start Qwen/Qwen3-8B-GGUF   # Start a local model
codg runm download user/model # Download a GGUF model
codg plugin install repo/name # Install a plugin
codg plugin list              # List installed plugins
codg install repo/name        # Shorthand for plugin install
codg mcp add myserver cmd     # Add an MCP server
codg mcp list                 # List configured MCP servers
codg skill url add <url>      # Add a skill source URL
codg themes set catppuccin    # Switch theme
# codg logs -f                # Tail application logs
codg toml                     # show the all config
codg stats/s                  # Show usage statistics
codg dirs                     # Print data/config directory paths
codg projects                 # List tracked project directories
codg lite 2                   # Set lite mode level (0-4)
codg merge origin main        # Safe git merge with v1/ backup
codg migrate                  # Migrate config from .claude/.opencode
codg vm build                 # Build on remote VM
codg vm run -- make test      # Execute command on VM
codg sandbox run -- ./test.sh # Run in sandbox
codg sandbox status           # Check sandbox availability
codg update                   # Update provider definitions
```

## Usage Examples

### Non-Interactive (`codg run`)

```bash
# Pipe input from another command.
cat errors.log | codg run "What's causing these errors?"
# Verbose mode (debug output to stderr).
codg run -v "Debug this function"
```

### Web UI

```bash
# API-only mode (no frontend, no browser).
codg web 0
```

### Plugin Management

```bash
# Install a plugin from a Git repository.
codg install github.com/user/codg-xxx-auth
```

### Custom Agents and Skills:

Copy xx_agent.md (.codg/agents/templates) or SKILL.md (.codg/skills) to the directory

# Configuration System

Create a `codg.toml` in your project root (or `~/.codg/config/codg.toml`
for global settings):

```toml
# codg.toml — Minimal project config.
[options]
lite_mode = 0          # 0 = all agents, 2 = default lean set, 4 = single agent
locale    = "en"       # UI language: en, zh-CN, ja

[options.tui]
theme     = "catppuccin"
dark_mode = true
compact_mode = false
```

### Provider Setup

```toml
# Use an API key (supports $ENV_VAR expansion).
[providers.anthropic]
api_key = "$ANTHROPIC_API_KEY"

# Use OAuth (set via `codg auth`).
[providers.openai]
oauth = true

# Custom / self-hosted provider.
[providers.local]
name     = "My Local LLM"
type     = "openai-compat"
base_url = "http://localhost:8080/v1"
api_key  = "not-needed"
```

### Agent Customization

```toml
# Shorthand: assign a model type.
agents.coder = "large"
agents.task  = "small"

# Full form: fine-tune an agent.
[agents.advisor]
model           = "large"
temperature     = 0.3
thinking_budget = 32000
```

### MCP Servers

```toml
# HTTP MCP server.
[mcp.websearch]
type = "http"
url  = "https://mcp.exa.ai/mcp?tools=web_search_exa"
```

### Skills

```toml
# Auto load and download in TUI or codg skill
[option]
skill_urls = ["https://github.com/user/skills"]
```

### Local Models (llama.cpp)

```toml
[llama]
port     = 8090
host     = "127.0.0.1"
ctx_size = 32000
gpu      = "auto"          # auto, cuda, off
```

### Messaging Channels

```toml
[channels.telegram]
enabled     = true
token       = "$TELEGRAM_BOT_TOKEN"
allowed_ids = ["123456789"]

[channels.discord]
enabled  = true
token    = "$DISCORD_BOT_TOKEN"
```

### Permissions

```toml
[permissions]
allowed_tools = ["bash", "edit", "view", "glob", "grep"]
allowed_dirs = ["**x"] # all directories
```
