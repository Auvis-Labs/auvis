package pipeline

import (
	"context"
	"encoding/json"

	"github.com/Auvis-Labs/auvis/policy"
	"github.com/Auvis-Labs/auvis/transform"
	"github.com/Auvis-Labs/auvis/types"
)

// NopDestination is a no-op Destination used when no downstream data sink is configured for a source pipeline.
// Records are governance-evaluated and audit-logged but not written to any external sink.
type NopDestination struct{}

func NewNopDestination() *NopDestination          { return &NopDestination{} }
func (n *NopDestination) Connect() error           { return nil }
func (n *NopDestination) WriteRecord(_ Record) error { return nil }
func (n *NopDestination) Flush() error             { return nil }
func (n *NopDestination) Close() error             { return nil }

// DefaultTransformEngine wraps transform.TransformFunc to satisfy the pipeline.TransformEngine interface.
// It JSON-encodes the Record's Data, applies the policy-governed transform function, and decodes the result.
type DefaultTransformEngine struct {
	fn transform.TransformFunc
}

// NewDefaultTransformEngine returns a TransformEngine backed by the gateway's policy-aware TransformFunc.
func NewDefaultTransformEngine(fn transform.TransformFunc) *DefaultTransformEngine {
	return &DefaultTransformEngine{fn: fn}
}

// Transform applies policy-governed field transformations to a pipeline Record.
func (e *DefaultTransformEngine) Transform(_ context.Context, record *Record, ep *policy.EffectivePolicy) (map[string]any, transform.Summary, error) {
	if record == nil {
		return map[string]any{}, transform.Summary{}, nil
	}
	if ep == nil {
		return record.Data, transform.Summary{}, nil
	}

	// JSON-encode the record data so TransformFunc can operate on it
	raw, err := json.Marshal(record.Data)
	if err != nil {
		return record.Data, transform.Summary{}, err
	}

	// Build a minimal RequestContext from the record's source and metadata
	reqCtx := types.RequestContext{
		UserID:   "system",
		UserRole: "pipeline-worker",
		Resource: record.Source,
		Source:   record.Source,
		Trigger:  "ingest",
	}
	if record.Metadata != nil {
		if uid, ok := record.Metadata["user_id"].(string); ok {
			reqCtx.UserID = uid
		}
		if role, ok := record.Metadata["user_role"].(string); ok {
			reqCtx.UserRole = role
		}
	}

	governed, summary, err := e.fn(raw, reqCtx)
	if err != nil {
		return record.Data, transform.Summary{}, err
	}

	var result map[string]any
	if jsonErr := json.Unmarshal(governed, &result); jsonErr != nil {
		return record.Data, summary, jsonErr
	}
	return result, summary, nil
}
