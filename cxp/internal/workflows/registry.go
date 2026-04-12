package workflows

import (
	"context"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
)

// Registry resolves workflow implementations by type.
type Registry struct {
	workflows map[string]Workflow
}

// NewRegistry wires the built-in workflow set.
func NewRegistry(deps Dependencies) *Registry {
	return &Registry{
		workflows: map[string]Workflow{
			"drift-scan": notImplementedWorkflow("drift-scan"),
			"canary":     NewCanaryWorkflow(deps).Run,
		},
	}
}

// Lookup returns the workflow for the given type.
func (r *Registry) Lookup(workflowType string) (Workflow, bool) {
	workflow, ok := r.workflows[canonicalWorkflowType(workflowType)]
	return workflow, ok
}

// Run resolves and executes the workflow for the schedule.
func (r *Registry) Run(ctx context.Context, schedule client.Schedule, run client.ScheduleRun) (any, error) {
	workflow, ok := r.Lookup(schedule.WorkflowType)
	if !ok {
		return nil, fmt.Errorf("unknown workflow type: %s", schedule.WorkflowType)
	}
	return workflow(ctx, schedule, run)
}

func canonicalWorkflowType(workflowType string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(workflowType), "_", "-"))
}

func notImplementedWorkflow(name string) Workflow {
	return func(ctx context.Context, schedule client.Schedule, run client.ScheduleRun) (any, error) {
		return nil, fmt.Errorf("workflow %q is not implemented", name)
	}
}
