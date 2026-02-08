package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var memoryTreeCmd = &cobra.Command{
	Use:   "tree [root-id]",
	Short: "Show memory hierarchy",
	Args:  cobra.MaximumNArgs(1),
	Example: `  cp memory tree
  cp memory tree pf-aa1
  cp memory tree --stats
  cp memory tree --max-depth 2`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		showStats, _ := cmd.Flags().GetBool("stats")
		maxDepth, _ := cmd.Flags().GetInt("max-depth")

		var rootID *string
		if len(args) > 0 {
			rootID = &args[0]
		}

		nodes, err := cpClient.GetMemoryTree(ctx, rootID)
		if err != nil {
			return err
		}

		if maxDepth > 0 {
			var filtered []client.MemoryTreeNode
			for _, n := range nodes {
				if n.Depth <= maxDepth {
					filtered = append(filtered, n)
				}
			}
			nodes = filtered
		}

		if outputFormat == "json" {
			jsonTree := buildJSONTree(nodes, showStats)
			s, _ := client.FormatJSON(jsonTree)
			fmt.Println(s)
			return nil
		}

		if len(nodes) == 0 {
			fmt.Println("No memories found.")
			return nil
		}

		// Build access count map for promotion detection
		accessMap := map[string]int{}
		for _, n := range nodes {
			accessMap[n.ID] = n.AccessCount
		}

		// Group by root for display
		printTree(nodes, showStats, accessMap)
		return nil
	},
}

// printTree renders the memory tree with Unicode box-drawing characters.
func printTree(nodes []client.MemoryTreeNode, showStats bool, accessMap map[string]int) {
	// Build parent -> children map
	children := map[string][]client.MemoryTreeNode{}
	var roots []client.MemoryTreeNode

	for _, n := range nodes {
		if n.ParentID == nil || *n.ParentID == "" {
			roots = append(roots, n)
		} else {
			children[*n.ParentID] = append(children[*n.ParentID], n)
		}
	}

	for _, root := range roots {
		printNode(root, "", true, true, showStats, children, accessMap)
	}
}

func printNode(node client.MemoryTreeNode, prefix string, isLast bool, isRoot bool, showStats bool, children map[string][]client.MemoryTreeNode, accessMap map[string]int) {
	var line strings.Builder

	if isRoot {
		// Root nodes have no prefix
	} else if isLast {
		line.WriteString(prefix + "└── ")
	} else {
		line.WriteString(prefix + "├── ")
	}

	line.WriteString(fmt.Sprintf("%s (%s)", node.Title, node.ID))

	if showStats {
		statsStr := fmt.Sprintf("  %d reads", node.AccessCount)
		if node.LastAccessed != nil {
			statsStr += fmt.Sprintf("  last: %s", formatTimeAgo(*node.LastAccessed))
		}
		// Check promotion candidate (accessed more than parent)
		if node.ParentID != nil {
			parentAccess, ok := accessMap[*node.ParentID]
			if ok && node.AccessCount > parentAccess {
				statsStr += "  ★"
			}
		}
		// Pad to align stats
		padding := 60 - len(line.String())
		if padding < 2 {
			padding = 2
		}
		line.WriteString(strings.Repeat(" ", padding))
		line.WriteString(statsStr)
	}

	fmt.Println(line.String())

	// Print children
	childNodes := children[node.ID]
	newPrefix := prefix
	if !isRoot {
		if isLast {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}
	}
	for i, child := range childNodes {
		isLastChild := i == len(childNodes)-1
		printNode(child, newPrefix, isLastChild, false, showStats, children, accessMap)
	}
}

// buildJSONTree converts flat nodes to nested JSON structure.
func buildJSONTree(nodes []client.MemoryTreeNode, _ bool) []map[string]any {
	childMap := map[string][]client.MemoryTreeNode{}
	var roots []client.MemoryTreeNode

	for _, n := range nodes {
		if n.ParentID == nil || *n.ParentID == "" {
			roots = append(roots, n)
		} else {
			childMap[*n.ParentID] = append(childMap[*n.ParentID], n)
		}
	}

	var result []map[string]any
	for _, root := range roots {
		result = append(result, buildJSONNode(root, childMap))
	}
	return result
}

func buildJSONNode(node client.MemoryTreeNode, childMap map[string][]client.MemoryTreeNode) map[string]any {
	m := map[string]any{
		"id":           node.ID,
		"title":        node.Title,
		"depth":        node.Depth,
		"access_count": node.AccessCount,
	}
	if node.Summary != nil {
		m["summary"] = *node.Summary
	}

	children := childMap[node.ID]
	var childJSON []map[string]any
	for _, ch := range children {
		childJSON = append(childJSON, buildJSONNode(ch, childMap))
	}
	if childJSON == nil {
		childJSON = []map[string]any{}
	}
	m["children"] = childJSON
	return m
}

