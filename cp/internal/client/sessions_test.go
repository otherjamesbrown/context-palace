package client

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrNoSession_IsDetectable(t *testing.T) {
	// ErrNoSession should be detectable with errors.Is
	assert.True(t, errors.Is(ErrNoSession, ErrNoSession))

	// A wrapped error should still match
	wrapped := fmt.Errorf("wrapped: %w", ErrNoSession)
	assert.True(t, errors.Is(wrapped, ErrNoSession))
}

func TestParseLastCheckpoint_Basic(t *testing.T) {
	content := `## Session started: 2026-02-17 10:00:00

Agent: agent-cxp

### [10:30:00] Checkpoint
First checkpoint note

### [11:00:00] Checkpoint
Second checkpoint note`

	entry, err := ParseLastCheckpoint(content, "")
	require.NoError(t, err)
	assert.Equal(t, "11:00:00", entry.Timestamp)
	assert.Equal(t, "Second checkpoint note", entry.Content)
	assert.Equal(t, "", entry.Tag)
}

func TestParseLastCheckpoint_WithTag(t *testing.T) {
	content := `### [10:00:00] Checkpoint
General note

### [11:00:00] Checkpoint [handoff:main]
Main handoff content

### [12:00:00] Checkpoint
Another general note`

	// Filter by tag "main"
	entry, err := ParseLastCheckpoint(content, "main")
	require.NoError(t, err)
	assert.Equal(t, "11:00:00", entry.Timestamp)
	assert.Equal(t, "main", entry.Tag)
	assert.Equal(t, "Main handoff content", entry.Content)
}

func TestParseLastCheckpoint_TagPrefix(t *testing.T) {
	content := `### [10:00:00] Checkpoint [tag:deploy]
Deploy checkpoint`

	entry, err := ParseLastCheckpoint(content, "deploy")
	require.NoError(t, err)
	assert.Equal(t, "deploy", entry.Tag)
	assert.Equal(t, "Deploy checkpoint", entry.Content)
}

func TestParseLastCheckpoint_NoCheckpoints(t *testing.T) {
	content := `## Session started: 2026-02-17

No checkpoints here.`

	_, err := ParseLastCheckpoint(content, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no checkpoints found")
}

func TestParseLastCheckpoint_NoMatchingTag(t *testing.T) {
	content := `### [10:00:00] Checkpoint
General note`

	_, err := ParseLastCheckpoint(content, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no checkpoints found with tag")
}

func TestParseLastCheckpoint_MultilineContent(t *testing.T) {
	content := `### [14:00:00] Checkpoint
## Deliverable 1 complete

**Files:** sessions.go
**Tests:** 3 passing
**Next:** deliverable 2`

	entry, err := ParseLastCheckpoint(content, "")
	require.NoError(t, err)
	assert.Equal(t, "14:00:00", entry.Timestamp)
	assert.Contains(t, entry.Content, "Deliverable 1 complete")
	assert.Contains(t, entry.Content, "**Next:** deliverable 2")
}
