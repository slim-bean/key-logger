package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	gklog "github.com/go-kit/kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/loki-client-go/loki"

	"key-logger/pkg/activewindow"
	"key-logger/pkg/buffer"
	"key-logger/pkg/keylogger"
	"key-logger/pkg/recorder"
	s32 "key-logger/pkg/s3"
	"key-logger/pkg/screencap"
)

// stringSlice implements flag.Value for repeatable string flags.
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ", ") }
func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {

	logger := gklog.NewLogfmtLogger(gklog.NewSyncWriter(os.Stdout))

	// S3 flags.
	endpoint := flag.String("s3-endpoint", "", "s3 url (without bucket)")
	bucketName := flag.String("bucket", "", "s3 bucket name for image caps")
	accessKeyId := flag.String("accessKey", "", "s3 access key Id")
	secretKey := flag.String("secretKey", "", "s3 secret key")

	// Feature toggle flags.
	enableKeylogger := flag.Bool("enable-keylogger", true, "enable keystroke logging")
	enableWindowTracker := flag.Bool("enable-window-tracker", true, "enable active window tracking")
	enableScreencap := flag.Bool("enable-screencap", true, "enable screenshot capture")

	// Tuning flags.
	screencapInterval := flag.Duration("screencap-interval", 5*time.Second, "screenshot capture interval")
	idleTimeout := flag.Duration("idle-timeout", 5*time.Minute, "idle time before pausing capture")

	// Output flags.
	outputMode := flag.String("output", "stdout", "output destination: stdout or loki")
	var labels stringSlice
	flag.Var(&labels, "label", "label in key=value format for Loki output (repeatable, required with --output=loki)")
	var filters stringSlice
	flag.Var(&filters, "filter", "regex filter to remove matching text from output (repeatable)")

	// Buffer flags.
	homeDir, _ := os.UserHomeDir()
	defaultBufferDir := filepath.Join(homeDir, ".key-logger", "buffer")
	bufferDir := flag.String("buffer-dir", defaultBufferDir, "directory for WAL buffer files")
	bufferMaxMB := flag.Int("buffer-max-mb", 100, "maximum buffer size in megabytes")

	lokiCfg := loki.Config{}
	// Sets defaults as well as anything from the command line.
	lokiCfg.RegisterFlags(flag.CommandLine)

	flag.Parse()

	// Normalize Loki URL: append the push path if user provided just the base.
	if lokiCfg.URL.URL != nil {
		p := strings.TrimRight(lokiCfg.URL.Path, "/")
		if !strings.HasSuffix(p, "/loki/api/v1/push") {
			lokiCfg.URL.Path = p + "/loki/api/v1/push"
		}
	}

	// Validate output mode.
	if *outputMode != "stdout" && *outputMode != "loki" {
		fmt.Fprintf(os.Stderr, "invalid --output value %q: must be stdout or loki\n", *outputMode)
		os.Exit(1)
	}

	// Parse labels.
	labelMap := make(map[string]string)
	for _, l := range labels {
		parts := strings.SplitN(l, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			fmt.Fprintf(os.Stderr, "invalid --label %q: expected key=value format\n", l)
			os.Exit(1)
		}
		labelMap[parts[0]] = parts[1]
	}

	if *outputMode == "loki" && len(labelMap) == 0 {
		fmt.Fprintln(os.Stderr, "at least one --label is required when --output=loki")
		os.Exit(1)
	}

	// Compile filter regexes.
	var compiledFilters []*regexp.Regexp
	for _, f := range filters {
		re, err := regexp.Compile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --filter regex %q: %v\n", f, err)
			os.Exit(1)
		}
		compiledFilters = append(compiledFilters, re)
	}

	// Build Loki client (for best-effort thumbnail uploads).
	lokiClient, err := loki.NewWithLogger(lokiCfg, logger)
	if err != nil {
		if *outputMode == "loki" && lokiCfg.URL.String() == "" {
			level.Error(logger).Log("msg", "--output=loki requires --client.url", "err", err)
			os.Exit(1)
		}
		level.Warn(logger).Log("msg", "Loki client not configured (thumbnails disabled)", "err", err)
		lokiClient = nil
	}

	// Create WAL and sender when output mode is loki.
	var wal *buffer.WAL
	var sender *buffer.Sender
	if *outputMode == "loki" {
		lokiURL := lokiCfg.URL.String()
		if lokiURL == "" {
			level.Error(logger).Log("msg", "--output=loki requires --client.url")
			os.Exit(1)
		}
		maxBytes := int64(*bufferMaxMB) * 1024 * 1024
		wal, err = buffer.NewWAL(*bufferDir, maxBytes, logger)
		if err != nil {
			level.Error(logger).Log("msg", "failed to create WAL", "err", err)
			os.Exit(1)
		}
		sender = buffer.NewSender(wal, lokiURL, lokiCfg.TenantID, logger)
		go sender.Run()
		level.Info(logger).Log("msg", "WAL buffer enabled",
			"dir", *bufferDir, "max_mb", *bufferMaxMB, "loki_url", lokiURL)
	}

	s3 := s32.New(*endpoint, *accessKeyId, *secretKey, *bucketName)

	// Conditionally create subsystems based on feature flags.
	var kl keylogger.KeyLogger
	if *enableKeylogger {
		kl = keylogger.New()
		level.Info(logger).Log("msg", "keylogger enabled")
	} else {
		level.Info(logger).Log("msg", "keylogger disabled")
	}

	var wt activewindow.Tracker
	if *enableWindowTracker {
		wt = activewindow.New(logger)
		level.Info(logger).Log("msg", "window tracker enabled")
	} else {
		level.Info(logger).Log("msg", "window tracker disabled")
	}

	var cap screencap.Capturer
	if *enableScreencap {
		if !*enableWindowTracker {
			level.Warn(logger).Log("msg", "screencap requires window tracker; disabling screencap")
		} else {
			cap = screencap.New()
			level.Info(logger).Log("msg", "screencap enabled",
				"interval", *screencapInterval, "idle-timeout", *idleTimeout)
		}
	} else {
		level.Info(logger).Log("msg", "screencap disabled")
	}

	recCfg := recorder.Config{
		ScreencapInterval: *screencapInterval,
		IdleTimeout:       *idleTimeout,
		OutputMode:        *outputMode,
		Labels:            labelMap,
		Filters:           compiledFilters,
	}

	rec := recorder.New(logger, recCfg, kl, wt, cap, s3, lokiClient, wal)
	if err := rec.Start(); err != nil {
		level.Error(logger).Log("msg", "error starting recorder", "err", err)
		os.Exit(1)
	}

	// Wait for termination signal for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	level.Info(logger).Log("msg", "received signal, shutting down", "signal", sig)

	if sender != nil {
		sender.Stop()
	}
	if wal != nil {
		wal.Close()
	}
}
