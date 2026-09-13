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
		if _, err := io.WriteString(w, "backend:"+r.URL.Path); err != nil {
			t.Errorf("backend write: %v", err)
		}
	}))
	t.Cleanup(backend.Close)

	mw, err := backendProxyMiddleware(backend.URL)
	if err != nil {
		t.Fatalf("backendProxyMiddleware: %v", err)
	}
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
	if string(body) != "backend:/health" {
		t.Fatalf("GET /health = %q, want backend response", body)
	}
}
