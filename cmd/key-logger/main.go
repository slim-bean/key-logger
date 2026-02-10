package main

import (
	"flag"
	"os"
	"time"

	gklog "github.com/go-kit/kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/loki-client-go/loki"

	"key-logger/pkg/activewindow"
	"key-logger/pkg/keylogger"
	"key-logger/pkg/recorder"
	s32 "key-logger/pkg/s3"
	"key-logger/pkg/screencap"
)

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

	cfg := loki.Config{}
	// Sets defaults as well as anything from the command line.
	cfg.RegisterFlags(flag.CommandLine)

	flag.Parse()

	lokiClient, err := loki.NewWithLogger(cfg, logger)
	if err != nil {
		level.Error(logger).Log("msg", "error building Loki client", "err", err)
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
	}

	rec := recorder.New(logger, recCfg, kl, wt, cap, s3, lokiClient)
	if err := rec.Start(); err != nil {
		level.Error(logger).Log("msg", "error starting recorder", "err", err)
		os.Exit(1)
	}

	// Block forever.
	select {}
}
