# Codg

<p align="left">
    <a href="https://github.com/vcaesar/codg/releases"><img src="https://img.shields.io/github/release/vcaesar/codg" alt="Dernière version"></a>
    <a href="https://github.com/vcaesar/codg/actions"><img src="https://github.com/vcaesar/codg/actions/workflows/go.yml/badge.svg" alt="Statut de build"></a>
    <a href="https://pkg.go.dev/github.com/vcaesar/codg?tab=doc"><img src="https://pkg.go.dev/badge/github.com/vcaesar/codg?status.svg" alt="GoDoc"></a>
    <a href="https://discord.gg/Dy5QZRbaND"><img src="https://img.shields.io/discord/1484658282777018551.svg?logo=discord&logoColor=white&label=Discord&color=5865F2" alt="Rejoignez le chat Discord sur https://discord.gg/Dy5QZRbaND"></a>
</p>

Le prochain système de harnais d'agents IA simple pour le code et le travail : automatique et asynchrone, haute concurrence et haute performance, efficace et précis.

[English](../README.md) | [繁體中文](./README.zht.md) | [简体中文](./README.zh.md) | [日本語](./README.ja.md) | [한국어](./README.ko.md) | [Français](./README.fr.md) | [Deutsch](./README.de.md) | [Español](./README.es.md) | [Português](./README.pt.md) | [Русский](./README.ru.md) | [العربية](./README.ar.md)

<p align="center">
<a href="https://atomai.cc" rel="nofollow">
<img width="800" alt="Démo Codg" src="https://github.com/vcaesar/codg/raw/main/demo/26-04.png" />
<img width="800" alt="Démo Codg" src="https://github.com/vcaesar/codg/raw/main/demo/26-04-1.png" />
</a>
</p>

## Installation

Mac et Linux :

```bash
# Homebrew
brew install vcaesar/tap/codg

# NPM
# npm install -g @vcaesar/codg
```

Windows :

```bash
# Winget
# winget install vcaesar.codg
```

```bash
# YOLO
curl -fsSL https://raw.githubusercontent.com/vcaesar/codg/main/demo/boot.sh | bash
```

