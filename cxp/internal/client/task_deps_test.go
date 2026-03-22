package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDispatchPlan_NoDeps(t *testing.T) {
	nodes := map[string]TaskNode{
		"pf-abc": {ID: "pf-abc", Title: "Schema migration", Status: "open", BlockedBy: nil},
		"pf-def": {ID: "pf-def", Title: "Config parser", Status: "open", BlockedBy: nil},
	}

	result, err := BuildDispatchPlan(nodes)
	require.NoError(t, err)

	assert.Len(t, result.Waves, 1)
	assert.Equal(t, 1, result.Waves[0].Wave)
	assert.Len(t, result.Waves[0].Tasks, 2)
	assert.Equal(t, []string{"pf-abc", "pf-def"}, result.Dispatchable)
}

func TestBuildDispatchPlan_TwoWaves(t *testing.T) {
	nodes := map[string]TaskNode{
		"pf-abc": {ID: "pf-abc", Title: "Schema migration", Status: "open", BlockedBy: nil},
		"pf-def": {ID: "pf-def", Title: "Config parser", Status: "open", BlockedBy: nil},
		"pf-ghi": {ID: "pf-ghi", Title: "Extract stage", Status: "open", BlockedBy: []string{"pf-abc"}},
		"pf-jkl": {ID: "pf-jkl", Title: "Rollup logic", Status: "open", BlockedBy: []string{"pf-abc"}},
	}

	result, err := BuildDispatchPlan(nodes)
	require.NoError(t, err)

	assert.Len(t, result.Waves, 2)

	// Wave 1: no deps
	assert.Equal(t, 1, result.Waves[0].Wave)
	wave1IDs := taskIDs(result.Waves[0].Tasks)
	assert.Equal(t, []string{"pf-abc", "pf-def"}, wave1IDs)

	// Wave 2: blocked by wave 1
	assert.Equal(t, 2, result.Waves[1].Wave)
	wave2IDs := taskIDs(result.Waves[1].Tasks)
	assert.Equal(t, []string{"pf-ghi", "pf-jkl"}, wave2IDs)

	// Only wave 1 tasks are dispatchable
	assert.Equal(t, []string{"pf-abc", "pf-def"}, result.Dispatchable)
}

func TestBuildDispatchPlan_ThreeWaves(t *testing.T) {
	nodes := map[string]TaskNode{
		"pf-a": {ID: "pf-a", Title: "First", Status: "open", BlockedBy: nil},
		"pf-b": {ID: "pf-b", Title: "Second", Status: "open", BlockedBy: []string{"pf-a"}},
		"pf-c": {ID: "pf-c", Title: "Third", Status: "open", BlockedBy: []string{"pf-b"}},
	}

	result, err := BuildDispatchPlan(nodes)
	require.NoError(t, err)

	assert.Len(t, result.Waves, 3)
	assert.Equal(t, "pf-a", result.Waves[0].Tasks[0].ID)
	assert.Equal(t, "pf-b", result.Waves[1].Tasks[0].ID)
	assert.Equal(t, "pf-c", result.Waves[2].Tasks[0].ID)
}

func TestBuildDispatchPlan_CycleDetection(t *testing.T) {
	nodes := map[string]TaskNode{
		"pf-a": {ID: "pf-a", Title: "First", Status: "open", BlockedBy: []string{"pf-b"}},
		"pf-b": {ID: "pf-b", Title: "Second", Status: "open", BlockedBy: []string{"pf-a"}},
	}

	_, err := BuildDispatchPlan(nodes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
	assert.Contains(t, err.Error(), "pf-a")
	assert.Contains(t, err.Error(), "pf-b")
}

func TestBuildDispatchPlan_CycleInLargerGraph(t *testing.T) {
	nodes := map[string]TaskNode{
		"pf-a": {ID: "pf-a", Title: "First", Status: "open", BlockedBy: nil},
		"pf-b": {ID: "pf-b", Title: "Second", Status: "open", BlockedBy: []string{"pf-a", "pf-c"}},
		"pf-c": {ID: "pf-c", Title: "Third", Status: "open", BlockedBy: []string{"pf-b"}},
	}

	_, err := BuildDispatchPlan(nodes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

func TestBuildDispatchPlan_Empty(t *testing.T) {
	result, err := BuildDispatchPlan(map[string]TaskNode{})
	require.NoError(t, err)
	assert.Empty(t, result.Waves)
	assert.Empty(t, result.Dispatchable)
}

func TestFindDispatchable_ClosedBlockers(t *testing.T) {
	nodes := map[string]TaskNode{
		"pf-a": {ID: "pf-a", Title: "First", Status: "closed", BlockedBy: nil},
		"pf-b": {ID: "pf-b", Title: "Second", Status: "open", BlockedBy: []string{"pf-a"}},
	}

	result, err := BuildDispatchPlan(nodes)
	require.NoError(t, err)

	// pf-a is closed so not dispatchable; pf-b's blocker (pf-a) is closed, so pf-b IS dispatchable
	assert.Equal(t, []string{"pf-b"}, result.Dispatchable)
}

func TestFindDispatchable_NeedsReviewNotDispatchable(t *testing.T) {
	nodes := map[string]TaskNode{
		"pf-a": {ID: "pf-a", Title: "First", Status: "needs-review", BlockedBy: nil},
		"pf-b": {ID: "pf-b", Title: "Second", Status: "open", BlockedBy: nil},
	}

	result, err := BuildDispatchPlan(nodes)
	require.NoError(t, err)

	// pf-a is needs-review so not dispatchable; pf-b is open so dispatchable
	assert.Equal(t, []string{"pf-b"}, result.Dispatchable)
}

func TestFindDispatchable_InProgressIsDispatchable(t *testing.T) {
	nodes := map[string]TaskNode{
		"pf-a": {ID: "pf-a", Title: "First", Status: "in_progress", BlockedBy: nil},
	}

	result, err := BuildDispatchPlan(nodes)
	require.NoError(t, err)

	assert.Equal(t, []string{"pf-a"}, result.Dispatchable)
}

func TestFindDispatchable_BlockerNotClosed(t *testing.T) {
	nodes := map[string]TaskNode{
		"pf-a": {ID: "pf-a", Title: "First", Status: "in_progress", BlockedBy: nil},
		"pf-b": {ID: "pf-b", Title: "Second", Status: "open", BlockedBy: []string{"pf-a"}},
	}

	result, err := BuildDispatchPlan(nodes)
	require.NoError(t, err)

	// pf-a is in_progress (dispatchable), pf-b's blocker not closed (not dispatchable)
	assert.Equal(t, []string{"pf-a"}, result.Dispatchable)
}

func TestBuildDispatchPlan_ExternalBlockerIgnored(t *testing.T) {
	// pf-b references a blocker not in the graph - it should be treated as no in-graph blockers
	nodes := map[string]TaskNode{
		"pf-a": {ID: "pf-a", Title: "First", Status: "open", BlockedBy: nil},
		"pf-b": {ID: "pf-b", Title: "Second", Status: "open", BlockedBy: []string{"pf-external"}},
	}

	result, err := BuildDispatchPlan(nodes)
	require.NoError(t, err)

	// Both should be in wave 1 since external blocker is not in the graph
	assert.Len(t, result.Waves, 1)
	assert.Len(t, result.Waves[0].Tasks, 2)
}

func taskIDs(tasks []TaskNode) []string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}
