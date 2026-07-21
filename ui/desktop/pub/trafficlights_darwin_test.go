//go:build darwin

package pub

import "testing"

// The traffic-light centre must stay at half the shared React toolbar
// height (h-10 = 40px) so the native buttons align with the "Toggle
// Sidebar" button in both desktop shells.
func TestTrafficLightCenterMatchesToolbar(t *testing.T) {
	t.Parallel()
	const toolbarHeight = 40.0
	if TrafficLightCenterFromTop != toolbarHeight/2 {
		t.Fatalf("TrafficLightCenterFromTop = %v, want %v", TrafficLightCenterFromTop, toolbarHeight/2)
	}
}

// A nil window handle must be ignored rather than crash in AppKit.
func TestCenterTrafficLightsNilWindow(t *testing.T) {
	t.Parallel()
	CenterTrafficLights(nil)
}

// Installing the centering off the main thread must not panic; the AppKit
// work is dispatched onto the (idle, in tests) main queue.
func TestInstallTrafficLightCenteringNoPanic(t *testing.T) {
	t.Parallel()
	InstallTrafficLightCentering()
}
