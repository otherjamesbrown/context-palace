package tui

import (
	"fmt"
	"sort"
	"time"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
)

// TreeNode represents a node in the navigable tree
type TreeNode struct {
	ID              string
	Title           string
	Type            string
	Status          string
	Labels          []string
	ChildCount      int
	Depth           int
	Expanded        bool
	Loaded          bool // true if children have been fetched
	IsGroup         bool // virtual group node (not a real shard)
	CreatedAt       time.Time
	Parent          *TreeNode
	Children        []*TreeNode
	EdgeTrigger     string // edge trigger text (knowledge tree)
	EdgeDescription string // edge description text (knowledge tree)
}

// BuildRoots converts ShardTreeNode slices into TreeNode roots
func BuildRoots(nodes []client.ShardTreeNode) []*TreeNode {
	roots := make([]*TreeNode, len(nodes))
	for i, n := range nodes {
		roots[i] = &TreeNode{
			ID:         n.ID,
			Title:      n.Title,
			Type:       n.Type,
			Status:     n.Status,
			Labels:     n.Labels,
			ChildCount: n.ChildCount,
			CreatedAt:  n.CreatedAt,
			Depth:      0,
		}
	}
	return roots
}

// InsertChildren adds child nodes under a parent
func InsertChildren(parent *TreeNode, nodes []client.ShardTreeNode) {
	parent.Loaded = true
	parent.Children = make([]*TreeNode, len(nodes))
	for i, n := range nodes {
		parent.Children[i] = &TreeNode{
			ID:         n.ID,
			Title:      n.Title,
			Type:       n.Type,
			Status:     n.Status,
			Labels:     n.Labels,
			ChildCount: n.ChildCount,
			CreatedAt:  n.CreatedAt,
			Depth:      parent.Depth + 1,
			Parent:     parent,
		}
	}
	parent.ChildCount = len(nodes)
}

// Flatten walks expanded tree nodes into a linear list for rendering
func Flatten(roots []*TreeNode) []*TreeNode {
	var flat []*TreeNode
	for _, root := range roots {
		flattenNode(root, &flat)
	}
	return flat
}

func flattenNode(node *TreeNode, flat *[]*TreeNode) {
	*flat = append(*flat, node)
	if node.Expanded {
		for _, child := range node.Children {
			flattenNode(child, flat)
		}
	}
}

// ExpandAll expands all loaded nodes recursively
func ExpandAll(roots []*TreeNode) {
	for _, root := range roots {
		expandAllNode(root)
	}
}

func expandAllNode(node *TreeNode) {
	if node.Loaded && node.ChildCount > 0 {
		node.Expanded = true
		for _, child := range node.Children {
			expandAllNode(child)
		}
	}
}

// CollapseAll collapses all nodes
func CollapseAll(roots []*TreeNode) {
	for _, root := range roots {
		collapseAllNode(root)
	}
}

func collapseAllNode(node *TreeNode) {
	node.Expanded = false
	for _, child := range node.Children {
		collapseAllNode(child)
	}
}

// GroupByDate groups root nodes by creation date (newest first).
func GroupByDate(roots []*TreeNode) []*TreeNode {
	buckets := make(map[string][]*TreeNode)
	for _, r := range roots {
		day := r.CreatedAt.Format("2006-01-02")
		buckets[day] = append(buckets[day], r)
	}

	// Sort date keys newest first
	days := make([]string, 0, len(buckets))
	for d := range buckets {
		days = append(days, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(days)))

	groups := make([]*TreeNode, 0, len(days))
	for _, day := range days {
		children := buckets[day]
		// Sort children within group newest first
		sort.Slice(children, func(i, j int) bool {
			return children[i].CreatedAt.After(children[j].CreatedAt)
		})

		g := &TreeNode{
			ID:         "group:date:" + day,
			Title:      day,
			IsGroup:    true,
			Loaded:     true,
			Expanded:   false,
			ChildCount: len(children),
			Depth:      0,
		}
		for _, c := range children {
			c.Parent = g
			c.Depth = 1
		}
		g.Children = children
		groups = append(groups, g)
	}
	return groups
}

