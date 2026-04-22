# Codg

<p align="left">
    <a href="https://github.com/vcaesar/codg/releases"><img src="https://img.shields.io/github/release/vcaesar/codg" alt="Neueste Version"></a>
    <a href="https://github.com/vcaesar/codg/actions"><img src="https://github.com/vcaesar/codg/actions/workflows/go.yml/badge.svg" alt="Build-Status"></a>
    <a href="https://pkg.go.dev/github.com/vcaesar/codg?tab=doc"><img src="https://pkg.go.dev/badge/github.com/vcaesar/codg?status.svg" alt="GoDoc"></a>
    <a href="https://discord.gg/Dy5QZRbaND"><img src="https://img.shields.io/discord/1484658282777018551.svg?logo=discord&logoColor=white&label=Discord&color=5865F2" alt="Discord-Chat beitreten unter https://discord.gg/Dy5QZRbaND"></a>
</p>

Das einfache Code- und Arbeits-KI-Agenten-Harness-System der nächsten Generation — automatisch und asynchron, hochparallel und leistungsstark, effizient und präzise.

[English](../README.md) | [繁體中文](./README.zht.md) | [简体中文](./README.zh.md) | [日本語](./README.ja.md) | [한국어](./README.ko.md) | [Français](./README.fr.md) | **Deutsch** | [Español](./README.es.md) | [Português](./README.pt.md) | [Русский](./README.ru.md) | [العربية](./README.ar.md)

<p align="center">
<a href="https://atomai.cc" rel="nofollow">
<img width="800" alt="Codg Demo" src="https://github.com/vcaesar/codg/raw/main/demo/26-04.png" />
<img width="800" alt="Codg Demo" src="https://github.com/vcaesar/codg/raw/main/demo/26-04-1.png" />
</a>
</p>

## Installation

Mac und Linux:

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

