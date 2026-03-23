package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

// ReviewHistoryEntry records a single review round
type ReviewHistoryEntry struct {
	Round     int    `json:"round"`
	Verdict   string `json:"verdict"`
	ShardID   string `json:"shard_id"`
	Timestamp string `json:"timestamp"`
}

// ReviewState tracks cumulative review history on a pipeline
type ReviewState struct {
	Round           int                  `json:"round"`
	LastVerdict     string               `json:"last_verdict"`
	LastReviewShard string               `json:"last_review_shard"`
	History         []ReviewHistoryEntry `json:"history"`
}

// PipelineState represents the pipeline metadata stored on a design shard
type PipelineState struct {
	Phase            string              `json:"phase"`
	LockedBy         *string             `json:"locked_by"`
	LockExpires      *time.Time          `json:"lock_expires"`
	WaitingFor       []string            `json:"waiting_for"`
	LastProgress     string              `json:"last_progress"`
	TaskShards       []string            `json:"task_shards"`
	CumulativeTokens int                 `json:"cumulative_tokens"`
	IterationCounts  map[string]int      `json:"iteration_counts"`
	Review           *ReviewState        `json:"review,omitempty"`
	Decompose        *DecomposeState     `json:"decompose,omitempty"`
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

// DecomposeHistoryEntry records a single decomposition round
type DecomposeHistoryEntry struct {
	Round     int    `json:"round"`
	Verdict   string `json:"verdict"`
	ShardID   string `json:"shard_id"`
	TaskCount int    `json:"task_count"`
	Timestamp string `json:"timestamp"`
}

// DecomposeState tracks cumulative decomposition history on a pipeline
type DecomposeState struct {
	Round              int                      `json:"round"`
	LastVerdict        string                   `json:"last_verdict"`
	LastDecomposeShard string                   `json:"last_decompose_shard"`
	History            []DecomposeHistoryEntry  `json:"history"`
}

// PipelineDecomposeResult holds the outcome of a pipeline decompose operation
type PipelineDecomposeResult struct {
	DesignID         string `json:"design_id"`
	DecomposeShardID string `json:"decompose_shard_id"`
	Round            int    `json:"round"`
	Verdict          string `json:"verdict"`
	TaskCount        int    `json:"task_count"`
	Phase            string `json:"phase"`
}

// PipelineReviewResult holds the outcome of a pipeline review operation
type PipelineReviewResult struct {
	DesignID      string `json:"design_id"`
	ReviewShardID string `json:"review_shard_id"`
	Round         int    `json:"round"`
	Verdict       string `json:"verdict"`
	Readiness     int    `json:"readiness"`
	Phase         string `json:"phase"`
}

// PipelineReview records a Phase 1 readiness review verdict, creates a review
// sub-shard, updates pipeline metadata, and optionally advances the phase.
func (c *Client) PipelineReview(ctx context.Context, designID, verdict string, readiness int, body string) (*PipelineReviewResult, error) {
	// 1. Get pipeline state — verify phase is "design"
	state, err := c.PipelineGet(ctx, designID)
	if err != nil {
		return nil, err
	}
	if state.Phase != "design" {
		return nil, fmt.Errorf("pipeline phase is %q, expected 'design' for review", state.Phase)
	}

	// 2. Compute round
	round := 1
	if state.Review != nil {
		round = state.Review.Round + 1
	}

	// 3. Build review shard content
	now := time.Now().UTC()
	timestamp := now.Format(time.RFC3339)
	verdictUpper := verdict
	if verdict == "pass" {
		verdictUpper = "PASS"
	} else {
		verdictUpper = "FAIL"
	}

	content := fmt.Sprintf("# Phase 1 Readiness Review — Round %d\n\n**Design:** %s\n**Reviewer:** %s\n**Timestamp:** %s\n**Verdict:** %s\n**Readiness Score:** %d/5\n\n## Findings\n\n%s",
		round, designID, c.Config.Agent, timestamp, verdictUpper, readiness, body)

	// 4. Create review shard
	title := fmt.Sprintf("Phase 1 Review — Round %d — %s", round, verdictUpper)
	meta, _ := json.Marshal(map[string]any{
		"design_id": designID,
		"round":     round,
		"verdict":   verdict,
		"readiness": readiness,
	})
	reviewID, err := c.CreateShardWithMetadata(ctx, title, content, "review", nil, []string{"phase1-review"}, json.RawMessage(meta))
	if err != nil {
		return nil, fmt.Errorf("failed to create review shard: %v", err)
	}

	// 5. Create child-of edge from review shard to design shard
	if err := c.CreateEdgeSimple(ctx, reviewID, designID, "child-of"); err != nil {
		return nil, fmt.Errorf("failed to create edge: %v", err)
	}

	// 6. Build updated ReviewState and write to pipeline metadata
	reviewState := &ReviewState{
		Round:           round,
		LastVerdict:     verdict,
		LastReviewShard: reviewID,
		History: append(func() []ReviewHistoryEntry {
			if state.Review != nil {
				return state.Review.History
			}
			return nil
		}(), ReviewHistoryEntry{
			Round:     round,
			Verdict:   verdict,
			ShardID:   reviewID,
			Timestamp: timestamp,
		}),
	}
	reviewJSON, err := json.Marshal(reviewState)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal review state: %v", err)
	}
	if _, err := c.SetMetadataPath(ctx, designID, []string{"pipeline", "review"}, reviewJSON); err != nil {
		return nil, fmt.Errorf("failed to update review metadata: %v", err)
	}

	// 7. If pass, advance phase to "decompose"
	resultPhase := state.Phase
	if verdict == "pass" {
		decompose := "decompose"
		updated, err := c.PipelineUpdate(ctx, designID, &decompose, nil, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to advance phase: %v", err)
		}
		resultPhase = updated.Phase
	}

	return &PipelineReviewResult{
		DesignID:      designID,
		ReviewShardID: reviewID,
		Round:         round,
		Verdict:       verdict,
		Readiness:     readiness,
		Phase:         resultPhase,
	}, nil
}

