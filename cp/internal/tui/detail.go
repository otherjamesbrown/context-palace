package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/otherjamesbrown/context-palace/cp/internal/client"
)

// RenderDetail renders a shard detail view for the right pane
func RenderDetail(d *client.ShardDetailResult, width int, styles Styles) string {
	if width < 10 {
		return ""
	}

	contentWidth := width - 2 // padding
	var b strings.Builder

	// Title
	b.WriteString(styles.DetailTitle.Width(contentWidth).Render(d.Title))
	b.WriteString("\n")

	// Separator
	b.WriteString(styles.Muted.Render(strings.Repeat("─", min(contentWidth, 40))))
	b.WriteString("\n\n")

	// Metadata fields
	writeField := func(label, value string) {
		b.WriteString(styles.DetailLabel.Render(fmt.Sprintf("%-10s", label)))
		b.WriteString(styles.DetailValue.Render(value))
		b.WriteString("\n")
	}

	writeField("ID:", d.ID)
	writeField("Type:", fmt.Sprintf("%s %s", TypeIcon(d.Type), d.Type))
	writeField("Status:", renderStatus(d.Status, styles))
	writeField("Creator:", d.Creator)

	if len(d.Labels) > 0 {
		writeField("Labels:", strings.Join(d.Labels, ", "))
	}

	writeField("Created:", d.CreatedAt.Format("2006-01-02 15:04"))
	writeField("Updated:", d.UpdatedAt.Format("2006-01-02 15:04"))

	if d.OutgoingEdgeCount > 0 || d.IncomingEdgeCount > 0 {
		writeField("Edges:", fmt.Sprintf("%d out, %d in", d.OutgoingEdgeCount, d.IncomingEdgeCount))
	}

	// Usage data (knowledge shards only)
	usage := parseUsageData(d)
	if usage != nil {
		writeField("Reads:", fmt.Sprintf("%d", usage.AccessCount))
		if usage.LastAccessed != "" && len(usage.AccessLog) > 0 {
			last := usage.AccessLog[len(usage.AccessLog)-1]
			writeField("Last read:", fmt.Sprintf("%s by %s", formatAccessTime(last.At), last.By))
		} else if usage.LastAccessed != "" {
			writeField("Last read:", usage.LastAccessed)
		}
	}

	// Content
	if d.Content != "" {
		b.WriteString("\n")
		b.WriteString(styles.Muted.Render(strings.Repeat("─", min(contentWidth, 40))))
		b.WriteString("\n\n")

		// Word-wrap content to width
		content := wordWrap(d.Content, contentWidth)
		b.WriteString(styles.DetailContent.Render(content))
	}

	// Knowledge children
	if len(d.KnowledgeChildren) > 0 {
		b.WriteString("\n\n")
		b.WriteString(styles.ChildrenHeader.Render(fmt.Sprintf("Children (%d)", len(d.KnowledgeChildren))))
		b.WriteString("\n")
		b.WriteString(styles.Muted.Render(strings.Repeat("─", min(contentWidth, 40))))
		b.WriteString("\n\n")

		for _, ch := range d.KnowledgeChildren {
			// Child ID and title
			b.WriteString(styles.ChildID.Render(ch.ID))
			if ch.Title != "" {
				b.WriteString(styles.DetailValue.Render("  " + ch.Title))
			}
			b.WriteString("\n")
			// Description
			if ch.Description != "" {
				desc := wordWrap(ch.Description, contentWidth-2)
				b.WriteString(styles.DetailContent.Render("  " + desc))
				b.WriteString("\n")
			}
			// Trigger
			if ch.Trigger != "" {
				trigger := wordWrap(ch.Trigger, contentWidth-4)
				b.WriteString(styles.ChildTrigger.Render("  → " + trigger))
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}

	// Recent access log
	if usage != nil && len(usage.AccessLog) > 0 {
		b.WriteString("\n")
		b.WriteString(styles.UsageHeader.Render("Recent Access"))
		b.WriteString("\n")
		b.WriteString(styles.Muted.Render(strings.Repeat("─", min(contentWidth, 40))))
		b.WriteString("\n\n")

		// Show last 5 entries, most recent first
		start := len(usage.AccessLog) - 5
		if start < 0 {
			start = 0
		}
		for i := len(usage.AccessLog) - 1; i >= start; i-- {
			entry := usage.AccessLog[i]
			b.WriteString(styles.DetailValue.Render(fmt.Sprintf("%-18s", formatAccessTime(entry.At))))
			b.WriteString(styles.Muted.Render(entry.By))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// RenderLoading renders a loading indicator for a shard
func RenderLoading(id string, styles Styles) string {
	return styles.Muted.Render(fmt.Sprintf("Loading %s...", id))
}

// RenderEmpty renders an empty detail pane message
func RenderEmpty(styles Styles) string {
	return styles.Muted.Render("Select a shard to view details")
}

func renderStatus(status string, styles Styles) string {
	switch status {
	case "open":
		return styles.StatusOpen.Render(status)
	case "closed":
		return styles.StatusClosed.Render(status)
	case "in_progress":
		return styles.StatusInProgress.Render(status)
	default:
		return status
	}
}

// wordWrap wraps text at the given width on word boundaries
func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	return lipgloss.NewStyle().Width(width).Render(text)
}

// usageData holds parsed access metadata from a knowledge shard
type usageData struct {
	AccessCount  int              `json:"access_count"`
	LastAccessed string           `json:"last_accessed"`
	AccessLog    []accessLogEntry `json:"access_log"`
}

type accessLogEntry struct {
	At string `json:"at"`
	By string `json:"by"`
}

// parseUsageData extracts usage info from shard metadata. Returns nil if not a
// knowledge shard or if there is no access data.
func parseUsageData(d *client.ShardDetailResult) *usageData {
	if d.Type != "knowledge" || len(d.Metadata) == 0 {
		return nil
	}
	var u usageData
	if err := json.Unmarshal(d.Metadata, &u); err != nil {
		return nil
	}
	if u.AccessCount == 0 && u.LastAccessed == "" && len(u.AccessLog) == 0 {
		return nil
	}
	return &u
}

// formatAccessTime trims a timestamp to "YYYY-MM-DD HH:MM" for display.
func formatAccessTime(ts string) string {
	if len(ts) >= 16 {
		return ts[:16]
	}
	return ts
}
