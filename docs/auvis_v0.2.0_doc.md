# Auvis Phase 2 — Pipeline Engine Foundation Architectural Decisions

> Living document. Updated as build decisions are made for Phase 2.
> Start date: May 2026

## Version History

| Version | Date | What it covers |
|---|---|---|
| v2.0.0 | May 31, 2026 | Initial Pipeline Abstraction Layer defined (Source, Destination, Record, Pipeline) |

---

## Architectural Decisions

### Decision 1: Streaming Ingestion Pipeline Abstraction Layer

**Context:** The Phase 1 engine worked as an HTTP reverse proxy, intercepting and transforming REST responses synchronously. Phase 2 extends this design by building a highly scalable, decoupled, and continuous streaming ingestion pipeline engine (e.g. processing records as they are ingested from sources like Kafka and sent to destinations like vector databases).

**Decision:**
Defined four lightweight core abstractions in the newly created `pipeline` package:
1. **`Source` Interface**: Employs Go's idiomatic channel-based design (`ReadRecords(ctx) <-chan Record`) to enable non-blocking read loops, native backpressure propagation, and elegant graceful shutdown via context cancellation.
2. **`Destination` Interface**: Exposes sequential `WriteRecord` with synchronous error feedback to facilitate deterministic error propagation (fail-open vs. fail-closed) and an explicit `Flush()` function to support batched/buffered downstream writes.
3. **`Record` Envelope**: Standardized record representation encapsulating standard ingestion attributes (`ID`, `Source`, `Schema`, `Data`, `Metadata`, and `${TIMESTAMP}`).
4. **`Pipeline` Coordinator**: Links components together to manage continuous ingestion workflows. Designed utilizing decoupled engine interfaces (`PolicyEngine`, `TransformEngine`, and `AuditEngine`) to ensure full modularity and testability.

---

### Decision 2: Pipeline Main Loop and Configurable Fail-Open vs. Fail-Closed Strategy

**Context:** In a streaming pipeline, records may hit compilation, policy resolution, context metadata lookup, or structural transformation failures. Depending on the enterprise compliance regime, a failure should either strictly quarantine the record (Fail-Closed) or let the records pass untouched to guarantee zero ingestion loss (Fail-Open).

**Decision:**
Implemented `Pipeline.Run(ctx)` with an active processor:
- **`Fail-Closed` (Strict Security Mode)**: Any policy evaluation or transformation failure blocks writing of that record to the destination. The record is dropped, and the transaction is safely logged as a `403` denied state in the audit logger to prevent accidental data leaks.
- **`Fail-Open` (High Throughput / Non-Disruptive Mode)**: If an error is caught during policy matching or processing, the system logs the incident but continues forwarding the raw untransformed record to the destination sink, ensuring uninterrupted data delivery.
- **Continuous Execution**: Single record failures do not crash the pipeline main loop; the system logs the error, updates the audit logs, and proceeds immediately to the next record in the stream.

---

### Decision 3: Reusing HTTP Proxy Transformations via Exposed Package Hooks

**Context:** We already have a highly audited transformation suite in the `transform` package supporting pattern-masking, check digit card validation, and rune-safe truncations. Re-implementing these for pipeline records is redundant and error-prone.

**Decision:**
Exposed `transform.Apply` publicly in the `transform` package as a clean wrapper around the private `apply` logic. This allows our `pipeline`'s default `TransformEngine` implementations to marshal the record's payload map to JSON bytes, execute standard transformations, and re-unmarshal back to `map[string]any`. This reuses 100% of Phase 1's field and row-level filtering rules securely.

---

### Decision 4: Declarative Pipeline Auth & TLS Credentials Configuration Model

**Context:** Ingestion endpoints (like Kakfa clusters, relational databases, vector DB indices) enforce strict access boundaries. The authentication patterns range from simple header API keys to enterprise-grade OAuth client flows, Mutual TLS certificates, and AWS IAM dynamic role assumptions.

**Decision:**
Created unified configuration structures inside `pipeline/config.go` to provide a declarative mapping of source/destination properties:
- **`AuthConfig`**: Houses parameters for five distinct validation styles: simple tokens (`api_key`), credential chains (`basic`), SASL challenges (`sasl_ssl`), token acquisition flows (`oauth2`), and assumed instance identities (`iam_role`).
- **`TLSConfig`**: Centralizes client and corporate root certificates, enabling mutual authentication and custom verification ignores (`skip_verify`) to accommodate local and production runtime targets.

