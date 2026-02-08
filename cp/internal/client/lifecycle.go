package client

import (
	"context"
	"fmt"
	"time"
)

// AssignResult holds the result of assigning a shard
type AssignResult struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Owner string `json:"owner"`
}

// AssignShard atomically claims a shard for an agent
func (c *Client) AssignShard(ctx context.Context, shardID string, agent string) (*AssignResult, error) {
	if agent == "" {
		agent = c.Config.Agent
	}

	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var title string
	err = conn.QueryRow(ctx, `SELECT shard_assign($1, $2, $3)`,
		c.Config.Project, shardID, agent).Scan(&title)
	if err != nil {
		return nil, fmt.Errorf("%s", extractPgMessage(err.Error()))
	}

	return &AssignResult{
		ID:    shardID,
		Title: title,
		Owner: agent,
	}, nil
}

// CloseResult holds the result of closing a shard
type CloseResult struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	ClosedAt  time.Time `json:"closed_at"`
	Reason    string    `json:"reason,omitempty"`
	Unblocked []UnblockedShard `json:"unblocked,omitempty"`
	WasAlreadyClosed bool `json:"was_already_closed,omitempty"`
}

// UnblockedShard represents a shard that was unblocked by a close operation
type UnblockedShard struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// CloseShard closes a shard with an optional reason, returning unblocked info
func (c *Client) CloseShard(ctx context.Context, shardID string, reason string) (*CloseResult, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var reasonArg interface{}
	if reason != "" {
		reasonArg = reason
	}

	rows, err := conn.Query(ctx, `
		SELECT closed_title, unblocked_id, unblocked_title
		FROM shard_close($1, $2, $3, $4)
	`, c.Config.Project, shardID, c.Config.Agent, reasonArg)
	if err != nil {
		return nil, fmt.Errorf("%s", extractPgMessage(err.Error()))
	}
	defer rows.Close()

	result := &CloseResult{
		ID:       shardID,
		Status:   "closed",
		ClosedAt: time.Now(),
		Reason:   reason,
	}

	first := true
	for rows.Next() {
		var closedTitle, unblockedID, unblockedTitle *string
		if err := rows.Scan(&closedTitle, &unblockedID, &unblockedTitle); err != nil {
			return nil, fmt.Errorf("failed to scan close result: %v", err)
		}
		if first && closedTitle != nil {
			result.Title = *closedTitle
			first = false
		}
		if unblockedID != nil && unblockedTitle != nil {
			result.Unblocked = append(result.Unblocked, UnblockedShard{
				ID:    *unblockedID,
				Title: *unblockedTitle,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("close result iteration error: %v", err)
	}

	return result, nil
}

// NextShard holds a candidate for "next work" queries
type NextShard struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Kind      string  `json:"kind"`
	Priority  *int    `json:"priority,omitempty"`
	EpicID    *string `json:"epic_id,omitempty"`
	EpicTitle *string `json:"epic_title,omitempty"`
}

// GetNextShards returns the next unblocked open shards
func (c *Client) GetNextShards(ctx context.Context, epicID *string, limit int) ([]NextShard, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	if limit <= 0 {
		limit = 1
	}
	if limit > 10 {
		limit = 10
	}

	rows, err := conn.Query(ctx, `
		SELECT id, title, kind, priority, epic_id, epic_title
		FROM shard_next($1, $2, $3)
	`, c.Config.Project, epicID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get next shards: %v", err)
	}
	defer rows.Close()

	var shards []NextShard
	for rows.Next() {
		var s NextShard
		if err := rows.Scan(&s.ID, &s.Title, &s.Kind, &s.Priority,
			&s.EpicID, &s.EpicTitle); err != nil {
			return nil, fmt.Errorf("failed to scan next shard: %v", err)
		}
		shards = append(shards, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("next shard iteration error: %v", err)
	}
	return shards, nil
}

// BoardShard holds a shard in the board view
type BoardShard struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Status     string     `json:"status"`
	Kind       string     `json:"kind"`
	Owner      *string    `json:"owner,omitempty"`
	Priority   *int       `json:"priority,omitempty"`
	EpicID     *string    `json:"epic_id,omitempty"`
	EpicTitle  *string    `json:"epic_title,omitempty"`
	AssignedAt *time.Time `json:"assigned_at,omitempty"`
	ClosedAt   *time.Time `json:"closed_at,omitempty"`
	BlockedBy  []string   `json:"blocked_by"`
}

// GetShardBoard returns shards for the board view
func (c *Client) GetShardBoard(ctx context.Context, epicID *string, agent *string) ([]BoardShard, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT id, title, status, kind, owner, priority,
			epic_id, epic_title, assigned_at, closed_at, blocked_by
		FROM shard_board($1, $2, $3)
	`, c.Config.Project, epicID, agent)
	if err != nil {
		return nil, fmt.Errorf("failed to get shard board: %v", err)
	}
	defer rows.Close()

	var shards []BoardShard
	for rows.Next() {
		var s BoardShard
		if err := rows.Scan(&s.ID, &s.Title, &s.Status, &s.Kind,
			&s.Owner, &s.Priority, &s.EpicID, &s.EpicTitle,
			&s.AssignedAt, &s.ClosedAt, &s.BlockedBy); err != nil {
			return nil, fmt.Errorf("failed to scan board shard: %v", err)
		}
		if s.BlockedBy == nil {
			s.BlockedBy = []string{}
		}
		shards = append(shards, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("board iteration error: %v", err)
	}
	return shards, nil
}
