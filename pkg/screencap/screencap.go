package screencap

import "image"

// Capturer captures screenshots from the display.
type Capturer interface {
	// Init performs any platform-specific initialization
	// (e.g., DPI awareness on Windows). Call once before CaptureRect.
	Init() error

	// CaptureRect captures a screenshot of the specified screen rectangle.
	CaptureRect(bounds image.Rectangle) (*image.RGBA, error)
}
