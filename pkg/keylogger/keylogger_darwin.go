package keylogger

type darwinKeyLogger struct {
	events chan KeyEvent
}

// New creates a new KeyLogger for macOS.
func New() KeyLogger {
	return &darwinKeyLogger{
		events: make(chan KeyEvent, 100),
	}
}

func (k *darwinKeyLogger) Start() error {
	// TODO: implement macOS key logging via CGEventTap
	return nil
}

func (k *darwinKeyLogger) Events() <-chan KeyEvent {
	return k.events
}

func (k *darwinKeyLogger) Stop() error {
	return nil
}
