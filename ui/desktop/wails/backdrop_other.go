//go:build !darwin

package main

import "unsafe"

// setBackdropAppearance is a no-op on non-macOS platforms, which have no
// translucent window backdrop.
func setBackdropAppearance(_ unsafe.Pointer, _ bool) bool { return false }
