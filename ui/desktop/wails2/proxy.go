// Proxy wiring for the Wails v2 shell. The middleware itself lives in the
// shared pub package; this file only adapts it to Wails v2's middleware type.
package main

import (
	"github.com/vcaesar/codg/ui/desktop/pub"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// desktopMarkerMiddleware injects the desktop marker without a backend.
func desktopMarkerMiddleware() assetserver.Middleware {
	return assetserver.Middleware(pub.DesktopMarkerMiddleware())
}

// backendProxyMiddleware adapts pub.BackendProxyMiddlewareWithRelay to the
// Wails v2 asset-server middleware type (same underlying func signature).
func backendProxyMiddleware(backendURL string) (assetserver.Middleware, func(), error) {
	mw, stop, err := pub.BackendProxyMiddlewareWithRelay(backendURL)
	if err != nil {
		return nil, nil, err
	}
	return assetserver.Middleware(mw), stop, nil
}
