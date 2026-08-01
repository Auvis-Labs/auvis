package main

import (
	"fmt"
	"net/http/httptest"
	"strings"

	"github.com/Auvis-Labs/auvis/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	fmt.Println("Simulating Auvis Gateway events and metric generation...")

	// 1. Simulate incoming HTTP requests & latencies
	metrics.ObserveRequest("GET", "/api/patients", 200, 0.0076)
	metrics.ObserveRequest("GET", "/api/patients", 200, 0.0052)
	metrics.ObserveRequest("POST", "/api/records", 403, 0.0012)

	// 2. Simulate request transformation outcomes
	metrics.IncOutcome("masked")
	metrics.IncOutcome("masked")
	metrics.IncOutcome("denied")
	metrics.IncOutcome("allowed")

	// 3. Simulate policy match telemetry
	metrics.IncPolicyMatch(true)
	metrics.IncPolicyMatch(true)
	metrics.IncPolicyMatch(false)

	// 4. Simulate destination writes
	metrics.IncDestinationWrite("postgres", "success")
	metrics.IncDestinationWrite("postgres", "success")
	metrics.IncDestinationWrite("pinecone", "failure")

	// 5. Simulate circuit breaker states (0 = Closed, 1 = Open)
	metrics.SetCircuitBreakerState("postgres", 0)
	metrics.SetCircuitBreakerState("pinecone", 1)

	// 6. Simulate PII scanner mode execution
	metrics.IncPIIScan("regex")
	metrics.IncPIIScan("regex")
	metrics.IncPIIScan("llm")

	// Render Prometheus exposition text output
	handler := promhttp.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	fmt.Println("\n=================== AUVIS PROMETHEUS /metrics OUTPUT ===================")
	lines := strings.Split(w.Body.String(), "\n")
	for _, line := range lines {
		if strings.Contains(line, "auvis_") {
			fmt.Println(line)
		}
	}
	fmt.Println("=========================================================================")
}