---
 
### Decision 5: Declarative Environment Variable Expansion in Pipeline Configuration Loader

**Context:** Security best practices strictly prohibit checking raw secrets (like Kafka passwords, API keys, and private TLS keys) into Git repositories. Instead, these configurations must reference environment variables dynamically in the YAML specification.

**Decision:**
Implemented safe pre-unmarshal environment variable resolution in `pipeline/config_loader.go`:
- **Strict Interpolation**: Utilizes a regular expression targeting the explicit `${ENV_VAR}` pattern strictly. This avoids standard library expansions like `$VAR` which might clash unescaped with inline strings, inline regular expressions, or custom system templates.
- **Pre-Unmarshal Substitution**: Replaces all active environment tags with their resolved OS environment values *prior* to parsing by `yaml.Unmarshal`. This maintains clean struct schemas and allows pipeline YAML files to check into version control safely while securing real credential values in runtime environment environments.

---

### Decision 6: Declarative Pipeline YAML Schema Specification Model

**Context:** Managing streaming pipelines involves declaring ingestion connections, targeting database sinks, routing policies, and error thresholds in a single, structured YAML configuration layout.

**Decision:**
Defined structured parsing models inside `pipeline/config.go` and `pipeline/config_loader.go`:
- **`PipelineConfig`**: Coordinates the root YAML properties: pipeline `name`, `source` properties, `destination` targets, dynamic `policies` directories, and loop `error_handling` strategies.
- **`ConnectorConfig`**: Unifies dynamic settings (`Settings map[string]string`) alongside strictly typed credential structures (`Auth` and `TLS`). This accommodates dynamic setups (e.g. topic definitions, URLs, or host configurations) across all connector drivers.
- **Pre-Integrated Resolvers**: Exposed standard loading hooks (`ParsePipelineConfig` and `LoadPipelineConfig`) integrating environment substitutions automatically.

---

### Decision 7: Pipeline Lifecycle Manager and Registry Core

**Context:** Production environments load, process, monitor, and scale multiple named ingestion streams concurrently. We need a central controller capable of running pipelines in background tasks, reporting health, and isolating failures.

**Decision:**
Designed a thread-safe `Registry` mapping registered named pipelines to concurrent `RunState` contexts:
- **Goroutine Context Isolation**: `Registry.Start` spawns individual background execution routines governed by isolated cancellations (`context.WithCancel`).
- **Active Telemetry and Diagnostics**: `Registry.HealthCheck` polls states, reporting standard statuses (`running`, `stopped`, `failed`) and capturing precise root-cause errors (e.g. source authentication or host offline faults) during run failures.
- **Atomic Operations**: Employs global locking (`sync.RWMutex`) to guarantee thread safety during hot registration, health polls, or rolling shutdowns.

---

### Decision 8: Graceful Shutdown and Channel In-Flight Record Draining

**Context:** Abruptly terminating streaming ingestion pipelines leaks memory, drops buffers, and corrupts transactional integrity. The system must drain all in-flight records inside internal buffers, flush downstream writes, and gracefully disconnect connections before exiting.

**Decision:**
Leveraged standard Go channel closing mechanics to achieve zero-loss shutdowns:
- **Channel Closing Chain**: Stopping a pipeline cancels the source context. The source halts new record generation and closes the standard `ReadRecords(ctx)` channel.
- **In-Flight Draining**: The pipeline loop continues consuming all buffered records inside the closed channel natively.
- **Teardown Synchronization**: `Registry.Stop` blocks on a completion channel (`<-state.Done`) until the pipeline loop is fully exhausted, flushes target databases (`Destination.Flush()`), and shuts down sockets gracefully.

---

### Decision 9: Real-Time Pipeline-Level Ingestion Metrics and High-Performance Telemetry

**Context:** Enterprise operational compliance requires real-time monitoring of ingestion rates (throughput), processing delays (latencies), transformation errors, and policy match occurrences without introducing execution overhead to high-speed streams.

**Decision:**
Designed an exceptionally lightweight, thread-safe metrics accumulator utilizing Go's standard atomic instructions (`sync/atomic`):
- **Atomic Operations**: Tracks four primary counters concurrently: `ProcessedCount`, `FailedCount` (capturing connections/transforms/denies), `MatchedCount` (capturing active policy match events), and `TotalDurationMS` (total processing time).
- **Latency Histogram**: Instantly indexes durations into standard Prometheus-style buckets (`<=2ms`, `<=5ms`, `<=10ms`, `<=50ms`, `le_inf`) using quick boundary checks.
- **Lock-Free Snapshots**: Exposes `Pipeline.GetMetrics()` returning a consistent `MetricsSnapshot` featuring error rates, average latencies, and match margins on-demand without memory locking.

