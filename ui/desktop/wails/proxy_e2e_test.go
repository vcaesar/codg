package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vcaesar/codg/ui/desktop/pub"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// TestEmbeddedUIWithProxy_EndToEnd exercises the exact chain the WebView
// talks to: the real Wails BundledAssetFileServer over the real embedded
// frontend/dist, wrapped by backendProxyMiddleware. "/" must return the
// staged web UI's HTML, and an API path unknown to the embed must be
// answered by the backend.
func TestEmbeddedUIWithProxy_EndToEnd(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.WriteString(w, "backend:"+r.URL.Path+";origin="+r.Header.Get("Origin")); err != nil {
			t.Errorf("backend write: %v", err)
		}
	}))
	t.Cleanup(backend.Close)

	mw, stopRelay, err := backendProxyMiddleware(backend.URL)
	if err != nil {
		t.Fatalf("backendProxyMiddleware: %v", err)
	}
	t.Cleanup(stopRelay)
	srv := httptest.NewServer(mw(application.BundledAssetFileServer(pub.Assets)))
	t.Cleanup(srv.Close)

	// "/" -> embedded index.html (real build or placeholder, always HTML).
	resp, err := srv.Client().Get(srv.URL + "/?codg_desktop=darwin")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if cerr := resp.Body.Close(); cerr != nil {
		t.Fatalf("close body: %v", cerr)
	}
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(strings.ToLower(string(body)), "<html") {
		t.Fatalf("GET / did not return HTML: %.120s", body)
	}
	// The websocket-redirect shim must be injected: same-origin websockets
	// (terminal PTY) cannot traverse the Wails scheme handler, so the shim
	// reroutes them to the backend's real ws origin.
	if !strings.Contains(string(body), "window.WebSocket") {
		t.Fatalf("GET / missing websocket shim: %.200s", body)
	}
	// The theme report must be injected too: without it the native glass
	// backdrop keeps the system appearance under a dark (or light) web UI.
	if !strings.Contains(string(body), "codg:theme:") {
		t.Fatalf("GET / missing theme report script: %.200s", body)
	}
	// Websockets must target the origin-stripping relay, not the backend:
	// the backend upgrader 403s the wails://localhost page Origin.
	if backendHost := strings.TrimPrefix(backend.URL, "http://"); strings.Contains(string(body), `"ws://`+backendHost+`"`) {
		t.Fatalf("shim targets backend %q directly, want the relay", backendHost)
	}

	// API path -> proxied to the backend.
	resp, err = srv.Client().Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	body, err = io.ReadAll(resp.Body)
	if cerr := resp.Body.Close(); cerr != nil {
		t.Fatalf("close body: %v", cerr)
	}
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "backend:/health;origin=" {
		t.Fatalf("GET /health = %q, want backend response", body)
	}

	// Mutations carry the webview Origin, which the backend's CORS 403s;
	// the proxy must drop it.
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/session", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Origin", "wails://localhost")
	resp, err = srv.Client().Do(req)
	if err != nil {
		t.Fatalf("POST /v1/session: %v", err)
	}
	body, err = io.ReadAll(resp.Body)
	if cerr := resp.Body.Close(); cerr != nil {
		t.Fatalf("close body: %v", cerr)
	}
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "backend:/v1/session;origin=" {
		t.Fatalf("POST /v1/session = %q, want Origin stripped", body)
	}
}
