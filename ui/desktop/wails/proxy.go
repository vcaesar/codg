// Proxy wiring for the Wails v3 shell. The middleware itself lives in the
// shared pub package; this file only adapts it to Wails v3's middleware type.
package main

import (
	"github.com/vcaesar/codg/ui/desktop/pub"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// backendProxyMiddleware adapts pub.BackendProxyMiddlewareWithRelay to the
// Wails v3 asset-server middleware type (same underlying func signature).
// Websockets go through the loopback relay because the backend's upgrader
// rejects the wails://localhost / http://wails.localhost page Origin with 403.
func backendProxyMiddleware(backendURL string) (application.Middleware, func(), error) {
	mw, stop, err := pub.BackendProxyMiddlewareWithRelay(backendURL, themeReportScript)
	if err != nil {
		return nil, nil, err
	}
	return application.Middleware(mw), stop, nil
}
