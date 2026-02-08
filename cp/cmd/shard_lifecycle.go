package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

// -- shard assign --

var shardAssignCmd = &cobra.Command{
	Use:     "assign <shard-id>",
	Short:   "Claim a shard (set owner + in_progress)",
	Args:    cobra.ExactArgs(1),
	Example: "  cp shard assign pf-abc123\n  cp shard assign pf-abc123 --agent agent-mycroft",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		agent, _ := cmd.Flags().GetString("agent")

		result, err := cpClient.AssignShard(ctx, id, agent)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":     result.ID,
				"title":  result.Title,
				"owner":  result.Owner,
				"status": "in_progress",
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Assigned %s %q to %s\n", result.ID, result.Title, result.Owner)
		return nil
	},
}

// -- shard next --

var shardNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Find next unblocked shard",
	Example: `  cp shard next                     # within focused epic (if set)
  cp shard next --epic pf-abc123    # within specific epic
  cp shard next --global            # all open work`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		epicFlag, _ := cmd.Flags().GetString("epic")
		globalFlag, _ := cmd.Flags().GetBool("global")
		nextLimit, _ := cmd.Flags().GetInt("limit")

		if nextLimit <= 0 {
			nextLimit = 1
		}

		// Determine scope
		var epicID *string
		scopeLabel := "global"

		if epicFlag != "" {
			epicID = &epicFlag
			scopeLabel = fmt.Sprintf("epic %q", epicFlag)
		} else if !globalFlag {
			// Check focus
			focus, _ := cpClient.GetFocus(ctx)
			if focus != nil {
				epicID = &focus.EpicID
				scopeLabel = fmt.Sprintf("epic %q", focus.EpicTitle)
			}
		}

		shards, err := cpClient.GetNextShards(ctx, epicID, nextLimit)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			scope := "global"
			if epicID != nil {
				scope = "epic"
			}
			out := map[string]any{
				"scope": scope,
				"next":  shards,
			}
			if epicID != nil {
				out["epic_id"] = *epicID
			}
			// If scoped to epic, show upcoming blocked items
			if epicID != nil {
				children, _ := cpClient.GetEpicChildren(ctx, *epicID)
				var upcoming []client.EpicChild
				for _, ch := range children {
					if ch.Status == "open" && len(ch.BlockedBy) > 0 {
						upcoming = append(upcoming, ch)
					}
				}
				if len(upcoming) > 0 {
					out["upcoming"] = upcoming
				}
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		if len(shards) == 0 {
			fmt.Printf("No unblocked shards available (%s).\n", scopeLabel)
			return nil
		}

		fmt.Printf("Next up (%s):\n", scopeLabel)
		for _, s := range shards {
			blocked := "no blockers"
			pri := "P?"
			if s.Priority != nil {
				pri = fmt.Sprintf("P%d", *s.Priority)
			}
			fmt.Printf("  %-10s %-8s %-40s (%s, %s)\n", s.ID, s.Kind, client.Truncate(s.Title, 40), pri, blocked)
		}

		// If scoped to epic, show "after that" section
		if epicID != nil {
			children, _ := cpClient.GetEpicChildren(ctx, *epicID)
			var upcoming []client.EpicChild
			for _, ch := range children {
				if ch.Status == "open" && len(ch.BlockedBy) > 0 {
					upcoming = append(upcoming, ch)
				}
			}
			if len(upcoming) > 0 {
				fmt.Println("\n  After that:")
				for _, ch := range upcoming {
					fmt.Printf("  %-10s %-8s %-40s (blocked by: %s)\n",
						ch.ID, ch.Kind, client.Truncate(ch.Title, 40),
						strings.Join(ch.BlockedBy, ", "))
				}
			}
		}

		return nil
	},
}

// -- shard board --

