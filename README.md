# IRIS — Intelligent Response Interception System

> A lightweight, policy-driven API proxy that sits in front of your existing services and enforces data access rules in real time — no code changes to your upstream, no database, no infrastructure.

---

## What Is IRIS?

IRIS is a Go reverse proxy that intercepts API responses and transforms them based on declarative YAML policies. When a request comes in, IRIS checks the caller's identity, evaluates the matching policy, and returns a cleaned response — masking sensitive fields, removing restricted data, or denying access entirely.

The entire system is a single binary, a folder of YAML files, and a log file.

```
Client → IRIS Proxy (Go) → Your Upstream API
               ↓
        Policy Engine
        (YAML files, in-memory, hot-reloaded)
               ↓
        Transform Response
        (mask fields, remove fields, deny)
               ↓
        Audit Log
        (append-only JSONL file)
```

**Core capabilities:**

- **Field masking** — `email: "user@example.com"` becomes `email: "***@example.com"`
- **Field removal** — strip entire fields from responses by role
- **Access denial** — block responses entirely for specific roles
- **Policy hot-reload** — edit a YAML file and it takes effect within seconds, no restart
- **Audit logging** — every request logged to a structured JSONL file
- **Response envelope support** — works with `{"data": [...]}` wrapped APIs, not just root arrays

---

## Project Structure

```
iris/
├── main.go                  # Entry point — wires everything together
├── config/
│   └── config.go            # Loads server config from environment variables
├── policy/
│   ├── types.go             # Policy struct definitions
│   ├── loader.go            # YAML loader, compiler, fsnotify hot-reload
│   └── engine.go            # Policy matching logic
├── proxy/
│   └── handler.go           # HTTP reverse proxy + interception layer
├── transform/
│   └── transform.go         # Response transformation (mask, remove, deny)
├── audit/
│   ├── types.go             # Audit Entry struct
│   ├── logger.go            # Async JSONL writer
│   └── iris.go              # /iris/audit and /iris/status endpoints
└── policies/                # YAML policy files (hot-reloaded)
    └── policies.yaml
```

---

## Getting Started

```bash
# Set your upstream API
export IRIS_UPSTREAM=http://localhost:3000
export IRIS_PORT=8080

# Run
go run ./main.go

# IRIS is now proxying :8080 → your upstream
# Drop .yaml files in ./policies/ to define access rules
# Edit them live — no restart needed
```

**Dependencies:**

```go
require (
    github.com/tidwall/gjson    // read JSON fields by dot-notation path
    github.com/tidwall/sjson    // set/delete JSON fields by path
    github.com/fsnotify/fsnotify // watch policy files for changes
    gopkg.in/yaml.v3            // parse policy YAML files
)
```

---

## How a Request Flows Through IRIS

Every request follows the same pipeline:

```
1.  Request arrives at IRIS
2.  Extract X-User-ID and X-User-Role from headers
3.  Validate identity — reject with 401 if missing
4.  Forward request to upstream API
5.  Capture the full response body in memory (not sent to client yet)
6.  Decode body (handles gzip transparently)
7.  Look up matching policy for this role + resource
8.  Apply transform — mask fields, remove fields, or deny
9.  Write audit log entry (async, non-blocking)
10. Send transformed response to client
```

The upstream never sees the identity headers (`X-User-ID`, `X-User-Role` are stripped before forwarding). The client never sees the raw upstream response.

---

## Policy Engine

### How Policies Work

Policies are YAML files in the `./policies/` directory. IRIS watches this directory and reloads automatically whenever a file is saved.

Each policy defines:
- **Who** it applies to (`match.roles`)
- **What resource** it applies to (`match.resource` — prefix matched)
- **What to do** with the response (`rules`)

### Policy File Format

```yaml
policies:
  - name: analyst-users
    description: "GDPR-aligned PII reduction for analyst role"
    tags: [gdpr, pii]
    match:
      roles: [analyst, data-scientist]
      resource: /users          # matches /users, /users/1, /users/1/posts
    rules:
      - field: email
        action: mask            # ***@domain.com
      - field: phone
        action: remove          # field deleted from response
      - field: address
        action: remove
      - field: company
        action: remove

  - name: admin-users
    match:
      roles: [admin]
      resource: /users
    rules:
      - field: "*"
        action: allow           # full passthrough, no transforms

  - name: block-guest
    match:
      roles: [guest]
      resource: /users
    rules:
      - field: "*"
        action: deny            # 403 returned, upstream data never exposed
```

