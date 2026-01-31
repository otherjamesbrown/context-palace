package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Task represents a context-palace task
type Task struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Status    string     `json:"status"`
	Owner     *string    `json:"owner"`
	Priority  *int       `json:"priority"`
	CreatedAt time.Time  `json:"created_at"`
	Artifacts []Artifact `json:"artifacts,omitempty"`
}

// Artifact represents a task artifact
type Artifact struct {
	Type        string `json:"type"`
	Reference   string `json:"reference"`
	Description string `json:"description"`
}

// getDB returns a database connection
func getDB(ctx context.Context) (*pgx.Conn, error) {
	conn, err := pgx.Connect(ctx, cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: check your configuration and network")
	}
	return conn, nil
}

// getTask fetches a task by ID with its artifacts
func getTask(ctx context.Context, id string) (*Task, error) {
	conn, err := getDB(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var task Task
	err = conn.QueryRow(ctx, `
		SELECT id, title, content, status, owner, priority, created_at
		FROM shards
		WHERE id = $1 AND type IN ('task', 'backlog')
	`, id).Scan(&task.ID, &task.Title, &task.Content, &task.Status, &task.Owner, &task.Priority, &task.CreatedAt)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch task")
	}

	// Fetch artifacts
	rows, err := conn.Query(ctx, `SELECT * FROM get_artifacts($1)`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var a Artifact
			var createdAt time.Time
			if err := rows.Scan(&a.Type, &a.Reference, &a.Description, &createdAt); err == nil {
				task.Artifacts = append(task.Artifacts, a)
			}
		}
	}

	return &task, nil
}

// claimTask claims a task for the current agent
func claimTask(ctx context.Context, id string) (bool, error) {
	conn, err := getDB(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close(ctx)

	var success bool
	err = conn.QueryRow(ctx, `SELECT claim_task($1, $2)`, id, cfg.Agent).Scan(&success)
	if err != nil {
		return false, fmt.Errorf("failed to claim task")
	}
	return success, nil
}

// addProgress adds a progress note to a task
func addProgress(ctx context.Context, id, note string) error {
	conn, err := getDB(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	// Append progress note to content with timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	progressNote := fmt.Sprintf("\n\n---\n**[%s] %s:** %s", timestamp, cfg.Agent, note)

	result, err := conn.Exec(ctx, `
		UPDATE shards
		SET content = content || $1, updated_at = NOW()
		WHERE id = $2 AND type IN ('task', 'backlog')
	`, progressNote, id)

	if err != nil {
		return fmt.Errorf("failed to add progress note")
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("task not found: %s", id)
	}
	return nil
}

// closeTask closes a task with a summary
func closeTaskDB(ctx context.Context, id, summary string) error {
	conn, err := getDB(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `SELECT close_task($1, $2)`, id, summary)
	if err != nil {
		return fmt.Errorf("failed to close task")
	}
	return nil
}

// addArtifact adds an artifact to a task
func addArtifactDB(ctx context.Context, id, artifactType, reference, description string) error {
	conn, err := getDB(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `SELECT add_artifact($1, $2, $3, $4)`, id, artifactType, reference, description)
	if err != nil {
		return fmt.Errorf("failed to add artifact")
	}
	return nil
}
