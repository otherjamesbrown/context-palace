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

// DefaultPipelineState returns a new PipelineState with default values.
// startPhase overrides the initial phase (empty string defaults to "design").
func DefaultPipelineState(startPhase ...string) PipelineState {
	phase := "design"
	if len(startPhase) > 0 && startPhase[0] != "" {
		phase = startPhase[0]
	}
	return PipelineState{
		Phase:            phase,
		LockedBy:         nil,
		LockExpires:      nil,
		WaitingFor:       []string{},
		LastProgress:     time.Now().UTC().Format(time.RFC3339),
		TaskShards:       []string{},
		CumulativeTokens: 0,
		IterationCounts:  map[string]int{},
	}
}

// PipelineInit initialises the pipeline metadata on a shard.
// Supports design, bug, and task shards — starting phase determined by workflow config.
func (c *Client) PipelineInit(ctx context.Context, id string) (*PipelineState, error) {
	shard, err := c.GetShard(ctx, id)
	if err != nil {
		return nil, err
	}

	// Validate shard type has a workflow
	validTypes := map[string]bool{"design": true, "bug": true, "task": true}
	if !validTypes[shard.Type] {
		return nil, fmt.Errorf("shard %s is type %q — pipeline supports design, bug, and task", id, shard.Type)
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

	// Determine starting phase from workflow config
	repoRoot, _ := RepoForProject(c.Config.Project)
	pCfg, _ := LoadPipelineConfig(repoRoot)
	startPhase := "design"
	if pCfg != nil {
		startPhase = pCfg.StartPhaseForType(shard.Type)
	}

	state := DefaultPipelineState(startPhase)
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
	validTypes := map[string]bool{"design": true, "bug": true, "task": true}
	if !validTypes[shard.Type] {
		return nil, fmt.Errorf("shard %s is type %q — pipeline supports design, bug, and task", id, shard.Type)
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
			detail, err := c.GetShardDetail(ctx, te.ShardID)
			if err != nil {
				continue
			}
			for _, label := range detail.Labels {
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

// PipelineGateResult holds the outcome of a generic pipeline gate operation.
type PipelineGateResult struct {
	DesignID      string `json:"design_id"`
	GateName      string `json:"gate_name"`
	Phase         string `json:"phase"`
	Round         int    `json:"round"`
	Verdict       string `json:"verdict"`
	ReviewShardID string `json:"review_shard_id"`
	NextPhase     string `json:"next_phase,omitempty"`
}

// isTableMissing returns true if the error indicates the table doesn't exist (migration not applied).
func isTableMissing(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "does not exist") || strings.Contains(msg, "relation") || strings.Contains(msg, "table not found")
}

// PipelineGatePass is a generic gate method that records a verdict at any pipeline phase.
// It creates a review sub-shard, records to DB tables (if available), updates shard metadata,
// and advances the phase on pass.
func (c *Client) PipelineGatePass(ctx context.Context, designID, gateName, verdict, body string, readiness int, cfg *PipelineConfig) (*PipelineGateResult, error) {
	// 1. Get pipeline state — determine current phase
	state, err := c.PipelineGet(ctx, designID)
	if err != nil {
		return nil, err
	}
	currentPhase := state.Phase

	// 2. Find gate config (optional — allow gate even if not configured)
	var gateConfig *GateConfig
	if cfg != nil {
		gateConfig = cfg.FindGate(currentPhase, gateName)
	}

	// 3. If verdict is "pass" and gate has RequiresLabel, check child tasks
	if verdict == "pass" && gateConfig != nil && gateConfig.RequiresLabel != "" {
		edges, err := c.GetShardEdges(ctx, designID, "incoming", []string{"child-of"})
		if err != nil {
			return nil, fmt.Errorf("failed to get child edges: %v", err)
		}
		hasLabel := false
		for _, e := range edges {
			if e.Type == "task" {
				detail, err := c.GetShardDetail(ctx, e.ShardID)
				if err != nil {
					continue
				}
				for _, label := range detail.Labels {
					if label == gateConfig.RequiresLabel {
						hasLabel = true
						break
					}
				}
				if hasLabel {
					break
				}
			}
		}
		if !hasLabel {
			return nil, fmt.Errorf("cannot pass gate %q: no child task with required label %q found", gateName, gateConfig.RequiresLabel)
		}
	}

	// 4. Try to get/create pipeline_run from DB (graceful if table missing)
	var pipelineID string
	dbAvailable := true
	pipelineRun, err := c.GetPipelineRun(ctx, designID)
	if err != nil {
		if isTableMissing(err) {
			dbAvailable = false
		} else if strings.Contains(err.Error(), "no pipeline run found") {
			// Create a new pipeline run
			shard, shardErr := c.GetShard(ctx, designID)
			project := ""
			if shardErr == nil {
				project = shard.Project
			}
			pipelineRun, err = c.CreatePipelineRun(ctx, designID, project, currentPhase)
			if err != nil {
				if isTableMissing(err) {
					dbAvailable = false
				} else {
					return nil, fmt.Errorf("failed to create pipeline run: %v", err)
				}
			}
		} else {
			return nil, fmt.Errorf("failed to get pipeline run: %v", err)
		}
	}
	if pipelineRun != nil {
		pipelineID = pipelineRun.ID
	}

	// 5. Compute round
	round := 1
	if dbAvailable && pipelineID != "" {
		latestRound, err := c.GetLatestGateRound(ctx, pipelineID, gateName)
		if err != nil {
			if isTableMissing(err) {
				dbAvailable = false
			}
			// fall through with round=1
		} else {
			round = latestRound + 1
		}
	} else {
		// Fall back to shard metadata for round
		if state.Review != nil && gateName == "readiness-review" {
			round = state.Review.Round + 1
		} else if state.Decompose != nil && gateName == "decomposition-review" {
			round = state.Decompose.Round + 1
		}
		// For unknown gates without DB, start at round 1
	}

	// 6. Create review sub-shard
	now := time.Now().UTC()
	timestamp := now.Format(time.RFC3339)
	verdictUpper := "FAIL"
	if verdict == "pass" {
		verdictUpper = "PASS"
	}

	content := fmt.Sprintf("# Gate: %s — Round %d\n\n**Design:** %s\n**Reviewer:** %s\n**Timestamp:** %s\n**Verdict:** %s\n",
		gateName, round, designID, c.Config.Agent, timestamp, verdictUpper)
	if readiness > 0 {
		content += fmt.Sprintf("**Readiness Score:** %d/5\n", readiness)
	}
	content += fmt.Sprintf("\n## Findings\n\n%s", body)

	title := fmt.Sprintf("Gate: %s — Round %d — %s", gateName, round, verdictUpper)
	metaMap := map[string]any{
		"design_id": designID,
		"gate_name": gateName,
		"round":     round,
		"verdict":   verdict,
	}
	if readiness > 0 {
		metaMap["readiness"] = readiness
	}
	meta, _ := json.Marshal(metaMap)
	reviewID, err := c.CreateShardWithMetadata(ctx, title, content, "review", nil, []string{gateName}, json.RawMessage(meta))
	if err != nil {
		return nil, fmt.Errorf("failed to create gate review shard: %v", err)
	}

	// Create child-of edge
	if err := c.CreateEdgeSimple(ctx, reviewID, designID, "child-of"); err != nil {
		return nil, fmt.Errorf("failed to create edge: %v", err)
	}

	// 7. Record gate in DB (skip gracefully if table doesn't exist)
	if dbAvailable && pipelineID != "" {
		var readinessPtr *int
		if readiness > 0 {
			readinessPtr = &readiness
		}
		var bodyPtr *string
		if body != "" {
			bodyPtr = &body
		}
		reviewer := c.Config.Agent
		_, err := c.RecordGate(ctx, PipelineGateInput{
			PipelineID:     pipelineID,
			DesignID:       designID,
			GateName:       gateName,
			Phase:          currentPhase,
			Verdict:        verdict,
			Reviewer:       &reviewer,
			ReadinessScore: readinessPtr,
			Body:           bodyPtr,
			ReviewShardID:  &reviewID,
		})
		if err != nil && !isTableMissing(err) {
			return nil, fmt.Errorf("failed to record gate in DB: %v", err)
		}
	}

	// 8. Dual-write to shard metadata for backward compat
	gateState := map[string]any{
		"round":        round,
		"last_verdict": verdict,
		"last_shard":   reviewID,
		"history": append(func() []map[string]any {
			// Try to read existing history from metadata
			return nil
		}(), map[string]any{
			"round":     round,
			"verdict":   verdict,
			"shard_id":  reviewID,
			"timestamp": timestamp,
		}),
	}

	// Read existing gate history from metadata
	if state.Review != nil && gateName == "readiness-review" {
		existing := make([]map[string]any, 0, len(state.Review.History))
		for _, h := range state.Review.History {
			existing = append(existing, map[string]any{
				"round":     h.Round,
				"verdict":   h.Verdict,
				"shard_id":  h.ShardID,
				"timestamp": h.Timestamp,
			})
		}
		gateState["history"] = append(existing, map[string]any{
			"round":     round,
			"verdict":   verdict,
			"shard_id":  reviewID,
			"timestamp": timestamp,
		})
	} else if state.Decompose != nil && gateName == "decomposition-review" {
		existing := make([]map[string]any, 0, len(state.Decompose.History))
		for _, h := range state.Decompose.History {
			existing = append(existing, map[string]any{
				"round":      h.Round,
				"verdict":    h.Verdict,
				"shard_id":   h.ShardID,
				"task_count": h.TaskCount,
				"timestamp":  h.Timestamp,
			})
		}
		gateState["history"] = append(existing, map[string]any{
			"round":     round,
			"verdict":   verdict,
			"shard_id":  reviewID,
			"timestamp": timestamp,
		})
	}

	gateJSON, err := json.Marshal(gateState)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gate state: %v", err)
	}
	if _, err := c.SetMetadataPath(ctx, designID, []string{"pipeline", gateName}, gateJSON); err != nil {
		return nil, fmt.Errorf("failed to update gate metadata: %v", err)
	}

	// Also write backward-compat fields for known gates
	if gateName == "readiness-review" {
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
		reviewJSON, _ := json.Marshal(reviewState)
		c.SetMetadataPath(ctx, designID, []string{"pipeline", "review"}, reviewJSON)
	} else if gateName == "decomposition-review" {
		decomposeState := &DecomposeState{
			Round:              round,
			LastVerdict:        verdict,
			LastDecomposeShard: reviewID,
			History: append(func() []DecomposeHistoryEntry {
				if state.Decompose != nil {
					return state.Decompose.History
				}
				return nil
			}(), DecomposeHistoryEntry{
				Round:     round,
				Verdict:   verdict,
				ShardID:   reviewID,
				Timestamp: timestamp,
			}),
		}
		decomposeJSON, _ := json.Marshal(decomposeState)
		c.SetMetadataPath(ctx, designID, []string{"pipeline", "decompose"}, decomposeJSON)
	}

	// 9. If pass, determine next phase and advance
	nextPhase := ""
	resultPhase := currentPhase
	if verdict == "pass" {
		if cfg != nil {
			nextPhase = cfg.NextPhase(currentPhase)
		}
		if nextPhase == "" {
			// Fallback for known gates
			switch gateName {
			case "readiness-review":
				nextPhase = "decompose"
			case "decomposition-review":
				nextPhase = "implement"
			}
		}
		if nextPhase != "" {
			// Update via shard metadata
			_, err := c.PipelineUpdate(ctx, designID, &nextPhase, nil, nil, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to advance phase: %v", err)
			}
			resultPhase = nextPhase
			// Update via pipeline_runs table
			if dbAvailable && pipelineID != "" {
				_ = c.UpdatePipelineRunPhase(ctx, designID, nextPhase)
			}
		}
	}

	return &PipelineGateResult{
		DesignID:      designID,
		GateName:      gateName,
		Phase:         resultPhase,
		Round:         round,
		Verdict:       verdict,
		ReviewShardID: reviewID,
		NextPhase:     nextPhase,
	}, nil
}

// PipelineAuditEntry represents a single entry in the pipeline audit trail.
type PipelineAuditEntry struct {
	Timestamp     time.Time `json:"timestamp"`
	GateName      string    `json:"gate_name"`
	Round         int       `json:"round"`
	Verdict       string    `json:"verdict"`
	ReviewShardID string    `json:"review_shard_id"`
	Body          string    `json:"body,omitempty"`
}

// PipelineAudit retrieves the audit trail for a design, trying the DB first,
// falling back to shard metadata.
func (c *Client) PipelineAudit(ctx context.Context, designID string) ([]PipelineAuditEntry, error) {
	// Try table mode first
	records, err := c.GetGateHistory(ctx, designID)
	if err == nil && len(records) > 0 {
		entries := make([]PipelineAuditEntry, len(records))
		for i, r := range records {
			body := ""
			if r.Body != nil {
				body = *r.Body
			}
			shardID := ""
			if r.ReviewShardID != nil {
				shardID = *r.ReviewShardID
			}
			entries[i] = PipelineAuditEntry{
				Timestamp:     r.CreatedAt,
				GateName:      r.GateName,
				Round:         r.Round,
				Verdict:       r.Verdict,
				ReviewShardID: shardID,
				Body:          body,
			}
		}
		return entries, nil
	}

	// Fallback: read from shard metadata
	state, err := c.PipelineGet(ctx, designID)
	if err != nil {
		return nil, err
	}

	var entries []PipelineAuditEntry

	if state.Review != nil {
		for _, h := range state.Review.History {
			ts, _ := time.Parse(time.RFC3339, h.Timestamp)
			entries = append(entries, PipelineAuditEntry{
				Timestamp:     ts,
				GateName:      "readiness-review",
				Round:         h.Round,
				Verdict:       h.Verdict,
				ReviewShardID: h.ShardID,
			})
		}
	}

	if state.Decompose != nil {
		for _, h := range state.Decompose.History {
			ts, _ := time.Parse(time.RFC3339, h.Timestamp)
			entries = append(entries, PipelineAuditEntry{
				Timestamp:     ts,
				GateName:      "decomposition-review",
				Round:         h.Round,
				Verdict:       h.Verdict,
				ReviewShardID: h.ShardID,
				Body:          fmt.Sprintf("%d tasks", h.TaskCount),
			})
		}
	}

	return entries, nil
}

