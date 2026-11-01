package pii

import (
	"context"
	"regexp"
)

var (
	ossRegexEmail = regexp.MustCompile(`(?i)[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	ossRegexPhone = regexp.MustCompile(`\b(?:\+?1[-.\s]?)?\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}\b`)
	ossRegexSSN   = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	ossRegexIPv4  = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
)

// ScanRegex detects common PII formats (Email, Phone, SSN, IPv4) in plain text.
func ScanRegex(text string) []Entity {
	var entities []Entity

	for _, m := range ossRegexEmail.FindAllStringIndex(text, -1) {
		entities = append(entities, Entity{
			Value:      text[m[0]:m[1]],
			Type:       "EMAIL",
			Start:      m[0],
			End:        m[1],
			Confidence: 0.95,
		})
	}

	for _, m := range ossRegexPhone.FindAllStringIndex(text, -1) {
		entities = append(entities, Entity{
			Value:      text[m[0]:m[1]],
			Type:       "PHONE",
			Start:      m[0],
			End:        m[1],
			Confidence: 0.90,
		})
	}

	for _, m := range ossRegexSSN.FindAllStringIndex(text, -1) {
		entities = append(entities, Entity{
			Value:      text[m[0]:m[1]],
			Type:       "SSN",
			Start:      m[0],
			End:        m[1],
			Confidence: 0.95,
		})
	}

	for _, m := range ossRegexIPv4.FindAllStringIndex(text, -1) {
		entities = append(entities, Entity{
			Value:      text[m[0]:m[1]],
			Type:       "IP",
			Start:      m[0],
			End:        m[1],
			Confidence: 0.85,
		})
	}

	return entities
}

// ScanLLM is a stub in the open-source distribution. LLM-based contextual scanning
// is available in Auvis Enterprise.
func ScanLLM(ctx context.Context, text string, apiKey string) ([]Entity, error) {
	return nil, nil
}

