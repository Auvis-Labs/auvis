# Auvis Gateway Configuration Reference (`auvis.yaml`)

This document is the exhaustive configuration reference for the **Auvis** gateway proxy (`auvis.yaml`).

---

## ⚙️ Configuration Loading Hierarchy

Auvis merges configuration settings from three layers in strict priority order (highest priority first):

1. **Environment Variables** (e.g. `AUVIS_PORT`, `AUVIS_SALT`) — overrides YAML settings.
2. **YAML File (`auvis.yaml`)** — explicit file settings.
3. **Built-in Defaults** — fallback values when no setting or env var is present.

---

## 📄 Gateway Settings (`gateway`)

All core gateway settings live under the top-level `gateway` block in `auvis.yaml`.

```yaml
version: "1"

gateway:
  port: 8080
  upstream: "http://localhost:8000"
  policies_dir: ./policies
  audit_file: ./audit.jsonl
  audit_max_size_mb: 100
  audit_max_age_days: 30
  idp_url: "http://idp.internal:8080/userinfo"
  salt: "your-production-salt-phrase"
  upstream_timeout_seconds: 10
```

### Reference Table

| Field | Type | Default | Env Var Override | Description |
|---|---|---|---|---|
| `port` | `integer` / `string` | `8080` | `AUVIS_PORT` | HTTP port Auvis listens on for inbound traffic. |
| `upstream` | `string` | `""` | `AUVIS_UPSTREAM` | Default upstream server URL (e.g., `http://backend:8000`). Can be overridden per-request via `X-Auvis-Upstream` header. |
| `policies_dir` | `string` | `"./policies"` | — | Local directory containing policy YAML files. Auvis watches this folder for instant hot-reload. |
| `audit_file` | `string` | `"./audit.jsonl"` | `AUVIS_AUDIT_FILE` | Append-only newline-delimited JSONL log file path. |
| `audit_max_size_mb` | `integer` | `10` | — | Maximum log file size in megabytes before trigger-based log file rotation. |
| `audit_max_age_days` | `integer` | `30` | — | Retention period in days for rotated log files before automatic cleanup. |
| `idp_url` | `string` | `""` | `AUVIS_IDP_URL` | Optional Identity Provider endpoint URL used to resolve bearer tokens or identity attributes. |
| `salt` | `string` | `"default-auvis-salt-value"` | `AUVIS_SALT` | Secret salt string used for SHA-256 field anonymization (`action: "anonymize"`). |
| `upstream_timeout_seconds` | `integer` | `10` | — | Maximum duration before an upstream HTTP request is cancelled. |

---

## 🛢️ Data Destinations (`destinations`)

Auvis allows fan-out writing of governed response records to one or more database or vector sinks. 

When configured, every governed response is written to all destinations before returning to the client. Destinations operate **fail-closed**: if writing to any destination fails, the client receives an `HTTP 500 Internal Server Error` and the payload is withheld.

```yaml
gateway:
  destinations:
    - type: postgres
      settings:
        host: postgres.internal
        port: "5432"
        dbname: auvis_db
        table: governed_records
        sslmode: require
        auto_create_table: "true"
      auth:
        username: auvis_app
        password: "${POSTGRES_PASSWORD}"

    - type: pinecone
      settings:
        index_host: "my-index-123.svc.pinecone.io"
        vector_field: "embedding"
      auth:
        password: "${PINECONE_API_KEY}"
```

### Destination Drivers

#### 1. PostgreSQL (`postgres`)
- **Settings**:
  - `host`: Database hostname/IP
  - `port`: Port string (default `"5432"`)
  - `dbname`: Target database name
  - `table`: Table name to store records
  - `sslmode`: SSL mode (`disable`, `require`, `verify-full`)
  - `auto_create_table`: `"true"` to automatically run table creation DDL
- **Auth**: `username`, `password`

#### 2. Pinecone Vector DB (`pinecone`)
- **Settings**:
  - `index_host`: Pinecone index host URL
  - `vector_field`: JSON response key containing float vector array
- **Auth**: `password` (Pinecone API Key)

#### 3. pgvector (`pgvector`)
- **Settings**:
  - `host`, `port`, `dbname`, `table`, `sslmode`
  - `vector_field`: Response payload key containing vector embedding
  - `vector_dim`: Vector dimension count (e.g. `"1536"`)
- **Auth**: `username`, `password`

#### 4. Weaviate (`weaviate`)
- **Settings**:
  - `host`: Weaviate instance URL (e.g. `http://weaviate:8081`)
  - `class`: Target Weaviate class name (e.g. `AuvisRecord`)
  - `vector_field`: Response vector field name
- **Auth**: `password` (Weaviate API Key)

---

## 🔐 Environment Variables Quick Reference

| Environment Variable | Description | Example |
|---|---|---|
| `AUVIS_PORT` | Gateway port override | `9090` |
| `AUVIS_UPSTREAM` | Upstream URL override | `http://api.internal:8000` |
| `AUVIS_AUDIT_FILE` | Audit log path override | `/var/log/auvis/audit.jsonl` |
| `AUVIS_SALT` | Hashing salt override | `prod-secret-salt-phrase` |
| `AUVIS_IDP_URL` | IdP resolve endpoint override | `https://idp.company.com/userinfo` |
