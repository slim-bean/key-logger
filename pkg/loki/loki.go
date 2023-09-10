package loki

import (
	"flag"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/loki-client-go/loki"
	"github.com/grafana/loki-client-go/pkg/urlutil"
	"key-logger/pkg/loki/loghttp"

	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Loki struct {
	logger     log.Logger
	lokiClient *loki.Client
	lokiUrl    string
}

func New(l log.Logger) *Loki {

	var lokiClient *loki.Client

	lokiURL := os.Getenv("LOKI_URL_2")
	if lokiURL == "" {
		level.Error(l).Log("msg", "LOKI_URL must be set to enable Loki integration, it was not set, plugin is disabled.")
	} else {
		cfg := loki.Config{}
		// Sets defaults as well as anything from the command line
		cfg.RegisterFlags(flag.NewFlagSet("empty", flag.PanicOnError))
		u := urlutil.URLValue{}
		err := u.Set(lokiURL + "/loki/api/v1/push")
		//err := u.Set("http://localhost:3100/loki/api/v1/push")
		if err != nil {
			level.Error(l).Log("msg", "error parsing LOKI_URL", "err", err)
		} else {
			cfg.URL = u
			lokiClient, err = loki.NewWithLogger(cfg, l)
			if err != nil {
				level.Error(l).Log("msg", "error building Loki client", "err", err)
			}
		}
	}

	lk := &Loki{
		logger:     l,
		lokiClient: lokiClient,
		lokiUrl:    lokiURL,
	}
	return lk
}

func (l *Loki) Query(query string, start, end time.Time, limit int64) loghttp.Streams {
	// split into smaller time windows
	day := time.Hour * 24
	windows := calcWindows(start.UTC().UnixNano(), end.UTC().UnixNano(), day.Nanoseconds())

	res := make(loghttp.Streams, 0, limit)

	// loop on each query until we hit end time or get no additional results
	for _, w := range windows {
		f := w.from
		for {
			level.Info(l.logger).Log("msg", "querying Loki", "from", f, "to", w.to)
			ir := l.singleQuery(query, f, w.to, limit)
			if ir == nil || len(ir) == 0 {
				level.Info(l.logger).Log("msg", "loki query returned no results, moving to next window")
				break
			}
			level.Info(l.logger).Log("msg", "loki query returned", "streams", len(ir))
			res = append(res, ir...)
			lastTs := int64(0)
			for _, s := range ir {
				for _, e := range s.Entries {
					if e.Timestamp.UTC().UnixNano() > lastTs {
						lastTs = e.Timestamp.UTC().UnixNano()
					}
				}
			}
			f = lastTs + 1
			if f >= w.to {
				level.Info(l.logger).Log("msg", "timestamp for next batch is outside window, moving to next window")
				break
			}
		}
	}
	return res
}

func (l *Loki) singleQuery(query string, start, end int64, limit int64) loghttp.Streams {
	params := url.Values{}
	params.Add("query", query)
	params.Add("limit", strconv.FormatInt(limit, 10))
	params.Add("start", strconv.FormatInt(start, 10))
	params.Add("end", strconv.FormatInt(end, 10))
	params.Add("direction", "FORWARD")
	rs := l.lokiUrl + "/loki/api/v1/query_range?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, rs, nil)
	if err != nil {
		level.Error(l.logger).Log("msg", "error building request", "err", err)
		return nil
	}
	//fmt.Println(req.URL.String())
	resp, err := http.DefaultClient.Do(req)
	r := loghttp.QueryResponse{}
	rb, err := io.ReadAll(resp.Body)
	if err != nil {
		level.Error(l.logger).Log("msg", "error reading response body", "err", err)
		return nil
	}
	err = r.UnmarshalJSON(rb)
	if err != nil {
		level.Error(l.logger).Log("msg", "error unmarshaling response", "err", err)
	}

	if r.Status != loghttp.QueryStatusSuccess {
		level.Error(l.logger).Log("msg", "query returned error")
		return nil
	}

	if r.Data.ResultType == loghttp.ResultTypeStream {
		streams := r.Data.Result.(loghttp.Streams)
		return streams
	}
	return nil
}

type window struct {
	from   int64
	to     int64
	number int
}

func calcWindows(from, to int64, shardBy int64) []window {
	// Calculate the sync ranges
	windows := []window{}
	// diff := to - from
	// shards := diff / shardBy
	currentFrom := from
	// currentTo := from
	currentTo := from + shardBy
	number := 0
	for currentFrom < to && currentTo <= to {
		s := window{
			from:   currentFrom,
			to:     currentTo,
			number: number,
		}
		windows = append(windows, s)
		number++

		currentFrom = currentTo + 1
		currentTo = currentTo + shardBy

		if currentTo > to {
			currentTo = to
		}
	}
	return windows
}
