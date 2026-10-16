// Package plugin provides the plugin system for Codg.
//
// Plugins extend Codg's functionality through a hook-based architecture
// inspired by opencode's plugin system. A plugin is a Go value that
// implements the [Plugin] interface, returning a set of [Hooks] that
// intercept and extend various parts of the agent lifecycle.
//
// # Plugin Types
//
// Plugins can be loaded from four sources:
//
//   - **Built-in plugins**: Go code compiled directly into Codg.
//   - **Shared plugins (.so)**: Dynamically loaded Go shared objects
//     from ~/.codg/plugins/ (installed via `codg install`).
//   - **Exec plugins**: External executables that communicate via
//     JSON-RPC over stdin/stdout (configured in codg.toml).
//   - **Script plugins**: Reserved for future use.
//
// # Standalone Package
//
// This package has NO dependency on the codg config or internal packages.
// External plugin authors can import only this package to build
// standalone shared-object plugins that are loaded at runtime.
//
// # Hook System
//
// Hooks follow an input/output pattern where the input provides
// read-only context and the output is a mutable struct that the hook
// modifies to affect behavior. This mirrors opencode's
// (input: ReadOnly, output: Mutable) => Promise<void> pattern.
//
// # Tool Plugins
//
// The most common plugin type provides custom tools that the LLM can
// call. Plugin tools are surfaced as [codg.AgentTool] instances
// alongside built-in and MCP tools.
package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"plugin"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	codgpkg "github.com/vcaesar/codg/codgpkg"
)

// --- Plugin configuration types (standalone, no config import) ---

// PluginType identifies the transport mechanism for a plugin.
type PluginType string

const (
	// PluginTypeBuiltin references a Go plugin compiled into Codg.
	PluginTypeBuiltin PluginType = "builtin"
	// PluginTypeShared references a Go shared-object plugin (.so)
	// loaded from the plugin directory at runtime.
	PluginTypeShared PluginType = "shared"
	// PluginTypeExec runs the plugin as an external process communicating
	// via JSON-RPC over stdin/stdout.
	PluginTypeExec PluginType = "exec"
)

// PluginConfig holds the configuration for a single plugin.
type PluginConfig struct {
	// Type is the plugin transport type.
	Type PluginType `json:"type"`
	// Source is the built-in plugin name (for builtin type) or the
	// path to the shared-object file (for shared type).
	Source string `json:"source,omitempty"`
	// Command is the executable path for exec plugins.
	Command string `json:"command,omitempty"`
	// Args are the arguments passed to the exec plugin command.
	Args []string `json:"args,omitempty"`
	// Env is a map of environment variables set for the plugin process.
	Env map[string]string `json:"env,omitempty"`
	// Disabled marks the plugin as inactive.
	Disabled bool `json:"disabled,omitempty"`
	// DisabledTools lists tool names from this plugin that should be hidden.
	DisabledTools []string `json:"disabled_tools,omitempty"`
	// Timeout is the initialization timeout in seconds.
	Timeout int `json:"timeout,omitempty"`
}

// InitConfig provides all the information the plugin system needs
// during initialization. This replaces the direct config.Config
// dependency so that pkg/plugin remains standalone.
type InitConfig struct {
	// Plugins is the map of plugin configurations keyed by name.
	Plugins map[string]PluginConfig
	// WorkingDir is the current working directory.
	WorkingDir string
	// PluginDir is the directory where shared-object plugins are
	// installed (typically ~/.codg/plugins/).
	PluginDir string
}

// State represents the lifecycle state of a plugin.
type State int

const (
	// StateDisabled means the plugin is configured but disabled.
	StateDisabled State = iota
	// StateLoading means the plugin is being initialized.
	StateLoading
	// StateReady means the plugin is loaded and its hooks are active.
	StateReady
	// StateError means the plugin failed to load or encountered a
	// runtime error.
	StateError
)

