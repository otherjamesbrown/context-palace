package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// RequirementDashboardRow represents a row from the requirement_dashboard() function
type RequirementDashboardRow struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	LifecycleStatus string    `json:"lifecycle_status"`
	Priority        int       `json:"priority"`
	Category        *string   `json:"category,omitempty"`
	TaskCountTotal  int       `json:"task_count_total"`
	TaskCountClosed int       `json:"task_count_closed"`
	TestCount       int       `json:"test_count"`
	BlockedByIDs    []string  `json:"blocked_by_ids,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// RequirementEdge represents an edge linked to a requirement
type RequirementEdge struct {
	FromID          string `json:"from_id"`
	ToID            string `json:"to_id"`
	EdgeType        string `json:"edge_type"`
	Title           string `json:"title"`
	LifecycleStatus string `json:"lifecycle_status,omitempty"`
}

// Valid lifecycle transitions: from -> []to
var validTransitions = map[string][]string{
	"draft":       {"approved"},
	"approved":    {"in_progress"},
	"in_progress": {"implemented", "approved"},
	"implemented": {"verified", "approved"},
	"verified":    {"approved"},
}

// validateTransition checks if a lifecycle transition is valid
func validateTransition(from, to string) error {
	allowed, ok := validTransitions[from]
	if !ok {
		return fmt.Errorf("unknown lifecycle status: %s", from)
	}
	for _, a := range allowed {
		if a == to {
			return nil
		}
	}
	return fmt.Errorf("cannot transition from '%s' to '%s'", from, to)
}

// getLifecycleStatus fetches the current lifecycle_status from a requirement's metadata
func (c *Client) getLifecycleStatus(ctx context.Context, conn *pgx.Conn, id string) (string, error) {
	var shardType string
	var meta json.RawMessage
	err := conn.QueryRow(ctx, `
		SELECT type, COALESCE(metadata, '{}') FROM shards WHERE id = $1
	`, id).Scan(&shardType, &meta)
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("shard not found: %s", id)
	}
	if err != nil {
		return "", fmt.Errorf("failed to fetch shard: %v", err)
	}
	if shardType != "requirement" {
		return "", fmt.Errorf("shard %s is type '%s', expected 'requirement'", id, shardType)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(meta, &m); err != nil {
		return "draft", nil
	}
	if status, ok := m["lifecycle_status"].(string); ok {
		return status, nil
	}
	return "draft", nil
}

// CreateRequirement creates a requirement shard with lifecycle metadata
func (c *Client) CreateRequirement(ctx context.Context, title, content string, priority int, category string) (string, error) {
	meta := map[string]interface{}{
		"lifecycle_status": "draft",
		"priority":         priority,
	}
	if category != "" {
		meta["category"] = category
	}

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %v", err)
	}

	pri := priority
	return c.CreateShardWithMetadata(ctx, title, content, "requirement", &pri, nil, json.RawMessage(metaJSON))
}

// ListRequirements lists requirements with optional filters
func (c *Client) ListRequirements(ctx context.Context, statusFilter []string, categoryFilter string, limit int) ([]RequirementDashboardRow, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	query := `
		SELECT
			s.id, s.title,
			COALESCE(s.metadata->>'lifecycle_status', 'draft'),
			COALESCE((s.metadata->>'priority')::int, 3),
			s.metadata->>'category',
			(SELECT count(*) FROM edges e WHERE e.to_id = s.id AND e.edge_type = 'implements')::int,
			(SELECT count(*) FROM edges e
			 JOIN shards t ON t.id = e.from_id
			 WHERE e.to_id = s.id AND e.edge_type = 'implements'
			 AND t.status = 'closed')::int,
			(SELECT count(*) FROM edges e
			 WHERE e.to_id = s.id AND e.edge_type = 'has-artifact'
			 AND EXISTS (SELECT 1 FROM shards a WHERE a.id = e.from_id AND a.type = 'test'))::int,
			s.created_at,
			s.updated_at
		FROM shards s
		WHERE s.project = $1 AND s.type = 'requirement' AND s.status != 'closed'
	`
	args := []interface{}{c.Config.Project}
	paramIdx := 2

	if len(statusFilter) > 0 {
		// Build lifecycle_status IN (...) filter
		query += fmt.Sprintf(` AND COALESCE(s.metadata->>'lifecycle_status', 'draft') = ANY($%d)`, paramIdx)
		args = append(args, statusFilter)
		paramIdx++
	}

	if categoryFilter != "" {
		query += fmt.Sprintf(` AND s.metadata->>'category' = $%d`, paramIdx)
		args = append(args, categoryFilter)
		paramIdx++
	}

	query += fmt.Sprintf(` ORDER BY COALESCE((s.metadata->>'priority')::int, 3), s.created_at LIMIT $%d`, paramIdx)
	args = append(args, limit)

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list requirements: %v", err)
	}
	defer rows.Close()

	var results []RequirementDashboardRow
	for rows.Next() {
		var r RequirementDashboardRow
		if err := rows.Scan(&r.ID, &r.Title, &r.LifecycleStatus, &r.Priority,
			&r.Category, &r.TaskCountTotal, &r.TaskCountClosed, &r.TestCount,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

// ShowRequirement fetches a requirement with full detail including edges
func (c *Client) ShowRequirement(ctx context.Context, id string) (*Shard, []RequirementEdge, int, int, int, error) {
	shard, err := c.GetShard(ctx, id)
	if err != nil {
		return nil, nil, 0, 0, 0, err
	}
	if shard.Type != "requirement" {
		return nil, nil, 0, 0, 0, fmt.Errorf("shard %s is type '%s', expected 'requirement'", id, shard.Type)
	}

	edges, err := c.GetRequirementEdges(ctx, id)
	if err != nil {
		return nil, nil, 0, 0, 0, err
	}

	// Count tasks and tests from edges
	var taskTotal, taskClosed, testCount int
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, nil, 0, 0, 0, err
	}
	defer conn.Close(ctx)

	conn.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM edges e WHERE e.to_id = $1 AND e.edge_type = 'implements')::int,
			(SELECT count(*) FROM edges e JOIN shards t ON t.id = e.from_id
			 WHERE e.to_id = $1 AND e.edge_type = 'implements' AND t.status = 'closed')::int,
			(SELECT count(*) FROM edges e
			 WHERE e.to_id = $1 AND e.edge_type = 'has-artifact'
			 AND EXISTS (SELECT 1 FROM shards a WHERE a.id = e.from_id AND a.type = 'test'))::int
	`, id).Scan(&taskTotal, &taskClosed, &testCount)

	return shard, edges, taskTotal, taskClosed, testCount, nil
}

