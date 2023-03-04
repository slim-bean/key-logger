package main

import (
	"key-logger/pkg/loki"
	"key-logger/pkg/playback"
	"key-logger/pkg/s3"
	"net/http"
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

func main() {
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger = level.NewFilter(logger, level.AllowInfo())
	logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.Caller(3))

	l := loki.New(logger)
	end := os.Getenv("ENDPOINT")
	ak := os.Getenv("ACCESS_KEY")
	sk := os.Getenv("SECRET_KEY")
	s := s3.New(end, ak, sk, "")

	p := playback.New(logger, s, l)

	http.HandleFunc("/makevid", p.ServeHTTP)

	if err := http.ListenAndServe(":9999", nil); err != nil {

	}

}
