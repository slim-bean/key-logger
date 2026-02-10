package activewindow

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework AppKit -framework ApplicationServices

#include <CoreGraphics/CoreGraphics.h>
#include <ApplicationServices/ApplicationServices.h>
#import <AppKit/AppKit.h>

typedef struct {
	int pid;
	char appName[256];
} FrontmostApp;

static FrontmostApp getFrontmostApp(void) {
	FrontmostApp app = {0};
	@autoreleasepool {
		NSRunningApplication *frontApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
		if (frontApp) {
			app.pid = frontApp.processIdentifier;
			const char *name = [frontApp.localizedName UTF8String];
			if (name) strncpy(app.appName, name, sizeof(app.appName) - 1);
		}
	}
	return app;
}

typedef struct {
	char windowName[512];
	double x, y, w, h;
	int found;       // 1 = found via AX, 2 = found via CGWindowList fallback
} CWindowInfo;

// getWindowViaAX uses the Accessibility API to get the focused window's
// title and bounds. Requires Accessibility permission.
static CWindowInfo getWindowViaAX(int targetPID) {
	CWindowInfo info = {0};

	AXUIElementRef appRef = AXUIElementCreateApplication(targetPID);
	if (!appRef) return info;

	AXUIElementRef windowRef = NULL;
	AXError err = AXUIElementCopyAttributeValue(
		appRef, kAXFocusedWindowAttribute, (CFTypeRef *)&windowRef);

	if (err != kAXErrorSuccess || !windowRef) {
		CFRelease(appRef);
		return info;
	}

	// Get window title.
	CFStringRef titleRef = NULL;
	err = AXUIElementCopyAttributeValue(
		windowRef, kAXTitleAttribute, (CFTypeRef *)&titleRef);
	if (err == kAXErrorSuccess && titleRef) {
		CFStringGetCString(titleRef, info.windowName,
			sizeof(info.windowName), kCFStringEncodingUTF8);
		CFRelease(titleRef);
	}

	// Get window position.
	AXValueRef posRef = NULL;
	CGPoint pos = {0, 0};
	err = AXUIElementCopyAttributeValue(
		windowRef, kAXPositionAttribute, (CFTypeRef *)&posRef);
	if (err == kAXErrorSuccess && posRef) {
		AXValueGetValue(posRef, kAXValueCGPointType, &pos);
		CFRelease(posRef);
	}

	// Get window size.
	AXValueRef sizeRef = NULL;
	CGSize size = {0, 0};
	err = AXUIElementCopyAttributeValue(
		windowRef, kAXSizeAttribute, (CFTypeRef *)&sizeRef);
	if (err == kAXErrorSuccess && sizeRef) {
		AXValueGetValue(sizeRef, kAXValueCGSizeType, &size);
		CFRelease(sizeRef);
	}

	info.x = pos.x;
	info.y = pos.y;
	info.w = size.width;
	info.h = size.height;
	info.found = 1;

	CFRelease(windowRef);
	CFRelease(appRef);
	return info;
}

