package client

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// BoardEntry represents a single shard in the board view
type BoardEntry struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Title         string    `json:"title"`
	Area          string    `json:"area"`
	Description   string    `json:"description"`
	Status        string    `json:"status"`
	Priority      *int      `json:"priority,omitempty"`
	TokenEstimate int       `json:"token_estimate"`
	Creator       string    `json:"creator,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
	AgeHours      int       `json:"age_hours"`
	ChildCount    int       `json:"child_count,omitempty"`
	HasFocus      bool      `json:"has_focus,omitempty"`
	NeedsReview   bool      `json:"needs_review,omitempty"`
	IsBlocked     bool      `json:"is_blocked,omitempty"`
}

// BoardGroup represents a group of shards by type
type BoardGroup struct {
	Type        string       `json:"type"`
	Items       []BoardEntry `json:"items"`
	TotalTokens int          `json:"total_tokens"`
}

// BoardResult holds the full board output
type BoardResult struct {
	Focus             []BoardEntry `json:"focus,omitempty"`
	FocusTokens       int          `json:"focus_tokens,omitempty"`
	NeedsReview       []BoardEntry `json:"needs_review,omitempty"`
	NeedsReviewTokens int          `json:"needs_review_tokens,omitempty"`
	Blocked           []BoardEntry `json:"blocked,omitempty"`
	BlockedTokens     int          `json:"blocked_tokens,omitempty"`
	RecentActivity    []BoardEntry `json:"recent_activity,omitempty"`
	RecentTokens      int          `json:"recent_tokens,omitempty"`
	Groups            []BoardGroup `json:"groups"`
	UnreadCount       int          `json:"unread_count"`
	MemoryCount       int          `json:"memory_count"`
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

// FormatAgeHours formats hours as -Nh string
func FormatAgeHours(hours int) string {
	return fmt.Sprintf("-%dh", hours)
}

// DefaultBoardTypes are the actionable shard types shown by default
var DefaultBoardTypes = []string{"bug", "task", "design", "doc"}

// RecentActivityWindow is the lookback window from the most recent shard update
const RecentActivityWindow = 4 * time.Hour

// BoardOpts holds options for GetBoardShards
type BoardOpts struct {
	Since  *time.Time // Include shards closed after this time
	Area   string     // Filter to area prefix
	Agent  string     // Filter by creator
	Budget int        // Token budget highlight threshold
	Types  []string   // Shard types to include (default: DefaultBoardTypes)
	All    bool       // Show all types (overrides Types)
}

// GetBoardShards returns shards grouped by type for the board view
func (c *Client) GetBoardShards(ctx context.Context, opts BoardOpts) (*BoardResult, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	// Determine which types to show
	types := opts.Types
	if opts.All {
		types = nil
	} else if len(types) == 0 {
		types = DefaultBoardTypes
	}

	// Build query: include focus label check via subquery
	query := `
		SELECT s.id, s.type, s.title, s.status, s.priority,
			COALESCE(LENGTH(s.content), 0), s.creator, s.updated_at,
			('focus' = ANY(s.labels) OR EXISTS(SELECT 1 FROM labels l WHERE l.shard_id = s.id AND l.label = 'focus')) AS has_focus,
			('needs-review' = ANY(s.labels) OR EXISTS(SELECT 1 FROM labels l WHERE l.shard_id = s.id AND l.label = 'needs-review')) AS needs_review,
			('blocked' = ANY(s.labels) OR EXISTS(SELECT 1 FROM labels l WHERE l.shard_id = s.id AND l.label = 'blocked')) AS is_blocked,
			(SELECT COUNT(*) FROM edges e WHERE e.to_id = s.id AND e.edge_type = 'child-of') +
			(SELECT COUNT(*) FROM shards c WHERE c.parent_id = s.id AND c.project = s.project) AS child_count
		FROM shards s
		WHERE s.project = $1
		  AND s.type NOT IN ('session', 'memory', 'message')
	`
	args := []interface{}{c.Config.Project}
	argN := 2

	if len(types) > 0 {
		query += fmt.Sprintf(` AND s.type = ANY($%d)`, argN)
		args = append(args, types)
		argN++
	}

	if opts.Since != nil {
		query += fmt.Sprintf(` AND (s.status IN ('open', 'ready', 'in_progress', 'needs-review') OR (s.status = 'closed' AND s.closed_at >= $%d))`, argN)
		args = append(args, *opts.Since)
		argN++
	} else {
		query += ` AND s.status IN ('open', 'ready', 'in_progress', 'needs-review')`
	}

	if opts.Agent != "" {
		query += fmt.Sprintf(` AND s.creator = $%d`, argN)
		args = append(args, opts.Agent)
		argN++
	}

	query += ` ORDER BY s.type, s.updated_at DESC`

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query board shards: %v", err)
	}
	defer rows.Close()

	now := time.Now()

	// Collect all entries
	var allEntries []BoardEntry
	var maxUpdated time.Time
	for rows.Next() {
		var e BoardEntry
		var contentLen int
		if err := rows.Scan(&e.ID, &e.Type, &e.Title, &e.Status, &e.Priority,
			&contentLen, &e.Creator, &e.UpdatedAt, &e.HasFocus, &e.NeedsReview, &e.IsBlocked, &e.ChildCount); err != nil {
			continue
		}
		e.TokenEstimate = contentLen / 4
		e.AgeHours = int(math.Ceil(now.Sub(e.UpdatedAt).Hours()))
		e.Area, e.Description = ParseArea(e.Title)

		// Filter by area if specified
		if opts.Area != "" && !strings.EqualFold(e.Area, opts.Area) {
			continue
		}

		if e.UpdatedAt.After(maxUpdated) {
			maxUpdated = e.UpdatedAt
		}
		allEntries = append(allEntries, e)
	}

	// Recent activity: everything within RecentActivityWindow of the most recent update
	recentCutoff := maxUpdated.Add(-RecentActivityWindow)

	// Partition into focus, needs-review, blocked, recent, and type groups
	result := &BoardResult{}
	focusSeen := make(map[string]bool)
	needsReviewSeen := make(map[string]bool)
	blockedSeen := make(map[string]bool)
	recentSeen := make(map[string]bool)

	// 1. Focus section
	for _, e := range allEntries {
		if e.HasFocus {
			result.Focus = append(result.Focus, e)
			result.FocusTokens += e.TokenEstimate
			focusSeen[e.ID] = true
		}
	}

	// 2. Needs Review section (skip if already in focus)
	for _, e := range allEntries {
		if focusSeen[e.ID] {
			continue
		}
		if e.NeedsReview || e.Status == "needs-review" {
			result.NeedsReview = append(result.NeedsReview, e)
			result.NeedsReviewTokens += e.TokenEstimate
			needsReviewSeen[e.ID] = true
		}
	}

	// 3. Blocked section (skip if already in focus or needs-review)
	for _, e := range allEntries {
		if focusSeen[e.ID] || needsReviewSeen[e.ID] {
			continue
		}
		if e.IsBlocked {
			result.Blocked = append(result.Blocked, e)
			result.BlockedTokens += e.TokenEstimate
			blockedSeen[e.ID] = true
		}
	}

	// 4. Recent activity section (exclude items in any above section)
	for _, e := range allEntries {
		if focusSeen[e.ID] || needsReviewSeen[e.ID] || blockedSeen[e.ID] {
			continue
		}
		if e.UpdatedAt.After(recentCutoff) {
			result.RecentActivity = append(result.RecentActivity, e)
			result.RecentTokens += e.TokenEstimate
			recentSeen[e.ID] = true
		}
	}

	// 5. Type groups (exclude items already shown in any above section)
	groupMap := make(map[string]*BoardGroup)
	var groupOrder []string
	for _, e := range allEntries {
		if focusSeen[e.ID] || needsReviewSeen[e.ID] || blockedSeen[e.ID] || recentSeen[e.ID] {
			continue
		}
		g, exists := groupMap[e.Type]
		if !exists {
			g = &BoardGroup{Type: e.Type}
			groupMap[e.Type] = g
			groupOrder = append(groupOrder, e.Type)
		}
		g.Items = append(g.Items, e)
		g.TotalTokens += e.TokenEstimate
	}
	for _, typ := range groupOrder {
		result.Groups = append(result.Groups, *groupMap[typ])
	}

	// Get inbox count and memory count
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