// ApproveRequirement transitions a requirement from draft to approved
func (c *Client) ApproveRequirement(ctx context.Context, id string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	status, err := c.getLifecycleStatus(ctx, conn, id)
	if err != nil {
		return err
	}
	if status != "draft" {
		return fmt.Errorf("cannot approve: status is '%s', expected 'draft'", status)
	}

	_, err = c.SetMetadataPath(ctx, id, []string{"lifecycle_status"}, json.RawMessage(`"approved"`))
	return err
}

// VerifyRequirement transitions a requirement from implemented to verified
func (c *Client) VerifyRequirement(ctx context.Context, id string, force bool) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	status, err := c.getLifecycleStatus(ctx, conn, id)
	if err != nil {
		return err
	}

	if status != "implemented" {
		// Count open tasks for a better error message
		var openTasks int
		conn.QueryRow(ctx, `
			SELECT count(*) FROM edges e
			JOIN shards t ON t.id = e.from_id
			WHERE e.to_id = $1 AND e.edge_type = 'implements' AND t.status != 'closed'
		`, id).Scan(&openTasks)
		if openTasks > 0 {
			return fmt.Errorf("cannot verify: status is '%s', expected 'implemented'. %d tasks still open", status, openTasks)
		}
		return fmt.Errorf("cannot verify: status is '%s', expected 'implemented'", status)
	}

	if !force {
		// Check test coverage
		var testCount int
		conn.QueryRow(ctx, `
			SELECT count(*) FROM edges e
			WHERE e.to_id = $1 AND e.edge_type = 'has-artifact'
			AND EXISTS (SELECT 1 FROM shards a WHERE a.id = e.from_id AND a.type = 'test')
		`, id).Scan(&testCount)
		if testCount == 0 {
			return fmt.Errorf("no test coverage. Use --force to verify without tests")
		}
	}

	_, err = c.SetMetadataPath(ctx, id, []string{"lifecycle_status"}, json.RawMessage(`"verified"`))
	return err
}

// ReopenRequirement transitions a requirement back to approved
func (c *Client) ReopenRequirement(ctx context.Context, id, reason string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	status, err := c.getLifecycleStatus(ctx, conn, id)
	if err != nil {
		return err
	}
	if status == "draft" {
		return fmt.Errorf("cannot reopen: requirement is already in 'draft' status")
	}

	if err := validateTransition(status, "approved"); err != nil {
		return fmt.Errorf("cannot reopen: %v", err)
	}

	// Set lifecycle_status and reopen_reason
	patch := map[string]interface{}{
		"lifecycle_status": "approved",
		"reopen_reason":    reason,
	}
	patchJSON, _ := json.Marshal(patch)
	_, err = c.UpdateMetadata(ctx, id, json.RawMessage(patchJSON))
	return err
}

// LinkTask creates an implements edge from task to requirement
func (c *Client) LinkTask(ctx context.Context, reqID, taskID string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	// Validate task shard exists and is type 'task'
	var shardType string
	err = conn.QueryRow(ctx, `SELECT type FROM shards WHERE id = $1`, taskID).Scan(&shardType)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("shard not found: %s", taskID)
	}
	if err != nil {
		return fmt.Errorf("failed to fetch shard: %v", err)
	}
	if shardType != "task" {
		return fmt.Errorf("shard %s is type '%s', expected 'task'", taskID, shardType)
	}

	// Validate requirement exists
	status, err := c.getLifecycleStatus(ctx, conn, reqID)
	if err != nil {
		return err
	}

	// Create edge: task --implements--> requirement
	_, err = conn.Exec(ctx, `SELECT link($1, $2, $3)`, taskID, reqID, "implements")
	if err != nil {
		return fmt.Errorf("failed to link task: %v", err)
	}

	// Auto-transition: approved → in_progress
	if status == "approved" {
		_, err = c.SetMetadataPath(ctx, reqID, []string{"lifecycle_status"}, json.RawMessage(`"in_progress"`))
		if err != nil {
			return fmt.Errorf("linked task but failed to update lifecycle: %v", err)
		}
	}

	return nil
}

