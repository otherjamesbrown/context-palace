package scheduler

import (
	"context"
	"encoding/json"
)

// WorkflowRunner executes a single scheduled workflow invocation.
type WorkflowRunner interface {
	// Name returns the workflow_type this runner handles (e.g. "noop", "drift-scan").
	Name() string

	// Run executes the workflow with the given config.
	// Returns a human-readable summary and a structured result for schedule_runs.result.
	Run(ctx context.Context, config json.RawMessage) (summary string, result json.RawMessage, err error)
}
