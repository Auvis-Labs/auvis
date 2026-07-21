# Auvis Policy Specification & Reference Guide

This document defines the complete YAML policy schema for **Auvis**. Policies dictate how API responses and data pipelines are governed, masked, filtered, and audited based on user identity, resource path, source, destination, and request context.

---

## 📐 Policy File Overview

Policy files are written in standard YAML and placed in the policies directory (configured via `policy.dir` in `auvis.yaml`). Auvis continuously hot-reloads these files at runtime.

### Complete Policy Schema Example
```yaml
id: "users-analyst-mask-pii"
description: "Mask sensitive user PII for analyst role on /users endpoint"
priority: 100

match:
  role: "analyst"
  resource: "/users*"
  service: "user-service"
  source: "kafka-user-events"
  destination: "postgres-warehouse"
  action: "read"

rules:
  email: "mask"
  phone: "mask"
  date_of_birth: "mask"
  ssn: "remove"
  internal_notes: "deny"
  credit_card: "redact"
  bio: "truncate"

rule_lengths:
  bio: 50

rule_conditions:
  email: "request.method == 'GET'"
  bio: "user.role != 'admin'"

tagged_rules:
  PII_EMAIL: "mask"
  PII_SSN: "remove"

response:
  data_path: "data"
  array: true

filter_condition: "account_status == 'active'"
compliance_tags: ["GDPR", "HIPAA", "PCI-DSS"]
deny: false

---

## 🔍 Field Specification

### 1. Metadata & Matching (`match`)

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `id` | `string` | Yes | — | Unique identifier for the policy. |
| `description` | `string` | No | `""` | Human-readable explanation of intent. |
| `priority` | `integer` | No | `0` | Precedence score when multiple policies match. Higher priority wins. |
| `match.role` | `string` | No | `"*"` | User role (e.g. `analyst`, `admin`, `support`). Supports exact string or `*`. |
| `match.resource` | `string` | No | `"*"` | API URI path or resource topic (e.g. `/users`, `/patients/*`). Supports prefix matching. |
| `match.service` | `string` | No | `"*"` | Upstream service name constraint. |
| `match.source` | `string` | No | `"*"` | Ingestion pipeline source connector ID (e.g. `kafka-main`). |
| `match.destination` | `string` | No | `"*"` | Destination sink ID (e.g. `pinecone-index`). |
| `match.action` | `string` | No | `"read"` | Operation type (`read`, `write`, `export`). |

---

## 🛠️ Field Transformation Actions (`rules`)

The `rules` map links JSON field dot-notation paths (e.g. `address.street` or `email`) to specific transformation actions:

| Action | Behavior | Example Input | Example Output |
|---|---|---|---|
| `allow` | Passthrough untouched. | `"john@example.com"` | `"john@example.com"` |
| `mask` | Pattern-aware masking. Automatically detects emails, SSNs, credit cards, phones. | `"john.doe@example.com"` | `"***@example.com"` |
| `remove` | Completely deletes the key from the output JSON object. | `{"ssn": "123-45-6789"}` | `{}` |
| `redact` | Replaces the field value with literal string `"[REDACTED]"`. | `"secret note"` | `"[REDACTED]"` |
| `truncate` | Truncate string to `rule_lengths` number of runes. | `"Long text sample"` | `"Long text..."` |
| `deny` | Drops the field and explicitly records field-level denial in audit trail. | `{"internal_id": 99}` | `{}` |
| `anonymize` | Deterministic SHA-256 hash combined with `AUVIS_SALT`. | `"usr_101"` | `"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"` |
| `llm_intercept` | Routes payload to secondary LLM fallback scanner for contextual PII detection. | `"Patient mentioned depression."` | *(Contextually redacted output)* |

### Smart Pattern Masking Rules (`mask`)
When `action: "mask"` is applied to a string, Auvis automatically evaluates value patterns:
- **Email**: `john.doe@domain.com` → `***@domain.com`
- **US SSN**: `123-45-6789` → `**-**-6789`
- **Credit Card**: `4111 2222 3333 4444` → `*** **** **** 4444`
- **Phone Number**: `+1-555-123-4567` → `***-***-4567`
- **Generic Fallback**: Values ≤ 4 chars become `****`; longer values preserve last 4 chars (`****5678`).

---

## ⚡ Conditional Enforcement (`rule_conditions` & `filter_condition`)

Auvis allows rules and row filters to trigger conditionally based on request context or field values.

```yaml
rule_conditions:
  email: "user.role != 'admin'"
  medical_history: "request.method == 'GET' and user.role == 'analyst'"

filter_condition: "account_status == 'active' and age >= 18"
```

### Supported Condition Operators

| Operator | Type | Example |
|---|---|---|
| `==`, `!=` | Equality | `user.role == 'analyst'` |
| `<`, `>`, `<=`, `>=` | Numeric comparison | `age >= 21` |
| `contains` | Substring match | `email contains '@company.com'` |
| `in` | Set membership | `region in ['US-East', 'US-West']` |
| `time.hour` | Time constraint | `time.hour >= 9 and time.hour <= 17` |

### 🔒 Fail-Closed Guarantee
If an attribute referenced in a condition is missing from both the payload and request context:
1. The condition status is marked **Indeterminate**.
2. A warning is emitted to structured logs (`Rule condition attribute missing`).
3. **Fail-closed rule enforcement is applied**: row filter drops the row, and field rule applies the masking/redaction action. Data is never leaked due to missing evaluation context.

---

## 🏷️ Dynamic PII Tag Rules (`tagged_rules`)

Instead of specifying explicit field paths, you can assign rules based on detected PII entity tags:

```yaml
tagged_rules:
  PII_EMAIL: "mask"
  PII_SSN: "remove"
  PII_PHONE: "mask"
  PII_FINANCIAL: "redact"
```

### Conflict Resolution Hierarchy
If a field matches multiple dynamic tags or explicit path rules, Auvis applies strict priority order:
1. **Explicit path rule** (`rules.address.street`) always overrides dynamic tag rules.
2. **Action Severity Priority**: `remove` (7) > `deny` (6) > `redact` (5) > `mask` (4) > `truncate` (3) > `anonymize` (2) > `allow` (1).

---

## 📜 Policy Merging & Specificity Rules

When a request matches multiple policies in the directory:
1. **Explicit Priority**: Higher `priority` values take precedence.
2. **Specificity Rule**: Policies with explicit `role` match take precedence over wildcard `*` policies.
3. **Rule Combination**: Non-overlapping field rules from all matched policies are merged into an Effective Policy (`ep`).
