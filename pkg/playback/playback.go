package playback

import (
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"

	"key-logger/pkg/loki"
	"key-logger/pkg/s3"
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

func (p *playback) Process() {

	//q := "{job=\"screencap\",thumbnail=\"false\"} | logfmt | line_format \"{{.loc}}\""
	//q := `{job="screencap",thumbnail="false"} |~ "GrafanaMozillaFirefox|GrafanaGoogleChrome" | logfmt | line_format "{{.loc}}"`
	q := `{job="screencap",thumbnail="false"} |= "_hackathon202303sendlesssellmore" |="Ubuntu" | logfmt | line_format "{{.loc}}"`

	from, err := time.Parse(time.RFC3339, "2023-03-12T00:00:00-05:00")
	if err != nil {
		panic(err)
	}

	to, err := time.Parse(time.RFC3339, "2023-03-18T00:00:00-05:00")
	if err != nil {
		panic(err)
	}

	resp := p.loki.Query(q, from, to, 1000)
	if resp == nil {
		level.Error(p.logger).Log("msg", "loki query returned no results")
		//w.WriteHeader(http.StatusNoContent)
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

	//aw, err := mjpeg.New(time.Now().Format(time.RFC3339)+".avi", 200, 100, 6)
	if err != nil {
		level.Error(p.logger).Log("msg", "failed to create image file writer", "err", err)
		//w.WriteHeader(http.StatusInternalServerError)
		return
	}
	for c, i := range images {
		level.Info(p.logger).Log("msg", "downloading image", "count", c, "of", total, "image", i.object)
		o, err := p.s3.GetObject(context.Background(), i.bucket, i.object)
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
		os.Stdout.Write(b)
		//err = aw.AddFrame(b)
		//if err != nil {
		//	level.Error(p.logger).Log("msg", "failed to add image to video", "err", err)
		//	continue
		//}
	}
	//err = aw.Close()
	//if err != nil {
	//	level.Error(p.logger).Log("msg", "failed to finish video", "err", err)
	//	//w.WriteHeader(http.StatusInternalServerError)
	//	return
	//}

	//w.WriteHeader(http.StatusOK)
	return
}

func splitKey(key string) (string, string) {
	splits := strings.SplitN(key, "/", 3)
	return splits[1], splits[2]
}
