// Package main provides native Wails v2 bindings for Codg Desktop.
package main

import (
	"context"
	"fmt"
	"net/url"
	"runtime"
	"strings"
	"sync"

	"github.com/vcaesar/codg/ui/desktop/pub"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// DesktopService exposes native window and desktop operations to the frontend.
type DesktopService struct {
	mu  sync.RWMutex
	ctx context.Context
}

func NewDesktopService() *DesktopService { return &DesktopService{} }

func (s *DesktopService) startup(ctx context.Context) {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
	if err := wailsruntime.InitializeNotifications(ctx); err != nil {
		// Notifications are optional; the app remains usable without them.
		return
	}
}

func (s *DesktopService) shutdown() {
	s.withContext(wailsruntime.CleanupNotifications)
}

func (s *DesktopService) withContext(fn func(context.Context)) bool {
	s.mu.RLock()
	ctx := s.ctx
	s.mu.RUnlock()
	if ctx == nil {
		return false
	}
	fn(ctx)
	return true
}

func (s *DesktopService) GetVersion() string { return pub.AppVersion }

func (s *DesktopService) GetPlatform() string { return runtime.GOOS }

// OpenLink opens a validated HTTP(S) URL in the system browser.
func (s *DesktopService) OpenLink(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("unsupported link URL")
	}
	if !s.withContext(func(ctx context.Context) { wailsruntime.BrowserOpenURL(ctx, parsed.String()) }) {
		return fmt.Errorf("desktop service not started")
	}
	return nil
}

func (s *DesktopService) ShowWindow() {
	s.withContext(func(ctx context.Context) {
		wailsruntime.WindowShow(ctx)
		wailsruntime.Show(ctx)
	})
}

func (s *DesktopService) HideWindow() {
	s.withContext(wailsruntime.WindowHide)
}

func (s *DesktopService) MinimizeWindow() {
	s.withContext(wailsruntime.WindowMinimise)
}

func (s *DesktopService) MaximizeWindow() {
	s.withContext(wailsruntime.WindowMaximise)
}

func (s *DesktopService) ToggleFullscreen() {
	s.withContext(func(ctx context.Context) {
		if wailsruntime.WindowIsFullscreen(ctx) {
			wailsruntime.WindowUnfullscreen(ctx)
			return
		}
		wailsruntime.WindowFullscreen(ctx)
	})
}

func (s *DesktopService) SetTitle(title string) {
	s.withContext(func(ctx context.Context) { wailsruntime.WindowSetTitle(ctx, title) })
}

// Notify uses Wails v2's native notification API. No text is interpolated
// into shell commands.
func (s *DesktopService) Notify(title, body string) error {
	var notifyErr error
	if !s.withContext(func(ctx context.Context) {
		notifyErr = wailsruntime.SendNotification(ctx, wailsruntime.NotificationOptions{
			Title: title,
			Body:  body,
		})
	}) {
		return fmt.Errorf("desktop service not started")
	}
	return notifyErr
}

func (s *DesktopService) Quit() {
	s.withContext(wailsruntime.Quit)
}

func (s *DesktopService) GetWindowSize() map[string]int {
	result := map[string]int{"width": 0, "height": 0}
	s.withContext(func(ctx context.Context) {
		width, height := wailsruntime.WindowGetSize(ctx)
		result["width"] = width
		result["height"] = height
	})
	return result
}

func (s *DesktopService) SetWindowSize(width, height int) {
	s.withContext(func(ctx context.Context) { wailsruntime.WindowSetSize(ctx, width, height) })
}

func (s *DesktopService) CenterWindow() {
	s.withContext(wailsruntime.WindowCenter)
}

func (s *DesktopService) SetAlwaysOnTop(onTop bool) {
	s.withContext(func(ctx context.Context) { wailsruntime.WindowSetAlwaysOnTop(ctx, onTop) })
}

// OpenDevTools asks the debug runtime to open the native inspector. Production
// builds omit inspector support, so this method intentionally becomes a no-op.
func (s *DesktopService) OpenDevTools() {
	s.withContext(func(ctx context.Context) {
		if isProMode() {
			return
		}
		wailsruntime.WindowExecJS(ctx, `if (window.WailsInvoke) window.WailsInvoke("wails:openInspector")`)
	})
}
