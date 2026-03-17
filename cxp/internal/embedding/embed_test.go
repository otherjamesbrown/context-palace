package embedding

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildEmbeddingText_AllFields(t *testing.T) {
	result := BuildEmbeddingText("note", "My Title", "Body text")
	assert.Equal(t, "note: My Title\n\nBody text", result)
}

func TestBuildEmbeddingText_NoType(t *testing.T) {
	result := BuildEmbeddingText("", "My Title", "Body text")
	assert.Equal(t, "My Title\n\nBody text", result)
}

func TestBuildEmbeddingText_NoContent(t *testing.T) {
	result := BuildEmbeddingText("note", "My Title", "")
	assert.Equal(t, "note: My Title", result)
}

func TestBuildEmbeddingText_WhitespaceContent(t *testing.T) {
	result := BuildEmbeddingText("note", "My Title", "  \n  ")
	assert.Equal(t, "note: My Title", result)
}

func TestBuildEmbeddingText_Truncation(t *testing.T) {
	longContent := strings.Repeat("x", 40000)
	result := BuildEmbeddingText("", "T", longContent)
	assert.Equal(t, 32000, len(result))
}

func TestBuildEmbeddingText_TypeOnly(t *testing.T) {
	result := BuildEmbeddingText("note", "", "")
	assert.Equal(t, "note: ", result)
}

func TestBuildEmbeddingText_Empty(t *testing.T) {
	result := BuildEmbeddingText("", "", "")
	assert.Equal(t, "", result)
}

func TestBuildEmbeddingText_ExactBoundary(t *testing.T) {
	// Content that results in exactly 32000 chars should not be truncated
	prefix := "T\n\n" // 3 chars for title + newlines
	content := strings.Repeat("x", 32000-len(prefix))
	result := BuildEmbeddingText("", "T", content)
	assert.Equal(t, 32000, len(result))
}

func TestFormatDimensionError_Message(t *testing.T) {
	err := FormatDimensionError(768, 384)
	assert.Contains(t, err.Error(), "expected 768")
	assert.Contains(t, err.Error(), "returns 384")
}
