package client

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// BoardEntry represents a single shard in the board view
type BoardEntry struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Title          string `json:"title"`
	Area           string `json:"area"`
	Description    string `json:"description"`
	Status         string `json:"status"`
	Priority       *int   `json:"priority,omitempty"`
	TokenEstimate  int    `json:"token_estimate"`
	Creator        string `json:"creator,omitempty"`
}

// BoardGroup represents a group of shards by area
type BoardGroup struct {
	Area          string       `json:"area"`
	Items         []BoardEntry `json:"items"`
	TotalTokens   int          `json:"total_tokens"`
}

// BoardResult holds the full board output
type BoardResult struct {
	Groups       []BoardGroup `json:"groups"`
	UnreadCount  int          `json:"unread_count"`
	MemoryCount  int          `json:"memory_count"`
}

// ParseArea extracts area and description from a structured title.
// Format: "[area] - [description]" e.g. "Pipeline/Ingest - concurrency bypass"
// If no separator found, area is "Other" and description is the full title.
func ParseArea(title string) (area, description string) {
	if idx := strings.Index(title, " - "); idx >= 0 {
		area = strings.TrimSpace(title[:idx])
		description = strings.TrimSpace(title[idx+3:])
		// Use top-level area for grouping (before first /)
		if slashIdx := strings.Index(area, "/"); slashIdx >= 0 {
			area = area[:slashIdx]
		}
		return area, description
	}
	return "Other", title
}

// EstimateTokens returns an approximate token count for content.
func EstimateTokens(content string) int {
	return len(content) / 4
}

// BoardOpts holds options for GetBoardShards
type BoardOpts struct {
	Since  *time.Time // Include shards closed after this time
	Area   string     // Filter to area prefix
	Agent  string     // Filter by creator
	Budget int        // Token budget highlight threshold
}

// GetBoardShards returns shards grouped by area for the board view
func (c *Client) GetBoardShards(ctx context.Context, opts BoardOpts) (*BoardResult, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	// Build query: open shards + optionally recently closed
	query := `
		SELECT id, type, title, status, priority, COALESCE(LENGTH(content), 0), creator
		FROM shards
		WHERE project = $1
		  AND type NOT IN ('session', 'memory', 'message')
	`
	args := []interface{}{c.Config.Project}
	argN := 2

	if opts.Since != nil {
		query += fmt.Sprintf(` AND (status IN ('open', 'in_progress') OR (status = 'closed' AND closed_at >= $%d))`, argN)
		args = append(args, *opts.Since)
		argN++
	} else {
		query += ` AND status IN ('open', 'in_progress')`
	}

	if opts.Agent != "" {
		query += fmt.Sprintf(` AND creator = $%d`, argN)
		args = append(args, opts.Agent)
		argN++
	}

	query += ` ORDER BY priority NULLS LAST, updated_at DESC`

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query board shards: %v", err)
	}
	defer rows.Close()

	// Collect entries and group by area
	groupMap := make(map[string]*BoardGroup)
	var groupOrder []string

	for rows.Next() {
		var e BoardEntry
		var contentLen int
		if err := rows.Scan(&e.ID, &e.Type, &e.Title, &e.Status, &e.Priority, &contentLen, &e.Creator); err != nil {
			continue
		}
		e.TokenEstimate = contentLen / 4
		e.Area, e.Description = ParseArea(e.Title)

		// Filter by area if specified
		if opts.Area != "" && !strings.EqualFold(e.Area, opts.Area) {
			continue
		}

		g, exists := groupMap[e.Area]
		if !exists {
			g = &BoardGroup{Area: e.Area}
			groupMap[e.Area] = g
			groupOrder = append(groupOrder, e.Area)
		}
		g.Items = append(g.Items, e)
		g.TotalTokens += e.TokenEstimate
	}

	// Build ordered groups
	var groups []BoardGroup
	for _, area := range groupOrder {
		groups = append(groups, *groupMap[area])
	}

	// Get inbox count and memory count
	result := &BoardResult{Groups: groups}

	inboxCount, err := c.InboxCount(ctx)
	if err == nil {
		result.UnreadCount = inboxCount
	}

	var memCount int
	_ = conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM shards WHERE project = $1 AND type = 'memory' AND status = 'open'`,
		c.Config.Project).Scan(&memCount)
	result.MemoryCount = memCount

	return result, nil
}
