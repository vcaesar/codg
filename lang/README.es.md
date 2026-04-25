# Codg

<p align="left">
    <a href="https://github.com/vcaesar/codg/releases"><img src="https://img.shields.io/github/release/vcaesar/codg" alt="Última versión"></a>
    <a href="https://github.com/vcaesar/codg/actions"><img src="https://github.com/vcaesar/codg/actions/workflows/go.yml/badge.svg" alt="Estado del build"></a>
    <a href="https://pkg.go.dev/github.com/vcaesar/codg?tab=doc"><img src="https://pkg.go.dev/badge/github.com/vcaesar/codg?status.svg" alt="GoDoc"></a>
    <a href="https://discord.gg/Dy5QZRbaND"><img src="https://img.shields.io/discord/1484658282777018551.svg?logo=discord&logoColor=white&label=Discord&color=5865F2" alt="Únete al chat de Discord en https://discord.gg/Dy5QZRbaND"></a>
</p>

El sistema de orquestación de agentes de IA de nueva generación, sencillo para código y trabajo: automático y asíncrono, con alta concurrencia y rendimiento, eficiente y preciso.

[English](../README.md) | [繁體中文](./README.zht.md) | [简体中文](./README.zh.md) | [日本語](./README.ja.md) | [한국어](./README.ko.md) | [Français](./README.fr.md) | [Deutsch](./README.de.md) | **Español** | [Português](./README.pt.md) | [Русский](./README.ru.md) | [العربية](./README.ar.md)

<p align="center">
<a href="https://atomai.cc" rel="nofollow">
<img width="800" alt="Demo de Codg" src="https://github.com/vcaesar/codg/raw/main/demo/26-04.png" />
<img width="800" alt="Demo de Codg" src="https://github.com/vcaesar/codg/raw/main/demo/26-04-1.png" />
</a>
</p>

## Instalación

Mac y Linux:

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

# YOLO (instalador nativo de PowerShell)
irm https://raw.githubusercontent.com/vcaesar/codg/main/demo/boot.ps1 | iex
```

Todos (macOS, Linux o Windows con Git Bash / MSYS2 / Cygwin / WSL):

```bash
# YOLO
curl -fsSL https://raw.githubusercontent.com/vcaesar/codg/main/demo/boot.sh | bash
```

O haz clic directamente en [Releases](https://github.com/vcaesar/codg/releases) para descargarlo y ejecutarlo.

Ve al directorio de tu proyecto y ejecuta `codg`.
Usa «/yolo» para alternar entre el modo automático y el modo de confirmación; los permisos se configuran desde codg.toml.

## Características

- Sistema de agentes automático y asíncrono, con alta concurrencia y rendimiento, y bajo uso de memoria
- Proveedores multi-modelo (40+ proveedores API y Pro, Custom URL API) y modelos locales vía openai-compat o claude-compat; soporte para los modelos gratuitos de Openrouter, Ollama, Nvidia y otros; úsalos con «/connect», «/models» o «codg auth»
- Compatible con cualquier terminal y SO, también con terminales web
- Fácil de usar: TUI disponible en todas partes, cercana a una GUI; versiones Desktop y Web en BETA
- Haz clic o usa «/xxx» para cambiar de sesión; todo es cliqueable en la TUI
- Haz clic en «Modified Files» o usa «/diff» y «/diff git» para ver los archivos de diff en la TUI, como en VSCode
- Autocompletado de letras y frases cortas en inglés

App Desktop (BETA), Web (BETA), Claw (BETA); algunas funciones aún requieren pruebas y corrección de errores antes de publicarla.

## Benchmark

### Uso de RAM

| Herramienta            | 1 sesión activa | 10 sesiones activas | PSS adicional por sesión añadida |
| ---------------------- | --------------- | ------------------- | -------------------------------- |
| **Codg**               | 65 MB           | 165 MB              | ~10 MB                           |
| **Codex CLI**          | 140.0 MB        | 334.8 MB            | ~21.6 MB                         |
| **Cursor Agent**       | 214.9 MB        | 1632.4 MB           | ~157.5 MB                        |
| **GitHub Copilot CLI** | 333.3 MB        | 1756.5 MB           | ~158.1 MB                        |
| **OpenCode**           | 371.5 MB        | 3237.2 MB           | ~318.4 MB                        |
| **Claude Code**        | 386.6 MB        | 2300.6 MB           | ~212.7 MB                        |

## Reportar bugs:

Abre un [Issue de Github](https://github.com/vcaesar/codg/issues)

## Cómo usamos tus datos:

Actualmente no se recopilan datos ni telemetría, y se admiten modelos 100% locales. Si usas una API, consulta las políticas del proveedor correspondiente.

# Comandos CLI

Usa `codg -h` o «/help» en la TUI

```bash
codg auth/login               # Autenticarse (Atom, OpenAI, GitHub...)
codg web                      # Iniciar la UI web en el puerto 4096
codg desktop                  # Lanzar la app de escritorio (Wails)
codg claw                     # Iniciar el agente de mensajería (Telegram/Discord/Slack)
codg gateway --private-only   # Iniciar el gateway protegido
codg models claude            # Listar modelos que coincidan con «claude»
codg runm start Qwen/Qwen3-8B-GGUF   # Iniciar un modelo local
codg runm download user/model # Descargar un modelo GGUF
codg plugin install repo/name # Instalar un plugin
codg plugin list              # Listar los plugins instalados
codg install repo/name        # Atajo para plugin install
codg mcp add myserver cmd     # Añadir un servidor MCP
codg mcp list                 # Listar servidores MCP configurados
codg skill url add <url>      # Añadir una URL de origen de skill
codg themes set catppuccin    # Cambiar de tema
# codg logs -f                # Ver los logs de la aplicación
codg toml                     # Mostrar toda la configuración
codg stats/s                  # Mostrar estadísticas de uso
codg dirs                     # Mostrar rutas de directorios de datos/config
codg projects                 # Listar los directorios de proyectos rastreados
codg lite 2                   # Definir el nivel del modo lite (0-4)
codg merge origin main        # Merge git seguro con backup v1/
codg migrate                  # Migrar la configuración desde .claude/.opencode
codg vm build                 # Compilar en una VM remota
codg vm run -- make test      # Ejecutar un comando en la VM
codg sandbox run -- ./test.sh # Ejecutar en el sandbox
codg sandbox status           # Comprobar la disponibilidad del sandbox
codg update                   # Actualizar las definiciones de proveedores
```

## Ejemplos de uso

### No interactivo (`codg run`)

```bash
# Canalizar la entrada desde otro comando.
cat errors.log | codg run "¿Qué está causando estos errores?"
# Modo detallado (salida de depuración a stderr).
codg run -v "Depurar esta función"
```

### UI Web

```bash
# Iniciar la UI web en el puerto predeterminado 4096; (esperar a las pruebas y luego publicarla).
codg web
# Puerto personalizado.
codg web -p 8080

