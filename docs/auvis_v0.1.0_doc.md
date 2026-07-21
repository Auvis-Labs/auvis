# Auvis — Architecture & Design Decisions

> Living document. Updated as build decisions are made.
> Start date: May 2026

## Version History

| Version | Date | What it covers |
|---|---|---|
| v0.1.0 | May 31, 2026 | Phase 1 complete — HTTP reverse proxy, policy engine, PII scanner, transform engine, audit logger, load tested at 1,710 req/sec |

---

## What Auvis Is

Auvis is a policy enforcement layer that governs data **before it reaches your AI system.**

It sits inside data ingestion pipelines — between source systems (Kafka, databases, document stores) and AI infrastructure (vector databases, RAG pipelines, LLMs) — enforcing field-level masking, row-level filtering, classification tagging, and complete audit logging. Zero changes to the AI application code.

**One-line pitch:**
> "We enforce your existing data policies on the pipelines that feed your AI — so your AI projects don't die in the security review."

---

## The Core Problem

When companies build internal AI tools, they pull data from Confluence, Salesforce, Kafka, databases — and feed it into vector databases or LLM context windows. The moment that happens, every access control from the source system disappears.

HR docs, salary data, customer PII, M&A discussions — all flattened into a vector database with no permissions, queryable by anyone with access to the AI tool. Security teams block the project. Compliance teams have no audit trail. Legal says "not until someone proves GDPR compliance."

**Why existing tools miss this:**
- Immuta / Privacera — govern Snowflake/Databricks queries, not ingestion pipelines
- LLM gateways (Portkey, Helicone) — filter prompts at inference time, data is already in the vector DB by then
- DLP tools (Proofpoint, Forcepoint) — watch email and endpoints, blind to internal data flows
- Collibra / Alation — documentation of policies, not runtime enforcement

The gap is the **pipeline layer** — between source systems and AI infrastructure. Nobody owns it cleanly. That's Auvis.

---

## What's Been Built (Current State)

Based on the existing codebase (`auvis-infra`):

### Working
- **Priority-Based Conflict Resolution**: The compilation logic in `policy/loader.go` resolves field rules, denies, and response shape block conflicts using policy priorities (higher priority overrides lower priority silently; equal priority conflicts log warnings).
- **Runtime Condition Evaluation**: Evaluates rule conditions (`EvaluateCondition` in `policy/condition.go`) at runtime against actual JSON records and user contexts, supporting strings, booleans, floats, and logic operators.
- **Context Resolution Chain**: Dynamically resolves user attributes in order of precedence: JWT payload claims (decoded from `Authorization`), explicit `X-User-*` headers, and an optional HTTP-based Identity Provider (IdP) lookup.
- **Fail-Closed Indeterminate Evaluator**: Safeguards undefined variables by treating missing attributes as `EvalIndeterminate`, logging custom diagnostic messages with request IDs, and failing closed (applying rules) to prevent accidental data leaks.
- **Enhanced `auvis validate` CLI**: Automatically parses and validates `compliance_tags` (enforcing GDPR, HIPAA, PCI-DSS, CCPA, SOX), custom free-form categorization `tags`, catches duplicate priorities on the same resource paths (emits validation warnings), catches identical structural priority conflicts (throws validation errors), and outputs a beautifully styled dashboard detailing each policy's characteristics.
- **Dynamic Multi-Constraint Matching (Services & Fields)**: The Lookup engine supports wildcard `services` matches (e.g. `analytics-*`) and permitted `fields` constraint lists, enabling precise microservice-specific security targeting.
- **Strict Structural Load-Time Conflict Errors**: Compilation strictly rejects ambiguous overlaps where different policies share identical role, resource, and priority targets with a hard error rather than a warning.
- **HTTP Reverse Proxy with Policy Enforcement**: Dynamic middleware layers governed by the compiled effective policies.
- **Policy YAML Parser**: DSL v1 flat syntax parser.
- **Transform Engine**: Applies `mask`, `remove`, `allow`, `deny`, `llm_intercept`, `redact`, and `truncate` actions.
- **Row-Level Array Filtering**: Evaluates `filter_condition` dynamically per-record inside JSON arrays (e.g., `record.consent == true` or `record.region == user.region`), failing closed on indeterminate resolution states (excluding the row), counting and writing `RowsFiltered` to the JSONL audit trails.
- **Nested Leaf-Level Protection**: Automatically traverses complex nested objects and arrays (`collectLeafPaths` in `transform.go`) to mask, redact, or truncate target primitive leaf attributes individually while preserving the parent structural schema envelope.
- **PII Scanner**: Advanced scanner with regex patterns and LLM fallback (Claude/Gemini models).
- **Async Audit Logger**: High-performance JSONL audit trails with row-filtering metrics and asynchronous size/age-based log rotation.
- **Hot-Reload**: Safe, concurrent filesystem watchers for hot-reloading policy directories instantly without gateway downtime.
- **Complete CLI Control Suite**: Fully operational `auvis dev` (development proxy engine), `auvis logs` (real-time filtering and tailing of audit logs by user/role/resource), and `auvis status` (dynamic gateway telemetry showing uptime, request throughput, and active policies).
- **E2E LLM Interceptor & Rehydration**: Outbound prompt scanning and masking of PII using placeholders with matching inbound answer rehydration (if configured).
- **High-Performance Gateway Optimizations**: Concurrent connection pooling (`http.Transport` with idle reuse) and semaphore-bounded concurrency control on downstream LLM integrations preventing queue starvation.

### Partially Implemented
- None — all core abstractions and components are fully realized.

### Stub / Incomplete
- None — reverse proxy, scanning, parsing, log rotation, and validation commands are 100% complete and fully verified.

---

## Key Architecture Decisions

---

