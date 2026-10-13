// Package scripts tests the desktop release workflow without publishing assets.
package scripts

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type releaseStep struct {
	Name             string
	Uses             string
	With             map[string]string
	Env              map[string]string
	Run              string
	WorkingDirectory string `yaml:"working-directory"`
}

type releaseJob struct {
	Needs   []string
	Outputs map[string]string
	Steps   []releaseStep
}

type releaseWorkflow struct {
	On struct {
		Push             struct{ Tags []string }
		WorkflowDispatch struct {
			Inputs map[string]struct {
				Required bool
				Type     string
			}
		} `yaml:"workflow_dispatch"`
	}
	Concurrency struct {
		Group            string
		CancelInProgress bool `yaml:"cancel-in-progress"`
	}
	Jobs map[string]releaseJob
}

func readReleaseWorkflow(t *testing.T) releaseWorkflow {
	t.Helper()
	data, err := os.ReadFile("../.github/workflows/release-desktop.yml")
	require.NoError(t, err)
	var workflow releaseWorkflow
	require.NoError(t, yaml.Unmarshal(data, &workflow))
	return workflow
}

func releaseStepByName(t *testing.T, job releaseJob, name string) releaseStep {
	t.Helper()
	for _, step := range job.Steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("release step %q not found", name)
	return releaseStep{}
}

func runReleaseScript(t *testing.T, dir, script string, env ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func readReleaseFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, err)
	return string(data)
}

func TestDesktopReleaseWorkflow(t *testing.T) {
	t.Parallel()
	workflow := readReleaseWorkflow(t)
	require.ElementsMatch(t, []string{"v[0-9]*", "desktop-v[0-9]*"}, workflow.On.Push.Tags)
	require.True(t, workflow.On.WorkflowDispatch.Inputs["tag"].Required)
	require.Equal(t, "string", workflow.On.WorkflowDispatch.Inputs["tag"].Type)
	require.Equal(t, "release-desktop-${{ inputs.tag || github.ref_name }}", workflow.Concurrency.Group)
	require.False(t, workflow.Concurrency.CancelInProgress)

	prepare := workflow.Jobs["prepare"]
	resolve := releaseStepByName(t, prepare, "Resolve version")
	require.Equal(t, "${{ inputs.tag || github.ref_name }}", resolve.Env["TAG"])
	require.NotContains(t, resolve.Run, "${{", "tag input must not be interpolated into shell source")
	for _, output := range []string{"tag", "version", "backend_tag", "prerelease"} {
		require.Equal(t, "${{ steps.ver.outputs."+output+" }}", prepare.Outputs[output])
	}
	for _, name := range []string{"build", "publish"} {
		job := workflow.Jobs[name]
		require.Contains(t, job.Needs, "prepare")
		require.True(t, strings.HasPrefix(job.Steps[0].Uses, "actions/checkout@"))
		require.Equal(t, "refs/tags/${{ needs.prepare.outputs.tag }}", job.Steps[0].With["ref"])
	}
	require.Contains(t, workflow.Jobs["publish"].Needs, "build")
	fetch := releaseStepByName(t, workflow.Jobs["build"], "Fetch codg binary")
	require.Equal(t, "${{ needs.prepare.outputs.backend_tag }}", fetch.Env["BACKEND_TAG"])
	build := releaseStepByName(t, workflow.Jobs["build"], "Build and package")
	require.Equal(t, "${{ needs.prepare.outputs.version }}", build.Env["VERSION"])
	require.Equal(t, "1", build.Env["CODG_REQUIRE_BUNDLE"])
	for _, step := range workflow.Jobs["build"].Steps {
		require.NotContains(t, step.Run, "npm run build", "desktop-release.sh syncs the web UI itself")
	}
	publish := releaseStepByName(t, workflow.Jobs["publish"], "Publish GitHub release")
	require.Equal(t, "${{ needs.prepare.outputs.tag }}", publish.Env["TAG"])
	require.Equal(t, "${{ needs.prepare.outputs.prerelease }}", publish.Env["PRERELEASE"])
}

func TestDesktopReleaseShellSyntax(t *testing.T) {
	t.Parallel()
	for jobName, job := range readReleaseWorkflow(t).Jobs {
		for _, step := range job.Steps {
			if step.Run == "" {
				continue
			}
			t.Run(jobName+"/"+step.Name, func(t *testing.T) {
				t.Parallel()
				cmd := exec.Command("bash", "-n")
				cmd.Stdin = strings.NewReader(step.Run)
				output, err := cmd.CombinedOutput()
				require.NoError(t, err, string(output))
			})
		}
	}
}