# Modo solo API (sin frontend ni navegador).
codg web 0
```

### Gestión de plugins

```bash
# Instalar un plugin desde un repositorio Git.
codg install github.com/user/codg-xxx-auth
```

### Agentes y skills personalizados:

Copia xx_agent.md (.codg/agents/templates) o SKILL.md (.codg/skills) al directorio correspondiente

# Sistema de configuración

Crea un `codg.toml` en la raíz de tu proyecto (o `~/.codg/config/codg.toml` para configuración global):

```toml
# codg.toml — Configuración mínima del proyecto.
[options]
lite_mode = 0          # 0 = todos los agentes, 2 = conjunto ligero por defecto, 4 = único agente
locale    = "en"       # Idioma UI: en, zh-CN, ja

[options.tui]
theme     = "catppuccin"
dark_mode = true
compact_mode = false
```

### Configuración de proveedor

```toml
# Usar una clave API (admite expansión $ENV_VAR).
[providers.anthropic]
api_key = "$ANTHROPIC_API_KEY"

# Usar OAuth (configurado vía `codg auth`).
[providers.openai]
oauth = true

# Proveedor personalizado / autoalojado.
[providers.local]
name     = "My Local LLM"
type     = "openai-compat"
base_url = "http://localhost:8080/v1"
api_key  = "not-needed"
```

### Personalización de agentes

```toml
# Forma corta: asignar un tipo de modelo.
agents.coder = "large"
agents.task  = "small"

# Forma completa: ajustar un agente.
[agents.advisor]
model           = "large"
temperature     = 0.3
thinking_budget = 32000
```

### Servidores MCP

```toml
# Servidor MCP HTTP.
[mcp.websearch]
type = "http"
url  = "https://mcp.exa.ai/mcp?tools=web_search_exa"
```

### Skills

```toml
# Carga y descarga automática en la TUI o con codg skill
[option]
skill_urls = ["https://github.com/user/skills"]
```

### Modelos locales (llama.cpp)

```toml
[llama]
port     = 8090
host     = "127.0.0.1"
ctx_size = 32000
gpu      = "auto"          # auto, cuda, off
```

### Canales de mensajería

```toml
[channels.telegram]
enabled     = true
token       = "$TELEGRAM_BOT_TOKEN"
allowed_ids = ["123456789"]

[channels.discord]
enabled  = true
token    = "$DISCORD_BOT_TOKEN"
```

### Permisos

```toml
[permissions]
allowed_tools = ["bash", "edit", "view", "glob", "grep"]
allowed_dirs = ["**x"] # todos los directorios
```
