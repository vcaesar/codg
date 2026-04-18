# Codg

<p align="left">
    <a href="https://github.com/vcaesar/codg/releases"><img src="https://img.shields.io/github/release/vcaesar/codg" alt="Последний релиз"></a>
    <a href="https://github.com/vcaesar/codg/actions"><img src="https://github.com/vcaesar/codg/actions/workflows/go.yml/badge.svg" alt="Статус сборки"></a>
    <a href="https://pkg.go.dev/github.com/vcaesar/codg?tab=doc"><img src="https://pkg.go.dev/badge/github.com/vcaesar/codg?status.svg" alt="GoDoc"></a>
    <a href="https://discord.gg/Dy5QZRbaND"><img src="https://img.shields.io/discord/1484658282777018551.svg?logo=discord&logoColor=white&label=Discord&color=5865F2" alt="Присоединяйтесь к чату Discord на https://discord.gg/Dy5QZRbaND"></a>
</p>

Следующее поколение простой AI-системы-харнесса агентов для кода и работы: автоматическая и асинхронная, с высокой конкурентностью и производительностью, эффективная и точная.

[English](../README.md) | [繁體中文](./README.zht.md) | [简体中文](./README.zh.md) | [日本語](./README.ja.md) | [한국어](./README.ko.md) | [Français](./README.fr.md) | [Deutsch](./README.de.md) | [Español](./README.es.md) | [Português](./README.pt.md) | [Русский](./README.ru.md) | [العربية](./README.ar.md)

<p align="center">
<a href="https://atomai.cc" rel="nofollow">
<img width="800" alt="Демо Codg" src="https://github.com/vcaesar/codg/raw/main/demo/26-04.png" />
<img width="800" alt="Демо Codg" src="https://github.com/vcaesar/codg/raw/main/demo/26-04-1.png" />
</a>
</p>

## Установка

Mac и Linux:

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

