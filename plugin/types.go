package plugin

import (
	"context"
	"errors"
	"iter"
	"maps"
	"sync"
)

// --- Concurrent map (replaces internal/csync.Map) ---

// syncMap is a generic thread-safe map. It replaces the internal csync
// package dependency so that /plugin can be imported independently.
type syncMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

func newSyncMap[K comparable, V any]() *syncMap[K, V] {
	return &syncMap[K, V]{m: make(map[K]V)}
}

func (s *syncMap[K, V]) Set(key K, val V) {
	s.mu.Lock()
	s.m[key] = val
	s.mu.Unlock()
}

func (s *syncMap[K, V]) Get(key K) (V, bool) {
	s.mu.RLock()
	v, ok := s.m[key]
	s.mu.RUnlock()
	return v, ok
}

// Copy returns a shallow copy of the map.
func (s *syncMap[K, V]) Copy() map[K]V {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return maps.Clone(s.m)
}

// Seq2 returns an iterator over key-value pairs. The iteration holds
// a read lock for each step and is safe for concurrent modification.
func (s *syncMap[K, V]) Seq2() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		s.mu.RLock()
		snapshot := maps.Clone(s.m)
		s.mu.RUnlock()
		for k, v := range snapshot {
			if !yield(k, v) {
				return
			}
		}
	}
}

// --- Event bus (replaces internal/pubsub) ---

// eventBus is a simple generic pub/sub broker. It replaces the
// internal pubsub package dependency.
type eventBus[T any] struct {
	mu   sync.RWMutex
	subs map[chan T]struct{}
	done chan struct{}
}

func newEventBus[T any]() *eventBus[T] {
	return &eventBus[T]{
		subs: make(map[chan T]struct{}),
		done: make(chan struct{}),
	}
}

func (b *eventBus[T]) Publish(payload T) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	select {
	case <-b.done:
		return
	default:
	}

	for ch := range b.subs {
		select {
		case ch <- payload:
		default: // Drop if full.
		}
	}
}

// Subscribe returns a channel that receives events until the context
// is cancelled.
func (b *eventBus[T]) Subscribe(ctx context.Context) <-chan T {
	b.mu.Lock()
	ch := make(chan T, 64)
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	go func() {
		select {
		case <-ctx.Done():
		case <-b.done:
		}
		b.mu.Lock()
		delete(b.subs, ch)
		close(ch)
		b.mu.Unlock()
	}()

	return ch
}

func (b *eventBus[T]) Shutdown() {
	select {
	case <-b.done:
		return
	default:
		close(b.done)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for ch := range b.subs {
		delete(b.subs, ch)
		close(ch)
	}
}

// --- Permission types (replaces internal/permission) ---

// ErrPermissionDenied is returned when the user denies a tool
// permission request.
var ErrPermissionDenied = errors.New("user denied permission")

// PermissionRequest describes a tool permission request.
type PermissionRequest struct {
	SessionID   string `json:"session_id"`
	ToolCallID  string `json:"tool_call_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

// PermissionService is the interface used by plugin tools to request
// execution permission from the user. This interface is defined here
// so that external plugin authors can use it without importing
// internal packages.
type PermissionService interface {
	Request(ctx context.Context, opts PermissionRequest) (bool, error)
}

// permissionAdapter wraps a function as a [PermissionService].
type permissionAdapter struct {
	fn func(ctx context.Context, opts PermissionRequest) (bool, error)
}

func (a *permissionAdapter) Request(ctx context.Context, opts PermissionRequest) (bool, error) {
	return a.fn(ctx, opts)
}

// NewPermissionService creates a [PermissionService] from a function.
// This is the recommended way to bridge internal permission
// implementations to the plugin API.
func NewPermissionService(fn func(ctx context.Context, opts PermissionRequest) (bool, error)) PermissionService {
	return &permissionAdapter{fn: fn}
}
