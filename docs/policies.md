# Auvis Policy YAML Reference

This document is the complete reference for writing Auvis policy files. Policies control how API responses and data pipeline records are governed, transformed, and audited based on identity, resource, source, destination, and execution context.

Policy files are stored in the `policies/` directory (configured via `gateway.policies_dir` in `auvis.yaml`). Auvis performs **zero-downtime hot-reload** of the entire directory on any `.yaml` or `.yml` file change.

---

## Complete Policy File Schema

```yaml
policies:
  - name: "pii-analyst-users"              # Required: unique policy identifier
    description: "Mask PII for analysts on /users"  # Optional: human-readable intent
    priority: 100                           # Optional: higher value wins in merge order
    enabled: true                           # Optional: set false to disable without deleting
    on: ["read"]                           # Optional: "read", "ingest", or both. Empty = all

    match:
      roles: ["analyst", "support"]         # Required: role list. Use ["*"] for all roles
      resource: "/users"                    # Required: URL path prefix
      services: ["user-service", "analytics-*"]  # Optional: service name glob patterns
      sources: ["kafka.topic('user-events')"]    # Optional: pipeline source patterns
      destinations: ["postgres.table('users')"]  # Optional: pipeline destination patterns
      fields: ["email", "ssn"]             # Optional: only apply rules to these fields

    response:
      data_path: "data"                     # Optional: path to records inside envelope
      array: true                           # Optional: override auto-detection of array vs object
      rehydrate: false                      # Optional: restore PII in LLM response (for llm_intercept)

    filter_condition: "record.status == 'active'"  # Optional: row-level filter

    rules:
      - field: "email"
        action: mask
        condition: "user.clearance != 'high'"  # Optional: conditional enforcement
      - field: "ssn"
        action: remove
      - field: "address.street"
        action: redact
      - field: "bio"
        action: truncate
        length: 100                         # Characters to keep
      - field: "user_id"
        action: anonymize
      - field: "internal_notes"
        action: deny
      - field: "*"
        action: allow                       # Wildcard allow — passthrough entire response
      - fields_tagged: ["@pii", "@financial"]  # Tag-based rules
        action: mask
        condition: "user.role != 'admin'"
      - field: "*"
        action: llm_intercept               # Enables LLM PII scanning on entire request

    compliance_tags: ["GDPR", "HIPAA", "PCI-DSS"]
```

---

## Field Reference

### Top-Level Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | `string` | ✅ | Unique policy identifier. Used in audit logs and status reports. |
| `description` | `string` | — | Human-readable intent statement. Not evaluated at runtime. |
| `priority` | `int` | — | Policy priority for merge ordering. Higher number = higher precedence. Default: `0`. |
| `enabled` | `bool` | — | Set `false` to disable without deleting. Default: `true` (enabled). |
| `on` | `[]string` | — | Trigger contexts: `"read"` (HTTP proxy), `"ingest"` (pipeline). Empty = both. |
| `match` | `Match` | ✅ | Matching criteria — all specified fields must match. |
| `response` | `Response` | — | Describes response envelope shape. |
| `rules` | `[]Rule` | ✅ | Field-level transformation rules. |
| `filter_condition` | `string` | — | Row-level condition expression to filter array elements. |
| `compliance_tags` | `[]string` | — | Compliance frameworks: `GDPR`, `HIPAA`, `PCI-DSS`, `CCPA`, `SOX`. |

---

### `match` Block

| Field | Type | Description |
|---|---|---|
| `roles` | `[]string` | Role list. Accepts exact strings or `["*"]` for all roles. |
| `resource` | `string` | URL path prefix. `/users` matches `/users/123`, `/users/search`, etc. |
| `services` | `[]string` | Service name glob patterns. `"analytics-*"` matches `"analytics-prod"`, `"analytics-staging"`. Empty = any service. |
| `sources` | `[]string` | Pipeline source patterns. `"kafka.topic('user-events')"` matches Kafka topic. Empty = any source. |
| `destinations` | `[]string` | Pipeline egress destination patterns. `"postgres.table('auvis_records')"`. Empty = any destination. |
| `fields` | `[]string` | Scope constraint: only apply rules to these fields. Empty = all fields. |

