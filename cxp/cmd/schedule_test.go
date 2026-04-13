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
