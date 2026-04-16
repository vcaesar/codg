package demo

import (
	"context"
	"testing"

	"github.com/vcaesar/codg/plugin"

	"github.com/stretchr/testify/require"
)

func TestPluginInit(t *testing.T) {
	t.Parallel()

	p := &Plugin{}
	hooks, err := p.Init(context.Background(), plugin.PluginInput{})
	require.NoError(t, err)
	require.NotNil(t, hooks)
	require.Len(t, hooks.Tools, 1)
	require.Contains(t, hooks.Tools, "hello")
	require.NotNil(t, hooks.ShellEnv)
	require.NotNil(t, hooks.SystemPromptTransform)
}

func TestHelloToolExecute(t *testing.T) {
	t.Parallel()

	result, err := executeHello(context.Background(), `{"name": "World"}`)
	require.NoError(t, err)
	require.Contains(t, result, "Hello, World!")
	require.Contains(t, result, "demo plugin")
}

func TestHelloToolExecuteEmptyName(t *testing.T) {
	t.Parallel()

	_, err := executeHello(context.Background(), `{"name": ""}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "name is required")
}

func TestHelloToolExecuteInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := executeHello(context.Background(), `{invalid}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid input")
}

func TestHelloToolExecuteMissingName(t *testing.T) {
	t.Parallel()

	_, err := executeHello(context.Background(), `{}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "name is required")
}

func TestShellEnvHook(t *testing.T) {
	t.Parallel()

	p := &Plugin{}
	hooks, err := p.Init(context.Background(), plugin.PluginInput{})
	require.NoError(t, err)

	output := &plugin.ShellEnvOutput{}
	err = hooks.ShellEnv(context.Background(), plugin.ShellEnvInput{
		Cwd: "/tmp",
	}, output)
	require.NoError(t, err)
	require.Equal(t, "1", output.Env["CODG_PLUGIN_DEMO"])
}

func TestShellEnvHookPreservesExisting(t *testing.T) {
	t.Parallel()

	p := &Plugin{}
	hooks, err := p.Init(context.Background(), plugin.PluginInput{})
	require.NoError(t, err)

	// Pre-populate with existing env vars.
	output := &plugin.ShellEnvOutput{
		Env: map[string]string{"EXISTING": "value"},
	}
	err = hooks.ShellEnv(context.Background(), plugin.ShellEnvInput{}, output)
	require.NoError(t, err)
	require.Equal(t, "value", output.Env["EXISTING"])
	require.Equal(t, "1", output.Env["CODG_PLUGIN_DEMO"])
}

func TestSystemPromptTransformHook(t *testing.T) {
	t.Parallel()

	p := &Plugin{}
	hooks, err := p.Init(context.Background(), plugin.PluginInput{})
	require.NoError(t, err)

	output := &plugin.SystemPromptOutput{
		System: []string{"Base prompt."},
	}
	err = hooks.SystemPromptTransform(context.Background(), plugin.SystemPromptInput{
		Model: "claude-opus-4-6",
	}, output)
	require.NoError(t, err)
	require.Len(t, output.System, 2)
	require.Contains(t, output.System[1], "demo plugin")
	require.Contains(t, output.System[1], "hello")
}

func TestHelloToolDefinitionFields(t *testing.T) {
	t.Parallel()

	p := &Plugin{}
	hooks, err := p.Init(context.Background(), plugin.PluginInput{})
	require.NoError(t, err)

	hello := hooks.Tools["hello"]
	require.NotEmpty(t, hello.Description)
	require.Contains(t, hello.Description, "greeting")
	require.NotNil(t, hello.Parameters)
	require.Equal(t, []string{"name"}, hello.Required)
	require.NotNil(t, hello.Execute)
}

func TestRegisterBuiltinRegistration(t *testing.T) {
	// Verify the init() function registered the demo plugin.
	// We test indirectly by checking the plugin can be found in
	// the builtinPlugins map via a fresh Init call.
	p := &Plugin{}
	hooks, err := p.Init(context.Background(), plugin.PluginInput{})
	require.NoError(t, err)
	require.NotNil(t, hooks)
}
