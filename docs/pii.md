# Module Documentation: `pii`

The `pii` module implements bidirectional PII (Personally Identifiable Information) detection and masking for LLM request/response interception. It combines fast regex-based scanning with an optional Gemini LLM fallback, and manages placeholder token maps for subsequent rehydration.

---

## 📂 File Index

- [`types.go`](#1-typesgo) — `Entity`, `PlaceholderMap`
- [`scanner.go`](#2-scannergo) — `Scanner` — hybrid scan orchestration
- [`llm.go`](#3-llmgo) — `ScanLLM` — Gemini API PII extraction
- [`llm_interceptor.go`](#4-llm_interceptorgo) — `LLMInterceptor` — request/response interception coordinator
- [`regex.go`](#5-regexgo) — `ScanRegex` — compiled pattern-based PII detection

---

## 1. `types.go`

### `type Entity struct`

Represents one detected PII occurrence.

| Field | Type | Description |
|---|---|---|
| `Value` | `string` | Raw PII value as it appears in the text |
| `Type` | `string` | PII category: `EMAIL`, `PHONE`, `SSN`, `CARD`, `NAME`, `IP`, `DATE_OF_BIRTH`, `IBAN`, `ROUTING_NUMBER`, `PASSPORT`, `NATIONAL_ID` |
| `Start` | `int` | Start byte offset in original text |
| `End` | `int` | End byte offset in original text |
| `Confidence` | `float64` | Detection confidence score (0.0–1.0) |

---

### `type PlaceholderMap struct`

Bidirectional, thread-safe mapping between real PII values and their placeholder tokens. Created per-request, never shared between requests.

| Field | Type | Description |
|---|---|---|
| `mu` | `sync.RWMutex` | Guards concurrent reads and writes |
| `forward` | `map[string]string` | `real_value → "[TYPE_N]"` |
| `reverse` | `map[string]string` | `"[TYPE_N]" → real_value` |
| `counters` | `map[string]int` | Per-type sequence counter: `EMAIL → 3` |

---

#### `NewPlaceholderMap() *PlaceholderMap`

Returns an initialised empty `PlaceholderMap`.

---

#### `(m *PlaceholderMap) Add(value, piiType string) string`

Registers a real PII value and returns its deterministic placeholder. **Idempotent** — the same value always gets the same placeholder within a request.

**Logic:**
1. RLock check: if `value` already in `forward` map → returns existing placeholder.
2. Upgrade to write lock: checks again (double-checked locking for correctness).
3. Increments `counters[piiType]`.
4. Creates placeholder: `fmt.Sprintf("[%s_%d]", piiType, counter)` — e.g. `"[EMAIL_1]"`, `"[EMAIL_2]"`, `"[SSN_1]"`.
5. Sets `forward[value] = placeholder` and `reverse[placeholder] = value`.

---

#### `(m *PlaceholderMap) Mask(text string) string`

Replaces all real PII values in `text` with their placeholder tokens. Iterates `forward` map and calls `strings.ReplaceAll` for each entry. Read-locked.

---

#### `(m *PlaceholderMap) Rehydrate(text string) string`

Replaces all placeholder tokens in `text` with their original real values. Iterates `reverse` map. Read-locked.

---

#### `(m *PlaceholderMap) DetectedTypes() []string`

Returns the unique PII type names detected in the map (e.g. `["EMAIL", "SSN"]`). Extracts types from placeholder strings by parsing `"[EMAIL_1]"` → `"EMAIL"`. Read-locked.

---

## 2. `scanner.go`

### `type Scanner struct`

Orchestrates the two-pass PII detection pipeline.

| Field | Type | Description |
|---|---|---|
| `APIKey` | `string` | Gemini API key. Empty string disables LLM fallback. |
| `LLMFallback` | `bool` | If `true` and `APIKey != ""`, enables LLM second pass. |
| `Mode` | `string` | Scan mode: `"regex_only"`, `"llm_only"`, or `"hybrid"` (default). |

---

#### `(s *Scanner) Scan(ctx context.Context, text string) (*PlaceholderMap, error)`

Two-pass scan orchestration.

**Pass 1 — Regex** (if `mode == "regex_only"` or `"hybrid"`):
- Calls `ScanRegex(text)` → `[]Entity`.
- Calls `pm.Add(e.Value, e.Type)` for each detected entity.

**Pass 2 — LLM Fallback** (if `mode == "llm_only"` or `"hybrid"`, AND `s.LLMFallback == true`, AND `s.APIKey != ""`):
- Calls `pm.Mask(text)` first — sends already-masked text to the LLM so known PII is not transmitted to an external API unnecessarily.
- Creates a derived context with **200ms hard timeout** (`context.WithTimeout`).
- Calls `ScanLLM(llmCtx, maskedText, s.APIKey)`.
- On LLM error or timeout: logs `"[PII] llm fallback error/timeout (continuing with regex only): ..."` and **continues** with regex-only results. The request is never blocked by LLM failures.
- On success: calls `pm.Add(e.Value, e.Type)` for each new entity.

Returns `(*PlaceholderMap, nil)`.

---

## 3. `llm.go`

### `ScanLLM(ctx context.Context, text string, apiKey string) ([]Entity, error)`

Calls the Gemini API to detect PII in `text` not caught by regex patterns (e.g. freeform names, dates, national IDs in unusual formats).

**Behavior:**
- Sends a structured prompt to Gemini requesting JSON-formatted PII entity list.
- Parses the response into `[]Entity`.
- Respects the calling context timeout (200ms from `Scanner.Scan`).
- Returns `nil, err` on API failure — caller handles gracefully.

---

## 4. `llm_interceptor.go`

### `type LLMInterceptor struct`

Coordinates end-to-end LLM request/response PII interception.

| Field | Type | Description |
|---|---|---|
| `Scanner` | `*Scanner` | PII scanner (hybrid mode by default) |
| `Rehydrate` | `bool` | If `true`, re-substitutes tokens in LLM response with original values |

---

#### `(li *LLMInterceptor) InterceptRequest(ctx context.Context, body []byte, reqCtx types.RequestContext) ([]byte, *PlaceholderMap, transform.Summary, error)`

Scans and masks PII in an outbound LLM API request body before it is forwarded to the upstream LLM service.

**Steps:**
1. JSON-decodes `body` into `map[string]any`.
2. Calls `extractLLMText(req)` to concatenate all `messages[*].content` text strings.
3. Calls `li.Scanner.Scan(ctx, allText)` → `*PlaceholderMap`.
4. Calls `pm.Mask(string(body))` to replace all PII occurrences in the full raw body string.
5. Builds `transform.Summary{PoliciesApplied: ["llm-pii-intercept"], FieldsMasked: [...types...], PiiTypesDetected: [...]}`.
6. Returns `(maskedBody, pm, summary, nil)`.

---

#### `(li *LLMInterceptor) InterceptResponse(body []byte, pm *PlaceholderMap) ([]byte, error)`

Restores original PII values in the LLM response body.

- If `!li.Rehydrate` or `pm == nil` → returns `body` unchanged.
- Calls `pm.Rehydrate(string(body))` → replaces all `[TYPE_N]` tokens with original values.
- Returns rehydrated bytes.

---

#### `extractLLMText(req map[string]any) string` *(private)*

Extracts all text content from an OpenAI-compatible chat message payload structure:

```json
{
  "messages": [
    {"role": "user", "content": "Hello, my name is John Doe"},
    {"role": "user", "content": [{"type": "text", "text": "My SSN is 123-45-6789"}]}
  ]
}
```

Handles both string `content` fields and array `content` fields with `{type, text}` block structure. Concatenates all text with spaces into a single string for scanning.

---

#### `collectPlaceholderTypes(pm *PlaceholderMap) []string` *(private)*

Reads `pm.forward` values and extracts unique type names from placeholder strings (e.g. `"[EMAIL_1]"` → `"EMAIL"`). Returns deduplicated list. Used to populate `Summary.FieldsMasked`.

---

## 5. `regex.go`

### `ScanRegex(text string) []Entity`

High-speed, zero-network PII detection using pre-compiled regular expressions.

**Detected patterns and their `Entity.Type` values:**

| Type | Pattern Description |
|---|---|
| `EMAIL` | Standard email format: `user@domain.tld` |
| `PHONE` | US and international phone numbers, various formats |
| `SSN` | US Social Security Number: `NNN-NN-NNNN` |
| `CARD` | Credit/debit card numbers: 13–16 digit groups |
| `IP` | IPv4 addresses |
| `DATE_OF_BIRTH` | Common date formats: `MM/DD/YYYY`, `YYYY-MM-DD`, etc. |
| `IBAN` | International Bank Account Numbers |
| `ROUTING_NUMBER` | US ABA routing numbers (9-digit) |
| `PASSPORT` | Passport number formats |
| `NATIONAL_ID` | Government-issued national ID patterns |

Each detected match is returned as an `Entity` with `Value`, `Type`, `Start`, `End`, and `Confidence` fields populated. All regex patterns are pre-compiled at package init time for zero per-call allocation.

---

## PII Interception Flow (end-to-end)

```
Client → Auvis Proxy
    ↓
proxy.Handler: detects llm_intercept policy
    ↓
pii.LLMInterceptor.InterceptRequest(bodyBytes)
    ├─ pii.Scanner.Scan(allMessageText)
    │   ├─ pii.ScanRegex(text)         → fast regex pass
    │   └─ pii.ScanLLM(maskedText)     → LLM fallback (200ms timeout)
    └─ pm.Mask(rawBody)                → [EMAIL_1], [SSN_1] substitution
    ↓
Modified request body forwarded to upstream LLM
    ↓
Upstream LLM responds (may contain [EMAIL_1] in its answer)
    ↓
pii.LLMInterceptor.InterceptResponse(responseBytes, pm)
    └─ pm.Rehydrate(response)          → restore original values (if rehydrate: true)
    ↓
Client receives response with original values restored
```
