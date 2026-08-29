//go:build darwin

package main

import "github.com/vcaesar/codg/ui/desktop/pub"

// installTrafficLightCentering vertically centres the native macOS window
// controls with the in-app toolbar and keeps them centred across window
// events.
func installTrafficLightCentering() {
	pub.InstallTrafficLightCentering()
}
