package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var epicCmd = &cobra.Command{
	Use:   "epic",
	Short: "Epic operations",
	Long:  `Commands for creating and managing epics (grouped work items).`,
}

var epicCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an epic",
	Example: `  cp epic create --title "Pipeline Quality" --body "Improve extraction reliability"
  cp epic create --title "Pipeline Quality" --adopt pf-a,pf-b,pf-c
  cp epic create --title "Pipeline Quality" --adopt pf-a,pf-b --order "pf-b:pf-a"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")
		bodyFile, _ := cmd.Flags().GetString("body-file")
		adoptFlag, _ := cmd.Flags().GetString("adopt")
		orderFlag, _ := cmd.Flags().GetString("order")
		priorityFlag, _ := cmd.Flags().GetInt("priority")
		labelFlag, _ := cmd.Flags().GetString("label")

		if title == "" {
			return fmt.Errorf("--title is required")
		}
		if body != "" && bodyFile != "" {
			return fmt.Errorf("cannot use both --body and --body-file")
		}

		var content string
		if bodyFile != "" {
			data, err := os.ReadFile(bodyFile)
			if err != nil {
				return fmt.Errorf("cannot read file '%s': %v", bodyFile, err)
			}
			content = string(data)
		} else {
			content = body
		}

		var labels []string
		if labelFlag != "" {
			labels = strings.Split(labelFlag, ",")
		}

		var adoptIDs []string
		if adoptFlag != "" {
			adoptIDs = strings.Split(adoptFlag, ",")
			for i := range adoptIDs {
				adoptIDs[i] = strings.TrimSpace(adoptIDs[i])
			}
		}

		var orderEdges []client.OrderEdge
		if orderFlag != "" {
			var err error
			orderEdges, err = parseOrderFlag(orderFlag, adoptIDs)
			if err != nil {
				return err
			}
		}

		priority := &priorityFlag

		epicID, err := cpClient.CreateEpic(ctx, title, content, priority, labels, adoptIDs, orderEdges)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":    epicID,
				"title": title,
			}
			if len(adoptIDs) > 0 {
				out["adopted"] = adoptIDs
			}
			if len(orderEdges) > 0 {
				out["edges"] = orderEdges
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Created epic %s: %q\n", epicID, title)
		if len(adoptIDs) > 0 {
			fmt.Printf("  Adopted %d shards\n", len(adoptIDs))
		}
		if len(orderEdges) > 0 {
			fmt.Printf("  Set %d dependency edges\n", len(orderEdges))
		}
		return nil
	},
}

var epicShowCmd = &cobra.Command{
	Use:     "show <epic-id>",
	Short:   "Show epic detail with progress",
	Args:    cobra.ExactArgs(1),
	Example: "  cp epic show pf-abc123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		// Fetch epic shard
		shard, err := cpClient.GetShard(ctx, id)
		if err != nil {
			return err
		}
		if shard.Type != "epic" {
			return fmt.Errorf("Shard %s is type '%s', expected 'epic'", id, shard.Type)
		}

		progress, err := cpClient.GetEpicProgress(ctx, id)
		if err != nil {
			return err
		}

		children, err := cpClient.GetEpicChildren(ctx, id)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":       id,
				"title":    shard.Title,
				"priority": shard.Priority,
				"progress": progress,
				"children": children,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		// Text output
		priorityStr := ""
		if shard.Priority != nil {
			priorityStr = fmt.Sprintf("Priority: %d", *shard.Priority)
		}
		bar := renderProgressBar(progress.Completed, progress.Total, 12)
		fmt.Printf("%s (%s)\n", shard.Title, id)
		if priorityStr != "" {
			fmt.Println(priorityStr)
		}
		fmt.Printf("Progress: %s %d/%d complete\n", bar, progress.Completed, progress.Total)

		// Group children by status
		var completed, inProgress, open []client.EpicChild
		for _, ch := range children {
			switch ch.Status {
			case "closed":
				completed = append(completed, ch)
			case "in_progress":
				inProgress = append(inProgress, ch)
			default:
				open = append(open, ch)
			}
		}

		if len(completed) > 0 {
			fmt.Println("\n  COMPLETED")
			for _, ch := range completed {
				owner := ""
				if ch.Owner != nil {
					owner = fmt.Sprintf("(%s)", shortAgent(*ch.Owner))
				}
				fmt.Printf("  \u2713 %-10s %-8s %-40s %s\n", ch.ID, ch.Kind, client.Truncate(ch.Title, 40), owner)
			}
		}

		if len(inProgress) > 0 {
			fmt.Println("\n  IN PROGRESS")
			for _, ch := range inProgress {
				owner := ""
				if ch.Owner != nil {
					owner = shortAgent(*ch.Owner)
				}
				since := ""
				if ch.AssignedAt != nil {
					since = fmt.Sprintf(", since %s", ch.AssignedAt.Format("15:04"))
				}
				fmt.Printf("  \u2192 %-10s %-8s %-40s (%s%s)\n", ch.ID, ch.Kind, client.Truncate(ch.Title, 40), owner, since)
			}
		}

		if len(open) > 0 {
			fmt.Println("\n  OPEN")
			for _, ch := range open {
				blocked := ""
				if len(ch.BlockedBy) > 0 {
					blocked = fmt.Sprintf("blocked by: %s", strings.Join(ch.BlockedBy, ", "))
				}
				fmt.Printf("    %-10s %-8s %-40s %s\n", ch.ID, ch.Kind, client.Truncate(ch.Title, 40), blocked)
			}
		}

		fmt.Println()
		return nil
	},
}

var epicListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List epics with progress",
	Example: "  cp epic list\n  cp epic list --status all",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		statusFlag, _ := cmd.Flags().GetString("status")
		if statusFlag == "" {
			statusFlag = "open"
		}

		// Fetch epic shards
		opts := client.ListShardsOpts{
			Types: []string{"epic"},
			Limit: limitFlag,
		}
		if statusFlag != "all" {
			if statusFlag == "open" {
				opts.Status = []string{"open", "in_progress"}
			} else {
				opts.Status = []string{statusFlag}
			}
		}

		results, err := cpClient.ListShardsFiltered(ctx, opts)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			type epicListItem struct {
				ID       string                `json:"id"`
				Title    string                `json:"title"`
				Status   string                `json:"status"`
				Progress *client.EpicProgress  `json:"progress"`
			}
			var items []epicListItem
			for _, r := range results {
				progress, _ := cpClient.GetEpicProgress(ctx, r.ID)
				items = append(items, epicListItem{
					ID:       r.ID,
					Title:    r.Title,
					Status:   r.Status,
					Progress: progress,
				})
			}
			s, _ := client.FormatJSON(items)
			fmt.Println(s)
			return nil
		}

		if len(results) == 0 {
			fmt.Println("No epics found.")
			return nil
		}

		fmt.Println("EPICS")
		fmt.Println(strings.Repeat("\u2500", 5))

		tbl := client.NewTable("ID", "PROGRESS", "PRIORITY", "TITLE")
		for _, r := range results {
			progress, _ := cpClient.GetEpicProgress(ctx, r.ID)
			bar := ""
			if progress != nil {
				bar = fmt.Sprintf("%s %d/%d", renderProgressBar(progress.Completed, progress.Total, 8), progress.Completed, progress.Total)
			}

			// Get priority from shard
			shard, _ := cpClient.GetShard(ctx, r.ID)
			priStr := ""
			if shard != nil && shard.Priority != nil {
				priStr = fmt.Sprintf("P%d", *shard.Priority)
			}

			titleStr := r.Title
			if r.Status == "closed" {
				titleStr += "  DONE"
			}

			tbl.AddRow(r.ID, bar, priStr, titleStr)
		}
		fmt.Print(tbl.String())
		return nil
	},
}

// parseOrderFlag parses "--order pf-a:pf-b,pf-c:pf-b" into OrderEdge pairs
func parseOrderFlag(s string, adoptIDs []string) ([]client.OrderEdge, error) {
	adoptSet := make(map[string]bool)
	for _, id := range adoptIDs {
		adoptSet[id] = true
	}

	var edges []client.OrderEdge
	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("Invalid order format '%s', expected 'child:blocker'", pair)
		}
		from := strings.TrimSpace(parts[0])
		blockedBy := strings.TrimSpace(parts[1])

		if from == blockedBy {
			return nil, fmt.Errorf("Shard cannot block itself: %s", from)
		}

		if len(adoptIDs) > 0 {
			if !adoptSet[from] {
				return nil, fmt.Errorf("Shard %s not in adopt list", from)
			}
			if !adoptSet[blockedBy] {
				return nil, fmt.Errorf("Shard %s not in adopt list", blockedBy)
			}
		}

		edges = append(edges, client.OrderEdge{From: from, BlockedBy: blockedBy})
	}

	// Check for simple circular deps
	blocksMap := make(map[string][]string)
	for _, e := range edges {
		blocksMap[e.From] = append(blocksMap[e.From], e.BlockedBy)
	}
	for _, e := range edges {
		for _, b := range blocksMap[e.BlockedBy] {
			if b == e.From {
				return nil, fmt.Errorf("Circular dependency detected: %s and %s block each other", e.From, e.BlockedBy)
			}
		}
	}

	return edges, nil
}

// renderProgressBar renders a progress bar like "███████░░░"
func renderProgressBar(completed, total int, width int) string {
	if total == 0 {
		return strings.Repeat("\u2591", width)
	}
	filled := (completed * width) / total
	if filled > width {
		filled = width
	}
	return strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", width-filled)
}

// shortAgent strips "agent-" prefix for display
func shortAgent(agent string) string {
	return strings.TrimPrefix(agent, "agent-")
}

func init() {
	// epic create flags
	epicCreateCmd.Flags().String("title", "", "Epic title (required)")
	epicCreateCmd.Flags().String("body", "", "Epic description")
	epicCreateCmd.Flags().String("body-file", "", "Read body from file")
	epicCreateCmd.Flags().String("adopt", "", "Comma-separated shard IDs to adopt")
	epicCreateCmd.Flags().String("order", "", "Dependency edges: child:blocker,child:blocker")
	epicCreateCmd.Flags().Int("priority", 2, "Priority 0-4")
	epicCreateCmd.Flags().String("label", "", "Additional labels")

	// epic list flags
	epicListCmd.Flags().String("status", "open", "Filter: open, closed, all")

	// Wire command tree
	epicCmd.AddCommand(epicCreateCmd)
	epicCmd.AddCommand(epicShowCmd)
	epicCmd.AddCommand(epicListCmd)

	rootCmd.AddCommand(epicCmd)
}
