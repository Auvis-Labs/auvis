# Auvis — Version 0.2.5 Architectural Decisions

> Living document. Updated as build decisions are made for version 0.2.5.
> Start date: June 22, 2026

## Version History

| Version | Date | What it covers |
|---|---|---|
| v0.2.5 | June 22, 2026 | Added Genuinely Synchronous Postgres Destination Connector (PostgresDestination) |
| v0.2.5 | June 27, 2026 | Fixed policy merging/allow overrides bug & added dedicated gateway /health endpoint |
| v0.2.5 | June 27, 2026 | Renamed Go module path to github.com/Auvis-Labs/auvis, updated codebase imports, and resolved version tags conflict |
| v0.2.5 | June 28, 2026 | Implemented multi-destination fan-out (Postgres + pgvector/Pinecone/Weaviate simultaneously) |
| v0.2.5 | June 29, 2026 | Added default upstream configuration key and proxy header fallback |
| v0.2.5 | June 29, 2026 | Implemented per-record audit outcomes for compliance-grade auditing |

---

## Architectural Decisions

### Decision 1: Genuinely Synchronous Postgres Destination Connector (PostgresDestination)

**Context:** Auvis pipelines process dynamic streams of records and need to write them to relational database sinks. To comply with real-time error handling mechanisms (such as fail-open and fail-closed policies on individual records), database writes must be immediate, blocking, and return synchronous feedback.

**Decision:**
Implemented `PostgresDestination` under the `pipeline/destination` package:
1. **Connection Pool Management**: Connects to Postgres using `pgxpool.Pool` for thread-safe concurrent connection pooling.
2. **Dynamic Connection Setup**: Connection parameters are built dynamically from connector settings (`host`, `port`, `dbname`, `sslmode`) and credentials, supporting fallback connection string overrides.
3. **Custom TLS secure transport**: Supports client/server certificate mapping and verification settings matching the core TLS configuration engine.
4. **Synchronous Writes**: `WriteRecord` directly serializes the record's payload and metadata maps to JSONB bytes and executes an immediate SQL `INSERT` query. Error conditions (e.g. database disconnect, constraint violation) are propagated back to the pipeline worker synchronously for immediate policy/flow routing.
5. **No-Op Buffering**: Employs no memory caching or thread-safe batch buffering to prioritize transactional reliability and real-time failure auditing. `Flush` behaves as a no-op returning `nil`.

---

## Reference Code

### [Postgres Destination Connector](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pipeline/destination/postgres_destination.go)
```go
type PostgresDestination struct {
	connStr    string
	tableName  string
	autoCreate bool
	auth       config.AuthConfig
	tls        config.TLSConfig
	pool       *pgxpool.Pool
}

func NewPostgresDestination(settings map[string]string, auth config.AuthConfig, tls config.TLSConfig) *PostgresDestination { ... }
func (p *PostgresDestination) Connect() error { ... }
func (p *PostgresDestination) WriteRecord(record pipeline.Record) error { ... }
func (p *PostgresDestination) Flush() error { ... }
func (p *PostgresDestination) Close() error { ... }
```

---

### Decision 2: Proxy Handler Destination Fan-Out (Synchronous, Fail-Closed)

**Context:** After governing a response through the transform/policy engine, the proxy must synchronously record the masked payload to the configured destination (Postgres) before returning data to the client. Option A, locked in during architectural review, mandates that no governed data escapes Auvis without being durably recorded.

**Decision:**
Wired `PostgresDestination` into `proxy.Handler()` as STEP 7.5 — between transform and the HTTP response write:

1. **Handler signature extended:** `proxy.Handler(...)` accepts an optional `pipeline.Destination` (nil = no destination, zero overhead on unconfigured deployments).
2. **Fan-out point:** After transform, `buf.Bytes()` is parsed into `map[string]any` (queryable JSONB) and wrapped in a `pipeline.Record`. Written via `dest.WriteRecord()` before `w.Write()`.
3. **Fail-closed semantics:** If `WriteRecord` errors, an audit entry is written and the handler returns without sending the response. Governed data never reaches the client without a durable write.
4. **Structured JSONB data column:** Response body is parsed as JSON so fields are queryable in Postgres. Non-JSON bodies fall back to `{"body": "..."}`.
5. **Metadata JSONB column:** Stores `user_id`, `user_role`, `method`, `upstream_status`, `fields_masked`, `policies_applied`.
6. **Audit tracking:** `audit.Entry` extended with `destination_written: bool` and `destination_ms: int64`.
7. **Gateway config:** `config.Config.Destination` loaded from `auvis.yaml`; `cmd/dev.go` conditionally instantiates + connects the destination at startup.

