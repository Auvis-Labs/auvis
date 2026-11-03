package source

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Auvis-Labs/auvis/pipeline"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	kafkaConsumerLag = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "auvis_kafka_consumer_lag",
			Help: "Current Kafka consumer lag in messages",
		},
		[]string{"topic", "partition", "group_id"},
	)
)

// PrometheusReporter implements pipeline.MetricsReporter for Prometheus.
type PrometheusReporter struct {
	server *http.Server
	mu     sync.Mutex
}

// NewPrometheusReporter creates and starts a Prometheus HTTP metrics server on the given port.
func NewPrometheusReporter(port string) *PrometheusReporter {
	r := &PrometheusReporter{}
	if port != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())

		r.server = &http.Server{
			Addr:    ":" + port,
			Handler: mux,
		}

		go func() {
			slog.Info("Starting Prometheus metrics server", "port", port)
			if err := r.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("Prometheus metrics server failed", "error", err)
			}
		}()
	}
	return r
}

// ReportLag updates the lag gauge metric.
func (p *PrometheusReporter) ReportLag(topic, partition, groupID string, lag int64) {
	kafkaConsumerLag.WithLabelValues(topic, partition, groupID).Set(float64(lag))
}

// Close shuts down the metrics HTTP server.
func (p *PrometheusReporter) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := p.server.Shutdown(ctx)
	p.server = nil
	slog.Info("Prometheus metrics server stopped")
	return err
}

// Ensure PrometheusReporter implements pipeline.MetricsReporter
var _ pipeline.MetricsReporter = (*PrometheusReporter)(nil)
