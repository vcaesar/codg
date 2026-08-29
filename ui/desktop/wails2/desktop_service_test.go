package main

import "testing"

func TestOpenLinkRejectsUnsafeSchemes(t *testing.T) {
	t.Parallel()
	service := NewDesktopService()
	for _, rawURL := range []string{
		"javascript:alert(1)",
		"file:///etc/passwd",
		"data:text/html,hello",
		"not a URL",
	} {
		if err := service.OpenLink(rawURL); err == nil {
			t.Fatalf("OpenLink(%q): expected rejection", rawURL)
		}
	}
}

func TestOpenLinkRequiresStartedService(t *testing.T) {
	t.Parallel()
	if err := NewDesktopService().OpenLink("https://example.com"); err == nil {
		t.Fatal("OpenLink before startup: expected error")
	}
}
