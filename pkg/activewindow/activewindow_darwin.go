package activewindow

import (
	"image"
	"time"
)

type darwinTracker struct{}

// New creates a new Tracker for macOS.
func New() Tracker {
	return &darwinTracker{}
}

func (t *darwinTracker) Start() error {
	// TODO: implement macOS active window tracking
	return nil
}

func (t *darwinTracker) GetActiveWindow() Info {
	return Info{
		Bounds: image.Rect(0, 0, 0, 0),
	}
}

func (t *darwinTracker) GetIdleTime() time.Duration {
	return 0
}

func (t *darwinTracker) Stop() error {
	return nil
}
