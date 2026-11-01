package pii

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cridiv/Auvis/transform"
	"github.com/cridiv/Auvis/types"
)

// LLMInterceptor handles outbound LLM requests.
// It scans the request body for PII, masks it, forwards the request,
// then optionally rehydrates the response.
type LLMInterceptor struct {
	Scanner   *Scanner
	Rehydrate bool
}

// InterceptRequest scans and masks PII in an LLM API request body.
// Returns the masked body and the placeholder map for rehydration.
func (li *LLMInterceptor) InterceptRequest(
	ctx context.Context,
	body []byte,
	reqCtx types.RequestContext,
) ([]byte, *PlaceholderMap, transform.Summary, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, nil, transform.Summary{}, fmt.Errorf("parsing llm request: %w", err)
	}

	allText := extractLLMText(req)

	pm, err := li.Scanner.Scan(ctx, allText)
	if err != nil {
		return nil, nil, transform.Summary{}, fmt.Errorf("pii scan: %w", err)
	}

	masked := pm.Mask(string(body))

	summary := transform.Summary{
		PoliciesApplied:  []string{"llm-pii-intercept"},
		FieldsMasked:     collectPlaceholderTypes(pm),
		PiiTypesDetected: pm.DetectedTypes(),
	}

	return []byte(masked), pm, summary, nil
}

// InterceptResponse optionally rehydrates placeholders in the LLM response.
func (li *LLMInterceptor) InterceptResponse(
	body []byte,
	pm *PlaceholderMap,
) ([]byte, error) {
	if !li.Rehydrate || pm == nil {
		return body, nil
	}

	rehydrated := pm.Rehydrate(string(body))
	return []byte(rehydrated), nil
}

func extractLLMText(req map[string]any) string {
	var sb strings.Builder

	messages, ok := req["messages"].([]any)
	if !ok {
		return ""
	}

	for _, msg := range messages {
		m, ok := msg.(map[string]any)
		if !ok {
			continue
		}

		switch content := m["content"].(type) {
		case string:
			sb.WriteString(content)
			sb.WriteString(" ")
		case []any:
			for _, block := range content {
				b, ok := block.(map[string]any)
				if !ok {
					continue
				}
				if text, ok := b["text"].(string); ok {
					sb.WriteString(text)
					sb.WriteString(" ")
				}
			}
		}
	}

	return sb.String()
}

func collectPlaceholderTypes(pm *PlaceholderMap) []string {
	seen := make(map[string]bool)
	var types []string
	for _, placeholder := range pm.forward {
		trimmed := strings.Trim(placeholder, "[]")
		parts := strings.Split(trimmed, "_")
		if len(parts) > 0 && !seen[parts[0]] {
			seen[parts[0]] = true
			types = append(types, parts[0])
		}
	}
	return types
}
