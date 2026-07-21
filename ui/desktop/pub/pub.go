// Package pub holds process-management code shared by the Wails v2
// (ui/desktop/wails2) and Wails v3 (ui/desktop/wails) desktop shells:
// spawning the codg backend in its own process group, signalling the whole
// tree on shutdown, and finding/stopping codg processes by listening port.
package pub

import "runtime"

// CodgBinaryName is the platform-specific filename of the codg executable.
func CodgBinaryName() string {
	if runtime.GOOS == "windows" {
		return "codg.exe"
	}
	return "codg"
}
