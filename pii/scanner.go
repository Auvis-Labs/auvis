package pii

import (
	"context"
	"strings"

	"github.com/cridiv/Auvis/metrics"
)

// Scanner runs regex first, then optionally LLM for anything missed.
type Scanner struct {
	APIKey      string
	LLMFallback bool
	Mode        string // "regex_only", "llm_only", "hybrid" (default)
}

// Scan detects all PII in text and returns a populated PlaceholderMap.
func (s *Scanner) Scan(ctx context.Context, text string) (*PlaceholderMap, error) {
	pm := NewPlaceholderMap()

	mode := strings.ToLower(s.Mode)
	if mode == "" {
		mode = "hybrid"
	}

	// Pass 1 — regex
	if mode == "regex_only" || mode == "hybrid" {
		metrics.IncPIIScan("regex")
		regexEntities := ScanRegex(text)
		for _, e := range regexEntities {
			pm.Add(e.Value, e.Type)
		}
	}

	// Pass 2 — LLM fallback
	if (mode == "llm_only" || mode == "hybrid") && s.LLMFallback && s.APIKey != "" {
		metrics.IncPIIScan("llm")
		maskedText := pm.Mask(text)
		llmEntities, err := ScanLLM(ctx, maskedText, s.APIKey)
		if err == nil {
			for _, e := range llmEntities {
				pm.Add(e.Value, e.Type)
			}
		}
	}

	return pm, nil
}