---

### Decision 10: Non-Disruptive Pipeline Dry-Run Simulation Mode

**Context:** Activating new security policies or modifying field masking rules on live customer pipelines presents regression risks. Administrators need a way to validate policy matching and inspect exact transformations across live data data streams without modifying target databases.

**Decision:**
Implemented `DryRun` validation directly within the pipeline process loop:
- **Bypassed routing**: When `DryRun` is active, the pipeline executes 100% of policy evaluation, sliding-window PII checks, and data transformations as usual, but bypasses writing to downstream sinks (`Destination.WriteRecord`).
- **Telemetry Transparency**: Writes comprehensive audits and logs detailing exactly what actions *would* have been taken, providing a zero-impact testing sandbox.

---

### Reference Code

#### [Source Interface](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/source.go)
```go
type Source interface {
	Connect() error
	ReadRecords(ctx context.Context) <-chan Record
	Close() error
}
```

#### [Destination Interface](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/destination.go)
```go
type Destination interface {
	Connect() error
	WriteRecord(record Record) error
	Flush() error
	Close() error
}
```

#### [Record Struct](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/record.go)
```go
type Record struct {
	ID        string                 `json:"id"`
	Source    string                 `json:"source"`
	Schema    string                 `json:"schema"`
	Data      map[string]any         `json:"data"`
	Metadata  map[string]any         `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
	AckFunc   func() error           `json:"-"`
}
```

#### [Pipeline Coordinator & Main Loop](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/pipeline.go)
```go
type Pipeline struct {
	Name            string
	Source          Source
	PolicyEngine    PolicyEngine
	TransformEngine TransformEngine
	Destination     Destination
	AuditEngine     AuditEngine
	FailClosed      bool
	DryRun          bool
	metrics         Metrics
}

func (p *Pipeline) Run(ctx context.Context) error { ... }
func (p *Pipeline) GetMetrics() MetricsSnapshot { ... }
```

#### [AuthConfig, TLSConfig & PipelineConfig Schema](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/config.go)
```go
type PipelineConfig struct {
	Name          string              `yaml:"name"`
	Source        ConnectorConfig     `yaml:"source"`
	Destination   ConnectorConfig     `yaml:"destination"`
	Policies      PoliciesConfig      `yaml:"policies"`
	ErrorHandling ErrorHandlingConfig `yaml:"error_handling"`
}

type ConnectorConfig struct {
	Type     string            `yaml:"type"`
	Settings map[string]string `yaml:"settings"`
	Auth     AuthConfig        `yaml:"auth"`
	TLS      TLSConfig         `yaml:"tls"`
}

type PoliciesConfig struct {
	Dir string `yaml:"dir"`
}

type ErrorHandlingConfig struct {
	FailClosed bool `yaml:"fail_closed"`
}
```

#### [YAML Loading and Parsing Hooks](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/config_loader.go)
```go
func ParsePipelineConfig(data []byte) (*PipelineConfig, error) { ... }
func LoadPipelineConfig(path string) (*PipelineConfig, error) { ... }
```

#### [Pipeline Registry Lifecycle and Teardown](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/registry.go)
```go
type Registry struct {
	pipelines map[string]*Pipeline
	states    map[string]*RunState
}

