package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ValidWorkflowTypes is the canonical list of supported workflow types.
var ValidWorkflowTypes = []string{"drift-scan", "canary", "triage"}

// Schedule represents a row in the schedules table.
type Schedule struct {
	ID            int             `json:"id"`
	Project       string          `json:"project"`
	Name          string          `json:"name"`
	WorkflowType  string          `json:"workflow_type"`
	ScheduleExpr  string          `json:"schedule_expr"`
	Enabled       bool            `json:"enabled"`
	OverlapPolicy string          `json:"overlap_policy"`
	Config        json.RawMessage `json:"config"`
	LastRunAt     *time.Time      `json:"last_run_at,omitempty"`
	NextRunAt     *time.Time      `json:"next_run_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// ScheduleRun represents a row in the schedule_runs table.
type ScheduleRun struct {
	ID           int             `json:"id"`
	ScheduleID   int             `json:"schedule_id"`
	Project      string          `json:"project"`
	WorkflowType string          `json:"workflow_type"`
	Status       string          `json:"status"`
	StartedAt    time.Time       `json:"started_at"`
	FinishedAt   *time.Time      `json:"finished_at,omitempty"`
	Summary      *string         `json:"summary,omitempty"`
	Result       json.RawMessage `json:"result"`
	CreatedAt    time.Time       `json:"created_at"`
}

// ValidateWorkflowType checks if a workflow_type is valid.
func ValidateWorkflowType(wt string) error {
	for _, v := range ValidWorkflowTypes {
		if wt == v {
			return nil
		}
	}
	return fmt.Errorf("invalid workflow_type %q. Valid types: %s", wt, strings.Join(ValidWorkflowTypes, ", "))
}

// ListSchedules returns all schedules for the current project.
func (c *Client) ListSchedules(ctx context.Context) ([]Schedule, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT id, project, name, workflow_type, schedule_expr, enabled,
		       overlap_policy, config, last_run_at, next_run_at, created_at, updated_at
		FROM schedules
		WHERE project = $1
		ORDER BY name
	`, c.Config.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to list schedules: %v", err)
	}
	defer rows.Close()

	var schedules []Schedule
	for rows.Next() {
		var s Schedule
		if err := rows.Scan(
			&s.ID, &s.Project, &s.Name, &s.WorkflowType, &s.ScheduleExpr,
			&s.Enabled, &s.OverlapPolicy, &s.Config, &s.LastRunAt, &s.NextRunAt,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan schedule: %v", err)
		}
		schedules = append(schedules, s)
	}
	return schedules, rows.Err()
}

// CreateSchedule inserts a new schedule for the current project.
func (c *Client) CreateSchedule(ctx context.Context, name, workflowType, scheduleExpr, configJSON string) (*Schedule, error) {
	if configJSON == "" {
		configJSON = "{}"
	}

	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var s Schedule
	err = conn.QueryRow(ctx, `
		INSERT INTO schedules (project, name, workflow_type, schedule_expr, config)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		RETURNING id, project, name, workflow_type, schedule_expr, enabled,
		          overlap_policy, config, last_run_at, next_run_at, created_at, updated_at
	`, c.Config.Project, name, workflowType, scheduleExpr, configJSON).Scan(
		&s.ID, &s.Project, &s.Name, &s.WorkflowType, &s.ScheduleExpr,
		&s.Enabled, &s.OverlapPolicy, &s.Config, &s.LastRunAt, &s.NextRunAt,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create schedule: %v", err)
	}
	return &s, nil
}

// SetScheduleEnabled enables or disables a schedule by name.
func (c *Client) SetScheduleEnabled(ctx context.Context, name string, enabled bool) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	tag, err := conn.Exec(ctx, `
		UPDATE schedules
		SET enabled = $1, updated_at = now()
		WHERE project = $2 AND name = $3
	`, enabled, c.Config.Project, name)
	if err != nil {
		return fmt.Errorf("failed to update schedule: %v", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("schedule %q not found in project %q", name, c.Config.Project)
	}
	return nil
}

