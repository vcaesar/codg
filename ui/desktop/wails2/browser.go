// BrowserService provides browser and file preview operations for Wails v2.
// Wails v2 supports one native window, so previews open in the system browser
// or in a temporary local HTML file instead of creating extra WebViews.
package main

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/vcaesar/codg/ui/desktop/pub"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type browserPreview struct {
	url      string
	tempPath string
}

// BrowserService tracks externally opened previews so callers retain the v3
// service's list/close API. Navigation is unsupported because system browser
// tabs are not controllable by Wails v2.
type BrowserService struct {
	mu         sync.Mutex
	ctx        context.Context
	nextID     int
	previews   map[string]browserPreview
	openURL    func(context.Context, string)
	previewDir string
}

func NewBrowserService() *BrowserService {
	return &BrowserService{
		previews: make(map[string]browserPreview),
		openURL:  wailsruntime.BrowserOpenURL,
	}
}

func (s *BrowserService) startup(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx = ctx
}

func (s *BrowserService) OpenURL(rawURL string) (string, error) {
	previewURL, err := pub.NormalizePreviewURL(rawURL)
	if err != nil {
		return "", err
	}
	return s.open(previewURL, "")
}

func (s *BrowserService) PreviewHTML(title, htmlContent string) (string, error) {
	if strings.TrimSpace(htmlContent) == "" {
		return "", fmt.Errorf("empty html content")
	}
	if title == "" {
		title = "HTML Preview"
	}
	return s.openHTML(title, htmlContent)
}

func (s *BrowserService) PreviewFile(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve preview path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat preview file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("cannot preview directory %s", abs)
	}

	name := filepath.Base(abs)
	if pub.DirectPreviewExts[strings.ToLower(filepath.Ext(abs))] {
		return s.open(pub.FileURL(abs), "")
	}
	if info.Size() > pub.MaxInlinePreviewSize {
		return "", fmt.Errorf("file %s too large to preview inline (%d bytes > %d)",
			name, info.Size(), pub.MaxInlinePreviewSize)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("read preview file: %w", err)
	}
	return s.openHTML(name, pub.BuildTextPreviewHTML(name, string(data)))
}

func (s *BrowserService) openHTML(title, content string) (string, error) {
	content = ensureHTMLTitle(content, title)
	dir, err := s.resolvePreviewDir()
	if err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, "codg-preview-*.html")
	if err != nil {
		return "", fmt.Errorf("create preview file: %w", err)
	}
	path := file.Name()
	cleanupFile := true
	defer func() {
		if cleanupFile {
			if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
				slog.Warn("Remove failed preview file", "path", path, "err", removeErr)
			}
		}
	}()
	if _, err = file.WriteString(content); err != nil {
		closeErr := file.Close()
		if closeErr != nil {
			return "", fmt.Errorf("write preview file: %w; close: %v", err, closeErr)
		}
		return "", fmt.Errorf("write preview file: %w", err)
	}
	if err = file.Close(); err != nil {
		return "", fmt.Errorf("close preview file: %w", err)
	}
	name, err := s.open(pub.FileURL(path), path)
	if err != nil {
		return "", err
	}
	cleanupFile = false
	return name, nil
}

func (s *BrowserService) resolvePreviewDir() (string, error) {
	s.mu.Lock()
	dir := s.previewDir
	s.mu.Unlock()
	if dir == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("resolve preview cache: %w", err)
		}
		dir = filepath.Join(cacheDir, "codg", "previews")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create preview cache: %w", err)
	}
	return dir, nil
}

func ensureHTMLTitle(content, title string) string {
	if title == "" || strings.Contains(strings.ToLower(content), "<title") {
		return content
	}
	titleTag := "<title>" + html.EscapeString(title) + "</title>"
	lower := strings.ToLower(content)
	if index := strings.Index(lower, "<head>"); index >= 0 {
		at := index + len("<head>")
		return content[:at] + titleTag + content[at:]
	}
	return "<!doctype html><html><head>" + titleTag + "</head><body>" + content + "</body></html>"
}

func (s *BrowserService) open(previewURL, tempPath string) (string, error) {
	s.mu.Lock()
	ctx := s.ctx
	if ctx == nil {
		s.mu.Unlock()
		return "", fmt.Errorf("browser service not attached to an application")
	}
	s.nextID++
	name := fmt.Sprintf("preview-%d", s.nextID)
	s.previews[name] = browserPreview{url: previewURL, tempPath: tempPath}
	opener := s.openURL
	s.mu.Unlock()

	opener(ctx, previewURL)
	return name, nil
}

func (s *BrowserService) Navigate(name, rawURL string) error {
	if _, err := pub.NormalizePreviewURL(rawURL); err != nil {
		return err
	}
	if _, err := s.get(name); err != nil {
		return err
	}
	return fmt.Errorf("navigate preview %q: Wails v2 cannot control system browser tabs", name)
}

func (s *BrowserService) SetContent(name, htmlContent string) error {
	if strings.TrimSpace(htmlContent) == "" {
		return fmt.Errorf("empty html content")
	}
	if _, err := s.get(name); err != nil {
		return err
	}
	return fmt.Errorf("set preview %q content: Wails v2 cannot control system browser tabs", name)
}

func (s *BrowserService) ClosePreview(name string) error {
	s.mu.Lock()
	preview, ok := s.previews[name]
	if ok {
		delete(s.previews, name)
	}
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("no preview named %q", name)
	}
	if preview.tempPath != "" {
		if err := os.Remove(preview.tempPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove preview file: %w", err)
		}
	}
	return nil
}

func (s *BrowserService) CloseAll() {
	s.mu.Lock()
	previews := s.previews
	s.previews = make(map[string]browserPreview)
	s.mu.Unlock()
	for _, preview := range previews {
		if preview.tempPath != "" {
			if err := os.Remove(preview.tempPath); err != nil && !os.IsNotExist(err) {
				slog.Warn("Remove preview file", "path", preview.tempPath, "err", err)
			}
		}
	}
}

func (s *BrowserService) ListPreviews() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.previews))
	for name := range s.previews {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s *BrowserService) get(name string) (browserPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	preview, ok := s.previews[name]
	if !ok {
		return browserPreview{}, fmt.Errorf("no preview named %q", name)
	}
	return preview, nil
}