func TestDesktopReleaseVersion(t *testing.T) {
	t.Parallel()
	step := releaseStepByName(t, readReleaseWorkflow(t).Jobs["prepare"], "Resolve version")
	for _, tt := range []struct {
		tag, version string
		prerelease   bool
	}{
		{tag: "v1.2.3", version: "1.2.3"},
		{tag: "desktop-v1.2.3", version: "1.2.3"},
		{tag: "v2.0.0-rc.1", version: "2.0.0-rc.1", prerelease: true},
		{tag: "desktop-v2.0.0-beta.2", version: "2.0.0-beta.2", prerelease: true},
		{tag: ""},
		{tag: "main"},
		{tag: "v1.2"},
		{tag: "vbad"},
		{tag: "desktop-v"},
		{tag: "v1.2.3/other"},
		{tag: "v1.2.3\nversion=9.9.9"},
		{tag: `v1.2.3$(touch injected)`},
	} {
		t.Run(tt.tag, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			output, err := runReleaseScript(t, dir, step.Run, "TAG="+tt.tag, "GITHUB_OUTPUT=outputs")
			if tt.version == "" {
				require.Error(t, err, output)
				require.NoFileExists(t, filepath.Join(dir, "outputs"))
				require.NoFileExists(t, filepath.Join(dir, "injected"))
				return
			}
			require.NoError(t, err, output)
			require.Equal(t, fmt.Sprintf("tag=%s\nversion=%s\nbackend_tag=v%s\nprerelease=%t\n",
				tt.tag, tt.version, tt.version, tt.prerelease), readReleaseFile(t, dir, "outputs"))
		})
	}
}

func TestDesktopReleaseWailsCLI(t *testing.T) {
	t.Parallel()
	step := releaseStepByName(t, readReleaseWorkflow(t).Jobs["build"], "Install Wails v2 CLI")
	require.Equal(t, "ui/desktop/wails2", step.WorkingDirectory)
	require.Contains(t, readReleaseFile(t, "..", step.WorkingDirectory+"/go.mod"), "github.com/wailsapp/wails/v2 ")

	const mocks = `
go() {
  printf '%s\n' "$*" >> go-calls
  if [ "$1" = list ]; then echo v2.16.0; fi
}
`
	dir := t.TempDir()
	output, err := runReleaseScript(t, dir, mocks+step.Run)
	require.NoError(t, err, output)
	require.Equal(t, "list -m -f {{.Version}} github.com/wailsapp/wails/v2\n"+
		"install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0\n", readReleaseFile(t, dir, "go-calls"))
}

func TestDesktopReleaseFetch(t *testing.T) {
	t.Parallel()
	step := releaseStepByName(t, readReleaseWorkflow(t).Jobs["build"], "Fetch codg binary")
	const mocks = `
attempts=0
curl() {
  attempts=$((attempts + 1))
  printf '%s\n' "${!#}" >> downloads
  if [ "$attempts" -le "$FAIL_DOWNLOADS" ]; then return 22; fi
  printf archive > ".codg-bin/$asset"
}
sleep() { printf '%s\n' "$1" >> sleeps; }
unzip() {
  if [ "$OMIT_BINARY" = false ]; then
    printf backend > ".codg-bin/$EXPECTED_BINARY"
  fi
}
chmod() { return 0; }
`
	for _, tt := range []struct {
		name, platform, binary string
		failDownloads          int
		omitBinary, wantError  bool
	}{
		{"linux-amd64", "linux/amd64", "codg_linux_amd64", 0, false, false},
		{"linux-arm64", "linux/arm64", "codg_linux_arm64", 2, false, false},
		{"darwin-arm64", "darwin/arm64", "codg_darwin_arm64", 2, false, false},
		{"darwin-amd64", "darwin/amd64", "codg_darwin_amd64", 2, false, false},
		{"windows-amd64", "windows/amd64", "codg_windows_amd64.exe", 2, false, false},
		{"unavailable", "linux/amd64", "codg_linux_amd64", 30, false, true},
		{"missing-binary", "linux/amd64", "codg_linux_amd64", 0, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			output, err := runReleaseScript(t, dir, mocks+step.Run,
				"PLATFORM="+tt.platform, "BACKEND_TAG=v1.2.3", "CODG_RELEASE_REPO=vcaesar/codg",
				"GITHUB_ENV=github-env", "EXPECTED_BINARY="+tt.binary,
				fmt.Sprintf("FAIL_DOWNLOADS=%d", tt.failDownloads), fmt.Sprintf("OMIT_BINARY=%t", tt.omitBinary))
			attempts := min(tt.failDownloads+1, 30)
			url := "https://github.com/vcaesar/codg/releases/download/v1.2.3/codg_" + strings.ReplaceAll(tt.platform, "/", "_") + ".zip\n"
			require.Equal(t, strings.Repeat(url, attempts), readReleaseFile(t, dir, "downloads"))
			if attempts > 1 {
				require.Equal(t, strings.Repeat("30\n", attempts-1), readReleaseFile(t, dir, "sleeps"))
			}
			if tt.wantError {
				require.Error(t, err, output)
				require.NoFileExists(t, filepath.Join(dir, "github-env"))
				return
			}
			require.NoError(t, err, output)
			require.Equal(t, "backend", readReleaseFile(t, dir, ".codg-bin/"+tt.binary))
			require.Contains(t, readReleaseFile(t, dir, "github-env"), "CODG_PREBUILT_BIN=")
			require.True(t, strings.HasSuffix(readReleaseFile(t, dir, "github-env"), "/.codg-bin/"+tt.binary+"\n"))
		})
	}
}

