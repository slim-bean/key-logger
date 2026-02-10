package buffer

// Entry represents a single log entry to be persisted in the WAL
// and eventually sent to Loki.
type Entry struct {
	// Ts is the nanosecond Unix timestamp. Matches the format Loki expects
	// in push request values.
	Ts int64 `json:"ts"`

	// Labels is the full label set for this entry, including the per-event
	// "job" label and all user-supplied labels.
	Labels map[string]string `json:"labels"`

	// Line is the logfmt-encoded log line content.
	Line string `json:"line"`
}
