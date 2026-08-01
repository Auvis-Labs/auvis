package transform

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/cridiv/Auvis/policy"
	"github.com/cridiv/Auvis/types"
)

// Summary carries what the transform did — fed into the audit log.
// Every field that came back from upstream falls into exactly one bucket.
type Summary struct {
	PoliciesApplied    []string
	FieldsMasked       []string  // value was transformed (mask/anonymize/redact/truncate)
	FieldsRemoved      []string  // field was dropped entirely (action: remove)
	FieldsAllowed      []string  // field passed through untouched
	FieldsDenied       []string  // field-level deny rule fired
	ResponseDenied     bool      // whole response was blocked (403)
	PiiTypesDetected   []string
	RowsFiltered       int
	Priority           int
	ComplianceTags     []string
	ConditionEvaluated []string
	Denied             bool   // internal — drives HTTP 403 in handler
	DenyReason         string
	// RecordOutcomes carries per-record field dispositions for multi-record responses.
	// Populated only when the response is a JSON array. Nil for single-record responses.
	RecordOutcomes []RecordOutcome
}

// RecordOutcome captures what happened to a single record within a multi-record response.
// Enables compliance-grade auditing: "what exactly happened to record PT-047?"
type RecordOutcome struct {
	RecordID      string   `json:"record_id"`
	FieldsMasked  []string `json:"fields_masked,omitempty"`
	FieldsRemoved []string `json:"fields_removed,omitempty"`
	FieldsAllowed []string `json:"fields_allowed,omitempty"`
	FieldsDenied  []string `json:"fields_denied,omitempty"`
	Filtered      bool     `json:"filtered,omitempty"` // true if row was excluded by filter condition
}

// TransformFunc is the signature the proxy handler expects.
type TransformFunc func([]byte, types.RequestContext) ([]byte, Summary, error)
type StreamTransformFunc func(src io.Reader, dst io.Writer, reqCtx types.RequestContext) (Summary, error)

// NewApplyFn returns a TransformFunc closure that captures the policy store.
func NewApplyFn(store *policy.Store) TransformFunc {
	return func(body []byte, reqCtx types.RequestContext) ([]byte, Summary, error) {
		return apply(body, reqCtx, store)
	}
}

func NewStreamApplyFn(store *policy.Store) StreamTransformFunc {
	return func(src io.Reader, dst io.Writer, reqCtx types.RequestContext) (Summary, error) {
		body, err := io.ReadAll(src)
		if err != nil {
			return Summary{}, fmt.Errorf("reading stream: %w", err)
		}

		result, summary, err := apply(body, reqCtx, store)
		if err != nil {
			return Summary{}, err
		}

		if _, err := dst.Write(result); err != nil {
			return Summary{}, fmt.Errorf("writing stream: %w", err)
		}

		return summary, nil
	}
}

// Apply evaluates the policy store and applies rules to the JSON body.
func Apply(body []byte, reqCtx types.RequestContext, store *policy.Store) ([]byte, Summary, error) {
	return apply(body, reqCtx, store)
}

