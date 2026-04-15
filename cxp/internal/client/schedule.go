package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/robfig/cron/v3"
)

const (
	ScheduleOverlapSkip          = "skip"
	ScheduleOverlapAllow         = "allow"
	ScheduleOverlapCancelRunning = "cancel_running"

	ScheduleRunStatusRunning   = "running"
	ScheduleRunStatusCompleted = "completed"
	ScheduleRunStatusFailed    = "failed"
)

var validScheduleOverlapPolicies = []string{
	ScheduleOverlapSkip,
	ScheduleOverlapAllow,
	ScheduleOverlapCancelRunning,
}

var validScheduleRunStatuses = []string{
	ScheduleRunStatusRunning,
	ScheduleRunStatusCompleted,
	ScheduleRunStatusFailed,
}

var standardCronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// Schedule stores one scheduled workflow subscription.
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

// ScheduleRun stores one execution attempt for a schedule.
type ScheduleRun struct {
	ID           int             `json:"id" yaml:"id"`
	ScheduleID   int             `json:"schedule_id" yaml:"schedule_id"`
	Project      string          `json:"project" yaml:"project"`
	WorkflowType string          `json:"workflow_type" yaml:"workflow_type"`
	Status       string          `json:"status" yaml:"status"`
	StartedAt    time.Time       `json:"started_at" yaml:"started_at"`
	FinishedAt   *time.Time      `json:"finished_at,omitempty" yaml:"finished_at,omitempty"`
	Summary      string          `json:"summary,omitempty" yaml:"summary,omitempty"`
	Result       json.RawMessage `json:"result" yaml:"result"`
	CreatedAt    time.Time       `json:"created_at" yaml:"created_at"`
}

// CreateScheduleInput captures the fields required to create a schedule.
type CreateScheduleInput struct {
	Project       string
	Name          string
	WorkflowType  string
	ScheduleExpr  string
	Enabled       *bool
	OverlapPolicy string
	Config        json.RawMessage
}

// CreateScheduleRunInput captures the fields required to insert a schedule run.
type CreateScheduleRunInput struct {
	ScheduleID   int
	Project      string
	WorkflowType string
	Status       string
	StartedAt    *time.Time
	Summary      string
	Result       json.RawMessage
}

// UpdateScheduleRunInput captures the fields required to complete or update a schedule run.
type UpdateScheduleRunInput struct {
	Status     string
	FinishedAt *time.Time
	Summary    string
	Result     json.RawMessage
}

// CreateSchedule inserts a schedule and computes its initial next_run_at timestamp.
func (c *Client) CreateSchedule(ctx context.Context, input CreateScheduleInput) (*Schedule, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	schedule, err := buildScheduleFromCreateInput(c, input)
	if err != nil {
		return nil, err
	}

	err = conn.QueryRow(ctx, `
		INSERT INTO schedules (
			project, name, workflow_type, schedule_expr, enabled,
			overlap_policy, config, next_run_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8
		)
		RETURNING id, created_at, updated_at
	`,
		schedule.Project, schedule.Name, schedule.WorkflowType, schedule.ScheduleExpr, schedule.Enabled,
		schedule.OverlapPolicy, schedule.Config, schedule.NextRunAt,
	).Scan(&schedule.ID, &schedule.CreatedAt, &schedule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create schedule: %v", err)
	}

	return schedule, nil
}

// ListSchedules returns schedules for one project ordered by name.
func (c *Client) ListSchedules(ctx context.Context, project string) ([]Schedule, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	project = strings.TrimSpace(project)
	if project == "" {
		project = c.Config.Project
	}
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}

	rows, err := conn.Query(ctx, `
		SELECT id, project, name, workflow_type, schedule_expr, enabled,
			overlap_policy, config, last_run_at, next_run_at, created_at, updated_at
		FROM schedules
		WHERE project = $1
		ORDER BY name ASC
	`, project)
	if err != nil {
		return nil, fmt.Errorf("failed to list schedules: %v", err)
	}
	defer rows.Close()

	var schedules []Schedule
	for rows.Next() {
		schedule, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("schedule iteration error: %v", err)
	}
	return schedules, nil
}

