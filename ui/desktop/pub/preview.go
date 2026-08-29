// Shared preview-browser helpers for the desktop shells: URL normalization,
// file:// URL building and the plain-text preview page. The BrowserService
// implementations themselves stay per-shell (Wails v3 opens extra
// WebviewWindows; Wails v2 opens the system browser).
package pub

import (
	"fmt"
	"html"
	"net/url"
	"path/filepath"
	"strings"
)

// MaxInlinePreviewSize caps files rendered inline as escaped text so a
// huge log file cannot balloon the WebView.
const MaxInlinePreviewSize = 2 << 20 // 2 MiB

// DirectPreviewExts lists file types the WebView renders natively — these
// are loaded via a file:// URL instead of being wrapped as escaped text.
var DirectPreviewExts = map[string]bool{
	".html": true, ".htm": true, ".xhtml": true, ".svg": true, ".pdf": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".bmp": true, ".ico": true, ".avif": true,
	".mp4": true, ".webm": true, ".mp3": true, ".wav": true, ".ogg": true,
}

// NormalizePreviewURL validates and normalizes a preview target URL.
// Bare hosts get https://; only http(s) and file schemes are allowed.
func NormalizePreviewURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty url")
	}
	// Schemeless ("example.com", "localhost:3000") — url.Parse would treat
	// "localhost:3000" as scheme "localhost", so detect by "://" instead.
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse preview url: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		if u.Host == "" {
			return "", fmt.Errorf("preview url has no host")
		}
	case "file":
		// Local file preview, e.g. produced by FileURL.
	default:
		return "", fmt.Errorf("unsupported preview url scheme %q", u.Scheme)
	}
	return u.String(), nil
}

// FileURL converts an absolute filesystem path to a file:// URL, handling
// Windows drive paths ("C:\x" -> "file:///C:/x") and escaping spaces.
func FileURL(abs string) string {
	p := filepath.ToSlash(abs)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	u := url.URL{Scheme: "file", Path: p}
	return u.String()
}

// BuildTextPreviewHTML wraps escaped text content in a minimal dark-themed
// page (matching the #1a1a2e app background) for previewing markdown, code
// and other plain-text files.
func BuildTextPreviewHTML(name, content string) string {
	return fmt.Sprintf(`<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>%[1]s</title>
<style>
  body { background: #1a1a2e; color: #e0e0e8; margin: 0;
         font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
  header { padding: 8px 16px; font-size: 12px; opacity: .7;
           border-bottom: 1px solid #2a2a4a; }
  pre { margin: 0; padding: 16px; font-size: 13px; line-height: 1.5;
        white-space: pre-wrap; word-break: break-word; }
</style>
</head>
<body>
<header>%[1]s</header>
<pre>%[2]s</pre>
</body>
</html>`, html.EscapeString(name), html.EscapeString(content))
}
