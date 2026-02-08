package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ValidEdgeTypes is the canonical registry of valid edge types
var ValidEdgeTypes = []string{
	"blocked-by", "blocks", "child-of", "discovered-from", "extends",
	"has-artifact", "implements", "parent", "previous-version",
	"references", "relates-to", "replies-to", "triggered-by",
}

// IsValidEdgeType checks if an edge type is in the registry
func IsValidEdgeType(edgeType string) bool {
	for _, t := range ValidEdgeTypes {
		if edgeType == t {
			return true
		}
	}
	return false
}

// EdgeInfo represents an edge with linked shard details
type EdgeInfo struct {
	Direction      string          `json:"direction"`
	EdgeType       string          `json:"edge_type"`
	ShardID        string          `json:"shard_id"`
	Title          string          `json:"title"`
	Type           string          `json:"type"`
	Status         string          `json:"status"`
	EdgeMetadata   json.RawMessage `json:"edge_metadata,omitempty"`
}

// EdgeTreeNode represents a node in the edge follow tree
type EdgeTreeNode struct {
	ShardID   string          `json:"shard_id"`
	Title     string          `json:"title"`
	Type      string          `json:"type"`
	Status    string          `json:"status"`
	EdgeType  string          `json:"edge_type"`
	Direction string          `json:"direction"`
	Children  []*EdgeTreeNode `json:"children,omitempty"`
	IsCycle   bool            `json:"is_cycle,omitempty"`
}

// CreateEdge creates a typed edge between two shards
func (c *Client) CreateEdge(ctx context.Context, fromID, toID, edgeType string, metadata json.RawMessage) error {
	if !IsValidEdgeType(edgeType) {
		return fmt.Errorf("unknown edge type: %s. Valid types: %s",
			edgeType, strings.Join(ValidEdgeTypes, ", "))
	}

	// For blocked-by edges, check circular dependencies
	if edgeType == "blocked-by" {
		circular, err := c.hasCircularDependency(ctx, fromID, toID)
		if err != nil {
			return fmt.Errorf("check circular dependency: %w", err)
		}
		if circular {
			return fmt.Errorf("circular dependency detected")
		}
	}

	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	var metaArg interface{}
	if metadata != nil {
		metaArg = string(metadata)
	}

	_, err = conn.Exec(ctx, `SELECT create_edge($1, $2, $3, $4)`,
		fromID, toID, edgeType, metaArg)
	if err != nil {
		return fmt.Errorf("%s", extractPgMessage(err.Error()))
	}
	return nil
}

// DeleteEdge removes an edge between two shards
func (c *Client) DeleteEdge(ctx context.Context, fromID, toID, edgeType string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `SELECT delete_edge($1, $2, $3)`,
		fromID, toID, edgeType)
	if err != nil {
		return fmt.Errorf("%s", extractPgMessage(err.Error()))
	}
	return nil
}

