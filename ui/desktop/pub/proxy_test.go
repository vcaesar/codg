package pub

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// assetHandler mimics the Wails bundled asset server: serves a fixed set of
// paths, 404s everything else.
func assetHandler(files map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		if _, err := io.WriteString(w, body); err != nil {
			panic(err) // test handler; cannot happen with httptest recorder
		}
	})
}

// newBackend returns a test backend that records whether it was hit and
// answers 200 "backend:<path>".
func newBackend(t *testing.T, hit *bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hit = true
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, "backend:"+r.URL.Path); err != nil {
			t.Errorf("backend write: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestBackendProxyMiddleware_ServesAssetLocally(t *testing.T) {
	t.Parallel()

	var hit bool
	backend := newBackend(t, &hit)
	mw, err := BackendProxyMiddleware(backend.URL)
	if err != nil {
		t.Fatalf("BackendProxyMiddleware: %v", err)
	}
	h := mw(assetHandler(map[string]string{"/index.html": "<html>ui</html>"}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/index.html", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// HTML documents are served from the embed with the websocket shim
	// injected; the original markup must survive intact.
	if got := rec.Body.String(); !strings.Contains(got, "<html>ui</html>") {
		t.Fatalf("body = %q, want asset content", got)
	}
	if hit {
		t.Fatal("backend was hit for an embedded asset")
	}
}

func TestBackendProxyMiddleware_FallsBackToBackendOn404(t *testing.T) {
	t.Parallel()

	var hit bool
	backend := newBackend(t, &hit)
	mw, err := BackendProxyMiddleware(backend.URL)
	if err != nil {
		t.Fatalf("BackendProxyMiddleware: %v", err)
	}
	h := mw(assetHandler(nil)) // asset server knows no files

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if !hit {
		t.Fatal("backend was not hit for a non-asset path")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 from backend", rec.Code)
	}
	if got := rec.Body.String(); got != "backend:/health" {
		t.Fatalf("body = %q, want backend response", got)
	}
	// The asset server's 404 headers must not leak into the replay.
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want backend's application/json", ct)
	}
}

func TestBackendProxyMiddleware_NonGETGoesStraightToBackend(t *testing.T) {
	t.Parallel()

	var hit bool
	backend := newBackend(t, &hit)
	mw, err := BackendProxyMiddleware(backend.URL)
	if err != nil {
		t.Fatalf("BackendProxyMiddleware: %v", err)
	}
	// Even a path the asset server COULD serve must go to the backend
	// when the method is not GET/HEAD.
	h := mw(assetHandler(map[string]string{"/api/session": "not-this"}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/session",
		strings.NewReader(`{"a":1}`)))

	if !hit {
		t.Fatal("backend was not hit for POST")
	}
	if got := rec.Body.String(); got != "backend:/api/session" {
		t.Fatalf("body = %q, want backend response", got)
	}
}

func TestBackendProxyMiddleware_SSEGoesStraightToBackend(t *testing.T) {
	t.Parallel()

	var hit bool
	backend := newBackend(t, &hit)
	mw, err := BackendProxyMiddleware(backend.URL)
	if err != nil {
		t.Fatalf("BackendProxyMiddleware: %v", err)
	}
	h := mw(assetHandler(map[string]string{"/events": "not-this"}))

	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !hit {
		t.Fatal("backend was not hit for SSE request")
	}
}

func TestBackendProxyMiddleware_BackendDownReturns502(t *testing.T) {
	t.Parallel()

	// Nothing listens here; the proxy's ErrorHandler must answer 502.
	mw, err := BackendProxyMiddleware("http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("BackendProxyMiddleware: %v", err)
	}
	h := mw(assetHandler(nil))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}

func TestBackendProxyMiddleware_RejectsBadURL(t *testing.T) {
	t.Parallel()

	if _, err := BackendProxyMiddleware("file:///tmp/x"); err == nil {
		t.Fatal("expected error for non-http scheme")
	}
	if _, err := BackendProxyMiddleware("http://"); err == nil {
		t.Fatal("expected error for missing host")
	}
}

func TestBackendProxyMiddleware_InjectsWSShimIntoDocument(t *testing.T) {
	t.Parallel()

	var hit bool
	backend := newBackend(t, &hit)
	mw, err := BackendProxyMiddleware(backend.URL)
	if err != nil {
		t.Fatalf("BackendProxyMiddleware: %v", err)
	}
	const doc = "<html><head><script src=\"/app.js\"></script></head><body></body></html>"
	h := mw(assetHandler(map[string]string{"/": doc}))

	req := httptest.NewRequest(http.MethodGet, "/?codg_desktop=darwin", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "window.WebSocket") {
		t.Fatalf("document missing websocket shim: %.200s", body)
	}
	backendHost := strings.TrimPrefix(backend.URL, "http://")
	if !strings.Contains(body, `"ws://`+backendHost+`"`) {
		t.Fatalf("shim does not target backend ws origin %q: %.400s", backendHost, body)
	}
	// The shim must run BEFORE the SPA's own scripts.
	if strings.Index(body, "window.WebSocket") > strings.Index(body, "/app.js") {
		t.Fatal("shim injected after the SPA script tag")
	}
	if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(len(body)) {
		t.Fatalf("Content-Length = %q, want %d", got, len(body))
	}
	if hit {
		t.Fatal("backend was hit for an embedded document")
	}
}

func TestBackendProxyMiddleware_InjectsShimIntoBackendFallbackDocument(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := io.WriteString(w, "<html><head></head><body>backend ui</body></html>"); err != nil {
			t.Errorf("backend write: %v", err)
		}
	}))
	t.Cleanup(backend.Close)

	mw, err := BackendProxyMiddleware(backend.URL)
	if err != nil {
		t.Fatalf("BackendProxyMiddleware: %v", err)
	}
	h := mw(assetHandler(nil)) // embed knows no files -> 404 -> backend

	req := httptest.NewRequest(http.MethodGet, "/sessions/abc", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "window.WebSocket") {
		t.Fatalf("backend fallback document missing websocket shim: %.200s", body)
	}
	if !strings.Contains(body, "backend ui") {
		t.Fatalf("backend document body lost: %.200s", body)
	}
}

