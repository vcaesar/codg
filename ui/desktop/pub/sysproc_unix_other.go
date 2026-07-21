//go:build unix && !linux

package pub

import "syscall"

// setPdeathsig is a no-op outside Linux (Pdeathsig is unavailable, e.g. on
// macOS/BSD). Orphans are still avoided via process-group signalling.
func setPdeathsig(_ *syscall.SysProcAttr) {}
