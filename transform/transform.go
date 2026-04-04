package transform

import (
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"hebris/policy"
	"hebris/types"
)

// Summary carries what the transform did — fed into the audit log.
type Summary struct {
	PoliciesApplied []string
	FieldsMasked    []string
	RowsFiltered    int
	Denied          bool
	DenyReason      string
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

// apply is the entry point for every response body.
// It looks up the policy, then delegates to the appropriate path.
func apply(body []byte, reqCtx types.RequestContext, store *policy.Store) ([]byte, Summary, error) {
	log.Printf("[TRANSFORM] request_id=%s role=%s resource=%s — evaluating...",
		reqCtx.RequestID, reqCtx.UserRole, reqCtx.Resource)

	ep := store.Lookup(reqCtx.UserRole, reqCtx.Resource)

	// No policy matched — deny by default
	if ep == nil {
		log.Printf("[TRANSFORM] request_id=%s — DENY no policy matched role=%s resource=%s",
			reqCtx.RequestID, reqCtx.UserRole, reqCtx.Resource)
		reason := "no policy found for your role on this resource"
		return deny(reason), Summary{Denied: true, DenyReason: reason}, nil
	}

	// Explicit deny
	if ep.Deny {
		log.Printf("[TRANSFORM] request_id=%s — DENY explicit deny (policies: %v)",
			reqCtx.RequestID, ep.SourcePolicies)
		reason := "access denied by policy"
		return deny(reason), Summary{
			Denied:          true,
			DenyReason:      reason,
			PoliciesApplied: ep.SourcePolicies,
		}, nil
	}

	// Wildcard allow — passthrough, nothing to transform
	if action, ok := ep.Rules["*"]; ok && action == policy.ActionAllow {
		log.Printf("[TRANSFORM] request_id=%s — ALLOW all fields (policies: %v)",
			reqCtx.RequestID, ep.SourcePolicies)
		return body, Summary{PoliciesApplied: ep.SourcePolicies}, nil
	}

	log.Printf("[TRANSFORM] request_id=%s — applying rules from policies: %v",
		reqCtx.RequestID, ep.SourcePolicies)

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
	var allMasked []string

	isArray := target.IsArray()
	if ep.Response.Array != nil {
		// Policy explicitly tells us — trust it over auto-detection
		isArray = *ep.Response.Array
	}

	if isArray {
		transformed, allMasked, err = transformArray(target, reqCtx, ep)
	} else {
		var masked []string
		transformed, masked, err = transformRecord([]byte(target.Raw), reqCtx, ep)
		allMasked = masked
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
		PoliciesApplied: ep.SourcePolicies,
		FieldsMasked:    dedupe(allMasked),
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

// transformArray applies field rules to every element in a JSON array.
func transformArray(arr gjson.Result, reqCtx types.RequestContext, ep *policy.EffectivePolicy) ([]byte, []string, error) {
	var sb strings.Builder
	sb.WriteString("[")

	var allMasked []string
	i := 0
	var lastErr error

	arr.ForEach(func(_, value gjson.Result) bool {
		out, masked, err := transformRecord([]byte(value.Raw), reqCtx, ep)
		if err != nil {
			lastErr = err
			return false
		}
		if i > 0 {
			sb.WriteString(",")
		}
		sb.Write(out)
		allMasked = append(allMasked, masked...)
		i++
		return true
	})

	if lastErr != nil {
		return nil, nil, lastErr
	}

	sb.WriteString("]")
	return []byte(sb.String()), allMasked, nil
}

// transformRecord applies field rules to a single JSON object.
// Returns the modified record and the list of fields that were masked.
func transformRecord(record []byte, reqCtx types.RequestContext, ep *policy.EffectivePolicy) ([]byte, []string, error) {
	out := record
	var err error
	var masked []string
	var removed []string

	for field, action := range ep.Rules {
		switch action {
		case policy.ActionMask:
			out, err = maskField(out, field)
			if err != nil {
				return nil, nil, fmt.Errorf("masking field %q: %w", field, err)
			}
			masked = append(masked, field)

		case policy.ActionRemove, policy.ActionDeny:
			if gjson.GetBytes(out, field).Exists() {
				out, err = sjson.DeleteBytes(out, field)
				if err != nil {
					return nil, nil, fmt.Errorf("removing field %q: %w", field, err)
				}
				removed = append(removed, field)
			}

		case policy.ActionAllow:
			// explicit allow on a specific field — do nothing
		}
	}

	log.Printf("[TRANSFORM] request_id=%s id=%v — masked: %v  removed: %v",
		reqCtx.RequestID,
		gjson.GetBytes(out, "id").Value(),
		masked,
		removed,
	)

	return out, masked, nil
}

// maskField detects the value type and applies the right mask pattern.
// Email → "***@domain.com", everything else → "****last4"
func maskField(record []byte, field string) ([]byte, error) {
	value := gjson.GetBytes(record, field).String()
	if value == "" {
		return record, nil
	}

	var masked string
	if strings.Contains(value, "@") {
		masked = maskEmail(value)
	} else {
		masked = maskGeneric(value)
	}

	return sjson.SetBytes(record, field, masked)
}

func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "***"
	}
	return "***@" + parts[1]
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