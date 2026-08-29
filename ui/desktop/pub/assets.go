// assets.go — the shared web-UI embed and app metadata for both desktop
// shells (Wails v2 in ui/desktop/wails2, Wails v3 in ui/desktop/wails).
package pub

import (
	"bytes"
	"embed"
)

// AppVersion is a variable (not a const) so release builds can override it:
//
//	-ldflags "-X github.com/vcaesar/codg/ui/desktop/pub.AppVersion=1.2.3"
var AppVersion = "0.10.0"

// Shared application metadata and window geometry for both desktop shells.
const (
	AppName = "Codg"
	WindowW = 1400
	WindowH = 900
	MinW    = 800
	MinH    = 600
)

// PlaceholderMark tags the compile-time placeholder index.html written by
// stage-dist.mjs so EmbedHasRealUI can tell it apart from a genuine build.
const PlaceholderMark = "CODG_PLACEHOLDER"

// Assets embeds the pre-built React frontend staged into frontend/dist by
// the shells' stage-dist.mjs scripts.
//
//go:embed all:frontend/dist
var Assets embed.FS

// EmbedHasRealUI reports whether the embedded frontend/dist holds a genuine
// web build rather than the stage-dist.mjs placeholder.
func EmbedHasRealUI() bool {
	b, err := Assets.ReadFile("frontend/dist/index.html")
	return err == nil && !bytes.Contains(b, []byte(PlaceholderMark))
}
