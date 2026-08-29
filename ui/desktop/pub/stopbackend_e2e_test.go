//go:build unix

package pub

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// fakeCodgSrc is a minimal stand-in for `codg web`: it serves /health on
// the port given as its first argument. SIGINT (what TerminatePid sends)
// terminates it via the default signal action, mirroring codg's clean
// Ctrl-C shutdown.
const fakeCodgSrc = `package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "ok")
	})
	if err := http.ListenAndServe("127.0.0.1:"+os.Args[1], nil); err != nil {
		panic(err)
	}
}
`

// TestStopExternalBackend_E2E builds a fake binary NAMED `codg`, runs it
// on a free port, and verifies StopExternalBackend — the default desktop
// exit behaviour for an attached `codg web` CLI — finds it by port,
// confirms its name, and terminates it.
func TestStopExternalBackend_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in -short mode")
	}
	// Spawns processes and depends on lsof/ps; no t.Parallel.

	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte(fakeCodgSrc), 0o644); err != nil {
		t.Fatalf("write fake codg source: %v", err)
	}
	bin := filepath.Join(dir, CodgBinaryName())
	if out, err := exec.Command("go", "build", "-o", bin, src).CombinedOutput(); err != nil {
		t.Skipf("cannot build fake codg (%v): %s", err, out)
	}

	port, err := FreePort()
	if err != nil {
		t.Fatalf("FreePort: %v", err)
	}
	cmd := exec.Command(bin, strconv.Itoa(port))
	if err := cmd.Start(); err != nil {
		t.Fatalf("start fake codg: %v", err)
	}
	exited := make(chan error, 1)
	go func() {
		// SIGINT death is the expected outcome; propagate Wait's result so the
		// test never silently discards a subprocess error.
		exited <- cmd.Wait()
		close(exited)
	}()
	defer func() {
		if kerr := cmd.Process.Kill(); kerr != nil && !errors.Is(kerr, os.ErrProcessDone) {
			t.Logf("cleanup kill: %v", kerr)
		}
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := WaitBackendHealthy(baseURL, 10*time.Second); err != nil {
		t.Fatalf("fake codg never became healthy: %v", err)
	}

	// The PID lookup must find exactly our fake codg.
	pids, err := CodgPidsOnPort(port)
	if err != nil {
		t.Fatalf("CodgPidsOnPort: %v", err)
	}
	if len(pids) != 1 || pids[0] != cmd.Process.Pid {
		t.Fatalf("CodgPidsOnPort = %v, want [%d]", pids, cmd.Process.Pid)
	}

	StopExternalBackend(baseURL)

	select {
	case waitErr := <-exited:
		if waitErr == nil {
			t.Log("fake codg exited cleanly")
		}
	case <-time.After(8 * time.Second):
		t.Fatal("fake codg still running after StopExternalBackend")
	}
	if ProbeBackend(baseURL) != "" {
		t.Fatal("backend still answering /health after StopExternalBackend")
	}
}
