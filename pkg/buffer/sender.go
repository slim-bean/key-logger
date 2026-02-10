package buffer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

const (
	maxBatchEntries = 100
	maxBatchBytes   = 512 * 1024 // 500KB
	cursorFile      = "cursor.json"
	cursorTmpFile   = "cursor.tmp"
	pollInterval    = 250 * time.Millisecond
)

// cursor tracks the sender's read position across restarts.
type cursor struct {
	Segment string `json:"segment"`
	Offset  int64  `json:"offset"`
}

// lokiPushRequest is the JSON body for POST /loki/api/v1/push.
type lokiPushRequest struct {
	Streams []lokiStream `json:"streams"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

// sendResult classifies the outcome of a send attempt.
type sendResult int

const (
	sendOK        sendResult = iota
	sendRetryable            // 429, 5xx, network error
	sendPermanent            // 4xx (non-429), bad data
)

// Sender reads entries from WAL segment files and pushes them to Loki
// via the HTTP push API, with retry and exponential backoff for errors.
type Sender struct {
	wal      *WAL
	lokiURL  string
	tenantID string
	client   *http.Client
	logger   log.Logger
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewSender creates a sender that reads from the given WAL and pushes
// to the Loki endpoint at lokiURL. tenantID may be empty for single-tenant.
func NewSender(wal *WAL, lokiURL, tenantID string, logger log.Logger) *Sender {
	ctx, cancel := context.WithCancel(context.Background())
	return &Sender{
		wal:      wal,
		lokiURL:  lokiURL,
		tenantID: tenantID,
		client:   &http.Client{Timeout: 30 * time.Second},
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Run starts the sender loop. It blocks until the context is cancelled
// (via Stop). Call this in a goroutine.
func (s *Sender) Run() {
	cur := s.loadCursor()
	bo := newBackoff(1*time.Second, 5*time.Minute)

	for {
		if s.ctx.Err() != nil {
			return
		}

		segments, err := s.wal.ListSegments()
		if err != nil {
			level.Error(s.logger).Log("msg", "listing segments", "err", err)
			s.sleep(1 * time.Second)
			continue
		}

		if len(segments) == 0 {
			s.sleep(pollInterval)
			continue
		}

		// Find starting segment and offset from the cursor.
		segPath, offset := s.resolveStart(segments, cur)
		if segPath == "" {
			s.sleep(pollInterval)
			continue
		}

		segName := filepath.Base(segPath)
		isCurrent := segName == s.wal.CurrentSegment()

		// Read a batch of entries.
		entries, newOffset, err := s.readBatch(segPath, offset)
		if err != nil {
			level.Error(s.logger).Log("msg", "reading batch", "segment", segName, "err", err)
			s.sleep(1 * time.Second)
			continue
		}

		if len(entries) == 0 {
			if !isCurrent {
				// Completed segment is fully read -- delete it and reset cursor.
				os.Remove(segPath)
				cur = cursor{}
				s.saveCursor(cur)
			} else {
				// Caught up on the current segment. Wait for more data.
				s.sleep(pollInterval)
			}
			continue
		}

		// Warn about old entries that Loki might reject.
		oldest := time.Unix(0, entries[0].Ts)
		if time.Since(oldest) > 7*24*time.Hour {
			level.Warn(s.logger).Log("msg", "sending entries older than 7 days; Loki may reject them",
				"oldest", oldest)
		}

		// Send the batch.
		result, retryAfter := s.sendBatch(entries)
		switch result {
		case sendOK:
			bo.reset()
			cur = cursor{Segment: segName, Offset: newOffset}
			s.saveCursor(cur)

			// If we've read all of a completed segment, delete it.
			if !isCurrent {
				fi, err := os.Stat(segPath)
				if err == nil && newOffset >= fi.Size() {
					os.Remove(segPath)
					cur = cursor{}
					s.saveCursor(cur)
				}
			}

		case sendRetryable:
			wait := retryAfter
			if wait == 0 {
				wait = bo.next()
			}
			level.Warn(s.logger).Log("msg", "send failed (retryable), backing off",
				"backoff", wait)
			s.sleep(wait)

		case sendPermanent:
			// Skip the batch -- permanent error, no point retrying.
			level.Error(s.logger).Log("msg", "send failed (permanent), skipping batch",
				"entries", len(entries))
			bo.reset()
			cur = cursor{Segment: segName, Offset: newOffset}
			s.saveCursor(cur)
		}
	}
}

// Stop signals the sender to shut down.
func (s *Sender) Stop() {
	s.cancel()
}

// resolveStart finds the segment to start reading from and the byte offset
// within it, based on the cursor and available segments.
func (s *Sender) resolveStart(segments []string, cur cursor) (string, int64) {
	if cur.Segment != "" {
		for _, seg := range segments {
			if filepath.Base(seg) == cur.Segment {
				return seg, cur.Offset
			}
		}
		// Cursor segment no longer exists; start from oldest.
		level.Warn(s.logger).Log("msg", "cursor segment not found, starting from oldest",
			"cursor_segment", cur.Segment)
	}
	// No cursor or cursor segment gone -- start from oldest.
	return segments[0], 0
}

// readBatch reads up to maxBatchEntries entries (or maxBatchBytes worth)
// from the segment starting at the given byte offset. Returns the entries
// read and the new byte offset after the last complete line.
func (s *Sender) readBatch(segPath string, offset int64) ([]Entry, int64, error) {
	f, err := os.Open(segPath)
	if err != nil {
		return nil, offset, fmt.Errorf("opening segment: %w", err)
	}
	defer f.Close()

	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return nil, offset, fmt.Errorf("seeking to offset %d: %w", offset, err)
		}
	}

	var entries []Entry
	var batchBytes int
	scanner := bufio.NewScanner(f)
	// Increase scanner buffer for potentially large lines.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	currentOffset := offset
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			currentOffset += 1 // just the newline
			continue
		}

		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			level.Warn(s.logger).Log("msg", "skipping corrupt WAL line",
				"segment", filepath.Base(segPath), "offset", currentOffset, "err", err)
			currentOffset += int64(len(line)) + 1
			continue
		}

		entries = append(entries, e)
		batchBytes += len(line)
		currentOffset += int64(len(line)) + 1 // line + newline

		if len(entries) >= maxBatchEntries || batchBytes >= maxBatchBytes {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return entries, currentOffset, fmt.Errorf("scanning segment: %w", err)
	}

	return entries, currentOffset, nil
}

// sendBatch groups entries by label set and POSTs them to Loki.
// Returns the send result and an optional Retry-After duration.
func (s *Sender) sendBatch(entries []Entry) (sendResult, time.Duration) {
	push := s.buildPushRequest(entries)

	body, err := json.Marshal(push)
	if err != nil {
		level.Error(s.logger).Log("msg", "marshaling push request", "err", err)
		return sendPermanent, 0
	}

	req, err := http.NewRequestWithContext(s.ctx, http.MethodPost, s.lokiURL, bytes.NewReader(body))
	if err != nil {
		level.Error(s.logger).Log("msg", "building request", "err", err)
		return sendPermanent, 0
	}
	req.Header.Set("Content-Type", "application/json")
	if s.tenantID != "" {
		req.Header.Set("X-Scope-OrgID", s.tenantID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		level.Error(s.logger).Log("msg", "sending to Loki", "err", err)
		return sendRetryable, 0
	}
	defer resp.Body.Close()
	// Drain body so the connection can be reused.
	io.Copy(io.Discard, resp.Body)

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return sendOK, 0

	case resp.StatusCode == 429:
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		level.Warn(s.logger).Log("msg", "Loki rate limited (429)", "retry_after", retryAfter)
		return sendRetryable, retryAfter

	case resp.StatusCode >= 500:
		level.Warn(s.logger).Log("msg", "Loki server error", "status", resp.StatusCode)
		return sendRetryable, 0

	default:
		level.Error(s.logger).Log("msg", "Loki rejected request", "status", resp.StatusCode)
		return sendPermanent, 0
	}
}

// buildPushRequest groups entries by their label set into Loki push streams.
func (s *Sender) buildPushRequest(entries []Entry) lokiPushRequest {
	// Group entries by label set key.
	type streamGroup struct {
		labels map[string]string
		values [][]string
	}
	groups := make(map[string]*streamGroup)

	for _, e := range entries {
		key := labelsKey(e.Labels)
		g, ok := groups[key]
		if !ok {
			g = &streamGroup{labels: e.Labels}
			groups[key] = g
		}
		g.values = append(g.values, []string{
			strconv.FormatInt(e.Ts, 10),
			e.Line,
		})
	}

	streams := make([]lokiStream, 0, len(groups))
	for _, g := range groups {
		streams = append(streams, lokiStream{
			Stream: g.labels,
			Values: g.values,
		})
	}
	return lokiPushRequest{Streams: streams}
}

// labelsKey produces a stable string key for a label map, used for grouping.
func labelsKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
		b.WriteByte(',')
	}
	return b.String()
}

// --- Cursor persistence ---

func (s *Sender) loadCursor() cursor {
	path := filepath.Join(s.wal.Dir(), cursorFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return cursor{} // Fresh start.
	}
	var c cursor
	if err := json.Unmarshal(data, &c); err != nil {
		level.Warn(s.logger).Log("msg", "corrupt cursor file, starting fresh", "err", err)
		return cursor{}
	}
	level.Info(s.logger).Log("msg", "loaded cursor", "segment", c.Segment, "offset", c.Offset)
	return c
}

func (s *Sender) saveCursor(c cursor) {
	data, err := json.Marshal(c)
	if err != nil {
		level.Error(s.logger).Log("msg", "marshaling cursor", "err", err)
		return
	}
	tmpPath := filepath.Join(s.wal.Dir(), cursorTmpFile)
	finalPath := filepath.Join(s.wal.Dir(), cursorFile)
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		level.Error(s.logger).Log("msg", "writing cursor tmp", "err", err)
		return
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		level.Error(s.logger).Log("msg", "renaming cursor", "err", err)
	}
}

// --- Retry-After parsing ---

func parseRetryAfter(val string) time.Duration {
	if val == "" {
		return 0
	}
	// Try seconds first.
	if secs, err := strconv.Atoi(val); err == nil {
		return time.Duration(secs) * time.Second
	}
	// Try HTTP-date.
	if t, err := http.ParseTime(val); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

// --- Exponential backoff with jitter ---

type backoff struct {
	min, max time.Duration
	attempt  int
}

func newBackoff(min, max time.Duration) *backoff {
	return &backoff{min: min, max: max}
}

func (b *backoff) next() time.Duration {
	dur := time.Duration(float64(b.min) * math.Pow(2, float64(b.attempt)))
	if dur > b.max {
		dur = b.max
	}
	b.attempt++
	// Jitter: 50-100% of calculated duration.
	jitter := time.Duration(rand.Int63n(int64(dur)/2 + 1))
	return dur/2 + jitter
}

func (b *backoff) reset() {
	b.attempt = 0
}

// --- Context-aware sleep ---

func (s *Sender) sleep(d time.Duration) {
	select {
	case <-time.After(d):
	case <-s.ctx.Done():
	}
}
