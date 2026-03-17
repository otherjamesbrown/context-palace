package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatProgressNote_Basic(t *testing.T) {
	result := FormatProgressNote("2024-01-15 10:30:00", "agent-test", "Started work")
	assert.Equal(t, "\n\n---\n**[2024-01-15 10:30:00] agent-test:** Started work", result)
}

func TestFormatProgressNote_EmptyNote(t *testing.T) {
	result := FormatProgressNote("2024-01-15 10:30:00", "agent-test", "")
	assert.Equal(t, "\n\n---\n**[2024-01-15 10:30:00] agent-test:** ", result)
}

func TestFormatProgressNote_MultilineNote(t *testing.T) {
	result := FormatProgressNote("2024-01-15 10:30:00", "agent-test", "Line1\nLine2\nLine3")
	assert.Contains(t, result, "Line1\nLine2\nLine3")
	assert.Contains(t, result, "---")
}

func TestFormatProgressNote_SpecialCharacters(t *testing.T) {
	result := FormatProgressNote("2024-01-15 10:30:00", "agent-cxp", "Fixed bug #123 & improved <performance>")
	assert.Contains(t, result, "agent-cxp")
	assert.Contains(t, result, "Fixed bug #123 & improved <performance>")
}
