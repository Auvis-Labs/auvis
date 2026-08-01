# Module Documentation: `proxy`

The `proxy` module implements Auvis's HTTP data governance proxy — the core layer between clients and upstream services. It performs identity extraction, cached reverse proxy forwarding, response stream capture, field-level JSON transformation, LLM PII interception, synchronous multi-destination fan-out with fail-closed guarantees, audit logging, and panic recovery.

Benchmarked at **6,500+ requests/second** with **7.4ms average latency**.

---

## 📂 File Index

- [`handler.go`](#1-handlergo)
- [`context.go`](#2-contextgo)

---

## 1. `handler.go`

Contains the full proxy pipeline — from inbound request to outbound governed response.

---

### Package-level types and variables

#### `type contextKey string` / `const auvisContextKey contextKey = "auvis_ctx"`

A typed context key used to attach `types.RequestContext` to a Go `context.Context`. Typed to avoid collisions with other packages using plain `string` keys.

---

#### `type streamCapture struct`

An internal response writer that captures upstream response headers and status code before the body is available, to support streaming body processing.

| Field | Type | Description |
|---|---|---|
| `header` | `http.Header` | Cloned response headers from upstream |
| `statusCode` | `int` | HTTP status code (defaults to 200) |
| `bodyReader` | `io.ReadCloser` | Body reader (unused in current path; body flows via `io.Pipe`) |

---

#### `type responseInterceptor struct`

Implements `http.ResponseWriter`. Used as the target for `rp.ServeHTTP` so that the upstream response body is piped through an `io.Pipe` to the transform function, rather than written directly to the client.

| Field | Type | Description |
|---|---|---|
| `pipeWriter` | `*io.PipeWriter` | Write end of the pipe; body bytes flow here |
| `infoCh` | `chan responseInfo` | Sends status code and headers once `WriteHeader` is called |
| `header` | `http.Header` | Accumulates response headers |
| `wroteHeader` | `bool` | Guards against double `WriteHeader` calls |

**Methods:**
- `Header() http.Header` — Returns `ri.header` so upstream can set response headers.
- `WriteHeader(code int)` — Sends `responseInfo{statusCode: code, header: ri.header.Clone()}` to `infoCh`, then sets `wroteHeader = true`. Idempotent.
- `Write(p []byte) (int, error)` — Calls `WriteHeader(200)` if not yet called, then pipes bytes to `pipeWriter`.

---

#### `type responseInfo struct`

| Field | Type | Description |
|---|---|---|
| `statusCode` | `int` | HTTP status code from upstream |
| `header` | `http.Header` | Cloned headers from upstream response |

---

#### `newStreamCapture() *streamCapture`

Constructor for `streamCapture` initializing the header map and defaulting `statusCode` to 200.

#### `newResponseInterceptor(pw *io.PipeWriter, ch chan responseInfo) *responseInterceptor`

Constructor for `responseInterceptor`.

---

### `Recovery(next http.Handler) http.HandlerFunc`

A middleware wrapper that catches **any** unhandled runtime panic in the wrapped handler.

**Behavior:**
1. Installs a `defer func() { recover() }()` guard.
2. On panic:
   - Calls `metrics.IncPanic("proxy_middleware")` — increments the `auvis_recovered_panics_total{component="proxy_middleware"}` counter.
   - Captures full goroutine stack trace via `string(debug.Stack())`.
   - Logs `slog.Error("Panic recovered in HTTP handler middleware", "error", rErr, "stack", stack, "method", ...)`.
   - Writes `HTTP 500 Internal Server Error` to the client.
3. If no panic: delegates to `next.ServeHTTP(w, r)`.

**Use case:** Applied as an outer middleware wrapper around any handler to ensure the HTTP server process never crashes from unexpected panics.

---

### `Handler(cfg, store, transformFn, auditLog, requestCount, dest) http.HandlerFunc`

The main catch-all reverse proxy handler. Constructed once at startup; returns a closure.

**Parameters:**
| Parameter | Type | Description |
|---|---|---|
| `cfg` | `config.Config` | Loaded gateway configuration |
| `store` | `*policy.Store` | Thread-safe in-memory policy registry |
| `transformFn` | `transform.StreamTransformFunc` | Streaming field transformation function |
| `auditLog` | `*audit.Logger` | Non-blocking audit logger |
| `requestCount` | `*int64` | Atomic request counter pointer |
| `dest` | `pipeline.Destination` | Destination sink (or `nil`) |

**Shared state initialized once at construction:**

| Variable | Type | Description |
|---|---|---|
| `transport` | `*http.Transport` | Shared transport pool: `MaxIdleConns=1000`, `MaxIdleConnsPerHost=1000`, `IdleConnTimeout=90s` |
| `bufPool` | `sync.Pool` | Reusable `*bytes.Buffer` pool, reducing GC pressure per request |
| `proxyCache` | `sync.Map` | Cache of `*httputil.ReverseProxy` keyed by upstream URL string |

---

**Handler closure — execution steps:**

#### Step 0: Panic Recovery Guard
```go
defer func() {
    if rErr := recover(); rErr != nil {
        metrics.IncPanic("proxy_handler")
        // logs stack, returns 500
    }
}()
```
Protects the entire handler from panics in any downstream code path (transform, destination, etc.).

#### Step 1: Request Counter & Identity Extraction
- Atomically increments `*requestCount` via `atomic.AddInt64`.
- Calls `extractContext(r, cfg.IdpURL)` to populate `reqCtx`.
- If `reqCtx.UserID == ""` or `reqCtx.UserRole == ""` → returns `HTTP 401 Unauthorized`.

#### Step 1.5: LLM PII Interception Check
- Calls `store.Lookup(reqCtx.UserRole, reqCtx.Resource, ...)` to find the effective policy.
- If the policy has a wildcard rule `"*": "llm_intercept"`:
  - Reads the full request body via `io.ReadAll(r.Body)`.
  - Reads `GEMINI_API_KEY` from environment. Reads `X-Auvis-Scanner-Mode` header (defaults to `"hybrid"`).
  - Constructs a `pii.Scanner{APIKey, LLMFallback, Mode}` and a `pii.LLMInterceptor{Scanner, Rehydrate}`.
  - Calls `llmInterceptor.InterceptRequest(ctx, bodyBytes, reqCtx)` — detects PII, replaces with tokens like `{{PII_EMAIL_1}}`, returns `maskedBody`, `placeholderMap`, and `requestSummary`.
  - Replaces `r.Body` with `io.NopCloser(bytes.NewReader(maskedBody))`. Updates `r.ContentLength` and `Content-Length` header.

#### Step 2: Upstream URL Resolution
- Reads `X-Auvis-Upstream` header. Falls back to `cfg.UpstreamURL`.
- If both empty → `HTTP 400 Bad Request`.

#### Step 3: Proxy Cache Lookup / Creation (`getProxy`)
The inner `getProxy(rawUpstream string) (*httputil.ReverseProxy, error)` function:
1. Checks `proxyCache.Load(rawUpstream)`. If found → returns cached `*httputil.ReverseProxy` immediately (**zero allocation**).
2. If not found:
   - Parses URL via `url.Parse`. Validates `Host != ""`.
   - Creates `httputil.NewSingleHostReverseProxy(parsedURL)`.
   - Sets `rp.Transport = transport` (shared pool).
   - Sets `rp.Director` to rewrite `URL.Scheme`, `URL.Host`, `Host` header, and **strip** Auvis-specific headers (`X-Auvis-Upstream`, `X-User-ID`, `X-User-Role`). Preserves `Authorization` header.
   - Sets `rp.ModifyResponse` to decompress gzip responses: if `Content-Encoding: gzip`, wraps `resp.Body` with `gzip.NewReader`, removes `Content-Encoding` and `Content-Length` headers.
   - Sets `rp.ErrorHandler` to: extract `reqCtx` from request context, log `slog.Error("Upstream service unavailable", ...)`, write an error audit entry, return `HTTP 502 Bad Gateway`.
   - Stores via `proxyCache.LoadOrStore` (race-safe).

#### Step 4: Response Streaming Pipeline
- Creates `io.Pipe()` (pipeReader, pipeWriter).
- Creates `responseInterceptor{pipeWriter, infoCh}`.
- Attaches `reqCtx` to request context via `r.WithContext(context.WithValue(..., auvisContextKey, reqCtx))`.
- Launches `go rp.ServeHTTP(interceptor, r)` — upstream response body bytes flow into `pipeWriter`.
- Calls `pipeWriter.Close()` after ServeHTTP returns.
- Blocks on `info := <-infoCh` to receive status code and headers.

#### Step 5: Header Forwarding
- Copies all headers from `info.header` to `w.Header()`.
- Deletes `Content-Encoding` and `Content-Length` (since transform changes body length).
- Sets `Content-Type: application/json`.

#### Step 6: Transform / Rehydrate
Three branches:
1. **LLM intercept path**: Reads all bytes from `pipeReader` via `io.ReadAll`. Calls `llmInterceptor.InterceptResponse(responseBytes, placeholderMap)` to replace placeholder tokens with original PII values. Writes to `buf`.
2. **Non-200 upstream**: Reads raw bytes into `buf` without applying field rules. Returns `summary = transform.Summary{}`.
3. **Normal 200 path**: Calls `transformFn(pipeReader, buf, reqCtx)` — streams body through transform engine, writing governed output into `buf`.

#### Step 7: Destination Fan-Out (Decoupled Availability)
If `dest != nil`:
1. Unmarshals `buf.Bytes()` into `map[string]any`. Falls back to `{"body": <raw string>}` if JSON parse fails.
2. Constructs `pipeline.Record{ID: reqCtx.RequestID, Source: rawUpstream, Schema: reqCtx.Resource, Data: dataPayload, Metadata: {...}, Timestamp: time.Now().UTC()}`.
3. Calls `dest.WriteRecord(rec)` wrapped in a `defer recover()` guard:
   - On panic: increments `metrics.IncPanic("destination_write")`, captures stack trace, logs `slog.Error`, and sets `writeErr`.
4. Unpacks `*destination.MultiError` if present to populate per-destination `DestResult` slice.
5. If `writeErr != nil`:
   - Logs warning (`slog.Warn("Destination write failed — delivering governed response to client (decoupled destination failure)")`).
   - Does **not** block or return HTTP 500 — execution continues to Step 8 to deliver the governed response payload to the client (`200 OK`).
   - Destination write outcome (`destination_written: false`, `destinations_failed: N`) is recorded in the audit log.

#### Step 8: Write Response & Audit
- Sets `responseStatus = info.statusCode`. If `summary.Denied == true` → overrides to `403 Forbidden`.
- Writes status code and `buf.Bytes()` to client.
- Enqueues full `audit.Entry` via `auditLog.Write(...)` containing destination result details (`destination_written: destsWritten > 0 && destsFailed == 0`).
- Logs `slog.Info("Request handled successfully", "status", responseStatus, "duration_ms", ...)`.

---

### `mapRecordOutcomes(in []transform.RecordOutcome) []audit.RecordOutcome` *(private)*

Converts `transform.RecordOutcome` slice into `audit.RecordOutcome` slice field-by-field. This mapping step exists solely to avoid a circular import between the `audit` and `transform` packages.

---

## 2. `context.go`

Resolves request identity metadata into a unified `types.RequestContext`.

---

### `extractContext(r *http.Request, idpURL string) types.RequestContext` *(private)*

Resolution chain for `UserID` and `UserRole`:

**Priority order (highest first):**
1. **JWT Bearer Claims** (`Authorization: Bearer <token>`): Calls `parseJWTClaims`. Checks `sub`, `uid`, `userid` for `UserID`; checks `role`, `userrole` for `UserRole`.
2. **Explicit Headers**: Falls back to `X-User-ID` and `X-User-Role` headers if JWT values are absent.

**Attribute collection into `attrs map[string]string]`:**
1. Collects all `X-User-*` headers (excluding `X-User-ID` and `X-User-Role`). Strips the `x-user-` prefix to produce attribute keys (e.g. `X-User-Department` → `department`).
2. Overlays JWT claims (all claim keys lowercased).
3. Overlays IdP attributes if `idpURL != ""` and `userID != ""`, calling `resolveIdpAttributes(userID, idpURL)`.
4. Ensures `attrs["role"]` and `attrs["id"]` are always set to avoid missing-attribute condition evaluation errors.

**Service extraction** (in priority order):
1. `X-Auvis-Service` header.
2. `X-Service-Name` header.
3. `X-Service` header.
4. Hostname extracted from `X-Auvis-Upstream` header URL (strips port).

**Fields extraction** (request-scoped field list):
1. `?fields=a,b,c` query parameter.
2. `X-Auvis-Fields` header (comma-separated).

**Returns `types.RequestContext{}`** with:
- `RequestID`: `fmt.Sprintf("req-%d", time.Now().UnixNano())` — nanosecond-precision unique ID.
- `UserID`, `UserRole`, `Resource` (`r.URL.Path`), `Method`, `Service`, `Fields`, `UserAttributes`, `Trigger` (hardcoded `"read"`).

---

### `parseJWTClaims(authHeader string) map[string]string` *(private)*

Decodes the payload segment of a Bearer JWT token **without signature verification** (claims are trusted only for identity routing, not cryptographic authentication).

**Logic:**
1. Checks prefix `"Bearer "`. Returns empty map if absent.
2. Splits token by `.`. Returns empty map if not 3 parts.
3. Takes `parts[1]` (payload segment). Pads with `=` to correct base64 length.
4. Attempts `base64.URLEncoding.DecodeString`. Falls back to `base64.StdEncoding.DecodeString`.
5. JSON-decodes the payload into `map[string]interface{}`.
6. Converts all values to strings via `fmt.Sprintf("%v", v)`. Lowercases all keys.
7. Returns the populated claims map.

---

### `resolveIdpAttributes(userID string, idpURL string) map[string]string` *(private)*

Makes a synchronous HTTP GET call to an external Identity Provider to fetch dynamic user attributes.

**Logic:**
1. Constructs request URL: `<idpURL>?user_id=<userID>`.
2. Uses `http.Client{Timeout: 2 * time.Second}`.
3. If request fails or response is not `200` → returns empty map silently (logged via `log.Printf`).
4. If `200` → decodes JSON response body into `map[string]interface{}`. Lowercases all keys, converts values to strings.
5. Returns the attribute map (overlaid into `reqCtx.UserAttributes`).
