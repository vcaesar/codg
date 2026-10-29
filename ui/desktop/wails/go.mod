module github/gogo/codg/ui/desktop/wails

go 1.27.3

// The desktop shell is a thin Wails v3 wrapper that launches the standalone
// `codg` binary as a child process (see backend.go). It deliberately does
// NOT import the codg agent stack, so this module needs only Wails + its
// transitive dependencies.

require (
	github.com/vcaesar/codg/ui/desktop/pub v0.0.0
	github.com/wailsapp/wails/v3 v3.0.0
)

// The shared desktop process-management code lives in the sibling pub
// package; use the in-repo copy.
replace (
	github.com/vcaesar/codg/plugin => ../../../plugin
	github.com/vcaesar/codg/ui/desktop/pub => ../pub
)

require (
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/vcaesar/codg/plugin v0.0.0-00010101000000-000000000000 // indirect
	golang.org/x/sys v0.48.0 // indirect
)
