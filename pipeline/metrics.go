package pipeline

// MetricsReporter defines a generalized telemetry interface for reporting operational pipeline metrics
// to external monitoring systems (e.g., Prometheus, Datadog, or OpenTelemetry).
type MetricsReporter interface {
	// ReportLag records the consumer lag of a subscription.
	ReportLag(topic, partition, groupID string, lag int64)

	// Close performs cleanup (e.g. stopping metric servers or flushing).
	Close() error
}

// NoOpReporter is a metrics reporter that discards all telemetry data.
type NoOpReporter struct{}

// ReportLag discards the reported lag.
func (n *NoOpReporter) ReportLag(topic, partition, groupID string, lag int64) {}

// Close does nothing and returns nil.
func (n *NoOpReporter) Close() error { return nil }