var memoryHotCmd = &cobra.Command{
	Use:   "hot",
	Short: "Show promotion candidates",
	Example: `  cp memory hot
  cp memory hot --min-depth 2
  cp memory hot --limit 10`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		minDepth, _ := cmd.Flags().GetInt("min-depth")
		limit, _ := cmd.Flags().GetInt("limit")
		if limit == 0 {
			limit = 20
		}

		results, err := cpClient.GetMemoryHot(ctx, minDepth, limit)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(results)
			fmt.Println(s)
			return nil
		}

		if len(results) == 0 {
			fmt.Println("No promotion candidates found.")
			return nil
		}

		fmt.Println("PROMOTION CANDIDATES (children accessed more than parent)")
		fmt.Println(strings.Repeat("─", 57))

		tbl := client.NewTable("ID", "DEPTH", "READS", "PARENT READS", "TITLE", "PARENT")
		for _, r := range results {
			tbl.AddRow(
				r.ID,
				fmt.Sprintf("%d", r.Depth),
				fmt.Sprintf("%d", r.AccessCount),
				fmt.Sprintf("%d", r.ParentAccessCount),
				client.Truncate(r.Title, 25),
				client.Truncate(r.ParentTitle, 20),
			)
		}
		fmt.Print(tbl.String())

		// Suggestion for top candidate
		if len(results) > 0 {
			top := results[0]
			if top.ParentAccessCount > 0 {
				ratio := float64(top.AccessCount) / float64(top.ParentAccessCount)
				fmt.Printf("\nSuggestion: %s has %.0fx parent reads — consider promoting.\n", top.ID, ratio)
			}
		}
		return nil
	},
}

var memorySyncCmd = &cobra.Command{
	Use:   "sync [parent-id]",
	Short: "Reconcile pointer blocks",
	Args:  cobra.MaximumNArgs(1),
	Example: `  cp memory sync --dry-run
  cp memory sync
  cp memory sync pf-aa1`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		var parentID *string
		if len(args) > 0 {
			parentID = &args[0]
		}

		result, err := cpClient.SyncMemoryPointers(ctx, parentID, dryRun)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		if len(result.Discrepancies) == 0 {
			fmt.Println("All pointer blocks are in sync. No changes needed.")
			return nil
		}

		if dryRun {
			fmt.Printf("Sync check: %d parents, %d discrepancies\n\n", result.ParentsChecked, len(result.Discrepancies))
			// Group by parent
			grouped := map[string][]client.SyncDiscrepancy{}
			for _, d := range result.Discrepancies {
				grouped[d.ParentID] = append(grouped[d.ParentID], d)
			}
			for pid, discs := range grouped {
				fmt.Printf("  %s:\n", pid)
				for _, d := range discs {
					switch d.Type {
					case "missing_pointer":
						fmt.Printf("    MISSING: child %s %q not in pointer block\n", d.ChildID, d.ChildTitle)
					case "stale_pointer":
						fmt.Printf("    STALE: pointer to %s — shard no longer exists\n", d.ChildID)
					}
				}
			}
			fmt.Printf("\nRun `cp memory sync` to fix.\n")
		} else {
			fmt.Printf("Sync: %d parents checked, %d fixed\n\n", result.ParentsChecked, len(result.Discrepancies))
			for _, d := range result.Discrepancies {
				switch d.Type {
				case "missing_pointer":
					fmt.Printf("  %s: added pointer for %s (placeholder summary)\n", d.ParentID, d.ChildID)
				case "stale_pointer":
					fmt.Printf("  %s: removed stale pointer to %s\n", d.ParentID, d.ChildID)
				}
			}
		}
		return nil
	},
}

// formatTimeAgo returns a human-readable time ago string.
func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

func init() {
	// tree flags
	memoryTreeCmd.Flags().Bool("stats", false, "Show access counts and promotion markers")
	memoryTreeCmd.Flags().Int("max-depth", 0, "Max tree depth to display")

	// hot flags
	memoryHotCmd.Flags().Int("min-depth", 1, "Minimum tree depth to consider")
	memoryHotCmd.Flags().Int("limit", 20, "Max results")

	// sync flags
	memorySyncCmd.Flags().Bool("dry-run", false, "Report without fixing")

	memoryCmd.AddCommand(memoryTreeCmd)
	memoryCmd.AddCommand(memoryHotCmd)
	memoryCmd.AddCommand(memorySyncCmd)
}