// GroupByLabel groups root nodes by their first label (alphabetical, unlabeled last).
func GroupByLabel(roots []*TreeNode) []*TreeNode {
	const unlabeled = "(unlabeled)"
	buckets := make(map[string][]*TreeNode)
	for _, r := range roots {
		label := unlabeled
		if len(r.Labels) > 0 {
			label = r.Labels[0]
		}
		buckets[label] = append(buckets[label], r)
	}

	// Sort labels alphabetically, unlabeled last
	labels := make([]string, 0, len(buckets))
	for l := range buckets {
		labels = append(labels, l)
	}
	sort.Slice(labels, func(i, j int) bool {
		if labels[i] == unlabeled {
			return false
		}
		if labels[j] == unlabeled {
			return true
		}
		return labels[i] < labels[j]
	})

	groups := make([]*TreeNode, 0, len(labels))
	for _, label := range labels {
		children := buckets[label]
		g := &TreeNode{
			ID:         "group:label:" + label,
			Title:      label,
			IsGroup:    true,
			Loaded:     true,
			Expanded:   false,
			ChildCount: len(children),
			Depth:      0,
		}
		for _, c := range children {
			c.Parent = g
			c.Depth = 1
		}
		g.Children = children
		groups = append(groups, g)
	}
	return groups
}

// GroupByStatus groups root nodes by workflow status in a fixed order.
// Order: In Progress > Ready > Open > Needs Review > Closed > Deferred
func GroupByStatus(roots []*TreeNode) []*TreeNode {
	statusOrder := []string{"in_progress", "ready", "open", "needs-review", "closed", "deferred"}
	statusLabels := map[string]string{
		"in_progress":  "In Progress",
		"ready":        "Ready",
		"open":         "Open",
		"needs-review": "Needs Review",
		"closed":       "Closed",
		"deferred":     "Deferred",
	}

	buckets := make(map[string][]*TreeNode)
	for _, r := range roots {
		buckets[r.Status] = append(buckets[r.Status], r)
	}

	var groups []*TreeNode
	for _, status := range statusOrder {
		children, ok := buckets[status]
		if !ok || len(children) == 0 {
			continue
		}
		label := statusLabels[status]
		if label == "" {
			label = status
		}
		g := &TreeNode{
			ID:         "group:status:" + status,
			Title:      label,
			IsGroup:    true,
			Loaded:     true,
			Expanded:   status != "closed" && status != "deferred",
			ChildCount: len(children),
			Depth:      0,
		}
		for _, c := range children {
			c.Parent = g
			c.Depth = 1
		}
		g.Children = children
		groups = append(groups, g)
	}

	// Any statuses not in the predefined order
	for status, children := range buckets {
		found := false
		for _, s := range statusOrder {
			if s == status {
				found = true
				break
			}
		}
		if !found {
			g := &TreeNode{
				ID:         "group:status:" + status,
				Title:      status,
				IsGroup:    true,
				Loaded:     true,
				Expanded:   true,
				ChildCount: len(children),
				Depth:      0,
			}
			for _, c := range children {
				c.Parent = g
				c.Depth = 1
			}
			g.Children = children
			groups = append(groups, g)
		}
	}

	return groups
}

// BuildKBTreeNodes converts KBTreeNode slices into a hierarchical TreeNode tree.
// KBTreeNodes come pre-ordered with depth info from the DB function.
func BuildKBTreeNodes(nodes []client.KBTreeNode) []*TreeNode {
	if len(nodes) == 0 {
		return nil
	}

	// Build flat nodes first
	nodeMap := make(map[string]*TreeNode)
	var allNodes []*TreeNode
	for _, n := range nodes {
		tn := &TreeNode{
			ID:         n.ID,
			Title:      n.Title,
			Type:       n.Type,
			Status:     n.Status,
			Labels:     n.Labels,
			ChildCount: n.ChildCount,
			Depth:      n.Depth,
			CreatedAt:  n.CreatedAt,
			Loaded:     true,
			Expanded:   n.Depth < 2, // expand first 2 levels
		}
		if n.EdgeTrigger != nil {
			tn.EdgeTrigger = *n.EdgeTrigger
		}
		if n.EdgeDescription != nil {
			tn.EdgeDescription = *n.EdgeDescription
		}
		nodeMap[n.ID] = tn
		allNodes = append(allNodes, tn)
	}

	// Wire parent-child relationships using ParentPath
	var roots []*TreeNode
	for i, n := range nodes {
		tn := allNodes[i]
		if len(n.ParentPath) > 0 {
			parentID := n.ParentPath[len(n.ParentPath)-1]
			if parent, ok := nodeMap[parentID]; ok {
				tn.Parent = parent
				parent.Children = append(parent.Children, tn)
				continue
			}
		}
		// No parent found — treat as root
		roots = append(roots, tn)
	}

	return roots
}

