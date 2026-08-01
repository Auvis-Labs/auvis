# Module Documentation: `config`

The `config` module handles configuration loading for the Auvis gateway: reading `auvis.yaml`, applying environment variable overrides, and exposing a fully resolved `Config` struct to the rest of the application.

---

## 📂 File Index

- [`config.go`](#1-configgo)

---

## 1. `config.go`

### `type DestinationConfig struct`

Configuration for a single destination sink. Used inside `Config.Destinations`.

| Field | YAML key | Type | Description |
|---|---|---|---|
| `Type` | `type` | `string` | Driver name: `"postgres"`, `"pinecone"`, `"weaviate"`, `"pgvector"`. Empty string = destination disabled. |
| `Settings` | `settings` | `map[string]string` | Driver-specific connection parameters (e.g. `host`, `port`, `table`, `index_host`). |
| `Auth` | `auth` | `pipelinecfg.AuthConfig` | Credentials: `username`, `password`, `token`. |
| `TLS` | `tls` | `pipelinecfg.TLSConfig` | TLS options: `ca_cert`, `client_cert`, `client_key`, `skip_verify`. |

---

### `type Config struct`

The fully resolved, flat configuration struct used throughout the application.

| Field | Type | Default | Source (in order of precedence) |
|---|---|---|---|
| `Port` | `string` | `"8080"` | `AUVIS_PORT` env → `gateway.port` YAML → default |
| `UpstreamURL` | `string` | `""` | `AUVIS_UPSTREAM` env → `gateway.upstream` YAML |
| `AuditFile` | `string` | `"./audit.jsonl"` | `AUVIS_AUDIT_FILE` env → `gateway.audit_file` YAML → default |
| `AuditMaxSizeMB` | `int` | `10` | `gateway.audit_max_size_mb` YAML → default |
| `AuditMaxAgeDays` | `int` | `30` | `gateway.audit_max_age_days` YAML → default |
| `PoliciesDir` | `string` | `"./policies"` | `gateway.policies_dir` YAML → default |
| `ShutdownTimeoutSec` | `int` | `30` | `gateway.shutdown_timeout_seconds` YAML → default (30s) |
| `IdpURL` | `string` | `""` | `AUVIS_IDP_URL` env → `gateway.idp_url` YAML |
| `Salt` | `string` | `"default-auvis-salt-value"` | `AUVIS_SALT` env → `gateway.salt` YAML → hardcoded default |
| `Destinations` | `[]DestinationConfig` | `nil` | `gateway.destinations` YAML list → legacy `gateway.destination` singular |

---

### `type yamlConfig struct` *(private)*

Internal struct used exclusively for YAML unmarshalling. Mirrors the `gateway:` block structure.

```yaml
gateway:
  port: 8080
  policies_dir: ./policies
  audit_file: ./audit.jsonl
  audit_max_size_mb: 10
  audit_max_age_days: 30
  idp_url: ""
  salt: ""
  upstream: ""
  destination:      # legacy singular form
    type: postgres
    settings: {}
  destinations:     # preferred list form (takes precedence)
    - type: postgres
      settings: {}
    - type: pinecone
      settings: {}
```

---

### `LoadConfig(path string) Config`

Main configuration loader. Call once at startup in `cmd/dev.go` before constructing any services.

**Steps:**

1. **`.env` loading**: Calls `godotenv.Load()`. Reads `.env` file from the current working directory if it exists. Silently continues if not found.

2. **Defaults**: Initialises `Config` with hard-coded defaults. `UpstreamURL` and `IdpURL` are populated from `AUVIS_UPSTREAM` and `AUVIS_IDP_URL` env vars at this stage.

3. **YAML parsing** (if `path != ""`):
   - Reads file at `path`.
   - Unmarshals into `yamlConfig`.
   - Overrides each default field if the YAML value is non-zero.
   - **Destination resolution**: Prefers `gateway.destinations` list (non-empty). Falls back to `gateway.destination` singular if destinations list is empty and singular type is non-empty.

4. **Environment variable overrides** (highest precedence):
   - `AUVIS_PORT` → `cfg.Port`.
   - `AUVIS_AUDIT_FILE` → `cfg.AuditFile`.

5. **Salt resolution** (special handling):
   - Reads `AUVIS_SALT` env var.
   - If empty: uses `cfg.Salt` from YAML, or `"default-auvis-salt-value"`.
   - Sets `cfg.Salt`.
   - Calls `os.Setenv("AUVIS_SALT", salt)` so the `transform.anonymizeField` function can read it without needing a config reference.

**Returns:** Fully resolved `Config`. Always succeeds (config errors are logged but do not stop startup).

---

## Environment Variables

| Variable | Description |
|---|---|
| `AUVIS_PORT` | Override listening port (e.g. `"9090"`) |
| `AUVIS_UPSTREAM` | Default upstream URL (e.g. `"http://localhost:3000"`) |
| `AUVIS_AUDIT_FILE` | Override audit log file path |
| `AUVIS_IDP_URL` | Identity provider URL for dynamic user attribute lookup |
| `AUVIS_SALT` | Salt for SHA-256 anonymization. **Must be kept secret**. |
| `GEMINI_API_KEY` | Gemini API key for LLM PII scanning (used by `pii.Scanner`) |

---

## `auvis.yaml` Full Reference

```yaml
gateway:
  port: 8080                          # Listening port
  upstream: "http://localhost:3000"   # Default upstream (overridable per-request via X-Auvis-Upstream)
  policies_dir: ./policies            # Directory containing .yaml policy files
  audit_file: ./audit.jsonl           # Audit log output path
  audit_max_size_mb: 10               # Rotate audit file after this many MB
  audit_max_age_days: 30              # Delete rotated files older than this many days
  idp_url: ""                         # Optional Identity Provider URL for dynamic attribute lookup
  salt: "my-secret-salt"              # Salt for anonymize action

  # Preferred: list of destinations
  destinations:
    - type: postgres
      settings:
        host: localhost
        port: "5432"
        dbname: auvis
        table: auvis_records
        auto_create_table: "true"
      auth:
        username: auvis
        password: secret
      tls:
        skip_verify: "false"

    - type: pinecone
      settings:
        index_host: "my-index.svc.pinecone.io"
        namespace: auvis
        vector_field: embedding
      auth:
        password: "your-pinecone-api-key"

  # Legacy: singular destination (used if destinations list is empty)
  destination:
    type: pgvector
    settings:
      host: localhost
      port: "5432"
      dbname: auvis
      table: auvis_vectors
      vector_field: embedding
      vector_dim: "1536"
```
