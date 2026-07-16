module github/gogo/codg/ui/desktop/wails

go 1.26.4

// The desktop shell is a thin Wails v3 wrapper that launches the standalone
// `codg` binary as a child process (see backend.go). It deliberately does
// NOT import the codg agent stack, so this module needs only Wails + its
// transitive dependencies.

require github.com/wailsapp/wails/v3 v3.0.0-alpha2.111

require (
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/jchv/go-winloader v0.0.0-20250406163304-c1995be93bd1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/wailsapp/wails/webview2 v1.0.27 // indirect
	golang.org/x/sys v0.43.0 // indirect
)
