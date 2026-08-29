//go:build unix

package pub

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// CodgPidsOnPort returns the PIDs of codg processes LISTENing on the given
// TCP port. PIDs whose executable is not the codg binary are filtered out
// so an unrelated process squatting on the port is never signalled.
func CodgPidsOnPort(port int) ([]int, error) {
	// -t: PIDs only, one per line. lsof exits 1 with empty output when
	// nothing matches — that is "no backend", not an error.
	out, err := exec.Command("lsof", "-nP", "-t",
		fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(strings.TrimSpace(string(out))) == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("lsof port %d: %w", port, err)
	}
	var pids []int
	for f := range strings.FieldsSeq(string(out)) {
		pid, aerr := strconv.Atoi(f)
		if aerr != nil {
			continue
		}
		if isCodgProcess(pid) {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

// isCodgProcess reports whether pid's executable is the codg binary.
func isCodgProcess(pid int) bool {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return false
	}
	return filepath.Base(strings.TrimSpace(string(out))) == CodgBinaryName()
}

// TerminatePid asks the process to stop gracefully (same signal Ctrl-C
// delivers, which `codg web` handles with a clean server shutdown).
func TerminatePid(pid int) error { return syscall.Kill(pid, syscall.SIGINT) }

// KillPid force-terminates the process.
func KillPid(pid int) error { return syscall.Kill(pid, syscall.SIGKILL) }
