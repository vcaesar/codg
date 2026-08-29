//go:build darwin

package main

import (
	"testing"

	"github.com/vcaesar/codg/ui/desktop/pub"
)

// The shared centering constant (asserted in ui/desktop/pub) must place the
// buttons at half the React toolbar height (h-10 = 40px) so they align with
// the "Toggle Sidebar" button, matching the Wails v3 shell.
func TestTrafficLightCenterMatchesToolbar(t *testing.T) {
	t.Parallel()
	const toolbarHeight = 40.0
	if pub.TrafficLightCenterFromTop != toolbarHeight/2 {
		t.Fatalf("pub.TrafficLightCenterFromTop = %v, want %v", pub.TrafficLightCenterFromTop, toolbarHeight/2)
	}
}

// Installing the centering off the main thread must not panic; the AppKit
// work is dispatched onto the (idle, in tests) main queue.
func TestInstallTrafficLightCenteringNoPanic(t *testing.T) {
	t.Parallel()
	installTrafficLightCentering()
}
