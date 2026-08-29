// Loopback websocket relay for the desktop shells.
//
// The WSRedirectShim (proxy.go) makes the SPA's same-origin websockets dial
// the codg backend directly — but the webview stamps every handshake with
// the page's own Origin (wails://wails on Wails v2 macOS/Linux
package pub

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

// StartWSRelay starts a loopback listener that forwards websocket upgrade
// requests to the codg backend at target with the Origin header stripped.
// It returns the relay's http base URL (the shim converts it to ws://) and
// an idempotent stop func.
func StartWSRelay(target *url.URL) (*url.URL, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, fmt.Errorf("ws relay listen: %w", err)
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			// The webview's custom-scheme Origin (e.g. wails://wails) is
			// rejected by the backend's upgrader; origin-less is accepted.
			pr.Out.Header.Del("Origin")
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Warn("WS relay proxy error", "path", r.URL.Path, "err", err)
			http.Error(w, "codg backend unreachable", http.StatusBadGateway)
		},
	}

	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The relay exists only for websockets; everything else stays on
		// the asset-server middleware path.
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.NotFound(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	})}
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			slog.Warn("WS relay stopped", "err", serveErr)
		}
	}()

	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			if closeErr := server.Close(); closeErr != nil {
				slog.Warn("WS relay close", "err", closeErr)
			}
		})
	}
	relayURL := &url.URL{Scheme: "http", Host: listener.Addr().String()}
	return relayURL, stop, nil
}

// BackendProxyMiddlewareWithRelay is BackendProxyMiddleware plus the
// websocket relay: the injected shim targets the relay instead of the
// backend, so webview origins the backend would 403 never reach it. When
// the relay cannot start, it degrades to the direct-backend shim. The
// returned stop func shuts the relay down and is always safe to call.
func BackendProxyMiddlewareWithRelay(backendURL string) (Middleware, func(), error) {
	target, err := parseBackendTarget(backendURL)
	if err != nil {
		return nil, nil, err
	}

	proxy := newBackendProxy(target)
	relayURL, stop, err := StartWSRelay(target)
	if err != nil {
		slog.Warn("WS relay unavailable; websockets dial the backend directly", "err", err)
		return documentMiddleware(proxy, target), func() {}, nil
	}
	return documentMiddleware(proxy, relayURL), stop, nil
}
