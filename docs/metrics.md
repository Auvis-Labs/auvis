# Auvis Prometheus Telemetry & Metrics Reference

Auvis exports production-grade Prometheus metrics for real-time monitoring of policy enforcement, gateway latency, circuit breaker states, and streaming ingestion health.

Metrics are served over standard HTTP on `GET /metrics` and `GET /auvis/metrics`.

> [!TIP]
> **Grafana Dashboard Ready:** Import **[examples/grafana_dashboard.json](file:///Users/Cridiv/Documents/Auvis/examples/grafana_dashboard.json)** directly into your Grafana instance for visual monitoring of throughput, P95/P99 latency, outcomes, policy match rates, circuit breakers, and destination health.

---

## 📊 Prometheus Metric Reference Table

| Metric Name | Type | Labels | Description |
|---|---|---|---|
| `auvis_requests_total` | Counter | `status` | Total HTTP requests processed, labeled by HTTP status code (`200`, `403`, `500`). |
| `auvis_requests_by_outcome_total` | Counter | `outcome` | Requests count by transformation disposition: `masked`, `removed`, `denied`, `allowed`. |
| `auvis_request_duration_seconds` | Histogram | `method`, `resource` | Latency histogram of HTTP proxy request handling in seconds. |
| `auvis_destination_writes_total` | Counter | `destination`, `status` | Storage destination write attempts by driver name (`postgres`, `pgvector`, `pinecone`, `weaviate`) and status (`success`, `failure`). |
| `auvis_circuit_breaker_state` | Gauge | `destination` | Current circuit breaker state per destination: `0 = Closed` (healthy), `1 = Open` (tripped/failing fast), `2 = Half-Open` (probing). |
| `auvis_policy_matches_total` | Counter | `matched` | Requests matching at least one policy vs unmatched (`true`, `false`). |
| `auvis_pii_scan_total` | Counter | `mode` | PII scanner invocations by execution path (`regex` = local zero-cost pattern scan, `llm` = Gemini fallback scan). |
| `auvis_recovered_panics_total` | Counter | `component` | Total count of runtime panics safely caught and isolated by Auvis recovery middleware. |
| `auvis_kafka_consumer_lag` | Gauge | `topic`, `partition`, `group_id` | Streaming lag count for Kafka/Redpanda consumer group sources (requires `metrics_provider: "prometheus"`). |

---

## 📈 Metric Details & Production Usage

### 1. Request Outcome Tracking (`auvis_requests_by_outcome_total`)
Tracks the governance outcome of every processed request:
* `outcome="masked"`: Request had one or more fields masked or pseudonymized.
* `outcome="removed"`: Request had sensitive fields deleted from the payload.
* `outcome="denied"`: Request was rejected (HTTP 403) by an explicit `action: deny` rule or shadow PHI `posture: block`.
* `outcome="allowed"`: Request matched an allow rule or had no sensitive fields.

```promql
# Rate of masked requests per second
rate(auvis_requests_by_outcome_total{outcome="masked"}[5m])
```

### 2. Destination Write Health & Circuit Breakers
Monitor storage sinks (e.g. pgvector, Pinecone) to detect degraded databases before they impact upstream systems:

```promql
# Destination write failure rate
sum by (destination) (rate(auvis_destination_writes_total{status="failure"}[5m]))

# Alert when any circuit breaker trips to Open (1)
auvis_circuit_breaker_state > 0
```

### 3. Latency Histograms (`auvis_request_duration_seconds`)
Measures end-to-end proxy processing latency, including policy lookup, JSON parsing, field transformation, and async audit submission:

```promql
# P99 latency by API resource
histogram_quantile(0.99, sum(rate(auvis_request_duration_seconds_bucket[5m])) by (le, resource))
```

### 4. Policy Matching Telemetry (`auvis_policy_matches_total`)
Monitors whether incoming API traffic is successfully matching defined policies.

```promql
# Percentage of traffic covered by governance policies
(sum(rate(auvis_policy_matches_total{matched="true"}[5m])) / sum(rate(auvis_requests_total[5m]))) * 100
```

---

## 🛠️ Prometheus Scrape Configuration

Add the following scrape job to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: "auvis-gateway"
    scrape_interval: 10s
    metrics_path: "/metrics"
    static_configs:
      - targets: ["auvis-gateway.internal:8082"]
```
