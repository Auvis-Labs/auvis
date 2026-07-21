package pipeline

// Destination is the interface that all pipeline data sinks must implement.
// A Destination receives processed, policy-evaluated records and writes them
// to a downstream system (vector database, search index, data lake, etc.).
//
// Lifecycle:
//
//	1. Connect()      — establish the connection, authenticate, validate config
//	2. WriteRecord()  — write records one at a time (called per-record by the pipeline)
//	3. Flush()        — ensure all buffered writes are committed
//	4. Close()        — flush remaining data, tear down connection
//
// Implementations must be safe for sequential WriteRecord calls.
// The pipeline guarantees it will not call WriteRecord concurrently.
type Destination interface {
	// Connect establishes the connection to the downstream data sink.
	// This is where authentication, TLS setup, and schema validation happen.
	// Must be called before WriteRecord.
	Connect() error

	// WriteRecord writes a single processed record to the destination.
	// The record has already passed through policy evaluation and transformation.
	// Returns an error if the write fails — the pipeline's error handling
	// strategy (fail-open vs fail-closed) determines what happens next.
	WriteRecord(record Record) error

	// Flush commits any buffered writes to the destination.
	// Called periodically by the pipeline and during graceful shutdown.
	Flush() error

	// Close gracefully shuts down the destination connection.
	// Implementations should call Flush internally before releasing resources.
	Close() error
}
