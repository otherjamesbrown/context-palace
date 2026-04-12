package workflows

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
)

// WorkflowResult is the structured output stored on schedule_runs.result.
type WorkflowResult struct {
	Summary string         `json:"summary"`
	Result  map[string]any `json:"result"`
}

// WorkflowFunc executes a named schedule workflow.
type WorkflowFunc func(ctx context.Context, cp *client.Client, project string, config json.RawMessage) (WorkflowResult, error)

var registry = map[string]WorkflowFunc{
	"drift-scan": RunDriftScan,
}

// Lookup returns a workflow implementation by workflow_type.
func Lookup(name string) (WorkflowFunc, bool) {
	fn, ok := registry[name]
	return fn, ok
}

// Run dispatches a configured workflow from the registry.
func Run(ctx context.Context, cp *client.Client, project, workflowType string, config json.RawMessage) (WorkflowResult, error) {
	fn, ok := Lookup(workflowType)
	if !ok {
		return WorkflowResult{}, fmt.Errorf("unknown workflow type: %s", workflowType)
	}
	return fn(ctx, cp, project, config)
}
