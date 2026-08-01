package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/cridiv/Auvis/audit"
	"github.com/cridiv/Auvis/metrics"
	"github.com/cridiv/Auvis/policy"
	"github.com/cridiv/Auvis/transform"
)

// PolicyEngine evaluates policies against records in the pipeline.
// It wraps policy resolution and evaluation logic (including source/destination checks).
type PolicyEngine interface {
	// Evaluate matches policies based on record attributes (source, destination, schemas, etc.)
	// and returns the consolidated EffectivePolicy containing rules to apply.
	Evaluate(ctx context.Context, record *Record) (*policy.EffectivePolicy, error)
}

// TransformEngine applies transformations (masking, removal, truncation, redaction, anonymization)
// to the record's payload based on the matched policy rules.
type TransformEngine interface {
	// Transform modifies the record's payload data and returns the transformed data map
	// along with the execution summary detailing field-level actions and row-filtering statistics.
	Transform(ctx context.Context, record *Record, ep *policy.EffectivePolicy) (map[string]any, transform.Summary, error)
}

// AuditEngine records pipeline transactional metadata for compliance, audit, and security monitoring.
type AuditEngine interface {
	// Log enqueues an audit entry to be written to a secure storage destination.
	Log(entry audit.Entry)
}

// DLQSink routes failed records to a dead-letter queue.
type DLQSink interface {
	Write(ctx context.Context, record Record, reason string) error
	Close() error
}

// Metrics encapsulates thread-safe atomic counters tracking pipeline statistics.
type Metrics struct {
	ProcessedCount    uint64
	FailedCount       uint64
	MatchedCount      uint64
	DLQCount          uint64
	TotalDurationMS   uint64
	Latency2msBucket  uint64
	Latency5msBucket  uint64
	Latency10msBucket uint64
	Latency50msBucket uint64
	LatencyInfBucket  uint64
}

// MetricsSnapshot exposes a JSON-serializable snapshot of pipeline execution counters.
type MetricsSnapshot struct {
	ProcessedCount   uint64             `json:"processed_count"`
	FailedCount      uint64             `json:"failed_count"`
	MatchedCount     uint64             `json:"matched_count"`
	DLQCount         uint64             `json:"dlq_count"`
	ErrorRate        float64            `json:"error_rate"`
	PolicyMatchRate  float64            `json:"policy_match_rate"`
	AvgLatencyMS     float64            `json:"avg_latency_ms"`
	LatencyHistogram map[string]uint64 `json:"latency_histogram"`
}

// Pipeline represents a continuous ingestion data stream.
// It is constructed dynamically from config and manages the lifecycle of reading records,
// running evaluations, executing transforms, routing to destinations, and logging audit metadata.
type Pipeline struct {
	Name            string
	Source          Source
	PolicyEngine    PolicyEngine
	TransformEngine TransformEngine
	Destination     Destination
	DLQ             DLQSink  // Optional dead-letter queue for failed records
	AuditEngine     AuditEngine
	FailClosed      bool // Strategy: if true, failures block record; if false, record is allowed un-transformed or partially written.
	DryRun          bool // Strategy: if true, evaluates policies and log actions but bypasses write to destination.
	metrics         Metrics // Internal high-performance metrics tracker
}

// Run starts the pipeline main loop.
// It establishes source and destination connections, reads records from the source,
// evaluates policies, applies transformations, routes to the destination, and logs audit entries.
// It runs until the context is cancelled or the source channel is exhausted.
func (p *Pipeline) Run(ctx context.Context) error {
	slog.Info("Starting pipeline execution", "pipeline", p.Name, "fail_closed", p.FailClosed)

	if err := p.Source.Connect(); err != nil {
		return fmt.Errorf("pipeline %s failed to connect to source: %w", p.Name, err)
	}
	defer p.Source.Close()

	if err := p.Destination.Connect(); err != nil {
		return fmt.Errorf("pipeline %s failed to connect to destination: %w", p.Name, err)
	}
	defer p.Destination.Close()
	defer func() {
		if err := p.Destination.Flush(); err != nil {
			slog.Error("Failed to flush destination during shutdown", "pipeline", p.Name, "error", err)
		}
	}()

	recordsChan := p.Source.ReadRecords(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Pipeline context cancelled, stopping main loop", "pipeline", p.Name)
			return ctx.Err()
		case record, ok := <-recordsChan:
			if !ok {
				slog.Info("Pipeline source exhausted, finished main loop", "pipeline", p.Name)
				return nil
			}

			p.processRecord(ctx, record)
		}
	}
}

