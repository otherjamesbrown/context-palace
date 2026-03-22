package client

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// TaskNode represents a task in the dependency graph
type TaskNode struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	BlockedBy []string `json:"blocked_by"`
}

// TaskWave is a group of tasks that can be dispatched together
type TaskWave struct {
	Wave  int        `json:"wave"`
	Tasks []TaskNode `json:"tasks"`
}

// TaskDepsResult is the full dependency graph result
type TaskDepsResult struct {
	Waves        []TaskWave `json:"waves"`
	Dispatchable []string   `json:"dispatchable"`
}

// GetTaskDeps builds the dependency graph for child tasks of a design shard.
func (c *Client) GetTaskDeps(ctx context.Context, designID string) (*TaskDepsResult, error) {
	// 1. Get child tasks of the design (incoming child-of edges = tasks pointing child-of to this design)
	childEdges, err := c.GetShardEdges(ctx, designID, "incoming", []string{"child-of"})
	if err != nil {
		return nil, fmt.Errorf("get child tasks: %w", err)
	}

	if len(childEdges) == 0 {
		return &TaskDepsResult{}, nil
	}

	// Build set of child task IDs
	childIDs := make(map[string]bool, len(childEdges))
	taskInfo := make(map[string]EdgeInfo, len(childEdges))
	for _, e := range childEdges {
		childIDs[e.ShardID] = true
		taskInfo[e.ShardID] = e
	}

	// 2. For each child task, find blocked-by edges
	blockedByMap := make(map[string][]string)
	for id := range childIDs {
		edges, err := c.GetShardEdges(ctx, id, "outgoing", []string{"blocked-by"})
		if err != nil {
			return nil, fmt.Errorf("get blocked-by edges for %s: %w", id, err)
		}
		for _, e := range edges {
			// Only include blockers that are siblings (children of the same design)
			if childIDs[e.ShardID] {
				blockedByMap[id] = append(blockedByMap[id], e.ShardID)
			}
		}
	}

	// 3. Build task nodes
	nodes := make(map[string]TaskNode, len(childIDs))
	for id := range childIDs {
		info := taskInfo[id]
		nodes[id] = TaskNode{
			ID:        id,
			Title:     info.Title,
			Status:    info.Status,
			BlockedBy: blockedByMap[id],
		}
	}

	// 4. Topological sort into waves
	return BuildDispatchPlan(nodes)
}

// BuildDispatchPlan takes task nodes and produces a wave-sorted dispatch plan.
// Exported for testing.
func BuildDispatchPlan(nodes map[string]TaskNode) (*TaskDepsResult, error) {
	if len(nodes) == 0 {
		return &TaskDepsResult{}, nil
	}

	// Detect cycles using topological sort (Kahn's algorithm)
	// inDegree counts how many unsatisfied blockers each task has (within the graph)
	inDegree := make(map[string]int, len(nodes))
	// dependents[a] = list of tasks blocked by a
	dependents := make(map[string][]string)

	for id, node := range nodes {
		inDegree[id] = 0 // initialize
		_ = node
	}
	for id, node := range nodes {
		count := 0
		for _, blocker := range node.BlockedBy {
			if _, ok := nodes[blocker]; ok {
				count++
				dependents[blocker] = append(dependents[blocker], id)
			}
		}
		inDegree[id] = count
	}

	// Process wave by wave
	var waves []TaskWave
	remaining := len(nodes)
	processed := make(map[string]bool)

	for remaining > 0 {
		// Find all tasks with inDegree 0
		var ready []string
		for id, deg := range inDegree {
			if deg == 0 && !processed[id] {
				ready = append(ready, id)
			}
		}

		if len(ready) == 0 {
			// Cycle detected - find the tasks involved
			var cycleIDs []string
			for id := range inDegree {
				if !processed[id] {
					cycleIDs = append(cycleIDs, id)
				}
			}
			sort.Strings(cycleIDs)
			return nil, fmt.Errorf("circular dependency detected among: %s", strings.Join(cycleIDs, ", "))
		}

		sort.Strings(ready) // deterministic ordering

		wave := TaskWave{
			Wave:  len(waves) + 1,
			Tasks: make([]TaskNode, 0, len(ready)),
		}
		for _, id := range ready {
			wave.Tasks = append(wave.Tasks, nodes[id])
			processed[id] = true
			remaining--
			// Reduce in-degree of dependents
			for _, dep := range dependents[id] {
				inDegree[dep]--
			}
		}
		waves = append(waves, wave)
	}

	// Identify dispatchable tasks: wave 1 tasks that are not closed and whose blockers are all closed
	// Since wave 1 has no blockers, all open/in_progress tasks in wave 1 are dispatchable.
	// For later waves, a task is dispatchable if all its blockers are closed.
	dispatchable := FindDispatchable(nodes, waves)

	return &TaskDepsResult{
		Waves:        waves,
		Dispatchable: dispatchable,
	}, nil
}

// FindDispatchable returns IDs of tasks that are ready to be dispatched.
// A task is dispatchable if:
// - it is not closed or needs-review
// - all its blockers (within the graph) are closed
func FindDispatchable(nodes map[string]TaskNode, waves []TaskWave) []string {
	closedSet := make(map[string]bool)
	for _, n := range nodes {
		if n.Status == "closed" {
			closedSet[n.ID] = true
		}
	}

	var dispatchable []string
	for _, wave := range waves {
		for _, task := range wave.Tasks {
			if task.Status == "closed" || task.Status == "needs-review" {
				continue
			}
			allBlockersClosed := true
			for _, blocker := range task.BlockedBy {
				if _, inGraph := nodes[blocker]; inGraph && !closedSet[blocker] {
					allBlockersClosed = false
					break
				}
			}
			if allBlockersClosed {
				dispatchable = append(dispatchable, task.ID)
			}
		}
	}
	sort.Strings(dispatchable)
	return dispatchable
}
