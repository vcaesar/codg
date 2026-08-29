//go:build !darwin

package main

// installTrafficLightCentering is a no-op on platforms without the macOS
// traffic-light window controls.
func installTrafficLightCentering() {}
