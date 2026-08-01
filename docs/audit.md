# Module Documentation: `audit`

The `audit` module provides asynchronous, non-blocking compliance audit logging with size-based file rotation, age-based log retention cleanup, queryable REST endpoints, and Prometheus metrics scraping.

---

## 📂 File Index

- [`types.go`](#1-typesgo)
- [`logger.go`](#2-loggergo)
- [`auvis.go`](#3-auvisgo)

---

## 1. `types.go`

Defines the data models for every audit record produced by Auvis.

---

### `type Entry struct`

One complete audit record. Written per HTTP request or per pipeline stream record. Each field maps directly to a JSONL line in the audit file.

| Field | Type | JSON key | Description |
|---|---|---|---|
| `Timestamp` | `time.Time` | `timestamp` | UTC timestamp when the record was processed |
| `RequestID` | `string` | `request_id` | Unique request correlation ID (`req-<UnixNano>`) |
| `UserID` | `string` | `user_id` | Authenticated user identity (from header, JWT, or IdP) |
| `UserRole` | `string` | `user_role` | Security role (e.g. `analyst`, `admin`) |
| `Resource` | `string` | `resource` | Target API path (e.g. `/users`) |
| `Method` | `string` | `method` | HTTP method or `INGEST` for pipeline records |
| `PoliciesApplied` | `[]string` | `policies_applied` | Policy file names that matched and ran |
| `Priority` | `int` | `priority` | Highest-priority policy that applied |
| `ComplianceTags` | `[]string` | `compliance_tags` | Frameworks satisfied: `GDPR`, `HIPAA`, `PCI-DSS`, `CCPA`, `SOX` |
| `ConditionEvaluated` | `[]string` | `condition_evaluated` | Policy condition expressions evaluated at runtime |
| `FieldsMasked` | `[]string` | `fields_masked` | Fields transformed (mask / anonymize / redact / truncate) |
| `FieldsRemoved` | `[]string` | `fields_removed` | Fields deleted from the response (`action: remove`) |
| `FieldsAllowed` | `[]string` | `fields_allowed` | Fields explicitly passed through (`action: allow`) |
| `FieldsDenied` | `[]string` | `fields_denied` | Fields blocked by field-level deny rule |
| `ResponseDenied` | `bool` | `response_denied` | `true` when the entire response was blocked (HTTP 403) |
| `PiiTypesDetected` | `[]string` | `pii_types_detected` | PII categories found: `PII_EMAIL`, `PII_SSN`, etc. |
| `RowsFiltered` | `int` | `rows_filtered` | Array rows excluded by `filter_condition` |
| `UpstreamStatus` | `int` | `upstream_status` | HTTP status code returned by upstream target |
| `DurationMS` | `int64` | `duration_ms` | End-to-end processing time in milliseconds |
| `DestinationWritten` | `bool` | `destination_written` | `true` if **all** configured destinations confirmed write (always serialized as `true` or `false`) |
| `DestinationError` | `string` | `destination_error` | Top-level failure reason string when a write fails (e.g. `"pinecone: circuit breaker is open"`) |
| `DestinationsWritten` | `int` | `destinations_written` | Count of destinations that succeeded |
| `DestinationsFailed` | `int` | `destinations_failed` | Count of destinations that failed |
| `DestinationMs` | `int64` | `destination_ms` | Wall-clock time spent in destination fan-out (ms) |
| `DestinationResults` | `[]DestResult` | `destination_results` | Per-destination write outcome breakdown |
| `RecordCount` | `int` | `record_count` | Number of records in an array response |
| `RecordOutcomes` | `[]RecordOutcome` | `record_outcomes` | Per-record field dispositions for array payloads |

> **Note**: `RecordOutcomes` and `DestinationResults` are `omitempty` — omitted entirely for single-record, single-destination responses to keep audit entries compact.

---

### `type DestResult struct`

Captures the outcome of one destination write in a multi-destination fan-out. Populated inside the `Entry.DestinationResults` slice.

| Field | Type | JSON key | Description |
|---|---|---|---|
| `Type` | `string` | `type` | Driver name: `postgres`, `pinecone`, `pgvector`, `weaviate` |
| `Written` | `bool` | `written` | `true` if `WriteRecord` returned nil |
| `ErrorMsg` | `string` | `error` | Error message string if write failed (omitted if empty) |
| `DurationMs` | `int64` | `duration_ms` | Duration of this individual write call (ms) |

---

### `type RecordOutcome struct`

Captures field-level dispositions for a single record within a multi-record JSON array response. Mirrors `transform.RecordOutcome` — kept as a separate type to avoid a circular import between the `audit` and `transform` packages.

| Field | Type | JSON key | Description |
|---|---|---|---|
| `RecordID` | `string` | `record_id` | Record identifier within the array |
| `FieldsMasked` | `[]string` | `fields_masked` | Masked/anonymized/redacted/truncated fields |
| `FieldsRemoved` | `[]string` | `fields_removed` | Removed fields |
| `FieldsAllowed` | `[]string` | `fields_allowed` | Allowed-through fields |
| `FieldsDenied` | `[]string` | `fields_denied` | Field-level denied fields |
| `Filtered` | `bool` | `filtered` | `true` if this record was dropped by a `filter_condition` rule |

---

## 2. `logger.go`

Implements non-blocking asynchronous audit log writing, size-based file rotation, and age-based log retention cleanup.

---

### `type Logger struct`

| Field | Type | Description |
|---|---|---|
| `ch` | `chan Entry` | Buffered channel queue. Capacity set at construction by `bufSize`. |
| `done` | `chan struct{}` | Signals that background goroutine has flushed and exited. |
| `once` | `sync.Once` | Guards `Close()` to prevent double-close panics. |
| `path` | `string` | Log file path, or the literal string `"stdout"`. |
| `maxSize` | `int64` | Maximum file size in bytes before rotation triggers. |
| `maxAgeDays` | `int` | Number of days to retain rotated log files. |
| `fsyncInterval` | `string` | Disk durability mode: `"never"`, `"always"`, or duration like `"100ms"`, `"5s"`. |
| `fsyncDuration` | `time.Duration` | Parsed ticker interval for periodic disk flushes. |

---

### `NewDurabilityLogger(path, bufSize, maxSizeMB, maxAgeDays, fsyncInterval) (*Logger, error)`

Creates a logger with explicit rotation and disk durability configuration.

* **`"never"` (Default):** Flushes on process shutdown (`Close()`); relies on OS buffer management. Maximum throughput (~6,500+ req/sec).
* **`"always"`:** Synchronously calls `f.Sync()` after every entry write for zero data loss (~1,700 req/sec).
* **Flexible Durations (e.g. `"100ms"`, `"500ms"`, `"5s"`, `"1m"`):** Background `time.Ticker` periodically calls `f.Sync()` at the specified interval, preserving high throughput (~6,400+ req/sec) with a guaranteed maximum data loss window.

---

### `NewLogger(path string, bufSize int) (*Logger, error)`

Creates a logger with default rotation values: **10 MB** max file size, **30 days** retention. Delegates to `NewRotatingLogger`.

**Parameters:**
- `path` — File path for the audit log file. Parent directories are created automatically (`os.MkdirAll`).
- `bufSize` — Channel buffer capacity. A value of `512` means up to 512 entries can be queued without blocking.

**Returns:** `(*Logger, error)` — Always returns `nil` error currently; error signature is preserved for future validation.

---

### `NewRotatingLogger(path string, bufSize int, maxSizeMB int, maxAgeDays int) (*Logger, error)`

Creates a logger with explicit rotation and retention configuration. Spawns a background goroutine (`go l.run()`) immediately.

**Parameters:**
- `maxSizeMB` — Maximum log file size in megabytes (`maxSize = maxSizeMB * 1024 * 1024`).
- `maxAgeDays` — Maximum age in days for rotated files before deletion.

---

### `NewStdoutLogger(bufSize int) *Logger`

Creates a logger that writes to `os.Stdout`. Used in development (`auvis dev` with `--verbose`) and test environments. Rotation logic is skipped for stdout paths.

---

### `(l *Logger) Write(e Entry)`

Non-blocking entry enqueue. Uses a `select` with a `default` case:
- If `l.ch` has buffer capacity → pushes `e` immediately.
- If `l.ch` is full → **drops** `e` and emits `slog.Warn("Audit queue channel full — entry dropped", "request_id", e.RequestID)`.

This design ensures the hot request path is never blocked by I/O.

---

### `(l *Logger) Close()`

Flushes all pending audit entries and terminates the background writer goroutine. Protected by `sync.Once` to prevent double-close panics. Closes `l.ch`, which signals `l.run()` to drain remaining entries and close `l.done`.

---

### `(l *Logger) run()` *(private)*

Background goroutine loop. Processes entries from `l.ch` until the channel is closed.

**Startup:**
1. If `path == "stdout"` → uses `os.Stdout`.
2. Otherwise, creates parent directories, opens file with `O_CREATE|O_APPEND|O_WRONLY`, and reads initial `currentSize` from `f.Stat()`.

**Per-Entry Processing:**
1. Marshals `Entry` to indented JSON (`json.MarshalIndent`).
2. Appends two newlines (`\n\n`) to produce human-readable blocks with blank line separators.
3. **Rotation Check**: If `currentSize + len(data) > l.maxSize`:
   - Closes current file.
   - Renames it to `<base>-YYYYMMDD-HHMMSS<ext>` using `os.Rename`.
   - Logs `slog.Info("Audit log rotated successfully", ...)`.
   - Calls `l.cleanupOldLogs()`.
   - Opens a fresh file at the original path. Resets `currentSize = 0`.
4. Writes `data` bytes to file. Tracks `currentSize += len(data)`.

**Shutdown:** When `l.ch` is closed, the `range` loop exits, `slog.Info("Audit logger background writer flushed and terminated")` is emitted, and `close(l.done)` unblocks `Logger.Close()`.

---

### `(l *Logger) cleanupOldLogs()` *(private)*

Scans the log directory for rotated audit files and deletes files older than `maxAgeDays`.

**Logic:**
1. Reads all entries from `filepath.Dir(l.path)`.
2. For each file matching `<base>-*<ext>` naming pattern:
   - Calls `os.Stat()` to get `ModTime`.
   - If `ModTime.Before(time.Now().AddDate(0, 0, -maxAgeDays))` → deletes with `os.Remove`.
   - Logs `slog.Info("Cleaning up expired rotated audit log file", ...)`.

---

## 3. `auvis.go`

Exposes REST management endpoints and Prometheus metrics scraping on the gateway's HTTP ServeMux.

---

### `RegisterHandlers(mux *http.ServeMux, auditPath string, startTime time.Time, requestCounter *int64, store *policy.Store)`

Mounts all internal management endpoints:

| Route | Handler | Description |
|---|---|---|
| `GET /auvis/audit` | `auditHandler` | Query recent audit log entries with filters |
| `GET /auvis/status` | `statusHandler` | Gateway uptime, request count, active policies |
| `GET /metrics` | `promhttp.Handler()` | Prometheus scrape endpoint (standard path) |
| `GET /auvis/metrics` | `promhttp.Handler()` | Prometheus scrape endpoint (namespaced path) |

---

### `auditHandler(auditPath string) http.HandlerFunc` *(private)*

Returns an HTTP handler that reads and filters the audit log file.

**Query Parameters:**
- `?limit=N` — Return last N entries (default: 50).
- `?role=analyst` — Filter by `UserRole`.
- `?user=user-123` — Filter by `UserID`.
- `?resource=/users` — Filter by `Resource`.

**Logic:**
1. Reads entire file at `auditPath` using `os.ReadFile`.
2. If file does not exist → returns `{"entries": [], "total": 0}`.
3. Calls `parseEntries(data)` to decode all audit blocks.
4. Iterates and applies role / user / resource filters.
5. Slices last `limit` entries from the filtered result.
6. Writes JSON response: `{"entries": [...], "total": N, "limit": N, "filters": {...}}`.

---

### `statusHandler(startTime time.Time, requestCounter *int64, store *policy.Store) http.HandlerFunc` *(private)*

Returns gateway runtime health telemetry:

```json
{
  "status": "ok",
  "uptime": "14m23s",
  "started_at": "2026-07-22T04:00:00Z",
  "requests_total": 14728,
  "active_policies": [
    {
      "role": "analyst",
      "resource": "/users",
      "source_policies": ["pii-analyst.yaml"],
      "rules": ["ssn:mask", "email:anonymize"]
    }
  ]
}
```

Calls `store.GetPolicies()` and formats each active `EffectivePolicy` entry via `policy.SplitKey`.

---

### `parseEntries(data []byte) []Entry` *(private)*

Parses the audit file's newline-separated JSON blocks. Entries are separated by double newlines (`\n\n`). Uses `bufio.Scanner` to read line-by-line, accumulating bytes into a `bytes.Buffer`. When a blank line is encountered, the accumulated block is JSON-decoded into an `Entry` and appended to results. The final block (if file doesn't end with a blank line) is decoded after the loop.

---

### `writeJSON(w http.ResponseWriter, status int, v interface{})` *(private)*

Helper that marshals `v` to indented JSON, sets `Content-Type: application/json`, writes the status code, and writes the body. Returns `500` if marshaling fails.
