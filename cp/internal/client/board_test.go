package client

import (
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