func (p *Pipeline) processRecord(ctx context.Context, record Record) {
	defer func() {
		if rErr := recover(); rErr != nil {
			metrics.IncPanic("pipeline_process")
			stack := string(debug.Stack())
			slog.Error("Panic recovered in pipeline record processing",
				"error", rErr,
				"stack", stack,
				"pipeline", p.Name,
				"record_id", record.ID,
			)
			atomic.AddUint64(&p.metrics.FailedCount, 1)
		}
	}()

	startTime := time.Now()
	requestID := record.ID
	if requestID == "" {
		requestID = fmt.Sprintf("rec-%d", startTime.UnixNano())
	}

	// Prepare dynamic context metadata from Record
	var userID, userRole string
	if record.Metadata != nil {
		if u, ok := record.Metadata["user_id"].(string); ok {
			userID = u
		}
		if r, ok := record.Metadata["user_role"].(string); ok {
			userRole = r
		}
	}
	if userID == "" {
		userID = "system"
	}
	if userRole == "" {
		userRole = "pipeline-worker"
	}

	var ep *policy.EffectivePolicy
	var evalErr error
	var summary transform.Summary
	var transformErr error
	var writeErr error

	// 1. Evaluate policies
	if p.PolicyEngine != nil {
		ep, evalErr = p.PolicyEngine.Evaluate(ctx, &record)
	}

	var transformedData map[string]any
	var shouldWrite = true
	var failedClosed = false

	if evalErr != nil {
		slog.Error("Pipeline policy evaluation failed", "pipeline", p.Name, "record_id", record.ID, "error", evalErr)
		if p.FailClosed {
			shouldWrite = false
			failedClosed = true
		} else {
			// Fail-open: proceed with untransformed data
			transformedData = record.Data
		}
	} else if ep == nil {
		// Nil policy means Deny by default in Auvis
		slog.Warn("No matching policy found for record, deny by default", "pipeline", p.Name, "record_id", record.ID)
		shouldWrite = false
		failedClosed = true
	} else if ep.Deny {
		slog.Warn("Explicit policy deny triggered for record", "pipeline", p.Name, "record_id", record.ID, "policies", ep.SourcePolicies)
		shouldWrite = false
	} else {
		// 2. Apply transforms
		if p.TransformEngine != nil {
			transformedData, summary, transformErr = p.TransformEngine.Transform(ctx, &record, ep)
		} else {
			transformedData = record.Data
		}

		if transformErr != nil {
			slog.Error("Pipeline transformation failed", "pipeline", p.Name, "record_id", record.ID, "error", transformErr)
			if p.FailClosed {
				shouldWrite = false
				failedClosed = true
			} else {
				// Fail-open: proceed with untransformed data
				transformedData = record.Data
			}
		}
	}

	// 3. Write to destination (or route to DLQ for dropped records)
	if shouldWrite {
		if p.DryRun {
			slog.Info("Dry-run mode active — bypassing write to destination", "pipeline", p.Name, "record_id", record.ID)
			if record.AckFunc != nil {
				if err := record.AckFunc(); err != nil {
					slog.Error("Failed to acknowledge record offset in dry-run", "pipeline", p.Name, "record_id", record.ID, "error", err)
				}
			}
		} else {
			recToWrite := Record{
				ID:        record.ID,
				Source:    record.Source,
				Schema:    record.Schema,
				Data:      transformedData,
				Metadata:  record.Metadata,
				Timestamp: record.Timestamp,
			}
			writeErr = p.Destination.WriteRecord(recToWrite)
			if writeErr != nil {
				slog.Error("Pipeline destination write failed", "pipeline", p.Name, "record_id", record.ID, "error", writeErr)
			} else if record.AckFunc != nil {
				if err := record.AckFunc(); err != nil {
					slog.Error("Failed to acknowledge record offset", "pipeline", p.Name, "record_id", record.ID, "error", err)
				}
			}
		}
	} else {
		if p.DLQ != nil {
			// Route dropped records to dead-letter queue
			var dlqReason string
			if evalErr != nil {
				dlqReason = fmt.Sprintf("policy_evaluation_error: %v", evalErr)
			} else if ep != nil && ep.Deny {
				dlqReason = fmt.Sprintf("policy_deny: %v", ep.SourcePolicies)
			} else if transformErr != nil {
				dlqReason = fmt.Sprintf("transform_error: %v", transformErr)
			} else {
				dlqReason = "no_matching_policy"
			}
			if dlqErr := p.DLQ.Write(ctx, record, dlqReason); dlqErr != nil {
				slog.Error("Failed to write record to DLQ", "pipeline", p.Name, "record_id", record.ID, "error", dlqErr)
			} else {
				atomic.AddUint64(&p.metrics.DLQCount, 1)
				if record.AckFunc != nil {
					if err := record.AckFunc(); err != nil {
						slog.Error("Failed to acknowledge record offset after DLQ write", "pipeline", p.Name, "record_id", record.ID, "error", err)
					}
				}
			}
		} else {
			// Dropped silently when DLQ is not configured
			if record.AckFunc != nil {
				if err := record.AckFunc(); err != nil {
					slog.Error("Failed to acknowledge record offset after silent drop", "pipeline", p.Name, "record_id", record.ID, "error", err)
				}
			}
		}
	}

	// 4. Log audit entry
	duration := time.Since(startTime).Milliseconds()
	var policiesApplied, complianceTags, fieldsMasked, fieldsRemoved, fieldsAllowed, fieldsDenied, conds []string
	var rowsFiltered int
	var priority int

	if ep != nil {
		policiesApplied = ep.SourcePolicies
		complianceTags = ep.ComplianceTags
		priority = ep.Priority
	}

	// Populate disposition fields from execution summary
	fieldsMasked = summary.FieldsMasked
	fieldsRemoved = summary.FieldsRemoved
	fieldsAllowed = summary.FieldsAllowed
	fieldsDenied = summary.FieldsDenied
	rowsFiltered = summary.RowsFiltered
	conds = summary.ConditionEvaluated

	// Handle denied/blocked records' disposition tracking
	if !shouldWrite {
		fieldsDenied = []string{"*"}
	}

	auditEntry := audit.Entry{
		Timestamp:          time.Now(),
		RequestID:          requestID,
		UserID:             userID,
		UserRole:           userRole,
		Resource:           record.Source,
		Method:             "INGEST",
		PoliciesApplied:    policiesApplied,
		Priority:           priority,
		ComplianceTags:     complianceTags,
		ConditionEvaluated: conds,
		FieldsMasked:       fieldsMasked,
		FieldsRemoved:      fieldsRemoved,
		FieldsAllowed:      fieldsAllowed,
		FieldsDenied:       fieldsDenied,
		ResponseDenied:     !shouldWrite,
		RowsFiltered:       rowsFiltered,
		UpstreamStatus:     200,
		DurationMS:         duration,
	}

	if writeErr != nil {
		auditEntry.UpstreamStatus = 500
	}
	if evalErr != nil || transformErr != nil {
		if failedClosed {
			auditEntry.UpstreamStatus = 403
		}
	}

	if p.AuditEngine != nil {
		p.AuditEngine.Log(auditEntry)
	}

	// --- Update Ingestion Metrics ---
	atomic.AddUint64(&p.metrics.ProcessedCount, 1)
	if evalErr != nil || transformErr != nil || writeErr != nil || !shouldWrite {
		atomic.AddUint64(&p.metrics.FailedCount, 1)
	}
	if ep != nil && !ep.Deny && evalErr == nil && transformErr == nil {
		atomic.AddUint64(&p.metrics.MatchedCount, 1)
	}
	atomic.AddUint64(&p.metrics.TotalDurationMS, uint64(duration))

	// Categorize duration into histogram buckets
	if duration <= 2 {
		atomic.AddUint64(&p.metrics.Latency2msBucket, 1)
	} else if duration <= 5 {
		atomic.AddUint64(&p.metrics.Latency5msBucket, 1)
	} else if duration <= 10 {
		atomic.AddUint64(&p.metrics.Latency10msBucket, 1)
	} else if duration <= 50 {
		atomic.AddUint64(&p.metrics.Latency50msBucket, 1)
	} else {
		atomic.AddUint64(&p.metrics.LatencyInfBucket, 1)
	}
}