// getWindowBoundsViaCGWindowList uses CGWindowListCopyWindowInfo as a
// fallback to get window bounds when the Accessibility API is not available.
// This does NOT require Accessibility or Screen Recording permission for bounds,
// but cannot retrieve window titles without Screen Recording permission.
// It finds the largest normal-layer window for the given PID.
static CWindowInfo getWindowBoundsViaCGWindowList(int targetPID) {
	CWindowInfo info = {0};
	CFArrayRef list = CGWindowListCopyWindowInfo(
		kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
		kCGNullWindowID
	);
	if (!list) return info;

	double bestArea = 0;
	CFIndex count = CFArrayGetCount(list);
	for (CFIndex i = 0; i < count; i++) {
		CFDictionaryRef win = (CFDictionaryRef)CFArrayGetValueAtIndex(list, i);

		CFNumberRef pidRef = (CFNumberRef)CFDictionaryGetValue(win, kCGWindowOwnerPID);
		if (!pidRef) continue;
		int pid = 0;
		CFNumberGetValue(pidRef, kCFNumberIntType, &pid);
		if (pid != targetPID) continue;

		CFNumberRef layerRef = (CFNumberRef)CFDictionaryGetValue(win, kCGWindowLayer);
		if (layerRef) {
			int layer = -1;
			CFNumberGetValue(layerRef, kCFNumberIntType, &layer);
			if (layer != 0) continue;
		}

		CFDictionaryRef boundsRef = (CFDictionaryRef)CFDictionaryGetValue(win, kCGWindowBounds);
		if (!boundsRef) continue;

		CGRect rect;
		CGRectMakeWithDictionaryRepresentation(boundsRef, &rect);

		// Skip tiny windows (menu bar items, status icons, etc.)
		if (rect.size.width < 50 || rect.size.height < 50) continue;

		double area = rect.size.width * rect.size.height;
		if (area > bestArea) {
			bestArea = area;
			info.x = rect.origin.x;
			info.y = rect.origin.y;
			info.w = rect.size.width;
			info.h = rect.size.height;
			info.found = 2;

			// Try to get window name (only works with Screen Recording permission).
			CFStringRef nameRef = (CFStringRef)CFDictionaryGetValue(win, kCGWindowName);
			if (nameRef) {
				CFStringGetCString(nameRef, info.windowName,
					sizeof(info.windowName), kCFStringEncodingUTF8);
			}
		}
	}

	CFRelease(list);
	return info;
}

static int isAccessibilityTrusted(void) {
	return AXIsProcessTrusted();
}

static double getSystemIdleTime(void) {
	return CGEventSourceSecondsSinceLastEventType(
		kCGEventSourceStateCombinedSessionState,
		kCGAnyInputEventType
	);
}
*/
import "C"

import (
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type darwinTracker struct {
	logger log.Logger
	mtx    sync.Mutex
	info   Info
	idle   time.Duration
}

// New creates a new Tracker for macOS using the Accessibility API
// with a CGWindowList fallback for bounds.
func New(logger log.Logger) Tracker {
	return &darwinTracker{logger: logger}
}

func (t *darwinTracker) Start() error {
	if C.isAccessibilityTrusted() == 0 {
		level.Warn(t.logger).Log("msg",
			"Accessibility permission not granted; window titles will show app name only. "+
				"Grant permission in System Settings > Privacy & Security > Accessibility.")
	}
	go t.pollLoop()
	return nil
}

func (t *darwinTracker) GetActiveWindow() Info {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.info
}

func (t *darwinTracker) GetIdleTime() time.Duration {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.idle
}

func (t *darwinTracker) Stop() error {
	return nil
}

func (t *darwinTracker) pollLoop() {
	for {
		app := C.getFrontmostApp()

		var info Info
		if app.pid > 0 {
			appName := C.GoString(&app.appName[0])

			// Try Accessibility API first (best results: title + bounds).
			winfo := C.getWindowViaAX(app.pid)

			if winfo.found == 0 {
				// AX failed; fall back to CGWindowList for bounds.
				winfo = C.getWindowBoundsViaCGWindowList(app.pid)
			}

			if winfo.found != 0 {
				winName := C.GoString(&winfo.windowName[0])
				if winName == "" {
					winName = appName
				} else {
					winName = fmt.Sprintf("%s - %s", winName, appName)
				}
				info = Info{
					WindowName: winName,
					Process:    appName,
					Bounds: image.Rect(
						int(winfo.x), int(winfo.y),
						int(winfo.x+winfo.w), int(winfo.y+winfo.h),
					),
				}
			} else {
				info = Info{
					WindowName: appName,
					Process:    appName,
				}
			}
		}

		idleSec := float64(C.getSystemIdleTime())
		idle := time.Duration(idleSec * float64(time.Second))

		t.mtx.Lock()
		t.info = info
		t.idle = idle
		t.mtx.Unlock()

		time.Sleep(500 * time.Millisecond)
	}
}
