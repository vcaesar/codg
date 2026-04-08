package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	codgpkg "github.com/vcaesar/codg/codgpkg"
)

// --- Mock permission service ---

type mockPermissionService struct{}

func newMockPermissions() *mockPermissionService {
	return &mockPermissionService{}
}

func (m *mockPermissionService) Request(_ context.Context, _ PermissionRequest) (bool, error) {
	return true, nil
}

// --- Tests ---

func TestPluginFuncAdapter(t *testing.T) {
	t.Parallel()

	called := false
	pf := PluginFunc(func(_ context.Context, _ PluginInput) (*Hooks, error) {
		called = true
		return &Hooks{
			Tools: map[string]ToolDefinition{
				"greet": {
					Description: "Say hello",
					Parameters:  map[string]any{"name": map[string]any{"type": "string"}},
					Execute: func(_ context.Context, input string) (string, error) {
						return "Hello!", nil
					},
				},
			},
		}, nil
	})

	hooks, err := pf.Init(context.Background(), PluginInput{})
	require.NoError(t, err)
	require.True(t, called)
	require.Len(t, hooks.Tools, 1)
	require.Equal(t, "Say hello", hooks.Tools["greet"].Description)
}

func TestToolDefinitionExecute(t *testing.T) {
	t.Parallel()

	td := ToolDefinition{
		Name:        "echo",
		Description: "Echoes input",
		Parameters:  map[string]any{"text": map[string]any{"type": "string"}},
		Execute: func(_ context.Context, input string) (string, error) {
			var args struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal([]byte(input), &args); err != nil {
				return "", err
			}
			return "echo: " + args.Text, nil
		},
	}

	result, err := td.Execute(context.Background(), `{"text": "world"}`)
	require.NoError(t, err)
	require.Equal(t, "echo: world", result)
}

func TestPluginToolInfo(t *testing.T) {
	t.Parallel()

	pt := &PluginTool{
		pluginName: "myplugin",
		def: ToolDefinition{
			Name:        "mytool",
			Description: "Does things",
			Parameters: map[string]any{
				"arg1": map[string]any{"type": "string", "description": "First argument"},
			},
			Required: []string{"arg1"},
		},
	}

	require.Equal(t, "plugin_myplugin_mytool", pt.Name())
	require.Equal(t, "myplugin", pt.PluginName())
	require.Equal(t, "mytool", pt.PluginToolName())

	info := pt.Info()
	require.Equal(t, "plugin_myplugin_mytool", info.Name)
	require.Equal(t, "Does things", info.Description)
	require.Len(t, info.Parameters, 1)
	require.Equal(t, []string{"arg1"}, info.Required)
}

func TestPluginToolProviderOptions(t *testing.T) {
	t.Parallel()

	pt := &PluginTool{
		pluginName: "test",
		def: ToolDefinition{
			Name: "tool",
		},
	}

	require.Nil(t, pt.ProviderOptions())

	pt.SetProviderOptions(codgpkg.ProviderOptions{})
	require.NotNil(t, pt.ProviderOptions())
}

func TestPluginToolRunWithPermission(t *testing.T) {
	t.Parallel()

	pt := &PluginTool{
		pluginName:  "test",
		permissions: newMockPermissions(),
		workingDir:  t.TempDir(),
		def: ToolDefinition{
			Name:        "greet",
			Description: "Greets",
			Execute: func(_ context.Context, input string) (string, error) {
				return "Hello from plugin!", nil
			},
		},
	}

	// Context needs session ID.
	ctx := context.WithValue(context.Background(), sessionIDContextKey("session_id"), "test-session")
	resp, err := pt.Run(ctx, codgpkg.ToolCall{
		ID:    "call-1",
		Name:  "plugin_test_greet",
		Input: `{"name": "world"}`,
	})
	require.NoError(t, err)
	require.Equal(t, "Hello from plugin!", resp.Content)
}

