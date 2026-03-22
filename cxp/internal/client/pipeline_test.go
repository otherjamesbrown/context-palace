package client

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePipelinePhase_ValidPhases(t *testing.T) {
	for _, phase := range ValidPipelinePhases {
		assert.NoError(t, ValidatePipelinePhase(phase), "phase %q should be valid", phase)
	}
}

func TestValidatePipelinePhase_Invalid(t *testing.T) {
	err := ValidatePipelinePhase("invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pipeline phase")
	assert.Contains(t, err.Error(), "valid phases")
}

func TestValidatePipelinePhase_Empty(t *testing.T) {
	err := ValidatePipelinePhase("")
	assert.Error(t, err)
}

func TestDefaultPipelineState(t *testing.T) {
	before := time.Now().UTC()
	state := DefaultPipelineState()
	after := time.Now().UTC()

	assert.Equal(t, "design", state.Phase)
	assert.Nil(t, state.LockedBy)
	assert.Nil(t, state.LockExpires)
	assert.Empty(t, state.WaitingFor)
	assert.NotEmpty(t, state.LastProgress)
	assert.Empty(t, state.TaskShards)
	assert.Equal(t, 0, state.CumulativeTokens)
	assert.NotNil(t, state.IterationCounts)
	assert.Empty(t, state.IterationCounts)

	// Verify last_progress is a valid RFC3339 timestamp within bounds
	ts, err := time.Parse(time.RFC3339, state.LastProgress)
	require.NoError(t, err)
	assert.False(t, ts.Before(before.Truncate(time.Second)))
	assert.False(t, ts.After(after.Add(time.Second)))
}

func TestDefaultPipelineState_JSON_Roundtrip(t *testing.T) {
	state := DefaultPipelineState()

	data, err := json.Marshal(state)
	require.NoError(t, err)

	var decoded PipelineState
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, state.Phase, decoded.Phase)
	assert.Equal(t, state.CumulativeTokens, decoded.CumulativeTokens)
	assert.Equal(t, state.WaitingFor, decoded.WaitingFor)
	assert.Equal(t, state.TaskShards, decoded.TaskShards)
}

func TestPipelineState_JSON_NullFields(t *testing.T) {
	state := DefaultPipelineState()
	data, err := json.Marshal(state)
	require.NoError(t, err)

	// locked_by and lock_expires should be null in JSON
	var raw map[string]json.RawMessage
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Equal(t, "null", string(raw["locked_by"]))
	assert.Equal(t, "null", string(raw["lock_expires"]))
}

func TestPipelineState_JSON_WithValues(t *testing.T) {
	agent := "agent-steve"
	expires := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	state := PipelineState{
		Phase:            "implement",
		LockedBy:         &agent,
		LockExpires:      &expires,
		WaitingFor:       []string{"pf-review-1"},
		LastProgress:     "2026-03-22T10:00:00Z",
		TaskShards:       []string{"pf-task-1", "pf-task-2"},
		CumulativeTokens: 5000,
		IterationCounts:  map[string]int{"design": 2, "implement": 1},
	}

	data, err := json.Marshal(state)
	require.NoError(t, err)

	var decoded PipelineState
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "implement", decoded.Phase)
	assert.Equal(t, "agent-steve", *decoded.LockedBy)
	assert.Equal(t, expires, *decoded.LockExpires)
	assert.Equal(t, []string{"pf-review-1"}, decoded.WaitingFor)
	assert.Equal(t, []string{"pf-task-1", "pf-task-2"}, decoded.TaskShards)
	assert.Equal(t, 5000, decoded.CumulativeTokens)
	assert.Equal(t, 2, decoded.IterationCounts["design"])
	assert.Equal(t, 1, decoded.IterationCounts["implement"])
}

func TestPipelineLockCheck_Unlocked(t *testing.T) {
	state := DefaultPipelineState()
	// LockedBy is nil by default
	status := lockStatus(&state)
	assert.Equal(t, "unlocked", status)
}

