package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

// Launch artifact types for task runs.
const (
	LaunchArtifactNone       = "none"
	LaunchArtifactLauncher   = "launcher"
	LaunchArtifactLaunchPack = "launch_pack"
	LaunchArtifactManual     = "manual"
)

// Observation types for the first task-run evidence loop.
const (
	ObservationCorrectSubsystemReached = "correct_subsystem_reached"
	ObservationExecutableProofFound    = "executable_proof_surface_found"
	ObservationDeadEnd                 = "dead_end"
	ObservationPackMissingReference    = "pack_missing_reference"
)

// TaskRun stores one agent run against a task or shard.
type TaskRun struct {
	ID                 string     `json:"id" yaml:"id"`
	Project            string     `json:"project" yaml:"project"`
	TaskID             string     `json:"task_id,omitempty" yaml:"task_id,omitempty"`
	ShardID            string     `json:"shard_id,omitempty" yaml:"shard_id,omitempty"`
	Branch             string     `json:"branch,omitempty" yaml:"branch,omitempty"`
	AgentRuntime       string     `json:"agent_runtime" yaml:"agent_runtime"`
	AgentName          string     `json:"agent_name,omitempty" yaml:"agent_name,omitempty"`
	LaunchArtifactType string     `json:"launch_artifact_type" yaml:"launch_artifact_type"`
	LaunchArtifactRef  string     `json:"launch_artifact_ref,omitempty" yaml:"launch_artifact_ref,omitempty"`
	Status             string     `json:"status" yaml:"status"`
	StartedAt          time.Time  `json:"started_at" yaml:"started_at"`
	FinishedAt         *time.Time `json:"finished_at,omitempty" yaml:"finished_at,omitempty"`
	Summary            string     `json:"summary,omitempty" yaml:"summary,omitempty"`
	CreatedAt          time.Time  `json:"created_at" yaml:"created_at"`
}

// StartTaskRunInput captures the metadata needed to create a task run.
type StartTaskRunInput struct {
	Project            string
	TaskID             string
	ShardID            string
	Branch             string
	AgentRuntime       string
	AgentName          string
	LaunchArtifactType string
	LaunchArtifactRef  string
	Status             string
	StartedAt          *time.Time
	Summary            string
}

// TaskRunObservation stores one high-signal runtime observation.
type TaskRunObservation struct {
	ID              string          `json:"id" yaml:"id"`
	TaskRunID       string          `json:"task_run_id" yaml:"task_run_id"`
	ObservedAt      time.Time       `json:"observed_at" yaml:"observed_at"`
	ObservationType string          `json:"observation_type" yaml:"observation_type"`
	SubjectType     string          `json:"subject_type" yaml:"subject_type"`
	SubjectRef      string          `json:"subject_ref" yaml:"subject_ref"`
	Role            string          `json:"role,omitempty" yaml:"role,omitempty"`
	Confidence      *float64        `json:"confidence,omitempty" yaml:"confidence,omitempty"`
	Source          string          `json:"source" yaml:"source"`
	Details         json.RawMessage `json:"details,omitempty" yaml:"details,omitempty"`
}

// RecordObservationInput captures one observation to append to a run.
type RecordObservationInput struct {
	TaskRunID       string
	ObservedAt      *time.Time
	ObservationType string
	SubjectType     string
	SubjectRef      string
	Role            string
	Confidence      *float64
	Source          string
	Details         json.RawMessage
}

// CompleteTaskRunInput captures run-completion fields.
type CompleteTaskRunInput struct {
	Status     string
	FinishedAt *time.Time
	Summary    string
}

var validLaunchArtifactTypes = []string{
	LaunchArtifactNone,
	LaunchArtifactLauncher,
	LaunchArtifactLaunchPack,
	LaunchArtifactManual,
}

var validTaskRunStatuses = []string{"started", "completed", "failed", "cancelled"}

var validObservationTypes = []string{
	ObservationCorrectSubsystemReached,
	ObservationExecutableProofFound,
	ObservationDeadEnd,
	ObservationPackMissingReference,
}

var validObservationSubjectTypes = []string{
	"file", "test", "shard", "warning", "service", "command", "migration",
	"package", "module", "subsystem", "repro",
}

