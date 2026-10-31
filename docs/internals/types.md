# Module Documentation: `types`

The `types` module defines shared data structures passed between Auvis's modules to avoid circular imports. It contains a single file with a single struct.

---

## 📂 File Index

- [`context.go`](#1-contextgo)

---

## 1. `context.go`

### `type RequestContext struct`

Carries per-request identity metadata and routing context through the proxy pipeline — from `proxy.extractContext` through `transform.apply` through `policy.Store.Lookup`. Passed by value (small struct, no heap allocation per field).

| Field | Type | Description |
|---|---|---|
| `RequestID` | `string` | Unique nanosecond-precision request correlation ID: `"req-<UnixNano>"`. Used in all log messages and audit entries. |
| `UserID` | `string` | Authenticated user identity. Resolved from JWT `sub`/`uid`/`userid` claims, then `X-User-ID` header. |
| `UserRole` | `string` | Security role of the user (e.g. `"analyst"`, `"admin"`). Resolved from JWT `role`/`userrole` claims, then `X-User-Role` header. |
| `Resource` | `string` | Request URL path (e.g. `"/users"`, `"/patients/123"`). Used for resource prefix matching in policy lookup. |
| `Method` | `string` | HTTP method (`"GET"`, `"POST"`, etc.) or `"INGEST"` for pipeline records. |
| `Service` | `string` | Upstream service name. Resolved from `X-Auvis-Service`, `X-Service-Name`, `X-Service`, or extracted hostname from `X-Auvis-Upstream`. Used for service wildcard matching. |
| `Fields` | `[]string` | Optional field scope constraint from `?fields=a,b,c` query param or `X-Auvis-Fields` header. |
| `UserAttributes` | `map[string]string` | All user attributes available for condition evaluation. Populated from: `X-User-*` headers → JWT claims → IdP lookup (in overlay order). Keys are always lowercase. Always includes `"role"` and `"id"`. |
| `Source` | `string` | Ingestion source identifier for pipeline mode (e.g. Kafka topic name). |
| `Destination` | `string` | Egress destination identifier for pipeline mode. |
| `FieldTags` | `map[string][]string` | Dynamic field semantic tags for tag-based policy rules. Key: field name. Value: list of tags (e.g. `["@pii", "@financial"]`). |
| `Trigger` | `string` | Execution context: `"read"` (proxy HTTP mode) or `"ingest"` (pipeline stream mode). Used for trigger-based policy matching. |

---

### Notes

- `UserAttributes` overlays sources in this order (last writer wins per key):
  1. `X-User-*` headers (excluding `X-User-ID` and `X-User-Role`)
  2. JWT payload claims
  3. IdP lookup response (if `idp_url` is configured)
  4. Explicit `"role"` and `"id"` from resolved identity (always set)

- `FieldTags` is not populated by the proxy handler — it is available as an extension point for pipeline use cases where field metadata is available from schema registries or upstream tagging.

- `RequestID` is generated fresh per request using `fmt.Sprintf("req-%d", time.Now().UnixNano())`. In the pipeline mode, it falls back to `record.ID` or `fmt.Sprintf("rec-%d", startTime.UnixNano())`.
