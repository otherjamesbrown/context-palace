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

// ShardContextResult holds a shard and its family (parent, siblings, children)
type ShardContextResult struct {
	Target   ShardTreeNode
	Parent   *ShardTreeNode
	Siblings []ShardTreeNode
	Children []ShardTreeNode
}

// GetShardContext fetches a shard and its family context for search display
func (c *Client) GetShardContext(ctx context.Context, id string) (*ShardContextResult, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	// Fetch target shard with parent_id (via column or child-of edge)
	var target ShardTreeNode
	var parentID *string
	err = conn.QueryRow(ctx, `
		SELECT s.id, s.title, s.type, s.status, s.labels,
			_shard_child_count(s.id, true),
			s.created_at, s.updated_at,
			COALESCE(s.parent_id, (
				SELECT e.to_id FROM edges e
				WHERE e.from_id = s.id AND e.edge_type = 'child-of'
				LIMIT 1
			))
		FROM shards s
		WHERE s.id = $1 AND s.project = $2
	`, id, c.Config.Project).Scan(
		&target.ID, &target.Title, &target.Type, &target.Status,
		&target.Labels, &target.ChildCount, &target.CreatedAt, &target.UpdatedAt,
		&parentID,
	)
	if err != nil {
		return nil, fmt.Errorf("shard %s not found", id)
	}

	result := &ShardContextResult{Target: target}

	// Fetch parent and siblings if parent exists
	if parentID != nil {
		var parent ShardTreeNode
		err = conn.QueryRow(ctx, `
			SELECT s.id, s.title, s.type, s.status, s.labels,
				_shard_child_count(s.id, true),
				s.created_at, s.updated_at
			FROM shards s WHERE s.id = $1
		`, *parentID).Scan(
			&parent.ID, &parent.Title, &parent.Type, &parent.Status,
			&parent.Labels, &parent.ChildCount, &parent.CreatedAt, &parent.UpdatedAt,
		)
		if err == nil {
			result.Parent = &parent
		}

		// Fetch siblings (other children of parent, excluding target)
		sibRows, err := conn.Query(ctx, `
			SELECT s.id, s.title, s.type, s.status, s.labels,
				_shard_child_count(s.id, true),
				s.created_at, s.updated_at
			FROM shards s
			WHERE s.id IN (
				SELECT c.id FROM shards c WHERE c.parent_id = $1
				UNION
				SELECT e.from_id FROM edges e
				WHERE e.to_id = $1 AND e.edge_type = 'child-of'
			)
			AND s.id != $2
			ORDER BY s.created_at
		`, *parentID, id)
		if err == nil {
			defer sibRows.Close()
			result.Siblings, _ = scanTreeNodes(sibRows)
		}
	}

	// Fetch children (reuse existing function)
	children, err := c.GetShardTreeChildren(ctx, id, true)
	if err == nil {
		result.Children = children
	}

	return result, nil
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
