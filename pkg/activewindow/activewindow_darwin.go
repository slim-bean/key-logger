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
	char windowName[512];
	double x, y, w, h;
	int found;
} CWindowInfo;

// getFrontmostWindow queries CGWindowList (which always returns live state
// from the window server) to find the frontmost normal application window.
// Windows are returned in Z-order (front to back), so the first large,
// normal-layer window belongs to the active application.
// This does NOT require NSWorkspace or an event loop.
static CWindowInfo getFrontmostWindow(void) {
	CWindowInfo info = {0};
	CFArrayRef list = CGWindowListCopyWindowInfo(
		kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
		kCGNullWindowID
	);
	if (!list) return info;

	CFIndex count = CFArrayGetCount(list);
	for (CFIndex i = 0; i < count; i++) {
		CFDictionaryRef win = (CFDictionaryRef)CFArrayGetValueAtIndex(list, i);

		// Only consider normal-layer windows (layer 0).
		CFNumberRef layerRef = (CFNumberRef)CFDictionaryGetValue(win, kCGWindowLayer);
		if (!layerRef) continue;
		int layer = -1;
		CFNumberGetValue(layerRef, kCFNumberIntType, &layer);
		if (layer != 0) continue;

		// Get window bounds.
		CFDictionaryRef boundsRef = (CFDictionaryRef)CFDictionaryGetValue(win, kCGWindowBounds);
		if (!boundsRef) continue;
		CGRect rect;
		CGRectMakeWithDictionaryRepresentation(boundsRef, &rect);

		// Skip tiny windows (menu bar items, status icons, toolbars, etc.)
		if (rect.size.width < 50 || rect.size.height < 50) continue;

		// This is the frontmost real window. Extract info.
		CFNumberRef pidRef = (CFNumberRef)CFDictionaryGetValue(win, kCGWindowOwnerPID);
		if (pidRef) {
			CFNumberGetValue(pidRef, kCFNumberIntType, &info.pid);
		}

		CFStringRef ownerRef = (CFStringRef)CFDictionaryGetValue(win, kCGWindowOwnerName);
		if (ownerRef) {
			CFStringGetCString(ownerRef, info.appName,
				sizeof(info.appName), kCFStringEncodingUTF8);
		}

		// kCGWindowName requires Screen Recording permission; may be NULL.
		CFStringRef nameRef = (CFStringRef)CFDictionaryGetValue(win, kCGWindowName);
		if (nameRef) {
			CFStringGetCString(nameRef, info.windowName,
				sizeof(info.windowName), kCFStringEncodingUTF8);
		}

		info.x = rect.origin.x;
		info.y = rect.origin.y;
		info.w = rect.size.width;
		info.h = rect.size.height;
		info.found = 1;
		break;
	}

	CFRelease(list);
	return info;
}

// getWindowTitleViaAX uses the Accessibility API to get the focused window's
// title. Returns 1 if successful, 0 otherwise. Requires Accessibility permission.
static int getWindowTitleViaAX(int targetPID, char *outTitle, int maxLen) {
	AXUIElementRef appRef = AXUIElementCreateApplication(targetPID);
	if (!appRef) return 0;

	AXUIElementRef windowRef = NULL;
	AXError err = AXUIElementCopyAttributeValue(
		appRef, kAXFocusedWindowAttribute, (CFTypeRef *)&windowRef);

	if (err != kAXErrorSuccess || !windowRef) {
		CFRelease(appRef);
		return 0;
	}

	int result = 0;
	CFStringRef titleRef = NULL;
	err = AXUIElementCopyAttributeValue(
		windowRef, kAXTitleAttribute, (CFTypeRef *)&titleRef);
	if (err == kAXErrorSuccess && titleRef) {
		CFStringGetCString(titleRef, outTitle, maxLen, kCFStringEncodingUTF8);
		CFRelease(titleRef);
		result = 1;
	}

	CFRelease(windowRef);
	CFRelease(appRef);
	return result;
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
	"unsafe"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type darwinTracker struct {
	logger log.Logger
	mtx    sync.Mutex
	info   Info
	idle   time.Duration
	axOK   bool // whether Accessibility permission is available
}

// New creates a new Tracker for macOS.
func New(logger log.Logger) Tracker {
	return &darwinTracker{logger: logger}
}

func (t *darwinTracker) Start() error {
	t.axOK = C.isAccessibilityTrusted() != 0
	if !t.axOK {
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
	// Re-check AX permission periodically in case the user grants it at runtime.
	axCheckCounter := 0

	for {
		// CGWindowList always returns live data from the window server.
		winfo := C.getFrontmostWindow()

		var info Info
		if winfo.found != 0 {
			appName := C.GoString(&winfo.appName[0])
			winTitle := C.GoString(&winfo.windowName[0])

			// Try Accessibility API for window title (more reliable, works
			// without Screen Recording permission).
			if t.axOK {
				var axTitle [512]C.char
				if C.getWindowTitleViaAX(winfo.pid, &axTitle[0], 512) != 0 {
					axStr := C.GoString(&axTitle[0])
					if axStr != "" {
						winTitle = axStr
					}
				}
			}

			displayName := appName
			if winTitle != "" {
				displayName = fmt.Sprintf("%s - %s", winTitle, appName)
			}

			info = Info{
				WindowName: displayName,
				Process:    appName,
				Bounds: image.Rect(
					int(winfo.x), int(winfo.y),
					int(winfo.x+winfo.w), int(winfo.y+winfo.h),
				),
			}
		}

		idleSec := float64(C.getSystemIdleTime())
		idle := time.Duration(idleSec * float64(time.Second))

		t.mtx.Lock()
		t.info = info
		t.idle = idle
		t.mtx.Unlock()

		// Re-check Accessibility permission every ~30s (60 iterations * 500ms).
		axCheckCounter++
		if !t.axOK && axCheckCounter%60 == 0 {
			t.axOK = C.isAccessibilityTrusted() != 0
			if t.axOK {
				level.Info(t.logger).Log("msg", "Accessibility permission granted; window titles now available")
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// Ensure unsafe is used (needed for potential future use with C pointers).
var _ = unsafe.Pointer(nil)
