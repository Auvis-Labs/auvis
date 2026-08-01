# Module Documentation: `policy`

The `policy` module is the rules engine of Auvis. It defines YAML data models, compiles raw policy files into an optimised runtime `EffectivePolicy` store, evaluates multi-dimensional matching (role, resource, service, source, destination, trigger), merges overlapping policies in priority order, and provides condition expression evaluation for fine-grained field and row control.

---

## 📂 File Index

- [`types.go`](#1-typesgo) — All data models
- [`loader.go`](#2-loadergo) — Compilation, matching, and hot-reload
- [`condition.go`](#3-conditiongo) — Condition expression evaluator

---

## 1. `types.go`

### `type PolicyFile struct`

Top-level structure of a `.yaml` policy file. Contains a list of `Policy` objects.

```yaml
policies:
  - name: pii-analyst
    # ...
```

---

### `type Policy struct`

One named policy entry as it appears in a YAML file.

| Field | YAML key | Description |
|---|---|---|
| `Name` | `name` | Human-readable unique identifier |
| `Description` | `description` | Intent description (not evaluated at runtime) |
| `Tags` | `tags` | Arbitrary labels (e.g. `[gdpr, pii]`) |
| `ComplianceTags` | `compliance_tags` | Framework tags emitted in audit log (`GDPR`, `HIPAA`, `PCI-DSS`, `CCPA`, `SOX`) |
| `Priority` | `priority` | Integer priority — higher number wins in merge conflicts |
| `Enabled` | `enabled` | Optional boolean; `false` disables without deleting. Default: enabled |
| `On` | `on` | Trigger list: `["read"]`, `["ingest"]`, or both. Empty = all triggers |
| `Match` | `match` | Matching criteria |
| `Response` | `response` | API response shape descriptor |
| `Rules` | `rules` | List of field-level rules |
| `FilterCondition` | `filter_condition` | Optional row-level filter condition expression |

---

### `type Match struct`

Defines which requests this policy applies to.

| Field | YAML key | Description |
|---|---|---|
| `Roles` | `roles` | Role list (exact match, or `*` for wildcard) |
| `Resource` | `resource` | URL path prefix (e.g. `/users` matches `/users/123`) |
| `Services` | `services` | Service name list with wildcard glob (e.g. `analytics-*`) |
| `Fields` | `fields` | Field scope constraint — rules only apply to fields in this list |
| `Sources` | `sources` | Kafka topic patterns (e.g. `kafka.topic('user-events')`) |
| `Destinations` | `destinations` | Egress destination patterns |

---

### `type Response struct`

Describes where the actual records live inside the API response envelope.

| Field | YAML key | Description |
|---|---|---|
| `DataPath` | `data_path` | gjson dot-notation path to records (empty = root). Examples: `"data"`, `"result.users"` |
| `Array` | `array` | Optional explicit array hint (nil = auto-detect from value type) |
| `Rehydrate` | `rehydrate` | If `true`, re-substitutes masked tokens in LLM responses (used with `llm_intercept`) |

**Response shape examples:**

| API Shape | `data_path` | `array` |
|---|---|---|
| `[{...},{...}]` | `""` | auto/omit |
| `{...}` | `""` | auto/omit |
| `{"data": [...]}` | `"data"` | auto/omit |
| `{"result": {"users": [...]}}` | `"result.users"` | auto/omit |
| `{"data": {...}}` | `"data"` | `false` |

---

### `type Rule struct`

Defines what to do with a single field or a set of tagged fields.

| Field | YAML key | Description |
|---|---|---|
| `Field` | `field` | Dot-notation field path (e.g. `"email"`, `"address.street"`, `"*"`) |
| `FieldsTagged` | `fields_tagged` | Tag list instead of a named field (e.g. `["@pii", "@financial"]`) |
| `Action` | `action` | Rule action (see `RuleAction` below) |
| `Condition` | `condition` | Optional condition expression (e.g. `"user.role != 'admin'"`) |
| `Length` | `length` | Used with `truncate` action — maximum character count |

---

### `type RuleAction string`

| Constant | Value | Behavior |
|---|---|---|
| `ActionMask` | `"mask"` | Partial redaction, preserving recognizable format (e.g. `***@domain.com`) |
| `ActionRemove` | `"remove"` | Delete field entirely from the response |
| `ActionAllow` | `"allow"` | Pass field through untouched (explicit allow) |
| `ActionDeny` | `"deny"` | Block the **entire** response (HTTP 403) or a specific field (field-level deny) |
| `ActionLLMIntercept` | `"llm_intercept"` | Scan request, replace PII with tokens, rehydrate response |
| `ActionAnonymize` | `"anonymize"` | One-way SHA-256 hash with configurable salt |
| `ActionRedact` | `"redact"` | Replace value(s) with literal `"[REDACTED]"` |
| `ActionTruncate` | `"truncate"` | Keep first N characters (Unicode-safe, configured by `length`) |

**Special field `"*"`:**
- `"*": deny` → blocks entire response.
- `"*": allow` → passthrough (clears any deny from lower-priority policy).
- `"*": llm_intercept` → enables LLM PII interception on the entire request body.

---

### `type EffectivePolicy struct`

The compiled, runtime-optimised ruleset for a specific policy (keyed by name in the store). This is what the transform and proxy engines work with at request time — never the raw YAML `Policy` structs.

| Field | Type | Description |
|---|---|---|
| `SourcePolicies` | `[]string` | Names of source YAML policies merged into this |
| `ComplianceTags` | `[]string` | Merged, deduplicated compliance tags |
| `Response` | `Response` | API response shape for this policy |
| `Rules` | `map[string]RuleAction` | Compiled field path → action map |
| `RuleConditions` | `map[string]string` | Field path → condition expression |
| `RuleLengths` | `map[string]int` | Field path → truncation length |
| `TaggedRules` | `map[string]RuleAction` | Tag → action map (for `fields_tagged` rules) |
| `TaggedRuleConditions` | `map[string]string` | Tag → condition expression |
| `TaggedRuleLengths` | `map[string]int` | Tag → truncation length |
| `Deny` | `bool` | `true` → short-circuit everything, return 403 |
| `Services` | `[]string` | Service name patterns |
| `Fields` | `[]string` | Permitted fields constraint |
| `Sources` | `[]string` | Ingestion source patterns |
| `Destinations` | `[]string` | Egress destination patterns |
| `Priority` | `int` | Highest priority from merged source policies |
| `Roles` | `[]string` | Matched roles |
| `Resource` | `string` | Matched resource prefix |
| `FilterCondition` | `string` | Row-level filter condition expression |
| `Triggers` | `[]string` | Execution contexts: `"read"`, `"ingest"` |

---

## 2. `loader.go`

### `type Store struct`

Thread-safe in-memory compiled policy registry.

| Field | Type | Description |
|---|---|---|
| `mu` | `sync.RWMutex` | Read-write mutex for concurrent policy access |
| `policies` | `map[string]*EffectivePolicy` | Keyed by policy name (not role::resource — each policy is its own entry) |

---

### `NewStore() *Store`

Initialises an empty policy store.

---

### `(s *Store) Lookup(role, resource, service, source, destination, trigger string) *EffectivePolicy`

Finds and merges all policies matching the provided request context. The central runtime lookup function.

**Matching algorithm (6 criteria, all must pass for a policy to be included):**

1. **Role match**: `ep.Roles` contains `role` or `"*"` wildcard.
2. **Resource prefix match**: `strings.HasPrefix(resource, ep.Resource)` — e.g. policy `"/users"` matches request `"/users/123"`.
3. **Service match** (if `ep.Services` is non-empty): calls `matchService(pattern, service)`.
4. **Source match** (if `ep.Sources` is non-empty): calls `matchSource(pattern, source)` — supports `kafka.topic('topic-name')` syntax.
5. **Destination match** (if `ep.Destinations` is non-empty): calls `matchDestination(pattern, destination)`.
6. **Trigger match** (if both `trigger != ""` and `ep.Triggers` is non-empty): exact string match in `ep.Triggers`.

**Sorting (ascending, so last entry is highest priority):**
1. By `Priority` integer (ascending).
2. Within same priority: wildcard-role policies (`"*"`) are sorted first (lower precedence); explicit-role policies last (higher precedence).

**Merging (applied in sorted order, so higher-priority policies override lower):**
- `SourcePolicies`: concatenated.
- `Response`: last non-empty `Response` block wins.
- `Deny`: any `true` sets it; last non-deny policy can clear it.
- `FilterCondition`: last non-empty wins.
- `ComplianceTags`: accumulated and deduplicated.
- `Rules` / `RuleConditions` / `RuleLengths`: later policies overwrite earlier entries for the same field.
- `TaggedRules` / `TaggedRuleConditions` / `TaggedRuleLengths`: same merge semantics.
- `Fields`: accumulated.

**Special wildcard rule handling during merge:**
- `"*": deny` → sets `merged.Deny = true`.
- `"*": allow` → sets `merged.Deny = false`.
- `"*": llm_intercept` → stored as `merged.Rules["*"] = ActionLLMIntercept`.

Returns `nil` if no policies match (caller must deny by default).

---

### `matchService(pattern, service string) bool` *(private)*

- `"*"` or `""` → always matches.
- Pattern with `*` → splits on `*`: `strings.HasPrefix(service, parts[0]) && strings.HasSuffix(service, parts[1])`. If only one `*`, uses prefix match only.
- Otherwise: exact string equality.

Examples: `"analytics-*"` matches `"analytics-prod"`, `"analytics-staging"`.

---

### `matchSource(pattern, source string) bool` *(private)*

Supports Kafka topic function syntax: `kafka.topic('topic-name')`. Regex: `^kafka\.topic\((.*)\)$`. Extracts topic name, delegates to `matchService("kafka." + topicPattern, source)`. Falls back to `matchService` for generic patterns.

---

### `matchDestination(pattern, destination string) bool` *(private)*

Supports generic function syntax: `postgres.table('my_table')`. Regex: `^([a-zA-Z0-9_]+)\.[a-zA-Z0-9_]+\((.*)\)$`. Falls back to `matchService` for generic patterns.

---

### `(s *Store) swap(next map[string]*EffectivePolicy)` *(private)*

Atomically replaces the entire policies map under a write lock. Called after `LoadDir` succeeds.

---

### `(s *Store) GetPolicies() map[string]*EffectivePolicy`

Returns a shallow copy of the policies map under a read lock. Safe for concurrent access. Used by `audit.statusHandler` to render the active policies list.

---

### `(s *Store) LoadDir(dir string) (int, error)`

Reads all `*.yaml` and `*.yml` files from `dir`. Calls `parseFile` on each. Compiles all parsed policies via `compile`. Calls `s.swap` to atomically replace the store. Returns the count of compiled effective policies.

**Validation during `parseFile`:**
- `name` must not be empty.
- `match.resource` must not be empty.
- `match.roles` must not be empty.
- `rules` list must not be empty.

---

### `(s *Store) WatchDir(dir string) error`

Starts a `fsnotify` background goroutine watching `dir` for `Write`, `Create`, or `Remove` filesystem events on YAML files. On any matching event: calls `s.LoadDir(dir)`.

- Success → logs `slog.Info("Policy reload completed successfully", "active_policies", n)`.
- Failure → logs `slog.Error("Policy reload failed — retaining active configuration")`. The store is **not** updated on failure (old policies stay active).

---

### `compile(policies []Policy) (map[string]*EffectivePolicy, error)` *(private)*

Three-phase compilation:

**Phase 1: Filter disabled policies.**
Skips any policy where `Enabled != nil && !*Enabled`. Logs `slog.Info("Skipping disabled policy configuration")`.

**Phase 2: Compile-time overlap detection.**
Builds `map[role::resource][]Policy`. For any group with 2+ policies: calls `findFieldOverlaps(a, b)` on every pair. Overlapping fields are reported as `slog.Warn("Compile-time policy overlap detected")`. This is a **warning** — policies still compile and run (higher-priority policy wins at runtime).

**Phase 3: Compile each policy into `EffectivePolicy`.**
For each rule in `p.Rules`:
- If `Field == "*"` and `Action == ActionDeny` → sets `ep.Deny = true`.
- If `Field == "*"` and `Action == ActionLLMIntercept` → stores `ep.Rules["*"] = ActionLLMIntercept`.
- If `Field != ""` → stores field rule, condition, and length.
- If `FieldsTagged != []` → stores tag rules, conditions, and lengths.

Each compiled policy is stored in the result map keyed by `p.Name`.

---

### `SplitKey(key string) (string, string, bool)`

Splits a `"role::resource"` key string into parts. Returns `(role, resource, true)` on success, `("", "", false)` if malformed. Used in `audit.statusHandler`.

---

### `findFieldOverlaps(a, b Policy) []string` *(private)*

Returns field names defined in both `a.Rules` and `b.Rules`. Used for compile-time overlap detection warnings.

---

### `dedupeSlice(in []string) []string` *(private)*

Removes duplicate strings from a slice, preserving first-occurrence order.

---

## 3. `condition.go`

### `type EvalResult int`

| Constant | Value | Meaning |
|---|---|---|
| `EvalFalse` | `0` | Condition evaluated to false |
| `EvalTrue` | `1` | Condition evaluated to true |
| `EvalIndeterminate` | `2` | A required attribute was missing; fail-closed logic applies |

---

### `type MissingAttribute struct`

Sentinel value returned by `resolveValue` when a `user.field` or `record.field` reference cannot be resolved.

```go
type MissingAttribute struct {
    Name string // The expression that could not be resolved, e.g. "user.department"
}
```

---

### `EvaluateCondition(condition string, record []byte, reqCtx types.RequestContext) (EvalResult, string)`

Parses and evaluates a condition string against a JSON record and request context.

**Supported operators** (checked in longest-first order to avoid partial matches):
`!=`, `==`, `<=`, `>=`, `<`, `>`, ` contains `, ` in `

**Parsing:**
1. Trims whitespace.
2. Empty condition → always `EvalTrue`.
3. Finds the first operator that appears in the condition string.
4. Splits into `lhs` and `rhs` at the operator position.
5. Calls `resolveValue(lhs)` and `resolveValue(rhs)`.
6. If either resolves to `MissingAttribute` → returns `EvalIndeterminate` with the missing attribute name.
7. Calls `compareValues(lhsVal, op, rhsVal)`.

---

### `resolveValue(expr string, record []byte, reqCtx types.RequestContext) interface{}` *(private)*

Resolves a single expression token to a Go value:

| Expression pattern | Resolution |
|---|---|
| `'value'` or `"value"` | Literal string |
| `true` / `false` | Boolean literal |
| `3.14` (parseable float) | `float64` literal |
| `record.<field>` | gjson.GetBytes lookup in record JSON. Returns typed value: bool, float64, `[]interface{}`, or string. Returns `nil` if field absent. |
| `user.<field>` | Looks up `reqCtx.UserAttributes[strings.ToLower(field)]`. Falls back to `reqCtx.UserID` / `reqCtx.UserRole` for `"id"`, `"userid"`, `"role"`, `"userrole"`. Returns `MissingAttribute{expr}` if not found. |
| `time.hour` | `float64(time.Now().Hour())` — current hour (0–23) |
| `['a','b']` | Array literal (single-quotes replaced with double-quotes for JSON parsing) |
| Anything else | The raw expression string |

---

### `compareValues(lhs interface{}, op string, rhs interface{}) bool` *(private)*

Multi-type comparison dispatcher:

1. **Nil handling**: For `==` and `!=`, compares by identity. Other operators return `false`.
2. **Numeric** (both `float64`): supports `==`, `!=`, `<`, `<=`, `>`, `>=`.
3. **Boolean** (both `bool`): supports `==`, `!=`.
4. **String fallback**: both values converted via `toString(v)`. Supports `==`, `!=`, `<`, `<=`, `>`, `>=`.
5. **` contains `**: If LHS is `[]interface{}`, checks if any element equals RHS via `compareValues(..., "==", ...)`. Otherwise: `strings.Contains(ls, rs)`.
6. **` in `**: If RHS is `[]interface{}`, checks if LHS equals any element. Otherwise: `strings.Contains(rs, ls)`.

---

### `toString(v interface{}) string` *(private)*

Converts `string`, `bool`, or `float64` to string. Returns `""` for all other types.

---

## Condition Expression Examples

```yaml
# Field-level condition
- field: ssn
  action: mask
  condition: "user.role != 'admin'"

# Row-level filter
filter_condition: "record.region == 'EU'"

# Time-based access control
- field: salary
  action: remove
  condition: "time.hour < 9 or time.hour >= 17"

# Array membership
- field: account_number
  action: redact
  condition: "record.department in ['finance', 'payroll']"

# LHS array contains check
filter_condition: "record.tags contains 'sensitive'"
```