func TestDesktopReleaseChecksums(t *testing.T) {
	t.Parallel()
	step := releaseStepByName(t, readReleaseWorkflow(t).Jobs["publish"], "Checksums")
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "dist"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dist/checksums.txt"), []byte("CLI checksums"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dist/codg-desktop-linux-amd64.tar.gz"), []byte("package"), 0o600))
	output, err := runReleaseScript(t, dir, `sha256sum() { printf 'sha256 %s\n' "$@"; }`+"\n"+step.Run)
	require.NoError(t, err, output)
	require.Equal(t, "CLI checksums", readReleaseFile(t, dir, "dist/checksums.txt"))
	require.Equal(t, "sha256 codg-desktop-linux-amd64.tar.gz\n", readReleaseFile(t, dir, "dist/desktop-checksums.txt"))
}

func TestDesktopReleasePublish(t *testing.T) {
	t.Parallel()
	step := releaseStepByName(t, readReleaseWorkflow(t).Jobs["publish"], "Publish GitHub release")
	const mocks = `
gh() {
  printf '%s\n' "$*" >> gh-calls
  if [ "$2" = view ]; then
    [ "$EXISTING_RELEASE" = true ]
  else
    return "$PUBLISH_STATUS"
  fi
}
`
	for _, tt := range []struct {
		name, tag            string
		existing, prerelease bool
		publishStatus        int
	}{
		{"existing", "v1.2.3", true, false, 0},
		{"existing-prerelease", "v1.2.3-rc.1", true, true, 0},
		{"create", "desktop-v1.2.3", false, false, 0},
		{"create-prerelease", "desktop-v1.2.3-rc.1", false, true, 0},
		{"upload-failure", "v1.2.3", true, false, 1},
		{"create-failure", "desktop-v1.2.3", false, false, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(dir, "dist"), 0o755))
			for _, name := range []string{"codg-desktop-linux-amd64.tar.gz", "desktop-checksums.txt"} {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "dist", name), []byte("asset"), 0o600))
			}
			output, err := runReleaseScript(t, dir, mocks+step.Run, "TAG="+tt.tag,
				fmt.Sprintf("PRERELEASE=%t", tt.prerelease), fmt.Sprintf("EXISTING_RELEASE=%t", tt.existing),
				fmt.Sprintf("PUBLISH_STATUS=%d", tt.publishStatus))
			if tt.publishStatus != 0 {
				require.Error(t, err, output)
			} else {
				require.NoError(t, err, output)
			}
			calls := strings.Split(strings.TrimSpace(readReleaseFile(t, dir, "gh-calls")), "\n")
			require.Len(t, calls, 2)
			require.Equal(t, "release view "+tt.tag, calls[0])
			if tt.existing {
				require.Equal(t, "release upload "+tt.tag+" dist/codg-desktop-linux-amd64.tar.gz dist/desktop-checksums.txt --clobber", calls[1])
			} else {
				want := "release create " + tt.tag + " dist/codg-desktop-linux-amd64.tar.gz dist/desktop-checksums.txt --verify-tag --latest=false --title Codg Desktop " + tt.tag + " --generate-notes"
				if tt.prerelease {
					want += " --prerelease"
				}
				require.Equal(t, want, calls[1])
			}
		})
	}
}

