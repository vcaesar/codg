//go:build !darwin

package main

import "unsafe"

// centerTrafficLights is a no-op on non-macOS platforms, which have no
// traffic-light buttons to reposition.
func centerTrafficLights(_ unsafe.Pointer) {}
