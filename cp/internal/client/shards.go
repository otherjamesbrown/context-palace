package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Shard represents a Context Palace shard
type Shard struct {
	ID        string     `json:"id" yaml:"id"`
	Project   string     `json:"project" yaml:"project"`
	Title     string     `json:"title" yaml:"title"`
	Content   string     `json:"content,omitempty" yaml:"content,omitempty"`
	Type      string     `json:"type" yaml:"type"`
	Status    string     `json:"status" yaml:"status"`
	Priority  *int       `json:"priority,omitempty" yaml:"priority,omitempty"`
	Creator   string     `json:"creator" yaml:"creator"`
	Owner     *string          `json:"owner,omitempty" yaml:"owner,omitempty"`
	CreatedAt time.Time        `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time        `json:"updated_at" yaml:"updated_at"`
	Metadata  json.RawMessage  `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Labels    []string         `json:"labels,omitempty" yaml:"labels,omitempty"`
	Artifacts []Artifact `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
}

// Artifact represents a task artifact
type Artifact struct {
	Type        string `json:"type" yaml:"type"`
	Reference   string `json:"reference" yaml:"reference"`
	Description string `json:"description" yaml:"description"`
}

// ShardCounts holds shard count statistics
type ShardCounts struct {
	Total  int `json:"total" yaml:"total"`
	Open   int `json:"open" yaml:"open"`
	Closed int `json:"closed" yaml:"closed"`
	Other  int `json:"other" yaml:"other"`
}

// GetShardCounts returns shard count statistics for a project
func (c *Client) GetShardCounts(ctx context.Context) (*ShardCounts, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	counts := &ShardCounts{}
	err = conn.QueryRow(ctx, `
		SELECT
			count(*),
			count(*) FILTER (WHERE status = 'open' OR status = 'in_progress'),
			count(*) FILTER (WHERE status = 'closed'),
			count(*) FILTER (WHERE status NOT IN ('open', 'in_progress', 'closed'))
		FROM shards WHERE project = $1
	`, c.Config.Project).Scan(&counts.Total, &counts.Open, &counts.Closed, &counts.Other)
	if err != nil {
		return nil, fmt.Errorf("failed to query shard counts: %v", err)
	}
	return counts, nil
}

// GetShard fetches a shard by ID
func (c *Client) GetShard(ctx context.Context, id string) (*Shard, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var s Shard
	err = conn.QueryRow(ctx, `
		SELECT id, project, title, COALESCE(content, ''), type, status,
			priority, creator, owner, created_at, updated_at,
			COALESCE(metadata, '{}')
		FROM shards WHERE id = $1
	`, id).Scan(&s.ID, &s.Project, &s.Title, &s.Content, &s.Type, &s.Status,
		&s.Priority, &s.Creator, &s.Owner, &s.CreatedAt, &s.UpdatedAt,
		&s.Metadata)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("shard not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch shard: %v", err)
	}

	// Fetch labels
	rows, err := conn.Query(ctx, `SELECT label FROM labels WHERE shard_id = $1 ORDER BY label`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var label string
			if rows.Scan(&label) == nil {
				s.Labels = append(s.Labels, label)
			}
		}
	}

	return &s, nil
}

// GetTask fetches a task by ID with its artifacts
func (c *Client) GetTask(ctx context.Context, id string) (*Shard, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var s Shard
	err = conn.QueryRow(ctx, `
		SELECT id, project, title, COALESCE(content, ''), type, status,
			priority, creator, owner, created_at, updated_at
		FROM shards WHERE id = $1 AND type IN ('task', 'backlog')
	`, id).Scan(&s.ID, &s.Project, &s.Title, &s.Content, &s.Type, &s.Status,
		&s.Priority, &s.Creator, &s.Owner, &s.CreatedAt, &s.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch task: %v", err)
	}

	// Fetch artifacts
	rows, err := conn.Query(ctx, `SELECT * FROM get_artifacts($1)`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var a Artifact
			var createdAt time.Time
			if rows.Scan(&a.Type, &a.Reference, &a.Description, &createdAt) == nil {
				s.Artifacts = append(s.Artifacts, a)
			}
		}
	}

	return &s, nil
}

// ClaimTask claims a task for the configured agent
func (c *Client) ClaimTask(ctx context.Context, id string) (bool, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close(ctx)

	var success bool
	err = conn.QueryRow(ctx, `SELECT claim_task($1, $2)`, id, c.Config.Agent).Scan(&success)
	if err != nil {
		return false, fmt.Errorf("failed to claim task: %v", err)
	}
	return success, nil
}

// AddProgress adds a progress note to a task
func (c *Client) AddProgress(ctx context.Context, id, note string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	progressNote := fmt.Sprintf("\n\n---\n**[%s] %s:** %s", timestamp, c.Config.Agent, note)

	result, err := conn.Exec(ctx, `
		UPDATE shards SET content = content || $1, updated_at = NOW()
		WHERE id = $2 AND type IN ('task', 'backlog')
	`, progressNote, id)

	if err != nil {
		return fmt.Errorf("failed to add progress note: %v", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("task not found: %s", id)
	}
	return nil
}

// CloseTask closes a task with a summary
func (c *Client) CloseTask(ctx context.Context, id, summary string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `SELECT close_task($1, $2)`, id, summary)
	if err != nil {
		return fmt.Errorf("failed to close task: %v", err)
	}
	return nil
}

// AddArtifact adds an artifact to a task
func (c *Client) AddArtifact(ctx context.Context, id, artifactType, reference, description string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `SELECT add_artifact($1, $2, $3, $4)`, id, artifactType, reference, description)
	if err != nil {
		return fmt.Errorf("failed to add artifact: %v", err)
	}
	return nil
}

// CreateShard creates a new shard and returns its ID
func (c *Client) CreateShard(ctx context.Context, title, content, shardType string, priority *int, labels []string) (string, error) {
	return c.CreateShardWithMetadata(ctx, title, content, shardType, priority, labels, nil)
}

// CreateShardWithMetadata creates a new shard with metadata and returns its ID
func (c *Client) CreateShardWithMetadata(ctx context.Context, title, content, shardType string, priority *int, labels []string, metadata json.RawMessage) (string, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close(ctx)

	if labels == nil {
		labels = []string{}
	}
	if metadata == nil {
		metadata = json.RawMessage("{}")
	}

	var newID string
	err = conn.QueryRow(ctx, `
		SELECT create_shard($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, c.Config.Project, c.Config.Agent, title, content, shardType,
		labels, nil, priority, metadata).Scan(&newID)
	if err != nil {
		return "", fmt.Errorf("failed to create shard: %v", err)
	}

	return newID, nil
}

