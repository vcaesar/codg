// Backend wiring for the Wails v2 shell. All launcher/stopper logic lives
// in the shared pub package; only the shell-specific default command stays
// here.
package main

import (
	"context"

	"github.com/vcaesar/codg/ui/desktop/pub"
)

// defaultBackendCmd starts an owned backend on a free loopback port when no
// codg backend is already running on the conventional 4096 port (see
// pub.RunningBackend, which refuses non-codg listeners there).
// Override with CODG_DESKTOP_CMD, e.g.
//
//	CODG_DESKTOP_CMD="api"
//	CODG_DESKTOP_CMD="web 0"               # dev mode (vite HMR)
//	CODG_DESKTOP_CMD="web --port {port}"   # {port} -> a free port
const defaultBackendCmd = "web --port {port}"

// startBackendProcess launches the codg backend with this shell's default
// command spec.
func startBackendProcess(ctx context.Context) (string, func(), error) {
	return pub.StartBackendProcess(ctx, defaultBackendCmd)
}