**Matching semantics**: ALL specified match criteria must pass. If `services` is empty, it matches any service; same for `sources` and `destinations`.

---

### `response` Block

Describes where the actual records live inside the API response body.

| Field | Type | Description |
|---|---|---|
| `data_path` | `string` | gjson dot-notation path to the records. Empty = root of the response. Set to `"auto"` (or `"detect"`) to auto-detect common envelope wrappers (`"data"`, `"result"`, `"payload"`, etc.). |
| `array` | `bool` | Override auto-detection: `true` = treat as array of records, `false` = single object. Omit to auto-detect. |
| `rehydrate` | `bool` | If `true`, placeholders substituted in the LLM request are restored in the LLM response. Only applies when `field: "*", action: llm_intercept`. |

**Examples:**

| API Response Shape | `data_path` | `array` |
|---|---|---|
| `[{...}, {...}]` | (empty) | auto-detect or `true` |
| `{...}` | (empty) | auto-detect or `false` |
| `{"data": [{...}]}` | `"data"` | auto-detect |
| `{"result": {"users": [{...}]}}` | `"result.users"` | auto-detect |
| `{"data": {...}}` | `"data"` | `false` (required to prevent misdetection) |
| `{"payload": {...}}` | `"auto"` | auto-detect |

---

### `rules` List

Each rule is an object with the following fields:

| Field | Type | Description |
|---|---|---|
| `field` | `string` | Dot-notation field path: `"email"`, `"address.street"`, `"*"`. |
| `fields_tagged` | `[]string` | Tag-based rule: matches fields annotated with these semantic tags (e.g. `["@pii"]`). Mutually exclusive with `field`. |
| `action` | `string` | Transformation action (see table below). |
| `condition` | `string` | Optional condition expression. Rule only applies when this evaluates `true`. |
| `length` | `int` | Maximum character count for `truncate` action. |
| `seed` | `string` | Optional per-rule cryptographic seed override for `pseudonymize` action (isolates tenant/department pseudonym spaces). |

---

## Transformation Actions

| Action | What it does | Input example | Output example |
|---|---|---|---|
| `allow` | Pass field through untouched | `"john@example.com"` | `"john@example.com"` |
| `mask` | Pattern-aware partial masking | `"john@example.com"` | `"***@example.com"` |
| `remove` | Delete field entirely from output | `{"ssn": "123-45-6789"}` | `{}` |
| `redact` | Replace with literal `"[REDACTED]"` | `"sensitive note"` | `"[REDACTED]"` |
| `truncate` | Keep first N characters (Unicode-safe) | `"A long biography..."` | `"A long biograp"` |
| `anonymize` | One-way SHA-256 hash + `AUVIS_SALT` | `"user-101"` | `"e3b0c44..."` |
| `pseudonymize` | One-way deterministic format-preserving HMAC + `AUVIS_SALT` | `"amara.okonkwo@gmail.com"` | `"usr_3a9f81b2c4@gmail.com"` |
| `deny` | Drop field and log field-level denial | `{"internal_id": 99}` | `{}` |
| `llm_intercept` | Scan request body for contextual PII, mask with tokens, optionally rehydrate response | *(whole body)* | *(tokenized body)* |

### Smart Mask Patterns

When `action: mask` is applied, Auvis auto-detects the value format and applies the appropriate pattern:

| Value Pattern | Masked Output |
|---|---|
| Email: `john.doe@example.com` | `***@example.com` |
| SSN: `123-45-6789` | `**-**-6789` |
| Credit card: `4111 2222 3333 4444` | `*** **** **** 4444` |
| IBAN: `GB29 NWBK 6016 1331 9268 19` | `GB29 **** **** **** **68 19` |
| ABA Routing: `123456789` | `*****6789` |
| Passport: `A1234567` | `A1****67` |
| Phone: `+1-555-123-4567` | `***-***-4567` |
| Generic (≤4 chars): `"abc"` | `"****"` |
| Generic (>4 chars): `"John Doe"` | `"****Doe"` (last 4 preserved) |
| Numbers, booleans, null | Passed through unchanged |

### Pseudonymization (`action: pseudonymize`)

`action: pseudonymize` produces **deterministic, format-preserving, one-way pseudonyms** powered by `HMAC-SHA256(seed, value)`.

