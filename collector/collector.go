package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

type Collector struct {
	*config

	totalConnectionCreated     *prometheus.Desc
	totalConnectionUpdated     *prometheus.Desc
	totalConnectionDestroyed   *prometheus.Desc
	totalSuccessfulConnections *prometheus.Desc
	totalOngoingConnections    *prometheus.Desc
	totalFailedConnections     *prometheus.Desc
	totalDurationSec           *prometheus.Desc
	totalTurnUdpConnections    *prometheus.Desc
	totalTurnTcpConnections    *prometheus.Desc
	averageDurationSec         *prometheus.Desc
	averageSetupTimeSec        *prometheus.Desc
}

type config struct {
	httpClient HTTPClient
	logger     log.Logger
	timeout    time.Duration
	soraURL    string
}

type Option func(cfg *config)

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

func WithHTTPClient(cli HTTPClient) Option {
	return func(cfg *config) {
		cfg.httpClient = cli
	}
}

func WithLogger(logger log.Logger) Option {
	return func(cfg *config) {
		cfg.logger = logger
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(cfg *config) {
		cfg.timeout = timeout
	}
}

func WithSoraURL(url string) Option {
	return func(cfg *config) {
		cfg.soraURL = url
	}
}

func New(opts ...Option) *Collector {
	cfg := new(config)
	for _, opt := range append(defaults(), opts...) {
		opt(cfg)
	}
	return &Collector{
		config:                     cfg,
		totalConnectionCreated:     newDesc("connections_created_total", "The total number of connections created."),
		totalConnectionUpdated:     newDesc("connections_updated_total", "The total number of connections updated."),
		totalConnectionDestroyed:   newDesc("connections_destroyed_total", "The total number of connections destryed."),
		totalSuccessfulConnections: newDesc("successfull_connections_total", "The total number of successfull connections."),
		totalOngoingConnections:    newDesc("ongoing_connections_total", "The total number of ongoing connections."),
		totalFailedConnections:     newDesc("failed_connections_total", "The total number of failed connections."),
		totalDurationSec:           newDesc("duration_seconds_total", "The total duration of connections."),
		totalTurnUdpConnections:    newDesc("turn_udp_connections_total", "The total number of connections with TURN-UDP."),
		totalTurnTcpConnections:    newDesc("turn_tcp_connections_total", "The total number of connections with TURN-TCP."),
		averageDurationSec:         newDesc("average_duration_seconds", "The average connection duration in seconds."),
		averageSetupTimeSec:        newDesc("average_setup_time_seconds", "The average setup time in seconds."),
	}
}

func newDesc(name, help string) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName("sora", "exporter", name), help, nil, nil)
}

func defaults() []Option {
	return []Option{
		WithHTTPClient(http.DefaultClient),
		WithLogger(log.NewJSONLogger(os.Stderr)),
		WithTimeout(1 * time.Second),
		WithSoraURL("http://127.0.0.1:3000/"),
	}
}

var _ prometheus.Collector = (*Collector)(nil)

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.soraURL, nil)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to create request to sora", "err", err)
		return
	}
	req.Header.Set("x-sora-target", "Sora_20171010.GetStatsReport")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to request to sora", "err", err)
		return
	}
	defer resp.Body.Close()

	var report statsReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		level.Error(c.logger).Log("msg", "failed to decode response body from sora", "err", err)
		return
	}

	ch <- newCounter(c.totalConnectionCreated, float64(report.TotalConnectionCreated))
	ch <- newCounter(c.totalConnectionUpdated, float64(report.TotalConnectionUpdated))
	ch <- newCounter(c.totalConnectionDestroyed, float64(report.TotalConnectionDestroyed))
	ch <- newCounter(c.totalSuccessfulConnections, float64(report.TotalSuccessfulConnections))
	ch <- newCounter(c.totalOngoingConnections, float64(report.TotalOngoingConnections))
	ch <- newCounter(c.totalFailedConnections, float64(report.TotalFailedConnections))
	ch <- newCounter(c.totalDurationSec, float64(report.TotalDurationSec))
	ch <- newCounter(c.totalTurnUdpConnections, float64(report.TotalTurnUdpConnections))
	ch <- newCounter(c.totalTurnTcpConnections, float64(report.TotalTurnTcpConnections))
	ch <- newGauge(c.averageDurationSec, float64(report.AverageDurationSec))
	ch <- newGauge(c.averageSetupTimeSec, float64(report.AverageSetupTimeMsec/1000))
}

func newGauge(d *prometheus.Desc, v float64) prometheus.Metric {
	return prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v)
}

func newCounter(d *prometheus.Desc, v float64) prometheus.Metric {
	return prometheus.MustNewConstMetric(d, prometheus.CounterValue, v)
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.totalConnectionCreated
	ch <- c.totalConnectionUpdated
	ch <- c.totalConnectionUpdated
	ch <- c.totalSuccessfulConnections
	ch <- c.totalOngoingConnections
	ch <- c.totalFailedConnections
	ch <- c.totalDurationSec
	ch <- c.totalTurnUdpConnections
	ch <- c.totalTurnTcpConnections
	ch <- c.averageDurationSec
	ch <- c.averageSetupTimeSec
}

type statsReport struct {
	TotalConnectionCreated     int64 `json:"total_connection_created"`
	TotalConnectionUpdated     int64 `json:"total_connection_updated"`
	TotalConnectionDestroyed   int64 `json:"total_connection_destroyed"`
	TotalSuccessfulConnections int64 `json:"total_successful_connections"`
	TotalOngoingConnections    int64 `json:"total_ongoing_connections"`
	TotalFailedConnections     int64 `json:"total_failed_connections"`
	TotalDurationSec           int64 `json:"total_duration_sec"`
	TotalTurnUdpConnections    int64 `json:"total_turn_udp_connections"`
	TotalTurnTcpConnections    int64 `json:"total_turn_tcp_connections"`
	AverageDurationSec         int64 `json:"average_duration_sec"`
	AverageSetupTimeMsec       int64 `json:"average_setup_time_msec"`
}
