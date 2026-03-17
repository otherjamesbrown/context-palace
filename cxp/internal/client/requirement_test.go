package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- validateTransition tests ---

func TestValidateTransition_ValidDraftToApproved(t *testing.T) {
	assert.NoError(t, validateTransition("draft", "approved"))
}

func TestValidateTransition_ValidApprovedToInProgress(t *testing.T) {
	assert.NoError(t, validateTransition("approved", "in_progress"))
}

func TestValidateTransition_ValidInProgressToImplemented(t *testing.T) {
	assert.NoError(t, validateTransition("in_progress", "implemented"))
}

func TestValidateTransition_ValidInProgressToApproved(t *testing.T) {
	assert.NoError(t, validateTransition("in_progress", "approved"))
}

func TestValidateTransition_ValidImplementedToVerified(t *testing.T) {
	assert.NoError(t, validateTransition("implemented", "verified"))
}

func TestValidateTransition_ValidImplementedToApproved(t *testing.T) {
	assert.NoError(t, validateTransition("implemented", "approved"))
}

func TestValidateTransition_ValidVerifiedToApproved(t *testing.T) {
	assert.NoError(t, validateTransition("verified", "approved"))
}

func TestValidateTransition_InvalidDraftToImplemented(t *testing.T) {
	err := validateTransition("draft", "implemented")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot transition from 'draft' to 'implemented'")
}

func TestValidateTransition_InvalidVerifiedToImplemented(t *testing.T) {
	err := validateTransition("verified", "implemented")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot transition")
}

func TestValidateTransition_UnknownFromStatus(t *testing.T) {
	err := validateTransition("nonexistent", "approved")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown lifecycle status: nonexistent")
}

func TestValidateTransition_SameStatus(t *testing.T) {
	err := validateTransition("draft", "draft")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot transition")
}

// --- parseLifecycleFromMeta tests ---

func TestParseLifecycleFromMeta_ValidStatus(t *testing.T) {
	meta := []byte(`{"lifecycle_status": "approved", "priority": 1}`)
	assert.Equal(t, "approved", parseLifecycleFromMeta(meta))
}

func TestParseLifecycleFromMeta_MissingField(t *testing.T) {
	meta := []byte(`{"priority": 1}`)
	assert.Equal(t, "draft", parseLifecycleFromMeta(meta))
}

func TestParseLifecycleFromMeta_InvalidJSON(t *testing.T) {
	meta := []byte(`not json`)
	assert.Equal(t, "draft", parseLifecycleFromMeta(meta))
}

func TestParseLifecycleFromMeta_EmptyJSON(t *testing.T) {
	meta := []byte(`{}`)
	assert.Equal(t, "draft", parseLifecycleFromMeta(meta))
}

func TestParseLifecycleFromMeta_NonStringStatus(t *testing.T) {
	meta := []byte(`{"lifecycle_status": 42}`)
	assert.Equal(t, "draft", parseLifecycleFromMeta(meta))
}

func TestParseLifecycleFromMeta_NullStatus(t *testing.T) {
	meta := []byte(`{"lifecycle_status": null}`)
	assert.Equal(t, "draft", parseLifecycleFromMeta(meta))
}

// --- resolveEdgeDirection tests ---

func TestResolveEdgeDirection_Implements(t *testing.T) {
	from, to, err := resolveEdgeDirection("req-1", "task-1", "implements")
	require.NoError(t, err)
	assert.Equal(t, "task-1", from) // task --implements--> requirement
	assert.Equal(t, "req-1", to)
}

func TestResolveEdgeDirection_HasArtifact(t *testing.T) {
	from, to, err := resolveEdgeDirection("req-1", "test-1", "has-artifact")
	require.NoError(t, err)
	assert.Equal(t, "test-1", from) // test --has-artifact--> requirement
	assert.Equal(t, "req-1", to)
}

func TestResolveEdgeDirection_BlockedBy(t *testing.T) {
	from, to, err := resolveEdgeDirection("req-1", "dep-1", "blocked-by")
	require.NoError(t, err)
	assert.Equal(t, "req-1", from) // requirement --blocked-by--> dependency
	assert.Equal(t, "dep-1", to)
}

func TestResolveEdgeDirection_Unknown(t *testing.T) {
	_, _, err := resolveEdgeDirection("req-1", "x", "unknown-type")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown edge type: unknown-type")
}
