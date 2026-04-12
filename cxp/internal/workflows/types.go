package workflows

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
)

// CommandRunner executes an external command and returns stdout.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// GapAppender records failures into a shard.
type GapAppender interface {
	AppendShardContent(ctx context.Context, id, newContent string) error
}

// Workflow is one schedule-executed unit of work.
type Workflow func(ctx context.Context, schedule client.Schedule, run client.ScheduleRun) (any, error)

// Dependencies holds shared workflow collaborators.
type Dependencies struct {
	Runner CommandRunner
	Gaps   GapAppender
	Now    func() time.Time
}

// NewDependencies builds the default dependency set for workflows.
func NewDependencies(gaps GapAppender) Dependencies {
	return Dependencies{
		Runner: execRunner{},
		Gaps:   gaps,
		Now:    time.Now,
	}
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText == "" {
			errText = strings.TrimSpace(stdout.String())
		}
		if errText == "" {
			return stdout.String(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return stdout.String(), fmt.Errorf("%s %s: %s: %w", name, strings.Join(args, " "), errText, err)
	}

	return stdout.String(), nil
}
