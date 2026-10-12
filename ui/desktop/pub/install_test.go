package pub

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

func TestVersionLess(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a, b string
		want bool
	}{
		{"2.5.12", "2.6.1", true},
		{"2.6.1", "2.5.12", false},
		{"2.6.1", "2.6.1", false},
		{"v2.6", "2.6.1", true},
		{"2.10.0", "2.9.9", false},
		{"devel", "2.6.1", false},
		{"2.6.1", "", false},
	}
	for _, c := range cases {
		if got := VersionLess(c.a, c.b); got != c.want {
			t.Fatalf("VersionLess(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestVerifyDigest(t *testing.T) {
	t.Parallel()

	data := []byte("codg")
	sum := sha256.Sum256(data)
	good := "sha256:" + hex.EncodeToString(sum[:])
	if err := verifyDigest(data, good); err != nil {
		t.Fatalf("good digest: %v", err)
	}
	if err := verifyDigest(data, ""); err != nil {
		t.Fatalf("empty digest: %v", err)
	}
	if err := verifyDigest([]byte("tampered"), good); err == nil {
		t.Fatal("expected mismatch error")
	}
	if err := verifyDigest(data, "md5:abc"); err == nil {
		t.Fatal("expected unsupported digest error")
	}
}

// fakeScript returns a shell script that prints "codg version <v>".
func fakeScript(v string) []byte {
	return []byte("#!/bin/sh\necho 'codg version " + v + "'\n")
}

func zipWith(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// fakeRelease serves a GitHub "latest release" document for tag whose
// platform asset is archive, and points codgReleaseAPI at it. It returns a
// counter of archive downloads.
func fakeRelease(t *testing.T, tag string, archive []byte, digest string) *atomic.Int32 {
	t.Helper()
	var downloads atomic.Int32
	asset := "codg_" + runtime.GOOS + "_" + runtime.GOARCH + ".zip"
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/latest", func(w http.ResponseWriter, _ *http.Request) {
		rel := codgRelease{TagName: tag, Assets: []codgAsset{
			{Name: "codg_other_os.zip", URL: srv.URL + "/other"},
			{Name: asset, URL: srv.URL + "/asset", Digest: digest},
		}}
		if err := json.NewEncoder(w).Encode(rel); err != nil {
			t.Error(err)
		}
	})
	mux.HandleFunc("/asset", func(w http.ResponseWriter, _ *http.Request) {
		downloads.Add(1)
		if _, err := w.Write(archive); err != nil {
			t.Error(err)
		}
	})
	old := codgReleaseAPI
	codgReleaseAPI = srv.URL + "/latest"
	t.Cleanup(func() { codgReleaseAPI = old })
	return &downloads
}

func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func TestInstallLatestCodg(t *testing.T) {
	// Mutates codgReleaseAPI; no t.Parallel.
	base := "codg_" + runtime.GOOS + "_" + runtime.GOARCH
	content := []byte("new-binary")
	archive := zipWith(t, base, content)
	downloads := fakeRelease(t, "v9.9.9", archive, sha(archive))
	dest := filepath.Join(t.TempDir(), "sub", CodgBinaryName())

	tag, err := InstallLatestCodg(context.Background(), dest, "")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if tag != "v9.9.9" {
		t.Fatalf("tag = %q", tag)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("dest content = %q", got)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(dest)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("dest not executable: %v", info.Mode())
		}
	}

	// Already on the latest release: no download.
	tag, err = InstallLatestCodg(context.Background(), dest, "9.9.9")
	if err != nil || tag != "" {
		t.Fatalf("up-to-date install = %q, %v", tag, err)
	}
	if n := downloads.Load(); n != 1 {
		t.Fatalf("downloads = %d, want 1", n)
	}
}

func TestInstallLatestCodg_RejectsBadDigest(t *testing.T) {
	// Mutates codgReleaseAPI; no t.Parallel.
	archive := zipWith(t, "codg", []byte("x"))
	fakeRelease(t, "v9.9.9", archive, sha([]byte("other")))
	dest := filepath.Join(t.TempDir(), CodgBinaryName())

	if _, err := InstallLatestCodg(context.Background(), dest, ""); err == nil ||
		!strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected digest error, got %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("dest must not exist after failed install: %v", err)
	}
}

