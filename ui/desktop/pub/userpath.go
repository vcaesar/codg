// PATH repair for GUI launches. A packaged app started from Finder/Dock (or
// a Linux desktop launcher) inherits launchd's minimal PATH
// (/usr/bin:/bin:/usr/sbin:/sbin), so a codg installed via Homebrew,
// `go install`, or ~/.local/bin is invisible and the shell falls back to the
// offline static UI. `wails dev` works only because it inherits the
// terminal's PATH.
package pub

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// pathMarker brackets the PATH printed by the login shell so banners or
// other rc-file output can be stripped.
const pathMarker = "__CODG_PATH__"

// EnsureUserPath extends the process PATH with the user's login-shell PATH
// and common install dirs, so both codg lookup and the spawned backend (git,
// LSPs, node, ...) see the same tools as in a terminal. No-op on Windows.
func EnsureUserPath() {
	if runtime.GOOS == "windows" {
		return
	}
	merged := MergePathLists(os.Getenv("PATH"), loginShellPath(), strings.Join(extraBinDirs(), string(os.PathListSeparator)))
	if err := os.Setenv("PATH", merged); err != nil {
		slog.Warn("Failed to extend PATH", "err", err)
	}
}

// MergePathLists joins PATH-style lists in order, dropping empty and
// duplicate entries.
func MergePathLists(lists ...string) string {
	seen := make(map[string]bool)
	var out []string
	for _, list := range lists {
		for _, dir := range filepath.SplitList(list) {
			if dir == "" || seen[dir] {
				continue
			}
			seen[dir] = true
			out = append(out, dir)
		}
	}
	return strings.Join(out, string(os.PathListSeparator))
}

// extraBinDirs lists common user install locations that may be missing
// even from the login-shell PATH (e.g. PATH set only in a non-login rc).
func extraBinDirs() []string {
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin"}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs,
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, "go", "bin"),
			filepath.Join(home, ".codg", "bin"))
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		dirs = append(dirs, filepath.Join(gopath, "bin"))
	}
	return dirs
}

// loginShellPath returns the PATH an interactive login shell of the user
// would have, or "" when it cannot be determined in time.
func loginShellPath() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
		if runtime.GOOS != "darwin" {
			shell = "/bin/sh"
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, "-ilc",
		`printf '%s%s%s' "`+pathMarker+`" "$PATH" "`+pathMarker+`"`)
	// rc files may start background helpers holding stdout open; don't
	// block on them past the deadline.
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil {
		slog.Debug("Login shell PATH lookup failed", "shell", shell, "err", err)
	}
	return extractMarkedPath(string(out))
}

// extractMarkedPath returns the text between the first pair of pathMarkers.
func extractMarkedPath(out string) string {
	_, rest, ok := strings.Cut(out, pathMarker)
	if !ok {
		return ""
	}
	path, _, ok := strings.Cut(rest, pathMarker)
	if !ok {
		return ""
	}
	return strings.TrimSpace(path)
}