func TestPluginToolRunNoSession(t *testing.T) {
	t.Parallel()

	pt := &PluginTool{
		pluginName:  "test",
		permissions: newMockPermissions(),
		workingDir:  t.TempDir(),
		def: ToolDefinition{
			Name: "greet",
			Execute: func(_ context.Context, input string) (string, error) {
				return "Hello!", nil
			},
		},
	}

	_, err := pt.Run(context.Background(), codgpkg.ToolCall{
		ID:    "call-1",
		Name:  "plugin_test_greet",
		Input: `{}`,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "session ID is required")
}

func TestPluginToolRunNilExecute(t *testing.T) {
	t.Parallel()

	pt := &PluginTool{
		pluginName:  "test",
		permissions: newMockPermissions(),
		workingDir:  t.TempDir(),
		def: ToolDefinition{
			Name:    "broken",
			Execute: nil,
		},
	}

	ctx := context.WithValue(context.Background(), sessionIDContextKey("session_id"), "test-session")
	resp, err := pt.Run(ctx, codgpkg.ToolCall{
		ID:    "call-1",
		Name:  "plugin_test_broken",
		Input: `{}`,
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "no execute function")
}

func TestHooksAllNil(t *testing.T) {
	t.Parallel()

	// Verify that a Hooks with all nil fields is valid.
	h := &Hooks{}
	require.Nil(t, h.ChatParams)
	require.Nil(t, h.ChatHeaders)
	require.Nil(t, h.PermissionAsk)
	require.Nil(t, h.ShellEnv)
	require.Nil(t, h.ToolExecuteBefore)
	require.Nil(t, h.ToolExecuteAfter)
	require.Nil(t, h.SystemPromptTransform)
	require.Nil(t, h.Tools)
}

func TestStateString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state State
		want  string
	}{
		{StateDisabled, "disabled"},
		{StateLoading, "loading"},
		{StateReady, "ready"},
		{StateError, "error"},
		{State(99), "unknown"},
	}

	for _, tt := range tests {
		require.Equal(t, tt.want, tt.state.String())
	}
}

func TestRegisterBuiltin(t *testing.T) {
	// Save and restore.
	orig := builtinPlugins
	builtinPlugins = map[string]Plugin{}
	defer func() { builtinPlugins = orig }()

	called := false
	RegisterBuiltin("test-builtin", PluginFunc(func(_ context.Context, _ PluginInput) (*Hooks, error) {
		called = true
		return &Hooks{}, nil
	}))

	p, ok := builtinPlugins["test-builtin"]
	require.True(t, ok)

	_, err := p.Init(context.Background(), PluginInput{})
	require.NoError(t, err)
	require.True(t, called)
}

func TestBuiltinSourcePathFallback(t *testing.T) {
	// Save and restore.
	orig := builtinPlugins
	builtinPlugins = map[string]Plugin{}
	defer func() { builtinPlugins = orig }()

	RegisterBuiltin("demo", PluginFunc(func(_ context.Context, _ PluginInput) (*Hooks, error) {
		return &Hooks{}, nil
	}))

	tests := []struct {
		name   string
		source string
	}{
		{"exact name", "demo"},
		{"relative path", "./pkg/plugin/demo"},
		{"absolute path", "/home/user/codg/pkg/plugin/demo"},
		{"nested path", "some/deep/path/demo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Sequential — shares mutable global state.
			_ = Close(context.Background())

			cfg := &InitConfig{
				Plugins: map[string]PluginConfig{
					"my-demo": {
						Type:   PluginTypeBuiltin,
						Source: tt.source,
					},
				},
			}

			initSinglePlugin(context.Background(), cfg, "my-demo", cfg.Plugins["my-demo"])

			state := GetStates()
			info, ok := state["my-demo"]
			require.True(t, ok, "plugin %q not in states", "my-demo")
			require.Equal(t, StateReady, info.State, "source=%q should resolve to builtin 'demo'", tt.source)
		})
	}
}

