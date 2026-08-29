package pub

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizePreviewURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"https passthrough", "https://example.com/a?b=1", "https://example.com/a?b=1", false},
		{"http passthrough", "http://localhost:8080/", "http://localhost:8080/", false},
		{"bare host gets https", "example.com", "https://example.com", false},
		{"host with port gets https", "localhost:3000", "https://localhost:3000", false},
		{"file url allowed", "file:///tmp/index.html", "file:///tmp/index.html", false},
		{"whitespace trimmed", "  https://example.com  ", "https://example.com", false},
		{"empty rejected", "", "", true},
		{"whitespace only rejected", "   ", "", true},
		{"javascript rejected", "javascript://alert(1)", "", true},
		{"data rejected", "data://text/html,<b>x</b>", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizePreviewURL(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NormalizePreviewURL(%q) = %q, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizePreviewURL(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizePreviewURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFileURL(t *testing.T) {
	t.Parallel()

	// Unix-style absolute path.
	if got := FileURL("/tmp/index.html"); got != "file:///tmp/index.html" {
		t.Fatalf("got %q, want file:///tmp/index.html", got)
	}
	// Spaces must be escaped.
	if got := FileURL("/tmp/my page.html"); got != "file:///tmp/my%20page.html" {
		t.Fatalf("got %q, want file:///tmp/my%%20page.html", got)
	}
	// A file URL produced here must pass NormalizePreviewURL.
	if _, err := NormalizePreviewURL(FileURL("/tmp/index.html")); err != nil {
		t.Fatalf("NormalizePreviewURL(FileURL): %v", err)
	}
}

func TestBuildTextPreviewHTML_Escapes(t *testing.T) {
	t.Parallel()

	out := BuildTextPreviewHTML("evil<script>.md", "# hi\n<script>alert(1)</script>")
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Fatal("content was not HTML-escaped")
	}
	if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatal("escaped content missing from output")
	}
	if !strings.Contains(out, "evil&lt;script&gt;.md") {
		t.Fatal("title was not HTML-escaped")
	}
}

func TestDirectPreviewExts(t *testing.T) {
	t.Parallel()

	direct := []string{"a.html", "b.PNG", "c.pdf", "d.svg", "e.mp4"}
	for _, f := range direct {
		if !DirectPreviewExts[strings.ToLower(filepath.Ext(f))] {
			t.Fatalf("%s should preview directly", f)
		}
	}
	inline := []string{"a.md", "b.go", "c.txt", "d.json", "noext"}
	for _, f := range inline {
		if DirectPreviewExts[strings.ToLower(filepath.Ext(f))] {
			t.Fatalf("%s should preview as inline text", f)
		}
	}
}
