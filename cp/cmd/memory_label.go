package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var memoryLabelCmd = &cobra.Command{
	Use:   "label",
	Short: "Label management for memories",
	Long:  `Add and remove labels on memory shards.`,
}

var memoryLabelAddCmd = &cobra.Command{
	Use:     "add <memory-id> <label> [label...]",
	Short:   "Add labels to a memory",
	Args:    cobra.MinimumNArgs(2),
	Example: "  cxp memory label add pf-abc123 standing-instruction pipeline",
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

var memoryLabelRemoveCmd = &cobra.Command{
	Use:     "remove <memory-id> <label> [label...]",
	Short:   "Remove labels from a memory",
	Args:    cobra.MinimumNArgs(2),
	Example: "  cxp memory label remove pf-abc123 pipeline",
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

func init() {
	memoryLabelCmd.AddCommand(memoryLabelAddCmd)
	memoryLabelCmd.AddCommand(memoryLabelRemoveCmd)

	memoryCmd.AddCommand(memoryLabelCmd)
}
