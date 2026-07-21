package pub

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProbeBackend_NoServer(t *testing.T) {
	t.Parallel()

	// Nothing listens on this URL; probe must return "".
	if got := ProbeBackend("http://127.0.0.1:1"); got != "" {
		t.Fatalf("expected empty URL, got %q", got)
	}
}

func TestResolveWorkdir(t *testing.T) {
	// CWD inside a temp dir resolves to itself.
	dir := t.TempDir()
	// macOS tempdirs are symlinked (/var -> /private/var); resolve so
	// the comparison with os.Getwd output matches.
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	if got := ResolveWorkdir(); got != dir {
		t.Fatalf("got %q, want %q", got, dir)
	}

	// CWD "/" (Finder launch) falls back to the home directory.
	t.Chdir("/")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir in test env")
	}
	if got := ResolveWorkdir(); got != home {
		t.Fatalf("got %q, want home %q", got, home)
	}
}

func TestFreePort(t *testing.T) {
	t.Parallel()

	port, err := FreePort()
	if err != nil {
		t.Fatalf("FreePort: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Fatalf("FreePort returned out-of-range port %d", port)
	}
}

func TestBackendCommand_FixedDefault(t *testing.T) {
	// Mutates env; no t.Parallel.
	t.Setenv("CODG_DESKTOP_CMD", "")
	// The Wails v3 shell default: `web` with no {port} token -> fixed port.
	args, port, err := BackendCommand("web")
	if err != nil {
		t.Fatalf("BackendCommand: %v", err)
	}
	if port != DefaultBackendPort {
		t.Fatalf("got port %d, want %d", port, DefaultBackendPort)
	}
	if len(args) != 1 || args[0] != "web" {
		t.Fatalf("got %v, want [web]", args)
	}
}

func TestBackendCommand_OwnedPortDefault(t *testing.T) {
	// Mutates env; no t.Parallel.
	t.Setenv("CODG_DESKTOP_CMD", "")
	// The Wails v2 shell default starts an owned backend on a free port.
	args, port, err := BackendCommand("web --port {port}")
	if err != nil {
		t.Fatalf("BackendCommand: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Fatalf("got invalid owned-backend port %d", port)
	}
	want := []string{"web", "--port", strconv.Itoa(port)}
	if len(args) != len(want) {
		t.Fatalf("got %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg %d: got %q, want %q", i, args[i], want[i])
		}
	}
}

func TestBackendCommand_EnvOverride(t *testing.T) {
	// Mutates env; no t.Parallel.
	t.Setenv("CODG_DESKTOP_CMD", "api")
	args, port, err := BackendCommand("web --port {port}")
	if err != nil {
		t.Fatalf("BackendCommand: %v", err)
	}
	if port != DefaultBackendPort {
		t.Fatalf("got port %d, want %d", port, DefaultBackendPort)
	}
	if len(args) != 1 || args[0] != "api" {
		t.Fatalf("got %v, want [api]", args)
	}
}

func TestResolveCodgBinary_EnvOverride(t *testing.T) {
	// Mutates env; no t.Parallel.
	bin := filepath.Join(t.TempDir(), "codg-fake")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODG_BIN", bin)

	got, err := ResolveCodgBinary()
	if err != nil {
		t.Fatalf("ResolveCodgBinary: %v", err)
	}
	if got != bin {
		t.Fatalf("got %q, want %q", got, bin)
	}
}

func TestResolveCodgBinary_EnvMissing(t *testing.T) {
	// Mutates env; no t.Parallel.
	t.Setenv("CODG_BIN", filepath.Join(t.TempDir(), "does-not-exist"))

	if _, err := ResolveCodgBinary(); err == nil {
		t.Fatal("expected error for missing CODG_BIN target, got nil")
	}
}

func TestValidateBackendURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		url     string
		want    string
		wantErr bool
	}{
		{"http://localhost:4096/", "http://localhost:4096", false},
		{"http://127.0.0.1:4096", "http://127.0.0.1:4096", false},
		{"http://[::1]:4096", "http://[::1]:4096", false},
		{"https://localhost:4096", "https://localhost:4096", false},
		{"https://example.com", "", true},
		{"file:///tmp/app", "", true},
		{"http://localhost:4096/prefix", "", true},
		{"http://user@localhost:4096", "", true},
	}
	for _, test := range tests {
		got, err := ValidateBackendURL(test.url)
		if test.wantErr {
			if err == nil {
				t.Fatalf("ValidateBackendURL(%q) = %q, want error", test.url, got)
			}
			continue
		}
		if err != nil || got != test.want {
			t.Fatalf("ValidateBackendURL(%q) = %q, %v; want %q", test.url, got, err, test.want)
		}
	}
}

func TestWaitBackendHealthyOrExit(t *testing.T) {
	t.Parallel()
	exited := make(chan struct{})
	close(exited)
	start := time.Now()
	err := WaitBackendHealthyOrExit("http://127.0.0.1:1", time.Second, exited)
	if err == nil || !strings.Contains(err.Error(), "exited") {
		t.Fatalf("WaitBackendHealthyOrExit = %v, want early-exit error", err)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("early backend exit was not detected promptly")
	}
}

func TestPortFromBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"http://localhost:4096", 4096, false},
		{"http://127.0.0.1:8080/", 8080, false},
		{"http://localhost", DefaultBackendPort, false}, // no port -> default
		{"http://localhost:notaport", 0, true},
		{"://bad", 0, true},
	}
	for _, tc := range tests {
		got, err := PortFromBaseURL(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("PortFromBaseURL(%q): expected error, got %d", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("PortFromBaseURL(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("PortFromBaseURL(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestKeepBackendOnExit(t *testing.T) {
	// Mutates env; no t.Parallel.
	tests := []struct {
		val  string
		want bool
	}{
		{"", false},
		{"0", false},
		{"false", false},
		{"1", true},
		{"true", true},
		{" YES ", true},
		{"on", true},
	}
	for _, tc := range tests {
		t.Setenv(KeepBackendEnv, tc.val)
		if got := KeepBackendOnExit(); got != tc.want {
			t.Fatalf("KeepBackendOnExit() with %s=%q = %v, want %v",
				KeepBackendEnv, tc.val, got, tc.want)
		}
	}
}

func TestExternalBackendShutdown(t *testing.T) {
	// Mutates env; no t.Parallel.

	// Default: exiting the desktop app stops the attached `codg web` CLI,
	// so a shutdown func must be returned.
	t.Setenv(KeepBackendEnv, "")
	if ExternalBackendShutdown(DefaultBackendURL) == nil {
		t.Fatal("expected shutdown func by default, got nil")
	}

	// Opt-out leaves the backend running: nil shutdown.
	t.Setenv(KeepBackendEnv, "1")
	if ExternalBackendShutdown(DefaultBackendURL) != nil {
		t.Fatalf("expected nil shutdown with %s=1", KeepBackendEnv)
	}
}

func TestStopExternalBackend_NoListener(t *testing.T) {
	t.Parallel()

	// A free port with nothing listening: must be a quiet no-op (no panic,
	// no signals sent, returns promptly instead of waiting the kill grace
	// period).
	port, err := FreePort()
	if err != nil {
		t.Fatalf("FreePort: %v", err)
	}
	done := make(chan struct{})
	go func() {
		StopExternalBackend(fmt.Sprintf("http://127.0.0.1:%d", port))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("StopExternalBackend did not return promptly for a dead port")
	}

	// Malformed URL: logged and ignored, never panics.
	StopExternalBackend("://not-a-url")
}
