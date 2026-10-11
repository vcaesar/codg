// Codg Desktop — Wails 3 desktop application entry point.
package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/vcaesar/codg/ui/desktop/pub"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// appMode is set via -ldflags at build time. When "pro"
var appMode string // -ldflags "-X main.appMode=pro"

// isProMode reports whether the binary was built in production mode.
func isProMode() bool {
	return appMode == "pro"
}

func main() {
	desktopSvc := NewDesktopService()
	browserSvc := NewBrowserService()

	// In production mode suppress debug/info logs.
	logLevel := slog.LevelDebug
	if isProMode() {
		logLevel = slog.LevelWarn
	}

	// --- Backend resolution (BEFORE the app: the asset-server middleware
	// needs the backend URL) ---
	ctx, cancelAPI := context.WithCancel(context.Background())
	var apiShutdown func()
	serverURL := os.Getenv("CODG_DESKTOP_SERVER_URL")
	if serverURL == "" {
		if serverURL = pub.ProbeBackend(pub.DefaultBackendURL); serverURL != "" {
			slog.Info("Using running codg backend", "url", serverURL)
			apiShutdown = pub.ExternalBackendShutdown(serverURL)
		}
	}
	if serverURL == "" {
		var err error
		serverURL, apiShutdown, err = startBackendProcess(ctx)
		if err != nil {
			slog.Warn("codg backend failed to start; "+
				"falling back to static assets", "err", err)
			serverURL = ""
		}
	}

	// --- Window URL + asset middleware ---
	windowURL := "/?codg_desktop=" + runtime.GOOS
	var assetMiddleware application.Middleware
	switch {
	case serverURL != "" && pub.EmbedHasRealUI():
		mw, err := backendProxyMiddleware(serverURL)
		if err != nil {
			slog.Warn("backend proxy unavailable; loading backend URL directly",
				"err", err)
			windowURL = strings.TrimRight(serverURL, "/") + windowURL
		} else {
			assetMiddleware = mw
			slog.Info("Serving embedded web UI; proxying API to backend",
				"backend", serverURL)
		}
	case serverURL != "":
		// Placeholder embed — the backend's own (embedded) UI is all we have.
		windowURL = strings.TrimRight(serverURL, "/") + windowURL
	}

	app := application.New(application.Options{
		Name:        pub.AppName,
		Description: "AI-powered coding assistant",
		Icon:        appIcon,
		LogLevel:    logLevel,
		Assets: application.AssetOptions{
			Handler:    application.BundledAssetFileServer(pub.Assets),
			Middleware: assetMiddleware,
		},
		Services: []application.Service{
			application.NewService(desktopSvc),
			application.NewService(browserSvc),
		},
		Mac: application.MacOptions{
			// Keep the app running when the last window closes (system
			// tray remains active). Users can re-open via tray icon.
			ApplicationShouldTerminateAfterLastWindowClosed: false,
			ActivationPolicy: application.ActivationPolicyRegular,
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
	})

	// Store app reference so the desktop service can emit events, etc.
	desktopSvc.setApp(app)
	browserSvc.setApp(app)

	// Stop the codg backend exactly once — on every quit path: the Wails
	// shutdown hook
	var stopOnce sync.Once
	stopAPI := func() {
		stopOnce.Do(func() {
			cancelAPI()
			if apiShutdown == nil {
				return
			}
			done := make(chan struct{})
			go func() {
				apiShutdown()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(8 * time.Second):
				slog.Warn("codg backend shutdown timed out; exiting anyway")
			}
		})
	}
	// On macOS, quitting (Cmd+Q / tray Quit / app.Quit) goes through
	// [NSApp terminate:]
	app.OnShutdown(stopAPI)
	if apiShutdown != nil {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			stopAPI()
			os.Exit(0)
		}()
	}

	// --- Main Window ---

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   "main",
		Title:  pub.AppName,
		Width:  pub.WindowW,
		Height: pub.WindowH,
		Mac: application.MacWindow{
			// Match the drag region to the 40px (h-10) React toolbar.
			InvisibleTitleBarHeight: 40,
			Backdrop:                application.MacBackdropTranslucent,
			// Unified toolbar style produces a compact (~40px) title bar
			TitleBar: application.MacTitleBarHiddenInsetUnified,
		},
		BackgroundColour: application.NewRGB(26, 26, 46), // #1a1a2e — matches TUI dark theme.
		URL:              windowURL,
		MinWidth:         pub.MinW,
		MinHeight:        pub.MinH,
	})

	// When the main window is closed, hide instead of destroying so the
	// system tray can restore it later.
	mainWindow.OnWindowEvent(events.Common.WindowClosing, func(e *application.WindowEvent) {
		mainWindow.Hide()
		e.Cancel()
	})

	// Vertically centre the native macOS traffic-light buttons with the React
	// toolbar (the "Toggle Sidebar" button).
	recenter := func(_ *application.WindowEvent) {
		application.InvokeAsync(func() {
			centerTrafficLights(mainWindow.NativeWindow())
		})
	}
	mainWindow.OnWindowEvent(events.Mac.WebViewDidFinishNavigation, recenter)
	mainWindow.OnWindowEvent(events.Mac.WindowDidBecomeKey, recenter)
	mainWindow.OnWindowEvent(events.Common.WindowDidResize, recenter)

	desktopSvc.setMainWindow(mainWindow)

	// --- System Tray ---

	setupSystemTray(app, mainWindow)

	// --- Native Menu Bar (per-window on Windows/Linux) ---

	setupMenuBar(app, mainWindow)

	// --- Run ---
	slog.Info("Starting Codg Desktop", "version", pub.AppVersion)
	runErr := app.Run()
	// Stop the child backend (if we started one) before exiting.
	stopAPI()
	if runErr != nil {
		log.Fatal(runErr)
	}
}

