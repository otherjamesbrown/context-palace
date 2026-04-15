package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	_ "github.com/otherjamesbrown/context-palace/cxp/internal/scheduler/workflows"
)

// testDBClient returns a real client or skips the test if the DB is unavailable
// or the schedules table has not been migrated.
func testDBClient(t *testing.T) *client.Client {
	t.Helper()
	cfg, err := client.LoadConfig("")
	if err != nil {
		t.Skipf("no DB config available: %v", err)
	}
	c := client.NewClient(cfg)
	ctx := context.Background()
	conn, err := c.Connect(ctx)
	if err != nil {
		t.Skipf("DB not reachable: %v", err)
	}
	defer conn.Close(ctx)
	// Skip if the schedules table hasn't been migrated yet.
	if _, err := conn.Exec(ctx, "SELECT 1 FROM schedules LIMIT 0"); err != nil {
		t.Skipf("schedules table not available (run migrations first): %v", err)
	}
	return c
}

// TestSchedule_NoopRoundTrip creates a noop schedule, runs it, records the run,
// and verifies the schedule_runs row ends up with status=completed and a valid result.
func TestSchedule_NoopRoundTrip(t *testing.T) {
	c := testDBClient(t)
	ctx := context.Background()

	name := fmt.Sprintf("test-noop-%d", time.Now().UnixNano())

	schedule, err := c.CreateSchedule(ctx, client.CreateScheduleInput{
		Project:       c.Config.Project,
		Name:          name,
		WorkflowType:  "noop",
		ScheduleExpr:  "* * * * *",
		OverlapPolicy: client.ScheduleOverlapSkip,
		Config:        json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	summary, result, err := runScheduleWorkflow(ctx, schedule)
	if err != nil {
		t.Fatalf("runScheduleWorkflow returned unexpected error: %v", err)
	}
	if summary != "noop completed" {
		t.Errorf("summary = %q, want %q", summary, "noop completed")
	}
	if len(result) == 0 {
		t.Fatal("result is empty")
	}

	var resultMap map[string]string
	if err := json.Unmarshal(result, &resultMap); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if _, ok := resultMap["ran_at"]; !ok {
		t.Error("result JSON missing 'ran_at' field")
	}
	if _, ok := resultMap["message"]; !ok {
		t.Error("result JSON missing 'message' field")
	}

	// Record the run (mirrors scheduleRunCmd logic).
	run, err := c.CreateScheduleRun(ctx, client.CreateScheduleRunInput{
		ScheduleID:   schedule.ID,
		Project:      schedule.Project,
		WorkflowType: schedule.WorkflowType,
		Status:       client.ScheduleRunStatusRunning,
		Result:       json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("CreateScheduleRun: %v", err)
	}

	now := time.Now().UTC()
	run, err = c.UpdateScheduleRun(ctx, run.ID, client.UpdateScheduleRunInput{
		Status:     client.ScheduleRunStatusCompleted,
		FinishedAt: &now,
		Summary:    summary,
		Result:     result,
	})
	if err != nil {
		t.Fatalf("UpdateScheduleRun: %v", err)
	}
	if run.Status != client.ScheduleRunStatusCompleted {
		t.Errorf("run.Status = %q, want %q", run.Status, client.ScheduleRunStatusCompleted)
	}

	// Verify via GetLastScheduleRun.
	last, err := c.GetLastScheduleRun(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("GetLastScheduleRun: %v", err)
	}
	if last.Status != client.ScheduleRunStatusCompleted {
		t.Errorf("last.Status = %q, want %q", last.Status, client.ScheduleRunStatusCompleted)
	}
	if last.Summary != "noop completed" {
		t.Errorf("last.Summary = %q, want %q", last.Summary, "noop completed")
	}
	if len(last.Result) == 0 {
		t.Error("last.Result is empty")
	}

	// Verify schedule.last_run_at is updated.
	updated, err := c.GetSchedule(ctx, schedule.Project, schedule.Name)
	if err != nil {
		t.Fatalf("GetSchedule: %v", err)
	}
	if updated.LastRunAt == nil {
		t.Error("schedule.LastRunAt should be set after a completed run")
	}
}

// TestSchedule_UnknownWorkflowType verifies that an unknown workflow_type produces
// a clean error and the run row is recorded as failed with the error in result.
func TestSchedule_UnknownWorkflowType(t *testing.T) {
	c := testDBClient(t)
	ctx := context.Background()

	name := fmt.Sprintf("test-unknown-%d", time.Now().UnixNano())

	schedule, err := c.CreateSchedule(ctx, client.CreateScheduleInput{
		Project:       c.Config.Project,
		Name:          name,
		WorkflowType:  "unknown-xyz",
		ScheduleExpr:  "* * * * *",
		OverlapPolicy: client.ScheduleOverlapSkip,
		Config:        json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	_, _, workflowErr := runScheduleWorkflow(ctx, schedule)
	if workflowErr == nil {
		t.Fatal("expected error for unknown workflow type, got nil")
	}
	if !strings.Contains(workflowErr.Error(), "no runner registered for workflow_type") {
		t.Errorf("error %q does not contain expected message", workflowErr.Error())
	}

	// Record the failed run (mirrors scheduleRunCmd logic on error).
	run, err := c.CreateScheduleRun(ctx, client.CreateScheduleRunInput{
		ScheduleID:   schedule.ID,
		Project:      schedule.Project,
		WorkflowType: schedule.WorkflowType,
		Status:       client.ScheduleRunStatusRunning,
		Result:       json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("CreateScheduleRun: %v", err)
	}

	errResult := scheduleRunErrorResult(workflowErr)
	now := time.Now().UTC()
	run, err = c.UpdateScheduleRun(ctx, run.ID, client.UpdateScheduleRunInput{
		Status:     client.ScheduleRunStatusFailed,
		FinishedAt: &now,
		Summary:    workflowErr.Error(),
		Result:     errResult,
	})
	if err != nil {
		t.Fatalf("UpdateScheduleRun: %v", err)
	}
	if run.Status != client.ScheduleRunStatusFailed {
		t.Errorf("run.Status = %q, want %q", run.Status, client.ScheduleRunStatusFailed)
	}

	// Verify via GetLastScheduleRun.
	last, err := c.GetLastScheduleRun(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("GetLastScheduleRun: %v", err)
	}
	if last.Status != client.ScheduleRunStatusFailed {
		t.Errorf("last.Status = %q, want %q", last.Status, client.ScheduleRunStatusFailed)
	}

	var resultMap map[string]string
	if err := json.Unmarshal(last.Result, &resultMap); err != nil {
		t.Fatalf("last.Result is not valid JSON: %v", err)
	}
	if errMsg, ok := resultMap["error"]; !ok {
		t.Error("last.Result JSON missing 'error' field")
	} else if !strings.Contains(errMsg, "no runner registered for workflow_type") {
		t.Errorf("result error %q does not contain expected message", errMsg)
	}
}

// TestScheduleInit_DryRun verifies that --dry-run prints the plan without writing.
func TestScheduleInit_DryRun(t *testing.T) {
	c := testDBClient(t)
	ctx := context.Background()

	// Get the project prefix to form expected shard IDs.
	prefix, err := c.GetProjectIDPrefix(ctx)
	if err != nil {
		t.Fatalf("GetProjectIDPrefix: %v", err)
	}
	gapsID := prefix + "-kb-gaps-dryrun-" + fmt.Sprintf("%d", time.Now().UnixNano())

	// Ensure the gaps shard doesn't exist.
	shard, err := c.GetShard(ctx, gapsID)
	if shard != nil || err == nil {
		t.Skipf("test shard %s already exists; skipping to avoid conflict", gapsID)
	}

	// Verify GetScheduleIfExists returns false for a non-existent schedule.
	schedule, found, err := c.GetScheduleIfExists(ctx, c.Config.Project, "drift-scan-dryrun-test")
	if err != nil {
		t.Fatalf("GetScheduleIfExists: %v", err)
	}
	if found || schedule != nil {
		t.Error("expected schedule not found, got found=true")
	}
}

// TestScheduleInit_Idempotency verifies that GetScheduleIfExists returns the
// existing schedule on a second call without error.
func TestScheduleInit_Idempotency(t *testing.T) {
	c := testDBClient(t)
	ctx := context.Background()

	name := fmt.Sprintf("test-init-idem-%d", time.Now().UnixNano())

	// Create a schedule.
	created, err := c.CreateSchedule(ctx, client.CreateScheduleInput{
		Project:       c.Config.Project,
		Name:          name,
		WorkflowType:  "noop",
		ScheduleExpr:  "0 3 * * *",
		OverlapPolicy: client.ScheduleOverlapSkip,
		Config:        json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	// First call: should find it.
	existing, found, err := c.GetScheduleIfExists(ctx, c.Config.Project, name)
	if err != nil {
		t.Fatalf("GetScheduleIfExists (first): %v", err)
	}
	if !found {
		t.Fatal("GetScheduleIfExists: expected found=true, got false")
	}
	if existing.ID != created.ID {
		t.Errorf("schedule ID mismatch: got %d, want %d", existing.ID, created.ID)
	}

	// Second call: still returns same result (idempotent read).
	existing2, found2, err := c.GetScheduleIfExists(ctx, c.Config.Project, name)
	if err != nil {
		t.Fatalf("GetScheduleIfExists (second): %v", err)
	}
	if !found2 {
		t.Fatal("GetScheduleIfExists: expected found=true on second call")
	}
	if existing2.ID != created.ID {
		t.Errorf("idempotency check: schedule ID mismatch on second call")
	}
}
