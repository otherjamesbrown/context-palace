package client

import (
	"context"
	"fmt"
	"time"
)

// EpicProgress holds completion stats for an epic's children
type EpicProgress struct {
	Total      int `json:"total"`
	Completed  int `json:"completed"`
	InProgress int `json:"in_progress"`
	Open       int `json:"open"`
	Blocked    int `json:"blocked"`
}

// EpicChild holds a child shard's detail within an epic
type EpicChild struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Status       string     `json:"status"`
	Kind         string     `json:"kind"`
	Owner        *string    `json:"owner,omitempty"`
	Priority     *int       `json:"priority,omitempty"`
	AssignedAt   *time.Time `json:"assigned_at,omitempty"`
	ClosedAt     *time.Time `json:"closed_at,omitempty"`
	ClosedBy     *string    `json:"closed_by,omitempty"`
	ClosedReason *string    `json:"closed_reason,omitempty"`
	BlockedBy    []string   `json:"blocked_by"`
}

// GetEpicProgress returns completion stats for an epic
func (c *Client) GetEpicProgress(ctx context.Context, epicID string) (*EpicProgress, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var p EpicProgress
	err = conn.QueryRow(ctx, `
		SELECT total, completed, in_progress, open, blocked
		FROM epic_progress($1, $2)
	`, c.Config.Project, epicID).Scan(&p.Total, &p.Completed, &p.InProgress, &p.Open, &p.Blocked)
	if err != nil {
		return nil, fmt.Errorf("failed to get epic progress: %v", err)
	}
	return &p, nil
}

// GetEpicChildren returns the children of an epic with full details
func (c *Client) GetEpicChildren(ctx context.Context, epicID string) ([]EpicChild, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT id, title, status, kind, owner, priority,
			assigned_at, closed_at, closed_by, closed_reason, blocked_by
		FROM epic_children($1, $2)
	`, c.Config.Project, epicID)
	if err != nil {
		return nil, fmt.Errorf("failed to get epic children: %v", err)
	}
	defer rows.Close()

	var children []EpicChild
	for rows.Next() {
		var ch EpicChild
		if err := rows.Scan(&ch.ID, &ch.Title, &ch.Status, &ch.Kind,
			&ch.Owner, &ch.Priority, &ch.AssignedAt, &ch.ClosedAt,
			&ch.ClosedBy, &ch.ClosedReason, &ch.BlockedBy); err != nil {
			return nil, fmt.Errorf("failed to scan epic child: %v", err)
		}
		if ch.BlockedBy == nil {
			ch.BlockedBy = []string{}
		}
		children = append(children, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("epic children iteration error: %v", err)
	}
	return children, nil
}

// CreateEpic creates an epic shard and optionally adopts children + sets order edges.
// Returns the epic ID.
func (c *Client) CreateEpic(ctx context.Context, title, body string, priority *int, labels []string, adoptIDs []string, orderEdges []OrderEdge) (string, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	// Add kind:epic label
	allLabels := append([]string{"kind:epic"}, labels...)

	// Create the epic shard
	var epicID string
	err = tx.QueryRow(ctx, `
		SELECT create_shard($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, c.Config.Project, c.Config.Agent, title, body, "epic",
		allLabels, nil, priority, "{}").Scan(&epicID)
	if err != nil {
		return "", fmt.Errorf("failed to create epic shard: %v", err)
	}

	// Adopt children: set parent_id
	for _, childID := range adoptIDs {
		var existingParent *string
		var childProject string
		err = tx.QueryRow(ctx, `
			SELECT project, parent_id FROM shards WHERE id = $1
		`, childID).Scan(&childProject, &existingParent)
		if err != nil {
			return "", fmt.Errorf("Shard %s not found", childID)
		}
		if childProject != c.Config.Project {
			return "", fmt.Errorf("Shard %s belongs to a different project", childID)
		}
		if existingParent != nil && *existingParent != "" {
			return "", fmt.Errorf("Shard %s already belongs to epic %s", childID, *existingParent)
		}

		_, err = tx.Exec(ctx, `
			UPDATE shards SET parent_id = $1, updated_at = NOW() WHERE id = $2
		`, epicID, childID)
		if err != nil {
			return "", fmt.Errorf("failed to adopt shard %s: %v", childID, err)
		}
	}

	// Create blocked-by edges for ordering
	for _, edge := range orderEdges {
		_, err = tx.Exec(ctx, `SELECT create_edge($1, $2, $3, $4)`,
			edge.From, edge.BlockedBy, "blocked-by", nil)
		if err != nil {
			return "", fmt.Errorf("failed to create edge %s blocked-by %s: %s",
				edge.From, edge.BlockedBy, extractPgMessage(err.Error()))
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("failed to commit epic creation: %v", err)
	}

	// Embed-on-write
	c.tryEmbed(ctx, epicID, "epic", title, body)

	return epicID, nil
}

// OrderEdge represents a blocked-by dependency between two shards
type OrderEdge struct {
	From      string `json:"from"`
	BlockedBy string `json:"blocked_by"`
}
