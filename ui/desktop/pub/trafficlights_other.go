//go:build !darwin

package pub

import "unsafe"

// CenterTrafficLights is a no-op on non-macOS platforms, which have no
// traffic-light buttons to reposition.
func CenterTrafficLights(_ unsafe.Pointer) {}

// InstallTrafficLightCentering is a no-op on non-macOS platforms.
func InstallTrafficLightCentering() {}