// GetSchedule fetches a schedule by project and name.
func (c *Client) GetSchedule(ctx context.Context, project, name string) (*Schedule, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	project = strings.TrimSpace(project)
	if project == "" {
		project = c.Config.Project
	}
	name = strings.TrimSpace(name)
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}
	if name == "" {
		return nil, fmt.Errorf("schedule name is required")
	}

	row := conn.QueryRow(ctx, `
		SELECT id, project, name, workflow_type, schedule_expr, enabled,
			overlap_policy, config, last_run_at, next_run_at, created_at, updated_at
		FROM schedules
		WHERE project = $1 AND name = $2
	`, project, name)

	schedule, err := scanSchedule(row)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("schedule not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule %s: %v", name, err)
	}
	return &schedule, nil
}

// GetScheduleIfExists fetches a schedule by project and name, returning (nil, false, nil) if not found.
func (c *Client) GetScheduleIfExists(ctx context.Context, project, name string) (*Schedule, bool, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, false, err
	}
	defer conn.Close(ctx)

	project = strings.TrimSpace(project)
	if project == "" {
		project = c.Config.Project
	}
	name = strings.TrimSpace(name)
	if project == "" {
		return nil, false, fmt.Errorf("project is required")
	}
	if name == "" {
		return nil, false, fmt.Errorf("schedule name is required")
	}

	row := conn.QueryRow(ctx, `
		SELECT id, project, name, workflow_type, schedule_expr, enabled,
			overlap_policy, config, last_run_at, next_run_at, created_at, updated_at
		FROM schedules
		WHERE project = $1 AND name = $2
	`, project, name)

	schedule, err := scanSchedule(row)
	if err == pgx.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to get schedule %s: %v", name, err)
	}
	return &schedule, true, nil
}

// EnableSchedule enables a schedule and recomputes the next run time from now.
func (c *Client) EnableSchedule(ctx context.Context, project, name string) (*Schedule, error) {
	return c.setScheduleEnabled(ctx, project, name, true)
}

// DisableSchedule disables a schedule and clears next_run_at.
func (c *Client) DisableSchedule(ctx context.Context, project, name string) (*Schedule, error) {
	return c.setScheduleEnabled(ctx, project, name, false)
}

// ListScheduleRuns returns recent runs for a schedule ordered by started_at descending.
func (c *Client) ListScheduleRuns(ctx context.Context, scheduleID, limit int) ([]ScheduleRun, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	if scheduleID <= 0 {
		return nil, fmt.Errorf("schedule ID is required")
	}
	if limit <= 0 {
		limit = 20
	}

	rows, err := conn.Query(ctx, `
		SELECT id, schedule_id, project, workflow_type, status,
			started_at, finished_at, summary, result, created_at
		FROM schedule_runs
		WHERE schedule_id = $1
		ORDER BY started_at DESC, id DESC
		LIMIT $2
	`, scheduleID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list schedule runs: %v", err)
	}
	defer rows.Close()

	var runs []ScheduleRun
	for rows.Next() {
		run, err := scanScheduleRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("schedule run iteration error: %v", err)
	}
	return runs, nil
}

// GetLastScheduleRun fetches the most recent run for a schedule.
func (c *Client) GetLastScheduleRun(ctx context.Context, scheduleID int) (*ScheduleRun, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	if scheduleID <= 0 {
		return nil, fmt.Errorf("schedule ID is required")
	}

	row := conn.QueryRow(ctx, `
		SELECT id, schedule_id, project, workflow_type, status,
			started_at, finished_at, summary, result, created_at
		FROM schedule_runs
		WHERE schedule_id = $1
		ORDER BY started_at DESC, id DESC
		LIMIT 1
	`, scheduleID)

	run, err := scanScheduleRun(row)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("no runs found for schedule %d", scheduleID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get last schedule run: %v", err)
	}
	return &run, nil
}

// CreateScheduleRun inserts a schedule_runs row.
func (c *Client) CreateScheduleRun(ctx context.Context, input CreateScheduleRunInput) (*ScheduleRun, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	run, err := buildScheduleRunFromCreateInput(input)
	if err != nil {
		return nil, err
	}

	err = conn.QueryRow(ctx, `
		INSERT INTO schedule_runs (
			schedule_id, project, workflow_type, status, started_at, summary, result
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
		RETURNING id, created_at
	`,
		run.ScheduleID, run.Project, run.WorkflowType, run.Status, run.StartedAt, nullableText(run.Summary), run.Result,
	).Scan(&run.ID, &run.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create schedule run: %v", err)
	}

	return run, nil
}