// apply is the entry point for every response body.
// It looks up the policy, then delegates to the appropriate path.
func apply(body []byte, reqCtx types.RequestContext, store *policy.Store) ([]byte, Summary, error) {
	slog.Debug("Evaluating request policies", "request_id", reqCtx.RequestID, "role", reqCtx.UserRole, "resource", reqCtx.Resource)

	ep := store.Lookup(reqCtx.UserRole, reqCtx.Resource, reqCtx.Service, reqCtx.Source, reqCtx.Destination, "read")

	// No policy matched — deny by default
	if ep == nil {
		slog.Warn("No matching policy found — access denied by default", "request_id", reqCtx.RequestID, "role", reqCtx.UserRole, "resource", reqCtx.Resource)
		reason := "no policy found for your role on this resource"
		return deny(reason), Summary{Denied: true, ResponseDenied: true, DenyReason: reason}, nil
	}

	// Explicit deny
	if ep.Deny {
		slog.Warn("Explicit policy deny triggered", "request_id", reqCtx.RequestID, "policies", ep.SourcePolicies)
		reason := "access denied by policy"
		return deny(reason), Summary{
			Denied:          true,
			ResponseDenied:  true,
			DenyReason:      reason,
			PoliciesApplied: ep.SourcePolicies,
		}, nil
	}

	// Wildcard allow — passthrough, nothing to transform
	if action, ok := ep.Rules["*"]; ok && action == policy.ActionAllow {
		slog.Debug("Wildcard policy allow — bypassing transform", "request_id", reqCtx.RequestID, "policies", ep.SourcePolicies)
		return body, Summary{PoliciesApplied: ep.SourcePolicies}, nil
	}

	slog.Debug("Applying policy transformation rules", "request_id", reqCtx.RequestID, "policies", ep.SourcePolicies)

	// --- Step 1: Extract the target node ---
	//
	// If data_path is set (e.g. "data" or "result.users"), we drill into
	// the response to find the actual records. If it's empty, the records
	// are at the root and we work on the whole body directly.
	//
	// Example — policy has data_path: "data":
	//   input:  {"data": [...], "meta": {"total": 10}}
	//   target: [...]          ← we only transform this part
	//   rest:   {"meta": ...}  ← untouched, re-embedded in step 3
	target, err := extractTarget(body, ep.Response.DataPath)
	if err != nil {
		return nil, Summary{}, fmt.Errorf("extracting data_path %q: %w", ep.Response.DataPath, err)
	}

	// --- Step 2: Transform the target node ---
	//
	// The target is either an array of records or a single object.
	// We auto-detect which unless the policy explicitly specifies via Array.
	// Either way, the same field rules are applied to each record.
	var transformed []byte
	var allMasked, allRemoved, allAllowed, allDenied []string
	var rowsFiltered int
	var conditionEvaluated []string

	isArray := target.IsArray()
	if ep.Response.Array != nil {
		// Policy explicitly tells us — trust it over auto-detection
		isArray = *ep.Response.Array
	}

	var recordOutcomes []RecordOutcome
	if isArray {
		transformed, allMasked, allRemoved, allAllowed, allDenied, rowsFiltered, recordOutcomes, err = transformArray(target, reqCtx, ep, &conditionEvaluated)
	} else {
		var rec recordDisposition
		transformed, rec, err = transformRecord([]byte(target.Raw), reqCtx, ep, &conditionEvaluated)
		allMasked = rec.masked
		allRemoved = rec.removed
		allAllowed = rec.allowed
		allDenied = rec.denied
		// Single-record path: RecordOutcomes intentionally left nil (omitted from audit)
	}
	if err != nil {
		return nil, Summary{}, err
	}

	// --- Step 3: Re-embed the transformed data ---
	//
	// If data_path was set, we need to put the transformed records back
	// into the original response envelope so the caller gets the full
	// response structure they expect — just with the records modified.
	//
	// Example:
	//   original: {"data": [...old...], "meta": {"total": 10}}
	//   after:    {"data": [...masked...], "meta": {"total": 10}}
	//
	// If data_path was empty, reEmbed is a no-op and just returns transformed.
	result, err := reEmbed(body, ep.Response.DataPath, transformed)
	if err != nil {
		return nil, Summary{}, fmt.Errorf("re-embedding into %q: %w", ep.Response.DataPath, err)
	}

	summary := Summary{
		PoliciesApplied:    ep.SourcePolicies,
		FieldsMasked:       dedupe(allMasked),
		FieldsRemoved:      dedupe(allRemoved),
		FieldsAllowed:      dedupe(allAllowed),
		FieldsDenied:       dedupe(allDenied),
		RowsFiltered:       rowsFiltered,
		Priority:           ep.Priority,
		ComplianceTags:     ep.ComplianceTags,
		ConditionEvaluated: conditionEvaluated,
		RecordOutcomes:     recordOutcomes,
	}

	return result, summary, nil
}