Ou cliquez directement sur [Releases](https://github.com/vcaesar/codg/releases) pour le télécharger et l'exécuter.

Rendez-vous dans votre répertoire de projet et exécutez `codg`.
Utilisez « /yolo » pour basculer entre le mode automatique et le mode de confirmation ; les permissions peuvent être configurées via codg.toml.

# Fonctionnalités

- Système d'agents automatique et asynchrone, à haute concurrence et haute performance, avec faible consommation mémoire
- Fournisseurs multi-modèles (API et Pro) et modèles locaux via openai-compat ou claude-compat, prise en charge des modèles gratuits d'Openrouter, via « /connect », « /models » ou « codg auth »
- Compatible avec tout terminal et OS, y compris les terminaux web
- Facile à utiliser : TUI disponible partout, proche d'une GUI ; versions Desktop et Web en BETA
- Cliquez ou utilisez « /xxx » pour changer de session, tout est cliquable dans le TUI
- Cliquez sur « Modified Files » ou utilisez « /diff » et « /diff git » pour visualiser les diffs dans le TUI, comme dans VSCode
- Autocomplétion des lettres anglaises et phrases courtes

Application Desktop (BETA), Web (BETA), Claw (BETA), certaines fonctionnalités nécessitent encore des tests et des corrections de bugs avant sa publication.

## Signaler un bug :

Ouvrez un [Github Issue](https://github.com/vcaesar/codg/issues)

## Utilisation de vos données :

Aucune donnée ni télémétrie n'est collectée ici, et les modèles 100 % locaux sont pris en charge ; si vous utilisez une API, consultez les politiques du fournisseur correspondant.

# Commandes CLI

Utilisez : `codg -h` ou « /help » dans le TUI

```bash
codg auth/login               # S'authentifier (Atom, OpenAI, GitHub...)
codg web                      # Démarrer l'interface web sur le port 4096
codg desktop                  # Lancer l'application Desktop (Wails)
codg claw                     # Démarrer l'agent de messagerie (Telegram/Discord/Slack)
codg gateway --private-only   # Démarrer une passerelle sécurisée
codg models claude            # Lister les modèles correspondant à « claude »
codg runm start Qwen/Qwen3-8B-GGUF   # Démarrer un modèle local
codg runm download user/model # Télécharger un modèle GGUF
codg plugin install repo/name # Installer un plugin
codg plugin list              # Lister les plugins installés
codg install repo/name        # Raccourci pour plugin install
codg mcp add myserver cmd     # Ajouter un serveur MCP
codg mcp list                 # Lister les serveurs MCP configurés
codg skill url add <url>      # Ajouter une URL de source de skill
codg themes set catppuccin    # Changer de thème
# codg logs -f                # Suivre les logs de l'application
codg toml                     # Afficher toute la configuration
codg stats/s                  # Afficher les statistiques d'utilisation
codg dirs                     # Afficher les chemins des dossiers de données/config
codg projects                 # Lister les répertoires de projets suivis
codg lite 2                   # Définir le niveau du mode lite (0-4)
codg merge origin main        # Merge git sûr avec sauvegarde v1/
codg migrate                  # Migrer la configuration depuis .claude/.opencode
codg vm build                 # Builder sur une VM distante
codg vm run -- make test      # Exécuter une commande sur la VM
codg sandbox run -- ./test.sh # Exécuter en sandbox
codg sandbox status           # Vérifier la disponibilité de la sandbox
codg update                   # Mettre à jour les définitions de fournisseurs
```

## Exemples d'utilisation

### Non interactif (`codg run`)

```bash
# Récupérer l'entrée d'une autre commande.
cat errors.log | codg run "Quelle est la cause de ces erreurs ?"
# Mode verbeux (sortie de debug vers stderr).
codg run -v "Débugger cette fonction"
```

### Interface Web

```bash
# Démarrer l'interface web sur le port par défaut 4096 ; (une fois les tests terminés, la publier).
codg web
# Port personnalisé.
codg web -p 8080

# Mode API uniquement (sans frontend, sans navigateur).
codg web 0
```

### Gestion des plugins

```bash
# Installer un plugin depuis un dépôt Git.
codg install github.com/user/codg-xxx-auth
```

### Agents et skills personnalisés :

Copiez xx_agent.md (.codg/agents/templates) ou SKILL.md (.codg/skills) dans le répertoire approprié

# Système de configuration

Créez un `codg.toml` à la racine de votre projet (ou `~/.codg/config/codg.toml` pour la configuration globale) :

```toml
# codg.toml — Configuration projet minimale.
[options]
lite_mode = 0          # 0 = tous les agents, 2 = ensemble léger par défaut, 4 = agent unique
locale    = "en"       # Langue UI : en, zh-CN, ja

[options.tui]
theme     = "catppuccin"
dark_mode = true
compact_mode = false
```

### Configuration des fournisseurs

```toml
# Utiliser une clé API (supporte l'expansion $ENV_VAR).
[providers.anthropic]
api_key = "$ANTHROPIC_API_KEY"

# Utiliser OAuth (configuré via `codg auth`).
[providers.openai]
oauth = true

# Fournisseur personnalisé / auto-hébergé.
[providers.local]
name     = "My Local LLM"
type     = "openai-compat"
base_url = "http://localhost:8080/v1"
api_key  = "not-needed"
```

### Personnalisation des agents

```toml
# Forme courte : attribuer un type de modèle.
agents.coder = "large"
agents.task  = "small"

# Forme complète : affiner un agent.
[agents.advisor]
model           = "large"
temperature     = 0.3
thinking_budget = 32000
```

### Serveurs MCP

```toml
# Serveur MCP HTTP.
[mcp.websearch]
type = "http"
url  = "https://mcp.exa.ai/mcp?tools=web_search_exa"
```

### Skills

```toml
# Chargement et téléchargement automatiques via TUI ou codg skill
[option]
skill_urls = ["https://github.com/user/skills"]
```

### Modèles locaux (llama.cpp)

```toml
[llama]
port     = 8090
host     = "127.0.0.1"
ctx_size = 32000
gpu      = "auto"          # auto, cuda, off
```

### Canaux de messagerie

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
allowed_dirs = ["**x"] # tous les répertoires
```
