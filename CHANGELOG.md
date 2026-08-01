# Changelog

All notable changes to **Auvis** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.6] - 2026-08-01

### Added
- **One-Way Deterministic Pseudonymization (`action: pseudonymize`)**: Added vaultless format-preserving HMAC-SHA256 pseudonymization (`usr_<hex>@<domain>`, `999-XX-XXXX`, `+1-555-01XX`, `usr_psd_<hex>`) for AI context tracking and vector index querying.
- **Per-Rule Seed Isolation**: Added optional `seed` rule parameter to isolate pseudonym spaces per department or tenant.
- **Configurable Audit Fsync Durability Modes**: Implemented `NewDurabilityLogger` supporting `"never"` (6,541 req/s), `"always"` (zero data loss, 1,700 req/s), and dynamic durations (`"100ms"`, `"5s"`, `"1m"`) with background ticker flushes (**ADR 001**).
- **Per-Record Audit Outcomes (`RecordOutcome`)**: Multi-record array responses populate granular per-record field dispositions (`fields_masked`, `fields_removed`, `filtered`) in `audit.jsonl` with positional ID fallback (`record[N]`).
- **Format-Preserving Masking Patterns**: Added dedicated format-aware maskers for `IBAN` (`GB29 **** **** **** **68 19`), `ABA Routing` (`*****6789`), and `Passport` (`A1****67`) numbers.
- **Liveness & Readiness Probes**: Operational `/healthz` (liveness) and `/readyz` (readiness with policy count & circuit breaker status) endpoints, 100% excluded from policy checks and audit logging.
- **Prometheus Telemetry (`GET /metrics`)**: Full Prometheus text exposition metrics (`auvis_requests_total`, `auvis_requests_by_outcome_total`, `auvis_request_duration_seconds`, `auvis_destination_writes_total`, `auvis_circuit_breaker_state`, `auvis_policy_matches_total`, `auvis_pii_scan_total`).
- **Static Multi-Upstream Microservice Routing**: `upstreams: [...]` config syntax allowing one gateway to route requests to multiple backend microservices based on path-prefix or header with isolated policy directories.
- **Grafana Observability Dashboard**: Added starter dashboard JSON (`examples/grafana_dashboard.json`).
- **Circuit Breaker Protection**: Added 3-state destination write protection (`Closed`, `Open`, `HalfOpen`) with fast-fail response handling.
- **Decoupled Availability**: Client HTTP responses deliver governed data independently of secondary storage destination failures (**ADR 003**).
- **Panic Recovery Middleware**: Stack trace logging and panic counter metrics across all proxy and pipeline goroutines.

### Changed
- **Performance**: Increased gateway benchmark throughput from 1,710 req/sec to 6,541 req/sec (3.8× throughput increase, 7.6ms average latency, 0% error rate over 400,000 requests with 50-way concurrency) via `ReverseProxy` caching, buffer pooling (`sync.Pool`), and non-blocking audit logging.

## [0.2.5] - 2026-06-27

### Added
- **Ingestion Pipeline Abstraction**: Added modular pipeline interface layer (`Source` → `Policy` → `Transform` → `Destination`).
- **Kafka Source Connector**: Added Kafka source driver supporting Schema Registry (Avro/Protobuf/JSON), DLQ routing, and manual offset commit callbacks.
- **Destination Sinks**: Added live destination support for PostgreSQL (`pgx/v5`), Pinecone (REST vector destination), pgvector, and Weaviate.
- **Sandboxed Condition Evaluator**: Added secure condition evaluation supporting `contains`, `in`, and time-based operators with fail-closed security for indeterminate states.
- **Docker Orchestration**: Added multi-stage `Dockerfile` and `docker-compose.yml` environment setup.
- **End-to-End Test Suite**: Consolidated integration tests into `tests/e2e/`.

## [0.1.0] - 2026-05-25

### Added
- **Core HTTP Gateway Proxy**: Reverse proxy engine forwarding requests with upstream dynamic header routing (`X-Auvis-Upstream`).
- **Policy Engine**: Role-based policy matching, wildcard rules (`*`), priority-based rule merging, and specificity precedence.
- **PII Detection Engine**: Hybrid regex and LLM fallback scanner with confidence scoring, validator rules, and overlap resolution.
- **Transformation Pipeline**: Response field transformation engine (`mask`, `remove`, `redact`, `truncate`, `deny`, `allow`, `llm_intercept`).
- **Row-Level Filtering**: Record-level filtering based on user context and evaluated field conditions.
- **Asynchronous Audit Logger**: Structured JSONL audit logging with size and age-based log file rotation.
- **CLI Commands & Validation**: Hot-reloading YAML configuration loader and `auvis validate` CLI tool.
