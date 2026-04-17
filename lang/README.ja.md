# Codg

<p align="left">
    <a href="https://github.com/vcaesar/codg/releases"><img src="https://img.shields.io/github/release/vcaesar/codg" alt="最新リリース"></a>
    <a href="https://github.com/vcaesar/codg/actions"><img src="https://github.com/vcaesar/codg/actions/workflows/go.yml/badge.svg" alt="ビルド状況"></a>
    <a href="https://pkg.go.dev/github.com/vcaesar/codg?tab=doc"><img src="https://pkg.go.dev/badge/github.com/vcaesar/codg?status.svg" alt="GoDoc"></a>
    <a href="https://discord.gg/Dy5QZRbaND"><img src="https://img.shields.io/discord/1484658282777018551.svg?logo=discord&logoColor=white&label=Discord&color=5865F2" alt="Discord チャットに参加 https://discord.gg/Dy5QZRbaND"></a>
</p>

次世代のシンプルなコーディング・業務 AI エージェントシステム。自動かつ非同期、高い並行性とパフォーマンス、効率性と正確性を両立。

[English](../README.md) | [繁體中文](./README.zht.md) | [简体中文](./README.zh.md) | [日本語](./README.ja.md) | [한국어](./README.ko.md) | [Français](./README.fr.md) | [Deutsch](./README.de.md) | [Español](./README.es.md) | [Português](./README.pt.md) | [Русский](./README.ru.md) | [العربية](./README.ar.md)

<p align="center">
<a href="https://atomai.cc" rel="nofollow">
<img width="800" alt="Codg デモ" src="https://github.com/vcaesar/codg/raw/main/demo/26-04.png" />
<img width="800" alt="Codg デモ" src="https://github.com/vcaesar/codg/raw/main/demo/26-04-1.png" />
</a>
</p>

## インストール

Mac と Linux:

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

プロジェクトのディレクトリに移動して `codg` を実行します。

# 機能

- 自動かつ非同期、高並行・高性能なエージェントシステムで、メモリ使用量も少ない
- openai-compat または claude-compat によるマルチモデルおよびローカルモデル対応。あらゆるターミナルと OS をサポート
- 使いやすさ:TUI はどこでも GUI のように使え、デスクトップ版と Web 版は BETA 提供中
- クリックまたは「/xxx」でセッション切り替え、TUI 内のあらゆる場所をクリック可能
- 「Modified Files」をクリックするか「/diff」「/diff git」で VSCode のように差分ファイルを表示
- 英文字と短文の自動補完

デスクトップアプリ(BETA)、Web(BETA)、Claw(BETA)

## バグ報告:

[Github Issues](https://github.com/vcaesar/codg/issues) を開いてください

## データの取り扱い:

現在、データや計測情報は一切収集しておらず、100% ローカルモデルにも対応しています。API を利用する場合は、各プロバイダーのポリシーをご確認ください。

# CLI コマンド

`codg -h` または TUI で「/help」を使用:

```bash
codg auth/login               # 認証 (Atom、OpenAI、GitHub...)
codg web                      # ポート 4096 で Web UI を起動
codg desktop                  # デスクトップアプリを起動 (Wails)
codg claw                     # メッセージングエージェントを起動 (Telegram/Discord/Slack)
codg gateway --private-only   # セキュアゲートウェイを起動
codg models claude            # "claude" に一致するモデルを一覧表示
codg runm start Qwen/Qwen3-8B-GGUF   # ローカルモデルを起動
codg runm download user/model # GGUF モデルをダウンロード
codg plugin install repo/name # プラグインをインストール
codg plugin list              # インストール済みプラグインを一覧表示
codg install repo/name        # plugin install の省略形
codg mcp add myserver cmd     # MCP サーバーを追加
codg mcp list                 # 設定済み MCP サーバーを一覧表示
codg skill url add <url>      # スキルソース URL を追加
codg themes set catppuccin    # テーマを切り替え
# codg logs -f                # アプリケーションログを追跡
codg toml                     # 全設定を表示
codg stats/s                  # 利用統計を表示
codg dirs                     # データ/設定ディレクトリのパスを表示
codg projects                 # 追跡中のプロジェクトディレクトリを一覧表示
codg lite 2                   # ライトモードレベルを設定 (0-4)
codg merge origin main        # v1/ バックアップ付きの安全な git マージ
codg migrate                  # .claude/.opencode から設定を移行
codg vm build                 # リモート VM でビルド
codg vm run -- make test      # VM でコマンドを実行
codg sandbox run -- ./test.sh # サンドボックスで実行
codg sandbox status           # サンドボックスの利用可否を確認
codg update                   # プロバイダー定義を更新
```

## 使用例

### 非対話モード (`codg run`)

```bash
# 別のコマンドから入力をパイプ。
cat errors.log | codg run "これらのエラーの原因は?"
# 詳細モード (デバッグ出力は stderr へ)。
codg run -v "この関数をデバッグして"
```

### Web UI

```bash
# API のみモード (フロントエンド・ブラウザなし)。
codg web 0
```

### プラグイン管理

```bash
# Git リポジトリからプラグインをインストール。
codg install github.com/user/codg-xxx-auth
```

### カスタムエージェントとスキル:

xx_agent.md (.codg/agents/templates) または SKILL.md (.codg/skills) をディレクトリにコピー

# 設定システム

プロジェクトルートに `codg.toml` を作成 (またはグローバル設定用に `~/.codg/config/codg.toml`):

```toml
# codg.toml — 最小限のプロジェクト設定。
[options]
lite_mode = 0          # 0 = 全エージェント、2 = デフォルト軽量セット、4 = 単一エージェント
locale    = "en"       # UI 言語: en, zh-CN, ja

[options.tui]
theme     = "catppuccin"
dark_mode = true
compact_mode = false
```

### プロバイダー設定

```toml
# API キーを使用 ($ENV_VAR 展開対応)。
[providers.anthropic]
api_key = "$ANTHROPIC_API_KEY"

# OAuth を使用 (`codg auth` で設定)。
[providers.openai]
oauth = true

# カスタム/セルフホスト プロバイダー。
[providers.local]
name     = "My Local LLM"
type     = "openai-compat"
base_url = "http://localhost:8080/v1"
api_key  = "not-needed"
```

### エージェントのカスタマイズ

```toml
# 省略形: モデルタイプを指定。
agents.coder = "large"
agents.task  = "small"

# 完全形: エージェントを細かく調整。
[agents.advisor]
model           = "large"
temperature     = 0.3
thinking_budget = 32000
```

### MCP サーバー

```toml
# HTTP MCP サーバー。
[mcp.websearch]
type = "http"
url  = "https://mcp.exa.ai/mcp?tools=web_search_exa"
```

### スキル

```toml
# TUI または codg skill で自動読み込み・ダウンロード
[option]
skill_urls = ["https://github.com/user/skills"]
```

### ローカルモデル (llama.cpp)

```toml
[llama]
port     = 8090
host     = "127.0.0.1"
ctx_size = 32000
gpu      = "auto"          # auto, cuda, off
```

### メッセージングチャネル

```toml
[channels.telegram]
enabled     = true
token       = "$TELEGRAM_BOT_TOKEN"
allowed_ids = ["123456789"]

[channels.discord]
enabled  = true
token    = "$DISCORD_BOT_TOKEN"
```

### 権限

```toml
[permissions]
allowed_tools = ["bash", "edit", "view", "glob", "grep"]
allowed_dirs = ["**x"] # すべてのディレクトリ
```
