package pipeline

import (
	"context"
	"fmt"
	"sync"

	"github.com/cridiv/Auvis/policy"
)

// DefaultPolicyEngine implements PolicyEngine with an LRU-style or simple map caching layer.
type DefaultPolicyEngine struct {
	store *policy.Store

	mu    sync.RWMutex
	cache map[string]*policy.EffectivePolicy
}

// NewDefaultPolicyEngine creates a new cached policy engine.
func NewDefaultPolicyEngine(store *policy.Store) *DefaultPolicyEngine {
	return &DefaultPolicyEngine{
		store: store,
		cache: make(map[string]*policy.EffectivePolicy),
	}
}

// Hash returns the active policy store SHA-256 hash.
func (e *DefaultPolicyEngine) Hash() string {
	if e.store == nil {
		return ""
	}
	return e.store.Hash()
}

// Evaluate matches policies based on record attributes and returns the consolidated EffectivePolicy.
// It leverages a thread-safe caching layer to bypass store.Lookup on repeated identical context evaluations.
func (e *DefaultPolicyEngine) Evaluate(ctx context.Context, record *Record) (*policy.EffectivePolicy, error) {
	// Determine user role and id from metadata
	userRole := "pipeline-worker" // Default fallback
	if record.Metadata != nil {
		if r, ok := record.Metadata["user_role"].(string); ok {
			userRole = r
		}
	}

	// Schema maps to resource in our model for pipelines.
	// If no explicit schema is set (common for raw Kafka JSON messages), fall back
	// to record.Source (e.g. "kafka.patient-records") so policies with resource: "*"
	// or matching the source name will correctly apply to ingest records.
	resource := record.Schema
	if resource == "" {
		resource = record.Source
	}

	// Determine cache key
	// Key structure: role::resource::source::destination
	// Note: We use "ingest" as the trigger for pipeline evaluations
	trigger := "ingest"
	cacheKey := fmt.Sprintf("%s::%s::%s::%s", userRole, resource, record.Source, trigger)

	e.mu.RLock()
	ep, ok := e.cache[cacheKey]
	e.mu.RUnlock()

	if ok {
		return ep, nil
	}

	// Cache miss: perform full lookup
	// Note: Pipelines might have a "service" mapped to something else, we pass empty string for now or derive from source
	ep = e.store.Lookup(userRole, resource, "", record.Source, "", trigger)

	// Store in cache
	e.mu.Lock()
	e.cache[cacheKey] = ep
	e.mu.Unlock()

	return ep, nil
}