// StartTaskRun creates a new task_run record and returns its generated ID.
func (c *Client) StartTaskRun(ctx context.Context, input StartTaskRunInput) (*TaskRun, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	if input.Project == "" {
		input.Project = c.Config.Project
	}
	if input.AgentName == "" {
		input.AgentName = c.Config.Agent
	}
	if input.AgentRuntime == "" {
		return nil, fmt.Errorf("agent runtime is required")
	}
	if input.LaunchArtifactType == "" {
		input.LaunchArtifactType = LaunchArtifactNone
	}
	if !slices.Contains(validLaunchArtifactTypes, input.LaunchArtifactType) {
		return nil, fmt.Errorf("invalid launch artifact type %q", input.LaunchArtifactType)
	}
	if input.Status == "" {
		input.Status = "started"
	}
	if !slices.Contains(validTaskRunStatuses, input.Status) {
		return nil, fmt.Errorf("invalid task run status %q", input.Status)
	}

	startedAt := time.Now().UTC()
	if input.StartedAt != nil {
		startedAt = input.StartedAt.UTC()
	}

	run := &TaskRun{
		ID:                 newEvidenceID("tr"),
		Project:            input.Project,
		TaskID:             input.TaskID,
		ShardID:            input.ShardID,
		Branch:             input.Branch,
		AgentRuntime:       input.AgentRuntime,
		AgentName:          input.AgentName,
		LaunchArtifactType: input.LaunchArtifactType,
		LaunchArtifactRef:  input.LaunchArtifactRef,
		Status:             input.Status,
		StartedAt:          startedAt,
		Summary:            input.Summary,
	}

	err = conn.QueryRow(ctx, `
        INSERT INTO task_runs (
            id, project, task_id, shard_id, branch, agent_runtime, agent_name,
            launch_artifact_type, launch_artifact_ref, status, started_at, summary
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7,
            $8, $9, $10, $11, $12
        )
        RETURNING created_at
    `,
		run.ID, run.Project, nullableText(run.TaskID), nullableText(run.ShardID), nullableText(run.Branch),
		run.AgentRuntime, nullableText(run.AgentName), run.LaunchArtifactType, nullableText(run.LaunchArtifactRef),
		run.Status, run.StartedAt, nullableText(run.Summary),
	).Scan(&run.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to start task run: %v", err)
	}

	return run, nil
}

// RecordTaskObservation appends one high-signal observation to a task run.
func (c *Client) RecordTaskObservation(ctx context.Context, input RecordObservationInput) (*TaskRunObservation, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	if input.TaskRunID == "" {
		return nil, fmt.Errorf("task run ID is required")
	}
	if input.ObservationType == "" {
		return nil, fmt.Errorf("observation type is required")
	}
	if !slices.Contains(validObservationTypes, input.ObservationType) {
		return nil, fmt.Errorf("invalid observation type %q", input.ObservationType)
	}
	if input.SubjectType == "" {
		return nil, fmt.Errorf("subject type is required")
	}
	if !slices.Contains(validObservationSubjectTypes, input.SubjectType) {
		return nil, fmt.Errorf("invalid subject type %q", input.SubjectType)
	}
	if input.SubjectRef == "" {
		return nil, fmt.Errorf("subject ref is required")
	}
	if input.Source == "" {
		input.Source = "cxp"
	}
	if len(input.Details) == 0 {
		input.Details = json.RawMessage(`{}`)
	}
	if input.Confidence != nil && (*input.Confidence < 0 || *input.Confidence > 1) {
		return nil, fmt.Errorf("confidence must be between 0 and 1")
	}

	observedAt := time.Now().UTC()
	if input.ObservedAt != nil {
		observedAt = input.ObservedAt.UTC()
	}

	obs := &TaskRunObservation{
		ID:              newEvidenceID("tro"),
		TaskRunID:       input.TaskRunID,
		ObservedAt:      observedAt,
		ObservationType: input.ObservationType,
		SubjectType:     input.SubjectType,
		SubjectRef:      input.SubjectRef,
		Role:            input.Role,
		Confidence:      input.Confidence,
		Source:          input.Source,
		Details:         input.Details,
	}

	_, err = conn.Exec(ctx, `
        INSERT INTO task_run_observations (
            id, task_run_id, observed_at, observation_type, subject_type,
            subject_ref, role, confidence, source, details
        ) VALUES (
            $1, $2, $3, $4, $5,
            $6, $7, $8, $9, $10
        )
    `,
		obs.ID, obs.TaskRunID, obs.ObservedAt, obs.ObservationType, obs.SubjectType,
		obs.SubjectRef, nullableText(obs.Role), obs.Confidence, obs.Source, obs.Details,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to record task observation: %v", err)
	}

	return obs, nil
}

