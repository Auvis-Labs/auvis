package audit

import "time"

// Entry is one record in the audit log — one per request.
// Written by logger.go, read by auvis.go.
//
// Field disposition buckets:
//   fields_masked   — value was transformed (masked/anonymized/redacted/truncated)
//   fields_removed  — field was dropped entirely (action: remove)
//   fields_allowed  — field passed through untouched
//   fields_denied   — field-level deny rule fired
//   response_denied — the whole response was blocked (HTTP 403)
type Entry struct {
	Timestamp          time.Time `json:"timestamp"`
	RequestID          string    `json:"request_id"`
	UserID             string    `json:"user_id"`
	UserRole           string    `json:"user_role"`
	Resource           string    `json:"resource"`
	Method             string    `json:"method"`
	PoliciesApplied    []string  `json:"policies_applied"`
	PolicyHash         string    `json:"policy_hash,omitempty"`
	Priority           int       `json:"priority"`
	ComplianceTags     []string  `json:"compliance_tags"`
	ConditionEvaluated []string  `json:"condition_evaluated,omitempty"`
	FieldsMasked       []string  `json:"fields_masked"`
	FieldsRemoved      []string  `json:"fields_removed"`
	FieldsAllowed      []string  `json:"fields_allowed"`
	FieldsDenied       []string  `json:"fields_denied"`
	FieldsExposedPHI   []string  `json:"fields_exposed_phi,omitempty"`
	Warnings           []string  `json:"warnings,omitempty"`
	ResponseDenied     bool      `json:"response_denied"`
	PiiTypesDetected   []string  `json:"pii_types_detected,omitempty"`
	RowsFiltered       int       `json:"rows_filtered"`
	UpstreamStatus     int       `json:"upstream_status"`
	DurationMS         int64     `json:"duration_ms"`

	// Destination tracking — multi-destination aware.
	// DestinationWritten is true if ALL configured destinations confirmed a successful write.
	// Explicitly serialized as false when a write fails (no omitempty).
	DestinationWritten  bool         `json:"destination_written"`
	DestinationError    string       `json:"destination_error,omitempty"`
	DestinationsWritten int          `json:"destinations_written,omitempty"`
	DestinationsFailed  int          `json:"destinations_failed,omitempty"`
	DestinationMs       int64        `json:"destination_ms,omitempty"`
	DestinationResults  []DestResult `json:"destination_results,omitempty"`

	// Per-record audit detail — populated for multi-record responses only.
	// Enables compliance-grade proof: "what exactly happened to record PT-047?"
	// Omitted entirely for single-record responses to keep the entry compact.
	RecordCount    int             `json:"record_count,omitempty"`
	RecordOutcomes []RecordOutcome `json:"record_outcomes,omitempty"`
}

// DestResult captures the outcome of a single destination write within a fan-out.
type DestResult struct {
	Type       string `json:"type"`
	Written    bool   `json:"written"`
	ErrorMsg   string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// RecordOutcome captures what happened to a single record within a multi-record response.
// Mirrors transform.RecordOutcome — kept as a separate type to avoid circular imports.
type RecordOutcome struct {
	RecordID      string   `json:"record_id"`
	FieldsMasked  []string `json:"fields_masked,omitempty"`
	FieldsRemoved []string `json:"fields_removed,omitempty"`
	FieldsAllowed []string `json:"fields_allowed,omitempty"`
	FieldsDenied  []string `json:"fields_denied,omitempty"`
	Filtered      bool     `json:"filtered,omitempty"`
}