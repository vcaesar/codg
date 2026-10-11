package main

import (
	"fmt"
	"runtime"
)

// titleBarHeight matches the 40px (h-10) React toolbar, mirroring the v3
// shell's MacWindow.InvisibleTitleBarHeight.
const titleBarHeight = 40

// titleBarDragShim returns an inline <script> that makes the top
// titleBarHeight pixels of the page drag the window, like the v3 shell's
// invisible title bar. Wails v2 only drags elements styled with
// --wails-draggable, which the shared web UI does not set, and its
// full-size-content webview covers the native macOS title bar. The drag is
// deferred to the first mousemove so clicks on toolbar controls still work,
// and interactive elements never start a drag. Non-macOS shells keep their
// native title bar, so the shim is empty there.
func titleBarDragShim(goos string) []byte {
	if goos != "darwin" {
		return nil
	}
	return fmt.Appendf(nil, `<script>/* codg desktop: Wails v2 title-bar drag region. */
(function () {
  "use strict";
  var height = %d, edge = 5, pending = false;
  var skip = "button,a,input,textarea,select,label,[role=button],[role=tab],[role=menuitem],[contenteditable],[data-no-drag]";
  window.addEventListener("mousedown", function (e) {
    pending = e.button === 0 && e.detail === 1 && e.clientY < height &&
      e.clientX > edge && e.clientX < window.innerWidth - edge &&
      !(e.target instanceof Element && e.target.closest(skip));
  }, true);
  window.addEventListener("mousemove", function (e) {
    if (!pending) { return; }
    pending = false;
    if (e.buttons === 1 && window.WailsInvoke) { window.WailsInvoke("drag"); }
  }, true);
  window.addEventListener("mouseup", function () { pending = false; }, true);
})();
</script>`, titleBarHeight)
}

// documentScripts lists the extra scripts this shell injects into HTML.
func documentScripts() [][]byte {
	if shim := titleBarDragShim(runtime.GOOS); shim != nil {
		return [][]byte{shim}
	}
	return nil
}