> [!NOTE]
> **Key Differences:**
> - **NOT Reversible:** Unlike `tokenize`, `pseudonymize` uses a one-way keyed cryptographic HMAC. There is no vault or lookup database to reverse a pseudonym back to original PII.
> - **Distinct from `mask`:** While `mask` applies static pattern masking (e.g. `***@domain.com`) that cannot be correlated across requests, `pseudonymize` guarantees that the **same input value always produces the exact same pseudonymous output**.
> - **Primary AI / LLM Use Case:** When feeding records into vector databases or LLM prompts, AI agents need to track entity relationships (e.g., correlating Patient #101 across 50 medical notes) without ever exposing real PII names or SSNs to the model.

#### Pseudonym Format Preservation Rules

| Field Format | Input Example | Pseudonymized Output Example |
|---|---|---|
| Email | `"amara.okonkwo@gmail.com"` | `"usr_3a9f81b2c4@gmail.com"` |
| SSN | `"123-45-6789"` | `"999-42-8819"` (uses US reserved non-issued 999 series) |
| Phone | `"+1-555-234-5678"` | `"+1-555-0142"` (uses US reserved 555-01XX test range) |
| ID / UUID | `"usr_101"` | `"usr_psd_3a9f81b2"` |
| Generic String | `"John Doe"` | `"Pseudo_3a9f81b2"` |

#### Salt / Seed Isolation Architecture

* **Global Seed (Default):** Uses `AUVIS_SALT` (environment variable or `gateway.salt` in `auvis.yaml`). All policies and services using the global seed will produce identical pseudonyms for identical input values, enabling cross-dataset joins and tracking.
* **Per-Rule Seed Override (`seed` parameter):** You can supply an optional `seed` parameter on any rule to isolate the pseudonym space for a specific team, tenant, or department:

```yaml
rules:
  - field: "patient_id"
    action: pseudonymize
    seed: "dept-oncology-secret-salt"  # Yields distinct pseudonyms isolated to this rule
```

### `field: "*"` (Wildcard Field)

The wildcard field is a **policy-level directive**, not a regular field rule:

- `field: "*", action: allow` → Passthrough entire response (no transform). Clears any deny from lower-priority policies.
- `field: "*", action: deny` → Block entire response (HTTP 403).
- `field: "*", action: llm_intercept` → Enable LLM PII scanning on the full request body.

---

## Conditional Rules

Both field rules and row filters support conditional expressions.

### Condition Syntax

```yaml
# Field-level condition
- field: ssn
  action: mask
  condition: "user.role != 'admin'"

# Row-level filter
filter_condition: "record.region == 'EU'"

# Time-based condition
- field: salary
  action: remove
  condition: "time.hour < 9"

# Set membership
filter_condition: "record.department in ['finance', 'payroll']"

# Array contains
filter_condition: "record.tags contains 'sensitive'"
```

### Supported Operators

| Operator | Description | Example |
|---|---|---|
| `==` | Equality | `user.role == 'analyst'` |
| `!=` | Inequality | `record.status != 'deleted'` |
| `<`, `>`, `<=`, `>=` | Numeric comparison | `record.age >= 18` |
| ` contains ` | String/array containment | `record.tags contains 'pii'` |
| ` in ` | Set membership | `user.region in ['US', 'EU']` |

### Value Resolver Prefixes

| Prefix | Resolves from | Example |
|---|---|---|
| `record.` | JSON field from the response record | `record.consent`, `record.department` |
| `user.` | User identity attributes (headers, JWT, IdP) | `user.role`, `user.clearance_level` |
| `time.hour` | Current hour (0–23) | `time.hour >= 9` |
| `'string'` | String literal | `'analyst'` |
| `true`/`false` | Boolean literal | `true` |
| `[1, 2, 3]` | Array literal | `['US', 'EU']` |
| Numeric | Float64 literal | `18`, `3.14` |

### Fail-Closed Guarantee

If an attribute referenced in a condition is missing:
1. Condition is marked **Indeterminate**.
2. `slog.Warn("Rule condition attribute missing — fail-closed rule enforcement applied")` is emitted.
3. **The rule fires anyway** for field conditions; **the row is excluded** for filter conditions.
4. This ensures data is never exposed due to missing evaluation context.

---

## Dynamic Tag Rules (`fields_tagged`)

Tag-based rules apply to fields annotated with semantic tags, instead of explicit paths. Useful for schema-level tagging systems.

```yaml
rules:
  - fields_tagged: ["@pii", "@email"]
    action: mask
  - fields_tagged: ["@financial", "@ssn"]
    action: remove
```

### Conflict Resolution (when a field matches multiple tags or explicit rules)

1. **Explicit `field:` rules always override `fields_tagged:` rules** (exact path wins).
2. Among competing tag rules: **highest severity action wins**:

   `remove` (7) > `deny` (6) > `redact` (5) > `mask` (4) > `truncate` (3) > `anonymize` (2) > `allow` (1)

---

## Policy Merging

When a request matches multiple policies, they are merged in priority order.

**Merge rules:**
1. **Sort**: Policies sorted by `priority` ascending (lower priority first). Within same priority, wildcard-role (`"*"`) policies sort before explicit-role policies.
2. **Apply in order** (later = higher precedence):
   - `Response` block: last non-empty wins.
   - `Deny`: any `true` sets it; last non-deny policy can clear it.
   - `FilterCondition`: last non-empty wins.
   - `ComplianceTags`: accumulated and deduplicated.
   - Field rules: later policy's rule for the same field overwrites earlier.
3. **Result**: A single `EffectivePolicy` with merged rules applied to every matching request.

**Compile-time overlap warnings**: If two policies cover the same `role + resource` combination and define rules for the same field, Auvis emits a warning at startup:
```
slog.Warn("Compile-time policy overlap detected — multiple policies act on same fields")
```

---

## LLM Intercept Mode

When `field: "*", action: llm_intercept` is configured, Auvis activates a full request/response interception loop:

1. **Request**: Reads the entire request body. Runs PII scanning (regex + optional Gemini LLM fallback). Replaces PII values with tokens (`[EMAIL_1]`, `[SSN_1]`). Forwards tokenized request to upstream LLM.
2. **Response**: If `response.rehydrate: true`, scans the response for placeholder tokens and replaces them with original values before returning to the client.

**Required**: `GEMINI_API_KEY` environment variable must be set for the LLM fallback to activate. Without it, only regex scanning runs.

```yaml
- name: "llm-pii-protect"
  match:
    roles: ["*"]
    resource: "/v1/chat/completions"
  response:
    rehydrate: true
  rules:
    - field: "*"
      action: llm_intercept
  compliance_tags: ["GDPR"]
```

---

## Compliance Tags

Compliance tags are included in every audit entry. Use them to filter and report by framework.

| Tag | Framework |
|---|---|
| `GDPR` | EU General Data Protection Regulation |
| `HIPAA` | US Health Insurance Portability and Accountability Act |
| `PCI-DSS` | Payment Card Industry Data Security Standard |
| `CCPA` | California Consumer Privacy Act |
| `SOX` | Sarbanes-Oxley Act |

---

## Example Policies

### Mask PII for Analysts

```yaml
policies:
  - name: "pii-analyst"
    description: "Mask PII fields for analyst role on /users"
    priority: 100
    on: ["read"]
    match:
      roles: ["analyst"]
      resource: "/users"
    response:
      data_path: "data"
    rules:
      - field: "email"
        action: mask
      - field: "ssn"
        action: remove
      - field: "phone"
        action: mask
    compliance_tags: ["GDPR", "HIPAA"]
```

### Time-Gated Access

```yaml
policies:
  - name: "after-hours-deny"
    priority: 200
    match:
      roles: ["contractor"]
      resource: "/financials"
    rules:
      - field: "*"
        action: deny
        condition: "time.hour < 9 or time.hour >= 18"
```

### Pipeline Ingestion Masking

```yaml
policies:
  - name: "kafka-pii-ingest"
    priority: 50
    on: ["ingest"]
    match:
      roles: ["pipeline-worker"]
      resource: "user-events"
      sources: ["kafka.topic('user-events')"]
      destinations: ["postgres.table('auvis_records')"]
    rules:
      - fields_tagged: ["@pii"]
        action: anonymize
    compliance_tags: ["GDPR"]
```