// String returns a human-readable representation of the state.
func (s State) String() string {
	switch s {
	case StateDisabled:
		return "disabled"
	case StateLoading:
		return "loading"
	case StateReady:
		return "ready"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// EventType identifies plugin lifecycle events.
type EventType uint

const (
	// EventStateChanged is published when a plugin's state changes.
	EventStateChanged EventType = iota
	// EventToolsChanged is published when a plugin's tool set changes.
	EventToolsChanged
	// EventHookFired is published when a plugin hook is triggered.
	EventHookFired
)

// Event carries plugin lifecycle information for pub/sub subscribers.
type Event struct {
	Type  EventType
	Name  string
	State State
	Error error
}

// PluginInfo holds metadata about a loaded plugin.
type PluginInfo struct {
	Name        string
	State       State
	Error       error
	Hooks       *Hooks
	Meta        *PluginMeta
	ConnectedAt time.Time
	Priority    int
}

// PluginMeta holds optional descriptive metadata for a plugin.
// Plugins can provide this by implementing the [PluginMetaProvider]
// interface alongside the [Plugin] interface.
type PluginMeta struct {
	// Version is the semantic version of the plugin.
	Version string `json:"version,omitempty"`
	// Author is the plugin author or organization.
	Author string `json:"author,omitempty"`
	// Description is a human-readable summary of the plugin.
	Description string `json:"description,omitempty"`
	// Homepage is a URL to the plugin's homepage or repository.
	Homepage string `json:"homepage,omitempty"`
	// Tags are free-form labels for categorizing the plugin.
	Tags []string `json:"tags,omitempty"`
}

// PluginMetaProvider is an optional interface plugins can implement to
// expose descriptive metadata. This is checked at init time via type
// assertion.
type PluginMetaProvider interface {
	Meta() PluginMeta
}

// ToolContext provides contextual information to tool executions.
type ToolContext struct {
	SessionID  string
	MessageID  string
	WorkingDir string
	Abort      context.Context
}

// ToolDefinition defines a custom tool provided by a plugin.
type ToolDefinition struct {
	// Name is the tool's identifier (must be unique across all plugins).
	Name string `json:"name"`
	// Description is the tool's description shown to the LLM.
	Description string `json:"description"`
	// Parameters is a flat map of property names to their JSON Schema
	// definitions. The framework wraps this in {"type":"object",
	// "properties": ...} automatically — do NOT include the outer
	// object wrapper yourself.
	//
	//   Parameters: map[string]any{
	//       "name": map[string]any{"type": "string", "description": "..."},
	//   }
	Parameters map[string]any `json:"parameters"`
	// Required lists the required parameter names.
	Required []string `json:"required,omitempty"`
	// Execute runs the tool with the given JSON input and returns the
	// text result. The context carries session and abort information.
	Execute func(ctx context.Context, input string) (string, error) `json:"-"`
	// Async indicates the tool may produce streaming output via the
	// OnProgress callback during execution.
	Async bool `json:"async,omitempty"`
	// OnProgress is called by async tools to report intermediate results.
	// It is set by the framework before Execute is called when Async is
	// true.
	OnProgress func(ctx context.Context, chunk string) `json:"-"`
}

// ChatParamsInput provides read-only context for the chat.params hook.
type ChatParamsInput struct {
	SessionID string
	Agent     string
	Model     string
	Provider  string
}

// ChatParamsOutput is the mutable output for the chat.params hook.
type ChatParamsOutput struct {
	Temperature *float64
	TopP        *float64
	TopK        *int64
	Options     map[string]any
}

// ChatHeadersInput provides read-only context for the chat.headers hook.
type ChatHeadersInput struct {
	SessionID string
	Agent     string
	Model     string
	Provider  string
}

// ChatHeadersOutput is the mutable output for the chat.headers hook.
type ChatHeadersOutput struct {
	Headers map[string]string
}

// PermissionInput provides read-only context for the permission.ask hook.
type PermissionInput struct {
	SessionID  string
	ToolCallID string
	ToolName   string
	Action     string
	Path       string
}

// PermissionOutput is the mutable output for the permission.ask hook.
type PermissionOutput struct {
	Status string // "ask", "deny", or "allow".
}

// ShellEnvInput provides read-only context for the shell.env hook.
type ShellEnvInput struct {
	Cwd       string
	SessionID string
	CallID    string
}

// ShellEnvOutput is the mutable output for the shell.env hook.
type ShellEnvOutput struct {
	Env map[string]string
}

// ToolExecuteBeforeInput provides read-only context for tool.execute.before.
type ToolExecuteBeforeInput struct {
	Tool      string
	SessionID string
	CallID    string
}

// ToolExecuteBeforeOutput is the mutable output for tool.execute.before.
type ToolExecuteBeforeOutput struct {
	Args json.RawMessage
}

// ToolExecuteAfterInput provides read-only context for tool.execute.after.
type ToolExecuteAfterInput struct {
	Tool      string
	SessionID string
	CallID    string
	Args      json.RawMessage
}

// ToolExecuteAfterOutput is the mutable output for tool.execute.after.
type ToolExecuteAfterOutput struct {
	Title    string
	Output   string
	Metadata map[string]any
}

// SystemPromptInput provides read-only context for the system prompt hook.
type SystemPromptInput struct {
	SessionID string
	Model     string
}

// SystemPromptOutput is the mutable output for the system prompt hook.
type SystemPromptOutput struct {
	System []string
}

// OAuthTokenInput provides read-only context for the oauth.token hook.
type OAuthTokenInput struct {
	// Provider is the provider name (e.g. "anthropic", "openai").
	Provider string
	// Account is the keychain account identifier.
	Account string
	// IsExpired indicates whether the existing token has expired.
	IsExpired bool
}

// OAuthTokenOutput is the mutable output for the oauth.token hook.
type OAuthTokenOutput struct {
	// AccessToken is the token value to use.
	AccessToken string
	// RefreshToken is the refresh token value.
	RefreshToken string
	// ExpiresIn is the token lifetime in seconds.
	ExpiresIn int
	// Handled indicates the plugin fully handled token resolution
	// and the default flow should be skipped.
	Handled bool
}

// ConfigTransformInput provides read-only context for the config.transform hook.
type ConfigTransformInput struct {
	SessionID  string
	WorkingDir string
}

// ConfigTransformOutput is the mutable output for the config.transform hook.
type ConfigTransformOutput struct {
	// Overrides is a map of dotted config paths to values that
	// override the loaded configuration (e.g. "options.auto_compact": true).
	Overrides map[string]any
}

// ProviderResolveInput provides read-only context for provider.resolve.
type ProviderResolveInput struct {
	SessionID string
	Provider  string
	Model     string
}

// ProviderResolveOutput is the mutable output for provider.resolve.
type ProviderResolveOutput struct {
	// BaseURL overrides the provider's API endpoint.
	BaseURL string
	// APIKey overrides the provider's API key.
	APIKey string
	// Headers are extra headers merged into the request.
	Headers map[string]string
	// Handled indicates the plugin fully resolved the provider.
	Handled bool
}

// SessionLifecycleInput provides context for session.start / session.end.
type SessionLifecycleInput struct {
	SessionID  string
	WorkingDir string
	Model      string
	Provider   string
	// Phase is "start" or "end".
	Phase string
}

// SessionLifecycleOutput is the mutable output for session lifecycle hooks.
type SessionLifecycleOutput struct {
	// Metadata is arbitrary key-value data the plugin wants to attach
	// to the session.
	Metadata map[string]any
}

// MessageTransformInput provides read-only context for message.transform.
type MessageTransformInput struct {
	SessionID string
	Role      string // "user" or "assistant".
	Content   string
}

// MessageTransformOutput is the mutable output for message.transform.
type MessageTransformOutput struct {
	// Content is the transformed message content.
	Content string
	// Metadata is extra data to attach to the message.
	Metadata map[string]any
}

// ErrorHandleInput provides read-only context for the error.handle hook.
type ErrorHandleInput struct {
	SessionID string
	ToolName  string
	Error     string
	Retryable bool
}

// ErrorHandleOutput is the mutable output for the error.handle hook.
type ErrorHandleOutput struct {
	// Retry indicates the operation should be retried.
	Retry bool
	// Message is an override message to return instead of the error.
	Message string
	// Handled indicates the plugin fully handled the error.
	Handled bool
}

// Hooks defines the extension points a plugin can implement. All hooks
// are optional. The pattern follows opencode's convention:
// (input ReadOnly, output *Mutable) where hooks mutate output to
// affect behavior.
type Hooks struct {
	// Tools maps tool names to their definitions. These are surfaced
	// as codg.AgentTool instances to the LLM.
	Tools map[string]ToolDefinition

	// ChatParams is called before each LLM request to allow plugins
	// to modify temperature, top_p, top_k, and provider options.
	ChatParams func(ctx context.Context, input ChatParamsInput, output *ChatParamsOutput) error

	// ChatHeaders is called before each LLM request to allow plugins
	// to inject custom HTTP headers.
	ChatHeaders func(ctx context.Context, input ChatHeadersInput, output *ChatHeadersOutput) error

	// PermissionAsk is called when a tool requests permission,
	// allowing plugins to auto-approve or deny.
	PermissionAsk func(ctx context.Context, input PermissionInput, output *PermissionOutput) error

	// ShellEnv is called before shell command execution, allowing
	// plugins to inject environment variables.
	ShellEnv func(ctx context.Context, input ShellEnvInput, output *ShellEnvOutput) error

	// ToolExecuteBefore is called before a tool executes, allowing
	// plugins to modify the tool's arguments.
	ToolExecuteBefore func(ctx context.Context, input ToolExecuteBeforeInput, output *ToolExecuteBeforeOutput) error

	// ToolExecuteAfter is called after a tool executes, allowing
	// plugins to modify the tool's output.
	ToolExecuteAfter func(ctx context.Context, input ToolExecuteAfterInput, output *ToolExecuteAfterOutput) error

	// SystemPromptTransform is called when building the system prompt,
	// allowing plugins to append or modify system prompt parts.
	SystemPromptTransform func(ctx context.Context, input SystemPromptInput, output *SystemPromptOutput) error

	// OAuthToken is called when a provider needs an authentication
	// token, allowing plugins to inject or override OAuth tokens.
	// This integrates with internal/oauth/ by letting plugins provide
	// custom token resolution strategies.
	OAuthToken func(ctx context.Context, input OAuthTokenInput, output *OAuthTokenOutput) error

	// ConfigTransform is called after configuration is loaded,
	// allowing plugins to dynamically override configuration values.
	ConfigTransform func(ctx context.Context, input ConfigTransformInput, output *ConfigTransformOutput) error

	// ProviderResolve is called when resolving a provider's
	// connection details, allowing plugins to override the API
	// endpoint, key, or headers.
	ProviderResolve func(ctx context.Context, input ProviderResolveInput, output *ProviderResolveOutput) error

	// SessionStart is called when a new session begins.
	SessionStart func(ctx context.Context, input SessionLifecycleInput, output *SessionLifecycleOutput) error

	// SessionEnd is called when a session ends.
	SessionEnd func(ctx context.Context, input SessionLifecycleInput, output *SessionLifecycleOutput) error

	// MessageTransform is called before a message is sent to the LLM
	// or returned to the user, allowing content transformation.
	MessageTransform func(ctx context.Context, input MessageTransformInput, output *MessageTransformOutput) error

	// ErrorHandle is called when a tool or LLM call encounters an
	// error, allowing plugins to handle, retry, or override the
	// error response.
	ErrorHandle func(ctx context.Context, input ErrorHandleInput, output *ErrorHandleOutput) error

	// Priority controls the execution order of hooks across plugins.
	// Lower values run first. The default is 0.
	Priority int
}

// Plugin is the interface that plugin implementations must satisfy.
// The Init method is called once during plugin loading with the plugin
// context and returns the set of hooks the plugin provides.
//
// For shared-object (.so) plugins, the .so must export a symbol named
// "CodgPlugin" of type plugin.Plugin (this interface).
type Plugin interface {
	// Init initializes the plugin and returns its hooks. The context
	// is cancelled when the plugin should shut down.
	Init(ctx context.Context, input PluginInput) (*Hooks, error)
}

// PluginInput provides context to the plugin during initialization.
// It uses only types defined in this package so that external plugin
// authors need not import the codg config package.
type PluginInput struct {
	// Config is a generic map of the raw Codg configuration.
	// Plugins that need config access can inspect relevant keys.
	Config map[string]any
	// WorkingDir is the current working directory.
	WorkingDir string
	// Permissions is the permission service for requesting tool permissions.
	Permissions PermissionService
}

// PluginFunc adapts a plain function to the [Plugin] interface.
type PluginFunc func(ctx context.Context, input PluginInput) (*Hooks, error)

// Init implements [Plugin].
func (f PluginFunc) Init(ctx context.Context, input PluginInput) (*Hooks, error) {
	return f(ctx, input)
}

// --- Package-level state (mirrors mcp package pattern) ---

// initState tracks completion of one Initialize round. It is swapped
// atomically by Close so concurrent WaitForInit calls never race.
type initState struct {
	done chan struct{}
	once sync.Once
}

func newInitState() *initState {
	return &initState{done: make(chan struct{})}
}

func (s *initState) finish() {
	s.once.Do(func() { close(s.done) })
}

var (
	plugins  = newSyncMap[string, *PluginInfo]()
	allTools = newSyncMap[string, []ToolDefinition]()
	broker   atomic.Pointer[eventBus[Event]]
	initCur  atomic.Pointer[initState]

	// stateMu serializes init-round commits with Close so a stale round
	// can never write state after a reset.
	stateMu sync.Mutex
)

func init() {
	broker.Store(newEventBus[Event]())
	initCur.Store(newInitState())
}

// Initialize loads and initializes all configured plugins in parallel.
// It should be called once during application startup (typically from
// app.go). It returns immediately; use [WaitForInit] to block until all
// plugins have been initialized or failed.
func Initialize(ctx context.Context, cfg *InitConfig, perms PermissionService) {
	st := initCur.Load()
	if cfg == nil || len(cfg.Plugins) == 0 {
		st.finish()
		return
	}

	var wg sync.WaitGroup
	for name, pluginCfg := range cfg.Plugins {
		if pluginCfg.Disabled {
			updateState(st, name, StateDisabled, nil, nil)
			continue
		}
		wg.Go(func() {
			initSinglePlugin(ctx, st, cfg, perms, name, pluginCfg)
		})
	}

	go func() {
		wg.Wait()
		st.finish()
	}()
}

// initSinglePlugin loads and initializes a single plugin by name. State
// is only committed while st is still the current init round.
func initSinglePlugin(ctx context.Context, st *initState, cfg *InitConfig, perms PermissionService, name string, pluginCfg PluginConfig) {
	updateState(st, name, StateLoading, nil, nil)

	// Auto-detect type when not set explicitly.
	if pluginCfg.Type == "" {
		pluginCfg.Type = resolvePluginType(cfg, name, pluginCfg)
	}

	var p Plugin
	switch pluginCfg.Type {
	case PluginTypeBuiltin:
		source := pluginCfg.Source
		p = builtinPlugins[source]
		if p == nil {
			// The source may be a path like "./pkg/plugin/demo";
			// fall back to the base name ("demo").
			base := filepath.Base(source)
			p = builtinPlugins[base]
		}
		if p == nil {
			// Also try the config entry name itself as a last resort.
			p = builtinPlugins[name]
		}
		if p == nil {
			if source == "" {
				source = name
			}
			available := availableBuiltinNames()
			err := fmt.Errorf("unknown built-in plugin %q; available: [%s]", source, strings.Join(available, ", "))
			slog.Error("Failed to load plugin", "name", name, "error", err, "available", available)
			updateState(st, name, StateError, err, nil)
			return
		}
	case PluginTypeShared:
		soPath := pluginCfg.Source
		if soPath == "" {
			// Auto-resolve from plugin directory.
			soPath = resolveSharedPath(cfg.PluginDir, name)
		}
		if soPath == "" {
			err := fmt.Errorf("shared plugin .so not found for %q in %s", name, cfg.PluginDir)
			slog.Error("Failed to load plugin", "name", name, "error", err)
			updateState(st, name, StateError, err, nil)
			return
		}
		loaded, err := loadSharedPlugin(soPath)
		if err != nil {
			slog.Error("Failed to load shared plugin", "name", name, "path", soPath, "error", err)
			updateState(st, name, StateError, err, nil)
			return
		}
		p = loaded
	case PluginTypeExec:
		p = newExecPlugin(name, pluginCfg)
	default:
		err := fmt.Errorf("unsupported plugin type: %s", pluginCfg.Type)
		slog.Error("Failed to load plugin", "name", name, "error", err)
		updateState(st, name, StateError, err, nil)
		return
	}

	input := PluginInput{
		WorkingDir:  cfg.WorkingDir,
		Permissions: perms,
	}

	timeout := time.Duration(pluginCfg.Timeout) * time.Second
	hooks, err := runInit(ctx, p, input, timeout)
	if err != nil {
		slog.Error("Failed to initialize plugin", "name", name, "error", err)
		updateState(st, name, StateError, err, nil)
		return
	}

	info := newInfo(name, StateReady, nil, hooks)
	// Extract optional metadata via PluginMetaProvider.
	if mp, ok := p.(PluginMetaProvider); ok {
		if err := safeCall(func() error {
			meta := mp.Meta()
			info.Meta = &meta
			return nil
		}); err != nil {
			slog.Warn("Failed to read plugin metadata", "name", name, "error", err)
		}
	}

	defs := make([]ToolDefinition, 0, len(hooks.Tools))
	for toolName, def := range hooks.Tools {
		def.Name = toolName
		defs = append(defs, def)
	}
	slices.SortFunc(defs, func(a, b ToolDefinition) int {
		return strings.Compare(a.Name, b.Name)
	})
	if !commit(st, info, defs) {
		slog.Debug("Discarded stale plugin initialization", "name", name)
		return
	}

	slog.Info("Plugin initialized", "name", name, "type", pluginCfg.Type, "tools", len(hooks.Tools))
}

// runInit calls p.Init, recovering from panics and enforcing the
// optional timeout. On timeout or failure the context passed to Init is
// cancelled so cooperative plugins stop; on success it stays alive for
// the parent's lifetime so plugins may retain it. A nil hook set from a
// successful Init is normalized to an empty one.
func runInit(ctx context.Context, p Plugin, input PluginInput, timeout time.Duration) (*Hooks, error) {
	cancel := func(error) {}
	stop := func() bool { return true }
	if timeout > 0 {
		ctx, cancel = context.WithCancelCause(ctx)
		timer := time.AfterFunc(timeout, func() {
			cancel(fmt.Errorf("plugin init timed out after %s", timeout))
		})
		stop = timer.Stop
	}

	type result struct {
		hooks *Hooks
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		var r result
		r.err = safeCall(func() (err error) {
			r.hooks, err = p.Init(ctx, input)
			return err
		})
		ch <- r
	}()

	select {
	case r := <-ch:
		if !stop() {
			// The timer fired; ctx is already cancelled.
			return nil, context.Cause(ctx)
		}
		if r.err != nil {
			if errors.Is(r.err, ctx.Err()) {
				// Init gave up because of our context; report why.
				r.err = context.Cause(ctx)
			}
			cancel(r.err)
			return nil, r.err
		}
		if r.hooks == nil {
			r.hooks = &Hooks{}
		}
		return r.hooks, nil
	case <-ctx.Done():
		stop()
		return nil, context.Cause(ctx)
	}
}

// safeCall runs fn and converts a panic into an error so a misbehaving
// plugin cannot crash the host.
func safeCall(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("plugin panic: %v", r)
		}
	}()
	return fn()
}

