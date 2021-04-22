package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/johejo/sora_exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	listenAddr   string
	metricsPath  string
	timeout      time.Duration
	soraURL      string
	printVersion bool

	version string
	commit  string
	date    string
)

func init() {
	flag.StringVar(&listenAddr, "listen-addr", ":9199", "Address to listen for telemetry.")
	flag.StringVar(&metricsPath, "metrics-path", "/metrics", "Path under which to expose metrics.")
	flag.DurationVar(&timeout, "timeout", 5*time.Second, "Timeout for scraping to sora.")
	flag.StringVar(&soraURL, "sora-url", "http://127.0.0.1:3000", "URL for sora stats endpoint.")
	flag.BoolVar(&printVersion, "version", false, "Print version.")
}

func main() {
	lg := log.NewJSONLogger(os.Stderr)
	flag.Parse()
	if printVersion {
		if version == "" {
			version = "devel"
		}
		if commit == "" {
			commit = "HEAD"
		}
		if date == "" {
			date = time.Now().UTC().Format(time.RFC3339)
		}
		b, err := json.Marshal(struct {
			Version string `json:"version"`
			Commit  string `json:"commit"`
			Date    string `json:"date"`
		}{
			Version: version,
			Commit:  commit,
			Date:    date,
		})
		if err != nil {
			level.Error(lg).Log("err", err)
			os.Exit(1)
		}
		fmt.Println(string(b))
		os.Exit(0)
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		prometheus.NewBuildInfoCollector(),
		prometheus.NewGoCollector(),
		collector.New(collector.WithLogger(lg), collector.WithTimeout(timeout), collector.WithSoraURL(soraURL)),
	)

	mux := http.NewServeMux()
	mux.Handle(metricsPath, promhttp.InstrumentMetricHandler(reg, promhttp.HandlerFor(reg, promhttp.HandlerOpts{})))

	srv := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	finish := make(chan struct{})

	go func() {
		defer close(finish)
		<-ctx.Done()
		shutDownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutDownCtx); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return
			}
			level.Error(lg).Log("msg", "failed to shutdown", "err", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			level.Error(lg).Log("msg", "failed to listen", "err", err)
		}
		os.Exit(1)
	}
	<-finish
	os.Exit(0)
}