// CompleteTaskRun marks a run as completed or otherwise terminal.
func (c *Client) CompleteTaskRun(ctx context.Context, runID string, input CompleteTaskRunInput) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	if runID == "" {
		return fmt.Errorf("task run ID is required")
	}
	if input.Status == "" {
		input.Status = "completed"
	}
	if !slices.Contains(validTaskRunStatuses, input.Status) {
		return fmt.Errorf("invalid task run status %q", input.Status)
	}

	finishedAt := time.Now().UTC()
	if input.FinishedAt != nil {
		finishedAt = input.FinishedAt.UTC()
	}

	result, err := conn.Exec(ctx, `
        UPDATE task_runs
        SET status = $2, finished_at = $3, summary = $4
        WHERE id = $1
    `, runID, input.Status, finishedAt, nullableText(input.Summary))
	if err != nil {
		return fmt.Errorf("failed to complete task run: %v", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("task run not found: %s", runID)
	}
	return nil
}

// GetTaskRun fetches a single task run by ID.
func (c *Client) GetTaskRun(ctx context.Context, runID string) (*TaskRun, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var run TaskRun
	err = conn.QueryRow(ctx, `
		SELECT id, project, task_id, shard_id, branch, agent_runtime, agent_name,
			launch_artifact_type, launch_artifact_ref, status, started_at, finished_at, summary, created_at
		FROM task_runs
		WHERE id = $1
	`, runID).Scan(
		&run.ID, &run.Project, &run.TaskID, &run.ShardID, &run.Branch, &run.AgentRuntime, &run.AgentName,
		&run.LaunchArtifactType, &run.LaunchArtifactRef, &run.Status, &run.StartedAt, &run.FinishedAt, &run.Summary, &run.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get task run %s: %v", runID, err)
	}
	return &run, nil
}

// ListTaskRunsForShard returns recent task runs linked to a shard.
func (c *Client) ListTaskRunsForShard(ctx context.Context, shardID string, limit int) ([]TaskRun, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	if shardID == "" {
		return nil, fmt.Errorf("shard ID is required")
	}
	if limit <= 0 {
		limit = 20
	}

	rows, err := conn.Query(ctx, `
		SELECT id, project, task_id, shard_id, branch, agent_runtime, agent_name,
			launch_artifact_type, launch_artifact_ref, status, started_at, finished_at, summary, created_at
		FROM task_runs
		WHERE shard_id = $1
		ORDER BY started_at DESC
		LIMIT $2
	`, shardID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list task runs for shard %s: %v", shardID, err)
	}
	defer rows.Close()

	var runs []TaskRun
	for rows.Next() {
		var run TaskRun
		if err := rows.Scan(
			&run.ID, &run.Project, &run.TaskID, &run.ShardID, &run.Branch, &run.AgentRuntime, &run.AgentName,
			&run.LaunchArtifactType, &run.LaunchArtifactRef, &run.Status, &run.StartedAt, &run.FinishedAt, &run.Summary, &run.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan task run: %v", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("task run iteration error: %v", err)
	}
	return runs, nil
}

// ListTaskRunObservations returns observations for one run in chronological order.
func (c *Client) ListTaskRunObservations(ctx context.Context, runID string) ([]TaskRunObservation, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	if runID == "" {
		return nil, fmt.Errorf("task run ID is required")
	}

	rows, err := conn.Query(ctx, `
		SELECT id, task_run_id, observed_at, observation_type, subject_type,
			subject_ref, role, confidence, source, details
		FROM task_run_observations
		WHERE task_run_id = $1
		ORDER BY observed_at ASC, id ASC
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to list observations for run %s: %v", runID, err)
	}
	defer rows.Close()

	var observations []TaskRunObservation
	for rows.Next() {
		var obs TaskRunObservation
		if err := rows.Scan(
			&obs.ID, &obs.TaskRunID, &obs.ObservedAt, &obs.ObservationType, &obs.SubjectType,
			&obs.SubjectRef, &obs.Role, &obs.Confidence, &obs.Source, &obs.Details,
		); err != nil {
			return nil, fmt.Errorf("failed to scan task observation: %v", err)
		}
		observations = append(observations, obs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("task observation iteration error: %v", err)
	}
	return observations, nil
}

func newEvidenceID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
	}
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UTC().UnixMilli(), hex.EncodeToString(buf))
}

func nullableText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