Oder klicken Sie direkt auf [Releases](https://github.com/vcaesar/codg/releases), um es herunterzuladen und auszuführen.

Wechseln Sie in Ihr Projektverzeichnis und führen Sie `codg` aus.
Mit „/yolo“ können Sie zwischen Auto- und Bestätigungsmodus wechseln; Berechtigungen lassen sich über codg.toml festlegen.

## Funktionen

- Automatisches und asynchrones, hochparalleles und leistungsstarkes Agentensystem mit geringem Speicherverbrauch
- Mehrere Modellanbieter (API und Pro) sowie lokale Modelle über openai-compat oder claude-compat; Unterstützung der kostenlosen Modelle von Openrouter, Ollama, Nvidia und weiteren; Nutzung via „/connect“, „/models“ oder „codg auth“
- Unterstützung für jedes Terminal und Betriebssystem, auch für Web-Terminals
- Benutzerfreundlich: TUI überall einsetzbar, GUI-nah; Desktop- und Web-Version im BETA-Stadium
- Klicken oder „/xxx" zum Sitzungswechsel, überall im TUI anklickbar
- Klick auf „Modified Files" oder „/diff" bzw. „/diff git" zeigt Diff-Dateien im TUI wie in VSCode
- Autovervollständigung englischer Buchstaben und kurzer Sätze

Desktop-App (BETA), Web (BETA), Claw (BETA); einige Funktionen müssen noch getestet und Bugs behoben werden, dann freigegeben.

## Benchmark

### RAM-Nutzung

| Tool                   | 1 aktive Sitzung | 10 aktive Sitzungen | Zusätzlicher PSS pro hinzugefügter Sitzung |
| ---------------------- | ---------------- | ------------------- | ------------------------------------------ |
| **Codg**               | 65 MB            | 165 MB              | ~10 MB                                     |
| **Codex CLI**          | 140.0 MB         | 334.8 MB            | ~21.6 MB                                   |
| **Cursor Agent**       | 214.9 MB         | 1632.4 MB           | ~157.5 MB                                  |
| **GitHub Copilot CLI** | 333.3 MB         | 1756.5 MB           | ~158.1 MB                                  |
| **OpenCode**           | 371.5 MB         | 3237.2 MB           | ~318.4 MB                                  |
| **Claude Code**        | 386.6 MB         | 2300.6 MB           | ~212.7 MB                                  |

## Fehler melden:

Erstellen Sie ein [Github Issue](https://github.com/vcaesar/codg/issues)

## Wie wir Ihre Daten verwenden:

Derzeit werden keinerlei Daten oder Telemetriedaten erhoben. 100 % lokale Modelle werden unterstützt. Bei API-Nutzung gelten die Richtlinien der jeweiligen Anbieter.

# CLI-Befehle

Verwenden Sie `codg -h` oder „/help" im TUI

```bash
codg auth/login               # Authentifizieren (Atom, OpenAI, GitHub...)
codg web                      # Web-UI auf Port 4096 starten
codg desktop                  # Desktop-App starten (Wails)
codg claw                     # Messaging-Agent starten (Telegram/Discord/Slack)
codg gateway --private-only   # Gesichertes Gateway starten
codg models claude            # Modelle filtern, die „claude" enthalten
codg runm start Qwen/Qwen3-8B-GGUF   # Lokales Modell starten
codg runm download user/model # GGUF-Modell herunterladen
codg plugin install repo/name # Plugin installieren
codg plugin list              # Installierte Plugins auflisten
codg install repo/name        # Kurzform für plugin install
codg mcp add myserver cmd     # MCP-Server hinzufügen
codg mcp list                 # Konfigurierte MCP-Server auflisten
codg skill url add <url>      # Skill-Quell-URL hinzufügen
codg themes set catppuccin    # Theme wechseln
# codg logs -f                # Anwendungs-Logs verfolgen
codg toml                     # Gesamte Konfiguration anzeigen
codg stats/s                  # Nutzungsstatistiken anzeigen
codg dirs                     # Daten-/Konfig-Verzeichnispfade anzeigen
codg projects                 # Verfolgte Projektverzeichnisse auflisten
codg lite 2                   # Lite-Modus-Stufe festlegen (0-4)
codg merge origin main        # Sicheres Git-Merge mit v1/-Backup
codg migrate                  # Konfiguration aus .claude/.opencode migrieren
codg vm build                 # Auf entfernter VM bauen
codg vm run -- make test      # Befehl in VM ausführen
codg sandbox run -- ./test.sh # In Sandbox ausführen
codg sandbox status           # Sandbox-Verfügbarkeit prüfen
codg update                   # Provider-Definitionen aktualisieren
```

## Anwendungsbeispiele

### Nicht interaktiv (`codg run`)

```bash
# Eingabe aus einem anderen Befehl piepen.
cat errors.log | codg run "Was verursacht diese Fehler?"
# Ausführlicher Modus (Debug-Ausgabe nach stderr).
codg run -v "Diese Funktion debuggen"
```

### Web-UI

```bash
# Web-UI auf Standardport 4096 starten; (nach Abschluss des Tests freigeben).
codg web
# Benutzerdefinierter Port.
codg web -p 8080

# Nur-API-Modus (kein Frontend, kein Browser).
codg web 0
```

### Plugin-Verwaltung

```bash
# Plugin aus einem Git-Repository installieren.
codg install github.com/user/codg-xxx-auth
```

### Eigene Agenten und Skills:

Kopieren Sie xx_agent.md (.codg/agents/templates) oder SKILL.md (.codg/skills) in das jeweilige Verzeichnis

# Konfigurationssystem

Erstellen Sie `codg.toml` im Projekt-Root (oder `~/.codg/config/codg.toml` für globale Einstellungen):

```toml
# codg.toml — Minimale Projektkonfiguration.
[options]
lite_mode = 0          # 0 = alle Agenten, 2 = Standard-Minimalsatz, 4 = einzelner Agent
locale    = "en"       # UI-Sprache: en, zh-CN, ja

[options.tui]
theme     = "catppuccin"
dark_mode = true
compact_mode = false
```

### Provider-Einrichtung

```toml
# API-Schlüssel verwenden (unterstützt $ENV_VAR-Erweiterung).
[providers.anthropic]
api_key = "$ANTHROPIC_API_KEY"

# OAuth verwenden (via `codg auth` einrichten).
[providers.openai]
oauth = true

# Eigener / selbst gehosteter Provider.
[providers.local]
name     = "My Local LLM"
type     = "openai-compat"
base_url = "http://localhost:8080/v1"
api_key  = "not-needed"
```

### Agenten-Anpassung

```toml
# Kurzform: Modelltyp zuweisen.
agents.coder = "large"
agents.task  = "small"

# Vollform: Agent feinjustieren.
[agents.advisor]
model           = "large"
temperature     = 0.3
thinking_budget = 32000
```

### MCP-Server

```toml
# HTTP-MCP-Server.
[mcp.websearch]
type = "http"
url  = "https://mcp.exa.ai/mcp?tools=web_search_exa"
```

### Skills

```toml
# Automatisches Laden und Herunterladen im TUI oder via codg skill
[option]
skill_urls = ["https://github.com/user/skills"]
```

### Lokale Modelle (llama.cpp)

```toml
[llama]
port     = 8090
host     = "127.0.0.1"
ctx_size = 32000
gpu      = "auto"          # auto, cuda, off
```

### Messaging-Kanäle

```toml
[channels.telegram]
enabled     = true
token       = "$TELEGRAM_BOT_TOKEN"
allowed_ids = ["123456789"]

[channels.discord]
enabled  = true
token    = "$DISCORD_BOT_TOKEN"
```

### Berechtigungen

```toml
[permissions]
allowed_tools = ["bash", "edit", "view", "glob", "grep"]
allowed_dirs = ["**x"] # alle Verzeichnisse
```
