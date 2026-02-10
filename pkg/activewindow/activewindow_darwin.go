package activewindow

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework AppKit

#include <CoreGraphics/CoreGraphics.h>
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
	char processName[256];
	double x, y, w, h;
	int found;
} CWindowInfo;

static CWindowInfo getWindowForPID(int targetPID) {
	CWindowInfo info = {0};
	CFArrayRef list = CGWindowListCopyWindowInfo(
		kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
		kCGNullWindowID
	);
	if (!list) return info;

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

		CFStringRef nameRef = (CFStringRef)CFDictionaryGetValue(win, kCGWindowName);
		if (nameRef) {
			CFStringGetCString(nameRef, info.windowName, sizeof(info.windowName), kCFStringEncodingUTF8);
		}

		CFStringRef ownerRef = (CFStringRef)CFDictionaryGetValue(win, kCGWindowOwnerName);
		if (ownerRef) {
			CFStringGetCString(ownerRef, info.processName, sizeof(info.processName), kCFStringEncodingUTF8);
		}

		CFDictionaryRef boundsRef = (CFDictionaryRef)CFDictionaryGetValue(win, kCGWindowBounds);
		if (boundsRef) {
			CGRect rect;
			CGRectMakeWithDictionaryRepresentation(boundsRef, &rect);
			info.x = rect.origin.x;
			info.y = rect.origin.y;
			info.w = rect.size.width;
			info.h = rect.size.height;
		}

		info.found = 1;
		break;
	}

	CFRelease(list);
	return info;
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
	"image"
	"sync"
	"time"
)

type darwinTracker struct {
	mtx  sync.Mutex
	info Info
	idle time.Duration
}

// New creates a new Tracker for macOS using CGWindowList and NSWorkspace.
func New() Tracker {
	return &darwinTracker{}
}

func (t *darwinTracker) Start() error {
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
			winfo := C.getWindowForPID(app.pid)
			if winfo.found != 0 {
				info = Info{
					WindowName: C.GoString(&winfo.windowName[0]),
					Process:    C.GoString(&winfo.processName[0]),
					Bounds: image.Rect(
						int(winfo.x), int(winfo.y),
						int(winfo.x+winfo.w), int(winfo.y+winfo.h),
					),
				}
			} else {
				// Window info not available (e.g., Screen Recording permission missing),
				// fall back to app name only.
				info = Info{
					WindowName: C.GoString(&app.appName[0]),
					Process:    C.GoString(&app.appName[0]),
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
