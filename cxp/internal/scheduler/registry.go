package scheduler

import (
	"fmt"
	"sort"
	"sync"
)

// Registry maps workflow_type strings to runner implementations.
type Registry struct {
	mu      sync.RWMutex
	runners map[string]WorkflowRunner
}

func NewRegistry() *Registry {
	return &Registry{runners: make(map[string]WorkflowRunner)}
}

// Register adds a runner. Overwrites any existing runner with the same Name.
func (r *Registry) Register(runner WorkflowRunner) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runners[runner.Name()] = runner
}

// Get returns the runner for the given workflow_type, or an error if none is registered.
func (r *Registry) Get(workflowType string) (WorkflowRunner, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	runner, ok := r.runners[workflowType]
	if !ok {
		return nil, fmt.Errorf("no runner registered for workflow_type %q", workflowType)
	}
	return runner, nil
}

// List returns the registered workflow_types, sorted.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.runners))
	for name := range r.runners {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DefaultRegistry is the package-level registry used by cxp schedule run.
var DefaultRegistry = NewRegistry()
