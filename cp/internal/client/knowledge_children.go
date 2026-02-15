package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// KnowledgeChild represents a child document linked to a parent knowledge doc
type KnowledgeChild struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Trigger     string `json:"trigger"`
	Order       int    `json:"order"`
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
func (c *Client) ListKnowledgeChildren(ctx context.Context, parentID string) ([]KnowledgeChild, error) {
	edges, err := c.GetShardEdges(ctx, parentID, "incoming", []string{"child-of"})
	if err != nil {
		return nil, fmt.Errorf("list children: %w", err)
	}

	var children []KnowledgeChild
	for _, e := range edges {
		child := KnowledgeChild{
			ID:    e.ShardID,
			Title: e.Title,
		}
		if e.EdgeMetadata != nil {
			var meta childEdgeMeta
			if json.Unmarshal(e.EdgeMetadata, &meta) == nil {
				child.Description = meta.Description
				child.Trigger = meta.Trigger
				child.Order = meta.Ordering
			}
		}
		children = append(children, child)
	}

	sort.Slice(children, func(i, j int) bool {
		return children[i].Order < children[j].Order
	})

	return children, nil
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
