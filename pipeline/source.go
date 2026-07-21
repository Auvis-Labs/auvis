package pipeline

import "context"

// Source is the interface that all pipeline data sources must implement.
// A Source connects to an external system (Kafka, database, API, file store, etc.)
// and streams records into the pipeline for policy evaluation and transformation.
//
// Lifecycle:
//
//	1. Connect()      — establish the connection, authenticate, validate config
//	2. ReadRecords()  — begin streaming records (blocks until ctx is cancelled)
//	3. Close()        — tear down connection, release resources
//
// Implementations must be safe for concurrent use after Connect() returns.
type Source interface {
	// Connect establishes the connection to the upstream data source.
	// This is where authentication, TLS setup, and initial handshake happen.
	// Must be called before ReadRecords.
	Connect() error

	// ReadRecords returns a channel of Record values read from the source.
	// The channel is closed when the context is cancelled or the source is exhausted.
	// Errors encountered during reading are embedded in the Record's Metadata
	// under the "error" key, allowing the pipeline to handle them per-record.
	ReadRecords(ctx context.Context) <-chan Record

	// Close gracefully shuts down the source connection and releases resources.
	// After Close returns, no more records will be sent on any previously
	// returned channel.
	Close() error
}