// UpdateScheduleRun updates a schedule run and rolls forward the parent schedule timestamps.
func (c *Client) UpdateScheduleRun(ctx context.Context, runID int, input UpdateScheduleRunInput) (*ScheduleRun, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	if runID <= 0 {
		return nil, fmt.Errorf("schedule run ID is required")
	}
	if input.Status == "" {
		return nil, fmt.Errorf("status is required")
	}
	if !containsString(validScheduleRunStatuses, input.Status) {
		return nil, fmt.Errorf("invalid schedule run status %q", input.Status)
	}
	if len(input.Result) == 0 {
		input.Result = json.RawMessage(`{}`)
	}

	finishedAt := time.Now().UTC()
	if input.FinishedAt != nil {
		finishedAt = input.FinishedAt.UTC()
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin schedule run update: %v", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		UPDATE schedule_runs
		SET status = $2,
			finished_at = $3,
			summary = $4,
			result = $5
		WHERE id = $1
		RETURNING id, schedule_id, project, workflow_type, status,
			started_at, finished_at, summary, result, created_at
	`, runID, input.Status, finishedAt, nullableText(input.Summary), input.Result)

	run, err := scanScheduleRun(row)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("schedule run not found: %d", runID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update schedule run: %v", err)
	}

	var scheduleExpr string
	var enabled bool
	err = tx.QueryRow(ctx, `
		SELECT schedule_expr, enabled
		FROM schedules
		WHERE id = $1
	`, run.ScheduleID).Scan(&scheduleExpr, &enabled)
	if err != nil {
		return nil, fmt.Errorf("failed to load schedule for run update: %v", err)
	}

	nextRunAt, err := computeNextRunAt(run.WorkflowType, scheduleExpr, enabled)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE schedules
		SET last_run_at = $2,
			next_run_at = $3,
			updated_at = NOW()
		WHERE id = $1
	`, run.ScheduleID, finishedAt, nextRunAt)
	if err != nil {
		return nil, fmt.Errorf("failed to update schedule timestamps: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit schedule run update: %v", err)
	}
	return &run, nil
}

// HasRunningScheduleRun reports whether a schedule currently has a running execution.
func (c *Client) HasRunningScheduleRun(ctx context.Context, scheduleID int) (bool, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close(ctx)

	var count int
	err = conn.QueryRow(ctx, `
		SELECT count(*)
		FROM schedule_runs
		WHERE schedule_id = $1 AND status = $2
	`, scheduleID, ScheduleRunStatusRunning).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check running schedule runs: %v", err)
	}
	return count > 0, nil
}

func buildScheduleFromCreateInput(c *Client, input CreateScheduleInput) (*Schedule, error) {
	project := strings.TrimSpace(input.Project)
	if project == "" {
		project = c.Config.Project
	}
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("schedule name is required")
	}

	workflowType := strings.TrimSpace(input.WorkflowType)
	if workflowType == "" {
		workflowType = name
	}

	scheduleExpr := strings.TrimSpace(input.ScheduleExpr)
	if scheduleExpr == "" {
		return nil, fmt.Errorf("cron expression is required")
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	overlapPolicy := strings.TrimSpace(input.OverlapPolicy)
	if overlapPolicy == "" {
		overlapPolicy = ScheduleOverlapSkip
	}
	if !containsString(validScheduleOverlapPolicies, overlapPolicy) {
		return nil, fmt.Errorf("invalid overlap policy %q", overlapPolicy)
	}

	config := input.Config
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	if !json.Valid(config) {
		return nil, fmt.Errorf("schedule config must be valid JSON")
	}

	nextRunAt, err := computeNextRunAt(workflowType, scheduleExpr, enabled)
	if err != nil {
		return nil, err
	}

	return &Schedule{
		Project:       project,
		Name:          name,
		WorkflowType:  workflowType,
		ScheduleExpr:  scheduleExpr,
		Enabled:       enabled,
		OverlapPolicy: overlapPolicy,
		Config:        config,
		NextRunAt:     nextRunAt,
	}, nil
}