func (r *Registry) Start(ctx context.Context, name string) error { ... }
func (r *Registry) Stop(name string) error { ... }
func (r *Registry) HealthCheck(name string) (PipelineStatus, error) { ... }
```

---

### Decision 11: Enterprise Kafka Source Connector Implementation

**Context:** Production messaging architectures utilize Apache Kafka as the main event broker for incoming streams of application, transactional, and audit logs. The ingestion engine requires a native source connector driver that implements the `Source` interface, with support for:
- Standard topic subscription, server brokers list, and consumer group orchestration.
- Secure transit using TLS (CA pools, Client TLS Certificate and Key pairs, and skipped hostname validation flags).
- Authentication mechanisms spanning SASL SSL with `PLAIN`, `SCRAM-SHA-256`, and `SCRAM-SHA-512` support.
- Configurable offset reset strategies (`earliest` or `latest`).
- Fallback data mapping to prevent raw JSON parsing failures from aborting the pipeline loop.

**Decision:**
Implemented `KafkaSource` inside the `pipeline` package using `github.com/segmentio/kafka-go`:
1. **Connection Dialer Setup**: `Connect` initiates connection hooks with the required SASL mechanism and TLS configs mapped via `AuthConfig` and `TLSConfig`.
2. **SASL Configuration**: Handled `PLAIN` and `SCRAM` mechanism setup via `getSASLMechanism`.
3. **TLS Config parsing**: Configured PEM-based certificate loading for CA pools and client TLS keypairs inline or from disk file paths.
4. **Resilient Read Loop**: Implemented `ReadRecords` with background read routines that safely unmarshal raw message payloads into dynamic record structures, fallback-wrapper schemas when JSON payload unmarshalling fails, and maps partition metadata directly to the record metadata envelope.

---

#### [Kafka Source Connector](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/kafka_source.go)
```go
type KafkaSource struct {
	brokers  []string
	topic    string
	groupID  string
	auth     AuthConfig
	tls      TLSConfig
	settings map[string]string
	reader   *kafka.Reader
	mu       sync.Mutex
}

func NewKafkaSource(settings map[string]string, auth AuthConfig, tls TLSConfig) *KafkaSource { ... }
func (k *KafkaSource) Connect() error { ... }
func (k *KafkaSource) ReadRecords(ctx context.Context) <-chan Record { ... }
func (k *KafkaSource) Close() error { ... }
```

---

### Decision 12: Kafka Consumer Group Management, Offset Reset, and At-Least-Once Commit Strategy

**Context:** Enterprise Kafka deployments rely on consumer groups for horizontal scaling and partition assignment. The source connector must support configurable consumer group IDs, offset reset behavior for new or reset groups, and commit interval tuning to guarantee at-least-once delivery semantics.

**Decision:**
Extended `KafkaSource.Connect()` to configure three core consumer behaviors:
1. **Consumer Group ID**: When `settings["group_id"]` is set, the `kafka.ReaderConfig.GroupID` is populated. This enables Kafka's built-in partition assignment protocol, allowing multiple pipeline instances to share work across topic partitions automatically.
2. **Auto Offset Reset**: When `settings["auto_offset_reset"]` is `"earliest"`, new consumer groups begin reading from the very first message in the partition (`kafka.FirstOffset`). Otherwise defaults to `kafka.LastOffset` (latest), reading only new messages.
3. **Configurable Commit Interval**: `settings["commit_interval"]` accepts Go duration strings (e.g. `"1s"`, `"500ms"`). Parsed via `time.ParseDuration` and mapped to `kafka.ReaderConfig.CommitInterval`. Shorter intervals trade throughput for lower reprocessing windows on crash; longer intervals reduce broker commit overhead.

---

### Decision 13: Dynamic Avro and Protobuf Deserialization via Confluent Schema Registry

**Context:** High-volume enterprise Kafka deployments transmit data in compact binary formats (Avro, Protobuf) rather than human-readable JSON. These formats strip field names from the wire payload to minimize bandwidth. To reconstruct the data, the pipeline must fetch the corresponding schema definition ("blueprint") from a central Confluent Schema Registry at runtime.

**Decision:**
Introduced a dedicated `pipeline/decoder` package with a modular decoder architecture:
1. **Decoder Interface** ([decoder.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/decoder/decoder.go)): Universal strategy interface `Decode(schema string, payload []byte) (map[string]any, error)` allowing format-agnostic pipeline integration.
2. **Schema Registry Client** ([schema_registry.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/decoder/schema_registry.go)): Thread-safe LRU-caching HTTP wrapper around `github.com/riferrei/srclient`. Supports Basic Auth credentials (`schema_registry_user`, `schema_registry_password`) and optional TLS skip-verify for development environments. Schemas are fetched once by ID and cached in memory for the lifetime of the pipeline.
3. **Avro Decoder** ([avro_decoder.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/decoder/avro_decoder.go)): Uses `github.com/hamba/avro/v2` to dynamically parse JSON-based Avro schema strings and unmarshal compact binary payloads directly into `map[string]any`. Parsed schemas are cached via `sync.Map` to avoid recompilation on every message.
4. **Protobuf Decoder** ([protobuf_decoder.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/decoder/protobuf_decoder.go)): Uses `github.com/jhump/protoreflect` to parse raw `.proto` schema definitions at runtime, construct dynamic message descriptors, unmarshal binary payloads, and convert to JSON→map without requiring ahead-of-time code generation. Descriptors are cached via `sync.Map`.
5. **Wire Format Extraction**: `KafkaSource.ReadRecords` inspects incoming messages for Schema IDs via two mechanisms:
   - Explicit Kafka header lookup (`schema_id` header key).
   - Confluent Wire Format detection (magic byte `0x00` + 4-byte Big Endian Schema ID prefix).
   Once an ID is resolved, the schema type (AVRO or PROTOBUF) determines which decoder is invoked. If no schema is detected, the pipeline falls back to standard JSON unmarshalling, and if that also fails, wraps the raw bytes in a `_raw_bytes` field to guarantee zero data loss.

---

#### [Decoder Interface](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/decoder/decoder.go)
```go
type Decoder interface {
    Decode(schema string, payload []byte) (map[string]any, error)
}
```

#### [Schema Registry Client](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/decoder/schema_registry.go)
```go
type SchemaRegistryClient struct {
    client *srclient.SchemaRegistryClient
}

