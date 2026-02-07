package client

import (
	"context"
	"fmt"
	"time"
)

// Session represents a work session
type Session struct {
	ID        string    `json:"id" yaml:"id"`
	Title     string    `json:"title" yaml:"title"`
	Content   string    `json:"content,omitempty" yaml:"content,omitempty"`
	Status    string    `json:"status" yaml:"status"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

// StartSession creates a new session shard
func (c *Client) StartSession(ctx context.Context, title string) (string, error) {
	if title == "" {
		title = fmt.Sprintf("Session: %s", time.Now().Format("2006-01-02"))
	}
	content := fmt.Sprintf("## Session started: %s\n\nAgent: %s\n",
		time.Now().Format("2006-01-02 15:04:05"), c.Config.Agent)

	return c.CreateShard(ctx, title, content, "session", nil, nil)
}

// Checkpoint appends a checkpoint to the current session
func (c *Client) Checkpoint(ctx context.Context, sessionID, note string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	timestamp := time.Now().Format("15:04:05")
	checkpoint := fmt.Sprintf("\n\n### [%s] Checkpoint\n%s", timestamp, note)

	result, err := conn.Exec(ctx, `
		UPDATE shards SET content = content || $1, updated_at = NOW()
		WHERE id = $2 AND type = 'session'
	`, checkpoint, sessionID)
	if err != nil {
		return fmt.Errorf("failed to add checkpoint: %v", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	return nil
}

// ShowSession fetches a session by ID
func (c *Client) ShowSession(ctx context.Context, sessionID string) (*Session, error) {
	shard, err := c.GetShard(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if shard.Type != "session" {
		return nil, fmt.Errorf("%s is not a session (type: %s)", sessionID, shard.Type)
	}
	return &Session{
		ID:        shard.ID,
		Title:     shard.Title,
		Content:   shard.Content,
		Status:    shard.Status,
		CreatedAt: shard.CreatedAt,
		UpdatedAt: shard.UpdatedAt,
	}, nil
}

// EndSession closes a session
func (c *Client) EndSession(ctx context.Context, sessionID string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	timestamp := time.Now().Format("15:04:05")
	ending := fmt.Sprintf("\n\n### [%s] Session ended", timestamp)

	_, err = conn.Exec(ctx, `
		UPDATE shards SET content = content || $1, status = 'closed',
			closed_at = NOW(), closed_reason = 'Session ended', updated_at = NOW()
		WHERE id = $2 AND type = 'session'
	`, ending, sessionID)
	if err != nil {
		return fmt.Errorf("failed to end session: %v", err)
	}
	return nil
}

// GetCurrentSession returns the most recent open session
func (c *Client) GetCurrentSession(ctx context.Context) (*Session, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var s Session
	err = conn.QueryRow(ctx, `
		SELECT id, title, COALESCE(content, ''), status, created_at, updated_at
		FROM shards
		WHERE project = $1 AND type = 'session' AND creator = $2 AND status = 'open'
		ORDER BY created_at DESC LIMIT 1
	`, c.Config.Project, c.Config.Agent).Scan(
		&s.ID, &s.Title, &s.Content, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("no open session found")
	}
	return &s, nil
}
