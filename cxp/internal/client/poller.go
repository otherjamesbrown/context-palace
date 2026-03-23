package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ShardSummary is a lightweight shard representation used by poller queries.
type ShardSummary struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Type      string          `json:"type"`
	Status    string          `json:"status"`
	UpdatedAt time.Time       `json:"updated_at,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

// PipelineWait represents a design whose waiting_for condition may be satisfied.
type PipelineWait struct {
	DesignID   string   `json:"design_id"`
	Title      string   `json:"title"`
	WaitingFor []string `json:"waiting_for"`
}

// FindNewDesigns returns open design shards that do NOT have pipeline metadata.
func (c *Client) FindNewDesigns(ctx context.Context) ([]ShardSummary, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT id, title, type, status
		FROM shards
		WHERE project = $1
			AND type = 'design'
			AND status = 'open'
			AND (metadata IS NULL OR NOT (metadata ? 'pipeline'))
		ORDER BY created_at ASC
	`, c.Config.Project)
	if err != nil {
		return nil, fmt.Errorf("FindNewDesigns: %w", err)
	}
	defer rows.Close()

	var results []ShardSummary
	for rows.Next() {
		var s ShardSummary
		if err := rows.Scan(&s.ID, &s.Title, &s.Type, &s.Status); err != nil {
			return nil, fmt.Errorf("FindNewDesigns scan: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// FindTasksNeedingReview returns tasks in needs-review status whose parent
// design has pipeline metadata with phase = "implement".
func (c *Client) FindTasksNeedingReview(ctx context.Context) ([]ShardSummary, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT t.id, t.title, t.type, t.status
		FROM shards t
		JOIN edges e ON e.from_id = t.id AND e.edge_type = 'child-of'
		JOIN shards d ON d.id = e.to_id AND d.type = 'design'
		WHERE t.project = $1
			AND t.type = 'task'
			AND t.status = 'needs-review'
			AND d.metadata IS NOT NULL
			AND d.metadata ? 'pipeline'
			AND d.metadata->'pipeline'->>'phase' = 'implement'
		ORDER BY t.updated_at ASC
	`, c.Config.Project)
	if err != nil {
		return nil, fmt.Errorf("FindTasksNeedingReview: %w", err)
	}
	defer rows.Close()

	var results []ShardSummary
	for rows.Next() {
		var s ShardSummary
		if err := rows.Scan(&s.ID, &s.Title, &s.Type, &s.Status); err != nil {
			return nil, fmt.Errorf("FindTasksNeedingReview scan: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// FindSatisfiedWaits returns designs whose pipeline.waiting_for list is
// non-empty AND every shard in that list has status = "closed".
func (c *Client) FindSatisfiedWaits(ctx context.Context) ([]PipelineWait, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	// Step 1: find designs with non-empty waiting_for
	rows, err := conn.Query(ctx, `
		SELECT id, title, metadata->'pipeline'->'waiting_for' AS waiting_for
		FROM shards
		WHERE project = $1
			AND type = 'design'
			AND status IN ('open', 'ready', 'in_progress', 'needs-review')
			AND metadata IS NOT NULL
			AND metadata ? 'pipeline'
			AND jsonb_array_length(COALESCE(metadata->'pipeline'->'waiting_for', '[]'::jsonb)) > 0
		ORDER BY updated_at ASC
	`, c.Config.Project)
	if err != nil {
		return nil, fmt.Errorf("FindSatisfiedWaits: %w", err)
	}
	defer rows.Close()

	type candidate struct {
		ID         string
		Title      string
		WaitingFor []string
	}
	var candidates []candidate
	for rows.Next() {
		var id, title string
		var wfRaw json.RawMessage
		if err := rows.Scan(&id, &title, &wfRaw); err != nil {
			return nil, fmt.Errorf("FindSatisfiedWaits scan: %w", err)
		}
		var wf []string
		if err := json.Unmarshal(wfRaw, &wf); err != nil {
			continue // skip malformed
		}
		if len(wf) == 0 {
			continue
		}
		candidates = append(candidates, candidate{ID: id, Title: title, WaitingFor: wf})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	// Step 2: for each candidate, check if ALL waiting_for IDs are closed
	// We use a second connection to avoid interfering with the first query.
	conn2, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn2.Close(ctx)

	var results []PipelineWait
	for _, cand := range candidates {
		// Build a query to count non-closed shards from the waiting list
		row := conn2.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM shards
			WHERE id = ANY($1)
				AND status != 'closed'
		`, cand.WaitingFor)

		var notClosed int
		if err := row.Scan(&notClosed); err != nil {
			continue
		}
		if notClosed == 0 {
			results = append(results, PipelineWait{
				DesignID:   cand.ID,
				Title:      cand.Title,
				WaitingFor: cand.WaitingFor,
			})
		}
	}

	return results, nil
}

// FindInProgressTasks returns all task shards currently in_progress,
// ordered by least recently updated first (most likely stalled).
func (c *Client) FindInProgressTasks(ctx context.Context) ([]ShardSummary, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT id, title, type, status, updated_at, metadata
		FROM shards
		WHERE project = $1
			AND type = 'task'
			AND status = 'in_progress'
		ORDER BY updated_at ASC
	`, c.Config.Project)
	if err != nil {
		return nil, fmt.Errorf("FindInProgressTasks: %w", err)
	}
	defer rows.Close()

	var results []ShardSummary
	for rows.Next() {
		var s ShardSummary
		if err := rows.Scan(&s.ID, &s.Title, &s.Type, &s.Status, &s.UpdatedAt, &s.Metadata); err != nil {
			return nil, fmt.Errorf("FindInProgressTasks scan: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}