func NewSchemaRegistryClient(url, username, password string, skipVerify bool) *SchemaRegistryClient { ... }
func (s *SchemaRegistryClient) GetSchema(id int) (*srclient.Schema, error) { ... }
```

#### [Updated Kafka Source Connector](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/source/kafka_source.go)
```go
type KafkaSource struct {
    brokers  []string
    topic    string
    groupID  string
    auth     config.AuthConfig
    tls      config.TLSConfig
    settings map[string]string
    readers   []*kafka.Reader
    dialer    *kafka.Dialer
    mu        sync.Mutex
    srClient  *decoder.SchemaRegistryClient
    avroDec   decoder.Decoder
    protoDec  decoder.Decoder
    dlqWriter *DLQWriter
}

func NewKafkaSource(settings map[string]string, auth config.AuthConfig, tls config.TLSConfig) *KafkaSource { ... }
func (k *KafkaSource) Connect() error { ... }
func (k *KafkaSource) ReadRecords(ctx context.Context) <-chan pipeline.Record { ... }
func (k *KafkaSource) Close() error { ... }
func (k *KafkaSource) DLQ() *DLQWriter { ... }
```

---

### Decision 14: Multi-Topic Kafka Consumption via Fan-In Reader Architecture

**Context:** Enterprise Kafka deployments frequently model domain events across multiple topics (e.g. `orders`, `payments`, `user-events`). Forcing operators to deploy a separate pipeline per topic creates unnecessary operational overhead and duplicated configuration. A single pipeline should be able to subscribe to multiple topics concurrently.

**Decision:**
Refactored `KafkaSource` to accept a comma-separated `topics` setting alongside the existing singular `topic` key (backward-compatible fallback):
1. **Consumer Group Mode (recommended):** When `group_id` is set and multiple topics are provided, `kafka.ReaderConfig.GroupTopics` is used. Kafka's built-in consumer group protocol handles partition assignment across all topics in a single reader, supporting horizontal scaling natively.
2. **No Consumer Group Mode:** When no `group_id` is set, `KafkaSource` spawns one independent `kafka.Reader` per topic, each running in its own goroutine. All goroutines fan-in to a shared output channel coordinated by `sync.WaitGroup`. The channel is only closed after all readers have exited.
3. **Topic Precedence:** The `topics` key takes precedence over `topic`. If `topics` is set, `topic` is ignored. This prevents ambiguous configuration states.
4. **Commit Interval Extraction:** The commit interval parsing was extracted into a reusable `applyCommitInterval(*kafka.ReaderConfig)` helper to avoid duplication across reader construction paths.

---

### Decision 15: Dead-Letter Queue (DLQ) Topic for Failed Record Routing

**Context:** In a fail-closed pipeline, records that fail policy evaluation, are explicitly denied, or encounter transformation errors are silently dropped. While this is correct from a security standpoint, it creates an observability blind spot — operators cannot inspect or replay failed records. A dead-letter queue captures these records for post-mortem analysis and potential reprocessing.

**Decision:**
Implemented a two-component DLQ architecture:

1. **DLQ Writer** ([dlq_writer.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/source/dlq_writer.go)): A lightweight Kafka producer wrapping `kafka.Writer`. Serializes the original record's `Data` as JSON and attaches failure context as Kafka headers:
   - `dlq.reason` — Human-readable failure description (e.g. `policy_deny: [pci-dss-rule]`).
   - `dlq.original_id`, `dlq.original_source` — Record identity provenance.
   - `dlq.original_topic`, `dlq.original_partition`, `dlq.original_offset` — Original Kafka transport coordinates for replay correlation.
   The writer reuses the same SASL/TLS transport configuration as the source connector, initialized during `KafkaSource.Connect()` when `settings["dead_letter_topic"]` is present.

2. **Pipeline DLQ Integration** ([pipeline.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/pipeline.go)): Added a `DLQSink` interface and optional `DLQ` field to the `Pipeline` struct. In `processRecord`, when a record is dropped (`shouldWrite == false`) and a DLQ is configured, the original untransformed record is routed to the DLQ with a structured reason string. A `DLQCount` atomic counter was added to `Metrics` and exposed in `MetricsSnapshot` for operational monitoring.

**DLQ Routing Triggers:**
| Failure Scenario | Reason String |
|---|---|
| Policy evaluation error (fail-closed) | `policy_evaluation_error: <err>` |
| Explicit policy deny | `policy_deny: [policy-names]` |
| Transformation error (fail-closed) | `transform_error: <err>` |
| No matching policy | `no_matching_policy` |

---

#### [DLQSink Interface](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/pipeline.go)
```go
type DLQSink interface {
    Write(ctx context.Context, record Record, reason string) error
    Close() error
}
```

#### [DLQ Writer](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/source/dlq_writer.go)
```go
type DLQWriter struct {
    writer *kafka.Writer
    mu     sync.Mutex
}

