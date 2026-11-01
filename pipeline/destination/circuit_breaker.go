package destination

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cridiv/Auvis/metrics"
	"github.com/cridiv/Auvis/pipeline"
)

// ErrCircuitOpen is returned immediately when a destination's circuit breaker is Open (tripped).
var ErrCircuitOpen = errors.New("circuit breaker is open (destination unavailable)")

// CBConfig specifies failure threshold and cooldown period for a destination's circuit breaker.
type CBConfig struct {
	MaxFailures int
	Cooldown    time.Duration
}

// State represents the 3 operational states of a circuit breaker.
type State int

const (
	// StateClosed — Normal operation: requests pass through to the destination.
	StateClosed State = iota
	// StateOpen — Tripped state: requests fail fast instantly (<1ms) during cooldown.
	StateOpen
	// StateHalfOpen — Test state: allows one test request to check if the destination recovered.
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "Closed"
	case StateOpen:
		return "Open"
	case StateHalfOpen:
		return "Half-Open"
	default:
		return "Unknown"
	}
}

// CircuitBreaker guards destination writes from cascading failures, memory spikes, and timeouts.
type CircuitBreaker struct {
	mu                  sync.Mutex
	name                string
	state               State
	maxFailures         int
	cooldown            time.Duration
	consecutiveFailures int
	openedAt            time.Time
}

// NewCircuitBreaker creates an independent circuit breaker for a named destination driver.
func NewCircuitBreaker(name string, maxFailures int, cooldown time.Duration) *CircuitBreaker {
	if maxFailures <= 0 {
		maxFailures = 5
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	cb := &CircuitBreaker{
		name:        name,
		state:       StateClosed,
		maxFailures: maxFailures,
		cooldown:    cooldown,
	}
	metrics.SetCircuitBreakerState(name, 0) // 0 = Closed
	return cb
}

// Execute runs the write operation inside the circuit breaker safety guard.
func (cb *CircuitBreaker) Execute(op func() error) error {
	cb.mu.Lock()

	now := time.Now()

	// Check if Open state cooldown has expired -> transition to Half-Open
	if cb.state == StateOpen {
		if now.Sub(cb.openedAt) >= cb.cooldown {
			cb.state = StateHalfOpen
			slog.Info("Circuit breaker entering Half-Open state (testing destination health)",
				"destination", cb.name,
			)
			metrics.SetCircuitBreakerState(cb.name, 2) // 2 = Half-Open
		} else {
			cooldownRemaining := (cb.cooldown - now.Sub(cb.openedAt)).Round(time.Second)
			cb.mu.Unlock()
			slog.Warn("Circuit breaker is OPEN — fast failing write operation",
				"destination", cb.name,
				"cooldown_remaining", cooldownRemaining.String(),
			)
			return fmt.Errorf("%w for destination %q", ErrCircuitOpen, cb.name)
		}
	}

	currentState := cb.state
	cb.mu.Unlock()

	// Execute destination write operation
	err := op()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.consecutiveFailures++
		if currentState == StateHalfOpen || cb.consecutiveFailures >= cb.maxFailures {
			if cb.state != StateOpen {
				cb.state = StateOpen
				cb.openedAt = time.Now()
				slog.Warn("Circuit breaker TRIPPED (Open state) — fast failing writes during cooldown",
					"destination", cb.name,
					"consecutive_failures", cb.consecutiveFailures,
					"cooldown", cb.cooldown,
					"last_error", err,
				)
				metrics.SetCircuitBreakerState(cb.name, 1) // 1 = Open
			}
		}
		return err
	}

	// Operation succeeded! Reset failure count and close circuit breaker
	if cb.state == StateHalfOpen || cb.consecutiveFailures > 0 {
		slog.Info("Circuit breaker RESET (Closed state) — destination health restored",
			"destination", cb.name,
		)
		cb.state = StateClosed
		cb.consecutiveFailures = 0
		metrics.SetCircuitBreakerState(cb.name, 0) // 0 = Closed
	}

	return nil
}

// State returns current breaker state.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Name returns the destination name.
func (cb *CircuitBreaker) Name() string {
	return cb.name
}

// BreakerDestination wraps any single pipeline.Destination with an independent CircuitBreaker.
type BreakerDestination struct {
	pipeline.Destination
	Name    string
	Breaker *CircuitBreaker
}

// NewBreakerDestination creates a destination wrapped with an independent circuit breaker.
func NewBreakerDestination(dest pipeline.Destination, name string, cbConfigs ...CBConfig) *BreakerDestination {
	maxFailures := 5
	cooldown := 30 * time.Second
	if len(cbConfigs) > 0 {
		if cbConfigs[0].MaxFailures > 0 {
			maxFailures = cbConfigs[0].MaxFailures
		}
		if cbConfigs[0].Cooldown > 0 {
			cooldown = cbConfigs[0].Cooldown
		}
	}
	return &BreakerDestination{
		Destination: dest,
		Name:        name,
		Breaker:     NewCircuitBreaker(name, maxFailures, cooldown),
	}
}

// WriteRecord executes the underlying WriteRecord call inside the CircuitBreaker.
func (b *BreakerDestination) WriteRecord(rec pipeline.Record) error {
	return b.Breaker.Execute(func() error {
		return b.Destination.WriteRecord(rec)
	})
}

// GetBreakerStates returns a map of destination name to breaker state string.
func (b *BreakerDestination) GetBreakerStates() map[string]string {
	if b.Breaker == nil {
		return map[string]string{b.Name: StateClosed.String()}
	}
	return map[string]string{b.Name: b.Breaker.State().String()}
}