// UpdateShardContent updates a shard's content
func (c *Client) UpdateShardContent(ctx context.Context, id, content string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	result, err := conn.Exec(ctx, `
		UPDATE shards SET content = $1, updated_at = NOW() WHERE id = $2
	`, content, id)
	if err != nil {
		return fmt.Errorf("failed to update shard: %v", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("shard not found: %s", id)
	}
	return nil
}

// UpdateShardStatus updates a shard's status
func (c *Client) UpdateShardStatus(ctx context.Context, id, status string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	result, err := conn.Exec(ctx, `
		UPDATE shards SET status = $1, updated_at = NOW() WHERE id = $2
	`, status, id)
	if err != nil {
		return fmt.Errorf("failed to update shard status: %v", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("shard not found: %s", id)
	}
	return nil
}

// ListShardsByType lists shards of a given type for the configured project
func (c *Client) ListShardsByType(ctx context.Context, shardType string, statusFilter string, limit int) ([]Shard, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	query := `
		SELECT id, project, title, COALESCE(LEFT(content, 200), ''), type, status,
			priority, creator, owner, created_at, updated_at,
			COALESCE(metadata, '{}')
		FROM shards WHERE project = $1 AND type = $2
	`
	args := []interface{}{c.Config.Project, shardType}

	if statusFilter != "" {
		query += ` AND status = $3`
		args = append(args, statusFilter)
		query += ` ORDER BY priority NULLS LAST, created_at DESC LIMIT $4`
		args = append(args, limit)
	} else {
		query += ` ORDER BY priority NULLS LAST, created_at DESC LIMIT $3`
		args = append(args, limit)
	}

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list shards: %v", err)
	}
	defer rows.Close()

	var shards []Shard
	for rows.Next() {
		var s Shard
		if err := rows.Scan(&s.ID, &s.Project, &s.Title, &s.Content, &s.Type, &s.Status,
			&s.Priority, &s.Creator, &s.Owner, &s.CreatedAt, &s.UpdatedAt,
			&s.Metadata); err != nil {
			continue
		}
		shards = append(shards, s)
	}
	return shards, nil
}

// SearchShards does full-text search across shards
func (c *Client) SearchShards(ctx context.Context, query string, shardType string, limit int) ([]Shard, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	sqlQuery := `
		SELECT id, project, title, COALESCE(LEFT(content, 200), ''), type, status,
			priority, creator, owner, created_at, updated_at,
			COALESCE(metadata, '{}'),
			ts_rank(search_vector, plainto_tsquery($2)) AS rank
		FROM shards
		WHERE project = $1 AND search_vector @@ plainto_tsquery($2)
	`
	args := []interface{}{c.Config.Project, query}

	if shardType != "" {
		sqlQuery += ` AND type = $3 ORDER BY rank DESC LIMIT $4`
		args = append(args, shardType, limit)
	} else {
		sqlQuery += ` ORDER BY rank DESC LIMIT $3`
		args = append(args, limit)
	}

	rows, err := conn.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search shards: %v", err)
	}
	defer rows.Close()

	var shards []Shard
	for rows.Next() {
		var s Shard
		var rank float64
		if err := rows.Scan(&s.ID, &s.Project, &s.Title, &s.Content, &s.Type, &s.Status,
			&s.Priority, &s.Creator, &s.Owner, &s.CreatedAt, &s.UpdatedAt,
			&s.Metadata, &rank); err != nil {
			continue
		}
		shards = append(shards, s)
	}
	return shards, nil
}