**Files changed:**
- `config/config.go` — `DestinationConfig` struct + YAML parsing
- `auvis.yaml` — commented destination template block
- `proxy/handler.go` — STEP 7.5 fan-out, new `dest pipeline.Destination` param
- `audit/types.go` — `DestinationWritten`, `DestinationMs`
- `cmd/dev.go` — conditional destination init + graceful `defer Close()`

**Tests added:**
- `proxy/handler_integration_test.go` — E2E: PII request → mask → Postgres row verified; audit `destination_written: true` confirmed. Guarded by `POSTGRES_TEST_DSN`.
- `proxy/handler_bench_test.go` — Latency benchmark targeting p99 <20ms on local Postgres.

---

### Decision 3: Policy Merging Override Resolution & Dedicated Health Check

**Context:** 
1. Wildcard `*` fields are omitted from compiled policy `ep.Rules` because they represent policy-level actions (allow/deny) rather than field-level transforms. When a role policy only specifies `field: "*", action: allow` (such as full access for `admin`), `len(ep.Rules)` results in `0`. 
2. The compilation merge loop in `Store.Lookup` previously only cleared a previous policy's `deny` status if `len(ep.Rules) > 0`. This caused explicit role allow rules to fail to override catch-all wildcard deny rules (e.g. `roles: ["*"], action: deny`), leading to unintended `403 Forbidden` errors for allowed roles.
3. System operators also needed a lightweight gateway-level health check endpoint to check online status that did not require identity headers (`X-User-ID`, `X-User-Role`) or proxying.

**Decision:**
1. **Precedence Engine Fix:** Modified `Store.Lookup` to reset the merged policy's `Deny` status to `false` when a matched explicit policy with `ep.Deny == false` is processed, ensuring that explicit role allow policies successfully override wildcard deny rules regardless of the number of field-level rules.
2. **Dedicated Health Endpoint:** Registered a `/health` endpoint directly on the custom serve multiplexer (`mux := http.NewServeMux()`) inside `cmd/dev.go` which returns `{"status": "ok", "service": "auvis"}` without evaluating request headers or attempting to forward to the upstream server.

**Files changed:**
- `policy/loader.go` — Reset `merged.Deny` based on matching policy `Deny` state.
- `cmd/dev.go` — Register a `/health` check handler directly on the serve multiplexer.

---

### Decision 4: Repository Renaming, Module Path Refactoring, and Binary Installation Guidance

**Context:** 
1. The project repository was renamed from `auvis-infra` to `auvis` on GitHub. 
2. To align the Go module and package structure with the new repository name, the module path in `go.mod` needed to be updated to `github.com/Auvis-Labs/auvis`. All internal package import path strings in Go files needed to be updated accordingly.
3. The Go module proxy cached the old module path for tag versions (like `v0.1.0`), creating version constraints conflicts when users tried to download the new path.
4. Additionally, installing a Go binary via `go install` writes to `$GOPATH/bin` by default, which may not be in the user's path, requiring explicit shell path exports or distribution alternatives.

**Decision:**
1. **Go Module Path Rename:** Renamed the module to `github.com/Auvis-Labs/auvis` in `go.mod` and updated import reference paths in all 36 `.go` files across the codebase.
2. **Release Version Tagging Resolution:** Created version tags (like `v0.1.5` and `v0.2.5`) on the newly renamed repo to overwrite version constraint cache conflicts on the Go module proxy (`proxy.golang.org`).
3. **Installation / Distribution Guidance:** Kept Go's standard binary installation path behavior but updated the documentation to recommend adding `$GOPATH/bin` to the system's `$PATH` via `~/.zshrc`, or compiling globally via `GOBIN=/usr/local/bin`. Plan to publish a Homebrew Formula or shell script installer for future production releases.

**Files changed:**
- `go.mod` — Declared module path update.
- 36 `.go` source files updated with revised import path.
- `README.md` — Updated with Go path instructions.

---

### Decision 5: Multi-Destination Fan-Out

**Context:**
Auvis's architecture doc described fan-out to multiple destinations but the code only ever held a single `pipeline.Destination`. A healthcare use case requires governed records to land in both Postgres (structured query store) and pgvector (similarity search) after every proxied response.

**Decision:**

1. **`MultiDestination` wrapper** (`pipeline/destination/multi_destination.go`): A new struct that implements `pipeline.Destination` and iterates over a slice of sub-destinations, calling `WriteRecord` on each. Returns a `*MultiError` carrying per-destination outcomes (`DestWriteResult`) if any write fails. No changes required to `proxy.Handler()` — the handler already accepts a `pipeline.Destination` interface.

2. **`MultiError` type**: Exposes `Results []DestWriteResult` so callers can unpack per-destination success/failure detail without re-querying the destinations.

3. **Config update** (`config/config.go`): `Config.Destination DestinationConfig` replaced by `Config.Destinations []DestinationConfig`. YAML parsing supports both the new `destinations:` list and the legacy singular `destination:` key for backward compatibility.