// --- Shared plugin (.so) loading ---

// resolveSharedPath searches the plugin directory for a .so file
// matching the given name. It tries <name>.so and <name>/<name>.so.
func resolveSharedPath(pluginDir, name string) string {
	if pluginDir == "" {
		return ""
	}

	// Try <pluginDir>/<name>.so.
	direct := filepath.Join(pluginDir, name+".so")
	if _, err := os.Stat(direct); err == nil {
		return direct
	}

	// Try <pluginDir>/<name>/<name>.so.
	nested := filepath.Join(pluginDir, name, name+".so")
	if _, err := os.Stat(nested); err == nil {
		return nested
	}

	return ""
}

// loadSharedPlugin opens a Go plugin shared object and looks up the
// "CodgPlugin" exported symbol. The symbol must be of type
// plugin.Plugin (the interface defined in this package).
func loadSharedPlugin(path string) (Plugin, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open shared plugin %s: %w", path, err)
	}

	sym, err := p.Lookup("CodgPlugin")
	if err != nil {
		return nil, fmt.Errorf("shared plugin %s does not export CodgPlugin symbol: %w", path, err)
	}

	// The symbol should be a pointer to a value implementing Plugin.
	pluginImpl, ok := sym.(Plugin)
	if !ok {
		// Try pointer-to-interface pattern.
		if pp, ok2 := sym.(*Plugin); ok2 && pp != nil {
			pluginImpl = *pp
		} else {
			return nil, fmt.Errorf("shared plugin %s: CodgPlugin symbol is %T, want plugin.Plugin", path, sym)
		}
	}

	return pluginImpl, nil
}

