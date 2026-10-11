// Package main provides the DesktopService, a Wails v3 bound service that
// exposes native desktop capabilities to the React frontend via auto-generated
// TypeScript bindings. The frontend calls these methods through the Wails
// bridge (<1 ms overhead) instead of HTTP.
package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"sync"

	"github.com/vcaesar/codg/ui/desktop/pub"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// DesktopService is registered with Wails via application.NewService and
// automatically generates TypeScript bindings for the frontend.
type DesktopService struct {
	mu         sync.RWMutex
	app        *application.App
	mainWindow *application.WebviewWindow
}

// NewDesktopService creates a new DesktopService instance.
func NewDesktopService() *DesktopService {
	return &DesktopService{}
}

// setApp stores the Wails app reference for event emission. Called once
// during startup before app.Run(). Unexported to avoid binding generation.
func (s *DesktopService) setApp(app *application.App) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.app = app
}

// setMainWindow stores the main window reference. Called once during
// startup before app.Run(). Unexported to avoid binding generation.
func (s *DesktopService) setMainWindow(w *application.WebviewWindow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mainWindow = w
}

// --- Methods exposed to the frontend (auto-generated bindings) ---

// GetVersion returns the application version string.
func (s *DesktopService) GetVersion() string {
	return pub.AppVersion
}

// GetPlatform returns the runtime OS ("darwin", "windows", "linux").
func (s *DesktopService) GetPlatform() string {
	return runtime.GOOS
}

// OpenLink opens a URL in the user's default browser.
func (s *DesktopService) OpenLink(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default: // linux, freebsd, etc.
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}

// ShowWindow brings the main window to the front.
func (s *DesktopService) ShowWindow() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mainWindow != nil {
		s.mainWindow.Show()
		s.mainWindow.Focus()
	}
}

// HideWindow hides the main window (it remains in the system tray).
func (s *DesktopService) HideWindow() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mainWindow != nil {
		s.mainWindow.Hide()
	}
}

// MinimizeWindow minimises the main window.
func (s *DesktopService) MinimizeWindow() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mainWindow != nil {
		s.mainWindow.Minimise()
	}
}

// MaximizeWindow maximises the main window.
func (s *DesktopService) MaximizeWindow() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mainWindow != nil {
		s.mainWindow.Maximise()
	}
}

// ToggleFullscreen toggles the main window between fullscreen and normal.
func (s *DesktopService) ToggleFullscreen() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mainWindow != nil {
		s.mainWindow.ToggleFullscreen()
	}
}

// SetTitle sets the main window title.
func (s *DesktopService) SetTitle(title string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mainWindow != nil {
		s.mainWindow.SetTitle(title)
	}
}

// Notify sends a native desktop notification. On macOS this uses
// osascript; on Linux notify-send; on Windows PowerShell toast.
func (s *DesktopService) Notify(title string, body string) error {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		return exec.Command("osascript", "-e", script).Start()
	case "linux":
		return exec.Command("notify-send", title, body).Start()
	case "windows":
		ps := fmt.Sprintf(
			`[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime] | Out-Null; `+
				`$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent(0); `+
				`$text = $template.GetElementsByTagName('text'); `+
				`$text.Item(0).AppendChild($template.CreateTextNode('%s - %s')) | Out-Null; `+
				`$notifier = [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('Codg'); `+
				`$notifier.Show([Windows.UI.Notifications.ToastNotification]::new($template))`,
			title, body,
		)
		return exec.Command("powershell", "-Command", ps).Start()
	default:
		return fmt.Errorf("Notifications not supported on %s", runtime.GOOS)
	}
}

// Quit terminates the application gracefully.
func (s *DesktopService) Quit() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.app != nil {
		s.app.Quit()
	}
}

// GetWindowSize returns the current main window width and height.
func (s *DesktopService) GetWindowSize() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mainWindow == nil {
		return map[string]int{"width": 0, "height": 0}
	}
	w, h := s.mainWindow.Size()
	return map[string]int{"width": w, "height": h}
}

// SetWindowSize sets the main window dimensions.
func (s *DesktopService) SetWindowSize(width, height int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mainWindow != nil {
		s.mainWindow.SetSize(width, height)
	}
}

// CenterWindow centres the main window on its current screen.
func (s *DesktopService) CenterWindow() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mainWindow != nil {
		s.mainWindow.Center()
	}
}

// SetAlwaysOnTop pins or unpins the main window above other windows.
func (s *DesktopService) SetAlwaysOnTop(onTop bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mainWindow != nil {
		s.mainWindow.SetAlwaysOnTop(onTop)
	}
}

// OpenDevTools opens the WebView developer tools (useful for debugging).
func (s *DesktopService) OpenDevTools() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mainWindow != nil {
		s.mainWindow.OpenDevTools()
	}
}