// extractTarget drills into the response body using gjson dot-notation path.
// If path is empty, returns the root of the body as a gjson.Result.
//
// Examples:
//   extractTarget(body, "")            → root value (array or object)
//   extractTarget(body, "data")        → body["data"]
//   extractTarget(body, "result.users")→ body["result"]["users"]
func extractTarget(body []byte, path string) (gjson.Result, error) {
	if path == "" {
		result := gjson.ParseBytes(body)
		return result, nil
	}

	result := gjson.GetBytes(body, path)
	if !result.Exists() {
		return gjson.Result{}, fmt.Errorf("data_path %q not found in response", path)
	}

	return result, nil
}

// reEmbed puts the transformed data back into the original envelope.
// If path is empty, the transformed data IS the full response — return as-is.
// If path is set, use sjson to write the transformed bytes back at that path.
func reEmbed(originalBody []byte, path string, transformed []byte) ([]byte, error) {
	if path == "" {
		// No envelope — transformed is the complete response
		return transformed, nil
	}

	// Parse transformed back to a raw interface{} so sjson can set it
	// We use gjson to parse then sjson to write back — keeps it as raw JSON
	raw := gjson.ParseBytes(transformed).Value()
	result, err := sjson.SetBytes(originalBody, path, raw)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// recordDisposition groups every field's fate into the four audit buckets.
type recordDisposition struct {
	masked  []string
	removed []string
	allowed []string
	denied  []string
}

// transformArray applies field rules to every element in a JSON array.
// Returns the transformed bytes, aggregate disposition slices, per-record outcomes, row filter count, and any error.
func transformArray(arr gjson.Result, reqCtx types.RequestContext, ep *policy.EffectivePolicy, conds *[]string) ([]byte, []string, []string, []string, []string, int, []RecordOutcome, error) {
	sb := sbPool.Get().(*strings.Builder)
	sb.Reset()
	defer sbPool.Put(sb)
	sb.WriteString("[")

	var allMasked, allRemoved, allAllowed, allDenied []string
	var outcomes []RecordOutcome
	rowsFiltered := 0
	pos := 0   // position counter for all records (including filtered)
	i := 0     // written-record counter
	var lastErr error

	arr.ForEach(func(_, value gjson.Result) bool {
		recordID := extractRecordID([]byte(value.Raw), pos)

		// Evaluate filter condition if it exists
		if ep.FilterCondition != "" {
			res, missingAttr := policy.EvaluateCondition(ep.FilterCondition, []byte(value.Raw), reqCtx)
			var status string
			if res == policy.EvalTrue {
				status = "true"
			} else if res == policy.EvalFalse {
				status = "false"
			} else {
				status = fmt.Sprintf("indeterminate (missing: %s)", missingAttr)
			}
			*conds = append(*conds, fmt.Sprintf("filter: %s (%s)", ep.FilterCondition, status))

			if res == policy.EvalFalse {
				rowsFiltered++
				outcomes = append(outcomes, RecordOutcome{RecordID: recordID, Filtered: true})
				pos++
				return true // skip row, continue iteration
			} else if res == policy.EvalIndeterminate {
				// Indeterminate state: fail-closed (exclude row) + log warning
				slog.Warn("Filter condition attribute missing — fail-closed row exclusion applied",
					"request_id", reqCtx.RequestID,
					"missing_attribute", missingAttr,
					"condition", ep.FilterCondition,
				)
				rowsFiltered++
				outcomes = append(outcomes, RecordOutcome{RecordID: recordID, Filtered: true})
				pos++
				return true // skip row, continue iteration
			}
		}

		out, rec, err := transformRecord([]byte(value.Raw), reqCtx, ep, conds)
		if err != nil {
			lastErr = err
			return false
		}
		if i > 0 {
			sb.WriteString(",")
		}
		sb.Write(out)
		allMasked = append(allMasked, rec.masked...)
		allRemoved = append(allRemoved, rec.removed...)
		allAllowed = append(allAllowed, rec.allowed...)
		allDenied = append(allDenied, rec.denied...)
		outcomes = append(outcomes, RecordOutcome{
			RecordID:      recordID,
			FieldsMasked:  rec.masked,
			FieldsRemoved: rec.removed,
			FieldsAllowed: rec.allowed,
			FieldsDenied:  rec.denied,
		})
		i++
		pos++
		return true
	})

	if lastErr != nil {
		return nil, nil, nil, nil, nil, 0, nil, lastErr
	}

	sb.WriteString("]")
	return []byte(sb.String()), allMasked, allRemoved, allAllowed, allDenied, rowsFiltered, outcomes, nil
}

// extractRecordID tries common identifier field names in priority order.
// Falls back to a positional label (e.g. "record[2]") if none are found.
func extractRecordID(record []byte, pos int) string {
	for _, key := range []string{"id", "patient_id", "record_id", "_id", "uuid", "ID"} {
		if v := gjson.GetBytes(record, key); v.Exists() && v.String() != "" {
			return v.String()
		}
	}
	return fmt.Sprintf("record[%d]", pos)
}

// resolveRules combines explicit field rules and dynamic tag-based rules.
func resolveRules(ep *policy.EffectivePolicy, fieldTags map[string][]string) (map[string]policy.RuleAction, map[string]string, map[string]int, map[string]string) {
	resolvedRules := make(map[string]policy.RuleAction)
	resolvedConditions := make(map[string]string)
	resolvedLengths := make(map[string]int)
	resolvedSeeds := make(map[string]string)

	// 1. Copy explicit rules (highest precedence)
	for k, v := range ep.Rules {
		resolvedRules[k] = v
		if cond, ok := ep.RuleConditions[k]; ok {
			resolvedConditions[k] = cond
		}
		if l, ok := ep.RuleLengths[k]; ok {
			resolvedLengths[k] = l
		}
		if s, ok := ep.RuleSeeds[k]; ok {
			resolvedSeeds[k] = s
		}
	}

	// Conflict resolution priority for tags
	actionPriority := map[policy.RuleAction]int{
		policy.ActionRemove:       8,
		policy.ActionDeny:         7,
		policy.ActionRedact:       6,
		policy.ActionMask:         5,
		policy.ActionTruncate:     4,
		policy.ActionPseudonymize: 3,
		policy.ActionAnonymize:    2,
		policy.ActionAllow:        1,
	}

	// 2. Resolve dynamic tag rules
	for field, tags := range fieldTags {
		if _, exists := resolvedRules[field]; exists {
			continue // Explicit path rule takes precedence
		}

		var bestAction policy.RuleAction
		bestPriority := 0
		var bestTag string

		for _, tag := range tags {
			if action, ok := ep.TaggedRules[tag]; ok {
				priority := actionPriority[action]
				if priority > bestPriority {
					bestPriority = priority
					bestAction = action
					bestTag = tag
				}
			}
		}

		if bestPriority > 0 {
			resolvedRules[field] = bestAction
			if cond, ok := ep.TaggedRuleConditions[bestTag]; ok {
				resolvedConditions[field] = cond
			}
			if l, ok := ep.TaggedRuleLengths[bestTag]; ok {
				resolvedLengths[field] = l
			}
			if s, ok := ep.TaggedRuleSeeds[bestTag]; ok {
				resolvedSeeds[field] = s
			}
		}
	}

	return resolvedRules, resolvedConditions, resolvedLengths, resolvedSeeds
}

// transformRecord applies field rules to a single JSON object.
// Returns the modified record and a disposition struct grouping every field into its audit bucket.
func transformRecord(record []byte, reqCtx types.RequestContext, ep *policy.EffectivePolicy, conds *[]string) ([]byte, recordDisposition, error) {
	out := record
	var err error
	var disp recordDisposition

	// Track which top-level fields were explicitly acted upon so we can compute "allowed" at the end.
	actedOn := make(map[string]bool)

	resolvedRules, resolvedConditions, resolvedLengths, resolvedSeeds := resolveRules(ep, reqCtx.FieldTags)

	for field, action := range resolvedRules {
		// If policy specifies Fields constraint, only apply rules to those fields
		if len(ep.Fields) > 0 {
			fieldAllowed := false
			for _, allowedField := range ep.Fields {
				if allowedField == field || allowedField == "*" || strings.HasPrefix(field, allowedField+".") {
					fieldAllowed = true
					break
				}
			}
			if !fieldAllowed {
				continue
			}
		}

		// Skip fields not present in the record — no action taken, no audit entry.
		if !gjson.GetBytes(out, field).Exists() {
			continue
		}

		// Evaluate condition if it exists
		if cond, ok := resolvedConditions[field]; ok && cond != "" {
			res, missingAttr := policy.EvaluateCondition(cond, record, reqCtx)
			var status string
			if res == policy.EvalTrue {
				status = "true"
			} else if res == policy.EvalFalse {
				status = "false"
			} else {
				status = fmt.Sprintf("indeterminate (missing: %s)", missingAttr)
			}
			*conds = append(*conds, fmt.Sprintf("field.%s: %s (%s)", field, cond, status))

			if res == policy.EvalFalse {
				// Condition is false — field is not acted on, don't mark as actedOn
				continue
			} else if res == policy.EvalIndeterminate {
				// Indeterminate state: fail-closed (apply rule) + log warning
				slog.Warn("Rule condition attribute missing — fail-closed rule enforcement applied",
					"request_id", reqCtx.RequestID,
					"missing_attribute", missingAttr,
					"condition", cond,
				)
			}
		}

		// Determine top-level key for allowed tracking (e.g. "address.street" → "address")
		topKey := field
		if idx := strings.Index(field, "."); idx != -1 {
			topKey = field[:idx]
		}
		actedOn[topKey] = true

		switch action {
		case policy.ActionMask:
			out, err = maskField(out, field)
			if err != nil {
				return nil, recordDisposition{}, fmt.Errorf("masking field %q: %w", field, err)
			}
			disp.masked = append(disp.masked, field)

		case policy.ActionAnonymize:
			out, err = anonymizeField(out, field)
			if err != nil {
				return nil, recordDisposition{}, fmt.Errorf("anonymizing field %q: %w", field, err)
			}
			disp.masked = append(disp.masked, field)

		case policy.ActionPseudonymize:
			seed := resolvedSeeds[field]
			out, err = pseudonymizeField(out, field, seed)
			if err != nil {
				return nil, recordDisposition{}, fmt.Errorf("pseudonymizing field %q: %w", field, err)
			}
			disp.masked = append(disp.masked, field)

		case policy.ActionRedact:
			out, err = redactField(out, field)
			if err != nil {
				return nil, recordDisposition{}, fmt.Errorf("redacting field %q: %w", field, err)
			}
			disp.masked = append(disp.masked, field)

		case policy.ActionTruncate:
			length := resolvedLengths[field]
			out, err = truncateField(out, field, length)
			if err != nil {
				return nil, recordDisposition{}, fmt.Errorf("truncating field %q: %w", field, err)
			}
			disp.masked = append(disp.masked, field)

		case policy.ActionRemove:
			out, err = sjson.DeleteBytes(out, field)
			if err != nil {
				return nil, recordDisposition{}, fmt.Errorf("removing field %q: %w", field, err)
			}
			disp.removed = append(disp.removed, field)

		case policy.ActionDeny:
			// Field-level deny: drop the field and record it separately from remove.
			out, err = sjson.DeleteBytes(out, field)
			if err != nil {
				return nil, recordDisposition{}, fmt.Errorf("denying field %q: %w", field, err)
			}
			disp.denied = append(disp.denied, field)

		case policy.ActionAllow:
			// explicit allow on a specific field — do nothing, field passes through
		}
	}

	// Compute allowed: top-level keys in the original record that were not acted upon.
	gjson.ParseBytes(record).ForEach(func(key, _ gjson.Result) bool {
		if !actedOn[key.String()] {
			disp.allowed = append(disp.allowed, key.String())
		}
		return true
	})

	slog.Debug("Record transformation details",
		"request_id", reqCtx.RequestID,
		"record_id", gjson.GetBytes(out, "id").Value(),
		"masked", disp.masked,
		"removed", disp.removed,
		"denied", disp.denied,
		"allowed", disp.allowed,
	)

	return out, disp, nil
}

var (
	ssnPattern      = regexp.MustCompile(`^\d{3}-\d{2}-\d{4}$`)
	phonePattern    = regexp.MustCompile(`^(?:\+?1[-.\s]?)?(?:\(?\d{3}\)?[-.\s]?)?\d{3}[-.\s]?\d{4}$`)
	cardPattern     = regexp.MustCompile(`^(?:\d[ -]?){13,16}$`)
	ibanPattern     = regexp.MustCompile(`^[A-Za-z]{2}\d{2}[A-Za-z0-9\s-]{11,30}$`)
	routingPattern  = regexp.MustCompile(`^\d{9}$`)
	passportPattern = regexp.MustCompile(`^[A-Za-z]{2}\d{6,7}$`)
)

// sbPool reuses strings.Builder instances across transformArray calls,
// avoiding a heap allocation per response body under concurrent load.
var sbPool = sync.Pool{New: func() any {
	sb := &strings.Builder{}
	sb.Grow(4096)
	return sb
}}

// maskField detects the value type and applies the right mask pattern recursively for nested objects/arrays.
func maskField(record []byte, field string) ([]byte, error) {
	res := gjson.GetBytes(record, field)
	if !res.Exists() {
		return record, nil
	}

	if !res.IsObject() && !res.IsArray() {
		masked := maskValue(res)
		return sjson.SetBytes(record, field, masked)
	}

	var paths []string
	collectLeafPaths(res, field, &paths)

	var err error
	out := record
	for _, p := range paths {
		val := gjson.GetBytes(out, p)
		masked := maskValue(val)
		out, err = sjson.SetBytes(out, p, masked)
		if err != nil {
			return record, err
		}
	}
	return out, nil
}

func collectLeafPaths(res gjson.Result, currentPath string, paths *[]string) {
	if res.IsObject() {
		res.ForEach(func(key, value gjson.Result) bool {
			p := key.String()
			if currentPath != "" {
				p = currentPath + "." + p
			}
			collectLeafPaths(value, p, paths)
			return true
		})
	} else if res.IsArray() {
		idx := 0
		res.ForEach(func(_, value gjson.Result) bool {
			p := fmt.Sprintf("%s.%d", currentPath, idx)
			collectLeafPaths(value, p, paths)
			idx++
			return true
		})
	} else {
		if currentPath != "" {
			*paths = append(*paths, currentPath)
		}
	}
}

func maskValue(val gjson.Result) interface{} {
	switch val.Type {
	case gjson.String:
		s := val.String()
		if strings.Contains(s, "@") {
			return maskEmail(s)
		} else if ssnPattern.MatchString(s) {
			return maskSSN(s)
		} else if cardPattern.MatchString(s) {
			return maskCreditCard(s)
		} else if ibanPattern.MatchString(s) {
			return maskIBAN(s)
		} else if routingPattern.MatchString(s) {
			return maskABARouting(s)
		} else if passportPattern.MatchString(s) {
			return maskPassport(s)
		} else if phonePattern.MatchString(s) {
			return maskPhone(s)
		}
		return maskGeneric(s)
	case gjson.Number:
		s := val.String()
		if routingPattern.MatchString(s) {
			return maskABARouting(s)
		}
		return maskGeneric(s)
	case gjson.True, gjson.False:
		return val.Bool()
	case gjson.Null:
		return nil
	}
	return val.Value()
}

func anonymizeField(record []byte, field string) ([]byte, error) {
	value := gjson.GetBytes(record, field).String()
	if value == "" {
		return record, nil
	}

	salt := os.Getenv("AUVIS_SALT")
	if salt == "" {
		salt = "default-auvis-salt-value"
	}

	// Calculate SHA-256 with salt
	hasher := sha256.New()
	hasher.Write([]byte(value + salt))
	hashed := hex.EncodeToString(hasher.Sum(nil))

	return sjson.SetBytes(record, field, hashed)
}

func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "***"
	}
	return "***@" + parts[1]
}

