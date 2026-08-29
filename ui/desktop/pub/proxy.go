// Same-origin backend proxy shared by the desktop shells.
//
// The WebView loads the PREBUILT web UI (staged into frontend/dist and
// embedded) from the Wails asset server, and the React SPA resolves its API
// base from window.location.origin. This middleware makes that origin fully
// functional: every request the asset server cannot satisfy — API calls,
// SSE streams — is forwarded to the codg backend (the spawned `codg web`
// child or an already-running server).
package pub

import (
	"bytes"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime"
	"strconv"
	"strings"
)

// Middleware is the http middleware shape both Wails v2's
// assetserver.Middleware and Wails v3's application.Middleware share;
// shells convert via a plain type conversion.
type Middleware func(http.Handler) http.Handler

// DesktopMarkerMiddleware injects only the desktop marker (no backend, no
// websocket redirect) into served HTML documents. Used when the shell runs
// without a backend.
func DesktopMarkerMiddleware() Middleware {
	return documentMiddleware(nil, nil)
}

// BackendProxyMiddleware returns an asset-server middleware that serves
// static assets from next (the embedded frontend) and forwards everything
// else to the codg backend at backendURL.
//
// Routing rules:
//   - non-GET/HEAD requests -> backend (static asset servers only serve GET)
//   - Upgrade (websocket) or text/event-stream (SSE) requests -> backend
//   - HTML document requests -> served (embed first, backend fallback)
//     with the websocket-redirect shim injected into <head>
//   - GET/HEAD the asset server answers 404/405 to -> replayed to backend
func BackendProxyMiddleware(backendURL string) (Middleware, error) {
	target, err := parseBackendTarget(backendURL)
	if err != nil {
		return nil, err
	}

	proxy := newBackendProxy(target)
	return documentMiddleware(proxy, target), nil
}

// parseBackendTarget validates backendURL as a proxyable http(s) target.
func parseBackendTarget(backendURL string) (*url.URL, error) {
	target, err := url.Parse(backendURL)
	if err != nil {
		return nil, fmt.Errorf("parse backend URL %q: %w", backendURL, err)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("parse backend URL %q: unsupported scheme %q", backendURL, target.Scheme)
	}
	if target.Host == "" {
		return nil, fmt.Errorf("parse backend URL %q: missing host", backendURL)
	}
	return target, nil
}

func documentMiddleware(proxy *httputil.ReverseProxy, backend *url.URL) Middleware {
	shim := WSRedirectShim(backend)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if proxy != nil && wantsBackend(r) {
				proxy.ServeHTTP(w, r)
				return
			}
			if isDocumentRequest(r) {
				// HTML documents are buffered so the websocket shim can
				// be injected — same-origin websockets cannot traverse
				// the Wails scheme handler (see package doc).
				buf := newHTMLBuffer()
				next.ServeHTTP(buf, r)
				if proxy != nil && (buf.status == http.StatusNotFound ||
					buf.status == http.StatusMethodNotAllowed) {
					// Not an embedded asset (e.g. SPA deep link when the
					// embed has no fallback) — let the backend answer,
					// still injecting into whatever HTML it returns.
					buf = newHTMLBuffer()
					proxy.ServeHTTP(buf, r)
				}
				buf.writeTo(w, shim)
				return
			}

			if proxy == nil {
				next.ServeHTTP(w, r)
				return
			}
			// Try the asset server first; when it 404s (not an asset),
			// suppress that response and replay against the backend with
			// the ORIGINAL writer so streaming/hijacking still work.
			fallback := &notFoundFallback{ResponseWriter: w}
			next.ServeHTTP(fallback, r)
			if fallback.fellBack {
				clearHeader(w.Header()) // drop headers the 404 path set
				proxy.ServeHTTP(w, r)
			}
		})
	}
}

func newBackendProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Warn("Backend proxy error", "path", r.URL.Path, "err", err)
		http.Error(w, "codg backend unreachable", http.StatusBadGateway)
	}
	return proxy
}

