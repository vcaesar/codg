package plugin

import (
	"context"
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
