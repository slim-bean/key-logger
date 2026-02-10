package buffer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

const (
	maxSegmentBytes   = 1 << 20 // 1MB per segment
	maxSegmentEntries = 10000
)

// WAL is a write-ahead log that persists log entries to disk as
// newline-delimited JSON in numbered segment files.
type WAL struct {
	dir          string
	maxTotalSize int64
	logger       log.Logger

	mu             sync.Mutex
	currentFile    *os.File
	currentSegment string // base name, e.g. "000001.wal"
	currentSize    int64
	currentEntries int
	segmentCounter uint64
}

// NewWAL creates a new WAL in the given directory. maxTotalSize is the
// maximum total buffer size in bytes; when exceeded the oldest segments
// are deleted. The WAL creates the directory if it does not exist and
// opens a new segment for writing.
func NewWAL(dir string, maxTotalSize int64, logger log.Logger) (*WAL, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating buffer directory: %w", err)
	}

	w := &WAL{
		dir:          dir,
		maxTotalSize: maxTotalSize,
		logger:       logger,
	}

	// Find the highest existing segment number so we continue the sequence.
	segments, err := w.listSegments()
	if err != nil {
		return nil, err
	}
	if len(segments) > 0 {
		last := filepath.Base(segments[len(segments)-1])
		fmt.Sscanf(last, "%06d.wal", &w.segmentCounter)
	}

	if err := w.rotateLocked(); err != nil {
		return nil, fmt.Errorf("opening initial segment: %w", err)
	}
	return w, nil
}

// Append writes an entry to the current segment. It rotates to a new
// segment when the current one exceeds size or entry count limits.
// Each call writes a complete JSON line atomically under the mutex.
func (w *WAL) Append(e Entry) error {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshaling entry: %w", err)
	}
	data = append(data, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentSize+int64(len(data)) > maxSegmentBytes || w.currentEntries >= maxSegmentEntries {
		if err := w.rotateLocked(); err != nil {
			return fmt.Errorf("rotating segment: %w", err)
		}
	}

	n, err := w.currentFile.Write(data)
	if err != nil {
		return fmt.Errorf("writing entry: %w", err)
	}
	w.currentSize += int64(n)
	w.currentEntries++

	w.enforceMaxSizeLocked()
	return nil
}

// CurrentSegment returns the base name of the segment currently being
// written to (e.g. "000003.wal").
func (w *WAL) CurrentSegment() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.currentSegment
}

// Dir returns the buffer directory path.
func (w *WAL) Dir() string {
	return w.dir
}

// Close flushes and closes the current segment file.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.currentFile != nil {
		return w.currentFile.Close()
	}
	return nil
}

// ListSegments returns all .wal files in the buffer directory, sorted
// by name (which is also chronological order).
func (w *WAL) ListSegments() ([]string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.listSegments()
}

// listSegments is the unlocked version for internal use.
func (w *WAL) listSegments() ([]string, error) {
	entries, err := filepath.Glob(filepath.Join(w.dir, "*.wal"))
	if err != nil {
		return nil, fmt.Errorf("listing segments: %w", err)
	}
	sort.Strings(entries)
	return entries, nil
}

// rotateLocked closes the current segment and opens a new one.
// Must be called with w.mu held.
func (w *WAL) rotateLocked() error {
	if w.currentFile != nil {
		w.currentFile.Close()
	}
	w.segmentCounter++
	name := fmt.Sprintf("%06d.wal", w.segmentCounter)
	path := filepath.Join(w.dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("opening segment %s: %w", name, err)
	}
	w.currentFile = f
	w.currentSegment = name
	w.currentSize = 0
	w.currentEntries = 0
	return nil
}

// enforceMaxSizeLocked deletes oldest segments if total buffer size
// exceeds the configured maximum. Never deletes the current segment.
// Must be called with w.mu held.
func (w *WAL) enforceMaxSizeLocked() {
	segments, err := w.listSegments()
	if err != nil {
		return
	}

	type segInfo struct {
		path string
		size int64
	}
	var infos []segInfo
	var totalSize int64
	for _, s := range segments {
		fi, err := os.Stat(s)
		if err != nil {
			continue
		}
		infos = append(infos, segInfo{s, fi.Size()})
		totalSize += fi.Size()
	}

	for totalSize > w.maxTotalSize && len(infos) > 1 {
		oldest := infos[0]
		if filepath.Base(oldest.path) == w.currentSegment {
			break
		}
		if err := os.Remove(oldest.path); err != nil {
			level.Error(w.logger).Log("msg", "failed to remove oldest segment", "segment", oldest.path, "err", err)
			break
		}
		level.Warn(w.logger).Log("msg", "buffer full, dropped oldest segment", "segment", filepath.Base(oldest.path))
		totalSize -= oldest.size
		infos = infos[1:]
	}
}