### Rule Actions

| Action | Behaviour |
|--------|-----------|
| `mask` | Partially redacts the field value. Emails become `***@domain.com`. Other strings keep the last 4 characters: `****3804`. |
| `remove` | Deletes the field from the response entirely. |
| `allow` | Passes the field through untouched. Use with `field: "*"` for full passthrough. |
| `deny` | Blocks the entire response. Returns `403` with a reason. Use with `field: "*"` to deny the whole resource. |

### Default Behaviour

**Deny by default.** If no policy matches a role + resource combination, IRIS returns:

```json
{
  "error": "forbidden",
  "reason": "no policy found for your role on this resource"
}
```

You must explicitly define a policy to grant any access.

### Resource Matching

Resources are matched by **longest prefix**:

- Policy resource `/users` matches: `/users`, `/users/1`, `/users/1/posts`
- Policy resource `/users/admin` matches: `/users/admin`, `/users/admin/settings`
- If both `/users` and `/users/admin` exist, `/users/admin/settings` matches `/users/admin` (more specific wins)

### Handling Wrapped API Responses

Real APIs rarely return a bare array. Most wrap their data in an envelope:

```json
{ "data": [...], "meta": { "total": 10, "page": 1 } }
```

Use `response.data_path` to tell IRIS where the records live:

```yaml
- name: analyst-wrapped
  match:
    roles: [analyst]
    resource: /v2/users
  response:
    data_path: "data"         # drill into response["data"]
  rules:
    - field: email
      action: mask
```

IRIS extracts the records at `data_path`, transforms them, then re-embeds them back into the original envelope. The `meta`, `links`, and any other envelope fields pass through untouched.

| API Shape | `data_path` |
|-----------|-------------|
| `[{...}]` | *(omit — root array)* |
| `{...}` | *(omit — root object)* |
| `{"data": [...]}` | `"data"` |
| `{"result": {"users": [...]}}` | `"result.users"` |

If `data_path` is omitted, IRIS operates on the root of the response. If the path does not exist in the response, IRIS returns a 500 error rather than silently passing raw data through.

### Multiple Policies, Same Role + Resource

If multiple policy files define rules for the same role + resource, IRIS **merges them at load time** into a single effective ruleset. At request time it's a single map lookup — no iteration, no duplication checks.

Conflicts (two policies setting different actions on the same field) are logged as warnings at load time, and the last-loaded policy wins.

---

## File Reference

### `main.go`

Entry point. Initialises all components in order and wires them together:

1. Loads config from environment
2. Opens the audit log file
3. Loads all YAML policies from `./policies/`
4. Starts the fsnotify file watcher for hot-reload
5. Creates the transform function (policy store injected as closure)
6. Registers internal endpoints (`/iris/audit`, `/iris/status`)
7. Registers the catch-all proxy handler
8. Starts the HTTP server
9. Listens for `SIGTERM`/`Ctrl+C` and shuts down gracefully — flushes audit log before exit

**Key variable:** `requestCount int64` — incremented atomically on every request, reported by `/iris/status`.

---

### `config/config.go`

Loads server configuration from environment variables.

| Variable | Default | Description |
|----------|---------|-------------|
| `IRIS_UPSTREAM` | *(required)* | URL of the upstream API to proxy to |
| `IRIS_PORT` | `8080` | Port IRIS listens on |

---

### `policy/types.go`

Defines all structs that represent a policy at every stage of its lifecycle.

**`PolicyFile`** — top-level struct that maps directly to a `.yaml` file. Contains a list of `Policy`.

**`Policy`** — a single named policy entry as written in YAML. Contains `Match`, `Response`, and a list of `Rule`. Also carries optional `Description` and `Tags` fields for documentation and grouping.

**`Match`** — defines which requests a policy applies to. `Roles` is a list of role strings. `Resource` is the path prefix to match against.

**`Response`** — optional block describing the API response shape. `DataPath` is the gjson dot-notation path to the records. `Array` is a `*bool` (pointer, not bool) so IRIS can distinguish "not set" from `false` — when nil, IRIS auto-detects by inspecting the actual response value.

**`Rule`** — a single field rule. `Field` is a dot-notation path (`email`, `address.street`, `*`). `Action` is one of `mask | remove | allow | deny`. `Condition` is an optional expression string for conditional rule evaluation (reserved for future use).