// syncWebUI returns sync_web_ui extracted from a release script.
func syncWebUI(t *testing.T, script string) string {
	t.Helper()
	src := readReleaseFile(t, ".", script)
	start := strings.Index(src, "\nsync_web_ui() {\n")
	require.NotEqual(t, -1, start, script)
	end := strings.Index(src[start:], "\n}\n")
	require.NotEqual(t, -1, end, script)
	return src[start : start+end+3]
}

func writeWebBuild(t *testing.T, dir, name string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", name+".js"), []byte(name), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"),
		[]byte(`<!doctype html><script type="module" src="/assets/`+name+`.js"></script>`), 0o600))
}

func TestDesktopReleaseSyncWebUI(t *testing.T) {
	t.Parallel()
	// npm mock: `run build` emits ui/web/dist/web unless NPM_EMPTY is set.
	const mocks = `
npm() {
  printf '%s\n' "$*" >> "$CALLS"
  if [ "$1 $2" = "run build" ] && [ -z "${NPM_EMPTY:-}" ]; then
    mkdir -p dist/web/assets
    printf src > dist/web/assets/src.js
    printf '<script src="/assets/src.js"></script>' > dist/web/index.html
  fi
}
`
	for _, script := range []string{"desktop-release.sh", "v3/desktop-release.sh"} {
		fn := syncWebUI(t, script)
		for _, tt := range []struct {
			name                  string
			webSource, empty      bool
			pub                   func(t *testing.T, dir string)
			wantErr, wantJS, gone string
		}{
			{name: "source-replaces-pub", webSource: true,
				pub:    func(t *testing.T, dir string) { writeWebBuild(t, dir, "old") },
				wantJS: "assets/src.js", gone: "assets/old.js"},
			{name: "no-source-keeps-pub",
				pub:    func(t *testing.T, dir string) { writeWebBuild(t, dir, "pub") },
				wantJS: "assets/pub.js"},
			{name: "source-build-empty", webSource: true, empty: true,
				pub:     func(t *testing.T, dir string) { writeWebBuild(t, dir, "pub") },
				wantErr: "produced no index.html"},
			{name: "placeholder", wantErr: "placeholder",
				pub: func(t *testing.T, dir string) {
					require.NoError(t, os.MkdirAll(dir, 0o755))
					require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!-- CODG_PLACEHOLDER -->"), 0o600))
				}},
			{name: "missing-asset", wantErr: "references missing",
				pub: func(t *testing.T, dir string) {
					writeWebBuild(t, dir, "pub")
					require.NoError(t, os.Remove(filepath.Join(dir, "assets", "pub.js")))
				}},
			{name: "missing-index", wantErr: "no index.html",
				pub: func(t *testing.T, dir string) { require.NoError(t, os.MkdirAll(dir, 0o755)) }},
		} {
			t.Run(script+"/"+tt.name, func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				web, pub := filepath.Join(dir, "web"), filepath.Join(dir, "pub")
				require.NoError(t, os.MkdirAll(web, 0o755))
				if tt.webSource {
					require.NoError(t, os.WriteFile(filepath.Join(web, "package.json"), []byte("{}"), 0o600))
				}
				dist := filepath.Join(pub, "frontend", "dist")
				tt.pub(t, dist)
				env := []string{"WEB_DIR=" + web, "PUB_DIR=" + pub, "CALLS=" + filepath.Join(dir, "calls")}
				if tt.empty {
					env = append(env, "NPM_EMPTY=1")
				}
				output, err := runReleaseScript(t, dir, mocks+fn+"sync_web_ui\n", env...)
				if tt.wantErr != "" {
					require.Error(t, err, output)
					require.Contains(t, output, tt.wantErr)
					return
				}
				require.NoError(t, err, output)
				require.FileExists(t, filepath.Join(dist, tt.wantJS))
				if tt.gone != "" {
					require.NoFileExists(t, filepath.Join(dist, tt.gone))
				}
				if tt.webSource {
					require.Equal(t, "ci\nrun build\n", readReleaseFile(t, dir, "calls"))
				} else {
					require.NoFileExists(t, filepath.Join(dir, "calls"))
				}
			})
		}
	}
}
