package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Schedule describes one configured scheduled workflow for a project.
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

// ScheduleRun captures one workflow execution.
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

// GetScheduleByName loads a schedule for the configured project.
func (c *Client) GetScheduleByName(ctx context.Context, name string) (*Schedule, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var s Schedule
	err = conn.QueryRow(ctx, `
		SELECT id, project, name, workflow_type, schedule_expr, enabled, overlap_policy,
			COALESCE(config, '{}'), last_run_at, next_run_at, created_at, updated_at
		FROM schedules
		WHERE project = $1 AND name = $2
	`, c.Config.Project, name).Scan(
		&s.ID, &s.Project, &s.Name, &s.WorkflowType, &s.ScheduleExpr, &s.Enabled, &s.OverlapPolicy,
		&s.Config, &s.LastRunAt, &s.NextRunAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("schedule not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load schedule %s: %v", name, err)
	}
	return &s, nil
}

// StartScheduleRun creates a running schedule_runs row.
func (c *Client) StartScheduleRun(ctx context.Context, scheduleID int, project, workflowType string) (*ScheduleRun, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var run ScheduleRun
	err = conn.QueryRow(ctx, `
		INSERT INTO schedule_runs (schedule_id, project, workflow_type, status)
		VALUES ($1, $2, $3, 'running')
		RETURNING id, schedule_id, project, workflow_type, status, started_at, finished_at,
			summary, COALESCE(result, '{}'), created_at
	`, scheduleID, project, workflowType).Scan(
		&run.ID, &run.ScheduleID, &run.Project, &run.WorkflowType, &run.Status, &run.StartedAt,
		&run.FinishedAt, &run.Summary, &run.Result, &run.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start schedule run: %v", err)
	}
	return &run, nil
}

// CompleteScheduleRun finalizes a schedule run and updates the parent schedule.
func (c *Client) CompleteScheduleRun(ctx context.Context, runID int, status, summary string, result map[string]any) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal schedule run result: %v", err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin schedule completion tx: %v", err)
	}
	defer tx.Rollback(ctx)

	var scheduleID int
	_, err = tx.Exec(ctx, `
		UPDATE schedule_runs
		SET status = $2, finished_at = NOW(), summary = $3, result = $4
		WHERE id = $1
	`, runID, status, summary, resultJSON)
	if err != nil {
		return fmt.Errorf("update schedule run: %v", err)
	}

	err = tx.QueryRow(ctx, `SELECT schedule_id FROM schedule_runs WHERE id = $1`, runID).Scan(&scheduleID)
	if err != nil {
		return fmt.Errorf("fetch schedule id for run %d: %v", runID, err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE schedules
		SET last_run_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, scheduleID)
	if err != nil {
		return fmt.Errorf("update schedule timestamps: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit schedule completion tx: %v", err)
	}
	return nil
}

// ListRecentScheduleRunsByWorkflow returns recent runs for the configured project/workflow.
func (c *Client) ListRecentScheduleRunsByWorkflow(ctx context.Context, workflowType string, limit int) ([]ScheduleRun, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT id, schedule_id, project, workflow_type, status, started_at, finished_at,
			summary, COALESCE(result, '{}'), created_at
		FROM schedule_runs
		WHERE project = $1 AND workflow_type = $2
		ORDER BY started_at DESC
		LIMIT $3
	`, c.Config.Project, workflowType, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list schedule runs: %v", err)
	}
	defer rows.Close()

	var runs []ScheduleRun
	for rows.Next() {
		var run ScheduleRun
		if err := rows.Scan(
			&run.ID, &run.ScheduleID, &run.Project, &run.WorkflowType, &run.Status, &run.StartedAt,
			&run.FinishedAt, &run.Summary, &run.Result, &run.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan schedule run: %v", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schedule runs: %v", err)
	}
	return runs, nil
}
