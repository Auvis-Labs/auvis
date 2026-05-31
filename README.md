<div align="center">

# Auvis

**Policy enforcement for AI data pipelines.**

Govern your data before it reaches your AI — so your AI projects don't die in the security review.

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat)](LICENSE)
[![Version](https://img.shields.io/badge/version-v0.1.0-green?style=flat)]()
[![Status](https://img.shields.io/badge/status-active%20development-orange?style=flat)]()

</div>

---

## The Problem

When companies build internal AI tools, they pull data from Confluence, Salesforce, Kafka, and databases — and feed it into vector databases or LLM context windows. The moment that happens, every access control from the source system disappears.

HR documents, salary records, customer PII, M&A discussions — all flattened into a vector database with no permissions, queryable by anyone with access to the AI tool.

> **Security teams block the project. Compliance teams have no audit trail. Legal says "not until someone proves GDPR compliance."**

Existing tools don't fix this:

| Tool | What they do | Why they miss this |
|---|---|---|
| Immuta / Privacera | Access control for Snowflake, Databricks | SQL analytics only — not ingestion pipelines |
| LLM Gateways (Portkey, Helicone) | Filter prompts at inference time | Data is already in the vector DB by then |
| DLP (Proofpoint, Forcepoint) | Watch email and endpoints | Blind to internal data flows |
| Collibra / Alation | Data catalog and metadata | Documentation, not runtime enforcement |

**Nobody enforces policies at the pipeline layer — between your source data and your AI system. That's Auvis.**

---

## What Auvis Does

Auvis sits inside your data ingestion pipelines and enforces your governance policies before data reaches a vector database, RAG system, or LLM.

```
Source Systems                    AI Infrastructure
──────────────────                ─────────────────────────
Confluence                        Vector DBs
SharePoint          ┌─────────┐   (Pinecone, Weaviate,
Kafka          ───► │  AUVIS  │ ──► pgvector, Qdrant)
PostgreSQL          │         │
Salesforce          └─────────┘   RAG Pipelines
S3 / GCS               │         LLM Context Windows
                        │
                   Audit Trail
                   Compliance Reports
```

**What happens inside Auvis on every record:**

1. **Policy evaluation** — match the record against your defined policies (<5ms)
2. **Field-level transformation** — mask, redact, tokenize, or drop sensitive fields
3. **ACL tag propagation** — carry source system permissions through to the vector index
4. **Audit logging** — record every decision, transformation, and policy match asynchronously

Zero changes to your AI application code.

---

## Features

### Policy Enforcement
- Field-level masking, redaction, tokenization, and removal
- Row-level filtering with dynamic condition evaluation
- Full-response deny with configurable alerting
- Priority-based conflict resolution — deterministic, always auditable
- Hot-reload — policy changes take effect in seconds without restart

### PII Detection
- Regex-based detection with mathematical check digit validation (Luhn, ISO 13616 Mod-97, ABA routing)
- 40-character sliding window semantic context boosting
- LLM fallback (Claude Haiku) for unstructured content — names, addresses, freeform text
- Confidence scoring (0.0–1.0) with configurable threshold
- Overlapping span resolution — highest confidence wins, everything logged
- Three scanner modes: `regex_only`, `llm_only`, `hybrid` (default)

### Access Control
- JWT claim extraction for user context (`user.region`, `user.role`, etc.)
- `X-User-*` header fallback for dev/simple deployments
- Optional IdP lookup (Okta, Auth0, LDAP) for enriched attributes
- Wildcard role policies (`*`) as catch-all fallback rules
- Specificity-based precedence — explicit role always beats wildcard

### Audit & Compliance
- Complete four-bucket field disposition per request: `fields_masked`, `fields_removed`, `fields_allowed`, `fields_denied`
- `response_denied` flag for full-response blocks
- PII category logging (`pii_types_detected`) — never raw values
- JSONL format — natively ingestible by Splunk, Datadog, ClickHouse
- Size-based and age-based log rotation
- Structured logging via `log/slog` throughout

---

## Quick Start

### Prerequisites

- Go 1.22+
- An upstream API or service to proxy

### Install

```bash
git clone https://github.com/Auvis-Labs/Auvis
cd auvis
go build -o bin/auvis ./...
```

### Run

```bash
# Start the proxy in dev mode
AUVIS_UPSTREAM=http://your-upstream:8000 \
AUVIS_PORT=8080 \
./bin/auvis dev
```

### Write your first policy

```yaml
# policies/mask-pii.yaml
policies:
  - name: mask-email-for-analysts
    description: Analysts see masked emails only
    priority: 100
    compliance_tags: [GDPR, CCPA]
    match:
      roles: [analyst]
      resource: /users
    rules:
      - field: email
        action: mask
      - field: ssn
        action: remove
      - field: phone
        action: mask
```

```bash
# Validate before deploying
./bin/auvis validate --policies ./policies/
```

Send a request through the proxy:

```bash
curl http://localhost:8080/users \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Auvis-Upstream: http://your-upstream:8000"
```

The analyst gets back:

```json
{
  "email": "***@example.com",
  "phone": "****7890",
  "ssn": null,
  "name": "Margaret Okonkwo"
}
```

And the audit log records:

```json
{
  "timestamp": "2026-05-31T01:48:51Z",
  "user_id": "usr_101",
  "user_role": "analyst",
  "resource": "/users",
  "policies_applied": ["mask-email-for-analysts"],
  "fields_masked": ["email", "phone"],
  "fields_removed": ["ssn"],
  "fields_allowed": ["name", "id", "region"],
  "fields_denied": [],
  "response_denied": false,
  "pii_types_detected": ["EMAIL", "PHONE", "SSN"],
  "upstream_status": 200,
  "duration_ms": 10
}
```

---

## Policy DSL

### Actions

| Action | What it does |
|---|---|
| `mask` | Pattern-preserving masking (`m.okonkwo@example.com` → `***@example.com`) |
| `remove` | Drop the field entirely |
| `redact` | Replace with `[REDACTED]` — hides length and format |
| `truncate` | Cut to N characters (rune-safe, UTF-8 aware) |
| `deny` | Block the entire response (403) |
| `allow` | Explicit passthrough — overrides wildcard deny |
| `llm_intercept` | Scan outbound request, mask PII, rehydrate in response |

### Pattern-Specific Masking

Auvis automatically detects field format and applies the right masking pattern:

```
Email:       m.okonkwo@example.com  →  ***@example.com
Phone:       +1-555-0198            →  ***-***-0198
SSN:         123-45-6789            →  ***-**-6789
Credit card: 4111-1111-1111-4242    →  ****-****-****-4242
```

### Conditions

Rules can be conditional — evaluated at runtime against the record and user context:

```yaml
rules:
  - field: salary
    action: mask
    condition: "user.role != 'hr'"

  - field: region_data
    action: remove
    condition: "record.region != user.region"
```

Supported operators: `==`, `!=`, `<`, `>`, `<=`, `>=`

Context variables: `user.fieldName` (from JWT), `record.fieldName` (from payload)

**Fail behavior:** If a condition can't be evaluated (missing attribute, parse error), Auvis fails closed — the rule applies. Missing attributes are never treated as empty string.

### Wildcard Roles

```yaml
# Catch-all — deny everyone not explicitly permitted
- name: deny-unmatched
  match:
    roles: ["*"]
    resource: /users
  rules:
    - field: "*"
      action: deny

# Explicit — analysts get masked data (overrides wildcard)
- name: analyst-access
  priority: 100
  match:
    roles: [analyst]
    resource: /users
  rules:
    - field: email
      action: mask
```

Explicit role policies always beat wildcard policies. No priority number needed — specificity resolves it automatically.

---

## Configuration

```yaml
# auvis.yaml
port: 8080
upstream: "http://your-service:8000"
policies_dir: "./policies"

pii_scanner:
  mode: hybrid                  # regex_only | llm_only | hybrid
  confidence_threshold: 0.70    # act on matches above this score
  llm_timeout_ms: 200           # fallback to regex if LLM exceeds this

audit:
  file: "./audit.jsonl"
  max_size_mb: 100              # rotate after 100MB
  max_age_days: 30              # purge rotated files older than 30 days

auth:
  jwt_secret: ${JWT_SECRET}
  idp_url: ""                   # optional: https://your-idp.com/userinfo
```

---

## CLI

```bash
auvis dev        # Start proxy with hot-reload and stdout logging
auvis validate   # Validate all policy files in the policies directory
auvis logs       # Tail and filter the audit log
auvis status     # Query running proxy health and metrics
auvis version    # Print version, git commit, build date
```

### Validate output

```
┌─────────────────────────────────────────────────────┐
│ Auvis Policy Validation                             │
├─────────────────────────────────────────────────────┤
│ ✓ mask-email-for-analysts    priority:100  GDPR     │
│ ✓ deny-unmatched-roles       priority:0             │
│ ✓ remove-ssn-globally        priority:200  HIPAA    │
│                                                     │
│ 3 policies loaded, 0 errors, 0 warnings             │
└─────────────────────────────────────────────────────┘
```

---

## Performance

Benchmarked on a dev laptop (Apple M-series) against a Go upstream server:

| Metric | Result |
|---|---|
| Requests/sec | 1,710 |
| Average latency | 28ms |
| p50 | 23ms |
| p95 | 60ms |
| Fastest single request | 0.9ms |
| Error rate (10,000 requests) | 0% |

The proxy's own processing overhead is sub-millisecond. End-to-end latency is dominated by upstream response time. Production hardware on a real upstream will significantly exceed these numbers.

*Tested with `hey -n 10000 -c 50` in `regex_only` mode against a Go mock upstream.*

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    AUVIS GATEWAY                         │
│                                                          │
│  ┌─────────────┐   ┌──────────────┐   ┌──────────────┐  │
│  │   Context   │   │   Policy     │   │  Transform   │  │
│  │  Resolver   │──►│   Engine     │──►│   Engine     │  │
│  │             │   │              │   │              │  │
│  │ JWT claims  │   │ Compiled     │   │ mask/remove  │  │
│  │ X-User-*    │   │ decision     │   │ redact/      │  │
│  │ IdP lookup  │   │ tree (<5ms)  │   │ truncate     │  │
│  └─────────────┘   └──────────────┘   └──────┬───────┘  │
│                                               │          │
│  ┌────────────────────────────────────────────┤          │
│  │              PII SCANNER                   │          │
│  │  Regex → Check Digit → Context Boost       │          │
│  │  LLM Fallback (200ms timeout)              │          │
│  └────────────────────────────────────────────┘          │
│                                               │          │
│  ┌────────────────────────────────────────────┘          │
│  │              AUDIT ENGINE                             │
│  │  Async JSONL writer — never blocks hot path          │
│  │  Four-bucket field disposition per request           │
│  │  Size + age-based log rotation                       │
│  └──────────────────────────────────────────────────────┘
└──────────────────────────────────────────────────────────┘
```

---

## Roadmap

**v0.1.0 — Complete ✓**
- HTTP reverse proxy with policy enforcement
- PII scanner (regex + LLM fallback + confidence scoring)
- Transform engine (mask, remove, redact, truncate, deny, allow, llm_intercept)
- Row-level filtering with condition evaluation
- JWT + header + IdP context resolution
- Wildcard role policies with specificity precedence
- Async audit logger with log rotation
- Hot-reload, `auvis validate` CLI
- Load tested at 1,710 req/sec, zero errors

**v0.2.0 — In Progress**
- Pipeline engine foundation (source → policy → transform → destination)
- Confluence source connector
- Pinecone + Weaviate + pgvector destinations
- DSL v2 (source declarations, model annotations, tag-based field matching)

**v0.3.0 — Planned**
- PostgreSQL CDC connector
- SharePoint source connector
- Python SDK (`PipelineGuard`)
- TypeScript SDK (`@auvis/sdk`)

**v0.4.0 — Planned**
- Control plane REST API
- Admin UI (policy editor + audit viewer)
- GDPR Article 30 report generator
- HIPAA access log generator

---

## Who Is Auvis For

**The AI project team blocked by security**
Your RAG system works great in the demo. Then the CISO asks "how do we know customer PII isn't being embedded in that vector database?" Auvis is your answer.

**The CISO with no visibility**
AI projects are happening — officially and unofficially. Auvis gives you a single place to define policies and an audit trail to show regulators.

**The compliance team without proof**
GDPR requires data minimization. HIPAA requires PHI access logs. PCI-DSS requires cardholder data controls. Auvis generates the reports.

---

## License

MIT — see [LICENSE](LICENSE)

---

<div align="center">
<sub>Auvis — your data is already governed. Now keep it that way when it reaches your AI.</sub>
</div>
