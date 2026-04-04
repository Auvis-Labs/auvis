package audit

import "time"

// Entry is one record in the audit log — one per request.
// Written by logger.go, read by iris.go.
type Entry struct {
	Timestamp       time.Time `json:"timestamp"`
	RequestID       string    `json:"request_id"`
	UserID          string    `json:"user_id"`
	UserRole        string    `json:"user_role"`
	Resource        string    `json:"resource"`
	Method          string    `json:"method"`
	PoliciesApplied []string  `json:"policies_applied"`
	FieldsMasked    []string  `json:"fields_masked"`
	RowsFiltered    int       `json:"rows_filtered"`
	UpstreamStatus  int       `json:"upstream_status"`
	DurationMS      int64     `json:"duration_ms"`
}