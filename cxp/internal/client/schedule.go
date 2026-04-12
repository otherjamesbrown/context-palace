package client

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

const (
	ScheduleWorkflowDriftScan = "drift-scan"
	ScheduleWorkflowCanary    = "canary"
	ScheduleWorkflowTriage    = "triage"
)

var validScheduleWorkflowTypes = []string{
	ScheduleWorkflowDriftScan,
	ScheduleWorkflowCanary,
	ScheduleWorkflowTriage,
}

type Schedule struct {
	ID            int             `json:"id" yaml:"id"`
	Project       string          `json:"project" yaml:"project"`
	Name          string          `json:"name" yaml:"name"`
	WorkflowType  string          `json:"workflow_type" yaml:"workflow_type"`
	ScheduleExpr  string          `json:"schedule_expr" yaml:"schedule_expr"`
	Enabled       bool            `json:"enabled" yaml:"enabled"`
	OverlapPolicy string          `json:"overlap_policy" yaml:"overlap_policy"`
	Config        json.RawMessage `json:"config" yaml:"config"`
	LastRunAt     *time.Time      `json:"last_run_at,omitempty" yaml:"last_run_at,omitempty"`
	NextRunAt     *time.Time      `json:"next_run_at,omitempty" yaml:"next_run_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at" yaml:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" yaml:"updated_at"`
}

type ScheduleRun struct {
	ID           int             `json:"id" yaml:"id"`
	ScheduleID   int             `json:"schedule_id" yaml:"schedule_id"`
	Project      string          `json:"project" yaml:"project"`
	WorkflowType string          `json:"workflow_type" yaml:"workflow_type"`
	Status       string          `json:"status" yaml:"status"`
	StartedAt    time.Time       `json:"started_at" yaml:"started_at"`
	FinishedAt   *time.Time      `json:"finished_at,omitempty" yaml:"finished_at,omitempty"`
	Summary      *string         `json:"summary,omitempty" yaml:"summary,omitempty"`
	Result       json.RawMessage `json:"result" yaml:"result"`
	CreatedAt    time.Time       `json:"created_at" yaml:"created_at"`
}

func ValidateScheduleWorkflowType(workflowType string) error {
	if !slices.Contains(validScheduleWorkflowTypes, workflowType) {
		return fmt.Errorf("invalid workflow type %q: use drift-scan, canary, or triage", workflowType)
	}
	return nil
}

func (c *Client) ListSchedules(ctx context.Context) ([]Schedule, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT id, project, name, workflow_type, schedule_expr, enabled, overlap_policy,
		       config, last_run_at, next_run_at, created_at, updated_at
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
			&s.ID, &s.Project, &s.Name, &s.WorkflowType, &s.ScheduleExpr, &s.Enabled, &s.OverlapPolicy,
			&s.Config, &s.LastRunAt, &s.NextRunAt, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan schedule: %v", err)
		}
		schedules = append(schedules, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate schedules: %v", err)
	}

	return schedules, nil
}

func (c *Client) CreateSchedule(ctx context.Context, name, workflowType, scheduleExpr string, config json.RawMessage, nextRunAt *time.Time) (*Schedule, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	if err := ValidateScheduleWorkflowType(workflowType); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("schedule name is required")
	}
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	s := &Schedule{}
	err = tx.QueryRow(ctx, `
		INSERT INTO schedules (
			project, name, workflow_type, schedule_expr, enabled, config, next_run_at
		) VALUES ($1, $2, $3, $4, true, $5, $6)
		RETURNING id, project, name, workflow_type, schedule_expr, enabled, overlap_policy,
		          config, last_run_at, next_run_at, created_at, updated_at
	`, c.Config.Project, name, workflowType, scheduleExpr, config, nextRunAt).Scan(
		&s.ID, &s.Project, &s.Name, &s.WorkflowType, &s.ScheduleExpr, &s.Enabled, &s.OverlapPolicy,
		&s.Config, &s.LastRunAt, &s.NextRunAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create schedule: %v", err)
	}

	if _, err := tx.Exec(ctx, `SELECT pg_notify('schedules', $1)`, fmt.Sprintf("%s/%s", c.Config.Project, name)); err != nil {
		return nil, fmt.Errorf("failed to notify schedule change: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit schedule create: %v", err)
	}

	return s, nil
}

func (c *Client) SetScheduleEnabled(ctx context.Context, name string, enabled bool, nextRunAt *time.Time) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, `
		UPDATE schedules
		SET enabled = $1, next_run_at = $2, updated_at = NOW()
		WHERE project = $3 AND name = $4
	`, enabled, nextRunAt, c.Config.Project, name)
	if err != nil {
		return fmt.Errorf("failed to update schedule: %v", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("schedule not found: %s", name)
	}

	if _, err := tx.Exec(ctx, `SELECT pg_notify('schedules', $1)`, fmt.Sprintf("%s/%s", c.Config.Project, name)); err != nil {
		return fmt.Errorf("failed to notify schedule change: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit schedule update: %v", err)
	}

	return nil
}

func (c *Client) GetScheduleHistory(ctx context.Context, name string, limit int) ([]ScheduleRun, error) {
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
		return nil, fmt.Errorf("failed to get schedule history: %v", err)
	}
	defer rows.Close()

	var runs []ScheduleRun
	for rows.Next() {
		var run ScheduleRun
		if err := rows.Scan(
			&run.ID, &run.ScheduleID, &run.Project, &run.WorkflowType, &run.Status,
			&run.StartedAt, &run.FinishedAt, &run.Summary, &run.Result, &run.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan schedule run: %v", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate schedule history: %v", err)
	}

	return runs, nil
}

func (c *Client) GetLastScheduleRun(ctx context.Context, name string) (*ScheduleRun, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var run ScheduleRun
	err = conn.QueryRow(ctx, `
		SELECT sr.id, sr.schedule_id, sr.project, sr.workflow_type, sr.status,
		       sr.started_at, sr.finished_at, sr.summary, sr.result, sr.created_at
		FROM schedule_runs sr
		JOIN schedules s ON s.id = sr.schedule_id
		WHERE s.project = $1 AND s.name = $2
		ORDER BY sr.started_at DESC
		LIMIT 1
	`, c.Config.Project, name).Scan(
		&run.ID, &run.ScheduleID, &run.Project, &run.WorkflowType, &run.Status,
		&run.StartedAt, &run.FinishedAt, &run.Summary, &run.Result, &run.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get last schedule run: %v", err)
	}

	return &run, nil
}
