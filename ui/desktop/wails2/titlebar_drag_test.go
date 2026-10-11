package main

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestTitleBarDragShim(t *testing.T) {
	t.Parallel()
	if shim := titleBarDragShim("windows"); shim != nil {
		t.Fatalf("non-macOS shells keep the native title bar, got %s", shim)
	}
	shim := string(titleBarDragShim("darwin"))
	for _, want := range []string{"var height = 40", `window.WailsInvoke("drag")`, "closest(skip)", "e.detail === 1"} {
		if !strings.Contains(shim, want) {
			t.Fatalf("drag shim missing %q: %s", want, shim)
		}
	}
}

func TestDesktopMarkerMiddlewareInjectsDragShim(t *testing.T) {
	t.Parallel()
	assets := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte("<html><head></head><body></body></html>")); err != nil {
			t.Errorf("write: %v", err)
		}
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept", "text/html")
	desktopMarkerMiddleware()(assets).ServeHTTP(recorder, request)

	injected := strings.Contains(recorder.Body.String(), "title-bar drag region")
	if injected != (runtime.GOOS == "darwin") {
		t.Fatalf("drag shim injected=%v on %s: %s", injected, runtime.GOOS, recorder.Body.String())
	}
}
