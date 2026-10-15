package main

import "testing"

func TestEmbeddedWindowURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos        string
		transparent bool
		want        string
	}{
		{"darwin", true, "/?codg_desktop=darwin"},
		{"darwin", false, "/"},
		{"windows", false, "/?codg_desktop=windows"},
		{"linux", false, "/?codg_desktop=linux"},
	}
	for _, tt := range tests {
		if got := embeddedWindowURL(tt.goos, tt.transparent); got != tt.want {
			t.Errorf("embeddedWindowURL(%q, %v) = %q, want %q", tt.goos, tt.transparent, got, tt.want)
		}
	}
}
