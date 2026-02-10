package screencap

import (
	"fmt"
	"image"
)

type darwinCapturer struct{}

// New creates a new Capturer for macOS.
func New() Capturer {
	return &darwinCapturer{}
}

func (c *darwinCapturer) Init() error {
	// No platform-specific initialization needed on macOS.
	return nil
}

func (c *darwinCapturer) CaptureRect(bounds image.Rectangle) (*image.RGBA, error) {
	// TODO: implement macOS screen capture using ScreenCaptureKit
	return nil, fmt.Errorf("screen capture not yet implemented on macOS")
}
