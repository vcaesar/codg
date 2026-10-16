package plugin

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	codgpkg "github.com/vcaesar/codg/codgpkg"

	"github.com/stretchr/testify/require"
)

type countingPermissions struct {
	mu    sync.Mutex
	calls int
}

func (c *countingPermissions) Request(_ context.Context, _ PermissionRequest) (bool, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	return true, nil
}

func TestEventBusShutdownThenCancelNoDoubleClose(t *testing.T) {
	t.Parallel()

	b := newEventBus[int]()
	ctx, cancel := context.WithCancel(context.Background())
	ch := b.Subscribe(ctx)

	b.Shutdown()
	b.Shutdown() // Idempotent.
	cancel()

	_, ok := <-ch
	require.False(t, ok)
	// Give the subscriber goroutine time to run its cleanup; a double
	// close would panic and fail the test binary.
	time.Sleep(20 * time.Millisecond)
	b.Publish(1)
}

func TestRunInitNilHooksNormalized(t *testing.T) {
	t.Parallel()

	p := PluginFunc(func(context.Context, PluginInput) (*Hooks, error) { return nil, nil })
	hooks, err := runInit(context.Background(), p, PluginInput{}, 0)
	require.NoError(t, err)
	require.NotNil(t, hooks)
}

func TestRunInitPanicRecovered(t *testing.T) {
	t.Parallel()

	p := PluginFunc(func(context.Context, PluginInput) (*Hooks, error) { panic("boom") })
	hooks, err := runInit(context.Background(), p, PluginInput{}, 0)
	require.Nil(t, hooks)
	require.ErrorContains(t, err, "boom")
}

func TestRunInitTimeout(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	defer close(release)
	p := PluginFunc(func(context.Context, PluginInput) (*Hooks, error) {
		<-release
		return &Hooks{}, nil
	})
	_, err := runInit(context.Background(), p, PluginInput{}, 10*time.Millisecond)
	require.ErrorContains(t, err, "timed out")
}

func TestRunInitError(t *testing.T) {
	t.Parallel()

	want := errors.New("init failed")
	p := PluginFunc(func(context.Context, PluginInput) (*Hooks, error) { return nil, want })
	_, err := runInit(context.Background(), p, PluginInput{}, time.Second)
	require.ErrorIs(t, err, want)
}

func TestInitSinglePluginNilHooksAndTimeout(t *testing.T) {
	orig := builtinPlugins
	builtinPlugins = map[string]Plugin{}
	defer func() { builtinPlugins = orig }()
	require.NoError(t, Close(context.Background()))
	defer func() { require.NoError(t, Close(context.Background())) }()

	block := make(chan struct{})
	defer close(block)
	RegisterBuiltin("nilhooks", PluginFunc(func(context.Context, PluginInput) (*Hooks, error) { return nil, nil }))
	RegisterBuiltin("slow", PluginFunc(func(context.Context, PluginInput) (*Hooks, error) {
		<-block
		return &Hooks{}, nil
	}))

	cfg := &InitConfig{Plugins: map[string]PluginConfig{
		"nilhooks": {Type: PluginTypeBuiltin, Source: "nilhooks"},
		"slow":     {Type: PluginTypeBuiltin, Source: "slow", Timeout: 1},
	}}
	Initialize(context.Background(), cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, WaitForInit(ctx))

	info, ok := GetState("nilhooks")
	require.True(t, ok)
	require.Equal(t, StateReady, info.State)

	info, ok = GetState("slow")
	require.True(t, ok)
	require.Equal(t, StateError, info.State)
	require.ErrorContains(t, info.Error, "timed out")
}

func TestTriggerHookPanicIsolated(t *testing.T) {
	require.NoError(t, Close(context.Background()))
	defer func() { require.NoError(t, Close(context.Background())) }()

	plugins.Set("a-bad", &PluginInfo{Name: "a-bad", State: StateReady, Hooks: &Hooks{
		ShellEnv: func(context.Context, ShellEnvInput, *ShellEnvOutput) error { panic("bad plugin") },
	}})
	plugins.Set("b-good", &PluginInfo{Name: "b-good", State: StateReady, Hooks: &Hooks{
		ShellEnv: func(_ context.Context, _ ShellEnvInput, out *ShellEnvOutput) error {
			out.Env = map[string]string{"OK": "1"}
			return nil
		},
	}})

	out := &ShellEnvOutput{}
	require.NotPanics(t, func() { TriggerShellEnv(context.Background(), ShellEnvInput{}, out) })
	require.Equal(t, "1", out.Env["OK"])
}

