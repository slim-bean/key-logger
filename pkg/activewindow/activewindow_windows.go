package activewindow

import (
	"fmt"
	"image"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

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

type windowsTracker struct {
	mtx        sync.Mutex
	info       Info
	idleTimeMs uint32
}

// New creates a new Tracker for Windows.
func New() Tracker {
	return &windowsTracker{}
}

func (t *windowsTracker) Start() error {
	go t.updateLoop()
	go t.idleLoop()
	return nil
}

func (t *windowsTracker) GetActiveWindow() Info {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.info
}

func (t *windowsTracker) GetIdleTime() time.Duration {
	ms := atomic.LoadUint32(&t.idleTimeMs)
	return time.Duration(ms) * time.Millisecond
}

func (t *windowsTracker) Stop() error {
	return nil
}

func (t *windowsTracker) updateLoop() {
	for {
		handle := w32.GetForegroundWindow()
		_, id := w32.GetWindowThreadProcessId(handle)
		ph := w32.OpenProcess(w32.PROCESS_QUERY_INFORMATION, false, uint32(id))
		filename := getProcessFileName(ph)
		w32.CloseHandle(ph)
		rect := w32.GetWindowRect(handle)
		bounds := image.Rect(int(rect.Left), int(rect.Top), int(rect.Right), int(rect.Bottom))

		t.mtx.Lock()
		t.info = Info{
			WindowName: w32.GetWindowText(handle),
			Process:    filename,
			Bounds:     bounds,
		}
		t.mtx.Unlock()
		time.Sleep(500 * time.Millisecond)
	}
}

type lastInputInfo struct {
	CbSize uint32
	DwTime uint32
}

func (t *windowsTracker) idleLoop() {
	lii := &lastInputInfo{}
	lii.CbSize = uint32(unsafe.Sizeof(lii))
	for {
		ret, _, _ := getLastInputInfoProc.Call(uintptr(unsafe.Pointer(lii)))
		if ret == 0 {
			fmt.Println("getLastInputInfo Failed")
		}
		tick := getTickCount()
		atomic.StoreUint32(&t.idleTimeMs, tick-lii.DwTime)
		time.Sleep(1 * time.Second)
	}
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

func getTickCount() uint32 {
	ret, _, _ := getTickCountProc.Call()
	return uint32(ret)
}