// GetMetrics compiles and returns a thread-safe snapshot of pipeline counters and rate statistics.
func (p *Pipeline) GetMetrics() MetricsSnapshot {
	processed := atomic.LoadUint64(&p.metrics.ProcessedCount)
	failed := atomic.LoadUint64(&p.metrics.FailedCount)
	matched := atomic.LoadUint64(&p.metrics.MatchedCount)
	totDuration := atomic.LoadUint64(&p.metrics.TotalDurationMS)

	var errorRate float64
	var matchRate float64
	var avgLatency float64

	if processed > 0 {
		errorRate = float64(failed) / float64(processed)
		matchRate = float64(matched) / float64(processed)
		avgLatency = float64(totDuration) / float64(processed)
	}

	return MetricsSnapshot{
		ProcessedCount:  processed,
		FailedCount:     failed,
		MatchedCount:    matched,
		DLQCount:        atomic.LoadUint64(&p.metrics.DLQCount),
		ErrorRate:       errorRate,
		PolicyMatchRate: matchRate,
		AvgLatencyMS:    avgLatency,
		LatencyHistogram: map[string]uint64{
			"le_2ms":  atomic.LoadUint64(&p.metrics.Latency2msBucket),
			"le_5ms":  atomic.LoadUint64(&p.metrics.Latency5msBucket),
			"le_10ms": atomic.LoadUint64(&p.metrics.Latency10msBucket),
			"le_50ms": atomic.LoadUint64(&p.metrics.Latency50msBucket),
			"le_inf":  atomic.LoadUint64(&p.metrics.LatencyInfBucket),
		},
	}
}


