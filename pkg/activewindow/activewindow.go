package activewindow

import (
	"image"
	"time"
)

// Info holds information about the currently active window.
type Info struct {
	WindowName string
	Process    string
	Bounds     image.Rectangle
}

// Tracker monitors the active window and system idle time.
type Tracker interface {
	// Start begins monitoring the active window and idle time.
	Start() error

	// GetActiveWindow returns the currently active window info.
	GetActiveWindow() Info

	// GetIdleTime returns how long the system has been idle.
	GetIdleTime() time.Duration

	// Stop releases OS resources.
	Stop() error
}
