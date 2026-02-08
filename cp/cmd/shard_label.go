package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

// -- shard label --

var shardLabelCmd = &cobra.Command{
	Use:   "label",
	Short: "Label management",
	Long:  `Add, remove, and list labels on shards.`,
}

// -- shard label add --

var shardLabelAddCmd = &cobra.Command{
	Use:     "add <shard-id> <label> [label...]",
	Short:   "Add labels to a shard",
	Args:    cobra.MinimumNArgs(2),
	Example: "  cp shard label add pf-abc123 architecture pipeline",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]
		labels := args[1:]

		result, err := cpClient.AddShardLabels(ctx, id, labels)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":     id,
				"labels": result,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Labels for %s: %s\n", id, strings.Join(result, ", "))
		return nil
	},
}

// -- shard label remove --

var shardLabelRemoveCmd = &cobra.Command{
	Use:     "remove <shard-id> <label> [label...]",
	Short:   "Remove labels from a shard",
	Args:    cobra.MinimumNArgs(2),
	Example: "  cp shard label remove pf-abc123 pipeline",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]
		labels := args[1:]

		result, err := cpClient.RemoveShardLabels(ctx, id, labels)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":     id,
				"labels": result,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		if len(result) == 0 {
			fmt.Printf("Labels for %s: (none)\n", id)
		} else {
			fmt.Printf("Labels for %s: %s\n", id, strings.Join(result, ", "))
		}
		return nil
	},
}

// -- shard label list --

var shardLabelListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all labels in use with counts",
	Example: "  cp shard label list",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		labels, err := cpClient.LabelSummary(ctx)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(labels)
			fmt.Println(s)
			return nil
		}

		if len(labels) == 0 {
			fmt.Println("No labels in use.")
			return nil
		}

		tbl := client.NewTable("LABEL", "COUNT")
		for _, l := range labels {
			tbl.AddRow(l.Label, fmt.Sprintf("%d", l.Count))
		}
		fmt.Print(tbl.String())
		return nil
	},
}

func init() {
	shardLabelCmd.AddCommand(shardLabelAddCmd)
	shardLabelCmd.AddCommand(shardLabelRemoveCmd)
	shardLabelCmd.AddCommand(shardLabelListCmd)

	shardCmd.AddCommand(shardLabelCmd)
}
