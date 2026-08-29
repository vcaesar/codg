// Codg Desktop is a thin Wails v2 shell around the shared Codg web UI.
// It embeds the prebuilt frontend and launches the standalone `codg web`
// backend as a child process, keeping the desktop binary independent from the
// agent stack.
package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/vcaesar/codg/ui/desktop/pub"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// appMode is set via -ldflags for release builds. The shared app metadata
// (pub.AppName, pub.AppVersion, window geometry) and the embedded web UI;
// override the version with -ldflags "-X github.com/vcaesar/codg/ui/desktop/pub.AppVersion=…".
var appMode string

func isProMode() bool { return appMode == "pro" }

func main() {
	desktopSvc := NewDesktopService()
	browserSvc := NewBrowserService()

	if isBindingGeneration() {
		// Wails executes this binary to discover bound methods. Avoid network
		// probes and subprocesses; wails.Run exits after writing the bindings.
		if err := wails.Run(newApplicationOptions(
			desktopSvc,
			browserSvc,
			&assetserver.Options{Assets: pub.Assets},
			func() {},
		)); err != nil {
			log.Fatal(err)
		}
		return
	}

	serverURL, stopAPI := resolveBackend()
	assetOptions := &assetserver.Options{
		Assets:     pub.Assets,
		Middleware: desktopMarkerMiddleware(),
	}
	if serverURL != "" {
		middleware, stopRelay, err := backendProxyMiddleware(serverURL)
		if err != nil {
			slog.Warn("Backend proxy unavailable", "err", err)
		} else {
			assetOptions.Middleware = middleware
			// Shut the websocket relay down on every quit path too.
			stopBackend := stopAPI
			stopAPI = func() {
				stopRelay()
				stopBackend()
			}
			if !pub.EmbedHasRealUI() {
				// Let the backend serve the document when no real UI was staged.
				assetOptions.Assets = nil
			}
		}
	}

	appOptions := newApplicationOptions(desktopSvc, browserSvc, assetOptions, stopAPI)
	installSignalHandler(func() {
		browserSvc.CloseAll()
		stopAPI()
	})

	slog.Info("Starting Codg Desktop", "version", pub.AppVersion, "wails", "v2")
	runErr := wails.Run(appOptions)
	stopAPI()
	if runErr != nil {
		log.Fatal(runErr)
	}
}

// resolveBackend attaches to an existing Codg backend or starts one as a child.
// The returned cleanup is idempotent and always safe to call.
func resolveBackend() (string, func()) {
	ctx, cancelAPI := context.WithCancel(context.Background())
	serverURL := os.Getenv("CODG_DESKTOP_SERVER_URL")
	var apiShutdown func()

	if serverURL != "" {
		validated, err := pub.ValidateBackendURL(serverURL)
		if err != nil {
			slog.Warn("Ignoring invalid desktop backend URL", "url", serverURL, "err", err)
			serverURL = ""
		} else if pub.ProbeBackend(validated) == "" {
			slog.Warn("Configured desktop backend is not healthy", "url", validated)
			serverURL = ""
		} else {
			serverURL = validated
			apiShutdown = pub.ExternalBackendShutdown(serverURL)
			slog.Info("Using explicitly configured codg backend", "url", serverURL)
		}
	}
	if serverURL == "" {
		var err error
		serverURL, apiShutdown, err = startBackendProcess(ctx)
		if err != nil {
			slog.Warn("Codg backend failed to start; serving static assets", "err", err)
			serverURL = ""
		}
	}

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
				slog.Warn("Codg backend shutdown timed out; exiting anyway")
			}
		})
	}
	return serverURL, stopAPI
}

func installSignalHandler(stopAPI func()) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		stopAPI()
		os.Exit(0)
	}()
}