func maskSSN(value string) string {
	return "**-**-" + value[7:]
}

func maskCreditCard(value string) string {
	if len(value) == 19 && value[4] == '-' && value[9] == '-' && value[14] == '-' {
		return "***-****-****-" + value[15:]
	}
	if len(value) == 19 && value[4] == ' ' && value[9] == ' ' && value[14] == ' ' {
		return "*** **** **** " + value[15:]
	}
	if len(value) == 16 {
		return "************" + value[12:]
	}
	return maskGeneric(value)
}

func maskPhone(value string) string {
	digitCount := 0
	last4StartIdx := -1
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] >= '0' && value[i] <= '9' {
			digitCount++
			if digitCount == 4 {
				last4StartIdx = i
				break
			}
		}
	}
	if last4StartIdx == -1 {
		return maskGeneric(value)
	}

	var sb strings.Builder
	for i := 0; i < len(value); i++ {
		if i < last4StartIdx {
			if (value[i] >= '0' && value[i] <= '9') || value[i] == '+' || (value[i] >= 'a' && value[i] <= 'z') || (value[i] >= 'A' && value[i] <= 'Z') {
				sb.WriteByte('*')
			} else {
				sb.WriteByte(value[i])
			}
		} else {
			sb.WriteByte(value[i])
		}
	}
	return sb.String()
}

