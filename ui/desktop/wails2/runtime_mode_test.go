//go:build !bindings

package main

import "testing"

func TestRuntimeModeIsNotBindingGeneration(t *testing.T) {
	t.Parallel()
	if isBindingGeneration() {
		t.Fatal("normal builds must not report binding-generation mode")
	}
}