Или перейдите напрямую на страницу [Releases](https://github.com/vcaesar/codg/releases), чтобы скачать и запустить.

Перейдите в каталог проекта и выполните `codg`.
«/yolo» переключает автоматический режим и режим подтверждения; разрешения настраиваются в codg.toml.

# Возможности

- Автоматическая и асинхронная, высоко конкурентная и высокопроизводительная система агентов с низким потреблением памяти
- Поддержка многочисленных провайдеров моделей (API и Pro) и локальных моделей через openai-compat или claude-compat; поддержка бесплатных моделей Openrouter; настройка через «/connect», «/models» или «codg auth»
- Поддержка любого терминала и ОС, в том числе веб-терминалов
- Удобство: TUI доступен повсюду и близок к GUI; Desktop- и Web-версии находятся в стадии BETA
- Клик или «/xxx» для переключения сессий; всё в TUI кликабельно
- Клик по «Modified Files» или «/diff» и «/diff git» — просмотр diff-файлов в TUI как в VSCode
- Автодополнение английских букв и коротких фраз

Desktop-приложение (BETA), Web (BETA), Claw (BETA); некоторые функции ожидают тестирования и исправления ошибок

## Сообщение об ошибках:

Откройте [Github Issues](https://github.com/vcaesar/codg/issues)

## Как мы используем ваши данные:

В данный момент никакие данные и телеметрия не собираются, поддерживаются 100% локальные модели; при использовании API смотрите политику соответствующего провайдера.

# CLI-команды

Используйте `codg -h` или «/help» в TUI

```bash
codg auth/login               # Аутентификация (Atom, OpenAI, GitHub...)
codg web                      # Запустить web-UI на порту 4096
codg desktop                  # Запустить desktop-приложение (Wails)
codg claw                     # Запустить мессенджер-агента (Telegram/Discord/Slack)
codg gateway --private-only   # Запустить защищённый шлюз
codg models claude            # Список моделей, соответствующих «claude»
codg runm start Qwen/Qwen3-8B-GGUF   # Запустить локальную модель
codg runm download user/model # Скачать модель GGUF
codg plugin install repo/name # Установить плагин
codg plugin list              # Список установленных плагинов
codg install repo/name        # Сокращённо для plugin install
codg mcp add myserver cmd     # Добавить MCP-сервер
codg mcp list                 # Список настроенных MCP-серверов
codg skill url add <url>      # Добавить URL источника навыков
codg themes set catppuccin    # Переключить тему
# codg logs -f                # Отслеживать логи приложения
codg toml                     # Показать всю конфигурацию
codg stats/s                  # Показать статистику использования
codg dirs                     # Вывести пути каталогов данных/конфига
codg projects                 # Список отслеживаемых каталогов проектов
codg lite 2                   # Установить уровень lite-режима (0-4)
codg merge origin main        # Безопасный git merge с бэкапом v1/
codg migrate                  # Миграция конфига из .claude/.opencode
codg vm build                 # Сборка на удалённой VM
codg vm run -- make test      # Выполнить команду на VM
codg sandbox run -- ./test.sh # Запуск в песочнице
codg sandbox status           # Проверить доступность песочницы
codg update                   # Обновить определения провайдеров
```

## Примеры использования

### Неинтерактивный режим (`codg run`)

```bash
# Передать ввод от другой команды по конвейеру.
cat errors.log | codg run "Что вызывает эти ошибки?"
# Подробный режим (отладочный вывод в stderr).
codg run -v "Отладить эту функцию"
```

### Web UI

```bash
# Запустить web-UI на порту по умолчанию 4096; (после тестов — собрать).
codg web
# Пользовательский порт.
codg web -p 8080

# Режим только API (без фронтенда и браузера).
codg web 0
```

### Управление плагинами

```bash
# Установить плагин из Git-репозитория.
codg install github.com/user/codg-xxx-auth
```

### Пользовательские агенты и навыки:

Скопируйте xx_agent.md (.codg/agents/templates) или SKILL.md (.codg/skills) в соответствующий каталог

# Система конфигурации

Создайте `codg.toml` в корне проекта (или `~/.codg/config/codg.toml` для глобальных настроек):

```toml
# codg.toml — Минимальная конфигурация проекта.
[options]
lite_mode = 0          # 0 = все агенты, 2 = стандартный облегчённый набор, 4 = единственный агент
locale    = "en"       # Язык UI: en, zh-CN, ja

[options.tui]
theme     = "catppuccin"
dark_mode = true
compact_mode = false
```

### Настройка провайдера

```toml
# Использовать API-ключ (поддерживает развёртывание $ENV_VAR).
[providers.anthropic]
api_key = "$ANTHROPIC_API_KEY"

# Использовать OAuth (настраивается через `codg auth`).
[providers.openai]
oauth = true

# Пользовательский / самостоятельно размещаемый провайдер.
[providers.local]
name     = "My Local LLM"
type     = "openai-compat"
base_url = "http://localhost:8080/v1"
api_key  = "not-needed"
```

### Настройка агентов

```toml
# Краткая форма: назначить тип модели.
agents.coder = "large"
agents.task  = "small"

# Полная форма: тонкая настройка агента.
[agents.advisor]
model           = "large"
temperature     = 0.3
thinking_budget = 32000
```

### MCP-серверы

```toml
# HTTP MCP-сервер.
[mcp.websearch]
type = "http"
url  = "https://mcp.exa.ai/mcp?tools=web_search_exa"
```

### Навыки

```toml
# Автозагрузка и скачивание в TUI или через codg skill
[option]
skill_urls = ["https://github.com/user/skills"]
```

### Локальные модели (llama.cpp)

```toml
[llama]
port     = 8090
host     = "127.0.0.1"
ctx_size = 32000
gpu      = "auto"          # auto, cuda, off
```

### Каналы сообщений

```toml
[channels.telegram]
enabled     = true
token       = "$TELEGRAM_BOT_TOKEN"
allowed_ids = ["123456789"]

[channels.discord]
enabled  = true
token    = "$DISCORD_BOT_TOKEN"
```

### Разрешения

```toml
[permissions]
allowed_tools = ["bash", "edit", "view", "glob", "grep"]
allowed_dirs = ["**x"] # все каталоги
```