func TestBackendProxyMiddleware_DoesNotInjectIntoNonHTML(t *testing.T) {
	t.Parallel()

	var hit bool
	backend := newBackend(t, &hit)
	mw, err := BackendProxyMiddleware(backend.URL)
	if err != nil {
		t.Fatalf("BackendProxyMiddleware: %v", err)
	}
	h := mw(assetHandler(nil)) // 404 -> backend (answers application/json)

	req := httptest.NewRequest(http.MethodGet, "/api/", nil) // trailing "/" = document heuristic
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Body.String(); got != "backend:/api/" {
		t.Fatalf("non-HTML body altered: %q", got)
	}
}

func TestDesktopMarkerMiddlewareWithoutBackend(t *testing.T) {
	t.Parallel()
	handler := DesktopMarkerMiddleware()(assetHandler(map[string]string{
		"/": "<html><head></head><body>ui</body></html>",
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept", "text/html")
	handler.ServeHTTP(recorder, request)
	body := recorder.Body.String()
	if !strings.Contains(body, `platform: "desktop"`) {
		t.Fatalf("desktop marker missing without backend: %s", body)
	}
	if strings.Contains(body, `var wsOrigin = "ws`) {
		t.Fatalf("no-backend marker must not redirect websockets: %s", body)
	}
}

func TestWSRedirectShim(t *testing.T) {
	t.Parallel()

	httpURL, err := url.Parse("http://localhost:4096")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s := string(WSRedirectShim(httpURL)); !strings.Contains(s, `"ws://localhost:4096"`) {
		t.Fatalf("http backend must yield ws origin: %s", s)
	} else if !strings.Contains(s, `platform: "desktop"`) ||
		!strings.Contains(s, `os: "`+runtime.GOOS+`"`) {
		t.Fatalf("desktop platform marker missing: %s", s)
	}
	httpsURL, err := url.Parse("https://example.com:8443")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s := string(WSRedirectShim(httpsURL)); !strings.Contains(s, `"wss://example.com:8443"`) {
		t.Fatalf("https backend must yield wss origin: %s", s)
	}
}

func TestInjectHeadScript(t *testing.T) {
	t.Parallel()

	script := []byte("<script>x</script>")

	got := string(injectHeadScript([]byte("<html><HEAD><title>t</title></HEAD></html>"), script))
	if !strings.HasPrefix(got, "<html><HEAD><script>x</script>") {
		t.Fatalf("not injected after <head>: %s", got)
	}

	got = string(injectHeadScript([]byte("<html><title>t</title></head><body></body></html>"), script))
	if !strings.Contains(got, "<script>x</script></head>") {
		t.Fatalf("not injected before </head>: %s", got)
	}

	got = string(injectHeadScript([]byte("no head at all"), script))
	if !strings.HasPrefix(got, "<script>x</script>no head at all") {
		t.Fatalf("not prepended without head: %s", got)
	}
}

func TestIsDocumentRequest(t *testing.T) {
	t.Parallel()

	doc := httptest.NewRequest(http.MethodGet, "/", nil)
	if !isDocumentRequest(doc) {
		t.Fatal("GET / must be a document request")
	}

	nav := httptest.NewRequest(http.MethodGet, "/sessions/1", nil)
	nav.Header.Set("Accept", "text/html,application/xhtml+xml")
	if !isDocumentRequest(nav) {
		t.Fatal("navigation Accept must be a document request")
	}

	asset := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	if isDocumentRequest(asset) {
		t.Fatal("JS asset must not be a document request")
	}

	post := httptest.NewRequest(http.MethodPost, "/", nil)
	if isDocumentRequest(post) {
		t.Fatal("POST must not be a document request")
	}
}

func TestWantsBackend(t *testing.T) {
	t.Parallel()

	get := httptest.NewRequest(http.MethodGet, "/x", nil)
	if wantsBackend(get) {
		t.Fatal("plain GET should try the asset server first")
	}

	post := httptest.NewRequest(http.MethodPost, "/x", nil)
	if !wantsBackend(post) {
		t.Fatal("POST must go to the backend")
	}

	ws := httptest.NewRequest(http.MethodGet, "/x", nil)
	ws.Header.Set("Upgrade", "websocket")
	if !wantsBackend(ws) {
		t.Fatal("websocket upgrade must go to the backend")
	}
}