**`RuleAction`** — typed string constant. Prevents invalid action values from being set.

**`EffectivePolicy`** — the compiled, merged ruleset that the engine works with at request time. Never the raw `Policy` structs. Contains `SourcePolicies` (list of YAML policy names that contributed to this effective policy — used in audit logs), `Rules` (merged field map), `Response`, and `Deny` flag.

---

### `policy/loader.go`

Responsible for loading YAML files, compiling them into effective policies, and keeping them live via hot-reload.

**`Store`** — thread-safe in-memory store. Holds a `map[string]*EffectivePolicy` keyed by `"role::resource"`. Protected by `sync.RWMutex` — concurrent reads never block each other, writes (reloads) take an exclusive lock.

**`NewStore()`** — initialises an empty store.

**`Store.Lookup(role, resource string) *EffectivePolicy`** — called on every request. Iterates the policy map and returns the effective policy with the longest matching resource prefix for the given role. Returns `nil` if nothing matches, which the transform treats as deny.

**`Store.LoadDir(dir string) (int, error)`** — reads all `.yaml` and `.yml` files from the given directory, parses them, compiles them into effective policies, and atomically swaps the store's internal map. Returns the count of effective policies loaded.

**`Store.WatchDir(dir string) error`** — starts a background goroutine using `fsnotify` that calls `LoadDir` whenever any `.yaml` file in the directory is written, created, or removed. On parse or compile error, logs the error and keeps the previous (last known good) policies active — IRIS never crashes on a bad YAML save.

**`parseFile(path string) ([]Policy, error)`** — reads and unmarshals a single YAML file. Validates that each policy has a name, resource, at least one role, and at least one rule. Returns a descriptive error on validation failure.

**`compile(policies []Policy) (map[string]*EffectivePolicy, error)`** — the core of the policy engine. Iterates all raw policies and builds the effective policy map. For each role in each policy's `match.roles`, it creates or updates the `EffectivePolicy` at key `"role::resource"`. Merges rules (last write wins on conflict, with a warning log). Carries the `Response` block through. Sets `Deny = true` if any rule is `field: "*", action: deny`.

---

### `proxy/handler.go`

The HTTP layer. Handles every incoming request.

**`bodyCapture`** — implements `http.ResponseWriter` but writes to an in-memory buffer instead of the real connection. This is the interception mechanism. The reverse proxy believes it is writing to the client; in reality it is writing to a `bytes.Buffer`. The handler reads from this buffer, transforms the content, then writes to the real connection. Fields: `header`, `body`, `statusCode`.

**`bodyCapture.rawBody() ([]byte, error)`** — reads the buffer and transparently decompresses gzip if the upstream sent `Content-Encoding: gzip`. The transform layer always receives plain JSON bytes regardless of how the upstream compressed the response.

**`Handler(cfg, transformFn, auditLog, requestCount) http.HandlerFunc`** — the main handler factory. Takes all dependencies as parameters — nothing is global. Returns an `http.HandlerFunc` that:

1. Increments `requestCount` atomically
2. Calls `extractContext` to pull `X-User-ID`, `X-User-Role`, and generate a `RequestID`
3. Rejects with `401` if identity headers are missing
4. Attaches the `RequestContext` to the request's `context.Context`
5. Creates a `bodyCapture` and passes it to the reverse proxy
6. Decodes the captured body
7. Calls `transformFn` with the body and request context
8. Writes an `audit.Entry` to the audit logger (non-blocking)
9. Sets the response status to `403` if `summary.Denied` is true, otherwise forwards the upstream status
10. Strips `Content-Encoding` and `Content-Length` headers (since the body may have changed size after decode + transform) and writes the transformed body to the real client

**`rp.Director`** — the upstream request mutator. Sets `Host` header to the upstream host (critical — without this many APIs return 403), adds browser-like headers for compatibility, and strips `X-User-ID`/`X-User-Role` so the upstream never sees IRIS's internal identity headers.

**`rp.ErrorHandler`** — called when the upstream is unreachable. Returns `502` to the client and still writes an audit entry so failed upstream calls are visible in the audit log.

---

### `transform/transform.go`

The policy enforcement layer. Takes raw JSON bytes and a request context, returns transformed bytes and a summary of what was done.

**`Summary`** — returned alongside the transformed body. Contains `PoliciesApplied` (list of policy names), `FieldsMasked` (deduplicated list of field names that were masked), `RowsFiltered`, `Denied` flag, and `DenyReason`. Fed directly into the audit log entry.