func TestExecPluginNotImplemented(t *testing.T) {
	t.Parallel()

	ep := newExecPlugin("test", PluginConfig{
		Type:    PluginTypeExec,
		Command: "my-plugin",
	})

	_, err := ep.Init(context.Background(), PluginInput{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exec plugins are not yet implemented")
	require.Contains(t, err.Error(), "my-plugin")
}

func TestPluginConfigTypes(t *testing.T) {
	t.Parallel()

	require.Equal(t, PluginType("exec"), PluginTypeExec)
	require.Equal(t, PluginType("builtin"), PluginTypeBuiltin)
	require.Equal(t, PluginType("shared"), PluginTypeShared)
}

func TestPluginConfigInConfig(t *testing.T) {
	t.Parallel()

	cfgPlugins := map[string]PluginConfig{
		"my-plugin": {
			Type:          PluginTypeExec,
			Command:       "my-plugin-binary",
			Args:          []string{"--verbose"},
			Env:           map[string]string{"FOO": "bar"},
			Disabled:      false,
			DisabledTools: []string{"dangerous-tool"},
			Timeout:       60,
		},
		"builtin-plugin": {
			Type:   PluginTypeBuiltin,
			Source: "example",
		},
	}

	require.Len(t, cfgPlugins, 2)

	ep := cfgPlugins["my-plugin"]
	require.Equal(t, PluginTypeExec, ep.Type)
	require.Equal(t, "my-plugin-binary", ep.Command)
	require.Equal(t, []string{"--verbose"}, ep.Args)
	require.Equal(t, "bar", ep.Env["FOO"])
	require.Equal(t, []string{"dangerous-tool"}, ep.DisabledTools)
	require.Equal(t, 60, ep.Timeout)

	bp := cfgPlugins["builtin-plugin"]
	require.Equal(t, PluginTypeBuiltin, bp.Type)
	require.Equal(t, "example", bp.Source)
}

func TestTriggerChatParams(t *testing.T) {
	t.Parallel()

	temp := float64(0.5)
	info := &PluginInfo{
		Name:  "test",
		State: StateReady,
		Hooks: &Hooks{
			ChatParams: func(_ context.Context, input ChatParamsInput, output *ChatParamsOutput) error {
				require.Equal(t, "session-1", input.SessionID)
				require.Equal(t, "coder", input.Agent)
				output.Temperature = &temp
				return nil
			},
		},
	}

	output := &ChatParamsOutput{}
	err := info.Hooks.ChatParams(context.Background(), ChatParamsInput{
		SessionID: "session-1",
		Agent:     "coder",
	}, output)
	require.NoError(t, err)
	require.NotNil(t, output.Temperature)
	require.InDelta(t, 0.5, *output.Temperature, 0.001)
}

func TestTriggerShellEnv(t *testing.T) {
	t.Parallel()

	info := &PluginInfo{
		Name:  "env-plugin",
		State: StateReady,
		Hooks: &Hooks{
			ShellEnv: func(_ context.Context, input ShellEnvInput, output *ShellEnvOutput) error {
				if output.Env == nil {
					output.Env = make(map[string]string)
				}
				output.Env["PLUGIN_VAR"] = "hello"
				return nil
			},
		},
	}

	output := &ShellEnvOutput{}
	err := info.Hooks.ShellEnv(context.Background(), ShellEnvInput{
		Cwd: "/tmp",
	}, output)
	require.NoError(t, err)
	require.Equal(t, "hello", output.Env["PLUGIN_VAR"])
}

func TestTriggerPermissionAsk(t *testing.T) {
	t.Parallel()

	info := &PluginInfo{
		Name:  "perm-plugin",
		State: StateReady,
		Hooks: &Hooks{
			PermissionAsk: func(_ context.Context, input PermissionInput, output *PermissionOutput) error {
				if input.ToolName == "bash" {
					output.Status = "allow"
				}
				return nil
			},
		},
	}

	output := &PermissionOutput{Status: "ask"}
	err := info.Hooks.PermissionAsk(context.Background(), PermissionInput{
		ToolName: "bash",
	}, output)
	require.NoError(t, err)
	require.Equal(t, "allow", output.Status)
}

func TestTriggerToolExecuteBeforeAfter(t *testing.T) {
	t.Parallel()

	info := &PluginInfo{
		Name:  "interceptor",
		State: StateReady,
		Hooks: &Hooks{
			ToolExecuteBefore: func(_ context.Context, input ToolExecuteBeforeInput, output *ToolExecuteBeforeOutput) error {
				require.Equal(t, "bash", input.Tool)
				output.Args = json.RawMessage(`{"command": "modified"}`)
				return nil
			},
			ToolExecuteAfter: func(_ context.Context, input ToolExecuteAfterInput, output *ToolExecuteAfterOutput) error {
				require.Equal(t, "bash", input.Tool)
				output.Output = "modified output"
				return nil
			},
		},
	}

	beforeOutput := &ToolExecuteBeforeOutput{}
	err := info.Hooks.ToolExecuteBefore(context.Background(), ToolExecuteBeforeInput{
		Tool:      "bash",
		SessionID: "s1",
		CallID:    "c1",
	}, beforeOutput)
	require.NoError(t, err)
	require.Equal(t, `{"command": "modified"}`, string(beforeOutput.Args))

	afterOutput := &ToolExecuteAfterOutput{}
	err = info.Hooks.ToolExecuteAfter(context.Background(), ToolExecuteAfterInput{
		Tool:      "bash",
		SessionID: "s1",
		CallID:    "c1",
	}, afterOutput)
	require.NoError(t, err)
	require.Equal(t, "modified output", afterOutput.Output)
}

func TestTriggerSystemPromptTransform(t *testing.T) {
	t.Parallel()

	info := &PluginInfo{
		Name:  "prompt-plugin",
		State: StateReady,
		Hooks: &Hooks{
			SystemPromptTransform: func(_ context.Context, input SystemPromptInput, output *SystemPromptOutput) error {
				output.System = append(output.System, "Extra instructions from plugin.")
				return nil
			},
		},
	}

	output := &SystemPromptOutput{
		System: []string{"Base system prompt."},
	}
	err := info.Hooks.SystemPromptTransform(context.Background(), SystemPromptInput{
		Model: "claude-opus-4-6",
	}, output)
	require.NoError(t, err)
	require.Len(t, output.System, 2)
	require.Equal(t, "Extra instructions from plugin.", output.System[1])
}

func TestChatHeadersHook(t *testing.T) {
	t.Parallel()

	info := &PluginInfo{
		Name:  "headers-plugin",
		State: StateReady,
		Hooks: &Hooks{
			ChatHeaders: func(_ context.Context, _ ChatHeadersInput, output *ChatHeadersOutput) error {
				if output.Headers == nil {
					output.Headers = make(map[string]string)
				}
				output.Headers["X-Plugin-Auth"] = "token-123"
				return nil
			},
		},
	}

	output := &ChatHeadersOutput{}
	err := info.Hooks.ChatHeaders(context.Background(), ChatHeadersInput{
		SessionID: "s1",
		Agent:     "coder",
	}, output)
	require.NoError(t, err)
	require.Equal(t, "token-123", output.Headers["X-Plugin-Auth"])
}

func TestPluginInfoState(t *testing.T) {
	t.Parallel()

	info := &PluginInfo{
		Name:        "test",
		State:       StateReady,
		ConnectedAt: time.Now(),
		Hooks:       &Hooks{},
	}

	require.Equal(t, "test", info.Name)
	require.Equal(t, StateReady, info.State)
	require.Nil(t, info.Error)
	require.NotNil(t, info.Hooks)
	require.False(t, info.ConnectedAt.IsZero())
}

func TestPluginInputFields(t *testing.T) {
	t.Parallel()

	input := PluginInput{
		Config:     map[string]any{},
		WorkingDir: "/tmp/test",
	}

	require.NotNil(t, input.Config)
	require.Equal(t, "/tmp/test", input.WorkingDir)
}

func TestToolContextFields(t *testing.T) {
	t.Parallel()

	tc := ToolContext{
		SessionID:  "s1",
		MessageID:  "m1",
		WorkingDir: "/tmp",
		Abort:      context.Background(),
	}

	require.Equal(t, "s1", tc.SessionID)
	require.Equal(t, "m1", tc.MessageID)
	require.Equal(t, "/tmp", tc.WorkingDir)
	require.NotNil(t, tc.Abort)
}

func TestGetPluginToolsEmpty(t *testing.T) {
	t.Parallel()

	// When no plugins are registered, GetPluginTools returns nil.
	perms := newMockPermissions()
	tools := GetPluginTools(perms, nil, "/tmp")
	require.Empty(t, tools)
}

func TestMultipleHooksChaining(t *testing.T) {
	t.Parallel()

	// Test that multiple plugins can chain their hooks.
	plugin1 := &PluginInfo{
		Name:  "plugin1",
		State: StateReady,
		Hooks: &Hooks{
			ShellEnv: func(_ context.Context, _ ShellEnvInput, output *ShellEnvOutput) error {
				if output.Env == nil {
					output.Env = make(map[string]string)
				}
				output.Env["VAR1"] = "from-plugin1"
				return nil
			},
		},
	}

	plugin2 := &PluginInfo{
		Name:  "plugin2",
		State: StateReady,
		Hooks: &Hooks{
			ShellEnv: func(_ context.Context, _ ShellEnvInput, output *ShellEnvOutput) error {
				if output.Env == nil {
					output.Env = make(map[string]string)
				}
				output.Env["VAR2"] = "from-plugin2"
				return nil
			},
		},
	}

	// Simulate hook chaining (as TriggerShellEnv would do).
	output := &ShellEnvOutput{}
	for _, info := range []*PluginInfo{plugin1, plugin2} {
		if info.Hooks.ShellEnv != nil {
			err := info.Hooks.ShellEnv(context.Background(), ShellEnvInput{Cwd: "/tmp"}, output)
			require.NoError(t, err)
		}
	}

	require.Equal(t, "from-plugin1", output.Env["VAR1"])
	require.Equal(t, "from-plugin2", output.Env["VAR2"])
}

func TestDisabledPluginHooksNotCalled(t *testing.T) {
	t.Parallel()

	info := &PluginInfo{
		Name:  "disabled",
		State: StateDisabled,
		Hooks: &Hooks{
			ShellEnv: func(_ context.Context, _ ShellEnvInput, output *ShellEnvOutput) error {
				t.Fatal("hook should not be called for disabled plugin")
				return nil
			},
		},
	}

	// Simulate the trigger check (as done in the real trigger functions).
	if info.State == StateReady && info.Hooks != nil && info.Hooks.ShellEnv != nil {
		t.Fatal("should not reach here")
	}
}

func TestErrorStatePluginHooksNotCalled(t *testing.T) {
	t.Parallel()

	info := &PluginInfo{
		Name:  "errored",
		State: StateError,
		Hooks: &Hooks{
			ChatParams: func(_ context.Context, _ ChatParamsInput, _ *ChatParamsOutput) error {
				t.Fatal("hook should not be called for errored plugin")
				return nil
			},
		},
	}

	if info.State == StateReady && info.Hooks != nil && info.Hooks.ChatParams != nil {
		t.Fatal("should not reach here")
	}
}

func TestPluginToolRunExecuteError(t *testing.T) {
	t.Parallel()

	pt := &PluginTool{
		pluginName:  "test",
		permissions: newMockPermissions(),
		workingDir:  t.TempDir(),
		def: ToolDefinition{
			Name: "failing",
			Execute: func(_ context.Context, _ string) (string, error) {
				return "", fmt.Errorf("Something went wrong")
			},
		},
	}

	ctx := context.WithValue(context.Background(), sessionIDContextKey("session_id"), "test-session")
	resp, err := pt.Run(ctx, codgpkg.ToolCall{
		ID:    "call-1",
		Name:  "plugin_test_failing",
		Input: `{}`,
	})
	require.NoError(t, err) // Run itself doesn't error; it wraps in error response.
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "something went wrong")
}

// --- New hook tests ---

func TestOAuthTokenHook(t *testing.T) {
	t.Parallel()

	info := &PluginInfo{
		Name:  "auth-plugin",
		State: StateReady,
		Hooks: &Hooks{
			OAuthToken: func(_ context.Context, input OAuthTokenInput, output *OAuthTokenOutput) error {
				if input.Provider == "custom" {
					output.AccessToken = "custom-token-123"
					output.ExpiresIn = 3600
					output.Handled = true
				}
				return nil
			},
		},
	}

	output := &OAuthTokenOutput{}
	err := info.Hooks.OAuthToken(context.Background(), OAuthTokenInput{
		Provider:  "custom",
		Account:   "default",
		IsExpired: true,
	}, output)
	require.NoError(t, err)
	require.True(t, output.Handled)
	require.Equal(t, "custom-token-123", output.AccessToken)
	require.Equal(t, 3600, output.ExpiresIn)
}

func TestOAuthTokenHookNotHandled(t *testing.T) {
	t.Parallel()

	info := &PluginInfo{
		Name:  "auth-plugin",
		State: StateReady,
		Hooks: &Hooks{
			OAuthToken: func(_ context.Context, _ OAuthTokenInput, _ *OAuthTokenOutput) error {
				return nil
			},
		},
	}

	output := &OAuthTokenOutput{}
	err := info.Hooks.OAuthToken(context.Background(), OAuthTokenInput{
		Provider: "unknown",
	}, output)
	require.NoError(t, err)
	require.False(t, output.Handled)
}

func TestConfigTransformHook(t *testing.T) {
	t.Parallel()

	info := &PluginInfo{
		Name:  "config-plugin",
		State: StateReady,
		Hooks: &Hooks{
			ConfigTransform: func(_ context.Context, _ ConfigTransformInput, output *ConfigTransformOutput) error {
				if output.Overrides == nil {
					output.Overrides = make(map[string]any)
				}
				output.Overrides["options.auto_compact"] = true
				output.Overrides["options.max_tokens"] = 4096
				return nil
			},
		},
	}

	output := &ConfigTransformOutput{}
	err := info.Hooks.ConfigTransform(context.Background(), ConfigTransformInput{
		SessionID:  "s1",
		WorkingDir: "/tmp",
	}, output)
	require.NoError(t, err)
	require.Equal(t, true, output.Overrides["options.auto_compact"])
	require.Equal(t, 4096, output.Overrides["options.max_tokens"])
}

func TestProviderResolveHook(t *testing.T) {
	t.Parallel()

	info := &PluginInfo{
		Name:  "provider-plugin",
		State: StateReady,
		Hooks: &Hooks{
			ProviderResolve: func(_ context.Context, input ProviderResolveInput, output *ProviderResolveOutput) error {
				if input.Provider == "custom" {
					output.BaseURL = "https://custom.api.example.com"
					output.APIKey = "custom-key"
					output.Handled = true
				}
				return nil
			},
		},
	}

	output := &ProviderResolveOutput{}
	err := info.Hooks.ProviderResolve(context.Background(), ProviderResolveInput{
		Provider: "custom",
		Model:    "gpt-4",
	}, output)
	require.NoError(t, err)
	require.True(t, output.Handled)
	require.Equal(t, "https://custom.api.example.com", output.BaseURL)
	require.Equal(t, "custom-key", output.APIKey)
}

func TestSessionLifecycleHooks(t *testing.T) {
	t.Parallel()

	startCalled := false
	endCalled := false

	info := &PluginInfo{
		Name:  "lifecycle-plugin",
		State: StateReady,
		Hooks: &Hooks{
			SessionStart: func(_ context.Context, input SessionLifecycleInput, output *SessionLifecycleOutput) error {
				startCalled = true
				require.Equal(t, "s1", input.SessionID)
				require.Equal(t, "start", input.Phase)
				if output.Metadata == nil {
					output.Metadata = make(map[string]any)
				}
				output.Metadata["started_at"] = "now"
				return nil
			},
			SessionEnd: func(_ context.Context, input SessionLifecycleInput, output *SessionLifecycleOutput) error {
				endCalled = true
				require.Equal(t, "s1", input.SessionID)
				require.Equal(t, "end", input.Phase)
				return nil
			},
		},
	}

	startOutput := &SessionLifecycleOutput{}
	err := info.Hooks.SessionStart(context.Background(), SessionLifecycleInput{
		SessionID: "s1",
		Phase:     "start",
	}, startOutput)
	require.NoError(t, err)
	require.True(t, startCalled)
	require.Equal(t, "now", startOutput.Metadata["started_at"])

	endOutput := &SessionLifecycleOutput{}
	err = info.Hooks.SessionEnd(context.Background(), SessionLifecycleInput{
		SessionID: "s1",
		Phase:     "end",
	}, endOutput)
	require.NoError(t, err)
	require.True(t, endCalled)
}

func TestMessageTransformHook(t *testing.T) {
	t.Parallel()

	info := &PluginInfo{
		Name:  "msg-plugin",
		State: StateReady,
		Hooks: &Hooks{
			MessageTransform: func(_ context.Context, input MessageTransformInput, output *MessageTransformOutput) error {
				output.Content = "[ENHANCED] " + input.Content
				if output.Metadata == nil {
					output.Metadata = make(map[string]any)
				}
				output.Metadata["transformed"] = true
				return nil
			},
		},
	}

	output := &MessageTransformOutput{}
	err := info.Hooks.MessageTransform(context.Background(), MessageTransformInput{
		SessionID: "s1",
		Role:      "user",
		Content:   "Hello world",
	}, output)
	require.NoError(t, err)
	require.Equal(t, "[ENHANCED] Hello world", output.Content)
	require.Equal(t, true, output.Metadata["transformed"])
}

func TestErrorHandleHook(t *testing.T) {
	t.Parallel()

	info := &PluginInfo{
		Name:  "error-plugin",
		State: StateReady,
		Hooks: &Hooks{
			ErrorHandle: func(_ context.Context, input ErrorHandleInput, output *ErrorHandleOutput) error {
				if input.Retryable {
					output.Retry = true
				}
				if input.ToolName == "critical" {
					output.Message = "Critical error handled"
					output.Handled = true
				}
				return nil
			},
		},
	}

	// Retryable error.
	output := &ErrorHandleOutput{}
	err := info.Hooks.ErrorHandle(context.Background(), ErrorHandleInput{
		ToolName:  "bash",
		Error:     "timeout",
		Retryable: true,
	}, output)
	require.NoError(t, err)
	require.True(t, output.Retry)
	require.False(t, output.Handled)

	// Critical error.
	output2 := &ErrorHandleOutput{}
	err = info.Hooks.ErrorHandle(context.Background(), ErrorHandleInput{
		ToolName:  "critical",
		Error:     "fatal",
		Retryable: false,
	}, output2)
	require.NoError(t, err)
	require.True(t, output2.Handled)
	require.Equal(t, "Critical error handled", output2.Message)
}

func TestPluginMetaProvider(t *testing.T) {
	t.Parallel()

	type metaPlugin struct {
		PluginFunc
	}

	mp := struct {
		Plugin
		PluginMetaProvider
	}{
		Plugin: PluginFunc(func(_ context.Context, _ PluginInput) (*Hooks, error) {
			return &Hooks{}, nil
		}),
		PluginMetaProvider: nil, // Not a real provider.
	}

	// Just testing the type relationship.
	require.NotNil(t, mp.Plugin)
}

func TestPluginMetaFields(t *testing.T) {
	t.Parallel()

	meta := PluginMeta{
		Version:     "1.0.0",
		Author:      "Test Author",
		Description: "Test plugin description.",
		Homepage:    "https://example.com",
		Tags:        []string{"test", "example"},
	}

	require.Equal(t, "1.0.0", meta.Version)
	require.Equal(t, "Test Author", meta.Author)
	require.Equal(t, "Test plugin description.", meta.Description)
	require.Equal(t, "https://example.com", meta.Homepage)
	require.Equal(t, []string{"test", "example"}, meta.Tags)
}

func TestPluginInfoPriority(t *testing.T) {
	t.Parallel()

	info := &PluginInfo{
		Name:     "test",
		State:    StateReady,
		Priority: 10,
		Hooks: &Hooks{
			Priority: 10,
		},
	}

	require.Equal(t, 10, info.Priority)
}

func TestSortedPluginsOrder(t *testing.T) {
	t.Parallel()

	// Save and restore package state.
	origPlugins := plugins
	plugins = newSyncMap[string, *PluginInfo]()
	defer func() { plugins = origPlugins }()

	// Register plugins with different priorities.
	plugins.Set("high-priority", &PluginInfo{
		Name:     "high-priority",
		State:    StateReady,
		Priority: -10,
		Hooks:    &Hooks{Priority: -10},
	})
	plugins.Set("low-priority", &PluginInfo{
		Name:     "low-priority",
		State:    StateReady,
		Priority: 100,
		Hooks:    &Hooks{Priority: 100},
	})
	plugins.Set("default-a", &PluginInfo{
		Name:     "default-a",
		State:    StateReady,
		Priority: 0,
		Hooks:    &Hooks{Priority: 0},
	})
	plugins.Set("default-b", &PluginInfo{
		Name:     "default-b",
		State:    StateReady,
		Priority: 0,
		Hooks:    &Hooks{Priority: 0},
	})
	plugins.Set("disabled", &PluginInfo{
		Name:  "disabled",
		State: StateDisabled,
		Hooks: &Hooks{},
	})

	sorted := sortedPlugins()

	// Should have 4 ready plugins (disabled excluded).
	require.Len(t, sorted, 4)

	// Order: high-priority (-10), default-a (0), default-b (0), low-priority (100).
	require.Equal(t, "high-priority", sorted[0].Name)
	require.Equal(t, "default-a", sorted[1].Name)
	require.Equal(t, "default-b", sorted[2].Name)
	require.Equal(t, "low-priority", sorted[3].Name)
}

func TestToolDefinitionAsync(t *testing.T) {
	t.Parallel()

	td := ToolDefinition{
		Name:        "async-tool",
		Description: "An async tool",
		Async:       true,
		Execute: func(_ context.Context, _ string) (string, error) {
			return "done", nil
		},
	}

	require.True(t, td.Async)
	require.Nil(t, td.OnProgress)

	// Set OnProgress callback.
	progressCalled := false
	td.OnProgress = func(_ context.Context, chunk string) {
		progressCalled = true
		require.Equal(t, "progress", chunk)
	}
	td.OnProgress(context.Background(), "progress")
	require.True(t, progressCalled)
}

func TestGetPluginToolsDisabledFiltering(t *testing.T) {
	t.Parallel()

	// Save and restore package state.
	origTools := allTools
	allTools = newSyncMap[string, []ToolDefinition]()
	defer func() { allTools = origTools }()

	// Register tools for a plugin.
	allTools.Set("myplugin", []ToolDefinition{
		{Name: "tool_a", Description: "Tool A"},
		{Name: "tool_b", Description: "Tool B"},
		{Name: "tool_c", Description: "Tool C"},
	})

	perms := newMockPermissions()
	pluginConfigs := map[string]PluginConfig{
		"myplugin": {
			Type:          PluginTypeBuiltin,
			Source:        "myplugin",
			DisabledTools: []string{"tool_b"},
		},
	}

	tools := GetPluginTools(perms, pluginConfigs, "/tmp")
	require.Len(t, tools, 2)

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.PluginToolName()
	}
	require.Contains(t, names, "tool_a")
	require.Contains(t, names, "tool_c")
	require.NotContains(t, names, "tool_b")
}

