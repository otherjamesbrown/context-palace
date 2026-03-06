package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
)

// parseOrderFlag parses "--order pf-a:pf-b,pf-c:pf-b" into OrderEdge pairs
func parseOrderFlag(s string, adoptIDs []string) ([]client.OrderEdge, error) {
	adoptSet := make(map[string]bool)
	for _, id := range adoptIDs {
		adoptSet[id] = true
	}

	var edges []client.OrderEdge
	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("Invalid order format '%s', expected 'child:blocker'", pair)
		}
		from := strings.TrimSpace(parts[0])
		blockedBy := strings.TrimSpace(parts[1])

		if from == blockedBy {
			return nil, fmt.Errorf("Shard cannot block itself: %s", from)
		}

		if len(adoptIDs) > 0 {
			if !adoptSet[from] {
				return nil, fmt.Errorf("Shard %s not in adopt list", from)
			}
			if !adoptSet[blockedBy] {
				return nil, fmt.Errorf("Shard %s not in adopt list", blockedBy)
			}
		}

		edges = append(edges, client.OrderEdge{From: from, BlockedBy: blockedBy})
	}

	// Check for simple circular deps
	blocksMap := make(map[string][]string)
	for _, e := range edges {
		blocksMap[e.From] = append(blocksMap[e.From], e.BlockedBy)
	}
	for _, e := range edges {
		for _, b := range blocksMap[e.BlockedBy] {
			if b == e.From {
				return nil, fmt.Errorf("Circular dependency detected: %s and %s block each other", e.From, e.BlockedBy)
			}
		}
	}

	return edges, nil
}

// renderProgressBar renders a progress bar like "███████░░░"
func renderProgressBar(completed, total int, width int) string {
	if total == 0 {
		return strings.Repeat("\u2591", width)
	}
	filled := (completed * width) / total
	if filled > width {
		filled = width
	}
	return strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", width-filled)
}

// shortAgent strips "agent-" prefix for display
func shortAgent(agent string) string {
	return strings.TrimPrefix(agent, "agent-")
}

// timeAgo returns a human-readable time difference
func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}
