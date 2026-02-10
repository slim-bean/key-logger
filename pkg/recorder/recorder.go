package recorder

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-logfmt/logfmt"
	"github.com/grafana/loki-client-go/loki"
	prom "github.com/prometheus/common/model"
	"golang.org/x/image/draw"

	"key-logger/pkg/activewindow"
	"key-logger/pkg/keylogger"
	"key-logger/pkg/model"
	"key-logger/pkg/s3"
	"key-logger/pkg/screencap"
)

var (
	eightyFivePercent = jpeg.Options{Quality: 85}
)

// Config holds tuning parameters for the recorder's behavior.
type Config struct {
	// ScreencapInterval is how often to capture screenshots and log window info.
	ScreencapInterval time.Duration

	// IdleTimeout is how long the system must be idle before capture pauses.
	IdleTimeout time.Duration

	// OutputMode controls where text log events are sent: "stdout" (default) or "loki".
	OutputMode string

	// Labels are key=value pairs attached to Loki log streams.
	// Required when OutputMode is "loki".
	Labels map[string]string

	// Filters are compiled regexes. Any substring matching a filter is removed
	// from string values before output (both stdout and Loki).
	Filters []*regexp.Regexp
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		ScreencapInterval: 5 * time.Second,
		IdleTimeout:       5 * time.Minute,
		OutputMode:        "stdout",
	}
}

// Recorder orchestrates key logging, active window tracking, and screen capture.
// It ties together the platform-specific implementations with the shared
// logging, upload, and processing logic.
type Recorder struct {
	logger       log.Logger
	config       Config
	keyLogger    keylogger.KeyLogger
	winTracker   activewindow.Tracker
	capturer     screencap.Capturer
	s3           *s3.S3
	lokiClient   *loki.Client
	cleanRegex   *regexp.Regexp
	outputLabels prom.LabelSet // labels for Loki output mode
}

// New creates a new Recorder. Any of the subsystems (keyLogger, winTracker,
// capturer, s3, lokiClient) may be nil to disable that functionality.
func New(
	logger log.Logger,
	cfg Config,
	kl keylogger.KeyLogger,
	wt activewindow.Tracker,
	cap screencap.Capturer,
	s3 *s3.S3,
	lokiClient *loki.Client,
) *Recorder {
	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		panic(err)
	}

	outputLabels := make(prom.LabelSet)
	for k, v := range cfg.Labels {
		outputLabels[prom.LabelName(k)] = prom.LabelValue(v)
	}

	return &Recorder{
		logger:       logger,
		config:       cfg,
		keyLogger:    kl,
		winTracker:   wt,
		capturer:     cap,
		s3:           s3,
		lokiClient:   lokiClient,
		cleanRegex:   reg,
		outputLabels: outputLabels,
	}
}

// Start initializes and starts all enabled subsystems.
func (r *Recorder) Start() error {
	if r.capturer != nil {
		if err := r.capturer.Init(); err != nil {
			level.Warn(r.logger).Log("msg", "screencap init failed", "err", err)
		}
	}

	if r.winTracker != nil {
		if err := r.winTracker.Start(); err != nil {
			return fmt.Errorf("starting window tracker: %w", err)
		}
		go r.captureLoop()
	}

	if r.keyLogger != nil {
		if err := r.keyLogger.Start(); err != nil {
			return fmt.Errorf("starting key logger: %w", err)
		}
		go r.processKeyEvents()
	}

	return nil
}

// applyFilters removes any text matching the configured filter regexes from
// all string values in the keyvals slice. Non-string values are left untouched.
func (r *Recorder) applyFilters(keyvals []interface{}) []interface{} {
	if len(r.config.Filters) == 0 {
		return keyvals
	}
	result := make([]interface{}, len(keyvals))
	copy(result, keyvals)
	for i := 1; i < len(result); i += 2 {
		if s, ok := result[i].(string); ok {
			for _, f := range r.config.Filters {
				s = f.ReplaceAllString(s, "")
			}
			result[i] = s
		}
	}
	return result
}

// log writes a log entry to the configured output destination (stdout or Loki),
// applying any configured filters first. The job parameter sets the "job" label
// on Loki streams (e.g. "keylogger", "window").
func (r *Recorder) log(ts time.Time, job string, keyvals ...interface{}) {
	filtered := r.applyFilters(keyvals)

	if r.config.OutputMode == "loki" && r.lokiClient != nil {
		labels := r.outputLabels.Clone()
		labels["job"] = prom.LabelValue(job)

		var buf bytes.Buffer
		enc := logfmt.NewEncoder(&buf)
		if err := enc.EncodeKeyvals(filtered...); err != nil {
			level.Error(r.logger).Log("msg", "failed to encode log line", "err", err)
			return
		}
		if err := r.lokiClient.Handle(labels, ts, buf.String()); err != nil {
			level.Error(r.logger).Log("msg", "failed to send to Loki", "err", err)
		}
	} else {
		level.Info(r.logger).Log(filtered...)
	}
}

