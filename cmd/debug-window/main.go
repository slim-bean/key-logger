package main

import (
	"fmt"
	"os"
	"time"

	gklog "github.com/go-kit/kit/log"

	"key-logger/pkg/activewindow"
)

func main() {
	fmt.Println("=== Active Window Debug Tool ===")
	fmt.Println("Polling every 2 seconds. Focus different windows to test.")
	fmt.Println("Press Ctrl+C to exit.")
	fmt.Println()

	logger := gklog.NewLogfmtLogger(gklog.NewSyncWriter(os.Stdout))

	tracker := activewindow.New(logger)
	if err := tracker.Start(); err != nil {
		fmt.Printf("ERROR starting tracker: %v\n", err)
		return
	}

	// Give the tracker a moment to poll
	time.Sleep(1 * time.Second)

	for i := 0; i < 15; i++ {
		info := tracker.GetActiveWindow()
		idle := tracker.GetIdleTime()
		fmt.Printf("[%s] window=%q process=%q bounds=%v idle=%v\n",
			time.Now().Format("15:04:05"),
			info.WindowName, info.Process, info.Bounds, idle)
		time.Sleep(2 * time.Second)
	}
}