// setupSystemTray creates a system tray icon with a context menu.
func setupSystemTray(app *application.App, mainWindow *application.WebviewWindow) {
	tray := app.SystemTray.New()

	// macOS template icons adapt to light/dark menu bar automatically.
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(trayTemplateIcon)
	} else {
		tray.SetIcon(appIcon)
	}

	tray.SetTooltip("Codg — AI Coding Assistant")

	// Tray context menu.
	menu := app.NewMenu()
	menu.Add("Show Codg").OnClick(func(_ *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
	})
	menu.AddSeparator()
	menu.Add("New Session").OnClick(func(_ *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
		app.Event.Emit("desktop:new-session", nil)
	})
	menu.AddSeparator()
	menu.Add("Quit Codg").OnClick(func(_ *application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)

	// Single-click on tray icon toggles the main window.
	tray.OnClick(func() {
		mainWindow.Show()
		mainWindow.Focus()
	})
}

// setupMenuBar creates the native application menu bar and attaches it
// to the main window.
func setupMenuBar(app *application.App, mainWindow *application.WebviewWindow) {
	menu := app.NewMenu()

	// --- App Menu (macOS only, ignored on other platforms) ---
	appMenu := menu.AddSubmenu(pub.AppName)
	appMenu.Add("About Codg").OnClick(func(_ *application.Context) {
		app.Event.Emit("desktop:about", pub.AppVersion)
	})
	appMenu.AddSeparator()
	appMenu.Add("Preferences…").OnClick(func(_ *application.Context) {
		mainWindow.Show()
		app.Event.Emit("desktop:preferences", nil)
	})
	appMenu.AddSeparator()
	appMenu.Add("Hide Codg").OnClick(func(_ *application.Context) {
		mainWindow.Hide()
	})
	appMenu.AddSeparator()
	appMenu.Add("Quit Codg").OnClick(func(_ *application.Context) {
		app.Quit()
	})

	// --- File Menu ---
	fileMenu := menu.AddSubmenu("File")
	fileMenu.Add("New Session").OnClick(func(_ *application.Context) {
		app.Event.Emit("desktop:new-session", nil)
	})
	fileMenu.Add("Open Folder…").OnClick(func(_ *application.Context) {
		app.Event.Emit("desktop:open-folder", nil)
	})
	fileMenu.AddSeparator()
	fileMenu.Add("Close Window").OnClick(func(_ *application.Context) {
		mainWindow.Hide()
	})

	// --- Edit Menu ---
	editMenu := menu.AddSubmenu("Edit")
	editMenu.Add("Undo").OnClick(func(_ *application.Context) {
		app.Event.Emit("desktop:undo", nil)
	})
	editMenu.Add("Redo").OnClick(func(_ *application.Context) {
		app.Event.Emit("desktop:redo", nil)
	})
	editMenu.AddSeparator()
	editMenu.Add("Cut").OnClick(func(_ *application.Context) {
		app.Event.Emit("desktop:cut", nil)
	})
	editMenu.Add("Copy").OnClick(func(_ *application.Context) {
		app.Event.Emit("desktop:copy", nil)
	})
	editMenu.Add("Paste").OnClick(func(_ *application.Context) {
		app.Event.Emit("desktop:paste", nil)
	})
	editMenu.AddSeparator()
	editMenu.Add("Select All").OnClick(func(_ *application.Context) {
		app.Event.Emit("desktop:select-all", nil)
	})

	// --- View Menu ---
	viewMenu := menu.AddSubmenu("View")
	viewMenu.Add("Toggle Sidebar").OnClick(func(_ *application.Context) {
		app.Event.Emit("desktop:toggle-sidebar", nil)
	})
	viewMenu.Add("Toggle Terminal").OnClick(func(_ *application.Context) {
		app.Event.Emit("desktop:toggle-terminal", nil)
	})
	viewMenu.AddSeparator()
	viewMenu.Add("Zoom In").OnClick(func(_ *application.Context) {
		mainWindow.ZoomIn()
	})
	viewMenu.Add("Zoom Out").OnClick(func(_ *application.Context) {
		mainWindow.ZoomOut()
	})
	viewMenu.Add("Reset Zoom").OnClick(func(_ *application.Context) {
		mainWindow.ZoomReset()
	})

	// --- Help Menu ---
	helpMenu := menu.AddSubmenu("Help")
	helpMenu.Add("Documentation").OnClick(func(_ *application.Context) {
		app.Event.Emit("desktop:open-docs", nil)
	})
	helpMenu.Add("Report Issue").OnClick(func(_ *application.Context) {
		app.Event.Emit("desktop:report-issue", nil)
	})

	// Attach menu to the main window (on macOS this sets the app menu bar;
	// on Windows/Linux it becomes the window menu bar).
	mainWindow.SetMenu(menu)
}