// WSRedirectShim returns an inline <script> that marks the page as running
// inside the desktop shell (window.__CODG__) and patches window.WebSocket
// so connections targeting the page's own origin are redirected to the codg
// backend's ws(s) origin. With a nil backend only the marker is emitted.
func WSRedirectShim(backend *url.URL) []byte {
	wsOrigin := ""
	if backend != nil {
		wsOrigin = "ws://" + backend.Host
		if backend.Scheme == "https" {
			wsOrigin = "wss://" + backend.Host
		}
	}
	return fmt.Appendf(nil, `<script>/* codg desktop: mark the native shell and redirect Wails-scheme websockets. */
(function () {
  "use strict";
  window.__CODG__ = Object.assign({}, window.__CODG__ || {}, {
    platform: "desktop",
    os: %q
  });
  var Native = window.WebSocket;
  var wsOrigin = %q;
  if (!Native || !wsOrigin) { return; }
  function Redirected(url, protocols) {
    var target = url;
    try {
      var u = new URL(url, window.location.href);
      if ((u.protocol === "ws:" || u.protocol === "wss:") &&
          u.host === window.location.host) {
        target = wsOrigin + u.pathname + u.search;
      }
    } catch (e) { /* leave the URL untouched */ }
    return protocols === undefined ? new Native(target) : new Native(target, protocols);
  }
  Redirected.prototype = Native.prototype;
  Redirected.CONNECTING = Native.CONNECTING;
  Redirected.OPEN = Native.OPEN;
  Redirected.CLOSING = Native.CLOSING;
  Redirected.CLOSED = Native.CLOSED;
  window.WebSocket = Redirected;
})();
</script>`, runtime.GOOS, wsOrigin)
}

// isDocumentRequest reports whether r is a navigation-style request for an
// HTML document (the only responses that receive the websocket shim).
func isDocumentRequest(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		return true
	}
	path := r.URL.Path
	return path == "/" || strings.HasSuffix(path, "/") || strings.HasSuffix(path, ".html")
}

// htmlBuffer captures a full response in memory so HTML documents can be
// rewritten before reaching the webview. Documents are small; buffering
// them is cheap and only document requests use this path.
type htmlBuffer struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newHTMLBuffer() *htmlBuffer          { return &htmlBuffer{header: make(http.Header)} }
func (b *htmlBuffer) Header() http.Header { return b.header }
func (b *htmlBuffer) WriteHeader(code int) {
	if b.status == 0 {
		b.status = code
	}
}
func (b *htmlBuffer) Write(body []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.body.Write(body)
}

// Flush is a no-op: the whole document is buffered by design.
func (b *htmlBuffer) Flush() {}

// writeTo replays the buffered response onto w, injecting shim into the
// <head> of successful, uncompressed HTML bodies. Anything else (errors,
// non-HTML, encoded bodies) passes through byte-for-byte.
func (b *htmlBuffer) writeTo(w http.ResponseWriter, shim []byte) {
	body := b.body.Bytes()
	contentType := b.header.Get("Content-Type")
	if b.status == http.StatusOK && b.header.Get("Content-Encoding") == "" &&
		(contentType == "" || strings.Contains(contentType, "text/html")) {
		body = injectHeadScript(body, shim)
	}
	maps.Copy(w.Header(), b.header)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	status := b.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		slog.Warn("Write injected document", "err", err)
	}
}

// injectHeadScript inserts script right after <head> (or before </head>,
// or prepended when no head tag exists) so it runs before the SPA's module
// scripts create any websocket.
func injectHeadScript(document, script []byte) []byte {
	lower := bytes.ToLower(document)
	at := 0
	if index := bytes.Index(lower, []byte("<head>")); index >= 0 {
		at = index + len("<head>")
	} else if index := bytes.Index(lower, []byte("</head>")); index >= 0 {
		at = index
	}
	result := make([]byte, 0, len(document)+len(script))
	result = append(result, document[:at]...)
	result = append(result, script...)
	result = append(result, document[at:]...)
	return result
}

// wantsBackend reports whether the request should bypass the asset server
// entirely and go straight to the codg backend.
func wantsBackend(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return true // mutations are API calls; static servers only GET
	}
	// Upgrade = websocket (e.g. terminal PTY); event-stream = SSE.
	return r.Header.Get("Upgrade") != "" || strings.Contains(r.Header.Get("Accept"), "text/event-stream")
}

// clearHeader removes every entry from h. Used before replaying a request
// to the backend so headers set by the asset server's 404 path (e.g.
// Content-Type: text/plain) don't leak into the proxied response.
func clearHeader(header http.Header) {
	for key := range header {
		delete(header, key)
	}
}

// notFoundFallback suppresses a 404/405 response from the asset server so
// the request can be replayed against the backend instead. All other
// statuses pass through untouched. Only used for GET/HEAD, so the request
// body never needs replaying.
type notFoundFallback struct {
	http.ResponseWriter
	wroteHeader bool
	fellBack    bool
}

func (w *notFoundFallback) WriteHeader(code int) {
	if w.fellBack || w.wroteHeader {
		return
	}
	if code == http.StatusNotFound || code == http.StatusMethodNotAllowed {
		w.fellBack = true // swallow; the backend gets a shot instead
		return
	}
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *notFoundFallback) Write(body []byte) (int, error) {
	if w.fellBack {
		return len(body), nil // discard the 404 body
	}
	w.wroteHeader = true // implicit 200 on first write
	return w.ResponseWriter.Write(body)
}

// Flush forwards to the underlying writer when supported, so streamed
// asset responses keep working through the wrapper.
func (w *notFoundFallback) Flush() {
	if w.fellBack {
		return
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
