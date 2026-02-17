package client

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrNoSession is returned when no open session exists (a valid state, not a real error).
var ErrNoSession = errors.New("no open session found")

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

// EnsureSessionResult contains the result of an EnsureSession call
type EnsureSessionResult struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Created bool   `json:"created"`
}

// EnsureSession returns the current open session or creates one if none exists
func (c *Client) EnsureSession(ctx context.Context) (*EnsureSessionResult, error) {
	session, err := c.GetCurrentSession(ctx)
	if err == nil {
		return &EnsureSessionResult{
			ID:      session.ID,
			Title:   session.Title,
			Created: false,
		}, nil
	}
	if !errors.Is(err, ErrNoSession) {
		return nil, err
	}

	// No open session — create one
	title := fmt.Sprintf("Session: %s", time.Now().Format("2006-01-02"))
	id, err := c.StartSession(ctx, title)
	if err != nil {
		return nil, err
	}
	return &EnsureSessionResult{
		ID:      id,
		Title:   title,
		Created: true,
	}, nil
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

// CheckpointEntry represents a parsed checkpoint from session content
type CheckpointEntry struct {
	Timestamp string `json:"timestamp"`
	Tag       string `json:"tag,omitempty"`
	Content   string `json:"content"`
}

// checkpointRe matches checkpoint headers like: ### [14:30:05] Checkpoint
// Optional tag formats: [handoff:main], [tag:deploy]
var checkpointRe = regexp.MustCompile(`### \[(\d{2}:\d{2}:\d{2})\] Checkpoint(?:\s+\[(?:handoff:|tag:)?([^\]]*)\])?`)

// LastCheckpoint returns the last checkpoint from the current session, optionally filtered by tag
func (c *Client) LastCheckpoint(ctx context.Context, tag string) (*CheckpointEntry, error) {
	session, err := c.GetCurrentSession(ctx)
	if err != nil {
		return nil, err
	}
	return ParseLastCheckpoint(session.Content, tag)
}

// ParseLastCheckpoint extracts the last checkpoint from session content text
func ParseLastCheckpoint(content, tag string) (*CheckpointEntry, error) {
	// Find all checkpoint header positions
	matches := checkpointRe.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no checkpoints found")
	}

	// Walk backwards to find the last matching checkpoint
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		timestamp := content[m[2]:m[3]]

		var cpTag string
		if m[4] >= 0 && m[5] >= 0 {
			cpTag = content[m[4]:m[5]]
		}

		// Filter by tag if specified
		if tag != "" && cpTag != tag {
			continue
		}

		// Extract content: from end of header line to next checkpoint header or end
		headerEnd := m[1]
		// Skip the newline after the header
		bodyStart := headerEnd
		if bodyStart < len(content) && content[bodyStart] == '\n' {
			bodyStart++
		}

		var body string
		if i+1 < len(matches) {
			// Content goes until the next checkpoint's ### marker
			nextStart := matches[i+1][0]
			body = strings.TrimSpace(content[bodyStart:nextStart])
		} else {
			body = strings.TrimSpace(content[bodyStart:])
		}

		return &CheckpointEntry{
			Timestamp: timestamp,
			Tag:       cpTag,
			Content:   body,
		}, nil
	}

	return nil, fmt.Errorf("no checkpoints found with tag %q", tag)
}

// InjectContext builds a formatted context payload combining session metadata,
// last checkpoint, and inbox count for Claude Code's additionalContext
func (c *Client) InjectContext(ctx context.Context, tag string) (string, error) {
	session, err := c.GetCurrentSession(ctx)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[Context Palace] Session %s (open since %s)\n",
		session.ID, session.CreatedAt.Format("2006-01-02")))

	// Last checkpoint (optional — don't fail if none)
	entry, cpErr := ParseLastCheckpoint(session.Content, tag)
	if cpErr == nil {
		if entry.Tag != "" {
			sb.WriteString(fmt.Sprintf("Last checkpoint [%s] at %s:\n", entry.Tag, entry.Timestamp))
		} else {
			sb.WriteString(fmt.Sprintf("Last checkpoint at %s:\n", entry.Timestamp))
		}
		// Indent checkpoint content
		for _, line := range strings.Split(entry.Content, "\n") {
			sb.WriteString(fmt.Sprintf("  %s\n", line))
		}
	}

	// Inbox count (optional — don't fail if unavailable)
	count, countErr := c.InboxCount(ctx)
	if countErr == nil {
		sb.WriteString(fmt.Sprintf("Inbox: %d unread messages\n", count))
	}

	return sb.String(), nil
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
	if err == pgx.ErrNoRows {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %v", err)
	}
	return &s, nil
}
