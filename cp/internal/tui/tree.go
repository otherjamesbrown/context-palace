package tui

import (
	"fmt"
	"sort"
	"time"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
)

// TreeNode represents a node in the navigable tree
type TreeNode struct {
	ID         string
	Title      string
	Type       string
	Status     string
	Labels     []string
	ChildCount int
	Depth      int
	Expanded   bool
	Loaded     bool // true if children have been fetched
	IsGroup    bool // virtual group node (not a real shard)
	CreatedAt  time.Time
	Parent     *TreeNode
	Children   []*TreeNode
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