4. **`auvis.yaml`**: Updated example blocks to show a `destinations:` list with Postgres + pgvector side-by-side.

5. **`cmd/dev.go`**: Replaced the `if/else if` single-destination block with `buildDestinations()` (loop over configs) and a `buildDestination()` factory. A single destination is wired directly (no wrapper overhead). Multiple destinations are wrapped in `MultiDestination`.

6. **Audit log** (`audit/types.go`): Added `DestinationsWritten int`, `DestinationsFailed int`, `DestinationResults []DestResult`. The old `DestinationWritten bool` is kept for backward compatibility (computed as `destinations_written > 0`).

7. **`proxy/handler.go`** STEP 7.5: Updated to unpack `*MultiError` results and populate granular per-destination audit fields.

**Fan-out semantics:** Fail-closed. If any destination write fails, all writes stop being written back to the client (HTTP 500) and a detailed audit entry is emitted showing which destination failed.

**Files changed:**
- `pipeline/destination/multi_destination.go` — [NEW] `MultiDestination` + `MultiError` + `DestWriteResult`
- `pipeline/destination/multi_destination_test.go` — [NEW] Unit tests (all-success, fail-closed, all-fail, connect fail-fast)
- `config/config.go` — `Destinations []DestinationConfig` + backward-compat YAML parsing
- `auvis.yaml` — Updated destination config examples
- `cmd/dev.go` — `buildDestinations()` loop + `buildDestination()` factory
- `audit/types.go` — Per-destination result fields + `DestResult` struct
- `proxy/handler.go` — STEP 7.5 unpacks `*MultiError` for audit detail

---

### Decision 6: Default Upstream Configuration and Fallback

**Context:**
Previously, Auvis required the `X-Auvis-Upstream` header to be present on every single incoming HTTP request. While this is useful for multi-tenant dynamic routing, it is cumbersome for deployments targeting a single, fixed upstream API (e.g. a specific healthcare endpoint).

**Decision:**
1. **Config Key (`config/config.go`):** Added support for parsing the `upstream` key under the `gateway` block in `auvis.yaml`. This maps directly to `Config.UpstreamURL`.
2. **Fallback Logic (`proxy/handler.go`):** In `STEP 2` of the proxy handler, if the `X-Auvis-Upstream` header is missing, the proxy now automatically falls back to using `cfg.UpstreamURL`. If both the header and the config are empty, the request is rejected with a `400 Bad Request`.

**Files changed:**
- `config/config.go` — Parsed `upstream` from YAML
- `proxy/handler.go` — Implemented fallback to `cfg.UpstreamURL` when header is missing

---

### Decision 7: Per-Record Audit Outcomes

**Context:**
Previously, Auvis logged audit records purely at the HTTP request level. For array-based responses (e.g. `GET /patients` returning 100 rows), the log only summarized what happened *in general* to the fields across the entire batch (e.g. `"fields_masked": ["first_name"]`). For compliance-grade auditing (e.g. HIPAA), this is a defensibility gap: you cannot prove exactly what happened to an individual patient record (e.g. record `PT-047`).

**Decision:**
1. **`RecordOutcome` Structs:** Added `RecordOutcome` structs to both the `transform` and `audit` packages to represent the fate of an individual record:
   ```go
   type RecordOutcome struct {
       RecordID      string   `json:"record_id"`
       FieldsMasked  []string `json:"fields_masked,omitempty"`
       FieldsRemoved []string `json:"fields_removed,omitempty"`
       FieldsAllowed []string `json:"fields_allowed,omitempty"`
       FieldsDenied  []string `json:"fields_denied,omitempty"`
       Filtered      bool     `json:"filtered,omitempty"`
   }
   ```
2. **Transform Layer (`transform/transform.go`):** Extended the `Summary` struct and `transformArray()` to collect these outcomes in parallel with existing aggregates. Added an `extractRecordID()` helper to auto-detect record IDs using common field names (`id`, `patient_id`, etc.) falling back to positional indexes.
3. **Audit Log (`audit/types.go` & `proxy/handler.go`):** Added `RecordCount` and `RecordOutcomes []RecordOutcome` (both `omitempty`) to the audit `Entry` struct. Wired the proxy handler to map and write these outcomes during the request audit phase.
4. **Performance & Size:** The feature maintains the single-write-per-request design (no database or disk write overhead). For single-record endpoints, the `record_outcomes` field is omitted entirely, keeping log sizes compact.

**Files changed:**
- `transform/transform.go` — Added `RecordOutcome`, collected per-record outcomes in `transformArray`, added `extractRecordID` helper
- `transform/advanced_transform_test.go` — Updated test call sites
- `audit/types.go` — Added `RecordCount` and `RecordOutcomes` to `Entry`
- `proxy/handler.go` — STEP 8: Mapped and wrote outcomes to the audit log


