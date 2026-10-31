# Auvis Audit Logging & Compliance Guide

Auvis provides **asynchronous, non-blocking compliance audit logging** designed for HIPAA Access Controls (§ 164.312(b)), GDPR Article 30 records, and PCI-DSS data tracking.

Every HTTP request and streaming record passing through Auvis generates an immutable audit record in newline-delimited JSON (`audit.jsonl`), capturing identity, policies applied, and field-level disposition.

---

## 📄 Audit Record Schema (`audit.jsonl`)

Each line in `audit.jsonl` is a complete, structured JSON object:

```json
{
  "timestamp": "2026-10-30T14:22:10.450Z",
  "request_id": "req-1730301730450",
  "user_id": "usr_9921",
  "user_role": "analyst",
  "resource": "/api/v1/patients",
  "method": "GET",
  "policies_applied": ["hipaa-analyst-mask.yaml"],
  "priority": 100,
  "compliance_tags": ["HIPAA", "GDPR"],
  "fields_masked": ["email", "phone", "date_of_birth"],
  "fields_removed": ["ssn", "insurance_id"],
  "fields_allowed": ["patient_id", "status"],
  "fields_denied": [],
  "response_denied": false,
  "upstream_status": 200,
  "duration_ms": 1,
  "destination_written": true,
  "destinations_written": 2,
  "destinations_failed": 0,
  "destination_ms": 4,
  "destination_results": [
    {
      "type": "postgres",
      "written": true,
      "duration_ms": 2
    },
    {
      "type": "pgvector",
      "written": true,
      "duration_ms": 2
    }
  ]
}
```

### Schema Reference Table

| JSON Key | Type | Description |
|---|---|---|
| `timestamp` | `string` | ISO 8601 UTC timestamp of request execution |
| `request_id` | `string` | Unique request correlation identifier (`req-<timestamp>`) |
| `user_id` | `string` | Authenticated user identity (from JWT, IdP, or `X-User-ID` header) |
| `user_role` | `string` | Security role (e.g. `analyst`, `doctor`, `auditor`, `admin`) |
| `resource` | `string` | Request path prefix or streaming topic name |
| `method` | `string` | HTTP method (`GET`, `POST`) or `INGEST` for stream records |
| `policies_applied` | `[]string` | Names of policy files that evaluated against this request |
| `compliance_tags` | `[]string` | Framework tags (`HIPAA`, `GDPR`, `PCI-DSS`, `CCPA`, `SOX`) |
| `fields_masked` | `[]string` | Fields masked, pseudonymized, redacted, or truncated |
| `fields_removed` | `[]string` | Fields completely stripped from the response |
| `fields_allowed` | `[]string` | Fields explicitly permitted by policy |
| `fields_denied` | `[]string` | Fields blocked by an explicit field-level deny rule |
| `response_denied` | `bool` | `true` if the entire response was rejected (HTTP 403) |
| `rows_filtered` | `int` | Number of array rows dropped by row-level `filter_condition` |
| `upstream_status` | `int` | HTTP status code returned by the upstream service |
| `duration_ms` | `int` | Total gateway processing latency in milliseconds |
| `destination_written` | `bool` | `true` if all configured storage sinks confirmed write |
| `destination_results` | `[]object` | Breakdown of individual write outcomes per sink driver |

---

## ⚡ Disk Durability & Fsync Modes

Auvis handles audit logging through an asynchronous channel to keep request latency sub-millisecond. You can configure disk durability in `auvis.yaml` via `gateway.audit_fsync_interval`:

```yaml
gateway:
  audit_file: ./audit.jsonl
  audit_max_size_mb: 100
  audit_max_age_days: 30
  audit_fsync_interval: "never"  # "never" | "always" | duration (e.g. "100ms", "5s")
```

| Mode | Durability Guarantee | Throughput | Use Case |
|---|---|---|---|
| `"never"` *(Default)* | Flushed on process shutdown; OS buffers writes. | **~6,500+ req/sec** | High-throughput APIs, development, non-critical logging. |
| `"always"` | Synchronous `fsync` after every single entry write. | **~1,700 req/sec** | Strict zero-data-loss compliance environments. |
| `"100ms"` / `"1s"` / `"5s"` | Background timer calls `fsync` periodically. | **~6,400+ req/sec** | **Recommended for production**: Balances near-zero data loss window with line-rate performance. |

---

## 🔄 Automatic Rotation & Retention

Auvis automatically rotates audit logs to prevent disk exhaustion:

* **Size-Based Rotation (`audit_max_size_mb`)**: When `audit.jsonl` reaches the specified size (default `100MB`), Auvis renames it to `audit-YYYYMMDD-HHMMSS.jsonl` and starts a fresh log file.
* **Age-Based Retention (`audit_max_age_days`)**: Rotated log files older than the retention limit (default `30` days) are automatically deleted.

---

## 🔍 Querying Audit Logs

### 1. Via CLI (`auvis logs`)
```bash
# Tail in real-time
auvis logs -f

# Filter by role and limit
auvis logs --role doctor --limit 25
```

### 2. Via HTTP REST API (`/auvis/audit`)
Auvis exposes a built-in query endpoint for SIEM tools and compliance dashboards:

```bash
GET /auvis/audit?role=analyst&resource=/patients&limit=50
```

#### Supported Query Parameters:
* `limit` (int): Number of entries to return (default: `50`).
* `role` (string): Filter by authenticated role.
* `user` (string): Filter by user ID.
* `resource` (string): Filter by resource path prefix.

### 3. Real-Time Server-Sent Events (`/auvis/stream`)
Stream live audit events to web dashboards or monitoring agents via SSE:

```bash
curl -N http://localhost:8082/auvis/stream
```

---

## 🛡️ Compliance Framework Mapping

### HIPAA Security Rule (§ 164.312(b))
* **Requirement**: Audit controls to record and examine activity in information systems that contain or use electronic protected health information (ePHI).
* **Auvis Mapping**: Logs user ID, timestamp, patient resource, exact fields masked vs removed, and destination vector write status.

### GDPR Article 30 (Records of Processing Activities)
* **Requirement**: Controller records demonstrating categories of processing, recipients, and security safeguards.
* **Auvis Mapping**: Every entry captures `compliance_tags: ["GDPR"]`, user role, and whether data was pseudonymized, masked, or withheld.
