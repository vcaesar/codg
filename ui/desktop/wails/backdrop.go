package main

// themeReportScript posts the web UI's resolved theme (the light/dark class
// on <html>) to the shell, initially and on every change. The macOS glass
// theme shows the native backdrop through a transparent page, and that
// backdrop follows the system appearance unless told otherwise, so a dark
// UI on a light system would sit on light vibrancy.
var themeReportScript = []byte(`<script>/* codg desktop: report the resolved theme to the native shell. */
(function () {
  "use strict";
  var external = window.webkit && window.webkit.messageHandlers &&
      window.webkit.messageHandlers.external;
  if (!external) { return; }
  var root = document.documentElement;
  var last = "";
  function report() {
    var theme = root.classList.contains("dark") ? "dark" :
        root.classList.contains("light") ? "light" : "";
    if (theme && theme !== last) {
      last = theme;
      external.postMessage("codg:theme:" + theme);
    }
  }
  new MutationObserver(report).observe(root, { attributes: true, attributeFilter: ["class"] });
  report();
})();
</script>`)

// backdropDark parses a message posted by themeReportScript; ok is false for
// any other raw webview message.
func backdropDark(message string) (dark, ok bool) {
	switch message {
	case "codg:theme:dark":
		return true, true
	case "codg:theme:light":
		return false, true
	}
	return false, false
}
