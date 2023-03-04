package playback

import (
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/icza/mjpeg"
	"io"
	"key-logger/pkg/loki"
	"key-logger/pkg/s3"
	"net/http"
	"strings"
	"time"
)

type playback struct {
	logger log.Logger
	s3     *s3.S3
	loki   *loki.Loki
}

type image struct {
	bucket string
	object string
}

func New(l log.Logger, s3 *s3.S3, loki *loki.Loki) *playback {
	return &playback{
		logger: l,
		s3:     s3,
		loki:   loki,
	}
}

func (p *playback) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	q := "{job=\"screencap\",thumbnail=\"\"} |= \"Explore\" | logfmt | line_format \"{{.loc}}\""

	resp := p.loki.Query(q, "1675573200000000000", "1677992399000000000", "5000", "FORWARD")
	if resp == nil {
		level.Error(p.logger).Log("msg", "loki query returned no results")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	total := len(resp)
	level.Info(p.logger).Log("msg", "loki query completed", "images", total)

	images := make([]image, 0, 5000)
	for _, s := range resp {
		for _, e := range s.Entries {
			b, o := splitKey(e.Line)
			i := image{
				bucket: b,
				object: o,
			}
			images = append(images, i)
		}
	}

	aw, err := mjpeg.New(time.Now().Format(time.RFC3339)+".avi", 200, 100, 6)
	if err != nil {
		level.Error(p.logger).Log("msg", "failed to create image file writer", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	for c, i := range images {
		level.Info(p.logger).Log("msg", "downloading image", "count", c, "of", total, "image", i.object)
		o, err := p.s3.GetObject(r.Context(), i.bucket, i.object)
		if err != nil {
			o.Close()
			level.Error(p.logger).Log("msg", "error getting object from s3", "err", err)
			continue
		}

		b, err := io.ReadAll(o)
		o.Close()
		if err != nil {
			level.Error(p.logger).Log("msg", "failed to read image into byte slice", "err", err)
			continue
		}
		err = aw.AddFrame(b)
		if err != nil {
			level.Error(p.logger).Log("msg", "failed to add image to video", "err", err)
			continue
		}
	}
	err = aw.Close()
	if err != nil {
		level.Error(p.logger).Log("msg", "failed to finish video", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	return
}

func splitKey(key string) (string, string) {
	splits := strings.SplitN(key, "/", 3)
	return splits[1], splits[2]
}