// LinkTest creates a has-artifact edge from test to requirement
func (c *Client) LinkTest(ctx context.Context, reqID, testID string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	// Validate test shard exists
	var shardType string
	err = conn.QueryRow(ctx, `SELECT type FROM shards WHERE id = $1`, testID).Scan(&shardType)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("shard not found: %s", testID)
	}
	if err != nil {
		return fmt.Errorf("failed to fetch shard: %v", err)
	}

	// Validate requirement exists
	_, err = c.getLifecycleStatus(ctx, conn, reqID)
	if err != nil {
		return err
	}

	// Create edge: test --has-artifact--> requirement
	_, err = conn.Exec(ctx, `SELECT link($1, $2, $3)`, testID, reqID, "has-artifact")
	if err != nil {
		return fmt.Errorf("failed to link test: %v", err)
	}

	return nil
}

// LinkDependency creates a blocked-by edge between requirements
func (c *Client) LinkDependency(ctx context.Context, reqID, dependsOnID string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	// Validate both are requirements
	_, err = c.getLifecycleStatus(ctx, conn, reqID)
	if err != nil {
		return err
	}
	_, err = c.getLifecycleStatus(ctx, conn, dependsOnID)
	if err != nil {
		return err
	}

	// Check for circular dependency
	var hasCycle bool
	err = conn.QueryRow(ctx, `SELECT has_circular_dependency($1, $2)`, reqID, dependsOnID).Scan(&hasCycle)
	if err != nil {
		return fmt.Errorf("failed to check circular dependency: %v", err)
	}
	if hasCycle {
		return fmt.Errorf("circular dependency detected: adding this edge would create a cycle")
	}

	// Create edge: reqID --blocked-by--> dependsOnID
	_, err = conn.Exec(ctx, `SELECT link($1, $2, $3)`, reqID, dependsOnID, "blocked-by")
	if err != nil {
		return fmt.Errorf("failed to link dependency: %v", err)
	}

	return nil
}

// UnlinkEdge removes an edge between a requirement and another shard
func (c *Client) UnlinkEdge(ctx context.Context, reqID, targetID, edgeType string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	var fromID, toID string
	switch edgeType {
	case "implements":
		// task --implements--> requirement
		fromID, toID = targetID, reqID
	case "has-artifact":
		// test --has-artifact--> requirement
		fromID, toID = targetID, reqID
	case "blocked-by":
		// requirement --blocked-by--> dependency
		fromID, toID = reqID, targetID
	default:
		return fmt.Errorf("unknown edge type: %s", edgeType)
	}

	result, err := conn.Exec(ctx, `
		DELETE FROM edges WHERE from_id = $1 AND to_id = $2 AND edge_type = $3
	`, fromID, toID, edgeType)
	if err != nil {
		return fmt.Errorf("failed to unlink: %v", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("edge not found: %s --%s--> %s", fromID, edgeType, toID)
	}

	return nil
}

// RequirementDashboard calls the requirement_dashboard() SQL function
func (c *Client) RequirementDashboard(ctx context.Context) ([]RequirementDashboardRow, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `SELECT * FROM requirement_dashboard($1)`, c.Config.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to get requirement dashboard: %v", err)
	}
	defer rows.Close()

	var results []RequirementDashboardRow
	for rows.Next() {
		var r RequirementDashboardRow
		if err := rows.Scan(&r.ID, &r.Title, &r.LifecycleStatus, &r.Priority,
			&r.Category, &r.TaskCountTotal, &r.TaskCountClosed, &r.TestCount,
			&r.BlockedByIDs, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan dashboard row: %v", err)
		}
		results = append(results, r)
	}
	return results, nil
}

// GetRequirementEdges fetches all edges connected to a requirement with shard titles
func (c *Client) GetRequirementEdges(ctx context.Context, id string) ([]RequirementEdge, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT e.from_id, e.to_id, e.edge_type, s.title,
			COALESCE(s.metadata->>'lifecycle_status', '')
		FROM edges e
		JOIN shards s ON s.id = CASE WHEN e.from_id = $1 THEN e.to_id ELSE e.from_id END
		WHERE e.from_id = $1 OR e.to_id = $1
		ORDER BY e.edge_type, s.title
	`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get requirement edges: %v", err)
	}
	defer rows.Close()

	var edges []RequirementEdge
	for rows.Next() {
		var edge RequirementEdge
		if err := rows.Scan(&edge.FromID, &edge.ToID, &edge.EdgeType, &edge.Title, &edge.LifecycleStatus); err != nil {
			continue
		}
		edges = append(edges, edge)
	}
	return edges, nil
}
