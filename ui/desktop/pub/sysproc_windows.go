//go:build windows

package pub

import (
	"os/exec"
	"strconv"
	"syscall"
)

// ConfigureSysProcAttr starts the child in a new process group so console
// control events don't implicitly propagate to it; termination is handled
// explicitly via a tree kill (see KillProcessTree).
func ConfigureSysProcAttr(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
}

// InterruptProcessTree asks the backend to stop. Windows has no SIGINT for a
// detached process group, so fall straight through to a tree kill.
func InterruptProcessTree(cmd *exec.Cmd) error { return KillProcessTree(cmd) }

// KillProcessTree force-terminates the child and all of its descendants via
// `taskkill /T` (tree), reaping codg's LSP/MCP grandchildren too. Falls back
// to a direct kill if taskkill is unavailable.
func KillProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pid := strconv.Itoa(cmd.Process.Pid)
	if err := exec.Command("taskkill", "/F", "/T", "/PID", pid).Run(); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
