// codg auto-install/update. A desktop shell with no codg on the machine
// would otherwise only show the offline static UI, so the binary is fetched
// from the latest GitHub release; an installed codg older than the desktop
// app is updated in place the same way.
package pub

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// codgReleaseAPI describes the newest codg release; a variable so tests can
// point it at a fake server.
var codgReleaseAPI = "https://api.github.com/repos/vcaesar/codg/releases/latest"

// maxArchiveSize caps a downloaded release archive (current ones are ~30MB).
const maxArchiveSize = 256 << 20

var versionRe = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

type codgRelease struct {
	TagName string      `json:"tag_name"`
	Assets  []codgAsset `json:"assets"`
}

type codgAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest,omitempty"`
}

// ManagedCodgPath is where codg is installed when none exists yet.
func ManagedCodgPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".codg", "bin", CodgBinaryName()), nil
}

// EnsureCodgBinary resolves the codg binary like ResolveCodgBinary, but
// installs the latest GitHub release into ManagedCodgPath when none is found
// and updates the binary in place when `codg -v` is older than minVersion.
// An explicit CODG_BIN is never replaced. Update failures are logged and the
// existing binary is kept, since an older backend beats no backend.
func EnsureCodgBinary(ctx context.Context, minVersion string) (string, error) {
	explicit := os.Getenv("CODG_BIN") != ""
	bin, err := ResolveCodgBinary()
	if err != nil {
		if explicit {
			return "", err
		}
		dest, derr := ManagedCodgPath()
		if derr != nil {
			return "", errors.Join(err, derr)
		}
		slog.Info("codg not found; installing from GitHub", "dest", dest)
		if _, ierr := InstallLatestCodg(ctx, dest, ""); ierr != nil {
			return "", fmt.Errorf("install codg: %w", errors.Join(err, ierr))
		}
		return dest, nil
	}
	if explicit {
		return bin, nil
	}
	current, err := CodgVersion(ctx, bin)
	if err != nil {
		slog.Warn("Read codg version", "bin", bin, "err", err)
		return bin, nil
	}
	if !VersionLess(current, minVersion) {
		return bin, nil
	}
	tag, err := InstallLatestCodg(ctx, bin, current)
	if err != nil {
		slog.Warn("Update codg", "bin", bin, "current", current, "err", err)
		return bin, nil
	}
	if tag != "" {
		slog.Info("Updated codg", "bin", bin, "from", current, "to", tag)
	}
	return bin, nil
}

// CodgVersion runs `bin -v` and returns the version it reports
// (e.g. "2.5.12" from "codg version 2.5.12").
func CodgVersion(ctx context.Context, bin string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "-v").Output()
	if err != nil {
		return "", fmt.Errorf("run %s -v: %w", bin, err)
	}
	v := versionRe.FindString(string(out))
	if v == "" {
		return "", fmt.Errorf("no version in %q", strings.TrimSpace(string(out)))
	}
	return v, nil
}

// VersionLess reports whether version a is older than b. Unparseable
// versions (dev builds) never compare as older.
func VersionLess(a, b string) bool {
	pa, oka := parseVersion(a)
	pb, okb := parseVersion(b)
	if !oka || !okb {
		return false
	}
	for i := range pa {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

func parseVersion(s string) ([3]int, bool) {
	var v [3]int
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return v, false
	}
	for i, part := range m[1:] {
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// InstallLatestCodg downloads the latest codg release for this platform and
// atomically writes its binary to dest. When current is set and the latest
// release is not newer, nothing is downloaded and "" is returned; otherwise
// the installed release tag is returned.
func InstallLatestCodg(ctx context.Context, dest, current string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Minute}
	var rel codgRelease
	if err := getJSON(ctx, client, codgReleaseAPI, &rel); err != nil {
		return "", fmt.Errorf("fetch latest release: %w", err)
	}
	if current != "" && !VersionLess(current, rel.TagName) {
		slog.Info("codg is already the latest release", "current", current, "latest", rel.TagName)
		return "", nil
	}
	base := fmt.Sprintf("codg_%s_%s", runtime.GOOS, runtime.GOARCH)
	var asset *codgAsset
	for i := range rel.Assets {
		if rel.Assets[i].Name == base+".zip" {
			asset = &rel.Assets[i]
			break
		}
	}
	if asset == nil {
		return "", fmt.Errorf("release %s has no asset %s.zip", rel.TagName, base)
	}
	archive, err := fetch(ctx, client, asset.URL, "application/octet-stream")
	if err != nil {
		return "", fmt.Errorf("download %s: %w", asset.Name, err)
	}
	if err := verifyDigest(archive, asset.Digest); err != nil {
		return "", fmt.Errorf("verify %s: %w", asset.Name, err)
	}
	if err := extractCodg(archive, base, dest); err != nil {
		return "", fmt.Errorf("extract %s: %w", asset.Name, err)
	}
	return rel.TagName, nil
}

func getJSON(ctx context.Context, client *http.Client, url string, v any) error {
	body, err := fetch(ctx, client, url, "application/vnd.github+json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}

func fetch(ctx context.Context, client *http.Client, url, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", accept)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			slog.Warn("Failed to close response body", "url", url, "err", cerr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get %s: status %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxArchiveSize+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	if len(body) > maxArchiveSize {
		return nil, fmt.Errorf("%s exceeds %d bytes", url, maxArchiveSize)
	}
	return body, nil
}

// verifyDigest checks data against a GitHub asset digest ("sha256:<hex>");
// an empty digest (older releases) is accepted.
func verifyDigest(data []byte, digest string) error {
	if digest == "" {
		return nil
	}
	want, ok := strings.CutPrefix(digest, "sha256:")
	if !ok {
		return fmt.Errorf("unsupported digest %q", digest)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, want) {
		return fmt.Errorf("sha256 mismatch: got %s, want %s", got, want)
	}
	return nil
}

// extractCodg writes the codg executable from a release zip (an entry named
// codg_<os>_<arch>[.exe] or codg[.exe]) to dest via a temp file + rename, so
// an interrupted install never leaves a truncated binary behind.
func extractCodg(archive []byte, base, dest string) error {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	var entry *zip.File
	for _, f := range zr.File {
		name := strings.TrimSuffix(filepath.Base(f.Name), ".exe")
		if !f.FileInfo().IsDir() && (name == base || name == "codg") {
			entry = f
			break
		}
	}
	if entry == nil {
		return fmt.Errorf("no codg binary in archive")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".codg-install-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	if err := writeEntry(tmp, entry); err != nil {
		return errors.Join(err, os.Remove(tmp.Name()))
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return errors.Join(fmt.Errorf("replace %s: %w", dest, err), os.Remove(tmp.Name()))
	}
	return nil
}

func writeEntry(tmp *os.File, entry *zip.File) error {
	rc, err := entry.Open()
	if err != nil {
		return errors.Join(fmt.Errorf("open %s: %w", entry.Name, err), tmp.Close())
	}
	_, err = io.Copy(tmp, rc)
	err = errors.Join(err, rc.Close())
	if err == nil {
		err = tmp.Chmod(0o755)
	}
	return errors.Join(err, tmp.Close())
}
