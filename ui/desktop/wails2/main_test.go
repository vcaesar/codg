package main

import (
	"testing"

	"github.com/vcaesar/codg/ui/desktop/pub"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func TestNewApplicationOptions(t *testing.T) {
	t.Setenv("CODG_DESKTOP_DEVTOOLS", "")
	oldMode := appMode
	appMode = "pro"
	t.Cleanup(func() { appMode = oldMode })

	desktop := NewDesktopService()
	browser := NewBrowserService()
	assetOptions := &assetserver.Options{Assets: pub.Assets}
	app := newApplicationOptions(desktop, browser, assetOptions, func() {})

	if app.Title != pub.AppName || app.Width != pub.WindowW || app.Height != pub.WindowH {
		t.Fatalf("window options = %q %dx%d", app.Title, app.Width, app.Height)
	}
	if app.MinWidth != pub.MinW || app.MinHeight != pub.MinH || app.HideWindowOnClose {
		t.Fatalf("window constraints = %dx%d, hide=%v", app.MinWidth, app.MinHeight, app.HideWindowOnClose)
	}
	if app.AssetServer != assetOptions || len(app.Bind) != 2 || app.Menu == nil {
		t.Fatal("asset server, service bindings, and menu must be configured")
	}
	if app.Mac == nil || app.Mac.TitleBar == nil ||
		!app.Mac.TitleBar.FullSizeContent || !app.Mac.TitleBar.UseToolbar {
		t.Fatal("hidden-inset macOS title bar must be configured")
	}
	if app.Debug.OpenInspectorOnStartup {
		t.Fatal("production builds must not open DevTools")
	}
}

func TestBuildMenu(t *testing.T) {
	t.Parallel()
	menu := buildMenu(NewDesktopService())
	if menu == nil || len(menu.Items) < 4 {
		t.Fatalf("menu items = %d, want application menus", len(menu.Items))
	}
}
