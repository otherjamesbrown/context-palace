package client

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseArea_WithSeparator(t *testing.T) {
	area, desc := ParseArea("Pipeline/Ingest - concurrency bypass")
	assert.Equal(t, "Pipeline", area)
	assert.Equal(t, "concurrency bypass", desc)
}

func TestParseArea_NestedArea(t *testing.T) {
	area, desc := ParseArea("Pipeline/Threading - metadata mismatch")
	assert.Equal(t, "Pipeline", area)
	assert.Equal(t, "metadata mismatch", desc)
}

func TestParseArea_SimpleArea(t *testing.T) {
	area, desc := ParseArea("CLI - penf ai model commands")
	assert.Equal(t, "CLI", area)
	assert.Equal(t, "penf ai model commands", desc)
}

func TestParseArea_NoSeparator(t *testing.T) {
	area, desc := ParseArea("Some random shard title")
	assert.Equal(t, "Other", area)
	assert.Equal(t, "Some random shard title", desc)
}

func TestParseArea_EmptyTitle(t *testing.T) {
	area, desc := ParseArea("")
	assert.Equal(t, "Other", area)
	assert.Equal(t, "", desc)
}

func TestParseArea_DashInDescription(t *testing.T) {
	// Only first " - " is the separator
	area, desc := ParseArea("Infra - migrate MLX to Ollama - phase 2")
	assert.Equal(t, "Infra", area)
	assert.Equal(t, "migrate MLX to Ollama - phase 2", desc)
}

func TestEstimateTokens(t *testing.T) {
	assert.Equal(t, 0, EstimateTokens(""))
	assert.Equal(t, 25, EstimateTokens("a]b]c]d]e]f]g]h]i]j]k]l]m]n]o]p]q]r]s]t]u]v]w]x]y]z]a]b]c]d]e]f]g]h]i]j]k]l]m]n]o]p]q]r]s]t]u]v]w]x]y]"))
	// 400 chars = ~100 tokens
	content := make([]byte, 400)
	for i := range content {
		content[i] = 'a'
	}
	assert.Equal(t, 100, EstimateTokens(string(content)))
}

func TestFormatBoardCompact_LargeFixture(t *testing.T) {
	// Build a fixture with 100 designs and 20 needs-review items
	designs := make([]BoardEntry, 100)
	for i := range designs {
		designs[i] = BoardEntry{
			ID:            fmt.Sprintf("cp-%06d", i),
			Type:          "design",
			Title:         fmt.Sprintf("Design item %d with a fairly long title to simulate real content", i),
			TokenEstimate: 150,
			AgeHours:      i + 1,
		}
	}

	needsReview := make([]BoardEntry, 20)
	for i := range needsReview {
		needsReview[i] = BoardEntry{
			ID:            fmt.Sprintf("cp-nr%04d", i),
			Type:          "task",
			Title:         fmt.Sprintf("Needs review task %d with some descriptive text here", i),
			TokenEstimate: 80,
			AgeHours:      i + 1,
		}
	}

	result := &BoardResult{
		Focus: []BoardEntry{
			{
				ID:            "cp-focus1",
				Type:          "task",
				Title:         "Active focus item currently in progress",
				TokenEstimate: 320,
				AgeHours:      2,
			},
		},
		FocusTokens: 320,
		NeedsReview: needsReview,
		Groups: []BoardGroup{
			{Type: "design", Items: designs, TotalTokens: 15000},
			{Type: "bug", Items: []BoardEntry{
				{ID: "cp-bug001", Type: "bug", Title: "Critical bug fix", TokenEstimate: 50, AgeHours: 5},
				{ID: "cp-bug002", Type: "bug", Title: "Minor rendering issue", TokenEstimate: 30, AgeHours: 10},
			}, TotalTokens: 80},
		},
	}

	output := FormatBoardCompact(result)

	// Must be under 2KB
	assert.Less(t, len(output), 2048, "compact output must be under 2KB, got %d bytes", len(output))

	// Must contain key sections
	assert.Contains(t, output, "# Board #")
	assert.Contains(t, output, "## Focus (1)")
	assert.Contains(t, output, "## Needs Review (20)")
	assert.Contains(t, output, "## Open Work")

	// Needs review capped at 5 shown items
	assert.Contains(t, output, "(+15 more — run `cxp board -v review`)")

	// Designs: count only
	assert.Contains(t, output, "Designs:")
	assert.Contains(t, output, "run `cxp board -v designs`")

	// Bugs: list IDs since count ≤ 10
	assert.Contains(t, output, "cp-bug001")
	assert.Contains(t, output, "cp-bug002")
}

func TestFormatBoardCompact_Empty(t *testing.T) {
	result := &BoardResult{}
	output := FormatBoardCompact(result)
	assert.Equal(t, "# Board #\n", output)
}

func TestFormatBoardCompact_BugsOverflow(t *testing.T) {
	// 11 bugs — should show count + hint, not IDs
	bugs := make([]BoardEntry, 11)
	for i := range bugs {
		bugs[i] = BoardEntry{
			ID:    fmt.Sprintf("cp-b%04d", i),
			Type:  "bug",
			Title: fmt.Sprintf("Bug %d", i),
		}
	}
	result := &BoardResult{
		Groups: []BoardGroup{
			{Type: "bug", Items: bugs},
		},
	}
	output := FormatBoardCompact(result)
	assert.Contains(t, output, "11")
	assert.NotContains(t, output, "cp-b0000")
	assert.Contains(t, output, "run `cxp board -v bugs`")
}