func maskIBAN(value string) string {
	runes := []rune(value)
	if len(runes) < 5 {
		return maskGeneric(value)
	}

	totalAlphanum := 0
	for _, r := range runes {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			totalAlphanum++
		}
	}
	if totalAlphanum < 5 {
		return maskGeneric(value)
	}

	var sb strings.Builder
	charCount := 0
	for _, r := range runes {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			charCount++
			// Preserve first 4 characters (Country Code + Check Digits) and last 4 characters
			if charCount <= 4 || charCount > totalAlphanum-4 {
				sb.WriteRune(r)
			} else {
				sb.WriteRune('*')
			}
		} else {
			sb.WriteRune(r) // Preserve formatting spaces and hyphens
		}
	}
	return sb.String()
}

func maskABARouting(value string) string {
	if len(value) != 9 {
		return maskGeneric(value)
	}
	return "*****" + value[5:]
}

func maskPassport(value string) string {
	n := len(value)
	if n < 5 {
		return maskGeneric(value)
	}
	var sb strings.Builder
	for i := 0; i < n; i++ {
		// Preserve first 2 characters and last 2 characters
		if i < 2 || i >= n-2 {
			sb.WriteByte(value[i])
		} else {
			sb.WriteByte('*')
		}
	}
	return sb.String()
}

