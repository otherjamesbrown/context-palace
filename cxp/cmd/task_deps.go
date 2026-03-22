package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var taskDepsCmd = &cobra.Command{
	Use:   "deps <design-id>",
	Short: "Show task dependency graph for a design",
	Long:  `Reads blocked-by edges between child tasks and outputs a wave-sorted dispatch plan.`,
	Args:  cobra.ExactArgs(1),
	Example: `  cxp task deps pf-design-123
  cxp task deps pf-design-123 -o json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		designID := args[0]

		result, err := cpClient.GetTaskDeps(ctx, designID)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		if len(result.Waves) == 0 {
			fmt.Printf("No tasks found for %s\n", designID)
			return nil
		}

		for _, wave := range result.Waves {
			fmt.Printf("Wave %d:\n", wave.Wave)
			for _, task := range wave.Tasks {
				status := task.Status
				marker := "○"
				if status == "closed" {
					marker = "●"
				} else if status == "in_progress" {
					marker = "◐"
				} else if status == "needs-review" {
					marker = "◑"
				}
				line := fmt.Sprintf("  %s %-12s %s", marker, task.ID, task.Title)
				if len(task.BlockedBy) > 0 {
					line += fmt.Sprintf("  [blocked by: %s]", strings.Join(task.BlockedBy, ", "))
				}
				fmt.Println(line)
			}
		}

		if len(result.Dispatchable) > 0 {
			fmt.Printf("\nDispatchable: %s\n", strings.Join(result.Dispatchable, ", "))
		} else {
			fmt.Println("\nNo tasks ready to dispatch.")
		}

		return nil
	},
}

func init() {
	taskCmd.AddCommand(taskDepsCmd)
}
