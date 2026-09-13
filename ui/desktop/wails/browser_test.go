package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vcaesar/codg/ui/desktop/pub"
)

func TestBrowserService_NoApp(t *testing.T) {
	t.Parallel()

	s := NewBrowserService()

	// Without an attached app every open must fail cleanly.
	if _, err := s.OpenURL("https://example.com"); err == nil {
		t.Fatal("OpenURL without app: expected error")
	}
	if _, err := s.PreviewHTML("t", "<h1>x</h1>"); err == nil {
		t.Fatal("PreviewHTML without app: expected error")
	}
	// Empty HTML is rejected before window creation.
	if _, err := s.PreviewHTML("t", "  "); err == nil {
		t.Fatal("PreviewHTML with empty content: expected error")
	}
	if got := s.ListPreviews(); len(got) != 0 {
		t.Fatalf("ListPreviews = %v, want empty", got)
	}
}

func TestBrowserService_UnknownWindow(t *testing.T) {
	t.Parallel()

	s := NewBrowserService()
	if err := s.Navigate("preview-1", "https://example.com"); err == nil {
		t.Fatal("Navigate unknown window: expected error")
	}
	if err := s.SetContent("preview-1", "<p>x</p>"); err == nil {
		t.Fatal("SetContent unknown window: expected error")
	}
	if err := s.ClosePreview("preview-1"); err == nil {
		t.Fatal("ClosePreview unknown window: expected error")
	}
	// Navigate validates the URL before the window lookup.
	if err := s.Navigate("preview-1", "javascript://x"); err == nil ||
		!strings.Contains(err.Error(), "scheme") {
		t.Fatalf("Navigate bad scheme: got %v, want scheme error", err)
	}
}

func TestPreviewFile_Errors(t *testing.T) {
	t.Parallel()

	s := NewBrowserService()

	// Missing file.
	if _, err := s.PreviewFile(filepath.Join(t.TempDir(), "nope.html")); err == nil {
		t.Fatal("PreviewFile missing file: expected error")
	}
	// Directory.
	if _, err := s.PreviewFile(t.TempDir()); err == nil {
		t.Fatal("PreviewFile directory: expected error")
	}
	// Oversized inline text file.
	big := filepath.Join(t.TempDir(), "big.txt")
	if err := os.WriteFile(big, make([]byte, pub.MaxInlinePreviewSize+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreviewFile(big); err == nil ||
		!strings.Contains(err.Error(), "too large") {
		t.Fatalf("PreviewFile oversized: got %v, want too-large error", err)
	}
	// A small text file passes validation and only fails at window creation
	// (no app attached in tests) — proving the read/wrap path is reached.
	small := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(small, []byte("# hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreviewFile(small); err == nil ||
		!strings.Contains(err.Error(), "not attached") {
		t.Fatalf("PreviewFile small text: got %v, want not-attached error", err)
	}
	// Same for a directly-rendered file type (file:// URL path).
	page := filepath.Join(t.TempDir(), "index.html")
	if err := os.WriteFile(page, []byte("<h1>x</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreviewFile(page); err == nil ||
		!strings.Contains(err.Error(), "not attached") {
		t.Fatalf("PreviewFile html: got %v, want not-attached error", err)
	}
}
