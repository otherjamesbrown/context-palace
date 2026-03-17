package summary

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSummaryResponse_ValidJSON(t *testing.T) {
	input := `{"summary":"trigger text","parent_needs_update":false,"parent_edits":null}`
	result, err := ParseSummaryResponse(input)
	require.NoError(t, err)
	assert.Equal(t, "trigger text", result.Summary)
	assert.False(t, result.ParentNeedsUpdate)
	assert.Nil(t, result.ParentEdits)
}

func TestParseSummaryResponse_WithCodeFence(t *testing.T) {
	input := "```json\n{\"summary\":\"test\",\"parent_needs_update\":false}\n```"
	result, err := ParseSummaryResponse(input)
	require.NoError(t, err)
	assert.Equal(t, "test", result.Summary)
}

func TestParseSummaryResponse_WithPlainFence(t *testing.T) {
	input := "```\n{\"summary\":\"test\",\"parent_needs_update\":false}\n```"
	result, err := ParseSummaryResponse(input)
	require.NoError(t, err)
	assert.Equal(t, "test", result.Summary)
}

func TestParseSummaryResponse_EmptySummary(t *testing.T) {
	input := `{"summary":"","parent_needs_update":false}`
	_, err := ParseSummaryResponse(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AI returned empty summary")
}

func TestParseSummaryResponse_InvalidJSON(t *testing.T) {
	_, err := ParseSummaryResponse("not json at all")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse AI response as JSON")
}

func TestParseSummaryResponse_WithParentEdits(t *testing.T) {
	input := `{"summary":"trigger","parent_needs_update":true,"parent_edits":"Change X to Y"}`
	result, err := ParseSummaryResponse(input)
	require.NoError(t, err)
	assert.True(t, result.ParentNeedsUpdate)
	require.NotNil(t, result.ParentEdits)
	assert.Equal(t, "Change X to Y", *result.ParentEdits)
}

func TestParseSummaryResponse_NestedFences(t *testing.T) {
	input := "```json\n{\"summary\":\"has ``` inside\",\"parent_needs_update\":false}\n```"
	result, err := ParseSummaryResponse(input)
	require.NoError(t, err)
	assert.Contains(t, result.Summary, "```")
}

func TestParseSummaryResponse_MinimalFence(t *testing.T) {
	input := "```json\n```"
	_, err := ParseSummaryResponse(input)
	require.Error(t, err)
}

func TestBuildSummaryPrompt_StripsSubMemories(t *testing.T) {
	parentContent := "Parent prose\n\n<!-- sub-memories -->\n[{\"id\":\"x\",\"title\":\"T\",\"summary\":\"S\"}]\n<!-- /sub-memories -->\n"
	result := BuildSummaryPrompt("abc", parentContent, "Child", "child body")
	assert.NotContains(t, result, "<!-- sub-memories -->")
	assert.NotContains(t, result, "<!-- /sub-memories -->")
	assert.Contains(t, result, "Parent prose")
}

func TestBuildSummaryPrompt_ContainsAllFields(t *testing.T) {
	result := BuildSummaryPrompt("abc", "Parent content", "My Child", "child body text")
	assert.Contains(t, result, "PARENT MEMORY (ID: abc)")
	assert.Contains(t, result, "NEW CHILD MEMORY (title: My Child)")
	assert.Contains(t, result, "Parent content")
	assert.Contains(t, result, "child body text")
	assert.Contains(t, result, "---")
}
