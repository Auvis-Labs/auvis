package pii

import (
	"fmt"
	"strings"
	"sync"
)

// Entity represents a detected PII value and its type.
type Entity struct {
	Value      string
	Type       string // EMAIL, PHONE, SSN, CARD, NAME, IP, DATE_OF_BIRTH, IBAN, ROUTING_NUMBER, PASSPORT, NATIONAL_ID
	Start      int
	End        int
	Confidence float64 `json:"confidence"`
}

// PlaceholderMap holds the bidirectional mapping for one request.
// It is safe for concurrent access.
type PlaceholderMap struct {
	mu       sync.RWMutex
	forward  map[string]string
	reverse  map[string]string
	counters map[string]int
}

func NewPlaceholderMap() *PlaceholderMap {
	return &PlaceholderMap{
		forward:  make(map[string]string),
		reverse:  make(map[string]string),
		counters: make(map[string]int),
	}
}

// Add registers a real value and returns its placeholder.
// Idempotent — same value always gets same placeholder.
func (m *PlaceholderMap) Add(value, piiType string) string {
	m.mu.RLock()
	existing, ok := m.forward[value]
	m.mu.RUnlock()
	if ok {
		return existing
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.forward[value]; ok {
		return existing
	}

	m.counters[piiType]++
	placeholder := fmt.Sprintf("[%s_%d]", piiType, m.counters[piiType])
	m.forward[value] = placeholder
	m.reverse[placeholder] = value
	return placeholder
}

// Mask replaces all known real values in text with placeholders.
func (m *PlaceholderMap) Mask(text string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := text
	for real, placeholder := range m.forward {
		result = strings.ReplaceAll(result, real, placeholder)
	}
	return result
}

// Rehydrate replaces all placeholders in text with real values.
func (m *PlaceholderMap) Rehydrate(text string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := text
	for placeholder, real := range m.reverse {
		result = strings.ReplaceAll(result, placeholder, real)
	}
	return result
}

// DetectedTypes returns a list of unique PII types detected in the map.
func (m *PlaceholderMap) DetectedTypes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	seen := make(map[string]bool)
	var types []string
	for _, placeholder := range m.forward {
		trimmed := strings.Trim(placeholder, "[]")
		parts := strings.Split(trimmed, "_")
		if len(parts) > 0 && !seen[parts[0]] {
			seen[parts[0]] = true
			types = append(types, parts[0])
		}
	}
	return types
}

