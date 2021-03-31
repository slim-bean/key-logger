package window

import (
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gonutz/w32/v2"
)

var (
	psapi                  = syscall.NewLazyDLL("psapi.dll")
	getProcessFilenameProc = psapi.NewProc("GetProcessImageFileNameW")
	winuser                = syscall.NewLazyDLL("user32.dll")
	getLastInputInfoProc   = winuser.NewProc("GetLastInputInfo")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	getTickCountProc       = kernel32.NewProc("GetTickCount")
)

type Window struct {
	logger           log.Logger
	idleTime         uint32
	wmtx             sync.Mutex
	activeProcess    string
	activeWindowName string
}

func New(logger log.Logger) *Window {
	w := &Window{
		logger: logger,
	}
	go w.updateActiveWindowInfoLoop()
	go w.getLastInputInfoLoop()
	go w.logLastInputInfoLoop()
	return w
}

func (w *Window) GetActiveWindowName() string {
	w.wmtx.Lock()
	defer w.wmtx.Unlock()
	return w.activeWindowName
}

func (w *Window) GetActiveProcess() string {
	w.wmtx.Lock()
	defer w.wmtx.Unlock()
	return w.activeProcess
}

func getProcessFileName(mod w32.HANDLE) string {
	var path [32768]uint16
	ret, _, _ := getProcessFilenameProc.Call(
		uintptr(mod),
		uintptr(unsafe.Pointer(&path[0])),
		uintptr(len(path)),
	)
	if ret == 0 {
		return ""
	}
	return syscall.UTF16ToString(path[:])
}

func (w *Window) updateActiveWindowInfoLoop() {
	for {
		handle := w32.GetForegroundWindow()
		_, id := w32.GetWindowThreadProcessId(handle)
		ph := w32.OpenProcess(w32.PROCESS_QUERY_INFORMATION, false, uint32(id))
		filename := getProcessFileName(ph)
		w32.CloseHandle(ph)
		w.wmtx.Lock()
		w.activeProcess = filename
		w.activeWindowName = w32.GetWindowText(handle)
		w.wmtx.Unlock()
		time.Sleep(500 * time.Millisecond)
	}
}

type lastInputInfo struct {
	CbSize uint32
	DwTime uint32
}

func getLastInputInfo(lii *lastInputInfo) bool {
	ret, _, _ := getLastInputInfoProc.Call(uintptr(unsafe.Pointer(lii)))
	return ret != 0
}

func getTickCount() uint32 {
	ret, _, _ := getTickCountProc.Call()
	return uint32(ret)
}

func (w *Window) getLastInputInfoLoop() {
	lii := &lastInputInfo{}
	lii.CbSize = uint32(unsafe.Sizeof(lii))
	for {
		ret := getLastInputInfo(lii)
		if !ret {
			fmt.Println("getLastInputInfo Failed")
		}
		tick := getTickCount()
		atomic.StoreUint32(&w.idleTime, tick-lii.DwTime)
		time.Sleep(1 * time.Second)
	}
}

func (w *Window) logLastInputInfoLoop() {
	for {
		idle := atomic.LoadUint32(&w.idleTime)
		if int64(idle) < (30 * time.Second).Milliseconds() {
			w.wmtx.Lock()
			win := w.activeWindowName
			proc := w.activeProcess
			w.wmtx.Unlock()
			level.Info(w.logger).Log("ts", time.Now(), "type", "active-window", "window", win, "process", proc)
		}
		time.Sleep(5 * time.Second)
	}

}
