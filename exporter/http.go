package exporter

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/oliver006/redis_exporter/webank"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

var (
	limit = rate.Every(time.Second * 10)
 	limiter = rate.NewLimiter(limit, 3)
)

func (e *Exporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.mux.ServeHTTP(w, r)
}

func (e *Exporter) healthHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(`ok`))
}

func (e *Exporter) indexHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(`<html>
<head><title>WeRedis Exporter</title></head>
<body>
<h1>WeRedis Exporter</h1>
<p>CurrentClusterName: ` + 	*webank.CurrentClusterName + `</p>
</body>
</html>
`))
}

func (e *Exporter) scrapeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	err := limiter.Wait(ctx)
	if err != nil {
		http.Error(w, "request is too frequent", http.StatusBadRequest)
		return
	}

	target := r.URL.Query().Get("target")
	if target == "" {
		http.Error(w, "'target' parameter must be specified", http.StatusBadRequest)
		e.targetScrapeRequestErrors.Inc()
		return
	}

	if !strings.Contains(target, "://") {
		target = "redis://" + target
	}

	u, err := url.Parse(target)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid 'target' parameter, parse err: %ck ", err), http.StatusBadRequest)
		e.targetScrapeRequestErrors.Inc()
		return
	}

	// get rid of username/password info in "target" so users don't send them in plain text via http
	u.User = nil
	target = u.String()

	opts := e.options

	if ck := r.URL.Query().Get("check-keys"); ck != "" {
		opts.CheckKeys = ck
	}

	if csk := r.URL.Query().Get("check-single-keys"); csk != "" {
		opts.CheckSingleKeys = csk
	}

	if cs := r.URL.Query().Get("check-streams"); cs != "" {
		opts.CheckStreams = cs
	}

	if css := r.URL.Query().Get("check-single-streams"); css != "" {
		opts.CheckSingleStreams = css
	}

	if cntk := r.URL.Query().Get("count-keys"); cntk != "" {
		opts.CountKeys = cntk
	}

	registry := prometheus.NewRegistry()
	opts.Registry = registry

	_, err = NewRedisExporter(target, opts)
	if err != nil {
		http.Error(w, "NewRedisExporter() err: err", http.StatusBadRequest)
		e.targetScrapeRequestErrors.Inc()
		return
	}

	promhttp.HandlerFor(
		registry, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError},
	).ServeHTTP(w, r)
}

func (e *Exporter) assembleHandler(w http.ResponseWriter, r *http.Request) {
	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	err := limiter.Wait(ctx)
	if err != nil {
		http.Error(w, "request is too frequent", http.StatusBadRequest)
		return
	}

	params := r.URL.Query()
	name := params.Get("clusterName")

	if !params.Has("clusterName") || name == "" {
		*webank.CurrentClusterName = ""
		w.Write([]byte("# cluster name is nil"))
		return
	}

	info, err := webank.GetCurrentClusterInfo(name)
	if err != nil {
		w.Write([]byte("# " + err.Error()))
		return
	}

	registry := prometheus.NewRegistry()
	scrapedPartition := []string{}

	for _, n := range info.GetNodes() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("error %s at scrape node:%s\n", err, n.Host)
			}
		}()

		opts := e.options	
		opts.Registry = registry
		opts.Partition = n.PartitionNum
		opts.PartitionName = n.PartitionName
		opts.Host = n.Host

		e, err := NewRedisExporter(n.Host, opts)

		// 每个分区选一个节点, 来采集cluster nodes信息
		// 每次请求时所选的分区都不同
		if !Contains(scrapedPartition, n.PartitionNum) {
			scrapedPartition = append(scrapedPartition, n.PartitionNum)
			NewClusterExporter(e, opts)
		}

		if err != nil {
			http.Error(w, "NewRedisExporter() err: err", http.StatusBadRequest)
			e.targetScrapeRequestErrors.Inc()
			return
		}
	}

	promhttp.HandlerFor(
		registry, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError},
	).ServeHTTP(w, r)

}
