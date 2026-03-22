package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// DashboardData holds all sections of the dashboard output
type DashboardData struct {
	Outcomes   []OutcomeStatus   `json:"outcomes"`
	Pipelines  []PipelineStatus  `json:"pipelines"`
	Blockers   []BlockerStatus   `json:"blockers"`
	Agents     []AgentStatus     `json:"agents"`
}

// OutcomeStatus holds an outcome with its design progress
type OutcomeStatus struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	DesignTotal   int    `json:"design_total"`
	DesignClosed  int    `json:"design_closed"`
	HasProgress   bool   `json:"has_progress"`
}

// PipelineStatus holds an active pipeline (design with pipeline metadata)
type PipelineStatus struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
	Phase string `json:"phase"`
	Note  string `json:"note,omitempty"`
}

// BlockerStatus holds a blocked shard
type BlockerStatus struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// AgentStatus holds agent activity info
type AgentStatus struct {
	Agent    string `json:"agent"`
	TaskID   string `json:"task_id,omitempty"`
	TaskTitle string `json:"task_title,omitempty"`
	Duration string `json:"duration,omitempty"`
	Idle     bool   `json:"idle"`
}

// GetDashboardData fetches all data needed for the dashboard view
func (c *Client) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	data := &DashboardData{}

	// 1. OUTCOMES: query open outcomes and count child designs
	outcomes, err := c.getDashboardOutcomes(ctx)
	if err != nil {
		return nil, fmt.Errorf("outcomes: %w", err)
	}
	data.Outcomes = outcomes

	// 2. ACTIVE PIPELINES: designs with pipeline metadata
	pipelines, err := c.getDashboardPipelines(ctx)
	if err != nil {
		return nil, fmt.Errorf("pipelines: %w", err)
	}
	data.Pipelines = pipelines

	// 3. BLOCKERS: shards with "blocked" label
	blockers, err := c.getDashboardBlockers(ctx)
	if err != nil {
		return nil, fmt.Errorf("blockers: %w", err)
	}
	data.Blockers = blockers

	// 4. AGENTS: in_progress tasks grouped by owner
	agents, err := c.getDashboardAgents(ctx)
	if err != nil {
		return nil, fmt.Errorf("agents: %w", err)
	}
	data.Agents = agents

	return data, nil
}

// getDashboardOutcomes fetches open outcomes and counts their child designs
func (c *Client) getDashboardOutcomes(ctx context.Context) ([]OutcomeStatus, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	// Get open outcomes with child design counts via edges
	rows, err := conn.Query(ctx, `
		SELECT
			o.id,
			o.title,
			COUNT(d.id) AS design_total,
			COUNT(d.id) FILTER (WHERE d.status = 'closed') AS design_closed
		FROM shards o
		LEFT JOIN edges e ON e.to_id = o.id AND e.edge_type = 'child-of'
		LEFT JOIN shards d ON d.id = e.from_id AND d.type = 'design'
		WHERE o.project = $1
			AND o.type = 'outcome'
			AND o.status IN ('open', 'ready', 'in_progress', 'needs-review')
		GROUP BY o.id, o.title
		ORDER BY o.title
	`, c.Config.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to query outcomes: %v", err)
	}
	defer rows.Close()

	var outcomes []OutcomeStatus
	for rows.Next() {
		var o OutcomeStatus
		if err := rows.Scan(&o.ID, &o.Title, &o.DesignTotal, &o.DesignClosed); err != nil {
			return nil, fmt.Errorf("failed to scan outcome: %v", err)
		}
		o.HasProgress = o.DesignClosed > 0
		outcomes = append(outcomes, o)
	}
	return outcomes, rows.Err()
}