func NewDLQWriter(brokers []string, topic string, dialer *kafka.Dialer) *DLQWriter { ... }
func (d *DLQWriter) Write(ctx context.Context, record pipeline.Record, reason string) error { ... }
func (d *DLQWriter) Close() error { ... }
```

#### [Updated Pipeline Struct](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/pipeline.go)
```go
type Pipeline struct {
    Name            string
    Source          Source
    PolicyEngine    PolicyEngine
    TransformEngine TransformEngine
    Destination     Destination
    DLQ             DLQSink
    AuditEngine     AuditEngine
    FailClosed      bool
    DryRun          bool
    metrics         Metrics
}
```

---

### Decision 16: Exactly-Once Semantics via Explicit Commit Callback

**Context:** The standard at-least-once offset commit strategy auto-commits offsets periodically on read. However, if a failure occurs during pipeline processing, transformation, or destination writing, messages can be lost or skipped. To prevent data loss and ensure exactly-once processing (or strictly bounded at-least-once delivery without pre-commit loss), offsets must only be committed to Kafka *after* the record has been fully and successfully written to the destination, routed to a DLQ, or dropped by policy.

**Decision:**
Implemented an explicit, callback-driven offset commit mechanism:
1. **Disabled Auto-Commit:** When `settings["delivery_semantics"]` is set to `"exactly_once"`, the Kafka reader configures `CommitInterval = 0`, disabling automatic offset commits.
2. **FetchMessage Read Loop:** The source connector switches from `reader.ReadMessage(ctx)` (which auto-commits) to `reader.FetchMessage(ctx)` to fetch messages from Kafka without committing offsets.
3. **Commit Callback (`AckFunc`):** Added a non-serialized `AckFunc func() error` callback to the `Record` struct. The `KafkaSource` populates this callback with a closure that captures the message details and reader. When invoked, it executes `reader.CommitMessages(commitCtx, msg)` with a dedicated 5-second context timeout.
4. **Pipeline Execution Integration:** In `pipeline.processRecord`, after a successful destination write, dry-run simulation, successful DLQ routing, or explicit policy silent drop, the pipeline executes `record.AckFunc()` to securely commit the offset in Kafka. If a destination or DLQ write fails, the callback is skipped, allowing the message to be safely retried.

---

### Decision 17: Decoupled Ingestion Telemetry via Generalized Metrics Interface

**Context:** Managing streaming ingestion pipelines requires real-time operational visibility (e.g. consumer lag). However, directly coupling core connectors like `KafkaSource` to concrete monitoring libraries (such as Prometheus) limits portability, complicates testing, and forces a specific metrics suite upon administrators.

**Decision:**
Implemented a decoupled, pluggable metrics architecture:
1. **Generalized Metrics Interface:** Introduced the `MetricsReporter` interface in the `pipeline` package defining `ReportLag(topic, partition, groupID string, lag int64)` and `Close() error`. Created a default `NoOpReporter` that safely discards metrics.
2. **Pluggable Prometheus Reporter:** Moved the Prometheus client dependency, the registered `prometheus.GaugeVec` for lag, and the HTTP server management (`/metrics` endpoint) into `PrometheusReporter` inside the `pipeline/source` package.
3. **Declarative Activation:** `KafkaSource` accepts the metrics provider dynamically from configuration settings (e.g. `metrics_provider: "prometheus"` and `metrics_port: "9090"`). When disabled, it defaults to the `NoOpReporter` to eliminate TCP listening overhead and metrics collection.
4. **Decoupled Polling:** Background polling routines and read loops in `KafkaSource` invoke `metrics.ReportLag(...)` without knowing or care about how it is collected, formatted, or exposed.

---

### Reference Code

#### [Metrics Reporter Interface](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/metrics.go)
```go
type MetricsReporter interface {
	ReportLag(topic, partition, groupID string, lag int64)
	Close() error
}
```

#### [Prometheus Reporter Exporter](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/source/prometheus_reporter.go)
```go
type PrometheusReporter struct {
	server *http.Server
	mu     sync.Mutex
}

