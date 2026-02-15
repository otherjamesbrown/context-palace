package client

import (
	"context"
	"fmt"
	"time"
)

// ShardType holds a shard type with its counts
type ShardType struct {
	Type        string `json:"type"`
	Count       int    `json:"count"`
	OpenCount   int    `json:"open_count"`
	ClosedCount int    `json:"closed_count"`
}

// ShardTreeNode holds a node in the shard tree
type ShardTreeNode struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	Labels     []string  `json:"labels,omitempty"`
	ChildCount int       `json:"child_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// GetShardTypes returns distinct shard types with counts for the project
func (c *Client) GetShardTypes(ctx context.Context, includeClosed bool) ([]ShardType, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx,
		`SELECT shard_type, shard_count, open_count, closed_count FROM shard_types($1, $2)`,
		c.Config.Project, includeClosed)
	if err != nil {
		return nil, fmt.Errorf("failed to get shard types: %v", err)
	}
	defer rows.Close()

	var types []ShardType
	for rows.Next() {
		var t ShardType
		if err := rows.Scan(&t.Type, &t.Count, &t.OpenCount, &t.ClosedCount); err != nil {
			return nil, fmt.Errorf("failed to scan shard type: %v", err)
		}
		types = append(types, t)
	}
	return types, rows.Err()
}

// GetShardTreeRoots returns root shards of a given type with child counts
func (c *Client) GetShardTreeRoots(ctx context.Context, shardType string, includeClosed bool) ([]ShardTreeNode, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx,
		`SELECT id, title, shard_type, status, labels, child_count, created_at, updated_at
		 FROM shard_tree_roots($1, $2, $3)`,
		c.Config.Project, shardType, includeClosed)
	if err != nil {
		return nil, fmt.Errorf("failed to get tree roots: %v", err)
	}
	defer rows.Close()

	return scanTreeNodes(rows)
}

// GetShardTreeChildren returns direct children of a parent shard with child counts
func (c *Client) GetShardTreeChildren(ctx context.Context, parentID string, includeClosed bool) ([]ShardTreeNode, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx,
		`SELECT id, title, shard_type, status, labels, child_count, created_at, updated_at
		 FROM shard_tree_children($1, $2, $3)`,
		c.Config.Project, parentID, includeClosed)
	if err != nil {
		return nil, fmt.Errorf("failed to get tree children: %v", err)
	}
	defer rows.Close()

	return scanTreeNodes(rows)
}

func scanTreeNodes(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]ShardTreeNode, error) {
	var nodes []ShardTreeNode
	for rows.Next() {
		var n ShardTreeNode
		if err := rows.Scan(&n.ID, &n.Title, &n.Type, &n.Status,
			&n.Labels, &n.ChildCount, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan tree node: %v", err)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}