// getDashboardPipelines fetches designs that have pipeline metadata
func (c *Client) getDashboardPipelines(ctx context.Context) ([]PipelineStatus, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT id, title, type, COALESCE(metadata, '{}')
		FROM shards
		WHERE project = $1
			AND type = 'design'
			AND status IN ('open', 'ready', 'in_progress', 'needs-review')
			AND metadata ? 'pipeline'
		ORDER BY updated_at DESC
		LIMIT 50
	`, c.Config.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to query pipelines: %v", err)
	}
	defer rows.Close()

	var pipelines []PipelineStatus
	for rows.Next() {
		var id, title, shardType string
		var meta json.RawMessage
		if err := rows.Scan(&id, &title, &shardType, &meta); err != nil {
			return nil, fmt.Errorf("failed to scan pipeline: %v", err)
		}

		phase, note := extractPipelineInfo(meta)
		pipelines = append(pipelines, PipelineStatus{
			ID:    id,
			Title: title,
			Type:  shardType,
			Phase: phase,
			Note:  note,
		})
	}
	return pipelines, rows.Err()
}

// extractPipelineInfo extracts phase and note from pipeline metadata
func extractPipelineInfo(meta json.RawMessage) (string, string) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(meta, &m); err != nil {
		return "", ""
	}
	pipelineRaw, ok := m["pipeline"]
	if !ok {
		return "", ""
	}
	var pipeline map[string]interface{}
	if err := json.Unmarshal(pipelineRaw, &pipeline); err != nil {
		return "", ""
	}

	phase, _ := pipeline["phase"].(string)
	note, _ := pipeline["note"].(string)
	return phase, note
}

// getDashboardBlockers fetches shards with the "blocked" label
func (c *Client) getDashboardBlockers(ctx context.Context) ([]BlockerStatus, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT s.id, s.title, s.type
		FROM shards s
		JOIN labels l ON l.shard_id = s.id AND l.label = 'blocked'
		WHERE s.project = $1
			AND s.status IN ('open', 'ready', 'in_progress', 'needs-review')
		ORDER BY s.updated_at DESC
		LIMIT 50
	`, c.Config.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to query blockers: %v", err)
	}
	defer rows.Close()

	var blockers []BlockerStatus
	for rows.Next() {
		var b BlockerStatus
		if err := rows.Scan(&b.ID, &b.Title, &b.Type); err != nil {
			return nil, fmt.Errorf("failed to scan blocker: %v", err)
		}
		blockers = append(blockers, b)
	}
	return blockers, rows.Err()
}

// getDashboardAgents fetches in_progress tasks grouped by owner
func (c *Client) getDashboardAgents(ctx context.Context) ([]AgentStatus, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	// Get all in_progress tasks with their owners
	rows, err := conn.Query(ctx, `
		SELECT COALESCE(owner, creator) AS agent, id, title, updated_at
		FROM shards
		WHERE project = $1
			AND type = 'task'
			AND status = 'in_progress'
		ORDER BY agent, updated_at DESC
	`, c.Config.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent tasks: %v", err)
	}
	defer rows.Close()

	// Track which agents have tasks
	agentTasks := make(map[string]AgentStatus)
	var agentOrder []string

	for rows.Next() {
		var agent, id, title string
		var updatedAt time.Time
		if err := rows.Scan(&agent, &id, &title, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan agent task: %v", err)
		}

		// Only keep the most recent task per agent
		if _, exists := agentTasks[agent]; !exists {
			duration := time.Since(updatedAt)
			agentTasks[agent] = AgentStatus{
				Agent:     agent,
				TaskID:    id,
				TaskTitle: title,
				Duration:  formatDuration(duration),
				Idle:      false,
			}
			agentOrder = append(agentOrder, agent)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Also get known named agents who are idle (filter out ephemeral agent-pf-* identities)
	idleRows, err := conn.Query(ctx, `
		SELECT DISTINCT COALESCE(owner, creator) AS agent
		FROM shards
		WHERE project = $1
			AND COALESCE(owner, creator) IS NOT NULL
			AND COALESCE(owner, creator) ~ '^agent-(steve|mycroft|penfold)$'
			AND COALESCE(owner, creator) NOT IN (
				SELECT DISTINCT COALESCE(owner, creator)
				FROM shards
				WHERE project = $1
					AND type = 'task'
					AND status = 'in_progress'
			)
		ORDER BY agent
	`, c.Config.Project)
	if err != nil {
		// Non-fatal: just skip idle agents
		var agents []AgentStatus
		for _, name := range agentOrder {
			agents = append(agents, agentTasks[name])
		}
		return agents, nil
	}
	defer idleRows.Close()

	for idleRows.Next() {
		var agent string
		if err := idleRows.Scan(&agent); err != nil {
			continue
		}
		if _, exists := agentTasks[agent]; !exists {
			agentTasks[agent] = AgentStatus{
				Agent: agent,
				Idle:  true,
			}
			agentOrder = append(agentOrder, agent)
		}
	}

	var agents []AgentStatus
	for _, name := range agentOrder {
		agents = append(agents, agentTasks[name])
	}
	return agents, nil
}

// formatDuration formats a duration in a human-friendly way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "<1 min"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	}
	days := int(d.Hours()) / 24
	return fmt.Sprintf("%dd", days)
}
