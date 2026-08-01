# Module Documentation: `metrics`

The `metrics` module centralises all Prometheus instrumentation for Auvis, providing standard counters, gauges, and histograms scraped from `/metrics` or `/auvis/metrics`.

> [!TIP]
> **Grafana Dashboard Ready:** Import **[examples/grafana_dashboard.json](file:///Users/Cridiv/Documents/Auvis/examples/grafana_dashboard.json)** directly into Grafana for instant visual monitoring of throughput, P95/P99 latency, outcomes, policy match rates, circuit breakers, and destination health.

---

## 📂 Prometheus Metric Reference Table

| Metric Name | Type | Labels | Description |
|---|---|---|---|
| `auvis_requests_total` | Counter | `status` | Total HTTP requests processed by HTTP status code (e.g. `200`, `403`, `500`). |
| `auvis_requests_by_outcome_total` | Counter | `outcome` | Requests count by transformation disposition (`masked`, `removed`, `denied`, `allowed`). |
| `auvis_request_duration_seconds` | Histogram | `method`, `resource` | Latency histogram of HTTP proxy request handling in seconds. |
| `auvis_destination_writes_total` | Counter | `destination`, `status` | Storage destination write attempts by driver name and status (`success`, `failure`). |
| `auvis_circuit_breaker_state` | Gauge | `destination` | Current state of destination circuit breakers (`0 = Closed`, `1 = Open`, `2 = Half-Open`). |
| `auvis_policy_matches_total` | Counter | `matched` | Requests matching at least one policy vs unmatched (`true`, `false`). |
| `auvis_pii_scan_total` | Counter | `mode` | PII scanner invocations by execution path (`regex`, `llm`). |
| `auvis_recovered_panics_total` | Counter | `component` | Total count of runtime panics recovered by Auvis safety handlers. |

---

## 1. `metrics.go`

All metrics are registered automatically at import via `promauto` and served over HTTP via `promhttp.Handler()`.

### Metric Specifications

#### `RequestsTotal *prometheus.CounterVec`
Tracks total HTTP proxy requests processed, broken down by HTTP status code (`status="200"`, `"403"`, `"500"`).

#### `RequestsByOutcomeTotal *prometheus.CounterVec`
Tracks request count by transformation disposition:
* `outcome="masked"`: Request contained fields masked by policy rules.
* `outcome="removed"`: Request contained fields removed/redacted.
* `outcome="denied"`: Request was rejected by `action: deny`.
* `outcome="allowed"`: Request passed through without transformation.

#### `RequestDurationSeconds *prometheus.HistogramVec`
Latency histogram capturing total proxy processing time in seconds (including policy lookup, payload transformation, and async audit dispatch) labelled by `method` and `resource`.

#### `DestinationWritesTotal *prometheus.CounterVec`
Tracks storage destination write attempts per driver (`destination="postgres"`, `"pinecone"`, `"weaviate"`, `"pgvector"`) and disposition (`status="success"`, `"failure"`).

#### `PolicyMatchesTotal *prometheus.CounterVec`
Tracks policy match rate (`matched="true"` vs `matched="false"`). Useful for alerting on misconfigured or silently-broken policy sets in production.

#### `PIIScanTotal *prometheus.CounterVec`
Tracks PII scanner mode execution counts (`mode="regex"` for fast zero-cost local scans vs `mode="llm"` for LLM-fallback scans).

#### `CircuitBreakerState *prometheus.GaugeVec`
Gauge tracking circuit breaker state per destination (`0 = Closed`, `1 = Open`, `2 = Half-Open`).

---

## Helper Functions

```go
ObserveRequest(method, resource string, statusCode int, durationSec float64)
IncOutcome(outcome string)
IncPolicyMatch(matched bool)
IncDestinationWrite(destination, status string)
IncPIIScan(mode string)
IncPanic(component string)
SetCircuitBreakerState(destination string, state int)
```
