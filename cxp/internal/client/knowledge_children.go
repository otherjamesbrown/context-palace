package client

import (
	"context"
	"encoding/json"
	"fmt"
)

// KnowledgeChild represents a child document linked to a parent knowledge doc
type KnowledgeChild struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Trigger     string `json:"trigger"`
	Order       int    `json:"order"`
	AccessCount int    `json:"access_count,omitempty"`
}

// childEdgeMeta is the metadata stored on child-of edges for knowledge children
type childEdgeMeta struct {
	Description string `json:"description"`
	Trigger     string `json:"trigger"`
	Ordering    int    `json:"ordering"`
}

// ListKnowledgeChildren returns child docs linked to a parent knowledge doc via child-of edges.
// DB convention: child-of edges go from_id=child, to_id=parent.
// So we query INCOMING child-of edges on the parent to find its children.
// Children are sorted by: explicit ordering first, then access_count DESC, then title.
func (c *Client) ListKnowledgeChildren(ctx context.Context, parentID string) ([]KnowledgeChild, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT s.id, s.title,
			COALESCE(e.metadata->>'description', ''),
			COALESCE(e.metadata->>'trigger', ''),
			COALESCE((e.metadata->>'ordering')::INT, 0),
			COALESCE((s.metadata->>'access_count')::INT, 0)
		FROM edges e
		JOIN shards s ON s.id = e.from_id
		WHERE e.to_id = $1 AND e.edge_type = 'child-of'
		ORDER BY
			COALESCE((e.metadata->>'ordering')::INT, 999),
			COALESCE((s.metadata->>'access_count')::INT, 0) DESC,
			s.title
	`, parentID)
	if err != nil {
		return nil, fmt.Errorf("list children: %w", err)
	}
	defer rows.Close()

	var children []KnowledgeChild
	for rows.Next() {
		var ch KnowledgeChild
		if err := rows.Scan(&ch.ID, &ch.Title, &ch.Description, &ch.Trigger, &ch.Order, &ch.AccessCount); err != nil {
			return nil, fmt.Errorf("scan child: %w", err)
		}
		children = append(children, ch)
	}
	return children, rows.Err()
}

// AddKnowledgeChild links a child knowledge doc to a parent with description and trigger metadata.
// DB convention: child-of edge from_id=child, to_id=parent.
func (c *Client) AddKnowledgeChild(ctx context.Context, parentID, childID, description, trigger string) error {
	// Validate both shards exist and are knowledge type
	for _, id := range []string{parentID, childID} {
		shard, err := c.GetShard(ctx, id)
		if err != nil {
			return fmt.Errorf("shard %s not found", id)
		}
		if shard.Type != "knowledge" {
			return fmt.Errorf("shard %s is type '%s', expected 'knowledge'", id, shard.Type)
		}
	}

	// Compute next ordering value from existing children
	existing, err := c.ListKnowledgeChildren(ctx, parentID)
	if err != nil {
		return err
	}
	nextOrder := 0
	for _, ch := range existing {
		if ch.Order >= nextOrder {
			nextOrder = ch.Order + 1
		}
	}

	meta := childEdgeMeta{
		Description: description,
		Trigger:     trigger,
		Ordering:    nextOrder,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	// Edge goes from child → parent (child is-child-of parent)
	return c.CreateEdge(ctx, childID, parentID, "child-of", metaJSON)
}

// UpdateKnowledgeChild updates the description and/or trigger on an existing child-of edge
func (c *Client) UpdateKnowledgeChild(ctx context.Context, parentID, childID, description, trigger string) error {
	// Get existing edge — incoming to parent from children
	edges, err := c.GetShardEdges(ctx, parentID, "incoming", []string{"child-of"})
	if err != nil {
		return fmt.Errorf("get existing edge: %w", err)
	}

	var existing *childEdgeMeta
	for _, e := range edges {
		if e.ShardID == childID && e.EdgeMetadata != nil {
			var meta childEdgeMeta
			if json.Unmarshal(e.EdgeMetadata, &meta) == nil {
				existing = &meta
				break
			}
		}
	}
	if existing == nil {
		return fmt.Errorf("no child-of edge from %s to %s", childID, parentID)
	}

	// Apply updates (only override if non-empty)
	if description != "" {
		existing.Description = description
	}
	if trigger != "" {
		existing.Trigger = trigger
	}

	// Delete and recreate with updated metadata (edge is from child → parent)
	if err := c.DeleteEdge(ctx, childID, parentID, "child-of"); err != nil {
		return fmt.Errorf("remove old edge: %w", err)
	}

	metaJSON, err := json.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	return c.CreateEdge(ctx, childID, parentID, "child-of", metaJSON)
}

// RemoveKnowledgeChild removes a child-of edge between parent and child
func (c *Client) RemoveKnowledgeChild(ctx context.Context, parentID, childID string) error {
	// Edge is from child → parent
	return c.DeleteEdge(ctx, childID, parentID, "child-of")
}