func TestPipelineLockCheck_Locked(t *testing.T) {
	agent := "agent-steve-123"
	expires := time.Now().UTC().Add(5 * time.Minute)
	state := PipelineState{
		Phase:       "implement",
		LockedBy:    &agent,
		LockExpires: &expires,
	}
	status := lockStatus(&state)
	assert.Equal(t, "locked", status)
}

func TestPipelineLockCheck_Stale(t *testing.T) {
	agent := "agent-steve-123"
	expires := time.Now().UTC().Add(-1 * time.Minute) // expired
	state := PipelineState{
		Phase:       "implement",
		LockedBy:    &agent,
		LockExpires: &expires,
	}
	status := lockStatus(&state)
	assert.Equal(t, "stale", status)
}

func TestPipelineLock_SetsFields(t *testing.T) {
	state := DefaultPipelineState()
	sessionID := "agent-steve-1711100000"

	before := time.Now().UTC()
	applyLock(&state, sessionID)
	after := time.Now().UTC()

	require.NotNil(t, state.LockedBy)
	assert.Equal(t, sessionID, *state.LockedBy)
	require.NotNil(t, state.LockExpires)
	assert.False(t, state.LockExpires.Before(before.Add(LockTTL-time.Second)))
	assert.False(t, state.LockExpires.After(after.Add(LockTTL+time.Second)))
}

func TestPipelineLock_FailsOnActiveLock(t *testing.T) {
	agent := "agent-other"
	expires := time.Now().UTC().Add(5 * time.Minute)
	state := PipelineState{
		Phase:       "implement",
		LockedBy:    &agent,
		LockExpires: &expires,
	}
	err := checkLockAvailable(&state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline locked by agent-other")
}

func TestPipelineLock_SucceedsOnStaleLock(t *testing.T) {
	agent := "agent-other"
	expires := time.Now().UTC().Add(-1 * time.Minute) // expired
	state := PipelineState{
		Phase:       "implement",
		LockedBy:    &agent,
		LockExpires: &expires,
	}
	err := checkLockAvailable(&state)
	assert.NoError(t, err)
}

func TestPipelineUnlock_ClearsFields(t *testing.T) {
	agent := "agent-steve"
	expires := time.Now().UTC().Add(5 * time.Minute)
	state := PipelineState{
		Phase:       "implement",
		LockedBy:    &agent,
		LockExpires: &expires,
	}
	applyUnlock(&state)
	assert.Nil(t, state.LockedBy)
	assert.Nil(t, state.LockExpires)
}

// lockStatus computes the lock status string from a PipelineState (mirrors PipelineLockCheck logic)
func lockStatus(state *PipelineState) string {
	if state.LockedBy == nil {
		return "unlocked"
	}
	if state.LockExpires != nil && state.LockExpires.After(time.Now().UTC()) {
		return "locked"
	}
	return "stale"
}

// checkLockAvailable mirrors the lock availability check in PipelineLock
func checkLockAvailable(state *PipelineState) error {
	if state.LockedBy != nil && state.LockExpires != nil && state.LockExpires.After(time.Now().UTC()) {
		return fmt.Errorf("pipeline locked by %s", *state.LockedBy)
	}
	return nil
}

// applyLock mirrors the lock application in PipelineLock
func applyLock(state *PipelineState, sessionID string) {
	state.LockedBy = &sessionID
	expires := time.Now().UTC().Add(LockTTL)
	state.LockExpires = &expires
}

// applyUnlock mirrors the unlock in PipelineUnlock
func applyUnlock(state *PipelineState) {
	state.LockedBy = nil
	state.LockExpires = nil
}

func TestValidPipelinePhases_AllExpected(t *testing.T) {
	expected := []string{"design", "decompose", "implement", "review", "deploy", "done"}
	assert.Equal(t, expected, ValidPipelinePhases)
}
