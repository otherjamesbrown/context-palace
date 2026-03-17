package client

import (
	"context"
	"fmt"
)

// LabelCount represents a label with its shard count
type LabelCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// AddShardLabels adds labels to a shard atomically, returns updated labels
func (c *Client) AddShardLabels(ctx context.Context, shardID string, labels []string) ([]string, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var result []string
	err = conn.QueryRow(ctx, `SELECT add_shard_labels($1, $2)`, shardID, labels).Scan(&result)
	if err != nil {
		return nil, fmt.Errorf("%s", extractPgMessage(err.Error()))
	}
	return result, nil
}

// RemoveShardLabels removes labels from a shard atomically, returns updated labels
func (c *Client) RemoveShardLabels(ctx context.Context, shardID string, labels []string) ([]string, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	var result []string
	err = conn.QueryRow(ctx, `SELECT remove_shard_labels($1, $2)`, shardID, labels).Scan(&result)
	if err != nil {
		return nil, fmt.Errorf("%s", extractPgMessage(err.Error()))
	}
	return result, nil
}

// LabelSummary returns all labels in use with their counts
func (c *Client) LabelSummary(ctx context.Context) ([]LabelCount, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `SELECT label, shard_count FROM label_summary($1)`, c.Config.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to get label summary: %v", err)
	}
	defer rows.Close()

	var labels []LabelCount
	for rows.Next() {
		var l LabelCount
		if err := rows.Scan(&l.Label, &l.Count); err != nil {
			return nil, fmt.Errorf("failed to scan label: %v", err)
		}
		labels = append(labels, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("label iteration error: %v", err)
	}
	return labels, nil
}
