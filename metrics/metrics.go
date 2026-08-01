package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RecoveredPanicsTotal tracks the total number of runtime panics recovered by Auvis.
	RecoveredPanicsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auvis_recovered_panics_total",
			Help: "Total count of runtime panics recovered by Auvis safety handlers.",
		},
		[]string{"component"},
	)

	// CircuitBreakerState tracks current circuit breaker state per destination:
	// 0 = Closed (Normal), 1 = Open (Tripped), 2 = Half-Open (Testing Health)
	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "auvis_circuit_breaker_state",
			Help: "Current state of destination circuit breakers (0 = Closed, 1 = Open, 2 = Half-Open)",
		},
		[]string{"destination"},
	)

	// RequestsTotal tracks the total number of HTTP proxy requests processed.
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auvis_requests_total",
			Help: "Total count of HTTP proxy requests processed by HTTP status code.",
		},
		[]string{"status"},
	)

	// RequestsByOutcomeTotal tracks requests by transformation disposition (masked, removed, denied, allowed).
	RequestsByOutcomeTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auvis_requests_by_outcome_total",
			Help: "Total count of processed requests by transformation outcome (masked, removed, denied, allowed).",
		},
		[]string{"outcome"},
	)

	// RequestDurationSeconds records latency histogram of HTTP proxy requests.
	RequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "auvis_request_duration_seconds",
			Help:    "Latency histogram of HTTP proxy request handling in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "resource"},
	)

	// DestinationWritesTotal tracks storage destination writes by driver name and status.
	DestinationWritesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auvis_destination_writes_total",
			Help: "Total count of storage destination write attempts by driver name and status (success, failure).",
		},
		[]string{"destination", "status"},
	)

	// PolicyMatchesTotal tracks policy match rate (true if matched at least one policy, false otherwise).
	PolicyMatchesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auvis_policy_matches_total",
			Help: "Count of requests matching at least one policy vs unmatched (true, false).",
		},
		[]string{"matched"},
	)

	// PIIScanTotal tracks PII scanner execution counts by mode (regex, llm).
	PIIScanTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auvis_pii_scan_total",
			Help: "Count of PII scanner invocations by execution path (regex, llm).",
		},
		[]string{"mode"},
	)
)

// IncPanic increments the recovered panic counter for a given component.
func IncPanic(component string) {
	RecoveredPanicsTotal.WithLabelValues(component).Inc()
}

// SetCircuitBreakerState updates the Prometheus gauge metric for a destination's breaker state.
func SetCircuitBreakerState(destination string, state int) {
	CircuitBreakerState.WithLabelValues(destination).Set(float64(state))
}

// ObserveRequest records a completed HTTP request's latency and status code.
func ObserveRequest(method, resource string, statusCode int, durationSec float64) {
	RequestsTotal.WithLabelValues(strconv.Itoa(statusCode)).Inc()
	RequestDurationSeconds.WithLabelValues(method, resource).Observe(durationSec)
}

// IncOutcome increments the request outcome counter ("masked", "removed", "denied", "allowed").
func IncOutcome(outcome string) {
	RequestsByOutcomeTotal.WithLabelValues(outcome).Inc()
}

// IncPolicyMatch increments policy match tracking ("true" or "false").
func IncPolicyMatch(matched bool) {
	PolicyMatchesTotal.WithLabelValues(strconv.FormatBool(matched)).Inc()
}

// IncDestinationWrite records destination write outcomes ("success" or "failure").
func IncDestinationWrite(destination, status string) {
	DestinationWritesTotal.WithLabelValues(destination, status).Inc()
}

// IncPIIScan records PII scanner mode execution ("regex" or "llm").
func IncPIIScan(mode string) {
	PIIScanTotal.WithLabelValues(mode).Inc()
}
