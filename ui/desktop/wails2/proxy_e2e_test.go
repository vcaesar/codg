package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vcaesar/codg/ui/desktop/pub"
	wailsassetserver "github.com/wailsapp/wails/v2/pkg/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

type testRuntimeAssets struct{}

func (testRuntimeAssets) DesktopIPC() []byte   { return []byte("window.WailsInvoke=function(){}") }
func (testRuntimeAssets) WebsocketIPC() []byte { return nil }
func (testRuntimeAssets) RuntimeDesktopJS() []byte {
	return []byte("window.__WAILS_RUNTIME_TEST__=true")
}

// TestEmbeddedUIWithProxy_EndToEnd exercises the exact chain the WebView
// talks to: Wails v2's full asset server over the real embedded frontend/dist,
// wrapped by backendProxyMiddleware. "/" must return the
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
	assetHandler, err := wailsassetserver.NewAssetServer(
		"",
		assetserver.Options{Assets: pub.Assets, Middleware: mw},
		false,
		nil,
		testRuntimeAssets{},
	)
	if err != nil {
		t.Fatalf("NewAssetServer: %v", err)
	}
	srv := httptest.NewServer(assetHandler)
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
	// reroutes them — via the loopback relay, which strips the wails://
	// page Origin the backend's upgrader would reject with 403.
	if !strings.Contains(string(body), "window.WebSocket") {
		t.Fatalf("GET / missing websocket shim: %.200s", body)
	}
	match := regexp.MustCompile(`var wsOrigin = "(ws://[^"]+)"`).FindStringSubmatch(string(body))
	if match == nil {
		t.Fatalf("GET / missing ws shim origin: %.200s", body)
	}
	shimTarget, err := url.Parse(match[1])
	if err != nil {
		t.Fatalf("parse shim ws origin %q: %v", match[1], err)
	}
	if backendHost := strings.TrimPrefix(backend.URL, "http://"); shimTarget.Host == backendHost {
		t.Fatalf("shim targets backend %q directly, want the origin-stripping relay", shimTarget.Host)
	}
	// A handshake carrying the Wails v2 page origin must reach the backend
	// with the Origin header stripped.
	conn, err := net.DialTimeout("tcp", shimTarget.Host, 5*time.Second)
	if err != nil {
		t.Fatalf("dial relay %s: %v", shimTarget.Host, err)
	}
	t.Cleanup(func() {
		if cerr := conn.Close(); cerr != nil {
			t.Logf("close relay conn: %v", cerr)
		}
	})
	if _, err := io.WriteString(conn, "GET /pty?id=e2e HTTP/1.1\r\nHost: "+shimTarget.Host+"\r\n"+
		"Connection: Upgrade\r\nUpgrade: websocket\r\nOrigin: wails://wails\r\n"+
		"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: x3JJHMbDL1EzLkh9GBhXDw==\r\n\r\n"); err != nil {
		t.Fatalf("write relay handshake: %v", err)
	}
	relayBuf := make([]byte, 4096)
	n, err := conn.Read(relayBuf)
	if err != nil {
		t.Fatalf("read relay response: %v", err)
	}
	relayResp := string(relayBuf[:n])
	if !strings.Contains(relayResp, "origin=") {
		t.Fatalf("relay did not reach the backend: %q", relayResp)
	}
	if strings.Contains(relayResp, "origin=wails://wails") {
		t.Fatalf("backend still saw the wails:// Origin: %q", relayResp)
	}
	if !strings.Contains(string(body), `src="/wails/runtime.js"`) {
		t.Fatalf("GET / missing Wails v2 runtime injection: %.200s", body)
	}
	resp, err = srv.Client().Get(srv.URL + "/wails/runtime.js")
	if err != nil {
		t.Fatalf("GET /wails/runtime.js: %v", err)
	}
	runtimeBody, readErr := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("close runtime body: %v", closeErr)
	}
	if readErr != nil || !strings.Contains(string(runtimeBody), "window.__WAILS_RUNTIME_TEST__") {
		t.Fatalf("runtime body = %q, err = %v", runtimeBody, readErr)
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
}
