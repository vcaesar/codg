// Subprocess launcher shared by the desktop shells (Wails v2 and v3).
//
// The desktop shells do NOT compile the codg agent stack into their own
// binaries. Instead they launch the standalone `codg` binary as a child
// process running its web/API server. This file owns everything about that
// child: resolving the binary, picking the command/port, spawning, health
// probing, and stopping owned or attached backends on exit.
package pub

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBackendPort matches the --port default of `codg web` /
	// `codg desktop`. Used only when probing an already-running backend.
	DefaultBackendPort = 4096
	// DefaultBackendURL is where a manually-started codg backend listens
	// by default; the shell probes it before spawning its own child.
	DefaultBackendURL = "http://localhost:4096"
	// KeepBackendEnv, when set truthy, leaves an ATTACHED `codg web` CLI
	// backend (one the shell probed and reused, not spawned) running when
	// the desktop app exits. Default is to stop it on exit.
	KeepBackendEnv = "CODG_DESKTOP_KEEP_BACKEND"
	// PortToken, when present in the command spec, is replaced with a free
	// port the shell picks (and the WebView is pointed at that port).
	// Without it the backend is assumed to listen on DefaultBackendPort.
	PortToken = "{port}"
)

// ValidateBackendURL accepts only root-level HTTP(S) loopback URLs. The Wails
// bridge is privileged, so remote origins and path-prefixed proxies are never
// trusted as desktop backends.
func ValidateBackendURL(baseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", fmt.Errorf("parse backend URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("backend URL scheme must be http or https")
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return "", fmt.Errorf("backend URL must use a loopback host")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("backend URL must not contain a path prefix")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return "", fmt.Errorf("backend URL must not contain credentials, query, or fragment")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

// ProbeBackend returns baseURL when a codg backend answers its /health
// endpoint there, or "" when nothing is listening.
func ProbeBackend(baseURL string) string {
	client := &http.Client{Timeout: 700 * time.Millisecond}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return ""
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			slog.Warn("Failed to close probe response body", "err", cerr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	return baseURL
}

// ResolveWorkdir returns a sane working directory for the child backend.
// Finder launches .app bundles with CWD set to "/", which is not writable
// and not a meaningful project dir — fall back to $HOME.
func ResolveWorkdir() string {
	cwd, err := os.Getwd()
	if err != nil || cwd == "/" {
		if home, herr := os.UserHomeDir(); herr == nil {
			return home
		}
	}
	if cwd == "" {
		return "."
	}
	return cwd
}

// ResolveCodgBinary locates the codg CLI binary the shell will launch.
// Resolution order:
//  1. CODG_BIN — explicit path override.
//  2. A codg binary sitting next to the desktop executable (bundled in
//     the packaged app — e.g. Contents/MacOS/codg on macOS).
//  3. The first codg on PATH.
func ResolveCodgBinary() (string, error) {
	if env := os.Getenv("CODG_BIN"); env != "" {
		if _, err := os.Stat(env); err != nil {
			return "", fmt.Errorf("CODG_BIN=%q not usable: %w", env, err)
		}
		return env, nil
	}
	if exe, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(exe), CodgBinaryName())
		if _, serr := os.Stat(sibling); serr == nil {
			return sibling, nil
		}
	}
	path, err := exec.LookPath(CodgBinaryName())
	if err != nil {
		return "", fmt.Errorf("codg binary not found: set CODG_BIN, bundle it "+
			"next to the app, or add it to PATH: %w", err)
	}
	return path, nil
}

// FreePort asks the OS for an unused TCP port and returns it. The listener
// is closed immediately; there is a small race window before the child
// binds the port, which is acceptable for a one-shot desktop launch.
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() {
		if cerr := l.Close(); cerr != nil {
			slog.Warn("Failed to close port-probe listener", "err", cerr)
		}
	}()
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address type %T", l.Addr())
	}
	return addr.Port, nil
}

// BackendCommand resolves the child codg invocation: the argv (after the
// binary name) and the port the backend will listen on. CODG_DESKTOP_CMD
// overrides defaultSpec (the shell's built-in command). When the spec
// contains the {port} token it is replaced with a free port the shell picks;
// otherwise the backend is assumed to use the fixed DefaultBackendPort (the
// case for `web 0` / `api`).
func BackendCommand(defaultSpec string) ([]string, int, error) {
	spec := os.Getenv("CODG_DESKTOP_CMD")
	if spec == "" {
		spec = defaultSpec
	}

	port := DefaultBackendPort
	if strings.Contains(spec, PortToken) {
		p, err := FreePort()
		if err != nil {
			return nil, 0, fmt.Errorf("pick free port: %w", err)
		}
		port = p
		spec = strings.ReplaceAll(spec, PortToken, strconv.Itoa(port))
	}

	args := strings.Fields(spec)
	if len(args) == 0 {
		return nil, 0, fmt.Errorf("empty backend command spec")
	}
	return args, port, nil
}