func TestPluginToolRunExecutePanic(t *testing.T) {
	t.Parallel()

	pt := &PluginTool{
		pluginName:  "test",
		permissions: newMockPermissions(),
		def: ToolDefinition{
			Name:    "boom",
			Execute: func(context.Context, string) (string, error) { panic("kaboom") },
		},
	}
	resp, err := pt.Run(WithSessionID(context.Background(), "s"), codgpkg.ToolCall{ID: "c", Params: `{}`})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "kaboom")
}

func TestPluginToolRunNilExecuteSkipsPermission(t *testing.T) {
	t.Parallel()

	perms := &countingPermissions{}
	pt := &PluginTool{pluginName: "test", permissions: perms, def: ToolDefinition{Name: "x"}}
	resp, err := pt.Run(WithSessionID(context.Background(), "s"), codgpkg.ToolCall{ID: "c", Params: `{}`})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Zero(t, perms.calls)
}

func TestWithSessionID(t *testing.T) {
	t.Parallel()

	ctx := WithSessionID(context.Background(), "abc")
	require.Equal(t, "abc", ctx.Value(SessionIDContextKey))
}

func TestCloseConcurrentWithWaitAndSubscribe(t *testing.T) {
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
			defer cancel()
			_ = WaitForInit(ctx)
			_ = SubscribeEvents(ctx)
		})
		wg.Go(func() { _ = Close(context.Background()) })
	}
	wg.Wait()
	require.NoError(t, Close(context.Background()))
}

func TestPluginToolRunNilPermissionsDenied(t *testing.T) {
	t.Parallel()

	pt := &PluginTool{pluginName: "test", def: ToolDefinition{
		Name:    "x",
		Execute: func(context.Context, string) (string, error) { return "ran", nil },
	}}
	require.NotPanics(t, func() {
		_, err := pt.Run(WithSessionID(context.Background(), "s"), codgpkg.ToolCall{ID: "c", Params: `{}`})
		require.ErrorIs(t, err, ErrPermissionDenied)
	})
}

func TestGetPluginToolsDeterministicOrder(t *testing.T) {
	require.NoError(t, Close(context.Background()))
	defer func() { require.NoError(t, Close(context.Background())) }()

	for _, name := range []string{"zeta", "alpha", "mid", "beta"} {
		allTools.Set(name, []ToolDefinition{{Name: "a"}, {Name: "b"}})
	}
	cfgs := map[string]PluginConfig{"mid": {DisabledTools: []string{"a"}}}

	var want []string
	for range 5 {
		var got []string
		for _, tool := range GetPluginTools(newMockPermissions(), cfgs, "") {
			got = append(got, tool.Name())
		}
		if want == nil {
			want = got
		}
		require.Equal(t, want, got)
	}
	require.Equal(t, []string{
		"plugin_alpha_a", "plugin_alpha_b", "plugin_beta_a", "plugin_beta_b",
		"plugin_mid_b", "plugin_zeta_a", "plugin_zeta_b",
	}, want)
}

func TestInitSinglePluginUnknownBuiltinNamesPlugin(t *testing.T) {
	orig := builtinPlugins
	builtinPlugins = map[string]Plugin{}
	defer func() { builtinPlugins = orig }()
	require.NoError(t, Close(context.Background()))
	defer func() { require.NoError(t, Close(context.Background())) }()

	Initialize(context.Background(), &InitConfig{Plugins: map[string]PluginConfig{
		"missing": {Type: PluginTypeBuiltin},
	}}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, WaitForInit(ctx))

	info, ok := GetState("missing")
	require.True(t, ok)
	require.Equal(t, StateError, info.State)
	require.ErrorContains(t, info.Error, `unknown built-in plugin "missing"`)
}

func TestTriggerHandledShortCircuitAndPriority(t *testing.T) {
	require.NoError(t, Close(context.Background()))
	defer func() { require.NoError(t, Close(context.Background())) }()

	var order []string
	mk := func(name string, prio int, handle bool) {
		plugins.Set(name, &PluginInfo{Name: name, State: StateReady, Priority: prio, Hooks: &Hooks{
			ErrorHandle: func(_ context.Context, _ ErrorHandleInput, out *ErrorHandleOutput) error {
				order = append(order, name)
				out.Handled = handle
				return nil
			},
		}})
	}
	mk("c", 5, false)
	mk("a", 1, false)
	mk("b", 3, true)
	mk("d", 9, false)

	out := &ErrorHandleOutput{}
	TriggerErrorHandle(context.Background(), ErrorHandleInput{}, out)
	require.True(t, out.Handled)
	require.Equal(t, []string{"a", "b"}, order)
}
