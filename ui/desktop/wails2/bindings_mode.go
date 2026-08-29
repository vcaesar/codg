//go:build bindings

package main

// isBindingGeneration prevents build-time binding extraction from launching
// the Codg backend. Wails runs the compiled application with the bindings tag.
func isBindingGeneration() bool { return true }
