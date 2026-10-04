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
