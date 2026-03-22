package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ValidPipelinePhases is the list of allowed pipeline phase values
var ValidPipelinePhases = []string{
	"design", "decompose", "implement", "review", "deploy", "done",
}

// ValidatePipelinePhase checks if a phase string is valid
func ValidatePipelinePhase(phase string) error {
	for _, valid := range ValidPipelinePhases {
		if phase == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid pipeline phase %q; valid phases: %v", phase, ValidPipelinePhases)
}

// PipelineState represents the pipeline metadata stored on a design shard
type PipelineState struct {
	Phase           string              `json:"phase"`
	LockedBy        *string             `json:"locked_by"`
	LockExpires     *time.Time          `json:"lock_expires"`
	WaitingFor      []string            `json:"waiting_for"`
	LastProgress    string              `json:"last_progress"`
	TaskShards      []string            `json:"task_shards"`
	CumulativeTokens int               `json:"cumulative_tokens"`
	IterationCounts map[string]int      `json:"iteration_counts"`
}

// DefaultPipelineState returns a new PipelineState with default values
func DefaultPipelineState() PipelineState {
	return PipelineState{
		Phase:            "design",
		LockedBy:         nil,
		LockExpires:      nil,
		WaitingFor:       []string{},
		LastProgress:     time.Now().UTC().Format(time.RFC3339),
		TaskShards:       []string{},
		CumulativeTokens: 0,
		IterationCounts:  map[string]int{},
	}
}

// PipelineInit initialises the pipeline metadata on a design shard.
// Returns an error if the shard is not a design or already has pipeline metadata.
func (c *Client) PipelineInit(ctx context.Context, id string) (*PipelineState, error) {
	// Verify it's a design shard
	shard, err := c.GetShard(ctx, id)
	if err != nil {
		return nil, err
	}
	if shard.Type != "design" {
		return nil, fmt.Errorf("shard %s is type %q, expected 'design'", id, shard.Type)
	}

	// Check if pipeline already exists
	if shard.Metadata != nil && len(shard.Metadata) > 2 { // not just "{}"
		var meta map[string]json.RawMessage
		if json.Unmarshal(shard.Metadata, &meta) == nil {
			if _, exists := meta["pipeline"]; exists {
				return nil, fmt.Errorf("shard %s already has pipeline metadata", id)
			}
		}
	}

	state := DefaultPipelineState()
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pipeline state: %v", err)
	}

	_, err = c.SetMetadataPath(ctx, id, []string{"pipeline"}, stateJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to init pipeline: %v", err)
	}

	return &state, nil
}

// PipelineGet retrieves the pipeline state from a design shard's metadata.
func (c *Client) PipelineGet(ctx context.Context, id string) (*PipelineState, error) {
	shard, err := c.GetShard(ctx, id)
	if err != nil {
		return nil, err
	}
	if shard.Type != "design" {
		return nil, fmt.Errorf("shard %s is type %q, expected 'design'", id, shard.Type)
	}

	if shard.Metadata == nil || len(shard.Metadata) <= 2 {
		return nil, fmt.Errorf("shard %s has no pipeline metadata; run `cxp shard pipeline init %s` first", id, id)
	}

	var meta map[string]json.RawMessage
	if err := json.Unmarshal(shard.Metadata, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %v", err)
	}

	raw, exists := meta["pipeline"]
	if !exists {
		return nil, fmt.Errorf("shard %s has no pipeline metadata; run `cxp shard pipeline init %s` first", id, id)
	}

	var state PipelineState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("failed to parse pipeline metadata: %v", err)
	}

	return &state, nil
}

// PipelineUpdate applies incremental updates to the pipeline state.
// Only non-nil / non-zero fields are applied.
func (c *Client) PipelineUpdate(ctx context.Context, id string, phase *string, waitingFor *json.RawMessage, addTask *string, addTokens *int) (*PipelineState, error) {
	state, err := c.PipelineGet(ctx, id)
	if err != nil {
		return nil, err
	}

	if phase != nil {
		if err := ValidatePipelinePhase(*phase); err != nil {
			return nil, err
		}
		state.Phase = *phase
	}

	if waitingFor != nil {
		var wf []string
		if err := json.Unmarshal(*waitingFor, &wf); err != nil {
			return nil, fmt.Errorf("--waiting-for must be a JSON array of strings: %v", err)
		}
		state.WaitingFor = wf
	}

	if addTask != nil {
		// Check for duplicates
		for _, existing := range state.TaskShards {
			if existing == *addTask {
				return nil, fmt.Errorf("task shard %s already in pipeline", *addTask)
			}
		}
		state.TaskShards = append(state.TaskShards, *addTask)
	}

	if addTokens != nil {
		state.CumulativeTokens += *addTokens
	}

	// Update last_progress timestamp
	state.LastProgress = time.Now().UTC().Format(time.RFC3339)

	// Increment iteration count for current phase
	if state.IterationCounts == nil {
		state.IterationCounts = map[string]int{}
	}
	state.IterationCounts[state.Phase]++

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pipeline state: %v", err)
	}

	_, err = c.SetMetadataPath(ctx, id, []string{"pipeline"}, stateJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to update pipeline: %v", err)
	}

	return state, nil
}

// LockTTL is the duration a pipeline lock is held before it becomes stale.
const LockTTL = 5 * time.Minute

// PipelineLock acquires a lock on the pipeline for the given session.
// Returns an error if the pipeline is already locked by another session and the lock has not expired.
func (c *Client) PipelineLock(ctx context.Context, id, sessionID string) (*PipelineState, error) {
	state, err := c.PipelineGet(ctx, id)
	if err != nil {
		return nil, err
	}

	// Check if actively locked
	if state.LockedBy != nil && state.LockExpires != nil && state.LockExpires.After(time.Now().UTC()) {
		return nil, fmt.Errorf("pipeline locked by %s", *state.LockedBy)
	}

	// Acquire lock
	state.LockedBy = &sessionID
	expires := time.Now().UTC().Add(LockTTL)
	state.LockExpires = &expires

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pipeline state: %v", err)
	}

	_, err = c.SetMetadataPath(ctx, id, []string{"pipeline"}, stateJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to lock pipeline: %v", err)
	}

	return state, nil
}

// PipelineUnlock releases the lock on the pipeline.
func (c *Client) PipelineUnlock(ctx context.Context, id string) (*PipelineState, error) {
	state, err := c.PipelineGet(ctx, id)
	if err != nil {
		return nil, err
	}

	state.LockedBy = nil
	state.LockExpires = nil

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pipeline state: %v", err)
	}

	_, err = c.SetMetadataPath(ctx, id, []string{"pipeline"}, stateJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to unlock pipeline: %v", err)
	}

	return state, nil
}

// PipelineLockCheck returns the lock status ("unlocked", "locked", or "stale") and the current pipeline state.
func (c *Client) PipelineLockCheck(ctx context.Context, id string) (string, *PipelineState, error) {
	state, err := c.PipelineGet(ctx, id)
	if err != nil {
		return "", nil, err
	}

	if state.LockedBy == nil {
		return "unlocked", state, nil
	}

	if state.LockExpires != nil && state.LockExpires.After(time.Now().UTC()) {
		return "locked", state, nil
	}

	return "stale", state, nil
}