// StartBackendProcess launches the codg web/API server as a child process
// (using defaultSpec unless CODG_DESKTOP_CMD overrides it) and waits until
// its /health endpoint answers. It returns the base URL and a shutdown func
// that gracefully terminates the child, or an error when the binary could
// not be found or did not become healthy.
func StartBackendProcess(ctx context.Context, defaultSpec string) (string, func(), error) {
	bin, err := ResolveCodgBinary()
	if err != nil {
		return "", nil, err
	}

	args, port, err := BackendCommand(defaultSpec)
	if err != nil {
		return "", nil, err
	}

	// A child context lets shutdown() terminate the process without
	// touching the caller's ctx, while ctx cancellation (app quit) still
	// propagates down and stops the child.
	procCtx, cancelProc := context.WithCancel(ctx)

	cmd := exec.CommandContext(procCtx, bin, args...)
	cmd.Dir = ResolveWorkdir()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	// Run the child in its own process group so the whole backend tree
	// (codg + its LSP/MCP/PTY helpers) can be reaped together and never
	// orphans on shell exit (see sysproc_*.go).
	ConfigureSysProcAttr(cmd)
	// Graceful cancellation signals the process group. shutdown below owns the
	// hard-kill deadline so descendants cannot outlive a directly killed child.
	cmd.Cancel = func() error { return InterruptProcessTree(cmd) }

	if err := cmd.Start(); err != nil {
		cancelProc()
		return "", nil, fmt.Errorf("start %s %v: %w", bin, args, err)
	}

	// Reap the child in the background so it never becomes a zombie.
	waitDone := make(chan struct{})
	go func() {
		if werr := cmd.Wait(); werr != nil {
			slog.Warn("codg backend exited", "err", werr)
		}
		close(waitDone)
	}()

	shutdown := func() {
		cancelProc()
		select {
		case <-waitDone:
		case <-time.After(5 * time.Second):
			slog.Warn("codg backend shutdown timed out; force-killing tree")
			if kerr := KillProcessTree(cmd); kerr != nil {
				slog.Warn("force-kill backend tree", "err", kerr)
			}
		}
	}

	baseURL := fmt.Sprintf("http://localhost:%d", port)
	if err := WaitBackendHealthyOrExit(baseURL, 20*time.Second, waitDone); err != nil {
		shutdown()
		return "", nil, fmt.Errorf("codg backend not healthy: %w", err)
	}
	slog.Info("codg backend running", "url", baseURL, "bin", bin,
		"pid", cmd.Process.Pid, "workdir", cmd.Dir)
	return baseURL, shutdown, nil
}

// KeepBackendOnExit reports whether the user opted out of stopping an
// attached (externally started) codg backend when the desktop app exits.
func KeepBackendOnExit() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(KeepBackendEnv))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// ExternalBackendShutdown returns the shutdown func for a codg backend the
// shell ATTACHED to (a `codg web` CLI already running on the default port)
// rather than spawned. By default exiting the desktop app stops that CLI
// too, so quitting never leaves a stray server behind; set
// CODG_DESKTOP_KEEP_BACKEND=1 to leave it running instead (returns nil).
func ExternalBackendShutdown(baseURL string) func() {
	if KeepBackendOnExit() {
		slog.Info("Will leave external codg backend running on exit",
			"url", baseURL, "reason", KeepBackendEnv)
		return nil
	}
	return func() { StopExternalBackend(baseURL) }
}

// PortFromBaseURL extracts the TCP port from a backend base URL, falling
// back to DefaultBackendPort when the URL carries no explicit port.
func PortFromBaseURL(baseURL string) (int, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return 0, fmt.Errorf("parse backend url %q: %w", baseURL, err)
	}
	p := u.Port()
	if p == "" {
		return DefaultBackendPort, nil
	}
	port, err := strconv.Atoi(p)
	if err != nil || port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid backend port %q in %q", p, baseURL)
	}
	return port, nil
}

// StopExternalBackend stops a `codg web`/`codg api` CLI process the shell
// did not spawn (so there is no *exec.Cmd handle to signal): it resolves
// the PID(s) listening on the backend port, verifies they belong to a codg
// binary (never kills an unrelated process squatting on the port), asks
// them to stop gracefully, and force-kills after a short grace period.
func StopExternalBackend(baseURL string) {
	port, err := PortFromBaseURL(baseURL)
	if err != nil {
		slog.Warn("Cannot stop external codg backend", "err", err)
		return
	}
	pids, err := CodgPidsOnPort(port)
	if err != nil {
		slog.Warn("Find external codg backend by port", "port", port, "err", err)
		return
	}
	if len(pids) == 0 {
		return // already gone, or not a codg process — leave it alone
	}
	for _, pid := range pids {
		if terr := TerminatePid(pid); terr != nil {
			slog.Warn("Stop external codg backend", "pid", pid, "err", terr)
		}
	}
	// Grace period: wait for /health to stop answering, then hard-kill.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ProbeBackend(baseURL) == "" {
			slog.Info("External codg backend stopped", "port", port, "pids", pids)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	slog.Warn("External codg backend ignored interrupt; force-killing",
		"port", port, "pids", pids)
	for _, pid := range pids {
		if kerr := KillPid(pid); kerr != nil {
			slog.Warn("Force-kill external codg backend", "pid", pid, "err", kerr)
		}
	}
}

// WaitBackendHealthy polls /health until 200 OK or the timeout elapses.
func WaitBackendHealthy(baseURL string, timeout time.Duration) error {
	return WaitBackendHealthyOrExit(baseURL, timeout, nil)
}

// WaitBackendHealthyOrExit is WaitBackendHealthy that additionally fails
// fast when the backend process exits before ever becoming healthy.
func WaitBackendHealthyOrExit(baseURL string, timeout time.Duration, exited <-chan struct{}) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		if ProbeBackend(baseURL) != "" {
			return nil
		}
		select {
		case <-ticker.C:
		case <-timer.C:
			return fmt.Errorf("timed out waiting for %s/health", baseURL)
		case <-exited:
			return fmt.Errorf("backend process exited before becoming healthy")
		}
	}
}
