# Changelog

All notable changes to **Auvis** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.6] - 2026-07-21

### Changed
- **Performance**: Increased gateway benchmark throughput from 1,710 req/sec to 6,541 req/sec (3.8× throughput increase, 7.6ms average latency, 0% error rate over 400,000 requests with 50-way concurrency) via four targeted optimizations:
  - **Reverse Proxy Caching**: Reused `httputil.ReverseProxy` instances per upstream URL via `sync.Map` rather than instantiating proxy structs, parsing URLs, and allocating closures per request.
  - **Buffer Pooling**: Implemented `sync.Pool` for `bytes.Buffer` response rehydration and `strings.Builder` array transform serialization, reducing memory allocation and GC pauses.
  - **Hot-Path Log Levels**: Downgraded per-request policy evaluation and per-record field disposition log messages from `slog.Info` to `slog.Debug`, removing log-writer stdout mutex contention under high concurrency.
  - **Audit Logger Non-blocking Writes**: Removed synchronous per-entry `f.Sync()` calls in background audit writer that previously forced kernel disk flushes after every log entry.

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