// BuildSearchTree converts a ShardContextResult into a hierarchy tree.
// Returns the tree roots and the flat-list index of the target shard.
func BuildSearchTree(ctx *client.ShardContextResult) ([]*TreeNode, int) {
	target := &TreeNode{
		ID:         ctx.Target.ID,
		Title:      ctx.Target.Title,
		Type:       ctx.Target.Type,
		Status:     ctx.Target.Status,
		Labels:     ctx.Target.Labels,
		ChildCount: ctx.Target.ChildCount,
		Loaded:     true,
		Expanded:   len(ctx.Children) > 0,
	}

	// Build children of target
	for _, c := range ctx.Children {
		child := &TreeNode{
			ID:         c.ID,
			Title:      c.Title,
			Type:       c.Type,
			Status:     c.Status,
			Labels:     c.Labels,
			ChildCount: c.ChildCount,
			CreatedAt:  c.CreatedAt,
			Parent:     target,
		}
		target.Children = append(target.Children, child)
	}

	if ctx.Parent == nil {
		// No parent: target at depth 0
		target.Depth = 0
		setChildDepths(target)
		flat := Flatten([]*TreeNode{target})
		return []*TreeNode{target}, indexOf(flat, target.ID)
	}

	// Parent exists: parent at depth 0, siblings + target at depth 1
	parent := &TreeNode{
		ID:         ctx.Parent.ID,
		Title:      ctx.Parent.Title,
		Type:       ctx.Parent.Type,
		Status:     ctx.Parent.Status,
		Labels:     ctx.Parent.Labels,
		ChildCount: ctx.Parent.ChildCount,
		Loaded:     true,
		Expanded:   true,
		Depth:      0,
	}

	// Add siblings before target, then target, preserving order
	var allChildren []*TreeNode
	for _, s := range ctx.Siblings {
		sib := &TreeNode{
			ID:         s.ID,
			Title:      s.Title,
			Type:       s.Type,
			Status:     s.Status,
			Labels:     s.Labels,
			ChildCount: s.ChildCount,
			CreatedAt:  s.CreatedAt,
			Depth:      1,
			Parent:     parent,
		}
		allChildren = append(allChildren, sib)
	}

	target.Depth = 1
	target.Parent = parent
	setChildDepths(target)
	allChildren = append(allChildren, target)

	parent.Children = allChildren
	parent.ChildCount = len(allChildren)

	flat := Flatten([]*TreeNode{parent})
	return []*TreeNode{parent}, indexOf(flat, target.ID)
}

func setChildDepths(node *TreeNode) {
	for _, c := range node.Children {
		c.Depth = node.Depth + 1
		setChildDepths(c)
	}
}

func indexOf(flat []*TreeNode, id string) int {
	for i, n := range flat {
		if n.ID == id {
			return i
		}
	}
	return 0
}

// BuildBoardTree converts a BoardResult into TreeNodes for the board tab.
func BuildBoardTree(result *client.BoardResult) []*TreeNode {
	var roots []*TreeNode

	if len(result.Focus) > 0 {
		roots = append(roots, boardGroup(
			"group:board:focus",
			fmt.Sprintf("Focus (%d)", len(result.Focus)),
			result.Focus,
		))
	}

	if len(result.NeedsReview) > 0 {
		roots = append(roots, boardGroup(
			"group:board:needs-review",
			fmt.Sprintf("Needs Review (%d)", len(result.NeedsReview)),
			result.NeedsReview,
		))
	}

	if len(result.Blocked) > 0 {
		roots = append(roots, boardGroup(
			"group:board:blocked",
			fmt.Sprintf("Blocked (%d)", len(result.Blocked)),
			result.Blocked,
		))
	}

	if len(result.RecentActivity) > 0 {
		roots = append(roots, boardGroup(
			"group:board:recent",
			fmt.Sprintf("Recent Activity (%d)", len(result.RecentActivity)),
			result.RecentActivity,
		))
	}

	for _, g := range result.Groups {
		roots = append(roots, boardGroup(
			"group:board:type:"+g.Type,
			fmt.Sprintf("%s (%d)", g.Type, len(g.Items)),
			g.Items,
		))
	}

	return roots
}

func boardGroup(id, title string, entries []client.BoardEntry) *TreeNode {
	g := &TreeNode{
		ID:         id,
		Title:      title,
		IsGroup:    true,
		Loaded:     true,
		Expanded:   true,
		ChildCount: len(entries),
		Depth:      0,
	}
	children := make([]*TreeNode, len(entries))
	for i, e := range entries {
		children[i] = &TreeNode{
			ID:         e.ID,
			Title:      e.Title,
			Type:       e.Type,
			Status:     e.Status,
			ChildCount: e.ChildCount,
			Depth:      1,
			Parent:     g,
		}
	}
	g.Children = children
	return g
}
