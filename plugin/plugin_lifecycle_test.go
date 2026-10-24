package plugin

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunInitTimeoutCancelsPluginContext(t *testing.T) {
	t.Parallel()

	stopped := make(chan struct{})
	p := PluginFunc(func(ctx context.Context, _ PluginInput) (*Hooks, error) {
		defer close(stopped)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	hooks, err := runInit(context.Background(), p, PluginInput{}, 20*time.Millisecond)
	require.Nil(t, hooks)
	require.ErrorContains(t, err, "timed out")

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Init still blocked after its timeout")
	}
}

func TestRunInitSuccessKeepsPluginContext(t *testing.T) {
	t.Parallel()

	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	var kept context.Context
	p := PluginFunc(func(ctx context.Context, _ PluginInput) (*Hooks, error) {
		kept = ctx
		return &Hooks{}, nil
	})
	hooks, err := runInit(parent, p, PluginInput{}, 20*time.Millisecond)
	require.NoError(t, err)
	require.NotNil(t, hooks)

	// Outlive the init timeout: the retained context must stay alive.
	time.Sleep(50 * time.Millisecond)
	require.NoError(t, kept.Err())

	cancelParent()
	select {
	case <-kept.Done():
	case <-time.After(time.Second):
		t.Fatal("Retained context did not follow parent cancellation")
	}
}

func TestRunInitParentCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := PluginFunc(func(ctx context.Context, _ PluginInput) (*Hooks, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	_, err := runInit(ctx, p, PluginInput{}, time.Second)
	require.ErrorIs(t, err, context.Canceled)
}

func TestCloseDiscardsStaleInit(t *testing.T) {
	require.NoError(t, Close(context.Background()))
	t.Cleanup(func() { require.NoError(t, Close(context.Background())) })
	orig := builtinPlugins
	builtinPlugins = map[string]Plugin{}
	t.Cleanup(func() { builtinPlugins = orig })

	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	RegisterBuiltin("slow", PluginFunc(func(context.Context, PluginInput) (*Hooks, error) {
		close(started)
		<-release
		return &Hooks{Tools: map[string]ToolDefinition{"old-tool": {}}}, nil
	}))

	oldState := initCur.Load()
	Initialize(context.Background(), &InitConfig{Plugins: map[string]PluginConfig{
		"slow": {Type: PluginTypeBuiltin, Source: "slow"},
	}}, nil)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("Init did not start")
	}

	require.NoError(t, Close(context.Background()))
	Initialize(context.Background(), &InitConfig{Plugins: map[string]PluginConfig{
		"slow": {Disabled: true},
	}}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, WaitForInit(ctx))

	unblock()
	select {
	case <-oldState.done:
	case <-time.After(time.Second):
		t.Fatal("old init round did not finish")
	}

	info, ok := GetState("slow")
	require.True(t, ok)
	require.Equal(t, StateDisabled, info.State)
	require.Empty(t, GetPluginTools(nil, nil, ""))
}

type panickingMeta struct{}

func (panickingMeta) Init(context.Context, PluginInput) (*Hooks, error) {
	return &Hooks{}, nil
}

func (panickingMeta) Meta() PluginMeta {
	panic("meta panic")
}

func TestMetaPanicIsolated(t *testing.T) {
	require.NoError(t, Close(context.Background()))
	t.Cleanup(func() { require.NoError(t, Close(context.Background())) })
	orig := builtinPlugins
	builtinPlugins = map[string]Plugin{}
	t.Cleanup(func() { builtinPlugins = orig })
	RegisterBuiltin("meta", panickingMeta{})

	require.NotPanics(t, func() {
		initSinglePlugin(context.Background(), initCur.Load(), &InitConfig{}, nil, "meta", PluginConfig{
			Type: PluginTypeBuiltin, Source: "meta",
		})
	})
	info, ok := GetState("meta")
	require.True(t, ok)
	require.Equal(t, StateReady, info.State)
	require.Nil(t, info.Meta)
}

func TestCloseDisposesInReverseOrder(t *testing.T) {
	require.NoError(t, Close(context.Background()))
	t.Cleanup(func() { require.NoError(t, Close(context.Background())) })

	var order []string
	st := initCur.Load()
	for _, tc := range []struct {
		name     string
		priority int
		err      error
	}{{"a", 0, nil}, {"b", 1, errors.New("boom")}, {"c", 2, nil}} {
		commit(st, newInfo(tc.name, StateReady, nil, &Hooks{
			Priority: tc.priority,
			Dispose: func(context.Context) error {
				order = append(order, tc.name)
				// Calling back into the package must not deadlock.
				GetStates()
				return tc.err
			},
		}), nil)
	}
	commit(st, newInfo("panics", StateReady, nil, &Hooks{
		Priority: 3,
		Dispose:  func(context.Context) error { panic("dispose panic") },
	}), nil)

	err := Close(context.Background())
	require.ErrorContains(t, err, "plugin b dispose: boom")
	require.ErrorContains(t, err, "plugin panics dispose: plugin panic")
	require.Equal(t, []string{"c", "b", "a"}, order)
	require.Empty(t, GetStates())
}

func TestPluginOptionsPassedToInit(t *testing.T) {
	require.NoError(t, Close(context.Background()))
	t.Cleanup(func() { require.NoError(t, Close(context.Background())) })
	orig := builtinPlugins
	builtinPlugins = map[string]Plugin{}
	t.Cleanup(func() { builtinPlugins = orig })

	var got map[string]any
	RegisterBuiltin("opts", PluginFunc(func(_ context.Context, in PluginInput) (*Hooks, error) {
		got = in.Options
		return &Hooks{}, nil
	}))
	initSinglePlugin(context.Background(), initCur.Load(), &InitConfig{}, nil, "opts", PluginConfig{
		Type: PluginTypeBuiltin, Options: map[string]any{"strict": true},
	})
	require.Equal(t, map[string]any{"strict": true}, got)
}

func TestTriggerPermissionAskDenyIsFinal(t *testing.T) {
	require.NoError(t, Close(context.Background()))
	t.Cleanup(func() { require.NoError(t, Close(context.Background())) })

	st := initCur.Load()
	set := func(name string, priority int, status string) {
		commit(st, newInfo(name, StateReady, nil, &Hooks{
			Priority: priority,
			PermissionAsk: func(_ context.Context, _ PermissionInput, out *PermissionOutput) error {
				out.Status = status
				return nil
			},
		}), nil)
	}
	set("allow1", 0, "allow")
	set("deny", 1, "deny")
	set("allow2", 2, "allow")

	out := &PermissionOutput{Status: "ask"}
	TriggerPermissionAsk(context.Background(), PermissionInput{ToolName: "bash"}, out)
	require.Equal(t, "deny", out.Status)
}

func TestPluginToolNameSanitized(t *testing.T) {
	t.Parallel()

	pt := &PluginTool{pluginName: "acme.tools", def: ToolDefinition{Name: "say hi/v2"}}
	require.Equal(t, "plugin_acme_tools_say_hi_v2", pt.Name())
	pt = &PluginTool{pluginName: "gemini-auth", def: ToolDefinition{Name: "Login_1"}}
	require.Equal(t, "plugin_gemini-auth_Login_1", pt.Name())
}
