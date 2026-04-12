package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

const (
	WorkflowTypeDriftScan = "drift-scan"
	WorkflowTypeCanary    = "canary"
	WorkflowTypeTriage    = "triage"
)

type Request struct {
	Project        string
	Config         json.RawMessage
	PreviousResult json.RawMessage
}

type Workflow interface {
	Name() string
	Run(ctx context.Context, req Request) (json.RawMessage, error)
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s %v: %w: %s", name, args, err, string(out))
	}
	return out, nil
}

type Registry struct {
	workflows map[string]Workflow
}

func NewRegistry(items ...Workflow) *Registry {
	r := &Registry{workflows: make(map[string]Workflow, len(items))}
	for _, item := range items {
		r.Register(item)
	}
	return r
}

func (r *Registry) Register(workflow Workflow) {
	if workflow == nil {
		return
	}
	r.workflows[workflow.Name()] = workflow
}

func (r *Registry) Get(name string) (Workflow, bool) {
	workflow, ok := r.workflows[name]
	return workflow, ok
}

type stubWorkflow struct {
	name string
}

func (w stubWorkflow) Name() string {
	return w.name
}

func (w stubWorkflow) Run(context.Context, Request) (json.RawMessage, error) {
	return nil, fmt.Errorf("%s workflow is not implemented yet", w.name)
}

func NewBuiltinRegistry(runner CommandRunner) *Registry {
	if runner == nil {
		runner = ExecRunner{}
	}

	return NewRegistry(
		stubWorkflow{name: WorkflowTypeDriftScan},
		stubWorkflow{name: WorkflowTypeCanary},
		NewTriageWorkflow(runner, time.Now),
	)
}
