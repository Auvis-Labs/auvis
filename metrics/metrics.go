package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RecoveredPanicsTotal tracks the total number of runtime panics recovered by Auvis.
	// Labels:
	//   component: "proxy_middleware", "proxy_handler", "destination_write", "pipeline_process"
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
)

// IncPanic increments the recovered panic counter for a given component.
func IncPanic(component string) {
	RecoveredPanicsTotal.WithLabelValues(component).Inc()
}

// SetCircuitBreakerState updates the Prometheus gauge metric for a destination's breaker state.
func SetCircuitBreakerState(destination string, state int) {
	CircuitBreakerState.WithLabelValues(destination).Set(float64(state))
}