// GetShardEdges returns all edges for a shard with optional filters
func (c *Client) GetShardEdges(ctx context.Context, shardID string, direction string, edgeTypes []string) ([]EdgeInfo, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var dirArg, edgeTypesArg interface{}
	if direction != "" {
		dirArg = direction
	}
	if edgeTypes != nil {
		edgeTypesArg = edgeTypes
	}

	rows, err := conn.Query(ctx, `
		SELECT direction, edge_type, linked_shard_id, linked_shard_title,
			linked_shard_type, linked_shard_status, edge_metadata
		FROM shard_edges($1, $2, $3)
	`, shardID, dirArg, edgeTypesArg)
	if err != nil {
		return nil, fmt.Errorf("failed to get shard edges: %v", err)
	}
	defer rows.Close()

	var edges []EdgeInfo
	for rows.Next() {
		var e EdgeInfo
		if err := rows.Scan(&e.Direction, &e.EdgeType, &e.ShardID,
			&e.Title, &e.Type, &e.Status, &e.EdgeMetadata); err != nil {
			return nil, fmt.Errorf("failed to scan edge: %v", err)
		}
		edges = append(edges, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("edge iteration error: %v", err)
	}
	return edges, nil
}

// GetShardEdgesFollow returns a tree of edges following links up to maxDepth
func (c *Client) GetShardEdgesFollow(ctx context.Context, shardID string, direction string, edgeTypes []string, maxDepth int) ([]*EdgeTreeNode, error) {
	visited := map[string]bool{shardID: true}
	return c.followEdges(ctx, shardID, direction, edgeTypes, maxDepth, 1, visited)
}

func (c *Client) followEdges(ctx context.Context, shardID string, direction string, edgeTypes []string, maxDepth, currentDepth int, visited map[string]bool) ([]*EdgeTreeNode, error) {
	edges, err := c.GetShardEdges(ctx, shardID, direction, edgeTypes)
	if err != nil {
		return nil, err
	}

	var nodes []*EdgeTreeNode
	for _, e := range edges {
		node := &EdgeTreeNode{
			ShardID:   e.ShardID,
			Title:     e.Title,
			Type:      e.Type,
			Status:    e.Status,
			EdgeType:  e.EdgeType,
			Direction: e.Direction,
		}

		if visited[e.ShardID] {
			node.IsCycle = true
			nodes = append(nodes, node)
			continue
		}

		visited[e.ShardID] = true

		if currentDepth < maxDepth {
			children, err := c.followEdges(ctx, e.ShardID, direction, edgeTypes, maxDepth, currentDepth+1, visited)
			if err != nil {
				return nil, err
			}
			node.Children = children
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

// hasCircularDependency checks if adding fromID -> toID as blocked-by would create a cycle
func (c *Client) hasCircularDependency(ctx context.Context, fromID, toID string) (bool, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close(ctx)

	var circular bool
	err = conn.QueryRow(ctx, `SELECT has_circular_dependency($1, $2)`, fromID, toID).Scan(&circular)
	if err != nil {
		// If function doesn't exist, skip the check
		return false, nil
	}
	return circular, nil
}

// ShardExists checks if a shard exists
func (c *Client) ShardExists(ctx context.Context, id string) (bool, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close(ctx)

	var exists bool
	err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shards WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check shard existence: %v", err)
	}
	return exists, nil
}

// CreateEdgeSimple creates a basic edge without metadata (convenience for memory --references)
func (c *Client) CreateEdgeSimple(ctx context.Context, fromID, toID, edgeType string) error {
	return c.CreateEdge(ctx, fromID, toID, edgeType, nil)
}

// ShardDetailResult holds the result of a shard_detail() call
type ShardDetailResult struct {
	ID                string          `json:"id"`
	Title             string          `json:"title"`
	Content           string          `json:"content"`
	Type              string          `json:"type"`
	Status            string          `json:"status"`
	Creator           string          `json:"creator"`
	Labels            []string        `json:"labels,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	OutgoingEdgeCount int             `json:"outgoing_edge_count"`
	IncomingEdgeCount int             `json:"incoming_edge_count"`
	Edges             []EdgeInfo      `json:"edges,omitempty"`
}

// GetShardDetail fetches a shard with full detail (including edge counts)
func (c *Client) GetShardDetail(ctx context.Context, id string) (*ShardDetailResult, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var d ShardDetailResult
	err = conn.QueryRow(ctx, `
		SELECT id, title, COALESCE(content, ''), type, status, creator,
			labels, COALESCE(metadata, '{}'), created_at, updated_at,
			outgoing_edge_count, incoming_edge_count
		FROM shard_detail($1)
	`, id).Scan(&d.ID, &d.Title, &d.Content, &d.Type, &d.Status, &d.Creator,
		&d.Labels, &d.Metadata, &d.CreatedAt, &d.UpdatedAt,
		&d.OutgoingEdgeCount, &d.IncomingEdgeCount)
	if err != nil {
		return nil, fmt.Errorf("Shard %s not found", id)
	}

	return &d, nil
}
