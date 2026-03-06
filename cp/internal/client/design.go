package client

import (
	"context"
	"fmt"
)

// CreateDesign creates a design shard and optionally sets parent, adopts children, and creates order edges.
func (c *Client) CreateDesign(ctx context.Context, title, body string, labels []string, parentID string, adoptIDs []string, orderEdges []OrderEdge) (string, error) {
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

	// Create the design shard
	var id string
	err = tx.QueryRow(ctx, `
		SELECT create_shard($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, c.Config.Project, c.Config.Agent, title, body, "design",
		labels, nil, nil, "{}").Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to create design shard: %v", err)
	}

	// Create child-of edge to parent
	if parentID != "" {
		_, err = tx.Exec(ctx, `SELECT create_edge($1, $2, $3, $4)`,
			id, parentID, "child-of", nil)
		if err != nil {
			return "", fmt.Errorf("failed to create parent edge: %s", extractPgMessage(err.Error()))
		}
	}

	// Adopt children: create child-of edges from child → this design
	for _, childID := range adoptIDs {
		_, err = tx.Exec(ctx, `SELECT create_edge($1, $2, $3, $4)`,
			childID, id, "child-of", nil)
		if err != nil {
			return "", fmt.Errorf("failed to adopt shard %s: %s", childID, extractPgMessage(err.Error()))
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
		return "", fmt.Errorf("failed to commit design creation: %v", err)
	}

	// Embed-on-write
	c.tryEmbed(ctx, id, "design", title, body)

	return id, nil
}
