package pub

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// wsEchoBackend is a test backend whose handler performs a real 101 upgrade:
// it records the Origin header it saw, hijacks the connection, and echoes a
// single line back prefixed with "echo:".
func wsEchoBackend(t *testing.T, sawOrigin *atomic.Value) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawOrigin.Store(r.Header.Get("Origin"))
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "not an upgrade", http.StatusBadRequest)
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("backend response writer is not a hijacker")
			return
		}
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer func() {
			if cerr := conn.Close(); cerr != nil {
				t.Logf("backend close: %v", cerr)
			}
		}()
		if _, err := io.WriteString(conn,
			"HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n"); err != nil {
			t.Errorf("write 101: %v", err)
			return
		}
		line, err := rw.Reader.ReadString('\n')
		if err != nil {
			t.Errorf("backend read: %v", err)
			return
		}
		if _, err := io.WriteString(conn, "echo:"+line); err != nil {
			t.Errorf("backend echo: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// dialUpgrade performs a raw websocket-style handshake against host with the
// hostile webview Origin and returns the connection and parsed response.
func dialUpgrade(t *testing.T, host, path string) (net.Conn, *bufio.Reader, *http.Response) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", host, 5*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", host, err)
	}
	t.Cleanup(func() {
		if cerr := conn.Close(); cerr != nil && !strings.Contains(cerr.Error(), "closed") {
			t.Logf("client close: %v", cerr)
		}
	})
	_, err = fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\n"+
		"Connection: Upgrade\r\nUpgrade: websocket\r\n"+
		"Origin: wails://wails\r\n"+
		"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: x3JJHMbDL1EzLkh9GBhXDw==\r\n\r\n",
		path, host)
	if err != nil {
		t.Fatalf("write handshake: %v", err)
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("read handshake response: %v", err)
	}
	return conn, reader, resp
}

func TestStartWSRelay_StripsOriginAndRelaysUpgrade(t *testing.T) {
	t.Parallel()

	var sawOrigin atomic.Value
	backend := wsEchoBackend(t, &sawOrigin)
	target, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("parse backend URL: %v", err)
	}

	relayURL, stop, err := StartWSRelay(target)
	if err != nil {
		t.Fatalf("StartWSRelay: %v", err)
	}
	t.Cleanup(stop)

	conn, reader, resp := dialUpgrade(t, relayURL.Host, "/pty?id=test")
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("handshake status = %d, want 101", resp.StatusCode)
	}
	if got := sawOrigin.Load(); got != "" {
		t.Fatalf("backend saw Origin %q, want stripped", got)
	}

	// The relayed connection must be bidirectional after the upgrade.
	if _, err := io.WriteString(conn, "ping\n"); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	echo, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if echo != "echo:ping\n" {
		t.Fatalf("echo = %q, want %q", echo, "echo:ping\n")
	}

	// stop is idempotent and severs the listener.
	stop()
	stop()
	if _, err := net.DialTimeout("tcp", relayURL.Host, time.Second); err == nil {
		t.Fatal("relay still accepting connections after stop")
	}
}

func TestStartWSRelay_RejectsNonUpgrade(t *testing.T) {
	t.Parallel()

	var sawOrigin atomic.Value
	backend := wsEchoBackend(t, &sawOrigin)
	target, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("parse backend URL: %v", err)
	}

	relayURL, stop, err := StartWSRelay(target)
	if err != nil {
		t.Fatalf("StartWSRelay: %v", err)
	}
	t.Cleanup(stop)

	resp, err := http.Get(relayURL.String() + "/api/anything")
	if err != nil {
		t.Fatalf("GET relay: %v", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			t.Errorf("close body: %v", cerr)
		}
	}()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("plain GET status = %d, want 404", resp.StatusCode)
	}
	if got := sawOrigin.Load(); got != nil {
		t.Fatal("non-upgrade request must not reach the backend")
	}
}

var wsOriginPattern = regexp.MustCompile(`var wsOrigin = "(ws://[^"]+)"`)

func TestBackendProxyMiddlewareWithRelay_ShimTargetsRelay(t *testing.T) {
	t.Parallel()

	var sawOrigin atomic.Value
	backend := wsEchoBackend(t, &sawOrigin)

	mw, stop, err := BackendProxyMiddlewareWithRelay(backend.URL)
	if err != nil {
		t.Fatalf("BackendProxyMiddlewareWithRelay: %v", err)
	}
	t.Cleanup(stop)

	h := mw(assetHandler(map[string]string{"/index.html": "<html><head></head><body>ui</body></html>"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/index.html", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /index.html = %d, want 200", rec.Code)
	}

	match := wsOriginPattern.FindStringSubmatch(rec.Body.String())
	if match == nil {
		t.Fatalf("document missing ws shim origin: %.300s", rec.Body.String())
	}
	shimTarget, err := url.Parse(match[1])
	if err != nil {
		t.Fatalf("parse shim ws origin %q: %v", match[1], err)
	}
	backendHost := strings.TrimPrefix(backend.URL, "http://")
	if shimTarget.Host == backendHost {
		t.Fatalf("shim targets backend %q directly, want relay", shimTarget.Host)
	}

	// The advertised relay origin must accept an upgrade carrying the
	// webview Origin the backend would reject, and strip it.
	conn, reader, resp := dialUpgrade(t, shimTarget.Host, "/pty?id=shim")
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("relay handshake status = %d, want 101", resp.StatusCode)
	}
	if got := sawOrigin.Load(); got != "" {
		t.Fatalf("backend saw Origin %q, want stripped", got)
	}
	// Complete the echo round-trip so the backend handler finishes cleanly.
	if _, err := io.WriteString(conn, "shim\n"); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	if echo, err := reader.ReadString('\n'); err != nil || echo != "echo:shim\n" {
		t.Fatalf("echo = %q, err = %v", echo, err)
	}
}
