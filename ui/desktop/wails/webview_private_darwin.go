//go:build darwin && private_mac_apis

package main

// transparentWebview reports whether Wails makes the WKWebView transparent,
// letting the MacBackdropTranslucent vibrancy show through the page.
const transparentWebview = true