// ScheduleHistory returns recent schedule_runs for the named schedule.
func (c *Client) ScheduleHistory(ctx context.Context, name string, limit int) ([]ScheduleRun, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT sr.id, sr.schedule_id, sr.project, sr.workflow_type, sr.status,
		       sr.started_at, sr.finished_at, sr.summary, sr.result, sr.created_at
		FROM schedule_runs sr
		JOIN schedules s ON s.id = sr.schedule_id
		WHERE s.project = $1 AND s.name = $2
		ORDER BY sr.started_at DESC
		LIMIT $3
	`, c.Config.Project, name, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query schedule history: %v", err)
	}
	defer rows.Close()

	var runs []ScheduleRun
	for rows.Next() {
		var r ScheduleRun
		if err := rows.Scan(
			&r.ID, &r.ScheduleID, &r.Project, &r.WorkflowType, &r.Status,
			&r.StartedAt, &r.FinishedAt, &r.Summary, &r.Result, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan run: %v", err)
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// ScheduleLastRun returns the most recent run for the named schedule.
func (c *Client) ScheduleLastRun(ctx context.Context, name string) (*ScheduleRun, error) {
	runs, err := c.ScheduleHistory(ctx, name, 1)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, fmt.Errorf("no runs found for schedule %q", name)
	}
	return &runs[0], nil
}

// getScheduleByName fetches a schedule by project+name.
func (c *Client) getScheduleByName(ctx context.Context, name string) (*Schedule, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var s Schedule
	err = conn.QueryRow(ctx, `
		SELECT id, project, name, workflow_type, schedule_expr, enabled,
		       overlap_policy, config, last_run_at, next_run_at, created_at, updated_at
		FROM schedules
		WHERE project = $1 AND name = $2
	`, c.Config.Project, name).Scan(
		&s.ID, &s.Project, &s.Name, &s.WorkflowType, &s.ScheduleExpr,
		&s.Enabled, &s.OverlapPolicy, &s.Config, &s.LastRunAt, &s.NextRunAt,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("schedule %q not found in project %q", name, c.Config.Project)
	}
	return &s, nil
}

// RunSchedule executes a workflow synchronously and records the run.
// Workflow implementations are stubs at this stage — they return an empty result.
func (c *Client) RunSchedule(ctx context.Context, name string) (*ScheduleRun, error) {
	sched, err := c.getScheduleByName(ctx, name)
	if err != nil {
		return nil, err
	}

	// Step 1: insert run row with status=running
	runID, err := c.insertScheduleRun(ctx, sched)
	if err != nil {
		return nil, err
	}

	// Step 2: execute workflow stub
	result, summary, runErr := executeWorkflowStub(sched.WorkflowType, sched.Config)

	// Step 3: update run row
	status := "completed"
	if runErr != nil {
		status = "failed"
		summary = runErr.Error()
	}

	run, err := c.finalizeScheduleRun(ctx, runID, sched.ID, status, summary, result)
	if err != nil {
		return nil, err
	}

	if runErr != nil {
		return run, fmt.Errorf("workflow failed: %v", runErr)
	}
	return run, nil
}

// insertScheduleRun creates a schedule_runs row with status=running.
func (c *Client) insertScheduleRun(ctx context.Context, sched *Schedule) (int, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close(ctx)

	var runID int
	err = conn.QueryRow(ctx, `
		INSERT INTO schedule_runs (schedule_id, project, workflow_type, status)
		VALUES ($1, $2, $3, 'running')
		RETURNING id
	`, sched.ID, sched.Project, sched.WorkflowType).Scan(&runID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert schedule run: %v", err)
	}
	return runID, nil
}

// finalizeScheduleRun updates the run row and the schedule's last/next run timestamps.
func (c *Client) finalizeScheduleRun(ctx context.Context, runID, schedID int, status, summary string, result json.RawMessage) (*ScheduleRun, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	if result == nil {
		result = json.RawMessage("{}")
	}

	var summaryPtr *string
	if summary != "" {
		summaryPtr = &summary
	}

	var run ScheduleRun
	err = conn.QueryRow(ctx, `
		UPDATE schedule_runs
		SET status = $1, finished_at = now(), summary = $2, result = $3::jsonb
		WHERE id = $4
		RETURNING id, schedule_id, project, workflow_type, status,
		          started_at, finished_at, summary, result, created_at
	`, status, summaryPtr, string(result), runID).Scan(
		&run.ID, &run.ScheduleID, &run.Project, &run.WorkflowType, &run.Status,
		&run.StartedAt, &run.FinishedAt, &run.Summary, &run.Result, &run.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to finalize schedule run: %v", err)
	}

	// Update last_run_at on the schedule (best-effort)
	_, _ = conn.Exec(ctx, `
		UPDATE schedules
		SET last_run_at = now(), updated_at = now()
		WHERE id = $1
	`, schedID)

	return &run, nil
}

// executeWorkflowStub is a stub for workflow execution.
// Real implementations (drift-scan, canary, triage) will be added in the next wave.
func executeWorkflowStub(workflowType string, config json.RawMessage) (json.RawMessage, string, error) {
	switch workflowType {
	case "drift-scan":
		result := json.RawMessage(`{"articles_scanned":0,"claims_checked":0,"claims_broken":0,"semantic_checks":0,"semantic_issues":0,"gaps_logged":0}`)
		return result, "drift-scan stub: no-op", nil
	case "canary":
		result := json.RawMessage(`{"questions_tested":0,"passed":0,"failed":0,"failures":[]}`)
		return result, "canary stub: no-op", nil
	case "triage":
		result := json.RawMessage(`{"gaps_reviewed":0,"tasks_created":0,"escalations":0,"report_shard":""}`)
		return result, "triage stub: no-op", nil
	default:
		return nil, "", fmt.Errorf("unknown workflow type: %s", workflowType)
	}
}