func buildScheduleRunFromCreateInput(input CreateScheduleRunInput) (*ScheduleRun, error) {
	if input.ScheduleID <= 0 {
		return nil, fmt.Errorf("schedule ID is required")
	}
	if strings.TrimSpace(input.Project) == "" {
		return nil, fmt.Errorf("project is required")
	}
	if strings.TrimSpace(input.WorkflowType) == "" {
		return nil, fmt.Errorf("workflow type is required")
	}
	if input.Status == "" {
		input.Status = ScheduleRunStatusRunning
	}
	if !containsString(validScheduleRunStatuses, input.Status) {
		return nil, fmt.Errorf("invalid schedule run status %q", input.Status)
	}
	if len(input.Result) == 0 {
		input.Result = json.RawMessage(`{}`)
	}
	if !json.Valid(input.Result) {
		return nil, fmt.Errorf("schedule run result must be valid JSON")
	}

	startedAt := time.Now().UTC()
	if input.StartedAt != nil {
		startedAt = input.StartedAt.UTC()
	}

	return &ScheduleRun{
		ScheduleID:   input.ScheduleID,
		Project:      strings.TrimSpace(input.Project),
		WorkflowType: strings.TrimSpace(input.WorkflowType),
		Status:       input.Status,
		StartedAt:    startedAt,
		Summary:      strings.TrimSpace(input.Summary),
		Result:       input.Result,
	}, nil
}

func (c *Client) setScheduleEnabled(ctx context.Context, project, name string, enabled bool) (*Schedule, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	project = strings.TrimSpace(project)
	if project == "" {
		project = c.Config.Project
	}
	name = strings.TrimSpace(name)
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}
	if name == "" {
		return nil, fmt.Errorf("schedule name is required")
	}

	var scheduleExpr string
	var workflowType string
	err = conn.QueryRow(ctx, `
		SELECT schedule_expr, workflow_type
		FROM schedules
		WHERE project = $1 AND name = $2
	`, project, name).Scan(&scheduleExpr, &workflowType)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("schedule not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load schedule %s: %v", name, err)
	}

	nextRunAt, err := computeNextRunAt(workflowType, scheduleExpr, enabled)
	if err != nil {
		return nil, err
	}

	row := conn.QueryRow(ctx, `
		UPDATE schedules
		SET enabled = $3,
			next_run_at = $4,
			updated_at = NOW()
		WHERE project = $1 AND name = $2
		RETURNING id, project, name, workflow_type, schedule_expr, enabled,
			overlap_policy, config, last_run_at, next_run_at, created_at, updated_at
	`, project, name, enabled, nextRunAt)

	schedule, err := scanSchedule(row)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("schedule not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update schedule %s: %v", name, err)
	}
	return &schedule, nil
}

func computeNextRunAt(workflowType, expr string, enabled bool) (*time.Time, error) {
	if !enabled {
		return nil, nil
	}
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("cron expression is required for workflow %s", workflowType)
	}

	spec, err := standardCronParser.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression %q: %v", expr, err)
	}

	next := spec.Next(time.Now().UTC())
	return &next, nil
}

func scanSchedule(scanner interface {
	Scan(dest ...any) error
}) (Schedule, error) {
	var schedule Schedule
	err := scanner.Scan(
		&schedule.ID,
		&schedule.Project,
		&schedule.Name,
		&schedule.WorkflowType,
		&schedule.ScheduleExpr,
		&schedule.Enabled,
		&schedule.OverlapPolicy,
		&schedule.Config,
		&schedule.LastRunAt,
		&schedule.NextRunAt,
		&schedule.CreatedAt,
		&schedule.UpdatedAt,
	)
	if err != nil {
		return Schedule{}, err
	}
	return schedule, nil
}

func scanScheduleRun(scanner interface {
	Scan(dest ...any) error
}) (ScheduleRun, error) {
	var run ScheduleRun
	err := scanner.Scan(
		&run.ID,
		&run.ScheduleID,
		&run.Project,
		&run.WorkflowType,
		&run.Status,
		&run.StartedAt,
		&run.FinishedAt,
		&run.Summary,
		&run.Result,
		&run.CreatedAt,
	)
	if err != nil {
		return ScheduleRun{}, err
	}
	return run, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
