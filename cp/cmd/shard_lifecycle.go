package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

// -- shard assign --

var shardAssignCmd = &cobra.Command{
	Use:     "assign <shard-id>",
	Short:   "Claim a shard (set owner + in_progress)",
	Args:    cobra.ExactArgs(1),
	Example: "  cxp shard assign pf-abc123\n  cxp shard assign pf-abc123 --agent agent-mycroft",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		agent, _ := cmd.Flags().GetString("agent")
		instance, _ := cmd.Flags().GetString("instance")
		if instance == "" {
			instance = os.Getenv("CLAUDE_INSTANCE_ID")
		}

		result, err := cpClient.AssignShard(ctx, id, agent, instance)
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
			if result.Instance != "" {
				out["instance"] = result.Instance
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		if result.Instance != "" {
			fmt.Printf("Assigned %s %q to %s (instance: %s)\n", result.ID, result.Title, result.Owner, result.Instance)
		} else {
			fmt.Printf("Assigned %s %q to %s\n", result.ID, result.Title, result.Owner)
		}
		return nil
	},
}

// -- shard next --

var shardNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Find next unblocked shard",
	Example: `  cxp shard next
  cxp shard next --global            # all open work`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		nextLimit, _ := cmd.Flags().GetInt("limit")

		if nextLimit <= 0 {
			nextLimit = 1
		}

		scopeLabel := "global"

		shards, err := cpClient.GetNextShards(ctx, nil, nextLimit)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"scope": "global",
				"next":  shards,
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

		return nil
	},
}

// -- shard board --

var shardBoardCmd = &cobra.Command{
	Use:   "board",
	Short: "Kanban view of shards by status",
	Example: `  cxp shard board                     # all open work
  cxp shard board --agent agent-mycroft`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		agentFilter, _ := cmd.Flags().GetString("agent")

		var agentPtr *string
		scopeLabel := "global"

		if agentFilter != "" {
			agentPtr = &agentFilter
			scopeLabel = agentFilter
		}

		shards, err := cpClient.GetShardBoard(ctx, nil, agentPtr)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			var openShards, inProgressShards, needsReviewShards, completedShards []client.BoardShard
			for _, s := range shards {
				switch s.Status {
				case "closed":
					completedShards = append(completedShards, s)
				case "in_progress":
					inProgressShards = append(inProgressShards, s)
				case "needs-review":
					needsReviewShards = append(needsReviewShards, s)
				default:
					openShards = append(openShards, s)
				}
			}
			out := map[string]any{
				"scope":        scopeLabel,
				"open":         openShards,
				"in_progress":  inProgressShards,
				"needs_review": needsReviewShards,
				"completed":    completedShards,
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

		if agentFilter != "" {
			fmt.Printf("%s is working on:\n\n", agentFilter)
		}

		// Group by status
		var openShards, inProgressShards, needsReviewShards, completedShards []client.BoardShard
		for _, s := range shards {
			switch s.Status {
			case "closed":
				completedShards = append(completedShards, s)
			case "in_progress":
				inProgressShards = append(inProgressShards, s)
			case "needs-review":
				needsReviewShards = append(needsReviewShards, s)
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

		if len(needsReviewShards) > 0 {
			fmt.Printf("NEEDS REVIEW (%d)\n", len(needsReviewShards))
			for _, s := range needsReviewShards {
				owner := ""
				if s.Owner != nil {
					owner = fmt.Sprintf("(%s)", shortAgent(*s.Owner))
				}
				fmt.Printf("  \u25C7 %-10s %-8s %-40s %s\n", s.ID, s.Kind, client.Truncate(s.Title, 40), owner)
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

// -- shard status --

var shardStatusCmd = &cobra.Command{
	Use:   "status <shard-id> <new-status>",
	Short: "Transition a shard to a new status",
	Long: `Valid statuses: open, ready, in_progress, needs-review, closed

Valid transitions:
  open         → ready, in_progress, closed
  ready        → open, in_progress, closed
  in_progress  → open, needs-review, closed
  needs-review → open, in_progress, closed
  closed       → open`,
	Args:    cobra.ExactArgs(2),
	Example: "  cxp shard status pf-abc123 ready\n  cxp shard status pf-abc123 needs-review",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]
		newStatus := args[1]

		result, err := cpClient.TransitionShardStatus(ctx, id, newStatus)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":         result.ID,
				"old_status": result.OldStatus,
				"new_status": result.NewStatus,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		if result.OldStatus == result.NewStatus {
			fmt.Printf("%s: already %s\n", result.ID, result.NewStatus)
		} else {
			fmt.Printf("%s: %s → %s\n", result.ID, result.OldStatus, result.NewStatus)
		}
		return nil
	},
}

func init() {
	// shard assign flags
	shardAssignCmd.Flags().String("agent", "", "Agent claiming the shard (default: config agent)")
	shardAssignCmd.Flags().String("instance", "", "Instance ID for this session (default: $CLAUDE_INSTANCE_ID)")

	// shard next flags
	shardNextCmd.Flags().Bool("global", false, "Ignore focus, search all open work")
	shardNextCmd.Flags().Int("limit", 1, "Number of candidates to return (max 10)")

	// shard board flags
	shardBoardCmd.Flags().Bool("global", false, "All open work")
	shardBoardCmd.Flags().String("agent", "", "Filter to specific agent's work")

	// Wire into shard command tree
	shardCmd.AddCommand(shardAssignCmd)
	shardCmd.AddCommand(shardStatusCmd)
	shardCmd.AddCommand(shardNextCmd)
	shardCmd.AddCommand(shardBoardCmd)
}
