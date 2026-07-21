package pipeline

import "time"

// Record represents a single data unit flowing through the ingestion pipeline.
// It is the standard envelope for streaming, evaluating, and transforming data in Phase 2.
type Record struct {
	// ID is the unique identifier for the record.
	ID string `json:"id"`

	// Source indicates where the record originated (e.g. "kafka.topic.customer-events").
	Source string `json:"source"`

	// Schema specifies the shape/model of the data if known.
	Schema string `json:"schema"`

	// Data contains the actual payload fields of the record.
	Data map[string]any `json:"data"`

	// Metadata holds operational attributes, headers, offsets, or ingestion tags.
	Metadata map[string]any `json:"metadata"`

	// Timestamp is when the record was ingested or created.
	Timestamp time.Time `json:"timestamp"`

	// FieldTags maps json paths to their schema tags (e.g. "email": ["@pii"]).
	FieldTags map[string][]string `json:"field_tags"`

	// AckFunc is an optional callback invoked by the pipeline after successful processing
	// to acknowledge the record (e.g. commit a Kafka offset for exactly-once semantics).
	// Nil when acknowledgement is not required (at-least-once mode).
	AckFunc func() error `json:"-"`
}
