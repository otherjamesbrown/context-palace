package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var outcomeCmd = &cobra.Command{
	Use:   "outcome",
	Short: "Outcome operations",
	Long:  `Commands for creating and managing outcome shards (business goals that group designs and tasks).`,
}

var outcomeCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create an outcome (business goal)",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp outcome create "Reduce onboarding time to < 5 minutes"
  cxp outcome create "Launch v2 API" --body "Ship public API v2 by Q2"
  cxp outcome create "Improve reliability" --adopt pf-a,pf-b`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		title := args[0]

		body, _ := cmd.Flags().GetString("body")
		bodyFile, _ := cmd.Flags().GetString("body-file")
		labels, _ := cmd.Flags().GetStringSlice("label")
		adoptFlag, _ := cmd.Flags().GetString("adopt")

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

		var adoptIDs []string
		if adoptFlag != "" {
			adoptIDs = strings.Split(adoptFlag, ",")
			for i := range adoptIDs {
				adoptIDs[i] = strings.TrimSpace(adoptIDs[i])
			}
		}

		id, err := cpClient.CreateShardWithMetadata(ctx, title, content, "outcome", nil, labels, nil)
		if err != nil {
			return err
		}

		// Adopt children: create child-of edges from child → this outcome
		for _, childID := range adoptIDs {
			if err := cpClient.CreateEdgeSimple(ctx, childID, id, "child-of"); err != nil {
				return fmt.Errorf("created outcome %s but failed to adopt %s: %v", id, childID, err)
			}
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":    id,
				"type":  "outcome",
				"title": title,
			}
			if len(adoptIDs) > 0 {
				out["adopted"] = adoptIDs
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Created outcome %s: %q\n", id, title)
		if len(adoptIDs) > 0 {
			fmt.Printf("  Adopted %d shards\n", len(adoptIDs))
		}
		return nil
	},
}

var outcomeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List outcomes",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		includeClosed, _ := cmd.Flags().GetBool("closed")

		statuses := []string{"open", "ready", "in_progress", "needs-review"}
		if includeClosed {
			statuses = append(statuses, "closed")
		}

		results, err := cpClient.ListShardsFiltered(ctx, client.ListShardsOpts{
			Types:  []string{"outcome"},
			Status: statuses,
			Limit:  50,
		})
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(results)
			fmt.Println(s)
			return nil
		}

		if len(results) == 0 {
			fmt.Println("No outcomes found.")
			return nil
		}

		for _, r := range results {
			fmt.Printf("%-12s %-8s %s\n", r.ID, r.Status, r.Title)
		}
		return nil
	},
}

var outcomeShowCmd = &cobra.Command{
	Use:   "show <outcome-id>",
	Short: "Show an outcome and its children",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		shard, err := cpClient.GetShard(ctx, id)
		if err != nil {
			return err
		}
		if shard.Type != "outcome" {
			return fmt.Errorf("shard %s is type %q, expected 'outcome'", id, shard.Type)
		}

		// Get children (incoming child-of edges = shards that are child-of this outcome)
		children, err := cpClient.GetShardEdges(ctx, id, "incoming", []string{"child-of"})
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":       shard.ID,
				"title":    shard.Title,
				"type":     shard.Type,
				"status":   shard.Status,
				"children": children,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("%s  %s  [%s]\n", shard.ID, shard.Title, shard.Status)
		if shard.Content != "" {
			fmt.Printf("\n%s\n", shard.Content)
		}

		if len(children) == 0 {
			fmt.Println("\nNo children.")
		} else {
			fmt.Printf("\nChildren (%d):\n", len(children))
			for _, c := range children {
				fmt.Printf("  %-12s %-8s %-8s %s\n", c.ShardID, c.Type, c.Status, c.Title)
			}
		}
		return nil
	},
}

var outcomeCloseCmd = &cobra.Command{
	Use:   "close <outcome-id>",
	Short: "Close an outcome",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		reason, _ := cmd.Flags().GetString("reason")

		result, err := cpClient.CloseShard(ctx, id, reason)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":        result.ID,
				"status":    result.Status,
				"closed_at": result.ClosedAt.Format(time.RFC3339),
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Closed outcome %s %q\n", result.ID, result.Title)
		return nil
	},
}

var outcomeAddCmd = &cobra.Command{
	Use:   "add <outcome-id> <shard-id>...",
	Short: "Add children to an outcome",
	Args:  cobra.MinimumNArgs(2),
	Example: `  cxp outcome add pf-outcome1 pf-design1
  cxp outcome add pf-outcome1 pf-design1 pf-task2 pf-task3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		outcomeID := args[0]

		// Verify it's an outcome
		shard, err := cpClient.GetShard(ctx, outcomeID)
		if err != nil {
			return err
		}
		if shard.Type != "outcome" {
			return fmt.Errorf("shard %s is type %q, expected 'outcome'", outcomeID, shard.Type)
		}

		childIDs := args[1:]
		for _, childID := range childIDs {
			if err := cpClient.CreateEdgeSimple(ctx, childID, outcomeID, "child-of"); err != nil {
				return fmt.Errorf("failed to add %s to outcome: %v", childID, err)
			}
			if outputFormat != "json" {
				fmt.Printf("Added %s to outcome %s\n", childID, outcomeID)
			}
		}

		if outputFormat == "json" {
			out := map[string]any{
				"outcome":  outcomeID,
				"children": childIDs,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
		}
		return nil
	},
}

var outcomeRemoveCmd = &cobra.Command{
	Use:   "remove <outcome-id> <shard-id>...",
	Short: "Remove children from an outcome",
	Args:  cobra.MinimumNArgs(2),
	Example: `  cxp outcome remove pf-outcome1 pf-design1`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		outcomeID := args[0]

		childIDs := args[1:]
		for _, childID := range childIDs {
			if err := cpClient.DeleteEdge(ctx, childID, outcomeID, "child-of"); err != nil {
				return fmt.Errorf("failed to remove %s from outcome: %v", childID, err)
			}
			if outputFormat != "json" {
				fmt.Printf("Removed %s from outcome %s\n", childID, outcomeID)
			}
		}

		if outputFormat == "json" {
			out := map[string]any{
				"outcome": outcomeID,
				"removed": childIDs,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
		}
		return nil
	},
}

func init() {
	outcomeCreateCmd.Flags().String("body", "", "Outcome description (inline)")
	outcomeCreateCmd.Flags().String("body-file", "", "Read description from file")
	outcomeCreateCmd.Flags().StringSlice("label", nil, "Labels (repeatable)")
	outcomeCreateCmd.Flags().String("adopt", "", "Comma-separated shard IDs to adopt as children")

	outcomeListCmd.Flags().Bool("closed", false, "Include closed outcomes")

	outcomeCloseCmd.Flags().String("reason", "", "Reason for closing")

	outcomeCmd.AddCommand(outcomeCreateCmd)
	outcomeCmd.AddCommand(outcomeListCmd)
	outcomeCmd.AddCommand(outcomeShowCmd)
	outcomeCmd.AddCommand(outcomeCloseCmd)
	outcomeCmd.AddCommand(outcomeAddCmd)
	outcomeCmd.AddCommand(outcomeRemoveCmd)
	rootCmd.AddCommand(outcomeCmd)
}
