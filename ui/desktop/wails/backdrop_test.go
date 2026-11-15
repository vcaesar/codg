package main

import (
	"bytes"
	"testing"
)

func TestBackdropDark(t *testing.T) {
	t.Parallel()

	tests := []struct {
		message  string
		dark, ok bool
	}{
		{"codg:theme:dark", true, true},
		{"codg:theme:light", false, true},
		{"codg:theme:", false, false},
		{"codg:theme:system", false, false},
		{"wails:runtime:ready", false, false},
		{"", false, false},
	}
	for _, tt := range tests {
		dark, ok := backdropDark(tt.message)
		if dark != tt.dark || ok != tt.ok {
			t.Errorf("backdropDark(%q) = %v, %v, want %v, %v", tt.message, dark, ok, tt.dark, tt.ok)
		}
	}
}

// The script and backdropDark must agree on the message format, and the
// script must watch the class the web UI's theme provider toggles on <html>.
func TestThemeReportScript(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		`"codg:theme:" + theme`,
		`"dark"`,
		`"light"`,
		`attributeFilter: ["class"]`,
		`window.webkit.messageHandlers.external`,
	} {
		if !bytes.Contains(themeReportScript, []byte(want)) {
			t.Errorf("themeReportScript missing %q", want)
		}
	}
}

// A nil window handle must be ignored rather than crash in AppKit.
func TestSetBackdropAppearanceNilWindow(t *testing.T) {
	t.Parallel()

	if setBackdropAppearance(nil, true) {
		t.Fatal("setBackdropAppearance(nil) = true, want false")
	}
}
