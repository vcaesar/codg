# Codg

<p align="left">
    <a href="https://github.com/vcaesar/codg/releases"><img src="https://img.shields.io/github/release/vcaesar/codg" alt="Última versão"></a>
    <a href="https://github.com/vcaesar/codg/actions"><img src="https://github.com/vcaesar/codg/actions/workflows/go.yml/badge.svg" alt="Status do build"></a>
    <a href="https://pkg.go.dev/github.com/vcaesar/codg?tab=doc"><img src="https://pkg.go.dev/badge/github.com/vcaesar/codg?status.svg" alt="GoDoc"></a>
    <a href="https://discord.gg/46DxmXR7"><img src="https://img.shields.io/discord/1484658282777018551.svg?logo=discord&logoColor=white&label=Discord&color=5865F2" alt="Entre no chat do Discord em https://discord.gg/46DxmXR7"></a>
</p>

O próximo sistema de agente de IA simples para código e trabalho: automático e assíncrono, com alta concorrência e desempenho, eficiente e preciso.

[English](../README.md) | [繁體中文](./README.zht.md) | [简体中文](./README.zh.md) | [日本語](./README.ja.md) | [한국어](./README.ko.md) | [Français](./README.fr.md) | [Deutsch](./README.de.md) | [Español](./README.es.md) | [Português](./README.pt.md) | [Русский](./README.ru.md) | [العربية](./README.ar.md)

<p align="center">
<a href="https://atomai.cc" rel="nofollow">
<img width="800" alt="Demo do Codg" src="https://github.com/vcaesar/codg/raw/main/demo/26-04.png" />
<img width="800" alt="Demo do Codg" src="https://github.com/vcaesar/codg/raw/main/demo/26-04-1.png" />
</a>
</p>

## Instalação

Mac e Linux:

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

Entre no diretório do seu projeto e execute `codg`.

# Recursos

- Sistema de agentes automático e assíncrono, com alta concorrência e desempenho, e baixo consumo de memória
- Suporte a múltiplos modelos e modelos locais via openai-compat ou claude-compat; funciona em qualquer terminal e SO
- Fácil de usar: TUI disponível em todos os lugares, próxima de uma GUI; versões Desktop e Web em BETA
- Clique ou use "/xxx" para alternar sessões; tudo é clicável na TUI
- Clique em "Modified Files" ou use "/diff" e "/diff git" para ver arquivos de diff na TUI, como no VSCode
- Autocompletar letras e frases curtas em inglês

App Desktop (BETA), Web (BETA), Claw (BETA)

## Reportar bugs:

Abra uma [Issue no Github](https://github.com/vcaesar/codg/issues)

## Como usamos seus dados:

Atualmente nenhum dado ou telemetria é coletado, e modelos 100% locais são suportados. Ao usar uma API, consulte as políticas do provedor correspondente.

# Comandos CLI

Use `codg -h` ou "/help" na TUI

```bash
codg auth/login               # Autenticar (Atom, OpenAI, GitHub...)
codg web                      # Iniciar a UI web na porta 4096
codg desktop                  # Lançar o aplicativo de desktop (Wails)
codg claw                     # Iniciar o agente de mensagens (Telegram/Discord/Slack)
codg gateway --private-only   # Iniciar o gateway protegido
codg models claude            # Listar modelos que correspondam a "claude"
codg runm start Qwen/Qwen3-8B-GGUF   # Iniciar um modelo local
codg runm download user/model # Baixar um modelo GGUF
codg plugin install repo/name # Instalar um plugin
codg plugin list              # Listar os plugins instalados
codg install repo/name        # Atalho para plugin install
codg mcp add myserver cmd     # Adicionar um servidor MCP
codg mcp list                 # Listar servidores MCP configurados
codg skill url add <url>      # Adicionar uma URL de origem de skill
codg themes set catppuccin    # Trocar de tema
# codg logs -f                # Acompanhar os logs da aplicação
codg toml                     # Mostrar toda a configuração
codg stats/s                  # Mostrar estatísticas de uso
codg dirs                     # Mostrar os caminhos dos diretórios de dados/config
codg projects                 # Listar diretórios de projetos rastreados
codg lite 2                   # Definir o nível do modo lite (0-4)
codg merge origin main        # Merge git seguro com backup v1/
codg migrate                  # Migrar a configuração do .claude/.opencode
codg vm build                 # Construir em uma VM remota
codg vm run -- make test      # Executar comando na VM
codg sandbox run -- ./test.sh # Executar no sandbox
codg sandbox status           # Checar disponibilidade do sandbox
codg update                   # Atualizar definições de provedores
```

## Exemplos de uso

### Não interativo (`codg run`)

```bash
# Canalizar entrada de outro comando.
cat errors.log | codg run "O que está causando esses erros?"
# Modo verboso (saída de depuração para stderr).
codg run -v "Debugar esta função"
```

### UI Web

```bash
# Modo somente API (sem frontend nem navegador).
codg web 0
```

### Gerenciamento de plugins

```bash
# Instalar um plugin a partir de um repositório Git.
codg install github.com/user/codg-xxx-auth
```

### Agentes e skills personalizados:

Copie xx_agent.md (.codg/agents/templates) ou SKILL.md (.codg/skills) para o diretório correspondente

# Sistema de configuração

Crie um `codg.toml` na raiz do projeto (ou `~/.codg/config/codg.toml` para configurações globais):

```toml
# codg.toml — Configuração mínima do projeto.
[options]
lite_mode = 0          # 0 = todos os agentes, 2 = conjunto enxuto padrão, 4 = agente único
locale    = "en"       # Idioma da UI: en, zh-CN, ja

[options.tui]
theme     = "catppuccin"
dark_mode = true
compact_mode = false
```

### Configuração do provedor

```toml
# Usar chave de API (suporta expansão $ENV_VAR).
[providers.anthropic]
api_key = "$ANTHROPIC_API_KEY"

# Usar OAuth (configurado via `codg auth`).
[providers.openai]
oauth = true

# Provedor personalizado / auto-hospedado.
[providers.local]
name     = "My Local LLM"
type     = "openai-compat"
base_url = "http://localhost:8080/v1"
api_key  = "not-needed"
```

### Personalização de agentes

```toml
# Forma curta: atribuir um tipo de modelo.
agents.coder = "large"
agents.task  = "small"

# Forma completa: ajustar um agente.
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
# Carregamento e download automáticos na TUI ou com codg skill
[option]
skill_urls = ["https://github.com/user/skills"]
```

### Modelos locais (llama.cpp)

```toml
[llama]
port     = 8090
host     = "127.0.0.1"
ctx_size = 32000
gpu      = "auto"          # auto, cuda, off
```

### Canais de mensagens

```toml
[channels.telegram]
enabled     = true
token       = "$TELEGRAM_BOT_TOKEN"
allowed_ids = ["123456789"]

[channels.discord]
enabled  = true
token    = "$DISCORD_BOT_TOKEN"
```

### Permissões

```toml
[permissions]
allowed_tools = ["bash", "edit", "view", "glob", "grep"]
allowed_dirs = ["**x"] # todos os diretórios
```
