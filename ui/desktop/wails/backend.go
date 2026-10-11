// Backend wiring for the Wails v3 shell. All launcher/stopper logic lives
// in the shared pub package; only the shell-specific default command stays
// here.
package main

import (
	"context"

	"github.com/vcaesar/codg/ui/desktop/pub"
)

// defaultBackendCmd is the codg subcommand the shell spawns.
const defaultBackendCmd = "web" // web 0 or "api"

// startBackendProcess launches the codg backend with this shell's default
// command spec.
func startBackendProcess(ctx context.Context) (string, func(), error) {
	return pub.StartBackendProcess(ctx, defaultBackendCmd)
}
