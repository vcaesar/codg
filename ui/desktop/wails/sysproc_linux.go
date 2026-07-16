//go:build linux

package main

import "syscall"

// setPdeathsig makes the kernel send SIGTERM to the child if THIS process
// dies, so a hard crash of the shell never orphans the backend. Linux-only;
// other platforms rely on process-group signalling at shutdown.
func setPdeathsig(attr *syscall.SysProcAttr) {
	attr.Pdeathsig = syscall.SIGTERM
}
