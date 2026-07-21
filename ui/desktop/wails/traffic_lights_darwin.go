//go:build darwin

package main

import (
	"unsafe"

	"github.com/vcaesar/codg/ui/desktop/pub"
)

// centerTrafficLights moves the native window controls so they vertically
// align with the in-app toolbar. The cgo implementation is shared with the
// Wails v2 shell in ui/desktop/pub (trafficlights_darwin.go); this shell
// re-applies it on window events from main.go. No-op when the handle is nil.
func centerTrafficLights(nsWindow unsafe.Pointer) {
	pub.CenterTrafficLights(nsWindow)
}
