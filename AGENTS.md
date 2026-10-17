# AGENTS.md — Codg Plugin System

Guide for AI agents working in `github.com/vcaesar/codg`.

## Overview

Codg is an AI coding assistant (CLI). This repo is the **plugin system** — hook-based architecture for extending Codg. Inspired by [opencode](https://github.com/anomalyco/opencode/tree/dev/packages/plugin). **Module**: `github.com/vcaesar/codg` · **Go**: 1.26.2 · **Tests**: [testify](https://github.com/stretchr/testify) `require` (only external dep).

## Commands

```bash
cd plugin && go test ./...                      # all tests (plugin/ is its own module)
cd plugin && go test -v ./demo/                 # single package
cd plugin/auth/gemini/standalone && go build -buildmode=plugin -o ~/.codg/plugins/gemini-auth.so .
```

No Makefile or linter config. CI: `.github/workflows/go.yml`.

## Layout (STANDALONE — no internal deps)

```
plugin/            # own module (plugin/go.mod); plugin.go (core), types.go (syncMap/eventBus/PermissionService), plugin_test.go (~1200 lines)
├── pkg/           # package codgpkg — shared types: AgentTool, ToolInfo, ProviderOptions
├── auth/          # gemini, claude (+ standalone/ for .so); codex/gpt stubs
├── demo/          # hello tool + ShellEnv + SystemPrompt hooks
├── enhancer/      # tool interception, message transform, error handling
└── oauthplugin/   # OAuth tokens, provider resolution
```

## Key Interfaces

```go
type Plugin interface { Init(ctx context.Context, input PluginInput) (*Hooks, error) }
type AgentTool interface {                      // plugin/pkg (codgpkg)
    Info() ToolInfo
    Run(ctx context.Context, params ToolCall) (ToolResponse, error)
    ProviderOptions() ProviderOptions
    SetProviderOptions(opts ProviderOptions)
}
type PermissionService interface { Request(ctx context.Context, opts PermissionRequest) (bool, error) }
type PluginMetaProvider interface { Meta() PluginMeta }
```

## Hooks

Signature: `(input ReadOnly, output *Mutable) error`. Plugins chain by mutating output, ordered by `Priority` (lower first; default 0). All optional.
- **Short-circuit** on `output.Handled = true`: `OAuthToken`, `ProviderResolve`, `ErrorHandle`
- **Chaining**: `ChatParams`, `ChatHeaders`, `PermissionAsk`, `ShellEnv`, `ToolExecuteBefore/After`, `SystemPromptTransform`, `ConfigTransform`, `SessionStart/End`, `MessageTransform`

## Plugin Types

| Type      | Mechanism                            | Status  |
| --------- | ------------------------------------ | ------- |
| `builtin` | Compiled in, registered via `init()` | ✅      |
| `shared`  | `.so` loaded at runtime              | ✅      |
| `exec`    | JSON-RPC over stdin/stdout           | 🚧 stub |

## Conventions

- **Files**: `<feature>.go` + `<feature>_test.go` (same package, white-box). Every package has a doc comment.
- **Register**: `plugin.RegisterBuiltin("name", &Plugin{})` in `init()`. Tool names: `plugin_<plugin>_<tool>`.
- **Hook I/O**: `<Hook>Input` (read-only) / `<Hook>Output` (mutable pointer); trigger `Trigger<Hook>`.
- **Naming**: PascalCase exported; camelCase unexported consts (e.g., `credCacheTTL`).
- **JSON tags**: all public types; `omitempty` optional; `json:"-"` for funcs.

### Skeleton

```go
type Plugin struct { mu sync.RWMutex }
func (p *Plugin) Meta() plugin.PluginMeta { ... }
func (p *Plugin) Init(_ context.Context, in plugin.PluginInput) (*plugin.Hooks, error) {
    return &plugin.Hooks{Tools: map[string]plugin.ToolDefinition{...}, Priority: 10}, nil
}
```

### Rules

- Wrap errors: `fmt.Errorf("desc: %w", err)`. Never ignore returned errors.
- Tool errors → `ToolResponse{IsError: true}`, not Go errors. Hook errors → logged warnings; others still run.
- Init map fields before writing (`if output.Env == nil { output.Env = make(...) }`).
- Concurrency: `syncMap`, `eventBus`, `sync.RWMutex` on auth caches. Plugins init in parallel goroutines.

### Testing

- `t.Parallel()` unless mutating env / package-level state. Use `require` (not `assert`); plain-struct mocks.
- Save/restore globals (`plugins`, `allTools`, `builtinPlugins`) via `defer`. Env tests: `t.Setenv()`, no `t.Parallel()`.

## Standalone Constraint (Critical)

`plugin/` (incl. `plugin/pkg/`) must have NO dependency on internal Codg packages (config, csync, pubsub, permission). External authors `go get github.com/vcaesar/codg/plugin` without the full tree. If internal types are needed, add standalone equivalents in `plugin/types.go`.

## Known Issues

Stubs: `plugin/auth/codex/codex.go`, `plugin/auth/gpt/`, `pkg/`. gopls: `unusedwrite` in tests (intentional); `stringsbuilder` at `oauthplugin/oauthplugin.go:156`.

## Adding a Plugin / Hook

- **Plugin**: `plugin/myplugin/` → implement `Init()` (+ optional `Meta()`); register in `init()`; tests + package doc. For `.so`: add `standalone/main.go` exporting `CodgPlugin`.
- **Hook**: define `<Hook>Input/Output` in `plugin.go`; add to `Hooks` struct; add `Trigger<Hook>` (check `Handled` if short-circuiting); test in `plugin_test.go`; update `README.md` tables.
