//go:build darwin

package pub

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// pubCenterTrafficLights repositions the three standard window buttons
// (close / minimise / zoom) so their vertical centre sits `centerFromTop`
// points below the top of their titlebar container view, while preserving
// their horizontal layout.
//
// macOS normally centres the buttons within the (short) titlebar container,
// which leaves them sitting higher than a custom CSS toolbar. By computing
// the offset from the container's *actual* height at call time, this works
// regardless of the configured Mac toolbar style, and must be re-applied
// after the window lays out and on every resize, because AppKit re-centres
// the buttons on those events.
static void pubCenterTrafficLights(NSWindow *window, double centerFromTop) {
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

// pubCenterTrafficLightsPtr is the untyped-handle entry point for shells
// that hold the NSWindow as a void* (Wails v3's NativeWindow()).
static void pubCenterTrafficLightsPtr(void *nsWindowPtr, double centerFromTop) {
	if (nsWindowPtr == NULL) {
		return;
	}
	pubCenterTrafficLights((NSWindow *)nsWindowPtr, centerFromTop);
}

// pubInstallTrafficLightCentering centres the buttons of every current
// window once, then keeps them centred by observing the notifications on
// which AppKit re-centres them (window resize, becoming key — the latter
// also fires when the window is first shown and after the WebView's initial
// layout). For shells that expose neither the NSWindow handle nor Mac
// window events to Go (Wails v2), the re-apply logic lives entirely here.
// All AppKit work is dispatched onto the main queue.
static void pubInstallTrafficLightCentering(double centerFromTop) {
	dispatch_async(dispatch_get_main_queue(), ^{
		void (^recenter)(NSNotification *) = ^(NSNotification *note) {
			NSWindow *w = (NSWindow *)note.object;
			if ([w isKindOfClass:[NSWindow class]]) {
				pubCenterTrafficLights(w, centerFromTop);
			}
		};
		NSNotificationCenter *nc = [NSNotificationCenter defaultCenter];
		NSOperationQueue *main = [NSOperationQueue mainQueue];
		// Observers live for the app's lifetime; the tokens are never removed.
		[nc addObserverForName:NSWindowDidResizeNotification object:nil queue:main usingBlock:recenter];
		[nc addObserverForName:NSWindowDidBecomeKeyNotification object:nil queue:main usingBlock:recenter];
		for (NSWindow *w in [NSApp windows]) {
			pubCenterTrafficLights(w, centerFromTop);
		}
	});
}
*/
import "C"

import "unsafe"

// TrafficLightCenterFromTop is the vertical centre (in points, from the top
// of the window) where the macOS traffic-light buttons should sit. It is
// half the shared React toolbar height (h-10 = 40px) so the buttons line up
// with the "Toggle Sidebar" button in both desktop shells.
const TrafficLightCenterFromTop = 20.0

// CenterTrafficLights moves the native window controls of the given NSWindow
// so they vertically align with the in-app toolbar. Used by the Wails v3
// shell, which re-applies it on window events; the caller must ensure it
// runs on the main thread (e.g. application.InvokeAsync). No-op when the
// handle is nil.
func CenterTrafficLights(nsWindow unsafe.Pointer) {
	if nsWindow == nil {
		return
	}
	C.pubCenterTrafficLightsPtr(nsWindow, C.double(TrafficLightCenterFromTop))
}

// InstallTrafficLightCentering centres the window controls of all current
// windows and keeps them centred via AppKit notifications. Used by the
// Wails v2 shell, which cannot observe Mac window events from Go. Safe to
// call from any goroutine; the work runs on the main queue.
func InstallTrafficLightCentering() {
	C.pubInstallTrafficLightCentering(C.double(TrafficLightCenterFromTop))
}
