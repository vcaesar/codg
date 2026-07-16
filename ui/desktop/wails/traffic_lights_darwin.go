//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// centerTrafficLights repositions the three standard window buttons
// (close / minimise / zoom) so their vertical centre sits `centerFromTop`
// points below the top of their titlebar container view, while preserving
// their horizontal layout.
//
// macOS normally centres the buttons within the (short) titlebar container,
// which leaves them sitting higher than a custom CSS toolbar. By computing
// the offset from the container's *actual* height at call time, this works
// regardless of the configured Mac toolbar style, and must be re-applied
// after the window lays out (navigation finished) and on every resize,
// because AppKit re-centres the buttons on those events.
static void centerTrafficLights(void *nsWindowPtr, double centerFromTop) {
	if (nsWindowPtr == NULL) {
		return;
	}
	NSWindow *window = (NSWindow *)nsWindowPtr;

	NSButton *close = [window standardWindowButton:NSWindowCloseButton];
	NSButton *mini  = [window standardWindowButton:NSWindowMiniaturizeButton];
	NSButton *zoom  = [window standardWindowButton:NSWindowZoomButton];
	if (close == nil || mini == nil || zoom == nil) {
		return;
	}

	NSView *container = [close superview];
	if (container == nil) {
		return;
	}

	// Titlebar container uses a bottom-left origin (non-flipped). To place a
	// button's centre `centerFromTop` points below the container top, its
	// origin.y is: containerHeight - centerFromTop - buttonHeight/2.
	CGFloat containerH = container.frame.size.height;
	CGFloat buttonH = close.frame.size.height;
	CGFloat y = containerH - centerFromTop - (buttonH / 2.0);

	NSButton *buttons[3] = {close, mini, zoom};
	for (int i = 0; i < 3; i++) {
		NSRect f = buttons[i].frame;
		f.origin.y = y;
		[buttons[i] setFrame:f];
	}
}
*/
import "C"

import "unsafe"

// trafficLightCenterFromTop is the vertical centre (in points, from the top
// of the window) where the macOS traffic-light buttons should sit. It is
// half the React toolbar height (h-10 = 40px) so the buttons line up with
// the "Toggle Sidebar" button.
const trafficLightCenterFromTop = 20.0

// centerTrafficLights moves the native window controls so they vertically
// align with the in-app toolbar. No-op when the handle is nil.
func centerTrafficLights(nsWindow unsafe.Pointer) {
	if nsWindow == nil {
		return
	}
	C.centerTrafficLights(nsWindow, C.double(trafficLightCenterFromTop))
}