func NewPrometheusReporter(port string) *PrometheusReporter { ... }
func (p *PrometheusReporter) ReportLag(topic, partition, groupID string, lag int64) { ... }
func (p *PrometheusReporter) Close() error { ... }
```

---

### Decision 18: Source-Aware Policy Matching for Streaming Ingestion Pipelines

**Context:** While standard HTTP proxy requests evaluate policies based on endpoint path resources (e.g. `/users`), streaming ingestion pipelines read records from distinct message brokers and topics. A single gateway or pipeline registry may process records from multiple event queues. Enforcing standard policies requires matching based on the message source (e.g. `kafka.customer-events`), matching wildcards (`kafka.*`), or matching specific function signatures (`kafka.topic("customer-events")`).

**Decision:**
Implemented source-aware policy compilation and matching logic in the `policy` package:
1. **Schema Extension:** Added a list of `sources` to the `Match` block configuration and propagated it into compiled `EffectivePolicy` structures.
2. **Dynamic Source Parser:** Created `matchSource(pattern, source string)` using RE2 regex to parse specialized functions (like `kafka.topic("customer-events")` or single-quoted/unquoted variants) and compile them to canonical glob paths (e.g. `kafka.customer-events`).
3. **Pluggable Lookup Resolution:** Updated `Store.Lookup(role, resource, service, source string)` to filter compiled rules by matching the target `source`. If `sources` constraint is present on the policy, it matches against the incoming context.
4. **Context Propagation:** Updated `types.RequestContext` to hold `Source` field, enabling both HTTP gateways and pipeline handlers to pass source information during record transformation lookups.

---

### Decision 19: Destination-Aware Policy Matching

**Context:** Paralleling the logic for Source-Aware matching, pipelines also need to restrict or transform records based on where they are being routed (e.g. `vector_db.index("my-index")` vs `postgres.table("users")`). Ensuring policies can be applied selectively on the egress side prevents unintended cross-destination data exposure.

**Decision:**
Implemented destination-aware policy matching logic leveraging the same patterns built for `Sources`:
1. **Schema Extension:** Added a list of `destinations` to the `Match` block configuration and `EffectivePolicy` structures.
2. **Generic Destination Parser:** Created `matchDestination(pattern, destination string)` utilizing a generic regex `^([a-zA-Z0-9_]+)\.[a-zA-Z0-9_]+\((.*)\)$` to parse functions like `vector_db.index("my-index")` and translate them into wildcard glob patterns (`vector_db.my-index`), supporting flexible destination types without hardcoding connector names.
3. **Lookup Resolution:** Extended `Store.Lookup(role, resource, service, source, destination string)` to filter policies by target destination.
4. **Context Propagation:** Updated `types.RequestContext` to hold the `Destination` field, propagating egress targets securely into the core evaluation loops.

---

### Decision 20: Tag-Based Field Matching

**Context:** Hardcoding dot-notation field paths (`user.email`, `transaction.card`) is brittle across sprawling schemas and dynamic record types. Data teams commonly use Schema Registries (like Confluent) or metadata catalogs that tag fields conceptually. Policies need to target fields based on these tags rather than strict schema paths.

**Decision:**
Implemented dynamic tag-based matching inside the policy transformation execution loop.
1. **Schema Update:** Expanded the flat v1 `Rule` syntax to include `fields_tagged: []string`. This allows fine-grained tag targeting mapped to distinct actions directly in the policy YAML.
   ```yaml
   rules:
     - fields_tagged: ["@pii", "@phi"]
       action: mask
     - fields_tagged: ["@financial"]
       action: remove
   ```
2. **Allowed Tags (Dynamic):** There is **no hardcoded list of allowed tags**. The engine treats tags as arbitrary string identifiers (e.g., `@pii`, `RESTRICTED`, `internal_only`). This guarantees forward compatibility with any external cataloging system.
3. **Context Extension & Assignment:** Added a `FieldTags map[string][]string` metadata field to `pipeline.Record` and `types.RequestContext`. The assignment of tags to fields happens dynamically at runtime — the HTTP proxy or pipeline decoders (e.g., Avro Schema Registry) parse the incoming metadata and inject the mapping (e.g., `{"user.email": ["@pii"]}`) into the context before executing the transform.
4. **Compilation & Resolution:** `policy.EffectivePolicy` tracks `TaggedRules`. During runtime, `transformRecord` uses a `resolveRules` helper to intersect explicit field path rules against matching tag rules dynamically.
5. **Conflict Resolution:** If a field has multiple tags mapped to different actions, the engine strictly prioritizes the most restrictive action (`Remove > Deny > Redact > Mask > Truncate > Anonymize > Allow`). Explicit field path rules (`field: "user.email"`) always take precedence over dynamic tag rules.

---

### Decision 21: Trigger Context Separation (`on: ingest` vs `on: read`)

**Context:** Policies previously applied globally across all evaluations matching a role and resource. However, policies intended for edge proxy enforcement (e.g., read masking) might unintentionally trigger in background ingestion pipelines evaluating the same schema/resource.

**Decision:**
Added trigger context separation to ensure policies are isolated to specific execution contexts:
1. **Configuration:** Added `on: []string` field to the YAML policy schema (e.g., `on: ["ingest"]`).
2. **Backward Compatibility:** If `on` is omitted or empty, it gracefully defaults to `["read"]` during evaluation to preserve existing proxy behavior.
3. **Execution Routing:** The `Store.Lookup()` method now requires a `trigger` argument (`"read"` or `"ingest"`).
4. **Integration:** Updated `proxy/handler.go` to evaluate policies with the `"read"` trigger, while `pipeline/policy_engine.go` evaluates policies under the `"ingest"` trigger.

---

### Decision 22: Pipeline Level Policy Caching

**Context:** The pipeline evaluated policies dynamically (`Store.Lookup`) for every single message. In high-throughput ingestion environments, repeated regex matching and prefix lookups for records sharing the same schema, role, and source resulted in significant processing overhead.

**Decision:**
Introduced a `DefaultPolicyEngine` inside the `pipeline` package incorporating thread-safe local caching.
1. **Mechanism:** Utilizes `sync.RWMutex` backed by a map to memoize evaluated policies.
2. **Cache Key:** Derived deterministically from the combination of `Role::Resource::Source::Trigger`.
3. **Behavior:** Eliminates redundant rule compilations by immediately returning the cached `*EffectivePolicy` on subsequent hits for identical schema flows.

---

### Decision 23: Audit Log Enrichment with Evaluated Conditions

**Context:** Compliance teams require visibility into why dynamic row-level rules fired or were skipped (e.g., verifying if a field was masked because `age < 18` evaluated to true).

**Decision:**
Extended the audit infrastructure to natively serialize condition evaluation results.
1. **Capture:** The transformation engine automatically tracks condition expressions (`summary.ConditionEvaluated`).
2. **Persistence:** Mapped this property natively to `audit.Entry.ConditionEvaluated`. Both HTTP Proxy and Streaming Pipelines now emit deterministic debug summaries showing exact condition rule outcomes in the audit log for security tracing.

---

### Decision 24: Compile-Time Overlap Detection

**Context:** Overly rigid constraints caused the engine to throw a fatal error when two policies shared identical priority levels and role/resource patterns, even if they operated on distinct JSON fields.

**Decision:**
Relaxed the hard failure into an informative `slog.Warn`.
1. **Validation Re-design:** Modified `policy/loader.go`'s `compile()` step. It now dynamically computes intersections using `findFieldOverlaps()`.
2. **Behavior:** The engine will successfully compile policies with identical priorities. However, if they target the exact same field, it emits a specific warning so operators are aware that execution order defaults to internal map sorting.