var shardBoardCmd = &cobra.Command{
	Use:   "board",
	Short: "Kanban view of shards by status",
	Example: `  cp shard board                     # within focused epic
  cp shard board --epic pf-abc123    # specific epic
  cp shard board --global            # all open work
  cp shard board --agent agent-mycroft`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		epicFlag, _ := cmd.Flags().GetString("epic")
		globalFlag, _ := cmd.Flags().GetBool("global")
		agentFilter, _ := cmd.Flags().GetString("agent")

		// Determine scope
		var epicID *string
		var agentPtr *string
		scopeLabel := "global"

		if agentFilter != "" {
			agentPtr = &agentFilter
			scopeLabel = agentFilter
		}

		if epicFlag != "" {
			epicID = &epicFlag
			scopeLabel = epicFlag
		} else if !globalFlag && agentFilter == "" {
			// Check focus
			focus, _ := cpClient.GetFocus(ctx)
			if focus != nil {
				epicID = &focus.EpicID
				scopeLabel = fmt.Sprintf("%s (%s)", focus.EpicTitle, focus.EpicID)
			}
		}

		shards, err := cpClient.GetShardBoard(ctx, epicID, agentPtr)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			var openShards, inProgressShards, completedShards []client.BoardShard
			for _, s := range shards {
				switch s.Status {
				case "closed":
					completedShards = append(completedShards, s)
				case "in_progress":
					inProgressShards = append(inProgressShards, s)
				default:
					openShards = append(openShards, s)
				}
			}
			out := map[string]any{
				"scope":       scopeLabel,
				"open":        openShards,
				"in_progress": inProgressShards,
				"completed":   completedShards,
			}
			if epicID != nil {
				out["epic_id"] = *epicID
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		if len(shards) == 0 {
			if agentFilter != "" {
				fmt.Printf("%s has no active or recent work.\n", agentFilter)
			} else {
				fmt.Println("No shards found.")
			}
			return nil
		}

		// Show epic header with progress if applicable
		if epicID != nil {
			progress, _ := cpClient.GetEpicProgress(ctx, *epicID)
			if progress != nil {
				bar := renderProgressBar(progress.Completed, progress.Total, 10)
				fmt.Printf("%s  %s %d/%d\n\n", scopeLabel, bar, progress.Completed, progress.Total)
			} else {
				fmt.Printf("%s\n\n", scopeLabel)
			}
		} else if agentFilter != "" {
			fmt.Printf("%s is working on:\n\n", agentFilter)
		}

		// Group by status
		var openShards, inProgressShards, completedShards []client.BoardShard
		for _, s := range shards {
			switch s.Status {
			case "closed":
				completedShards = append(completedShards, s)
			case "in_progress":
				inProgressShards = append(inProgressShards, s)
			default:
				openShards = append(openShards, s)
			}
		}

		if len(openShards) > 0 {
			fmt.Printf("OPEN (%d)\n", len(openShards))
			for _, s := range openShards {
				blocked := ""
				if len(s.BlockedBy) > 0 {
					blocked = fmt.Sprintf("blocked by: %s", strings.Join(s.BlockedBy, ", "))
				}
				fmt.Printf("  %-10s %-8s %-40s %s\n", s.ID, s.Kind, client.Truncate(s.Title, 40), blocked)
			}
			fmt.Println()
		}

		if len(inProgressShards) > 0 {
			fmt.Printf("IN PROGRESS (%d)\n", len(inProgressShards))
			for _, s := range inProgressShards {
				owner := ""
				if s.Owner != nil {
					owner = shortAgent(*s.Owner)
				}
				ago := ""
				if s.AssignedAt != nil {
					ago = fmt.Sprintf(", %s", timeAgo(*s.AssignedAt))
				}
				fmt.Printf("  \u2192 %-10s %-8s %-40s (%s%s)\n", s.ID, s.Kind, client.Truncate(s.Title, 40), owner, ago)
			}
			fmt.Println()
		}

		if len(completedShards) > 0 {
			fmt.Printf("COMPLETED (%d)\n", len(completedShards))
			for _, s := range completedShards {
				owner := ""
				if s.Owner != nil {
					owner = fmt.Sprintf("(%s)", shortAgent(*s.Owner))
				}
				fmt.Printf("  \u2713 %-10s %-8s %-40s %s\n", s.ID, s.Kind, client.Truncate(s.Title, 40), owner)
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	// shard assign flags
	shardAssignCmd.Flags().String("agent", "", "Agent claiming the shard (default: config agent)")

	// shard next flags
	shardNextCmd.Flags().String("epic", "", "Scope to specific epic")
	shardNextCmd.Flags().Bool("global", false, "Ignore focus, search all open work")
	shardNextCmd.Flags().Int("limit", 1, "Number of candidates to return (max 10)")

	// shard board flags
	shardBoardCmd.Flags().String("epic", "", "Scope to specific epic")
	shardBoardCmd.Flags().Bool("global", false, "All open work across epics")
	shardBoardCmd.Flags().String("agent", "", "Filter to specific agent's work")

	// Wire into shard command tree
	shardCmd.AddCommand(shardAssignCmd)
	shardCmd.AddCommand(shardNextCmd)
	shardCmd.AddCommand(shardBoardCmd)
}