### Decision 1: Proxy vs Pipeline Engine

**Context:** The existing codebase is an HTTP reverse proxy — it sits between a client and an upstream API, intercepting responses. The Auvis product vision requires a pipeline engine that processes data at ingestion time (Kafka → vector DB, Confluence → RAG, etc.).

**Decision: Option A — keep both, build alongside each other.**

The proxy becomes the **HTTP API source connector** — a legitimate integration path for customers who want to govern REST API responses. The pipeline engine is built as a separate, new abstraction. They share the same policy engine, transform engine, and audit logger.

Rationale: Option B (refactor proxy into a connector type) costs 3–4 weeks of architectural work before shipping anything new. Option A keeps a working product that can demo while the pipeline engine gets built.

---

### Decision 2: Kafka Deprioritized

**Context:** Original implementation plan had Kafka as the first source connector in Phase 2.

**Decision: Build Confluence → vector DB pipeline first. Add Kafka later.**

Rationale:
- Kafka requires infrastructure to test against (can't just run locally easily)
- The team has no Kafka experience yet
- The most common enterprise RAG pipeline right now is document stores (Confluence, SharePoint) → vector DB — not Kafka
- Kafka is the right connector for a large enterprise customer who already has it. That's customer 3 or 4, not the first design partner.
- The pipeline engine abstraction doesn't change when Kafka is added — it's just another source connector

Add Kafka when a real customer asks for it.

---

### Decision 3: DSL Versioning

**Context:** Current DSL (v1) is flat YAML — role + resource + rules. The Auvis pipeline vision needs a richer DSL (v2) with source declarations, model annotations, destination routing, and tag-based field matching.

**Decision: Support both. No breaking changes to v1.**

v1 stays valid for HTTP proxy policies. v2 is new syntax for pipeline policies. Same underlying policy engine compiles both.

```yaml
# v1 — still valid, HTTP proxy use case
policy: mask-email
match:
  field: email
  roles: [analyst]
action: mask

# v2 — new, pipeline use case
policy MaskPIIBeforeEmbedding {
  on: ingest
  match {
    source: kafka.topic("customer-events")
    fields_tagged: [@pii]
  }
  action: mask
}
```

---

### Decision 4: Condition Evaluator Design

**Status: Fully Implemented** (See [policy/condition.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/policy/condition.go) and [policy/condition_test.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/policy/condition_test.go))

**What it supports:**
```
[left side] [operator] [right side]
```
- Left/right sides: `record.fieldName`, `user.fieldName`, string literals (`'EU'`), booleans (`true`/`false`), numbers (`18`)
- Operators: `==`, `!=`, `<`, `>`, `<=`, `>=`
- `record.fieldName` — extracted dynamically from the JSON payload via `gjson`
- `user.fieldName` — extracted from the resolved user context (see Decision 5)

**Implementation:** Built custom, sandboxed parsing and evaluation engine (`policy.EvaluateCondition`) that enforces sandboxed execution and parses fields safely without arbitrary scripts, ensuring high throughput and compliance security.

---

### Decision 5: User Context Resolution

**Status: Fully Implemented** (See [proxy/context.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/proxy/context.go))

**Context Resolution Chain Precedence:**
```
1. JWT claims      — decode unverified payload from Authorization: Bearer <token>
2. X-User-* headers — explicit headers as fallback / development mode override
3. IdP lookup      — dynamic HTTP querying to external IdP service (configured via IdpURL)
4. Default/Fallback— auto-maps role/id to protect standard flows
```

**Why not just require headers:**
Real-world APIs typically supply Bearer tokens. Requiring customers to retrofit upstream services with dynamic custom headers like `X-User-Region` creates friction. Supporting JWT claims as a first-class source allows instant out-of-the-box metadata support.

---

### Decision 6: Fail Behavior on Condition Errors

**Status: Fully Implemented** (See [transform/transform.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/transform/transform.go))

**Missing Attribute / Indeterminate Behavior:**
A missing attribute (e.g. `user.region` not resolved through JWT, headers, or IdP) is **not** treated as an empty string `""` because that would yield incorrect boolean results under operators like `!=`.

Instead, the evaluator returns a special `EvalIndeterminate` code:
```
attribute not found
→ condition is "indeterminate"
→ fail closed: apply the rules (mask/remove/deny)
→ audit log warning: "[TRANSFORM] request_id=... — condition skipped — user.region not resolved (fail-closed, rule applied)"
```
This guarantees that unresolved compliance parameters always fail securely (closed) and alert the system engineer via logs.

---

### Decision 7: Conflict Detection Levels

**Status: Fully Implemented**

- **Level 1 — Parse/load time (structural)**: Catch duplicate policy names (hard error), duplicate priorities on the identical resource paths (warns author during `auvis validate`), and strict structural conflicts where overlapping `role::resource` matches share identical priorities (hard validation error during compilation/validation).
- **Level 2 — Compile time (logical overlap)**: Unpredictable overlaps are resolved deterministic-first during compilation via policy priority order in [policy/loader.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/policy/loader.go).
- **Level 3 — Runtime (data-dependent)**: Conditions depending on actual values (e.g. regions/consent status) are evaluated live at runtime, executing fail-closed logic under metadata resolution errors.

---

### Decision 8: Priority Semantics

**Context:** Are all policies equally important? Why does priority exist?

**Priority doesn't mean importance — it means determinism under conflict.**

All policies need to be enforced. Priority only answers: *what happens when two valid policies try to do different things to the same field at the same moment?*

Without priority: unpredictable behavior — whichever policy loads first wins, or an error is thrown.

With priority: always deterministic, always auditable.

**Rule of thumb for policy authors:**
- Hard compliance blocks (deny all PCI data) → highest priority (e.g. 1000)
- Tenant isolation rules → always highest priority (e.g. 10000)
- Time/location-based restrictions → high priority (e.g. 500–900)
- Role-based masking → medium priority (e.g. 100–400)
- Default allow rules → lowest priority (e.g. 1–50)

---

### Decision 9: Pattern-Specific PII Masking

**Status: Fully Implemented** (See [transform/transform.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/transform/transform.go) and [transform/mask_test.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/transform/mask_test.go))

**Context:** Standard generic masking overrides every string value with `****last4`. While secure, this destroys value structures for heavily audited data types (like phone numbers, SSNs, and credit cards) that need to retain specific formatting characters (hyphens/spaces/brackets) to pass schema validations or downstream ingestion rules.

**Decision: Pattern-Specific Masking Dispatcher**
The transform engine matches values against specific pattern regexes at runtime and applies tailored masking rules:
- **SSN** (`123-45-6789` → `**-**-6789`): Masks the first two digit groups, leaving the hyphens and last 4 digits intact.
- **Phone** (`+1-555-0198` → `**-***-0198`, `(555) 555-0198` → `(***) ***-0198`): Generically identifies separator structures and country codes, converting all digits/letters before the last 4 digits to asterisks while retaining original hyphens, spaces, and brackets.
- **Credit Card** (`4111-1111-1111-4242` → `***-****-****-4242`, raw 16-digit cards → `************4242`): Preserves separators, replacing digits prior to the last 4 block with asterisks.

---

### Decision 10: Row-Level Array Filtering

**Status: Fully Implemented** (See [transform/transform.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/transform/transform.go))

**Context:** Compliance requirements (like GDPR Article 17 "Right to Erasure" or dynamic consent rules) often require completely excluding records (rows) inside data arrays rather than just masking specific fields (e.g., removing user rows where `record.consent == false`).

**Decision: Live Row Filtering with Fail-Closed Safety**
Inside JSON arrays (e.g. lists of records or objects), the transform engine evaluates the policy's `filter_condition` dynamically for every entry.
*   **Dynamic Evaluation**: Evaluates rule expressions per-row against the individual record and the live user request context (e.g., `record.region == user.region`).
*   **Fail-Closed on Indeterminate**: If dynamic attributes used in the filter cannot be resolved (returning `EvalIndeterminate`), the row is safely excluded (fail-closed) and a diagnostic warning is logged.
*   **Audit Integration**: Tracked and audited at the request level, updating the `rows_filtered` field of the async JSONL audit log to provide precise security auditing of excluded rows.

---

### Decision 11: Advanced Transform Actions (Redact & Truncate)

**Status: Fully Implemented** (See [transform/transform.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/transform/transform.go))

**Context:** The standard `mask` action retains the original length of the data (helping passing downstream validations). However, certain situations demand stronger obfuscation boundaries:
1.  **Zero-Leak Obfuscation**: Hiding even the metadata/length clues of sensitive data.
2.  **Tail-end PII Leaks**: Protecting long text blocks (where users often type PII at the end) by restricting length.

**Decision: Redact & Truncate Pipeline Actions**
We added two new high-performance transform actions to the DSL and execution pipeline:
*   **Redact (`redact`)**: Replaces the targeted field value (or all primitive leaf values recursively if nested) with a fixed, static `[REDACTED]` string. This completely conceals string length or format.
*   **Truncate (`truncate`)**: Cuts long text values down to a compiled `length` parameter.
    *   **Rune-safe Slicing**: Slices strings at the rune/character level rather than byte-level to prevent breaking multi-byte UTF-8 characters (like non-English scripts and emojis).
    *   **Recursive Nesting**: Traverses deep structures recursively, truncating primitive leaf strings while preserving the schema envelope.

---

### Decision 12: Production-Ready Asynchronous Log Rotation (Size & Age rollover)

**Status: Fully Implemented** (See [audit/logger.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/audit/logger.go) and [audit/logger_test.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/audit/logger_test.go))

**Context:** Compliance audit logs under heavy RAG ingestion pipeline usage grow exceptionally fast, presenting a threat of server disk space exhaustion.

**Decision: Thread-Safe Async Rotating Logger**
We designed a high-performance rotating log writer directly within the background event loop of the asynchronous `audit.Logger` queue consumer:
*   **Size-based Rollover**: When the active `audit.jsonl` file size exceeds the configured megabyte limit (e.g. `audit_max_size_mb`), it renames the active file to `audit-YYYYMMDD-HHMMSS.jsonl` and initiates a fresh active file.
*   **Age-based Cleanup**: Instantly sweeps the log directory upon rollover, purging expired rotated log files older than a customizable threshold (e.g. `audit_max_age_days`) to maintain a clean filesystem footprint.

---

### Decision 13: Standard Structured Logging (`log/slog`)

**Status: Fully Implemented** (See [cmd/dev.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/cmd/dev.go) and [transform/transform.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/transform/transform.go))

**Context:** Unstructured raw print lines (`log.Printf`, `fmt.Println`) cannot be reliably parsed by centralized monitoring platforms or log shippers (Splunk, Datadog, ELK).

**Decision: Standard library `log/slog`**
Standardized all server components to use Go's standard library structured logging (`log/slog`). It outputs structured, machine-parseable log entries, providing clear request tracing and hot-reload diagnostics without adding external library overhead.

---

### Decision 14: Mathematical Check Digit Validations

**Status: Fully Implemented** (See [pii/regex.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pii/regex.go) and [pii/confidence_test.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pii/confidence_test.go))

**Context:** Raw regular expressions matching travel, banking, and credit card identities are highly prone to false positives. For instance, any random 16-digit order identifier matches credit card patterns, and any 9-digit serial number matches American banking transit (routing) patterns. Blocking or masking these incorrect hits destroys harmless analytical telemetry, while ignoring them leads to data leaks.

**Decision: Algorithmic Validation of Candidate Entities**
To solve the false-positive problem with mathematical certainty, the PII scanning engine overlays regex capture groups with check digit algorithms prior to finalizing identity detection:
*   **Credit Cards (CARD)**: Evaluates candidate numeric patterns using the **Luhn Mod-10 algorithm**. Genuine card numbers are assigned high confidence (`0.85`), whereas non-conforming numbers drop to low confidence (`0.20`), bypassing automatic blocks.
*   **International Bank Accounts (IBAN)**: Implements standard **ISO 13616 Mod-97** validation. Rearranges candidate IBAN arrays, translates alpha characters into numeric values, and verifies the mod-97 check digit. Validations that succeed get a base confidence of `0.90`; invalid strings fall to `0.15`.
*   **ABA Routing (ROUTING_NUMBER)**: Validates standard 9-digit American bank routing keys via the federal mod-10 transit formula: `(3*(d1+d4+d7) + 7*(d2+d5+d8) + (d3+d6+d9)) % 10 == 0`. Matches that pass get boosted to `0.70` confidence; matches failing mod-10 drop to `0.15`.

---

### Decision 15: Sliding Window Semantic Context Boosting

**Status: Fully Implemented** (See [pii/regex.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pii/regex.go) and [pii/confidence_test.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pii/confidence_test.go))

**Context:** Certain identity records (e.g. passport numbers or national security IDs) lack structural checksums, consisting of raw alphanumeric string formats. These patterns are heavily overlapped by common internal references, like invoice numbers or parts inventory identifiers. Relying exclusively on regex yields excessive false positives, yet lowering scanning sensitivity opens security loopholes.

**Decision: 40-Character Sliding Semantic Window**
To determine when a candidate string represents actual PII, the scanner inspects a sliding window of **40 characters surrounding the match boundaries (left and right)**:
*   **Dynamic Boosting**: If the window contains relevant context keywords (e.g. "passport", "tax id", "travel doc", "bank"), the engine dynamically boosts confidence by up to `+0.55` (up to a ceiling of `0.95`).
*   **Confidence Scoring Integration**: All entities carry a rounded `confidence` metadata field (`float64` in `Entity`), capped at two decimal places to prevent float serialization artifacts in JSONL audit logs. This enables downstream policies to dynamically determine block actions based on target confidence thresholds (e.g., alert on confidence `>= 0.50`, drop/mask on `>= 0.80`).

---

### Decision 16: Overlapping Span Resolution via Confidence Scores

**Status: Fully Implemented** (See [pii/regex.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pii/regex.go) and [pii/confidence_test.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pii/confidence_test.go))

**Context:** During text scanning, different regular expressions can match identical or overlapping character spans. For example, a 9-digit numeric sequence matches the `ROUTING_NUMBER` pattern (e.g. `\b\d{9}\b`) and simultaneously matches the alphanumeric `PASSPORT` travel document pattern (e.g. `\b[A-Z0-9]{6,9}\b`). If both match entities are retained, downstream pipelines would apply two different masking or policy actions to the identical text block, yielding formatting corruption or structural proxy conflict blocks.

**Decision: Highest Confidence Overlap Winner Selection**
To handle overlapping entity spans deterministically:
*   **Confidence Priority Ordering**: The deduplication scanner sorts all candidate matches by confidence score descending, falling back to longest character length descending, and then earliest start position ascending.
*   **Iterative Overlap Pruning**: Iterates through the sorted queue, accepting candidate spans only if they do not overlap with any previously accepted (higher confidence) spans. Overlapping lower-confidence entities are discarded.
*   **Telemetric Transparency**: Discarded overlapping entities are written to the structured log stream using `slog.Info` logs (carrying `winner_type`, `winner_confidence`, `discarded_type`, `discarded_confidence`, `winner_span`, and `discarded_span` fields). This guarantees that both matches remain fully auditable in the system telemetry logs, while the pipeline downstream receives a clean, single-labeled entity.

---

### Decision 17: LLM PII Scanning Result Cache

**Status: Fully Implemented** (See [pii/llm.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pii/llm.go) and [pii/confidence_test.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pii/confidence_test.go))

**Context:** The LLM fallback scanner calls external APIs (Gemini) to parse unstructured or ambiguous fields (like names or complex addresses) that static regex rules fail to reliably catch. In continuous ingestion loops or multi-turn conversational request flows, users frequently repeat identical sentences or blocks of text. Making redundant, blocking network HTTP requests to external foundation models for identical text is highly inefficient, costly, and introduces severe latency overhead.

**Decision: Thread-Safe In-Memory Cache**
To optimize performance and eliminate redundant remote API requests, we implemented a robust, thread-safe in-memory cache directly inside the LLM scan module:
*   **Safe Concurrent Access**: Leverages a global `sync.RWMutex` to serialize access to an in-memory key-value map mapping candidate texts to their detected entity slices.
*   **Immediate Cache Returns**: Prior to initiating any external network payload marshaling, the scanner performs R-locked lookups on the map. If a match is found, the cached `[]Entity` is returned immediately with zero network latency.
*   **Complete Outcome Coverage**: Cache writes are performed under full write-locks upon receiving successful API responses, as well as on graceful empty/error fallback returns, protecting subsequent calls from redundant round-trips.

---

### Decision 18: Derived 200ms Timeout and Regex Fallback

**Status: Fully Implemented** (See [pii/scanner.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pii/scanner.go) and [pii/confidence_test.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pii/confidence_test.go))

**Context:** The reverse proxy governs incoming REST requests in runtime. While the regex-based scanning pass is synchronous and takes less than a millisecond, falling back to an external LLM API introduces unpredictable network latency and round-trip times (often exceeding several seconds under peak traffic or service degradation). Under high-throughput environments, a single degraded remote API call could block the proxy pipeline, yielding a request backlog and service-wide latency spike.

**Decision: Derived 200ms Timeout Context with Fail-Safe Fallback**
To enforce strict, predictable latency bounds on the proxy ingestion path, we introduced a strict timeout context for all LLM calls:
*   **Derived Timeout Context**: The scanner derives a sub-context with a strict `200ms` timeout (`context.WithTimeout`) from the request context before invoking the LLM scan.
*   **Fail-Safe Fallback**: If the external API call fails, or times out (returning `context.DeadlineExceeded` at exactly `200ms`), the system catches the error gracefully, prints a warning to the standard log stream, and immediately returns the high-confidence regex-only results. The request proceeds without blocking the client.

---

### Decision 19: PII Telemetry inside the Ingestion Audit Trail

**Status: Fully Implemented** (See [audit/types.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/audit/types.go) and [proxy/handler.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/proxy/handler.go))

**Context:** Compliance officers and security engineers must be able to audit *which* types of PII were scanned and detected in transit. However, logging the actual sensitive text values (such as names or credit card digits) directly to centralized log collectors violates GDPR / PCI storage regulations, creating secondary data exposure risks.

**Decision: Non-Sensitive PII Category Auditing**
To satisfy audit compliance without leaking PII:
*   **Encapsulated Metadata Scanning**: The scanner compiles the list of unique matched PII labels directly using a dedicated `PiiTypesDetected []string` field on the audit entry, avoiding logging the actual raw content value.
*   **JSONL Integration**: The array is serialized under `pii_types_detected` directly inside the standard asynchronous JSONL audit trail. This enables compliance tools to trace the presence of sensitive categories (e.g. `EMAIL`, `CARD`) in the stream while maintaining a zero-leak storage standard.

---

### Decision 20: Configurable Multi-Mode PII Scanner (Regex, LLM, Hybrid)

**Status: Fully Implemented** (See [pii/scanner.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pii/scanner.go) and [pii/confidence_test.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pii/confidence_test.go))

**Context:** Different downstream ingestion channels carry distinct performance constraints and security targets. Real-time proxy microservices might prioritize sub-millisecond response latency, preferring simple static rules. Outbound email or critical compliance endpoints might prioritize precision, accepting API latency to verify names and addresses via a large language model fallback.

**Decision: Three-tier Configurable Scanner Modes**
We introduced a configurable `Mode` setting on the PII Scanner allowing callers to target their exact security-latency envelope:
*   **`regex_only`**: Bypasses LLM fallback entirely. Delivers instantaneous scanning performance (typically `<1ms`) with zero remote API costs, relying on Luhn/ISO checksum-validated regex rules.
*   **`llm_only`**: Skips the regex layer. Directly triggers the context-bounded LLM pass to leverage advanced model semantics on raw inputs.
*   **`hybrid` (Default)**: Combines both worlds. Executes regex scanning first, masks high-confidence entities to preserve data privacy, and sends the remaining content to the LLM within a strict 200ms window, capturing named and unstructured entities safely.

---

### Decision 21: Python-Based End-to-End Integration Test

**Status: Fully Implemented** (See [test-server/e2e_test.py](file:///Users/Cridiv/Documents/Auvis/test-server/e2e_test.py))

**Context:** Unit and component tests alone cannot verify that the full request lifecycle works correctly — JWT extraction, policy lookup, upstream proxying, response transformation, field masking, and audit logging all interact. We needed a black-box test that drives the real binary end-to-end.

**Decision: Standalone Python Script Against Real Processes**
We implemented `e2e_test.py` in the `test-server/` directory using only Python stdlib. The test:
1. **Pre-flight cleanup** — `pkill -f auvis` and `pkill -f uvicorn` to avoid port collisions from any backgrounded terminal session.
2. **Starts the FastAPI mock upstream** (port 8000) and polls `/health` until ready.
3. **Starts the compiled `./auvis dev` binary** (port 8080) via subprocess with `AUVIS_UPSTREAM`, `AUVIS_PORT`, and `AUVIS_AUDIT_FILE` environment overrides; waits for a 401 on `/users` to confirm the proxy is live.
4. **Generates a JWT** for `usr_101 / role=analyst` via `auth.py::create_access_token`, which enriches the token with all of Amara Okonkwo's PII attributes from `data.py`.
5. **Sends `GET /users`** through the proxy with the JWT in `Authorization: Bearer` and `X-Auvis-Upstream` header.
6. **Asserts masking**: email → `***@gmail.com`, SSN → removed, phone → masked, DOB → masked, address.street → removed.
7. **Graceful shutdown** via `SIGTERM` to flush the audit logger to disk.
8. **Asserts the audit log**: reads `audit.jsonl`, parses the first JSONL line, checks `policy_name` contains `users-analyst-masked` and `fields_masked` contains `email`.
9. **Non-blocking stderr capture** — if the proxy returns a non-200, the test dumps the full Go slog output to aid debugging without blocking the test runner.

---

### Decision 22: FastAPI Route Registration — No Trailing-Slash Redirect

**Status: Fixed** (See [test-server/routes/users.py](file:///Users/Cridiv/Documents/Auvis/test-server/routes/users.py))

**Context:** During the first E2E run the proxy returned HTTP 500 with the error `data_path "data" not found in response (body was "")`. The upstream response body was completely empty.

**Root Cause:** FastAPI's `APIRouter(prefix="/users")` with `@router.get("/")` registers the canonical route at `/users/`. With `redirect_slashes=True` (FastAPI default), a `GET /users` request triggers a `307 Temporary Redirect` to `/users/`. Go's `httputil.ReverseProxy` does **not** follow redirects — it proxies the 307 response back to the `responseInterceptor`. A 307 response carries no body, so the transform received zero bytes and failed when trying to extract `data_path: "data"`.

**Fix:** Changed `@router.get("/")` → `@router.get("")` on the list-users endpoint. With `prefix="/users"` and route `""`, FastAPI registers the handler directly at `/users`, eliminating the redirect entirely.

---

### Decision 23: Proxy Passthrough for Non-200 Upstream Responses

**Status: Implemented** (See [proxy/handler.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/proxy/handler.go))

**Context:** After the trailing-slash redirect fix, the proxy was still attempting to apply field transformation rules to upstream error responses (4xx, 5xx). Trying to extract `data_path: "data"` from an error JSON like `{"detail": "..."}` (or an empty body from a 307) always fails.

**Decision:** Added an explicit passthrough branch in the `ModifyResponse` pipeline: if `info.statusCode != 200`, the response body is piped through to the client verbatim with no field rules applied. This is the correct semantic — transform rules are data-shaping rules for successful responses, not error handlers.

```go
} else if info.statusCode != http.StatusOK {
    // Non-200 upstream response — pass through raw
    responseBytes, _ := io.ReadAll(pipeReader)
    buf.Write(responseBytes)
    summary = transform.Summary{}
} else {
    summary, transformErr = transformFn(pipeReader, &buf, reqCtx)
    ...
}
```

---

### Decision 24: Wildcard Field Rule (`field: "*"`) Is a Policy Directive, Not a Field Name

**Status: Fixed** (See [policy/loader.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/policy/loader.go))

**Context:** After the redirect fix succeeded (body arrived), the proxy returned a new 500 error: `removing field "*": cannot delete value from a complex path`. The `users-deny-unmatched-roles` policy has `field: "*", action: deny`. This caused the transform to call `sjson.Delete(record, "*")`, which sjson rejects as an invalid path because `*` is a glob token, not a JSON key.

**Root Cause (two layers):**
1. **`compile()`** — when building a standalone `EffectivePolicy`, it stored `Rules["*"] = ActionDeny`. This is wrong: `field: "*"` is a whole-policy directive (deny everything / allow everything), not a literal JSON field to remove.
2. **`Lookup()` merge loop** — when merging matched policies, the `Rules["*"] = ActionDeny` entry from the wildcard-role policy was written into `merged.Rules`, where it survived even after the explicit-role policy (`users-analyst-masked`) cleared `merged.Deny`. The transform then received `merged.Rules["*"] = deny` and tried to act on it as a field name.

**Fix (both sites):**
- In `compile()`: `field: "*"` rules are intercepted with `continue` — only `ep.Deny` is toggled, `"*"` is never written to `ep.Rules`.
- In `Lookup()` merge loop: same guard — `field == "*"` rules toggle `merged.Deny` and then `continue`, never touching `merged.Rules`.

This preserves the intended semantics: a policy with `field: "*", action: deny` means "deny the whole response" (sets `ep.Deny = true`); a policy with `field: "*", action: allow` means "passthrough everything" (clears `ep.Deny`). The transform already handles both via the existing `ep.Deny` and wildcard-allow checks before entering the per-field loop.

---

### Decision 25: JSONL Audit Log Format (Compact, One Object Per Line)

**Status: Fixed** (See [audit/logger.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/audit/logger.go))

**Context:** The E2E test's audit log assertion failed with `Expecting property name enclosed in double quotes: line 1 column 2 (char 1)`. The test reads `audit.jsonl` line-by-line and calls `json.loads(line)`. The logger was writing pretty-printed JSON (using `json.MarshalIndent`) separated by blank lines — meaning the first "line" was always just `{`, which is not a valid JSON document.

**Decision:** Changed `json.MarshalIndent` to `json.Marshal` (compact) and the trailing `\n\n` to a single `\n`. This produces true JSONL format: one self-contained JSON object per line, with no indentation and no blank separators.

**Trade-off:** The file is less human-readable inline, but:
- Every standard log processing tool (jq, Splunk, ClickHouse, Python) natively handles JSONL.
- `jq` with `--slurpfile` or `jq .` works perfectly on compact JSONL.
- Human inspection remains easy with `jq . audit.jsonl` for pretty-printing on demand.

---

## E2E Test Run — First Full Pass (May 2026)

**Test:** `python3 test-server/e2e_test.py`
**Binary:** `auvis-infra/auvis` (compiled from latest source)
**Upstream:** FastAPI mock server (`test-server/main.py`) on port 8000
**Proxy:** `auvis dev` on port 8080, policies from `auvis-infra/policies/`
**JWT User:** `usr_101` / `role=analyst` (Amara Okonkwo — full PII profile from `data.py`)

**Result: PASSED ✓**

| Assertion | Expected | Actual | Result |
|---|---|---|---|
| HTTP status | 200 | 200 | ✓ |
| `email` | `***@gmail.com` | `***@gmail.com` | ✓ |
| `ssn` | removed (None) | None | ✓ |
| `phone` | masked (contains `*`) | `****7890` | ✓ |
| `date_of_birth` | masked (contains `*`) | `****3-15` | ✓ |
| `address.street` | removed (None) | None | ✓ |
| Audit log written | yes | yes | ✓ |
| `policy_name` in audit | `users-analyst-masked` | present | ✓ |
| `fields_masked` in audit | includes `email` | present | ✓ |

**Bugs surfaced and fixed during this test run:**
- Decision 22: FastAPI trailing-slash redirect (307 → empty body → 500)
- Decision 23: Proxy non-200 passthrough (transform rules on error bodies)
- Decision 24: Wildcard field `"*"` leaking into `merged.Rules` (sjson path error)
- Decision 25: Pretty-printed audit log breaking JSONL line-by-line parsing

---

### Decision 26: Full Field Disposition Audit Schema

**Status: Implemented** (See [audit/types.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/audit/types.go), [transform/transform.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/transform/transform.go), [proxy/handler.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/proxy/handler.go))

**Context:** The original audit entry only recorded `fields_masked`. A compliance auditor or CISO looking at the log had no way to know which fields were removed entirely, which were blocked by a deny rule, or which passed through untouched. The log told half the story — the least important half. Masked fields are a transformation; removed fields and denied responses are enforcement actions and arguably more significant.

**Decision: Four-Bucket Field Disposition Model**

Every field that arrives in an upstream response is accounted for in exactly one of four buckets. Nothing is unaccounted for:

| Bucket | JSON key | Meaning |
|---|---|---|
| Masked | `fields_masked` | Value was transformed in place (mask / anonymize / redact / truncate) |
| Removed | `fields_removed` | Field was dropped entirely (`action: remove`) |
| Denied | `fields_denied` | Field-level deny rule fired (`action: deny` on a specific field) |
| Allowed | `fields_allowed` | Field passed through untouched — no rule matched it |

`response_denied` is a separate top-level boolean because a full-response deny is qualitatively different from any field-level action — it's the entire response being blocked with a 403. Burying it in a `fields_denied` array would conflate two distinct enforcement levels.

**Example audit entry (analyst role on `/users`):**
```json
{
  "timestamp": "2026-05-31T01:20:48.592421Z",
  "request_id": "req-1780190448582984000",
  "user_id": "usr_101",
  "user_role": "analyst",
  "resource": "/users",
  "method": "GET",
  "policies_applied": ["users-deny-unmatched-roles", "users-analyst-masked"],
  "priority": 0,
  "compliance_tags": null,
  "fields_masked": ["email", "phone", "date_of_birth"],
  "fields_removed": ["ssn", "address.street"],
  "fields_allowed": ["id", "name", "role", "account_status", "created_at", "region", "address"],
  "fields_denied": [],
  "response_denied": false,
  "rows_filtered": 0,
  "upstream_status": 200,
  "duration_ms": 9
}
```

**Removed:** `policy_name` — this was a duplicate of `policies_applied` carrying identical data under a different key. `policies_applied` is the unambiguous name; `policy_name` is dropped.

**Implementation details:**

- `transform/transform.go` — New `recordDisposition` struct (`masked`, `removed`, `allowed`, `denied` string slices) replaces the bare `[]string` that `transformRecord` returned. The function now tracks which top-level keys were acted upon via an `actedOn` map, then iterates the original record at the end to populate `allowed` with everything not touched. Dotted paths (`address.street`) attribute to their top-level key (`address`) for the `actedOn` check. `transformArray` signature expanded to 7 return values to propagate all four buckets up.

- `ActionRemove` and `ActionDeny` are now separated at the field level: `remove` goes to `fields_removed`, field-level `deny` goes to `fields_denied`. Previously both were merged into the same bucket (and both were silently discarded from the audit entirely).

- Both deny paths in `apply()` (no policy match + explicit policy deny) now set `ResponseDenied: true` in the returned `Summary`, which maps directly to `response_denied` in the audit entry.

- All existing tests updated to match new signatures (`go test ./...` passes clean).

---

### Decision 27: E2E LLM Interceptor Outbound Masking & Inbound Response Rehydration

**Status: Fully Implemented & Verified** (See [test-server/e2e_llm_test.py](file:///Users/Cridiv/Documents/Auvis/test-server/e2e_llm_test.py) and [pii/llm_interceptor.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pii/llm_interceptor.go))

*   **Context**: LLM pipelines require sending prompt logs outbound to third-party endpoints. Safeguarding this data demands scrubbing it in outbound transit, but the application expects a seamless response where the masked entities are restored.
*   **Decision**: Implemented end-to-end request intercepting and response rehydration. An outbound POST payload containing raw PII gets matched against `llm_intercept` policies, replaced with structured placeholders (like `[EMAIL_1]`), and forwarded to the upstream. Upon response, the gateway transparently maps the placeholders back to original values in the downstream payload returned to the client (if `rehydrate: true` is configured).

---

### Decision 28: Merged Policy Priority Propagation

**Status: Fully Implemented & Verified** (See [policy/loader.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/policy/loader.go) and [proxy/handler.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/proxy/handler.go))

*   **Context**: When multiple overlapping policies matched a single request, the resolved priority field returned by the merged `EffectivePolicy` was logged as `0` because priority was not copied during compile/merge.
*   **Decision**: Dynamic priority propagation. The compilation step now retains individual policy priorities, and `Store.Lookup` assigns `merged.Priority` to the highest matching policy (the last element of the sorted matches). In addition, properties like priority, compliance tags, and policy name are dynamically mapped from the matched policy onto the request summary during LLM interception.

---

### Decision 29: Dynamic Scanner Mode Header Override (`X-Auvis-Scanner-Mode`)

**Status: Fully Implemented & Verified** (See [proxy/handler.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/proxy/handler.go) and [pii/scanner.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pii/scanner.go))

*   **Context**: Systems run dynamic workloads. For latency-sensitive paths or performance testing, operators need a request-level switch to override default policy scanner modes (e.g. bypassing LLM fallback steps entirely).
*   **Decision**: Added support for the `X-Auvis-Scanner-Mode` HTTP request header (e.g., `X-Auvis-Scanner-Mode: regex_only`). When passed, the proxy overrides the default scanner mode dynamically, allowing instant under-the-hood performance optimization.

---

### Decision 30: Concurrent Connection Pooling & LLM Throttling Semaphore

**Status: Fully Implemented & Verified** (See [proxy/handler.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/proxy/handler.go) and [pii/llm.go](file:///Users/Cridiv/Documents/Auvis/auvis-infra/pii/llm.go))

*   **Context**: Load benchmarks with high concurrency (50+ parallel workers) suffered from socket churn and queue compounding because Go's default transport restricts idle connections per host to 2, and concurrent calls could swamp the LLM backend.
*   **Decision**:
    *   **Shared Connection Pool**: Initialized a shared high-concurrency `http.Transport` inside the Handler constructor with `MaxIdleConns` and `MaxIdleConnsPerHost` set to `1000`, enabling efficient socket reuse and slashing tail latency.
    *   **LLM Semaphore Throttling**: Added a global semaphore channel with a capacity of `10` in `pii/llm.go`. Concurrent calls block on a non-blocking `select` block while respecting context cancellations, preventing cascading queues under sudden spikes.

---

## High-Performance Benchmarking Baseline (May 2026)

To measure the true capacity of the Auvis proxy engine without Python's GIL bottle-necking the load test, we implemented a native Go-based upstream mock server ([test-cmd/main.go](file:///Users/Cridiv/Documents/Auvis/test-cmd/main.go)) serving the exact same rich PII payload.

Under benchmarking (`hey -n 10000 -c 50` in `regex_only` mode), switching from the Python FastAPI server to the Go mock upstream server revealed the true performance footprint of the gateway:

| Metric | Python upstream | Go upstream | Performance Improvement |
|---|---|---|---|
| **Throughput (req/sec)** | 815 | 1,710 | **+110% (More than doubled)** |
| **Average Latency** | 61ms | 28ms | **-54% (More than halved)** |
| **p50 Latency** | 49ms | 23ms | **-53%** |
| **p95 Latency** | 123ms | 60ms | **-51%** |
| **p99 Latency** | 346ms | 219ms | **-37%** |
| **Fastest Request** | 2.6ms | 0.9ms | **-65% (Sub-millisecond processing)** |

This confirms that Auvis's internal processing overhead is exceptionally light (sub-millisecond fastest processing times), and the proxy easily handles high-throughput enterprise workloads.

---

## Things Still To Decide

- [ ] **Tokenization vault** — HashiCorp Vault only, built-in SQLite vault for dev, or both?
- [ ] **Audit storage** — ClickHouse in production, JSONL as fallback when ClickHouse unavailable?
- [ ] **Embedding responsibility** — does Auvis call the embedding API, or does it pass governed data back to the caller who embeds?
- [ ] **LangChain vs raw SDK first** — which do design partners actually use?
- [ ] **First source connector after the proxy** — Confluence or PostgreSQL CDC?

---

## Competitive Landscape

| Competitor | What they do | Why they miss the pipeline layer |
|---|---|---|
| Immuta / Privacera | Access control for Snowflake, Databricks | Designed for SQL analytics, not streaming or vector DB ingestion |
| LLM Gateways (Portkey, Helicone, Kong) | Filter prompts at inference time | Data already in vector DB — too late |
| DLP (Proofpoint, Forcepoint) | Email, endpoint, web traffic | Blind to internal microservice data flows |
| Collibra / Alation | Data catalog and metadata | Documentation only, no runtime enforcement |
| Databricks Unity / AWS Bedrock | PII filtering in their managed pipelines | Locked to their ecosystem |

**Positioning:** Immuta and Auvis are complementary, not competitive. Immuta governs your Snowflake queries. Auvis governs what enters your AI systems. Most enterprise customers will use both.

---

## Target Customer (MVP)

- Regulated industry: financial services or healthcare first
- 500+ employees (compliance obligations, dedicated security team)
- Actively building or deploying internal AI tools / RAG systems
- Pain signal: "our AI project is stuck in security review" or "we don't know what data is in our vector DB"
- Data stack: Confluence/SharePoint ingestion or Kafka, feeding a vector DB
- Already has Immuta or similar for analytics — nothing for AI pipelines

---

## Open Questions Log

| Date | Question | Status |
|---|---|---|
| May 2026 | Should Kafka be in Phase 2? | **Decided** — deprioritized, add when customer asks |
| May 2026 | Fail behavior when condition evaluator errors | **Decided** — fail closed except deny rules |
| May 2026 | How to resolve user context attributes | **Decided** — JWT first, headers as fallback, IdP optional |
| May 2026 | Missing attribute vs empty string in conditions | **Decided** — missing = indeterminate, not empty string |
| May 2026 | Priority semantics | **Decided** — determinism under conflict, not importance ranking |
| May 2026 | Does the E2E test cover real masking end-to-end? | **Decided** — yes, first full pass confirmed all assertions pass |
| May 2026 | What should the audit log record beyond masked fields? | **Decided** — four-bucket disposition model: masked / removed / denied / allowed + response_denied flag |
| May 2026 | Does LLM Interception work end-to-end? | **Decided** — yes, verified outbound prompt masking and inbound response rehydration |
| May 2026 | Does Policy Hot-Reload operate reliably in under 1 second? | **Decided** — yes, verified dynamic real-time YAML reload & unload behaviors without gateway restart |
| May 2026 | Does CLI Validation cover structural and syntactic errors? | **Decided** — yes, E2E validate test verified exit status security (exit 1) and printed diagnostics across 10 distinct error scenarios |
| — | Tokenization vault implementation | **Open** |
| — | Embedding responsibility | **Open** |
| — | First pipeline source connector | **Open** |

---

*Last updated: May 2026 (Added Decisions 27-30 detailing Outbound LLM interception and rehydration, priority propagation, scanner headers, connection pooling, and log rotation, alongside full E2E Hot-Reload and CLI Validation test suites) · Keep this doc updated as decisions are made — especially the "things still to decide" and "open questions" sections.*
