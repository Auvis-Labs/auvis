package destination

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"github.com/cridiv/Auvis/metrics"
	"github.com/cridiv/Auvis/pipeline"
)

// DestWriteResult records the outcome of a single destination write within a fan-out.
type DestWriteResult struct {
	// Type is the destination driver name (e.g. "postgres", "pgvector").
	Type string
	// Written is true if WriteRecord returned nil.
	Written bool
	// Err holds the error returned by WriteRecord, if any.
	Err error
	// DurationMs is wall-clock time for this individual write.
	DurationMs int64
}

// MultiError is returned by MultiDestination.WriteRecord when one or more destinations fail.
// It carries per-destination outcomes so the proxy handler can populate granular audit fields
// without re-interrogating the destinations.
type MultiError struct {
	Results []DestWriteResult
}

func (m *MultiError) Error() string {
	var failures []string
	for _, r := range m.Results {
		if r.Err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.Type, r.Err))
		}
	}
	return "multi-destination write failed: " + strings.Join(failures, "; ")
}

// MultiDestination fans out WriteRecord calls to a slice of pipeline.Destination implementations.
// Fan-out semantics are fail-closed: if any destination write fails, WriteRecord returns a
// *MultiError carrying per-destination outcomes. The proxy handler must inspect this error to
// populate audit fields and must withhold the response from the client (HTTP 500).
//
// Connect, Flush, and Close are sequential and fail-fast on first error.
type MultiDestination struct {
	destinations []pipeline.Destination
	names        []string // human-readable type labels for logging
	breakers     []*CircuitBreaker
}

// NewMultiDestination creates a MultiDestination.
// dests and names must have the same length.
// Optional cbConfigs sets custom failure thresholds and cooldown periods per destination.
func NewMultiDestination(dests []pipeline.Destination, names []string, cbConfigs ...CBConfig) *MultiDestination {
	breakers := make([]*CircuitBreaker, len(dests))
	for i, name := range names {
		maxFailures := 5
		cooldown := 30 * time.Second
		if i < len(cbConfigs) {
			if cbConfigs[i].MaxFailures > 0 {
				maxFailures = cbConfigs[i].MaxFailures
			}
			if cbConfigs[i].Cooldown > 0 {
				cooldown = cbConfigs[i].Cooldown
			}
		}
		breakers[i] = NewCircuitBreaker(name, maxFailures, cooldown)
	}
	return &MultiDestination{
		destinations: dests,
		names:        names,
		breakers:     breakers,
	}
}

// GetBreaker returns the circuit breaker for a given destination index or name.
func (m *MultiDestination) GetBreaker(name string) *CircuitBreaker {
	for i, n := range m.names {
		if strings.EqualFold(n, name) {
			return m.breakers[i]
		}
	}
	return nil
}

// GetBreakerStates returns a map of destination names to their current circuit breaker state strings.
func (m *MultiDestination) GetBreakerStates() map[string]string {
	states := make(map[string]string, len(m.names))
	for i, name := range m.names {
		if i < len(m.breakers) && m.breakers[i] != nil {
			states[name] = m.breakers[i].State().String()
		} else {
			states[name] = StateClosed.String()
		}
	}
	return states
}

// Connect establishes all destination connections sequentially.
// Fails fast on the first error.
func (m *MultiDestination) Connect() error {
	for i, d := range m.destinations {
		if err := d.Connect(); err != nil {
			return fmt.Errorf("destination[%d] %s connect: %w", i, m.names[i], err)
		}
	}
	return nil
}

// WriteRecord fans out to all destinations and collects results.
// If all writes succeed, returns nil.
// If any write fails, returns a *MultiError with per-destination outcomes.
func (m *MultiDestination) WriteRecord(rec pipeline.Record) error {
	results := make([]DestWriteResult, len(m.destinations))
	anyFailed := false

	for i, d := range m.destinations {
		start := time.Now()
		var err error

		err = m.breakers[i].Execute(func() error {
			var writeErr error
			defer func() {
				if rErr := recover(); rErr != nil {
					metrics.IncPanic("destination_write")
					stack := string(debug.Stack())
					writeErr = fmt.Errorf("panic in destination write (%s): %v", m.names[i], rErr)
					slog.Error("Panic in destination write execution",
						"destination", m.names[i],
						"error", writeErr,
						"stack", stack,
						"request_id", rec.ID,
					)
				}
			}()
			writeErr = d.WriteRecord(rec)
			return writeErr
		})

		dur := time.Since(start).Milliseconds()

		results[i] = DestWriteResult{
			Type:       m.names[i],
			Written:    err == nil,
			Err:        err,
			DurationMs: dur,
		}

		if err != nil {
			anyFailed = true
			if errors.Is(err, ErrCircuitOpen) {
				slog.Warn("Destination write fast-failed (circuit breaker OPEN)",
					"destination", m.names[i],
					"request_id", rec.ID,
				)
			} else {
				slog.Error("Destination write failed",
					"destination", m.names[i],
					"error", err,
					"request_id", rec.ID,
					"duration_ms", dur,
				)
			}
		} else {
			slog.Debug("Destination write succeeded",
				"destination", m.names[i],
				"request_id", rec.ID,
				"duration_ms", dur,
			)
		}
	}

	if anyFailed {
		return &MultiError{Results: results}
	}
	return nil
}

// Flush flushes all destinations sequentially.
// Collects all errors and returns a combined error if any failed.
func (m *MultiDestination) Flush() error {
	var errs []error
	for i, d := range m.destinations {
		if err := d.Flush(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", m.names[i], err))
		}
	}
	return errors.Join(errs...)
}

// Close closes all destinations sequentially.
// Collects all errors and returns a combined error if any failed.
func (m *MultiDestination) Close() error {
	var errs []error
	for i, d := range m.destinations {
		if err := d.Close(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", m.names[i], err))
		}
	}
	return errors.Join(errs...)
}