// DiscoverSharedPlugins scans the plugin directory for .so files and
// returns a map of plugin names to their paths. This is used by the
// config layer to auto-discover installed plugins.
func DiscoverSharedPlugins(pluginDir string) map[string]string {
	result := make(map[string]string)
	if pluginDir == "" {
		return result
	}

	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		name := entry.Name()

		// Direct .so files: <name>.so.
		if !entry.IsDir() && strings.HasSuffix(name, ".so") {
			pluginName := strings.TrimSuffix(name, ".so")
			result[pluginName] = filepath.Join(pluginDir, name)
			continue
		}

		// Subdirectory: <name>/<name>.so.
		if entry.IsDir() {
			soPath := filepath.Join(pluginDir, name, name+".so")
			if _, serr := os.Stat(soPath); serr == nil {
				result[name] = soPath
			}
		}
	}

	return result
}

// DefaultPluginDir returns the default plugin directory path
// (~/.codg/plugins/).
func DefaultPluginDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codg", "plugins")
}

// WaitForInit blocks until all plugins have been initialized or the
// context is cancelled.
func WaitForInit(ctx context.Context) error {
	select {
	case <-initCur.Load().done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close shuts down all plugins and releases resources. It resets all
// package-level state so that [Initialize] can be called again (e.g.
// in tests or after a hot-reload).
func Close(_ context.Context) error {
	stateMu.Lock()
	defer stateMu.Unlock()
	initCur.Store(newInitState())
	broker.Swap(newEventBus[Event]()).Shutdown()
	allTools.Reset()
	plugins.Reset()
	return nil
}

// --- State management ---

func newInfo(name string, state State, err error, hooks *Hooks) *PluginInfo {
	priority := 0
	if hooks != nil {
		priority = hooks.Priority
	}
	return &PluginInfo{
		Name:        name,
		State:       state,
		Error:       err,
		Hooks:       hooks,
		Priority:    priority,
		ConnectedAt: time.Now(),
	}
}

func setInfo(info *PluginInfo) {
	plugins.Set(info.Name, info)
	broker.Load().Publish(Event{
		Type:  EventStateChanged,
		Name:  info.Name,
		State: info.State,
		Error: info.Error,
	})
}

// commit stores info and registers tools unless Close has started a new
// init round since st was captured. It reports whether it committed.
func commit(st *initState, info *PluginInfo, tools []ToolDefinition) bool {
	stateMu.Lock()
	defer stateMu.Unlock()
	if initCur.Load() != st {
		return false
	}
	setInfo(info)
	if len(tools) > 0 {
		allTools.Set(info.Name, tools)
		broker.Load().Publish(Event{
			Type:  EventToolsChanged,
			Name:  info.Name,
			State: StateReady,
		})
	}
	return true
}

func updateState(st *initState, name string, state State, err error, hooks *Hooks) {
	commit(st, newInfo(name, state, err, hooks), nil)
}

// GetStates returns a snapshot of all plugin states.
func GetStates() map[string]*PluginInfo {
	return plugins.Copy()
}

// GetState returns the state of a single plugin.
func GetState(name string) (*PluginInfo, bool) {
	return plugins.Get(name)
}

// SubscribeEvents returns a channel that receives plugin events.
func SubscribeEvents(ctx context.Context) <-chan Event {
	return broker.Load().Subscribe(ctx)
}

// --- Tool access ---

// Tools returns an iterator over all plugin tools grouped by plugin name.
func Tools() iter.Seq2[string, []ToolDefinition] {
	return allTools.Seq2()
}

// --- Hook triggers ---

// sortedPlugins returns all ready plugins sorted by priority (lower
// values first). This ensures deterministic hook execution order.
func sortedPlugins() []*PluginInfo {
	snapshot := plugins.Copy()
	result := make([]*PluginInfo, 0, len(snapshot))
	for _, info := range snapshot {
		if info.State == StateReady && info.Hooks != nil {
			result = append(result, info)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].Name < result[j].Name
	})
	return result
}

// trigger runs one hook across all ready plugins in priority order.
// Hook errors and panics are logged and never stop the chain; when
// handled is non-nil the chain stops as soon as it reports true.
func trigger[I, O any](ctx context.Context, name string, hook func(*Hooks) func(context.Context, I, *O) error, input I, output *O, handled func(*O) bool) {
	for _, info := range sortedPlugins() {
		fn := hook(info.Hooks)
		if fn == nil {
			continue
		}
		if err := safeCall(func() error { return fn(ctx, input, output) }); err != nil {
			slog.Warn("Plugin hook error", "hook", name, "plugin", info.Name, "error", err)
		}
		if handled != nil && handled(output) {
			return
		}
	}
}

// TriggerChatParams invokes the chat.params hook on all loaded plugins.
func TriggerChatParams(ctx context.Context, input ChatParamsInput, output *ChatParamsOutput) {
	trigger(ctx, "chat.params", func(h *Hooks) func(context.Context, ChatParamsInput, *ChatParamsOutput) error { return h.ChatParams }, input, output, nil)
}

// TriggerChatHeaders invokes the chat.headers hook on all loaded plugins.
func TriggerChatHeaders(ctx context.Context, input ChatHeadersInput, output *ChatHeadersOutput) {
	trigger(ctx, "chat.headers", func(h *Hooks) func(context.Context, ChatHeadersInput, *ChatHeadersOutput) error { return h.ChatHeaders }, input, output, nil)
}

// TriggerPermissionAsk invokes the permission.ask hook on all loaded plugins.
func TriggerPermissionAsk(ctx context.Context, input PermissionInput, output *PermissionOutput) {
	trigger(ctx, "permission.ask", func(h *Hooks) func(context.Context, PermissionInput, *PermissionOutput) error { return h.PermissionAsk }, input, output, nil)
}

// TriggerShellEnv invokes the shell.env hook on all loaded plugins.
func TriggerShellEnv(ctx context.Context, input ShellEnvInput, output *ShellEnvOutput) {
	trigger(ctx, "shell.env", func(h *Hooks) func(context.Context, ShellEnvInput, *ShellEnvOutput) error { return h.ShellEnv }, input, output, nil)
}

// TriggerToolExecuteBefore invokes the tool.execute.before hook on all
// loaded plugins.
func TriggerToolExecuteBefore(ctx context.Context, input ToolExecuteBeforeInput, output *ToolExecuteBeforeOutput) {
	trigger(ctx, "tool.execute.before", func(h *Hooks) func(context.Context, ToolExecuteBeforeInput, *ToolExecuteBeforeOutput) error {
		return h.ToolExecuteBefore
	}, input, output, nil)
}

// TriggerToolExecuteAfter invokes the tool.execute.after hook on all
// loaded plugins.
func TriggerToolExecuteAfter(ctx context.Context, input ToolExecuteAfterInput, output *ToolExecuteAfterOutput) {
	trigger(ctx, "tool.execute.after", func(h *Hooks) func(context.Context, ToolExecuteAfterInput, *ToolExecuteAfterOutput) error {
		return h.ToolExecuteAfter
	}, input, output, nil)
}

// TriggerSystemPromptTransform invokes the system prompt transform hook
// on all loaded plugins.
func TriggerSystemPromptTransform(ctx context.Context, input SystemPromptInput, output *SystemPromptOutput) {
	trigger(ctx, "system.prompt.transform", func(h *Hooks) func(context.Context, SystemPromptInput, *SystemPromptOutput) error {
		return h.SystemPromptTransform
	}, input, output, nil)
}

// TriggerOAuthToken invokes the oauth.token hook on all loaded plugins.
// This integrates with internal/oauth by allowing plugins to provide
// or override authentication tokens for any provider. It stops at the
// first plugin that sets output.Handled.
func TriggerOAuthToken(ctx context.Context, input OAuthTokenInput, output *OAuthTokenOutput) {
	trigger(ctx, "oauth.token", func(h *Hooks) func(context.Context, OAuthTokenInput, *OAuthTokenOutput) error { return h.OAuthToken }, input, output, func(o *OAuthTokenOutput) bool { return o.Handled })
}

// TriggerConfigTransform invokes the config.transform hook on all
// loaded plugins.
func TriggerConfigTransform(ctx context.Context, input ConfigTransformInput, output *ConfigTransformOutput) {
	trigger(ctx, "config.transform", func(h *Hooks) func(context.Context, ConfigTransformInput, *ConfigTransformOutput) error {
		return h.ConfigTransform
	}, input, output, nil)
}

// TriggerProviderResolve invokes the provider.resolve hook on all
// loaded plugins. It stops at the first plugin that sets output.Handled.
func TriggerProviderResolve(ctx context.Context, input ProviderResolveInput, output *ProviderResolveOutput) {
	trigger(ctx, "provider.resolve", func(h *Hooks) func(context.Context, ProviderResolveInput, *ProviderResolveOutput) error {
		return h.ProviderResolve
	}, input, output, func(o *ProviderResolveOutput) bool { return o.Handled })
}

// TriggerSessionStart invokes the session.start hook on all loaded plugins.
func TriggerSessionStart(ctx context.Context, input SessionLifecycleInput, output *SessionLifecycleOutput) {
	trigger(ctx, "session.start", func(h *Hooks) func(context.Context, SessionLifecycleInput, *SessionLifecycleOutput) error {
		return h.SessionStart
	}, input, output, nil)
}

// TriggerSessionEnd invokes the session.end hook on all loaded plugins.
func TriggerSessionEnd(ctx context.Context, input SessionLifecycleInput, output *SessionLifecycleOutput) {
	trigger(ctx, "session.end", func(h *Hooks) func(context.Context, SessionLifecycleInput, *SessionLifecycleOutput) error {
		return h.SessionEnd
	}, input, output, nil)
}

// TriggerMessageTransform invokes the message.transform hook on all
// loaded plugins.
func TriggerMessageTransform(ctx context.Context, input MessageTransformInput, output *MessageTransformOutput) {
	trigger(ctx, "message.transform", func(h *Hooks) func(context.Context, MessageTransformInput, *MessageTransformOutput) error {
		return h.MessageTransform
	}, input, output, nil)
}

// TriggerErrorHandle invokes the error.handle hook on all loaded
// plugins. It stops at the first plugin that sets output.Handled.
func TriggerErrorHandle(ctx context.Context, input ErrorHandleInput, output *ErrorHandleOutput) {
	trigger(ctx, "error.handle", func(h *Hooks) func(context.Context, ErrorHandleInput, *ErrorHandleOutput) error { return h.ErrorHandle }, input, output, func(o *ErrorHandleOutput) bool { return o.Handled })
}

// --- Built-in plugin registry ---

var builtinPlugins = map[string]Plugin{}

// RegisterBuiltin registers a built-in plugin by source name. This is
// called during init() by packages that provide built-in plugins.
func RegisterBuiltin(name string, p Plugin) {
	builtinPlugins[name] = p
}

// availableBuiltinNames returns a sorted list of registered built-in
// plugin names for use in error messages.
func availableBuiltinNames() []string {
	names := make([]string, 0, len(builtinPlugins))
	for name := range builtinPlugins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// --- Exec plugin ---

// resolvePluginType auto-detects the plugin type when the config entry
// has no explicit type. It checks builtin registry first, then looks
// for a .so in the plugin directory.
func resolvePluginType(cfg *InitConfig, name string, pc PluginConfig) PluginType {
	source := pc.Source
	// Check builtin registry (exact, basename, and config name).
	if _, ok := builtinPlugins[source]; ok {
		return PluginTypeBuiltin
	}
	if source != "" {
		if _, ok := builtinPlugins[filepath.Base(source)]; ok {
			return PluginTypeBuiltin
		}
	}
	if _, ok := builtinPlugins[name]; ok {
		return PluginTypeBuiltin
	}

	// Check for an installed .so file.
	if cfg.PluginDir != "" {
		if soPath := resolveSharedPath(cfg.PluginDir, name); soPath != "" {
			return PluginTypeShared
		}
	}
	if source != "" {
		// Source itself might be a .so path.
		if strings.HasSuffix(source, ".so") {
			return PluginTypeShared
		}
	}

	// If command is set, assume exec.
	if pc.Command != "" {
		return PluginTypeExec
	}

	// Default to builtin — initSinglePlugin will produce a clear
	// error if the name isn't registered.
	return PluginTypeBuiltin
}

// execPlugin implements [Plugin] for external executable plugins that
// communicate via JSON-RPC over stdin/stdout.
type execPlugin struct {
	name   string
	config PluginConfig
}

func newExecPlugin(name string, cfg PluginConfig) *execPlugin {
	return &execPlugin{
		name:   name,
		config: cfg,
	}
}

// Init starts the exec plugin process and negotiates capabilities.
func (p *execPlugin) Init(ctx context.Context, input PluginInput) (*Hooks, error) {
	// TODO: Implement exec plugin subprocess lifecycle.
	// This will spawn the plugin command, send an init request via
	// JSON-RPC, and map returned capabilities to Hooks.
	return nil, fmt.Errorf("exec plugins are not yet implemented; plugin %q configured with command %q", p.name, p.config.Command)
}

// --- Plugin tool → codgpkg.AgentTool adapter ---

// PluginTool wraps a [ToolDefinition] from a plugin as a
// [codgpkg.AgentTool] so it can be used in the agent tool chain.
type PluginTool struct {
	pluginName      string
	def             ToolDefinition
	permissions     PermissionService
	workingDir      string
	providerOptions codgpkg.ProviderOptions
}

// SetProviderOptions implements [codgpkg.AgentTool].
func (t *PluginTool) SetProviderOptions(opts codgpkg.ProviderOptions) {
	t.providerOptions = opts
}

// ProviderOptions implements [codgpkg.AgentTool].
func (t *PluginTool) ProviderOptions() codgpkg.ProviderOptions {
	return t.providerOptions
}

// Name returns the fully qualified tool name (plugin_<plugin>_<tool>).
func (t *PluginTool) Name() string {
	return fmt.Sprintf("plugin_%s_%s", t.pluginName, t.def.Name)
}

// PluginName returns the source plugin name.
func (t *PluginTool) PluginName() string {
	return t.pluginName
}

// PluginToolName returns the tool's name within the plugin.
func (t *PluginTool) PluginToolName() string {
	return t.def.Name
}

// Info implements [codgpkg.AgentTool].
func (t *PluginTool) Info() codgpkg.ToolInfo {
	return codgpkg.ToolInfo{
		Name:        t.Name(),
		Description: t.def.Description,
		Parameters:  t.def.Parameters,
		Required:    t.def.Required,
	}
}

// Run implements [codgpkg.AgentTool].
func (t *PluginTool) Run(ctx context.Context, params codgpkg.ToolCall) (codgpkg.ToolResponse, error) {
	sessionID, ok := ctx.Value(SessionIDContextKey).(string)
	if !ok || sessionID == "" {
		return codgpkg.ToolResponse{}, fmt.Errorf("session ID is required")
	}

	if t.def.Execute == nil {
		return codgpkg.NewTextErrorResponse("plugin tool has no execute function"), nil
	}

	if t.permissions == nil {
		return codgpkg.ToolResponse{}, fmt.Errorf("plugin tool %s: no permission service configured: %w", t.Name(), ErrPermissionDenied)
	}

	desc := fmt.Sprintf("execute plugin tool %s with parameters:", t.Name())
	granted, err := t.permissions.Request(ctx, PermissionRequest{
		SessionID:   sessionID,
		ToolCallID:  params.ID,
		Path:        t.workingDir,
		ToolName:    t.Name(),
		Action:      "execute",
		Description: desc,
		Params:      params.Params,
	})
	if err != nil {
		return codgpkg.ToolResponse{}, err
	}
	if !granted {
		return codgpkg.ToolResponse{}, ErrPermissionDenied
	}

	var result string
	err = safeCall(func() (err error) {
		result, err = t.def.Execute(ctx, params.Params)
		return err
	})
	if err != nil {
		return codgpkg.NewTextErrorResponse(err.Error()), nil
	}

	return codgpkg.NewTextResponse(result), nil
}

type sessionIDContextKey string

// SessionIDContextKey is the context key [PluginTool.Run] reads the
// session ID from. Context keys are compared by type, so the host must
// set this key explicitly (see [WithSessionID]); keys from other
// packages never match.
const SessionIDContextKey sessionIDContextKey = "session_id"

// WithSessionID returns a context carrying the session ID for plugin
// tool execution.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, SessionIDContextKey, sessionID)
}

// GetPluginTools returns all currently registered plugin tools as
// [*PluginTool] instances ready to be used as [codgpkg.AgentTool].
// Tools listed in a plugin's DisabledTools configuration are excluded.
// The result is sorted by plugin name, then tool name, so the tool list
// sent to the LLM is stable across calls.
func GetPluginTools(perms PermissionService, pluginConfigs map[string]PluginConfig, wd string) []*PluginTool {
	snapshot := allTools.Copy()
	var result []*PluginTool
	for _, pluginName := range slices.Sorted(maps.Keys(snapshot)) {
		disabledTools := pluginConfigs[pluginName].DisabledTools
		for _, def := range snapshot[pluginName] {
			if slices.Contains(disabledTools, def.Name) {
				continue
			}
			result = append(result, &PluginTool{
				pluginName:  pluginName,
				def:         def,
				permissions: perms,
				workingDir:  wd,
			})
		}
	}
	return result
}