func TestInstallLatestCodg_NoBinaryInArchive(t *testing.T) {
	// Mutates codgReleaseAPI; no t.Parallel.
	archive := zipWith(t, "README.md", []byte("x"))
	fakeRelease(t, "v9.9.9", archive, "")
	dir := t.TempDir()

	if _, err := InstallLatestCodg(context.Background(), filepath.Join(dir, CodgBinaryName()), ""); err == nil {
		t.Fatal("expected error for archive without codg")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("leftover files: %v", entries)
	}
}

// isolateLookup makes ResolveCodgBinary see only binDir.
func isolateLookup(t *testing.T, binDir string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CODG_BIN", "")
	t.Setenv("PATH", binDir)
	return home
}

func TestEnsureCodgBinary_InstallsWhenMissing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses shell-script fake binaries")
	}
	home := isolateLookup(t, t.TempDir())
	archive := zipWith(t, "codg_"+runtime.GOOS+"_"+runtime.GOARCH, fakeScript("9.9.9"))
	fakeRelease(t, "v9.9.9", archive, sha(archive))

	bin, err := EnsureCodgBinary(context.Background(), "2.6.1")
	if err != nil {
		t.Fatalf("EnsureCodgBinary: %v", err)
	}
	if want := filepath.Join(home, ".codg", "bin", CodgBinaryName()); bin != want {
		t.Fatalf("bin = %q, want %q", bin, want)
	}
	if v, err := CodgVersion(context.Background(), bin); err != nil || v != "9.9.9" {
		t.Fatalf("installed version = %q, %v", v, err)
	}
}

func TestEnsureCodgBinary_UpdatesOldInPlace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses shell-script fake binaries")
	}
	binDir := t.TempDir()
	isolateLookup(t, binDir)
	old := filepath.Join(binDir, CodgBinaryName())
	if err := os.WriteFile(old, fakeScript("1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	archive := zipWith(t, "codg_"+runtime.GOOS+"_"+runtime.GOARCH, fakeScript("9.9.9"))
	fakeRelease(t, "v9.9.9", archive, sha(archive))

	bin, err := EnsureCodgBinary(context.Background(), "2.6.1")
	if err != nil {
		t.Fatalf("EnsureCodgBinary: %v", err)
	}
	if bin != old {
		t.Fatalf("bin = %q, want %q", bin, old)
	}
	if v, err := CodgVersion(context.Background(), bin); err != nil || v != "9.9.9" {
		t.Fatalf("updated version = %q, %v", v, err)
	}
}

func TestEnsureCodgBinary_KeepsCurrentAndExplicit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses shell-script fake binaries")
	}
	binDir := t.TempDir()
	isolateLookup(t, binDir)
	bin := filepath.Join(binDir, CodgBinaryName())
	if err := os.WriteFile(bin, fakeScript("1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	downloads := fakeRelease(t, "v9.9.9", zipWith(t, "codg", fakeScript("9.9.9")), "")

	// New enough: untouched.
	if got, err := EnsureCodgBinary(context.Background(), "1.0.0"); err != nil || got != bin {
		t.Fatalf("current = %q, %v", got, err)
	}
	// Explicit CODG_BIN is never replaced, even when old.
	t.Setenv("CODG_BIN", bin)
	if got, err := EnsureCodgBinary(context.Background(), "2.6.1"); err != nil || got != bin {
		t.Fatalf("explicit = %q, %v", got, err)
	}
	if n := downloads.Load(); n != 0 {
		t.Fatalf("downloads = %d, want 0", n)
	}
	if v, err := CodgVersion(context.Background(), bin); err != nil || v != "1.0.0" {
		t.Fatalf("version = %q, %v", v, err)
	}
}

func TestEnsureCodgBinary_UpdateFailureKeepsOld(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses shell-script fake binaries")
	}
	binDir := t.TempDir()
	isolateLookup(t, binDir)
	bin := filepath.Join(binDir, CodgBinaryName())
	if err := os.WriteFile(bin, fakeScript("1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	old := codgReleaseAPI
	codgReleaseAPI = srv.URL
	t.Cleanup(func() { codgReleaseAPI = old })

	if got, err := EnsureCodgBinary(context.Background(), "2.6.1"); err != nil || got != bin {
		t.Fatalf("got %q, %v; want old binary kept", got, err)
	}
}
