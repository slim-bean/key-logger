package loki

import (
	"flag"
	"fmt"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/loki-client-go/loki"
	"github.com/grafana/loki-client-go/pkg/urlutil"
	"io"
	"key-logger/pkg/loki/loghttp"
	"net/http"
	"net/url"
	"os"
)

type Loki struct {
	logger     log.Logger
	lokiClient *loki.Client
	lokiUrl    string
}

func New(l log.Logger) *Loki {

	var lokiClient *loki.Client

	lokiURL := os.Getenv("LOKI_URL")
	if lokiURL == "" {
		level.Error(l).Log("msg", "LOKI_URL must be set to enable Loki integration, it was not set, plugin is disabled.")
	} else {
		cfg := loki.Config{}
		// Sets defaults as well as anything from the command line
		cfg.RegisterFlags(flag.NewFlagSet("empty", flag.PanicOnError))
		u := urlutil.URLValue{}
		err := u.Set(lokiURL + "/loki/api/v1/push")
		if err != nil {
			level.Error(l).Log("msg", "error parsing LOKI_URL", "err", err)
		} else {
			cfg.URL = u
			cfg.Client.TLSConfig.InsecureSkipVerify = true
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

func (l *Loki) Query(query, start, end, limit, direction string) loghttp.Streams {
	params := url.Values{}
	params.Add("query", query)
	params.Add("limit", limit)
	params.Add("start", start)
	params.Add("end", end)
	params.Add("direction", direction)
	rs := l.lokiUrl + "/loki/api/v1/query_range?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, rs, nil)
	if err != nil {
		level.Error(l.logger).Log("msg", "error building request", "err", err)
		return nil
	}
	fmt.Println(req.URL.String())
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