// processKeyEvents reads from the key logger event channel, accumulates text,
// and logs a key-event entry each time Enter is pressed.
func (r *Recorder) processKeyEvents() {
	var currentText string
	var backCounter, charCounter int
	var startTime time.Time

	for ev := range r.keyLogger.Events() {
		if ev.IsReturn {
			words := len(strings.Split(currentText, " "))
			dur := time.Since(startTime)
			wpm := float64(words) / dur.Minutes()
			charCounter++ // Enter counts as a key

			var winName, proc string
			if r.winTracker != nil {
				info := r.winTracker.GetActiveWindow()
				winName = info.WindowName
				proc = info.Process
			}

			now := time.Now()
			r.log(now, "keylogger",
				"ts", now,
				"type", "key-event",
				"window", winName,
				"process", proc,
				"words", words,
				"backspace_count", backCounter,
				"duration", dur,
				"wpm", wpm,
				"text", currentText,
				"chars", charCounter,
			)
			currentText = ""
			backCounter = 0
			charCounter = 0
		} else if ev.IsBack {
			if len(currentText) == 0 {
				startTime = time.Now()
			}
			sz := len(currentText)
			if sz > 0 {
				currentText = currentText[:sz-1]
			}
			backCounter++
			charCounter++
		} else if ev.Name != "" {
			if len(currentText) == 0 {
				startTime = time.Now()
			}
			currentText += ev.Name
			charCounter++
		}
	}
}

// captureLoop periodically captures the active window screenshot, uploads it
// to S3, sends a thumbnail to Loki, and logs the active window info.
func (r *Recorder) captureLoop() {
	sendInterval := r.config.ScreencapInterval
	idleTimeout := r.config.IdleTimeout
	jpegBuff := &bytes.Buffer{}
	lineBuff := &bytes.Buffer{}
	logfmtEncoder := logfmt.NewEncoder(lineBuff)

	host, err := os.Hostname()
	if err != nil {
		level.Error(r.logger).Log("msg", "could not get hostname", "err", err)
	}
	thumbnailLabels := prom.LabelSet{
		"host":      prom.LabelValue(host),
		"job":       "screencap",
		"thumbnail": "true",
	}

	for {
		start := time.Now()
		idle := r.winTracker.GetIdleTime()

		if idle < idleTimeout {
			info := r.winTracker.GetActiveWindow()

			if r.s3 != nil && r.capturer != nil && info.Bounds.Dx() > 0 && info.Bounds.Dy() > 0 {
				img, err := r.capturer.CaptureRect(info.Bounds)
				if err != nil {
					level.Error(r.logger).Log("msg", "screenshot capture failed", "err", err)
				} else {
					now := time.Now()
					loc := fmt.Sprintf("caps/%d/%d/%d/%d_%s.jpg",
						now.Year(), now.Month(), now.Day(), now.Unix(),
						r.cleanRegex.ReplaceAllString(info.WindowName, ""))
					im := &model.Image{Location: loc, Image: img}
					r.s3.Send(im)

					// Create and send thumbnail.
					dst := image.NewRGBA(image.Rect(0, 0, 640, 360))
					draw.CatmullRom.Scale(dst, dst.Rect, img, img.Bounds(), draw.Over, nil)
					jpegBuff.Reset()
					jpeg.Encode(jpegBuff, dst, &eightyFivePercent)
					b64Str := base64.StdEncoding.EncodeToString(jpegBuff.Bytes())
					imageLoc := fmt.Sprintf("%s/%s/%s", r.s3.GetEndpoint(), r.s3.GetBucket(), loc)

						ts := time.Now()
					// Send thumbnail to Loki (uses dedicated thumbnail labels).
					if r.lokiClient != nil {
						lineBuff.Reset()
						logfmtEncoder.Reset()
						logfmtEncoder.EncodeKeyvals("ts", ts, "type", "screen-cap", "loc", imageLoc, "thumb", b64Str)
						r.lokiClient.Handle(thumbnailLabels, ts, lineBuff.String())
					}
					r.log(ts, "screencap", "ts", ts, "type", "screen-cap", "loc", imageLoc)
				}
			}

			now := time.Now()
		r.log(now, "window", "ts", now, "type", "active-window",
			"window", info.WindowName, "process", info.Process)
		}

		// Adjust sleep to maintain consistent send interval.
		executionTime := time.Since(start)
		if executionTime > sendInterval {
			executionTime = sendInterval
		}
		time.Sleep(sendInterval - executionTime)
	}
}
