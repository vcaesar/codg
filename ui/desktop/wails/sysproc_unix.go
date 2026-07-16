//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

// configureSysProcAttr puts the child in its own process group so the whole
// tree (codg plus the LSP/MCP/PTY helpers it spawns) can be signalled as a
// unit, and — on Linux — dies if this shell dies (see setPdeathsig). Without
// this, a hard crash of the shell would orphan the backend, which (running
// on a random free port) would never be found and reused, accumulating
// stale processes holding ports and DB locks.
func configureSysProcAttr(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	setPdeathsig(cmd.SysProcAttr)
}

// signalGroup sends sig to the child's entire process group. A negative PID
// targets the group led by the child — Setpgid above makes the child a group
// leader, so its PID equals the PGID.
func signalGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, sig)
}

// interruptProcessTree asks the backend tree to stop gracefully.
func interruptProcessTree(cmd *exec.Cmd) error { return signalGroup(cmd, syscall.SIGINT) }

// killProcessTree force-terminates the backend tree.
func killProcessTree(cmd *exec.Cmd) error { return signalGroup(cmd, syscall.SIGKILL) }
