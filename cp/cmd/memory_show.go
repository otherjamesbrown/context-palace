package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/otherjamesbrown/context-palace/cp/internal/pointer"
	"github.com/spf13/cobra"
)

var memoryShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a memory with sub-memory pointers",
	Args:  cobra.ExactArgs(1),
	Example: `  cp memory show pf-aa1
  cp memory show pf-aa1 --depth 1
  cp memory show pf-aa1 --depth 2 -o json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		depth, _ := cmd.Flags().GetInt("depth")
		if depth < 0 || depth > 5 {
			return fmt.Errorf("maximum depth is 5")
		}

		// Fetch shard
		shard, err := cpClient.GetShard(ctx, id)
		if err != nil {
			return err
		}
		if shard.Type != "memory" {
			return fmt.Errorf("shard %s is type '%s', expected 'memory'. Use `cp shard show` for other types", id, shard.Type)
		}

		// Touch telemetry (record access)
		memPath, _ := cpClient.GetMemoryPath(ctx, id)
		myDepth := 0
		for _, n := range memPath {
			if n.ID == id {
				myDepth = n.Depth
				break
			}
		}
		_ = cpClient.MemoryTouch(ctx, id, cpClient.Config.Agent, myDepth)

		// Parse pointer block
		mainContent, entries, parseErr := pointer.ParseSubMemories(shard.Content)

		// Get access count from metadata
		var accessCount int
		var lastAccessed string
		if shard.Metadata != nil {
			var meta map[string]any
			if json.Unmarshal(shard.Metadata, &meta) == nil {
				if v, ok := meta["access_count"]; ok {
					if f, ok := v.(float64); ok {
						accessCount = int(f)
					}
				}
				if v, ok := meta["last_accessed"]; ok {
					if s, ok := v.(string); ok {
						lastAccessed = s
					}
				}
			}
		}

		if outputFormat == "json" {
			return showMemoryJSON(ctx, shard, mainContent, entries, accessCount, lastAccessed, depth)
		}

		// Text output
		fmt.Println(shard.Title)
		fmt.Println(strings.Repeat("─", len(shard.Title)))
		fmt.Printf("ID:       %s\n", shard.ID)
		fmt.Printf("Type:     %s\n", shard.Type)
		fmt.Printf("Status:   %s\n", shard.Status)
		if len(shard.Labels) > 0 {
			fmt.Printf("Labels:   %s\n", strings.Join(shard.Labels, ", "))
		}
		if entries != nil {
			fmt.Printf("Children: %d\n", len(entries))
		}
		if accessCount > 0 {
			accessStr := fmt.Sprintf("%d times", accessCount)
			if lastAccessed != "" {
				accessStr += fmt.Sprintf(" (last: %s)", lastAccessed)
			}
			fmt.Printf("Accessed: %s\n", accessStr)
		}

		// Content (without pointer block)
		if parseErr != nil {
			// If parse failed, show raw content with warning
			fmt.Fprintf(cmd.ErrOrStderr(), "\nWarning: sub-memories block has invalid format\n")
			fmt.Printf("\n%s\n", shard.Content)
		} else {
			fmt.Printf("\n%s\n", mainContent)
		}

		// Show sub-memories
		if depth == 0 && len(entries) > 0 {
			fmt.Println("\nSub-memories:")
			for _, e := range entries {
				fmt.Printf("  %-12s %-20s %s\n", e.ID, e.Title, e.Summary)
			}
		} else if depth > 0 && len(entries) > 0 {
			fmt.Println("\nSub-memories (expanded):")
			err := showExpandedChildren(ctx, id, depth, 1, "  ")
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func showExpandedChildren(ctx context.Context, parentID string, maxDepth, currentDepth int, prefix string) error {
	children, err := cpClient.GetMemoryChildren(ctx, parentID)
	if err != nil {
		return err
	}

	for i, child := range children {
		isLast := i == len(children)-1

		// Touch telemetry for this child
		_ = cpClient.MemoryTouch(ctx, child.ID, cpClient.Config.Agent, currentDepth)

		// Connector character
		connector := "├─"
		lineChar := "│"
		if isLast {
			connector = "└─"
			lineChar = " "
		}

		fmt.Printf("\n%s%s %s: %s\n", prefix, connector, child.ID, child.Title)

		childPrefix := prefix + lineChar + "  "
		if len(child.Labels) > 0 {
			fmt.Printf("%sLabels: %s\n", childPrefix, strings.Join(child.Labels, ", "))
		}
		if child.ChildCount > 0 {
			fmt.Printf("%sChildren: %d\n", childPrefix, child.ChildCount)
		}

		// Parse and display main content
		childMain, childEntries, _ := pointer.ParseSubMemories(child.Content)
		fmt.Printf("%s\n", childPrefix)
		// Indent content lines
		for _, line := range strings.Split(childMain, "\n") {
			fmt.Printf("%s%s\n", childPrefix, line)
		}

		// Show child's sub-memories (pointers or expanded)
		if currentDepth < maxDepth && child.ChildCount > 0 {
			if err := showExpandedChildren(ctx, child.ID, maxDepth, currentDepth+1, childPrefix); err != nil {
				return err
			}
		} else if len(childEntries) > 0 {
			fmt.Printf("%s\n", childPrefix)
			fmt.Printf("%sSub-memories:\n", childPrefix)
			for _, e := range childEntries {
				fmt.Printf("%s  %-12s %-20s %s\n", childPrefix, e.ID, e.Title, e.Summary)
			}
		}
	}
	return nil
}

func showMemoryJSON(ctx context.Context, shard *client.Shard, mainContent string, entries []pointer.SubMemoryEntry, accessCount int, lastAccessed string, depth int) error {
	result := map[string]any{
		"id":           shard.ID,
		"title":        shard.Title,
		"content":      mainContent,
		"labels":       shard.Labels,
		"access_count": accessCount,
	}
	if lastAccessed != "" {
		result["last_accessed"] = lastAccessed
	}

	if depth == 0 {
		// Pointers only
		var children []map[string]any
		for _, e := range entries {
			children = append(children, map[string]any{
				"id":      e.ID,
				"title":   e.Title,
				"summary": e.Summary,
			})
		}
		if children == nil {
			children = []map[string]any{}
		}
		result["children"] = children
	} else {
		// Expanded children
		children, err := buildExpandedJSON(ctx, shard.ID, depth, 1)
		if err != nil {
			return err
		}
		result["children"] = children
	}

	s, _ := client.FormatJSON(result)
	fmt.Println(s)
	return nil
}

func buildExpandedJSON(ctx context.Context, parentID string, maxDepth, currentDepth int) ([]map[string]any, error) {
	children, err := cpClient.GetMemoryChildren(ctx, parentID)
	if err != nil {
		return nil, err
	}

	var result []map[string]any
	for _, child := range children {
		_ = cpClient.MemoryTouch(ctx, child.ID, cpClient.Config.Agent, currentDepth)

		childMain, childEntries, _ := pointer.ParseSubMemories(child.Content)

		m := map[string]any{
			"id":           child.ID,
			"title":        child.Title,
			"content":      childMain,
			"labels":       child.Labels,
			"access_count": child.AccessCount,
		}

		if currentDepth < maxDepth && child.ChildCount > 0 {
			nested, err := buildExpandedJSON(ctx, child.ID, maxDepth, currentDepth+1)
			if err != nil {
				return nil, err
			}
			m["children"] = nested
		} else {
			// Show as pointers
			var ptrs []map[string]any
			for _, e := range childEntries {
				ptrs = append(ptrs, map[string]any{
					"id":      e.ID,
					"title":   e.Title,
					"summary": e.Summary,
				})
			}
			if ptrs == nil {
				ptrs = []map[string]any{}
			}
			m["children"] = ptrs
		}

		result = append(result, m)
	}

	if result == nil {
		result = []map[string]any{}
	}
	return result, nil
}

func init() {
	memoryShowCmd.Flags().Int("depth", 0, "Expand children inline (0-5)")

	memoryCmd.AddCommand(memoryShowCmd)
}
