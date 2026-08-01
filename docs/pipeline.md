# Module Documentation: `pipeline`

The `pipeline` module implements Auvis's continuous streaming data ingestion engine: reading records from sources (Kafka), evaluating policies, transforming payloads, routing to storage destinations (with independent circuit breakers per destination), handling dead-letter queues, and emitting audit logs.

---

## 📂 File Index

- [`pipeline.go`](#1-pipelinego) — Core interfaces, `Pipeline` struct, execution loop, panic recovery, audit integration, and metrics.
- [`record.go`](#2-recordgo) — `Record` struct representing a single stream message.
- [`source.go`](#3-sourcego) — `Source` interface for data stream consumers.
- [`destination.go`](#4-destinationgo) — `Destination` interface for governed data storage sinks.
- [`registry.go`](#5-registrygo) — Global pipeline registry for dynamic pipeline instantiation.
- [`metrics.go`](#6-metricsgo) — `Metrics` and `MetricsSnapshot` types.
- [`policy_engine.go`](#7-policyenginego) — `PolicyEngine` bridge interface.
- [`config/`](#8-config-sub-package) — Pipeline YAML configuration models and loaders.
- [`decoder/`](#9-decoder-sub-package) — JSON, Avro, Protobuf decoders.
- [`source/`](#10-source-sub-package) — Kafka source, DLQ writer, Prometheus reporter.
- [`destination/`](#11-destination-sub-package) — Destination drivers and circuit breakers.

---

## 1. `pipeline.go`

### Interfaces

#### `PolicyEngine`
```go
type PolicyEngine interface {
    Evaluate(ctx context.Context, record *Record) (*policy.EffectivePolicy, error)
}
```
Wraps policy resolution logic. `Evaluate` receives the record's metadata (source, destination, schema, user attributes) and returns the compiled `*policy.EffectivePolicy` with field rules to apply, or `nil` if no policy matches (treated as deny-by-default).

---

#### `TransformEngine`
```go
type TransformEngine interface {
    Transform(ctx context.Context, record *Record, ep *policy.EffectivePolicy) (map[string]any, transform.Summary, error)
}
```
Applies field-level transformations (mask, remove, anonymize, redact, truncate, row-filter) to the record payload. Returns the transformed data map, a `transform.Summary` with disposition counts, and an error.

---

#### `AuditEngine`
```go
type AuditEngine interface {
    Log(entry audit.Entry)
}
```
Thin wrapper around `audit.Logger.Write`. Enables swappable audit backends in tests.

---

#### `DLQSink`
```go
type DLQSink interface {
    Write(ctx context.Context, record Record, reason string) error
    Close() error
}
```
Routes failed or policy-denied records to a dead-letter queue. `reason` is a human-readable string such as `"policy_deny: [pii-analyst.yaml]"` or `"transform_error: ..."`.

---

### `type Metrics struct`

Internal atomic counters. All fields are `uint64` incremented via `sync/atomic`. Never directly accessed externally.

| Field | Description |
|---|---|
| `ProcessedCount` | Total records processed (every record, regardless of outcome) |
| `FailedCount` | Records with any error: eval error, transform error, write error, or blocked |
| `MatchedCount` | Records with a successful policy match + transform + write |
| `DLQCount` | Records routed to dead-letter queue |
| `TotalDurationMS` | Cumulative processing duration for average latency calculation |
| `Latency2msBucket` | Records completed in ≤2ms |
| `Latency5msBucket` | Records completed in ≤5ms |
| `Latency10msBucket` | Records completed in ≤10ms |
| `Latency50msBucket` | Records completed in ≤50ms |
| `LatencyInfBucket` | Records completing in >50ms |

---

### `type MetricsSnapshot struct`

JSON-serializable snapshot returned by `GetMetrics()`.

| Field | JSON key | Description |
|---|---|---|
| `ProcessedCount` | `processed_count` | Total records processed |
| `FailedCount` | `failed_count` | Total records with any failure |
| `MatchedCount` | `matched_count` | Total records successfully matched + written |
| `DLQCount` | `dlq_count` | Total records sent to DLQ |
| `ErrorRate` | `error_rate` | `FailedCount / ProcessedCount` |
| `PolicyMatchRate` | `policy_match_rate` | `MatchedCount / ProcessedCount` |
| `AvgLatencyMS` | `avg_latency_ms` | `TotalDurationMS / ProcessedCount` |
| `LatencyHistogram` | `latency_histogram` | Map with `le_2ms`, `le_5ms`, `le_10ms`, `le_50ms`, `le_inf` buckets |

---

### `type Pipeline struct`

| Field | Type | Description |
|---|---|---|
| `Name` | `string` | Unique pipeline identifier used in all log messages |
| `Source` | `Source` | Data stream source (e.g. Kafka consumer) |
| `PolicyEngine` | `PolicyEngine` | Policy evaluation engine (can be nil to pass all records) |
| `TransformEngine` | `TransformEngine` | Payload transform engine (can be nil to pass untransformed) |
| `Destination` | `Destination` | Target storage destination (required) |
| `DLQ` | `DLQSink` | Optional dead-letter queue for blocked/failed records |
| `AuditEngine` | `AuditEngine` | Audit logger (can be nil to skip audit logging) |
| `FailClosed` | `bool` | If `true`, policy eval errors and transform errors block records. If `false`, errors allow the untransformed record through. |
| `DryRun` | `bool` | If `true`, evaluates policies and logs audit events but does **not** call `Destination.WriteRecord` |
| `metrics` | `Metrics` | Internal unexported atomic metrics tracker |

---

### `(p *Pipeline) Run(ctx context.Context) error`

Main pipeline execution loop.

**Startup:**
1. Logs `slog.Info("Starting pipeline execution", "pipeline", p.Name, "fail_closed", p.FailClosed)`.
2. Calls `p.Source.Connect()`. Returns error on failure.
3. Calls `p.Destination.Connect()`. Returns error on failure.
4. Defers `p.Source.Close()`, `p.Destination.Close()`, and `p.Destination.Flush()`.

**Main Loop:**
- Consumes from `p.Source.ReadRecords(ctx)` channel.
- On `ctx.Done()`: logs and returns `ctx.Err()`.
- On channel close (`ok == false`): logs and returns `nil`.
- For each `record`: calls `p.processRecord(ctx, record)`.

---

### `(p *Pipeline) processRecord(ctx context.Context, record Record)` *(private)*

Processes one stream record. This is the hottest code path in the pipeline.

**Panic Recovery Guard (first action):**
```go
defer func() {
    if rErr := recover(); rErr != nil {
        metrics.IncPanic("pipeline_process")    // Prometheus counter
        stack := string(debug.Stack())
        slog.Error("Panic recovered in pipeline record processing",
            "error", rErr, "stack", stack,
            "pipeline", p.Name, "record_id", record.ID)
        atomic.AddUint64(&p.metrics.FailedCount, 1)
    }
}()
```

**Identity Resolution:**
- Reads `user_id` and `user_role` from `record.Metadata` map.
- Defaults to `"system"` / `"pipeline-worker"` if absent.
- Generates `requestID = record.ID` or `fmt.Sprintf("rec-%d", UnixNano)` if empty.

**Phase 1: Policy Evaluation**
- If `p.PolicyEngine != nil`: calls `Evaluate(ctx, &record)`.
- `evalErr != nil` → if `FailClosed`: blocks record (`shouldWrite = false`). If fail-open: uses untransformed `record.Data`.
- `ep == nil` (no matching policy) → deny-by-default: `shouldWrite = false`.
- `ep.Deny == true` → explicit policy deny: `shouldWrite = false`.

**Phase 2: Transform**
- If `p.TransformEngine != nil` and `shouldWrite`: calls `Transform(ctx, &record, ep)`.
- `transformErr != nil` → if `FailClosed`: blocks. If fail-open: uses `record.Data` untransformed.

**Phase 3: Write or DLQ**

*If `shouldWrite == true`:*
- If `p.DryRun`: logs and calls `record.AckFunc()` only.
- Otherwise: calls `p.Destination.WriteRecord(recToWrite)`.
  - On write success: calls `record.AckFunc()`.
  - On write failure: logs error (record is not acknowledged).

*If `shouldWrite == false`:*
- If `p.DLQ != nil`: calls `p.DLQ.Write(ctx, record, dlqReason)`. On DLQ success: increments `DLQCount`, calls `record.AckFunc()`. On DLQ failure: logs error.
- If `p.DLQ == nil`: silently drops; calls `record.AckFunc()`.

**Phase 4: Audit Logging**
Constructs and enqueues a full `audit.Entry`:
- `Method: "INGEST"` (pipeline records always use this method string).
- `Resource`: set to `record.Source` (e.g. Kafka topic name).
- `UpstreamStatus: 200`. Overridden to `500` on write error, `403` on policy deny + fail-closed.
- If `!shouldWrite`: `FieldsDenied = ["*"]` and `ResponseDenied = true`.

**Phase 5: Metrics Update**
- Increments `ProcessedCount` always.
- Increments `FailedCount` if any error occurred or record was blocked.
- Increments `MatchedCount` if `ep != nil && !ep.Deny && evalErr == nil && transformErr == nil`.
- Adds `duration` to `TotalDurationMS`.
- Increments the appropriate latency histogram bucket (≤2ms, ≤5ms, ≤10ms, ≤50ms, or >50ms).

---

### `(p *Pipeline) GetMetrics() MetricsSnapshot`

Returns a thread-safe snapshot of pipeline counters. Uses `atomic.LoadUint64` for each field. Computes derived rates:
- `ErrorRate = FailedCount / ProcessedCount`
- `PolicyMatchRate = MatchedCount / ProcessedCount`
- `AvgLatencyMS = TotalDurationMS / ProcessedCount`

All rates default to `0` if `ProcessedCount == 0`.

---

## 11. `destination/` Sub-Package

### `circuit_breaker.go`

Implements the **Circuit Breaker Pattern** — a fuse box protecting destination writes from cascading failures.

---

#### `var ErrCircuitOpen`
```go
var ErrCircuitOpen = errors.New("circuit breaker is open (destination unavailable)")
```
Sentinel error returned immediately during `StateOpen` — no I/O attempted.

---

#### `type State int`

| Constant | Value | Meaning |
|---|---|---|
| `StateClosed` | `0` | Normal operation. Writes reach destination. |
| `StateOpen` | `1` | Tripped. All writes fail-fast (`<0.1ms`) with `ErrCircuitOpen`. |
| `StateHalfOpen` | `2` | Testing. One probe write is allowed through. |

`(s State) String() string` returns `"Closed"`, `"Open"`, `"Half-Open"`, or `"Unknown"`.

---

#### `type CircuitBreaker struct`

| Field | Type | Description |
|---|---|---|
| `mu` | `sync.Mutex` | Guards all state transitions |
| `name` | `string` | Destination driver name (used in logs and Prometheus labels) |
| `state` | `State` | Current state |
| `maxFailures` | `int` | Consecutive failure threshold to trip to `StateOpen` (default: 5) |
| `cooldown` | `time.Duration` | Time to wait in `StateOpen` before transitioning to `StateHalfOpen` (default: 30s) |
| `consecutiveFailures` | `int` | Running count of consecutive write failures |
| `openedAt` | `time.Time` | Timestamp when breaker last transitioned to `StateOpen` |

---

#### `NewCircuitBreaker(name string, maxFailures int, cooldown time.Duration) *CircuitBreaker`

Constructor. Validates parameters (defaults: `maxFailures=5`, `cooldown=30s`). Initializes to `StateClosed`. Calls `metrics.SetCircuitBreakerState(name, 0)` to publish initial Prometheus state.

---

#### `(cb *CircuitBreaker) Execute(op func() error) error`

Core state machine execution method. Thread-safe via `sync.Mutex`.

**On entry (lock held):**
1. If `state == StateOpen`:
   - If `time.Since(openedAt) >= cooldown` → transition to `StateHalfOpen`, log info, set Prometheus state to `2`.
   - Else → unlock and return `fmt.Errorf("%w for destination %q", ErrCircuitOpen, name)` immediately.
2. Capture `currentState`, unlock.

**Execute `op()` (lock released for actual I/O):**
This is where the real destination write call runs.

**On return (lock re-acquired):**

*If `op()` returned an error:*
- Increments `consecutiveFailures`.
- If `currentState == StateHalfOpen` OR `consecutiveFailures >= maxFailures`:
  - Transitions to `StateOpen`. Records `openedAt = time.Now()`.
  - Logs `slog.Warn("Circuit breaker TRIPPED (Open state) — fast failing writes during cooldown", "consecutive_failures", ...)`.
  - Sets Prometheus state to `1`.
- Returns the error.

*If `op()` succeeded:*
- If `state == StateHalfOpen` OR `consecutiveFailures > 0`:
  - Resets: `state = StateClosed`, `consecutiveFailures = 0`.
  - Logs `slog.Info("Circuit breaker RESET (Closed state) — destination health restored")`.
  - Sets Prometheus state to `0`.
- Returns `nil`.

---

#### `(cb *CircuitBreaker) State() State`
Thread-safe state reader. Acquires `mu`, reads `cb.state`, returns.

#### `(cb *CircuitBreaker) Name() string`
Returns the destination name string (no lock needed; immutable after construction).

---

#### `type BreakerDestination struct`

Wraps any `pipeline.Destination` with an independent `CircuitBreaker`. Suitable for wrapping single-destination deployments (as opposed to `MultiDestination` which manages its own breaker slice).

| Field | Type | Description |
|---|---|---|
| `Destination` | `pipeline.Destination` | Embedded destination (all methods except `WriteRecord` are delegated) |
| `Name` | `string` | Display name |
| `Breaker` | `*CircuitBreaker` | Independent circuit breaker |

#### `NewBreakerDestination(dest pipeline.Destination, name string) *BreakerDestination`
Constructor. Creates breaker with defaults (`maxFailures=5`, `cooldown=30s`).

#### `(b *BreakerDestination) WriteRecord(rec pipeline.Record) error`
Delegates to `b.Breaker.Execute(func() error { return b.Destination.WriteRecord(rec) })`.

---

### `multi_destination.go`

Fan-out destination that writes to multiple storage sinks simultaneously, each with an independent circuit breaker.

---

#### `type DestWriteResult struct`

| Field | Type | Description |
|---|---|---|
| `Type` | `string` | Driver name (`"postgres"`, `"pinecone"`, etc.) |
| `Written` | `bool` | `true` if `WriteRecord` returned nil |
| `Err` | `error` | Error returned by `WriteRecord` |
| `DurationMs` | `int64` | Wall-clock duration for this write |

---

#### `type MultiError struct`
Returned when one or more destination writes fail. Contains `Results []DestWriteResult`.
`Error() string` formats as `"multi-destination write failed: pinecone: timeout; postgres: connection refused"`.

---

#### `type MultiDestination struct`

| Field | Type | Description |
|---|---|---|
| `destinations` | `[]pipeline.Destination` | Ordered slice of destination drivers |
| `names` | `[]string` | Human-readable labels matching `destinations` by index |
| `breakers` | `[]*CircuitBreaker` | Independent circuit breaker per destination, created in `NewMultiDestination` |

---

#### `NewMultiDestination(dests []pipeline.Destination, names []string) *MultiDestination`

Constructor. For each destination name, creates `NewCircuitBreaker(name, 5, 30*time.Second)`. The `dests` and `names` slices **must have the same length**.

---

#### `(m *MultiDestination) Connect() error`

Calls `d.Connect()` on each destination sequentially. Fails fast on the first error with `fmt.Errorf("destination[%d] %s connect: %w", i, name, err)`.

---

#### `(m *MultiDestination) WriteRecord(rec pipeline.Record) error`

Fan-out with per-destination circuit breaker protection.

For each `(i, d)` in `m.destinations`:
1. Calls `m.breakers[i].Execute(func() error { ... })` wrapping the actual write:
   - **Inside the Execute closure**: a `defer recover()` guard catches panics. On panic:
     - Calls `metrics.IncPanic("destination_write")`.
     - Captures `string(debug.Stack())`.
     - Logs `slog.Error("Panic in destination write execution", "stack", stack, ...)`.
     - Sets `writeErr = fmt.Errorf("panic in destination write (%s): %v", name, rErr)`.
   - Calls `d.WriteRecord(rec)`.
2. Measures `dur = time.Since(start).Milliseconds()`.
3. Stores `DestWriteResult{Type, Written, Err, DurationMs}`.
4. Logs:
   - `ErrCircuitOpen` → `slog.Warn("Destination write fast-failed (circuit breaker OPEN)", ...)`.
   - Other errors → `slog.Error("Destination write failed", ...)`.
   - Success → `slog.Debug("Destination write succeeded", ...)`.

Returns `nil` if all succeed. Returns `&MultiError{Results}` if any failed.

---

#### `(m *MultiDestination) GetBreaker(name string) *CircuitBreaker`

Lookup a breaker by destination name (case-insensitive via `strings.EqualFold`). Returns `nil` if not found.

---

#### `(m *MultiDestination) Flush() error`

Calls `Flush()` on each destination. Collects all errors and returns them joined via `errors.Join`.

#### `(m *MultiDestination) Close() error`

Calls `Close()` on each destination. Collects all errors and returns them joined via `errors.Join`.

---

### Destination Drivers

#### `postgres_destination.go`

Writes governed records to a PostgreSQL table.

**`PostgresDestination` fields:** `connStr`, `tableName`, `autoCreate`, `auth`, `tls`, `pool (*pgxpool.Pool)`.

**`NewPostgresDestination(settings, auth, tls)`**: Reads `table` (default: `auvis_records`), `auto_create_table` (default: `true`). Calls `buildConnectionString(settings, auth)`. Sanitizes table name via `sanitizeTableName`.

**`Connect()`**: Parses DSN via `pgxpool.ParseConfig`. Configures TLS if `ca_cert`, `client_cert`, or `skip_verify` are set. Initializes pool via `pgxpool.NewWithConfig`. Pings with `pool.Ping`. If `autoCreate`: calls `ensureTableExists`.

**`WriteRecord(record)`**: Marshals `record.Data` and `record.Metadata` to JSON. Executes `INSERT INTO ... ON CONFLICT (id) DO UPDATE SET ...` upsert query. The table schema:
```sql
CREATE TABLE IF NOT EXISTS auvis_records (
    id         TEXT PRIMARY KEY,
    source     TEXT,
    schema_name TEXT,
    data       JSONB,
    metadata   JSONB,
    timestamp  TIMESTAMPTZ
)
```

**`Flush()`**: no-op (returns nil).
**`Close()`**: Calls `pool.Close()`.

---

#### `pinecone_destination.go`

Upserts vector records to Pinecone via REST API.

**`PineconeDestination` fields:** `indexHost`, `apiKey`, `namespace`, `vectorField`, `httpClient`.

**`NewPineconeDestination(settings, auth, tls)`**: Strips `https://` / `http://` from `index_host`. Reads `namespace`, `vector_field` (default: `"embedding"`). Uses `auth.Password` or `settings["api_key"]` as API key. Creates `http.Client{Timeout: 10s}` with optional TLS skip verify.

**`Connect()`**: Validates `indexHost != ""` and `apiKey != ""`.

**`WriteRecord(record)`**: Extracts `vector_field` from `record.Data` as `[]float64` or `[]any`. Builds Pinecone upsert payload:
```json
{"vectors": [{"id": "...", "values": [...], "metadata": {...}}], "namespace": "..."}
```
POSTs to `https://<indexHost>/vectors/upsert` with `Api-Key: <apiKey>` header. Fails-closed on non-2xx response.

---

#### `pgvector_destination.go`

Inserts vector embeddings into PostgreSQL with the `pgvector` extension.

**`PgVectorDestination` fields:** `connStr`, `TableName`, `VectorField`, `VectorDim`, `AutoCreate`, `pool`.

**`NewPgVectorDestination(settings, auth, tls)`**: Reads `table` (default: `auvis_vectors`), `vector_field` (default: `"embedding"`), `vector_dim` (default: `3`), `auto_create_table`. Reuses `buildConnectionString` from `postgres_destination.go`.

**`Connect()`**: Connects via pgxpool. Runs `CREATE EXTENSION IF NOT EXISTS vector` to enable pgvector. Calls `ensureTableExists` if `AutoCreate`:
```sql
CREATE TABLE IF NOT EXISTS auvis_vectors (
    id          VARCHAR(255) PRIMARY KEY,
    source      VARCHAR(255),
    schema_name VARCHAR(255),
    data        JSONB,
    metadata    JSONB,
    embedding   vector(<VectorDim>),
    timestamp   TIMESTAMPTZ
)
```

**`WriteRecord(record)`**: Calls `extractFloatSlice(record.Data, VectorField)`. Serializes `data` (without vector field) and `metadata` to JSON. Calls `FormatPgVector(values)` → `"[0.1,0.2,0.3]"`. Executes:
```sql
INSERT INTO <table> (id, source, schema_name, data, metadata, embedding, timestamp)
VALUES ($1,$2,$3,$4,$5,$6::vector,$7)
ON CONFLICT (id) DO UPDATE SET data=..., metadata=..., embedding=..., timestamp=...
```

**`FormatPgVector(values []float64) string`**: Converts float64 slice to pgvector text literal `"[0.1,0.2,0.3]"` using `strconv.FormatFloat`.

---

#### `weaviate_destination.go`

Upserts objects to Weaviate via REST API with optional vector fields.

---

## Source sub-package

### `source/kafka_source.go`

Kafka consumer source using `confluent-kafka-go`.

**`KafkaSource` fields:** `brokers`, `topic`, `groupID`, `settings`, `consumer`, `decoderRegistry`, `metricsReporter`.

**`Connect()`**: Creates `kafka.NewConsumer(configMap)`. Calls `consumer.Subscribe(topic)`.

**`ReadRecords(ctx) <-chan Record`**: Goroutine polls `consumer.ReadMessage(1s)`. Decodes binary message via configured decoder (JSON, Avro, Protobuf). Sends decoded `Record` to output channel. Assigns `AckFunc = func() { consumer.CommitMessage(msg) }`.

**Prometheus lag reporting**: If `metrics_provider == "prometheus"` in settings, instantiates `PrometheusReporter` and reports lag periodically.

---

### `source/dlq_writer.go`

Writes failed records to a Dead-Letter Queue Kafka topic.

**`DLQWriter` fields:** `producer`, `topic`.

**`Write(ctx, record, reason)`**: Marshals `record` to JSON. Produces Kafka message to `dlq_topic` with `reason` as a message header.

---

### `source/prometheus_reporter.go`

Exports Kafka consumer lag metrics to Prometheus.

**`PrometheusReporter` fields:** `server *http.Server`, `mu sync.Mutex`.

**`NewPrometheusReporter(port string)`**: If `port != ""`, starts an HTTP server with `mux.Handle("/metrics", promhttp.Handler())`.

**`ReportLag(topic, partition, groupID string, lag int64)`**: Sets `auvis_kafka_consumer_lag{topic, partition, group_id}` gauge.

**`Close()`**: Calls `server.Shutdown(ctx)` with 5-second timeout.
