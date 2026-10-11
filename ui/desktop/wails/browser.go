// browser.go — in-app preview browser for Codg Desktop.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/vcaesar/codg/ui/desktop/pub"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	previewW = 1024
	previewH = 768
)

// BrowserService is registered with Wails via application.NewService and
// exposes preview-browser windows to the React frontend through the
// auto-generated TypeScript bindings.
type BrowserService struct {
	mu      sync.Mutex
	app     *application.App
	nextID  int
	windows map[string]*application.WebviewWindow // preview name -> window
}

// NewBrowserService creates a new BrowserService instance.
func NewBrowserService() *BrowserService {
	return &BrowserService{windows: make(map[string]*application.WebviewWindow)}
}

// setApp stores the Wails app reference used to create windows. Called once
// during startup before app.Run(). Unexported to avoid binding generation.
func (s *BrowserService) setApp(app *application.App) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.app = app
}

// --- Methods exposed to the frontend (auto-generated bindings) ---

// OpenURL opens a preview window at the given http(s) or file URL and
// returns the preview name. Bare hosts ("example.com") default to https.
func (s *BrowserService) OpenURL(rawURL string) (string, error) {
	u, err := pub.NormalizePreviewURL(rawURL)
	if err != nil {
		return "", err
	}
	return s.open(application.WebviewWindowOptions{Title: u, URL: u})
}

// PreviewHTML opens a preview window rendering the given raw HTML string
// and returns the preview name.
func (s *BrowserService) PreviewHTML(title, htmlContent string) (string, error) {
	if strings.TrimSpace(htmlContent) == "" {
		return "", fmt.Errorf("empty html content")
	}
	if title == "" {
		title = "HTML Preview"
	}
	return s.open(application.WebviewWindowOptions{Title: title, HTML: htmlContent})
}

// PreviewFile opens a preview window for a local file and returns the
// preview name. HTML, images, PDF and audio/video load natively via a
// file:// URL; markdown, code and other text files render as escaped,
// monospaced text.
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
		return s.open(application.WebviewWindowOptions{Title: name, URL: pub.FileURL(abs)})
	}
	if info.Size() > pub.MaxInlinePreviewSize {
		return "", fmt.Errorf("file %s too large to preview inline (%d bytes > %d)",
			name, info.Size(), pub.MaxInlinePreviewSize)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("read preview file: %w", err)
	}
	return s.open(application.WebviewWindowOptions{
		Title: name,
		HTML:  pub.BuildTextPreviewHTML(name, string(data)),
	})
}

// Navigate points an existing preview window at a new URL.
func (s *BrowserService) Navigate(name, rawURL string) error {
	u, err := pub.NormalizePreviewURL(rawURL)
	if err != nil {
		return err
	}
	win, err := s.get(name)
	if err != nil {
		return err
	}
	win.SetURL(u)
	return nil
}

// SetContent replaces the HTML content of an existing preview window.
func (s *BrowserService) SetContent(name, htmlContent string) error {
	win, err := s.get(name)
	if err != nil {
		return err
	}
	win.SetHTML(htmlContent)
	return nil
}

// ClosePreview closes the named preview window.
func (s *BrowserService) ClosePreview(name string) error {
	win, err := s.get(name)
	if err != nil {
		return err
	}
	win.Close() // the WindowClosing handler removes it from the map
	return nil
}

// CloseAll closes every open preview window.
func (s *BrowserService) CloseAll() {
	s.mu.Lock()
	wins := make([]*application.WebviewWindow, 0, len(s.windows))
	for _, w := range s.windows {
		wins = append(wins, w)
	}
	s.mu.Unlock()
	for _, w := range wins {
		w.Close()
	}
}

// ListPreviews returns the names of the open preview windows, sorted.
func (s *BrowserService) ListPreviews() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.windows))
	for name := range s.windows {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// --- Internals ---

// open creates a preview WebviewWindow with sensible defaults, tracks it by
// a generated "preview-N" name and returns that name.
func (s *BrowserService) open(opts application.WebviewWindowOptions) (string, error) {
	s.mu.Lock()
	app := s.app
	s.nextID++
	name := fmt.Sprintf("preview-%d", s.nextID)
	s.mu.Unlock()
	if app == nil {
		return "", fmt.Errorf("browser service not attached to an application")
	}

	opts.Name = name
	if opts.Width == 0 {
		opts.Width = previewW
	}
	if opts.Height == 0 {
		opts.Height = previewH
	}
	if opts.Title == "" {
		opts.Title = "Preview"
	}

	win := app.Window.NewWithOptions(opts)
	// Bookkeeping only — the default close (destroy) still proceeds.
	win.OnWindowEvent(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		s.remove(name)
	})

	s.mu.Lock()
	s.windows[name] = win
	s.mu.Unlock()
	return name, nil
}

// get returns the tracked preview window for name.
func (s *BrowserService) get(name string) (*application.WebviewWindow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	win, ok := s.windows[name]
	if !ok {
		return nil, fmt.Errorf("no preview window named %q", name)
	}
	return win, nil
}

// remove drops a closed preview window from the registry.
func (s *BrowserService) remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.windows, name)
}
