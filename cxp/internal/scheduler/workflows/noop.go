package workflows

import (
	"context"
	"encoding/json"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/scheduler"
)

type NoopRunner struct{}

func (NoopRunner) Name() string { return "noop" }

func (NoopRunner) Run(ctx context.Context, config json.RawMessage) (string, json.RawMessage, error) {
	result, err := json.Marshal(map[string]any{
		"ran_at":  time.Now().UTC().Format(time.RFC3339),
		"message": "noop workflow completed successfully",
	})
	if err != nil {
		return "", nil, err
	}
	return "noop completed", result, nil
}

func init() {
	scheduler.DefaultRegistry.Register(NoopRunner{})
}
