package stream

import "time"

// StreamEvent represents a single real-time transformation event broadcasted over SSE.
type StreamEvent struct {
	Timestamp       time.Time      `json:"timestamp"`
	RequestID       string         `json:"request_id"`
	Source          string         `json:"source"`           // e.g. "kafka:patient-records" or "proxy"
	Resource        string         `json:"resource"`         // e.g. "/api/v1/patients"
	UserRole        string         `json:"user_role"`
	RawPayload      map[string]any `json:"raw_payload"`      // Payload before governance (with PII)
	GovernedPayload map[string]any `json:"governed_payload"` // Payload after governance (masked/pseudonymized)
	RulesApplied    []string       `json:"rules_applied"`    // e.g. ["mask: email", "remove: ssn", "pseudonymize: patient_id"]
	FieldsMasked    []string       `json:"fields_masked"`
	FieldsRemoved   []string       `json:"fields_removed"`
}
