// Proxy wiring for the Wails v3 shell. The middleware itself lives in the
// shared pub package; this file only adapts it to Wails v3's middleware type.
package main

import (
	"github.com/vcaesar/codg/ui/desktop/pub"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// backendProxyMiddleware adapts pub.BackendProxyMiddleware to the Wails v3
// asset-server middleware type (same underlying func signature).
func backendProxyMiddleware(backendURL string) (application.Middleware, error) {
	mw, err := pub.BackendProxyMiddleware(backendURL)
	if err != nil {
		return nil, err
	}
	return application.Middleware(mw), nil
}