func TestPluginInputPermissions(t *testing.T) {
	t.Parallel()

	perms := newMockPermissions()
	input := PluginInput{
		Config:      map[string]any{},
		WorkingDir:  "/tmp/test",
		Permissions: perms,
	}

	require.NotNil(t, input.Permissions)
	require.NotNil(t, input.Config)
	require.Equal(t, "/tmp/test", input.WorkingDir)
}

func TestHooksNewFields(t *testing.T) {
	t.Parallel()

	h := &Hooks{
		OAuthToken:      func(_ context.Context, _ OAuthTokenInput, _ *OAuthTokenOutput) error { return nil },
		ConfigTransform: func(_ context.Context, _ ConfigTransformInput, _ *ConfigTransformOutput) error { return nil },
		ProviderResolve: func(_ context.Context, _ ProviderResolveInput, _ *ProviderResolveOutput) error { return nil },
		SessionStart:    func(_ context.Context, _ SessionLifecycleInput, _ *SessionLifecycleOutput) error { return nil },
		SessionEnd:      func(_ context.Context, _ SessionLifecycleInput, _ *SessionLifecycleOutput) error { return nil },
		MessageTransform: func(_ context.Context, _ MessageTransformInput, _ *MessageTransformOutput) error {
			return nil
		},
		ErrorHandle: func(_ context.Context, _ ErrorHandleInput, _ *ErrorHandleOutput) error { return nil },
		Priority:    5,
	}

	require.NotNil(t, h.OAuthToken)
	require.NotNil(t, h.ConfigTransform)
	require.NotNil(t, h.ProviderResolve)
	require.NotNil(t, h.SessionStart)
	require.NotNil(t, h.SessionEnd)
	require.NotNil(t, h.MessageTransform)
	require.NotNil(t, h.ErrorHandle)
	require.Equal(t, 5, h.Priority)
}