func newApplicationOptions(
	desktopSvc *DesktopService,
	browserSvc *BrowserService,
	assetOptions *assetserver.Options,
	stopAPI func(),
) *options.App {
	logLevel := logger.DEBUG
	if isProMode() {
		logLevel = logger.WARNING
	}

	startup := func(ctx context.Context) {
		desktopSvc.startup(ctx)
		browserSvc.startup(ctx)
		// Vertically centre the macOS traffic-light buttons with the React
		// toolbar, matching the Wails v3 shell. No-op on other platforms.
		installTrafficLightCentering()
	}
	shutdown := func(context.Context) {
		desktopSvc.shutdown()
		browserSvc.CloseAll()
		stopAPI()
	}

	return &options.App{
		Title:              pub.AppName,
		Width:              pub.WindowW,
		Height:             pub.WindowH,
		MinWidth:           pub.MinW,
		MinHeight:          pub.MinH,
		HideWindowOnClose:  false,
		BackgroundColour:   options.NewRGB(26, 26, 46),
		AssetServer:        assetOptions,
		Menu:               buildMenu(desktopSvc),
		LogLevel:           logLevel,
		LogLevelProduction: logger.WARNING,
		OnStartup:          startup,
		OnShutdown:         shutdown,
		Bind:               []any{desktopSvc, browserSvc},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.atomai.codg.desktop",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				desktopSvc.ShowWindow()
			},
		},
		Mac: &mac.Options{
			// Wails v2 has no TitleBarHiddenInsetUnified() constructor; this
			// literal is TitleBarHiddenInset() plus UseToolbar, reproducing
			// the v3 shell's MacTitleBarHiddenInsetUnified (taller unified
			// toolbar). traffic_lights_darwin.go then centres the
			// close/min/max buttons with the 40px React toolbar.
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				FullSizeContent:            true,
				UseToolbar:                 true,
				HideToolbarSeparator:       true,
			},
		},
		Debug: options.Debug{
			OpenInspectorOnStartup: !isProMode() && os.Getenv("CODG_DESKTOP_DEVTOOLS") == "1",
		},
	}
}

// buildMenu keeps the v3 shell's frontend events while using Wails v2's
// application-menu API. Wails v2 is single-window and has no supported tray
// API, so closing the main window exits rather than hiding an unreachable app.
func buildMenu(app *DesktopService) *menu.Menu {
	appMenu := menu.NewMenu()
	if runtime.GOOS == "darwin" {
		appMenu.Append(menu.AppMenu())
	}

	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("New Session", keys.CmdOrCtrl("n"), emitMenuEvent(app, "desktop:new-session"))
	fileMenu.AddText("Open Folder…", keys.CmdOrCtrl("o"), emitMenuEvent(app, "desktop:open-folder"))
	fileMenu.AddSeparator()
	fileMenu.AddText("Quit Codg", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		app.Quit()
	})

	// Native roles preserve clipboard and undo shortcuts in focused controls.
	appMenu.Append(menu.EditMenu())

	viewMenu := appMenu.AddSubmenu("View")
	viewMenu.AddText("Toggle Sidebar", keys.CmdOrCtrl("b"), emitMenuEvent(app, "desktop:toggle-sidebar"))
	viewMenu.AddText("Toggle Terminal", keys.CmdOrCtrl("j"), emitMenuEvent(app, "desktop:toggle-terminal"))
	viewMenu.AddSeparator()
	viewMenu.AddText("Toggle Full Screen", keys.CmdOrCtrl("f"), func(_ *menu.CallbackData) {
		app.ToggleFullscreen()
	})

	helpMenu := appMenu.AddSubmenu("Help")
	helpMenu.AddText("Documentation", nil, emitMenuEvent(app, "desktop:open-docs"))
	helpMenu.AddText("Report Issue", nil, emitMenuEvent(app, "desktop:report-issue"))

	return appMenu
}

func emitMenuEvent(app *DesktopService, name string) menu.Callback {
	return func(_ *menu.CallbackData) {
		app.withContext(func(ctx context.Context) {
			wailsruntime.EventsEmit(ctx, name)
		})
	}
}
