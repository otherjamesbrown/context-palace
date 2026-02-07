package cmd

import (
	"context"
	"fmt"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Task management",
	Long:  `Commands for reading, claiming, updating, and closing tasks.`,
}

var taskGetCmd = &cobra.Command{
	Use:     "get <shard-id>",
	Short:   "Get task details",
	Args:    cobra.ExactArgs(1),
	Example: "  cp task get pf-123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		task, err := cpClient.GetTask(ctx, args[0])
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(task)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("ID:       %s\n", task.ID)
		fmt.Printf("Title:    %s\n", task.Title)
		fmt.Printf("Status:   %s\n", task.Status)
		if task.Owner != nil {
			fmt.Printf("Owner:    %s\n", *task.Owner)
		} else {
			fmt.Printf("Owner:    (unassigned)\n")
		}
		if task.Priority != nil {
			fmt.Printf("Priority: %d\n", *task.Priority)
		}
		fmt.Printf("Created:  %s\n", task.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("\n--- Content ---\n%s\n", task.Content)

		if len(task.Artifacts) > 0 {
			fmt.Printf("\n--- Artifacts ---\n")
			for _, a := range task.Artifacts {
				fmt.Printf("  [%s] %s: %s\n", a.Type, a.Reference, a.Description)
			}
		}

		return nil
	},
}

var taskClaimCmd = &cobra.Command{
	Use:     "claim <shard-id>",
	Short:   "Claim a task",
	Args:    cobra.ExactArgs(1),
	Example: "  cp task claim pf-123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		success, err := cpClient.ClaimTask(ctx, args[0])
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"success": %t, "task_id": "%s", "agent": "%s"}`+"\n",
				success, args[0], cpClient.Config.Agent)
			return nil
		}

		if success {
			fmt.Printf("Claimed task %s for %s\n", args[0], cpClient.Config.Agent)
		} else {
			fmt.Printf("Task %s is already claimed by another agent\n", args[0])
		}
		return nil
	},
}

var taskProgressCmd = &cobra.Command{
	Use:     "progress <shard-id> <note>",
	Short:   "Log progress on a task",
	Args:    cobra.ExactArgs(2),
	Example: `  cp task progress pf-123 "Found bug in oauth.go line 45"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		err := cpClient.AddProgress(ctx, args[0], args[1])
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"success": true, "task_id": "%s"}`+"\n", args[0])
			return nil
		}

		fmt.Printf("Added progress note to %s\n", args[0])
		return nil
	},
}

var taskCloseCmd = &cobra.Command{
	Use:     "close <shard-id> <summary>",
	Short:   "Close a task",
	Args:    cobra.ExactArgs(2),
	Example: `  cp task close pf-123 "Fixed OAuth token refresh"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		err := cpClient.CloseTask(ctx, args[0], args[1])
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"success": true, "task_id": "%s", "status": "closed"}`+"\n", args[0])
			return nil
		}

		fmt.Printf("Closed task %s\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskGetCmd)
	taskCmd.AddCommand(taskClaimCmd)
	taskCmd.AddCommand(taskProgressCmd)
	taskCmd.AddCommand(taskCloseCmd)
}
