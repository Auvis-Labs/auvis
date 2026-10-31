# Module Documentation: `transform`

The `transform` module executes field-level data governance transformations on JSON payloads: masking, removal, anonymization, redaction, truncation, field-level deny, row-level filtering, dynamic tag-based rules, and data-path envelope extraction/re-embedding. Uses `gjson` for zero-allocation JSON reads and `sjson` for surgical JSON writes. Uses `sync.Pool` for buffer reuse under concurrency.

---

## 📂 File Index

- [`transform.go`](#1-transformgo)

---

## 1. `transform.go`

---

### `type Summary struct`

Carries every field disposition produced during a transform execution. Fed directly into the `audit.Entry` and returned to `proxy.Handler`.

| Field | Description |
|---|---|
| `PoliciesApplied []string` | Names of policies matched and applied |
| `FieldsMasked []string` | Fields transformed (mask, anonymize, redact, truncate) |
| `FieldsRemoved []string` | Fields deleted entirely (`action: remove`) |
| `FieldsAllowed []string` | Fields that passed through untouched |
| `FieldsDenied []string` | Fields blocked by field-level deny rule |
| `ResponseDenied bool` | `true` if the entire response was blocked (403) |
| `PiiTypesDetected []string` | PII category strings detected during scanning |
| `RowsFiltered int` | Count of array rows excluded by `filter_condition` |
| `Priority int` | Priority of the winning policy |
| `ComplianceTags []string` | Compliance framework tags from applied policies |
| `ConditionEvaluated []string` | Human-readable condition expressions and their results |
| `Denied bool` | Internal flag driving HTTP 403 in proxy handler |
| `DenyReason string` | Human-readable deny reason (returned in response body) |
| `RecordOutcomes []RecordOutcome` | Per-record dispositions (nil for single-record responses) |

---

### `type RecordOutcome struct`

Captures what happened to one record within a multi-record JSON array response. Enables compliance-grade auditing: "what exactly happened to record PT-047?"

| Field | JSON key | Description |
|---|---|---|
| `RecordID` | `record_id` | Record identifier extracted from id/patient_id/record_id/uuid fields |
| `FieldsMasked` | `fields_masked` | Masked/anonymized/redacted/truncated fields |
| `FieldsRemoved` | `fields_removed` | Removed fields |
| `FieldsAllowed` | `fields_allowed` | Allowed-through fields |
| `FieldsDenied` | `fields_denied` | Field-level denied fields |
| `Filtered` | `filtered` | `true` if row was excluded by `filter_condition` |

---

### `type TransformFunc func([]byte, types.RequestContext) ([]byte, Summary, error)`

Function signature for non-streaming (buffer-based) transforms. Accepts raw body bytes and request context; returns governed body bytes and summary.

---

### `type StreamTransformFunc func(src io.Reader, dst io.Writer, reqCtx types.RequestContext) (Summary, error)`

Function signature for streaming transforms. Reads from `src` (upstream pipe reader), writes governed output to `dst` (response buffer). Used by `proxy.Handler`.

---

### `NewApplyFn(store *policy.Store) TransformFunc`

Returns a `TransformFunc` closure capturing `store`. Calls `apply(body, reqCtx, store)` on each invocation.

---

### `NewStreamApplyFn(store *policy.Store) StreamTransformFunc`

Returns a `StreamTransformFunc` closure capturing `store`. Reads all bytes from `src` via `io.ReadAll`, calls `apply`, writes result to `dst`.

---

### `Apply(body []byte, reqCtx types.RequestContext, store *policy.Store) ([]byte, Summary, error)`

Public wrapper for `apply`. Used in tests and by the pipeline `TransformEngine`.

---

### `apply(body []byte, reqCtx types.RequestContext, store *policy.Store) ([]byte, Summary, error)` *(private)*

Main transform entrypoint. Every request body passes through this function.

**Steps:**

1. **Policy Lookup**: Calls `store.Lookup(reqCtx.UserRole, reqCtx.Resource, reqCtx.Service, reqCtx.Source, reqCtx.Destination, "read")`.
   - `nil` result → **deny-by-default**: returns `{"error":"forbidden","reason":"no policy found for your role on this resource"}`, `Summary{Denied: true}`.
   - `ep.Deny == true` → **explicit deny**: returns `{"error":"forbidden","reason":"access denied by policy"}`, `Summary{Denied: true}`.
   - Wildcard allow rule (`"*": "allow"`) → returns body untouched with empty summary.

2. **Target Extraction** (`extractTarget`): If `ep.Response.DataPath != ""` (e.g. `"data"` or `"result.users"`), drills into the response JSON with `gjson.GetBytes(body, path)`. Returns error if path not found. If `DataPath == ""`, uses the root of `body`.

3. **Array vs Object Detection**: Checks `target.IsArray()`. If `ep.Response.Array` (pointer) is set in policy, that overrides auto-detection.

4. **Transformation**:
   - Array path → calls `transformArray(target, reqCtx, ep, &conditionEvaluated)`.
   - Single object path → calls `transformRecord([]byte(target.Raw), reqCtx, ep, &conditionEvaluated)`.

5. **Re-embedding** (`reEmbed`): If `DataPath != ""`, uses `sjson.SetBytes(originalBody, path, transformedData)` to write the transformed records back into the original response envelope. If `DataPath == ""`, the transformed data is the complete response.

6. **Summary Construction**: Deduplicates field lists using `dedupe()`. Returns complete `Summary`.

---

### `extractTarget(body []byte, path string) (gjson.Result, error)` *(private)*

If `path == ""`: returns `gjson.ParseBytes(body)` (root).
If `path != ""`: returns `gjson.GetBytes(body, path)`. Returns error if path does not exist.

---

### `reEmbed(originalBody []byte, path string, transformed []byte) ([]byte, error)` *(private)*

If `path == ""`: returns `transformed` unchanged.
If `path != ""`: parses `transformed` as JSON value and sets it at `path` within `originalBody` using `sjson.SetBytes`.

---

### `type recordDisposition struct` *(private)*

Groups field names into the four audit buckets during a single record transform:
- `masked []string`, `removed []string`, `allowed []string`, `denied []string`.

---

### `transformArray(arr, reqCtx, ep, conds) (...) ` *(private)*

Iterates each element of a JSON array. Uses `sbPool.Get()` to obtain a reusable `*strings.Builder` for output serialization.

**Per-element logic:**

1. **Record ID extraction** (`extractRecordID`): Tries fields `"id"`, `"patient_id"`, `"record_id"`, `"_id"`, `"uuid"`, `"ID"` in order. Falls back to `"record[N]"` positional label.

2. **Filter Condition** (if `ep.FilterCondition != ""`): Calls `policy.EvaluateCondition(ep.FilterCondition, elementBytes, reqCtx)`:
   - `EvalTrue` → record passes, continues to transform.
   - `EvalFalse` → record is **excluded**: increments `rowsFiltered`, appends `RecordOutcome{Filtered: true}`, skips to next element.
   - `EvalIndeterminate` (missing attribute) → **fail-closed**: record is excluded, logs `slog.Warn("Filter condition attribute missing — fail-closed row exclusion applied", ...)`.

3. **Record transform**: Calls `transformRecord(elementBytes, reqCtx, ep, conds)`.

4. **Output**: Appends comma-delimited JSON to `strings.Builder`. Accumulates dispositions.

Returns `[]byte`, aggregate masked/removed/allowed/denied slices, rowsFiltered count, `[]RecordOutcome`, and error.

---

### `extractRecordID(record []byte, pos int) string` *(private)*

Tries fields `["id", "patient_id", "record_id", "_id", "uuid", "ID"]` in order via `gjson.GetBytes`. Returns first non-empty string value found. Falls back to `"record[<pos>]"`.

---

### `resolveRules(ep, fieldTags) (map[string]RuleAction, map[string]string, map[string]int)` *(private)*

Merges explicit field rules with dynamic tag-based rules (used when `reqCtx.FieldTags` provides per-field semantic tags).

1. **Explicit rules** (highest precedence): Copies `ep.Rules`, `ep.RuleConditions`, `ep.RuleLengths` directly.
2. **Tag-based rules** (only for fields not already explicitly covered): For each `(field, tags)` in `fieldTags`:
   - Checks each tag against `ep.TaggedRules`.
   - Selects the highest-priority action using a priority table: `remove=7 > deny=6 > redact=5 > mask=4 > truncate=3 > anonymize=2 > allow=1`.
   - Resolves corresponding `TaggedRuleConditions` and `TaggedRuleLengths`.
Returns the merged resolved rules, conditions, and lengths maps.

---

### `transformRecord(record []byte, reqCtx, ep, conds) ([]byte, recordDisposition, error)` *(private)*

Applies field rules to a single JSON object byte slice.

**Per-field iteration:**
1. **Fields constraint check**: If `ep.Fields` is set, only applies rules to matching fields (exact, wildcard `"*"`, or prefix match).
2. **Existence check**: Skips fields not present in the record via `gjson.GetBytes(out, field).Exists()`.
3. **Condition evaluation** (if `ep.RuleConditions[field] != ""`): Calls `policy.EvaluateCondition(cond, record, reqCtx)`:
   - `EvalFalse` → skips the rule (field left alone, not marked as actedOn).
   - `EvalIndeterminate` → **fail-closed**: applies the rule anyway, logs `slog.Warn("Rule condition attribute missing — fail-closed rule enforcement applied", ...)`.
4. **Action dispatch**:
   - `mask` → `maskField(out, field)` → appends to `disp.masked`.
   - `anonymize` → `anonymizeField(out, field)` → appends to `disp.masked`.
   - `redact` → `redactField(out, field)` → appends to `disp.masked`.
   - `truncate` → `truncateField(out, field, resolvedLengths[field])` → appends to `disp.masked`.
   - `remove` → `sjson.DeleteBytes(out, field)` → appends to `disp.removed`.
   - `deny` (field-level) → `sjson.DeleteBytes(out, field)` → appends to `disp.denied`.
   - `allow` → no-op (field passes through).
5. **Allowed fields**: Iterates all top-level keys in the **original** record. Any key not in `actedOn` → appended to `disp.allowed`.

---

### Field Transformation Helpers

#### `maskField(record []byte, field string) ([]byte, error)` *(private)*
Reads field value via `gjson`. For scalars: calls `maskValue`. For nested objects/arrays: calls `collectLeafPaths` to enumerate all leaf paths, then masks each leaf individually.

#### `collectLeafPaths(res gjson.Result, currentPath string, paths *[]string)` *(private)*
Recursively traverses a gjson value, building dot-notation paths for all leaf (non-object, non-array) nodes. Used by `maskField`, `redactField`, and `truncateField` to handle nested structures.

#### `maskValue(val gjson.Result) interface{}` *(private)*

Auto-detects value format and applies the appropriate mask pattern:
- Contains `@` → email: `"***@domain.com"` (`maskEmail`).
- Matches `^\d{3}-\d{2}-\d{4}$` → SSN: `"**-**-XXXX"` (`maskSSN`).
- Matches credit card pattern → card: `"************XXXX"` (`maskCreditCard`).
- Matches phone pattern → phone: masks all but last 4 digits (`maskPhone`).
- Anything else → generic: `"****XXXX"` (last 4 chars preserved, `maskGeneric`).
- Numbers → masked as generic string.
- Booleans and null → passed through unchanged.

#### `maskEmail(email string) string` *(private)*
Splits on `@`. Returns `"***@<domain>"`.

#### `maskSSN(value string) string` *(private)*
Returns `"**-**-" + value[7:]` (last 4 digits visible).

#### `maskCreditCard(value string) string` *(private)*
Handles formats: `XXXX-XXXX-XXXX-XXXX`, `XXXX XXXX XXXX XXXX`, and raw 16-digit. Returns last 4 digits with masked prefix.

#### `maskPhone(value string) string` *(private)*
Walks backward through the string counting digits. Locates the start of the last 4 digits. Replaces all preceding alphanumeric characters with `*` while preserving separator characters (`-`, `.`, space, `(`, `)`).

#### `maskGeneric(value string) string` *(private)*
If `len(value) <= 4`: returns `"****"`.
Otherwise: returns `"****" + value[len(value)-4:]`.

---

#### `anonymizeField(record []byte, field string) ([]byte, error)` *(private)*
Computes SHA-256 hash of `value + salt`:
- Reads `AUVIS_SALT` environment variable (falls back to `"default-auvis-salt-value"`).
- `hasher.Write([]byte(value + salt))`.
- Returns 64-character lowercase hex digest via `hex.EncodeToString(hasher.Sum(nil))`.
- Uses `sjson.SetBytes` to write hash string back to field path.

---

#### `redactField(record []byte, field string) ([]byte, error)` *(private)*
For scalars: `sjson.SetBytes(record, field, "[REDACTED]")`.
For nested objects/arrays: enumerates leaf paths via `collectLeafPaths`, sets each leaf to `"[REDACTED]"`.

---

#### `truncateField(record []byte, field string, length int) ([]byte, error)` *(private)*
For scalars: calls `truncateValue(val, length)`.
For nested structures: enumerates leaf paths, truncates each leaf.

#### `truncateValue(val gjson.Result, length int) interface{}` *(private)*
If not a string → returns value unchanged.
If `len([]rune(s)) <= length` → returns string unchanged (rune-safe, handles Unicode).
Otherwise → returns `string([]rune(s)[:length])`.

---

#### `deny(reason string) []byte` *(private)*
Returns `{"error":"forbidden","reason":"<reason>"}` as bytes. Written to the response body when a policy deny fires.

---

### Package-level variables

#### `sbPool sync.Pool`
Pre-allocates `*strings.Builder` instances with `Grow(4096)`. Shared across concurrent `transformArray` calls. Eliminates per-request heap allocations for response array serialization.

#### Compiled regexes
- `ssnPattern = regexp.MustCompile(...)` — SSN detector `^\d{3}-\d{2}-\d{4}$`.
- `phonePattern = regexp.MustCompile(...)` — Phone number detector.
- `cardPattern = regexp.MustCompile(...)` — Credit card detector `^(?:\d[ -]?){13,16}$`.

---

#### `dedupe(in []string) []string` *(private)*
Removes duplicate strings from a slice. Uses a `map[string]bool` seen-set. Preserves first-occurrence order. Used on masked/removed/allowed/denied slices before populating `Summary`.