// PipelineDecompose records a Phase 2 decomposition verdict, creates an audit
// shard, updates pipeline metadata, and optionally advances the phase to "implement".
func (c *Client) PipelineDecompose(ctx context.Context, designID, verdict string, body string) (*PipelineDecomposeResult, error) {
	// 1. Get pipeline state — verify phase is "decompose"
	state, err := c.PipelineGet(ctx, designID)
	if err != nil {
		return nil, err
	}
	if state.Phase != "decompose" {
		return nil, fmt.Errorf("pipeline phase is %q, expected 'decompose' for decomposition gate", state.Phase)
	}

	// 2. Compute round
	round := 1
	if state.Decompose != nil {
		round = state.Decompose.Round + 1
	}

	// 3. Get child tasks
	edges, err := c.GetShardEdges(ctx, designID, "incoming", []string{"child-of"})
	if err != nil {
		return nil, fmt.Errorf("failed to get child edges: %v", err)
	}
	var taskEdges []EdgeInfo
	for _, e := range edges {
		if e.Type == "task" {
			taskEdges = append(taskEdges, e)
		}
	}

	taskCount := len(taskEdges)

	// 4. If pass, run validations
	if verdict == "pass" {
		// a. At least 1 task
		if taskCount == 0 {
			return nil, fmt.Errorf("cannot pass decomposition: no child tasks found for design %s", designID)
		}

		// b. Each task has substantive content (>50 chars)
		for _, te := range taskEdges {
			shard, err := c.GetShard(ctx, te.ShardID)
			if err != nil {
				return nil, fmt.Errorf("failed to get task shard %s: %v", te.ShardID, err)
			}
			if len(shard.Content) <= 50 {
				return nil, fmt.Errorf("cannot pass decomposition: task %s (%s) has insufficient content (%d chars, need >50)", te.ShardID, shard.Title, len(shard.Content))
			}
		}

		// c. Validate integration test task exists (labeled "integration-test")
		hasIntegrationTest := false
		for _, te := range taskEdges {
			shard, err := c.GetShard(ctx, te.ShardID)
			if err != nil {
				continue
			}
			for _, label := range shard.Labels {
				if label == "integration-test" {
					hasIntegrationTest = true
					break
				}
			}
			if hasIntegrationTest {
				break
			}
		}
		if !hasIntegrationTest {
			return nil, fmt.Errorf("cannot pass decomposition: no integration test task found (create a task with label 'integration-test' that verifies the design's success criteria end-to-end)")
		}

		// d. Validate acyclic
		_, depsErr := c.GetTaskDeps(ctx, designID)
		if depsErr != nil {
			if strings.Contains(depsErr.Error(), "circular") {
				return nil, fmt.Errorf("cannot pass decomposition: circular dependency detected: %v", depsErr)
			}
			// Non-circular errors are non-fatal for the gate
		}

		// d. Auto-fix: add any tasks not in state.TaskShards
		existingTasks := make(map[string]bool)
		for _, ts := range state.TaskShards {
			existingTasks[ts] = true
		}
		for _, te := range taskEdges {
			if !existingTasks[te.ShardID] {
				addTask := te.ShardID
				_, err := c.PipelineUpdate(ctx, designID, nil, nil, &addTask, nil)
				if err != nil {
					return nil, fmt.Errorf("failed to auto-add task %s to pipeline: %v", te.ShardID, err)
				}
			}
		}
	}

	// 5. Build task list table for audit shard content
	now := time.Now().UTC()
	timestamp := now.Format(time.RFC3339)
	verdictUpper := "FAIL"
	if verdict == "pass" {
		verdictUpper = "PASS"
	}

	var taskRows string
	for i, te := range taskEdges {
		shard, err := c.GetShard(ctx, te.ShardID)
		if err != nil {
			taskRows += fmt.Sprintf("| %d | %s | (error loading) | — |\n", i+1, te.ShardID)
			continue
		}
		// Get blocked-by deps
		depEdges, _ := c.GetShardEdges(ctx, te.ShardID, "outgoing", []string{"blocked-by"})
		deps := "—"
		if len(depEdges) > 0 {
			depIDs := make([]string, len(depEdges))
			for j, d := range depEdges {
				depIDs[j] = d.ShardID
			}
			deps = strings.Join(depIDs, ", ")
		}
		taskRows += fmt.Sprintf("| %d | %s | %s | %s |\n", i+1, te.ShardID, shard.Title, deps)
	}

	content := fmt.Sprintf("# Phase 2 Decomposition — Round %d\n\n**Design:** %s\n**Reviewer:** %s\n**Timestamp:** %s\n**Verdict:** %s\n**Tasks:** %d\n\n## Task List\n\n| # | ID | Title | Blocked by |\n|---|-----|-------|------------|\n%s\n## Findings\n\n%s",
		round, designID, c.Config.Agent, timestamp, verdictUpper, taskCount, taskRows, body)

	// 6. Create decomposition audit shard
	title := fmt.Sprintf("Phase 2 Decomposition — Round %d — %s", round, verdictUpper)
	meta, _ := json.Marshal(map[string]any{
		"design_id":  designID,
		"round":      round,
		"verdict":    verdict,
		"task_count": taskCount,
	})
	decomposeID, err := c.CreateShardWithMetadata(ctx, title, content, "review", nil, []string{"phase2-decompose"}, json.RawMessage(meta))
	if err != nil {
		return nil, fmt.Errorf("failed to create decompose shard: %v", err)
	}

	// 7. Create child-of edge from decompose shard to design shard
	if err := c.CreateEdgeSimple(ctx, decomposeID, designID, "child-of"); err != nil {
		return nil, fmt.Errorf("failed to create edge: %v", err)
	}

	// 8. Build DecomposeState and write to pipeline metadata
	decomposeState := &DecomposeState{
		Round:              round,
		LastVerdict:        verdict,
		LastDecomposeShard: decomposeID,
		History: append(func() []DecomposeHistoryEntry {
			if state.Decompose != nil {
				return state.Decompose.History
			}
			return nil
		}(), DecomposeHistoryEntry{
			Round:     round,
			Verdict:   verdict,
			ShardID:   decomposeID,
			TaskCount: taskCount,
			Timestamp: timestamp,
		}),
	}
	decomposeJSON, err := json.Marshal(decomposeState)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal decompose state: %v", err)
	}
	if _, err := c.SetMetadataPath(ctx, designID, []string{"pipeline", "decompose"}, decomposeJSON); err != nil {
		return nil, fmt.Errorf("failed to update decompose metadata: %v", err)
	}

	// 9. If pass, advance phase to "implement"
	resultPhase := state.Phase
	if verdict == "pass" {
		implement := "implement"
		phaseJSON, _ := json.Marshal(implement)
		if _, err := c.SetMetadataPath(ctx, designID, []string{"pipeline", "phase"}, phaseJSON); err != nil {
			return nil, fmt.Errorf("failed to advance phase: %v", err)
		}
		resultPhase = "implement"
	}

	return &PipelineDecomposeResult{
		DesignID:         designID,
		DecomposeShardID: decomposeID,
		Round:            round,
		Verdict:          verdict,
		TaskCount:        taskCount,
		Phase:            resultPhase,
	}, nil
}