func TestEventHookFired(t *testing.T) {
	t.Parallel()

	require.Equal(t, EventType(2), EventHookFired)
}

// --- Shared plugin discovery tests ---

func TestDiscoverSharedPluginsEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result := DiscoverSharedPlugins(dir)
	require.Empty(t, result)
}

func TestDiscoverSharedPluginsDirectSO(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	soFile := filepath.Join(dir, "my-plugin.so")
	err := os.WriteFile(soFile, []byte("fake"), 0o644)
	require.NoError(t, err)

	result := DiscoverSharedPlugins(dir)
	require.Len(t, result, 1)
	require.Equal(t, soFile, result["my-plugin"])
}

func TestDiscoverSharedPluginsNestedSO(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	subDir := filepath.Join(dir, "gemini-auth")
	err := os.MkdirAll(subDir, 0o755)
	require.NoError(t, err)

	soFile := filepath.Join(subDir, "gemini-auth.so")
	err = os.WriteFile(soFile, []byte("fake"), 0o644)
	require.NoError(t, err)

	result := DiscoverSharedPlugins(dir)
	require.Len(t, result, 1)
	require.Equal(t, soFile, result["gemini-auth"])
}

func TestDiscoverSharedPluginsEmptyDir(t *testing.T) {
	t.Parallel()

	result := DiscoverSharedPlugins("")
	require.Empty(t, result)
}

func TestResolveSharedPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Direct path.
	soFile := filepath.Join(dir, "demo.so")
	err := os.WriteFile(soFile, []byte("fake"), 0o644)
	require.NoError(t, err)

	resolved := resolveSharedPath(dir, "demo")
	require.Equal(t, soFile, resolved)

	// Nested path.
	subDir := filepath.Join(dir, "nested")
	err = os.MkdirAll(subDir, 0o755)
	require.NoError(t, err)

	nestedSO := filepath.Join(subDir, "nested.so")
	err = os.WriteFile(nestedSO, []byte("fake"), 0o644)
	require.NoError(t, err)

	resolved = resolveSharedPath(dir, "nested")
	require.Equal(t, nestedSO, resolved)

	// Not found.
	resolved = resolveSharedPath(dir, "nonexistent")
	require.Empty(t, resolved)

	// Empty dir.
	resolved = resolveSharedPath("", "any")
	require.Empty(t, resolved)
}

func TestDefaultPluginDir(t *testing.T) {
	t.Parallel()

	dir := DefaultPluginDir()
	require.Contains(t, dir, ".codg")
	require.Contains(t, dir, "plugins")
}

func TestInitConfigStandalone(t *testing.T) {
	t.Parallel()

	// Verify InitConfig is fully standalone with no config import.
	cfg := &InitConfig{
		Plugins: map[string]PluginConfig{
			"demo": {
				Type:   PluginTypeBuiltin,
				Source: "demo",
			},
			"my-shared": {
				Type:   PluginTypeShared,
				Source: "/path/to/my-shared.so",
			},
		},
		WorkingDir: "/tmp",
		PluginDir:  "/home/user/.codg/plugins",
	}

	require.Len(t, cfg.Plugins, 2)
	require.Equal(t, PluginTypeBuiltin, cfg.Plugins["demo"].Type)
	require.Equal(t, PluginTypeShared, cfg.Plugins["my-shared"].Type)
}
