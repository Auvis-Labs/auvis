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
)

// IncPanic increments the recovered panic counter for a given component.
func IncPanic(component string) {
	RecoveredPanicsTotal.WithLabelValues(component).Inc()
}
