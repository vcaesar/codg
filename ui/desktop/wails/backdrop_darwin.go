//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// codgSetBackdropAppearance pins the translucent backdrop (the
// NSVisualEffectView Wails adds below the webview) to the light or dark
// material. Only that view is touched: the window, and with it the webview's
// prefers-color-scheme, keeps following the system appearance.
static bool codgSetBackdropAppearance(void *nsWindow, bool dark) {
	if (nsWindow == NULL) {
		return false;
	}
	NSAppearance *appearance = [NSAppearance appearanceNamed:dark ? NSAppearanceNameDarkAqua : NSAppearanceNameAqua];
	bool found = false;
	for (NSView *view in [[(NSWindow *)nsWindow contentView] subviews]) {
		if ([view isKindOfClass:[NSVisualEffectView class]]) {
			[view setAppearance:appearance];
			found = true;
		}
	}
	return found;
}
*/
import "C"

import "unsafe"

// setBackdropAppearance makes the window's translucent backdrop light or
// dark and reports whether the window has one. The caller must ensure it
// runs on the main thread (e.g. application.InvokeAsync).
func setBackdropAppearance(nsWindow unsafe.Pointer, dark bool) bool {
	return bool(C.codgSetBackdropAppearance(nsWindow, C.bool(dark)))
}
