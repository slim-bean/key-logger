package keylogger

// KeyEvent represents a platform-independent keyboard event.
type KeyEvent struct {
	// Name is the human-readable representation of the key.
	// For printable characters: the character itself (e.g. "a", "1", ";").
	// For special keys: a bracketed name (e.g. "[Ctrl]", "[F1]", "[Tab]").
	Name string

	// IsReturn indicates this is an Enter/Return key press,
	// which triggers flushing the accumulated text buffer.
	IsReturn bool

	// IsBack indicates this is a Backspace key press,
	// which triggers removal of the last character from the text buffer.
	IsBack bool
}

// KeyLogger captures keyboard events from the operating system.
type KeyLogger interface {
	// Start begins capturing keyboard events.
	// This may install OS-level hooks or event taps.
	Start() error

	// Events returns a read-only channel that emits KeyEvent values
	// as keys are pressed.
	Events() <-chan KeyEvent

	// Stop releases any OS resources (hooks, event taps, etc.)
	Stop() error
}
