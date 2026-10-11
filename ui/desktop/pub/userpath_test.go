package pub

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMergePathLists(t *testing.T) {
	t.Parallel()

	sep := string(os.PathListSeparator)
	got := MergePathLists("/usr/bin"+sep+"/bin", "", "/opt/homebrew/bin"+sep+"/usr/bin"+sep+sep+"/x")
	want := strings.Join([]string{"/usr/bin", "/bin", "/opt/homebrew/bin", "/x"}, sep)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExtractMarkedPath(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"banner\n" + pathMarker + "/a:/b" + pathMarker + "trailer": "/a:/b",
		pathMarker + "/a":   "",
		"no markers at all": "",
		"":                  "",
	}
	for in, want := range cases {
		if got := extractMarkedPath(in); got != want {
			t.Fatalf("extractMarkedPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestEnsureUserPath_FindsCodgFromLoginShell reproduces a Finder launch:
// PATH is launchd's minimal default and codg is only reachable via the
// user's login-shell PATH.
func TestEnsureUserPath_FindsCodgFromLoginShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH repair is a no-op on Windows")
	}
	// Mutates env; no t.Parallel.
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, CodgBinaryName()), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Fake login shell that prints a PATH containing binDir.
	shell := filepath.Join(t.TempDir(), "fakesh")
	script := "#!/bin/sh\necho 'rc noise'\nprintf '%s%s%s' " + pathMarker + " '" + binDir + "' " + pathMarker + "\n"
	if err := os.WriteFile(shell, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", shell)
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")
	t.Setenv("CODG_BIN", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOPATH", "")

	EnsureUserPath()

	path := os.Getenv("PATH")
	if !strings.HasPrefix(path, "/usr/bin:/bin:/usr/sbin:/sbin:") {
		t.Fatalf("original PATH entries must stay first, got %q", path)
	}
	if !strings.Contains(path, "/opt/homebrew/bin") {
		t.Fatalf("expected common install dirs in PATH, got %q", path)
	}
	got, err := ResolveCodgBinary()
	if err != nil {
		t.Fatalf("ResolveCodgBinary: %v", err)
	}
	// Login-shell entries precede the fallback dirs, so binDir wins.
	if want := filepath.Join(binDir, CodgBinaryName()); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