func maskGeneric(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return "****" + value[len(value)-4:]
}

func deny(reason string) []byte {
	return []byte(fmt.Sprintf(`{"error":"forbidden","reason":"%s"}`, reason))
}

func redactField(record []byte, field string) ([]byte, error) {
	res := gjson.GetBytes(record, field)
	if !res.Exists() {
		return record, nil
	}

	if !res.IsObject() && !res.IsArray() {
		return sjson.SetBytes(record, field, "[REDACTED]")
	}

	var paths []string
	collectLeafPaths(res, field, &paths)

	var err error
	out := record
	for _, p := range paths {
		out, err = sjson.SetBytes(out, p, "[REDACTED]")
		if err != nil {
			return record, err
		}
	}
	return out, nil
}

func truncateField(record []byte, field string, length int) ([]byte, error) {
	res := gjson.GetBytes(record, field)
	if !res.Exists() {
		return record, nil
	}
	if length <= 0 {
		return record, nil
	}

	if !res.IsObject() && !res.IsArray() {
		truncated := truncateValue(res, length)
		return sjson.SetBytes(record, field, truncated)
	}

	var paths []string
	collectLeafPaths(res, field, &paths)

	var err error
	out := record
	for _, p := range paths {
		val := gjson.GetBytes(out, p)
		truncated := truncateValue(val, length)
		out, err = sjson.SetBytes(out, p, truncated)
		if err != nil {
			return record, err
		}
	}
	return out, nil
}

func truncateValue(val gjson.Result, length int) interface{} {
	if val.Type != gjson.String {
		return val.Value()
	}
	s := val.String()
	runes := []rune(s)
	if len(runes) <= length {
		return s
	}
	return string(runes[:length])
}

func dedupe(in []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}