// Backend wiring for the Wails v2 shell. All launcher/stopper logic lives
// in the shared pub package; only the shell-specific default command stays
// here.
package main

import (
	"context"

	"github.com/vcaesar/codg/ui/desktop/pub"
)

// defaultBackendCmd starts an owned backend on a free loopback port. This
// avoids attaching the privileged Wails runtime to an unrelated process on
// the conventional 4096 port.
const defaultBackendCmd = "web --port {port}"

// startBackendProcess launches the codg backend with this shell's default
// command spec.
func startBackendProcess(ctx context.Context) (string, func(), error) {
	return pub.StartBackendProcess(ctx, defaultBackendCmd)
}
