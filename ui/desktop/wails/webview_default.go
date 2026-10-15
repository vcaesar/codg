//go:build !darwin || !private_mac_apis

package main

// transparentWebview is false: without -tags private_mac_apis Wails v3 keeps
// the macOS WKWebView opaque above the translucent backdrop.
const transparentWebview = false