**`TransformFunc`** — the function type `func([]byte, types.RequestContext) ([]byte, Summary, error)`. This is the seam between the proxy and the policy engine. The proxy handler holds a `TransformFunc` and calls it — it has no knowledge of policies, YAML, or field rules.

**`NewApplyFn(store *policy.Store) TransformFunc`** — factory that captures the policy store in a closure and returns a `TransformFunc`. This is how the store is injected into the transform without globals.

**`apply(body, reqCtx, store)`** — core logic. Looks up the effective policy via `store.Lookup`. Returns a deny response if no policy matches or if the policy has `Deny: true`. Returns the body unchanged if the policy is wildcard allow. Otherwise proceeds to extract, transform, and re-embed.

**`extractTarget(body []byte, path string) (gjson.Result, error)`** — drills into the response body using `gjson` dot-notation. If `path` is empty, returns the root. If the path does not exist in the response, returns an error (IRIS fails closed rather than passing raw data).

**`reEmbed(originalBody []byte, path string, transformed []byte) ([]byte, error)`** — the inverse of `extractTarget`. If `path` is empty, returns `transformed` as the complete response. If `path` is set, uses `sjson` to write the transformed bytes back into the original envelope at the given path. This preserves pagination metadata, links, and any other envelope fields.

**`transformArray(arr, reqCtx, ep)`** — iterates every element of a JSON array and calls `transformRecord` on each. Accumulates masked field names across all records. Rebuilds the array as a JSON string.

**`transformRecord(record, reqCtx, ep)`** — applies the effective policy's rule map to a single JSON object. For each field/action pair: `mask` calls `maskField`, `remove`/`deny` call `sjson.DeleteBytes`, `allow` is a no-op. Returns the modified record bytes and the list of fields that were masked.

**`maskField(record, field)`** — reads the field value, detects its type by inspecting the value (email detection via `@`), and applies the appropriate mask. Emails become `***@domain.com`. Everything else keeps the last 4 characters: `****3804`.

**`deny(reason string) []byte`** — returns a consistent JSON error body: `{"error":"forbidden","reason":"..."}`. Called for both no-policy-found and explicit deny cases.

---

### `audit/types.go`

Single definition of the `Entry` struct. Shared by `logger.go` (writes entries) and `iris.go` (reads and serves entries). Keeping this in one file prevents redeclaration errors and makes the audit schema easy to find and modify.

**`Entry`** fields:

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | `time.Time` | UTC time the request completed |
| `request_id` | `string` | Unique ID generated per request |
| `user_id` | `string` | From `X-User-ID` header |
| `user_role` | `string` | From `X-User-Role` header |
| `resource` | `string` | Request path e.g. `/users` |
| `method` | `string` | HTTP method |
| `policies_applied` | `[]string` | Names of policies that ran |
| `fields_masked` | `[]string` | Fields that were masked (deduplicated) |
| `rows_filtered` | `int` | Reserved for row filtering (future) |
| `upstream_status` | `int` | HTTP status returned by upstream |
| `duration_ms` | `int64` | Total request duration in milliseconds |

---

### `audit/logger.go`

Async, non-blocking audit writer.

**`Logger`** — holds a buffered channel of `Entry` values and a background goroutine that drains it. The channel has a configurable buffer depth (default 512). The response path never waits for disk — if the channel is full, the entry is dropped with a warning log rather than blocking the request.

**`NewLogger(path, bufSize) (*Logger, error)`** — opens the audit file in append mode (creates if not exists) and starts the background goroutine.

**`Logger.Write(e Entry)`** — non-blocking enqueue via `select`/`default`. Called from the proxy handler after every request.

**`Logger.Close()`** — closes the channel, which causes the background goroutine to drain all remaining entries and exit. Called during graceful shutdown. Uses `sync.Once` to prevent double-close panics.

**`Logger.run(f *os.File)`** — the background goroutine. Reads from the channel using `range` (blocks until an entry arrives or the channel is closed). Writes each entry as pretty-printed JSON using `json.MarshalIndent` followed by a blank line separator. Each entry in `audit.jsonl` looks like:

```json
{
  "timestamp": "2026-04-04T22:25:28Z",
  "request_id": "req-1775337912407896900",
  "user_id": "user-123",
  "user_role": "analyst",
  "resource": "/users",
  "method": "GET",
  "policies_applied": ["analyst-users"],
  "fields_masked": ["email"],
  "rows_filtered": 0,
  "upstream_status": 200,
  "duration_ms": 992
}
```

---

### `audit/iris.go`

Internal IRIS management endpoints. Registered before the catch-all proxy handler so they are never forwarded to upstream.

**`RegisterHandlers(mux, auditPath, startTime, requestCounter)`** — mounts `/iris/audit` and `/iris/status` on the provided mux.

**`GET /iris/audit`** — reads the audit file, parses all entries, applies filters, returns the last N as a JSON array.

Query parameters:

| Parameter | Description |
|-----------|-------------|
| `limit` | Max entries to return (default: 50) |
| `role` | Filter by `user_role` |
| `user` | Filter by `user_id` |
| `resource` | Filter by `resource` |

Example response:
```json
{
  "entries": [...],
  "total": 42,
  "limit": 50,
  "filters": { "role": "analyst", "user": "", "resource": "" }
}
```

**`GET /iris/status`** — returns current proxy state.

```json
{
  "status": "ok",
  "uptime": "2m34s",
  "started_at": "2026-04-04T21:45:12Z",
  "requests_total": 17
}
```

**`parseEntries(data []byte) []Entry`** — parses the raw audit file content. The file contains pretty-printed JSON blocks separated by blank lines. This function uses a line scanner to accumulate lines into blocks, splitting on blank lines, then unmarshals each block into an `Entry`. Handles files that don't end with a trailing blank line.

**`writeJSON(w, status, v)`** — shared helper that marshals any value to indented JSON and writes it with `Content-Type: application/json`.

---

## Identity Model

IRIS does not authenticate requests. Identity comes from two headers that the caller is responsible for setting:

| Header | Description |
|--------|-------------|
| `X-User-ID` | Identifies the calling user |
| `X-User-Role` | Determines which policies apply |

Both headers are required. Requests missing either header are rejected with `401`.

Both headers are stripped before forwarding to upstream — the upstream API never sees them.

In production, these headers should be set by your API gateway or authentication layer, not by the client directly.

---

## Demo Script

```bash
# Step 1 — Admin sees everything
curl -H "X-User-ID: u1" -H "X-User-Role: admin" http://localhost:8080/users

# Step 2 — Analyst sees masked + reduced data
curl -H "X-User-ID: u1" -H "X-User-Role: analyst" http://localhost:8080/users

# Step 3 — Guest is blocked
curl -H "X-User-ID: u1" -H "X-User-Role: guest" http://localhost:8080/users

# Step 4 — Unknown role blocked by default
curl -H "X-User-ID: u1" -H "X-User-Role: hacker" http://localhost:8080/users

# Step 5 — Edit policies/policies.yaml while server is running
#           Add "username" with action: remove to analyst-users
#           Save the file — watch the server log:
#           [POLICY] reload OK — 6 effective policies active

# Step 6 — Same request, username now gone, no restart
curl -H "X-User-ID: u1" -H "X-User-Role: analyst" http://localhost:8080/users

# Step 7 — Inspect audit log
curl http://localhost:8080/iris/audit
curl "http://localhost:8080/iris/audit?role=analyst"
curl "http://localhost:8080/iris/audit?limit=5"

# Step 8 — Check status
curl http://localhost:8080/iris/status
```

---

## What Is Not in This Version

The following are intentional exclusions from the current release:

- **No authentication** — identity is header-based; intended to sit behind an API gateway
- **No database** — policies are YAML files, audit log is a flat file
- **No frontend UI** — management is via curl and the audit endpoints
- **No Docker/Kubernetes** — single binary, runs anywhere Go runs
- **No condition evaluation** — rule conditions (`record.region != user.region`) are parsed but not yet evaluated
- **No row filtering** — filtering entire records from array responses is on the roadmap
- **No XML/Protobuf support** — JSON only
- **No multi-region or distributed state** — single instance

---

## Roadmap

- [ ] Condition evaluator — per-rule conditional logic against record + user context
- [ ] Row filtering — remove entire records from arrays based on conditions
- [ ] Policy grouping — load from subdirectories for large deployments
- [ ] `/iris/audit` time range filtering
- [ ] Audit log rotation
- [ ] HIPAA / SOX policy templates
- [ ] XML and Protobuf response support
- [ ] Policy validation CLI — lint your YAML before deploying