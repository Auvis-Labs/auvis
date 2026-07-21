package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

// PipelineStatus represents the operational status of a pipeline.
type PipelineStatus string

const (
	StatusStopped PipelineStatus = "stopped"
	StatusRunning PipelineStatus = "running"
	StatusFailed  PipelineStatus = "failed"
)

// RunState monitors the running lifecycle context and execution errors of a pipeline.
type RunState struct {
	Cancel context.CancelFunc
	Status PipelineStatus
	Err    error
	Done   chan struct{}
}

// Registry manages registration, concurrent execution, graceful teardowns,
// and health diagnostics of multiple named ingestion pipelines.
type Registry struct {
	mu        sync.RWMutex
	pipelines map[string]*Pipeline
	states    map[string]*RunState
}

// NewRegistry creates a thread-safe pipelines Registry.
func NewRegistry() *Registry {
	return &Registry{
		pipelines: make(map[string]*Pipeline),
		states:    make(map[string]*RunState),
	}
}

// Register registers a new pipeline with the registry.
// Returns an error if a pipeline with the identical Name is already registered.
func (r *Registry) Register(p *Pipeline) error {
	if p.Name == "" {
		return errors.New("cannot register pipeline with empty name")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.pipelines[p.Name]; exists {
		return fmt.Errorf("pipeline with name %q is already registered", p.Name)
	}

	r.pipelines[p.Name] = p
	r.states[p.Name] = &RunState{
		Status: StatusStopped,
		Done:   make(chan struct{}),
	}
	close(r.states[p.Name].Done) // stop/done by default

	slog.Info("Pipeline registered successfully", "pipeline", p.Name)
	return nil
}

// Start launches the named pipeline in a background goroutine.
// Blocks briefly to verify the pipeline exists and isn't already running.
func (r *Registry) Start(ctx context.Context, name string) error {
	r.mu.Lock()
	p, ok := r.pipelines[name]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("pipeline %q not found in registry", name)
	}

	state := r.states[name]
	select {
	case <-state.Done:
		// Not running — we can proceed to start it.
	default:
		r.mu.Unlock()
		return fmt.Errorf("pipeline %q is already running", name)
	}

	// Create running context
	runCtx, cancel := context.WithCancel(ctx)
	doneChan := make(chan struct{})
	r.states[name] = &RunState{
		Cancel: cancel,
		Status: StatusRunning,
		Done:   doneChan,
	}
	r.mu.Unlock()

	slog.Info("Launching pipeline", "pipeline", name)

	go func() {
		defer close(doneChan)

		err := p.Run(runCtx)

		r.mu.Lock()
		defer r.mu.Unlock()

		// Retrieve latest state to avoid overwriting newer runs
		currState := r.states[name]
		if currState.Done == doneChan {
			if err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("Pipeline execution terminated with error", "pipeline", name, "error", err)
				currState.Status = StatusFailed;
				currState.Err = err
			} else {
				slog.Info("Pipeline execution completed gracefully", "pipeline", name)
				currState.Status = StatusStopped
				currState.Err = nil
			}
		}
	}()

	return nil
}

// StartAll launches all registered pipelines in the background concurrently.
func (r *Registry) StartAll(ctx context.Context) {
	r.mu.RLock()
	names := make([]string, 0, len(r.pipelines))
	for name := range r.pipelines {
		names = append(names, name)
	}
	r.mu.RUnlock()

	for _, name := range names {
		if err := r.Start(ctx, name); err != nil {
			slog.Error("Failed to auto-start pipeline", "pipeline", name, "error", err)
		}
	}
}

// Stop gracefully signals a running pipeline to stop, drains any in-flight records,
// flushes its destination buffer, and blocks until teardown is fully complete.
func (r *Registry) Stop(name string) error {
	r.mu.Lock()
	state, exists := r.states[name]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("pipeline %q not registered", name)
	}

	select {
	case <-state.Done:
		// Already stopped
		r.mu.Unlock()
		return nil
	default:
		// Active, trigger cancellation
		state.Cancel()
	}
	r.mu.Unlock()

	slog.Info("Stopping pipeline gracefully (draining buffered records)", "pipeline", name)
	<-state.Done // block until execution terminates cleanly

	return nil
}

// StopAll halts all registered pipelines concurrently and waits for all to exit cleanly.
func (r *Registry) StopAll() {
	r.mu.RLock()
	names := make([]string, 0, len(r.pipelines))
	for name := range r.pipelines {
		names = append(names, name)
	}
	r.mu.RUnlock()

	var wg sync.WaitGroup
	for _, name := range names {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			if err := r.Stop(n); err != nil {
				slog.Error("Error stopping pipeline during shutdown", "pipeline", n, "error", err)
			}
		}(name)
	}
	wg.Wait()
	slog.Info("All registry pipelines stopped successfully")
}

// HealthCheck returns the current operational status and last recorded error of the named pipeline.
func (r *Registry) HealthCheck(name string) (PipelineStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, ok := r.states[name]
	if !ok {
		return StatusStopped, fmt.Errorf("pipeline %q not found in registry", name)
	}

	return state.Status, state.Err
}

// Unregister deletes a pipeline from the registry. The pipeline must be stopped first.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.states[name]
	if !ok {
		return nil
	}

	select {
	case <-state.Done:
		// Stopped, safe to unregister
	default:
		return fmt.Errorf("cannot unregister running pipeline %q, stop it first", name)
	}

	delete(r.pipelines, name)
	delete(r.states, name)
	slog.Info("Pipeline unregistered", "pipeline", name)
	return nil
}
