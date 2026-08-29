//go:build windows

package pub

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// CodgPidsOnPort returns the PIDs of codg processes LISTENing on the given
// TCP port, parsed from `netstat -ano`. PIDs whose executable is not the
// codg binary are filtered out so an unrelated process squatting on the
// port is never signalled.
func CodgPidsOnPort(port int) ([]int, error) {
	out, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return nil, fmt.Errorf("netstat: %w", err)
	}
	suffix := fmt.Sprintf(":%d", port)
	seen := make(map[int]bool)
	var pids []int
	for line := range strings.SplitSeq(string(out), "\n") {
		// "  TCP    0.0.0.0:4096    0.0.0.0:0    LISTENING    1234"
		fields := strings.Fields(line)
		if len(fields) < 5 || !strings.EqualFold(fields[0], "TCP") {
			continue
		}
		if !strings.HasSuffix(fields[1], suffix) ||
			!strings.EqualFold(fields[3], "LISTENING") {
			continue
		}
		pid, aerr := strconv.Atoi(fields[4])
		if aerr != nil || pid <= 0 || seen[pid] {
			continue
		}
		if isCodgProcess(pid) {
			seen[pid] = true
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

// isCodgProcess reports whether pid's executable is the codg binary.
func isCodgProcess(pid int) bool {
	out, err := exec.Command("tasklist", "/FI",
		fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)),
		strings.ToLower(CodgBinaryName()))
}

// TerminatePid asks the process tree to stop. Windows has no SIGINT
// equivalent for unrelated processes; taskkill without /F sends WM_CLOSE
// (console apps may ignore it — KillPid follows after the grace period).
func TerminatePid(pid int) error {
	return exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T").Run()
}

// KillPid force-terminates the process tree.
func KillPid(pid int) error {
	return exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
}
