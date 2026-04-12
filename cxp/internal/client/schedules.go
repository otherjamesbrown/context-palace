package client

import (
	"encoding/json"
	"time"
)

// Schedule stores one configured periodic workflow for a project.
type Schedule struct {
	ID            int             `json:"id" db:"id" yaml:"id"`
	Project       string          `json:"project" db:"project" yaml:"project"`
	Name          string          `json:"name" db:"name" yaml:"name"`
	WorkflowType  string          `json:"workflow_type" db:"workflow_type" yaml:"workflow_type"`
	ScheduleExpr  string          `json:"schedule_expr" db:"schedule_expr" yaml:"schedule_expr"`
	Enabled       bool            `json:"enabled" db:"enabled" yaml:"enabled"`
	OverlapPolicy string          `json:"overlap_policy" db:"overlap_policy" yaml:"overlap_policy"`
	Config        json.RawMessage `json:"config" db:"config" yaml:"config"`
	LastRunAt     *time.Time      `json:"last_run_at,omitempty" db:"last_run_at" yaml:"last_run_at,omitempty"`
	NextRunAt     *time.Time      `json:"next_run_at,omitempty" db:"next_run_at" yaml:"next_run_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at" yaml:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" db:"updated_at" yaml:"updated_at"`
}

// ScheduleRun stores one execution record for a schedule.
type ScheduleRun struct {
	ID           int             `json:"id" db:"id" yaml:"id"`
	ScheduleID   int             `json:"schedule_id" db:"schedule_id" yaml:"schedule_id"`
	Project      string          `json:"project" db:"project" yaml:"project"`
	WorkflowType string          `json:"workflow_type" db:"workflow_type" yaml:"workflow_type"`
	Status       string          `json:"status" db:"status" yaml:"status"`
	StartedAt    time.Time       `json:"started_at" db:"started_at" yaml:"started_at"`
	FinishedAt   *time.Time      `json:"finished_at,omitempty" db:"finished_at" yaml:"finished_at,omitempty"`
	Summary      *string         `json:"summary,omitempty" db:"summary" yaml:"summary,omitempty"`
	Result       json.RawMessage `json:"result" db:"result" yaml:"result"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at" yaml:"created_at"`
}
