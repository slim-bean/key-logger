package screencap

import (
	"fmt"
	"image"
	"syscall"

	"github.com/kbinani/screenshot"
)

var (
	shellscaling               = syscall.NewLazyDLL("Shcore.dll")
	setProcessDpiAwarenessProc = shellscaling.NewProc("SetProcessDpiAwareness")
)

type windowsCapturer struct{}

// New creates a new Capturer for Windows.
func New() Capturer {
	return &windowsCapturer{}
}

func (c *windowsCapturer) Init() error {
	// Tell Windows we are per monitor DPI aware.
	ret, _, err := setProcessDpiAwarenessProc.Call(2)
	if ret != 0 {
		fmt.Println("Set DPI Error", err)
	}
	return nil
}

func (c *windowsCapturer) CaptureRect(bounds image.Rectangle) (*image.RGBA, error) {
	return screenshot.CaptureRect(bounds)
}
