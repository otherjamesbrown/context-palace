package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

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
	Example: "  cxp task get pf-123",
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
	Example: "  cxp task claim pf-123",
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
	Example: `  cxp task progress pf-123 "Found bug in oauth.go line 45"`,
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
	Example: `  cxp task close pf-123 "Fixed OAuth token refresh"`,
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

var taskCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a task",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp task create "Fix auth bug" --body "Token refresh broken"
  cxp task create "Implement caching" --parent pf-xxx --assign mycroft
  cxp task create "Write tests" --blocked-by pf-yyy`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		title := args[0]

		body, _ := cmd.Flags().GetString("body")
		bodyFile, _ := cmd.Flags().GetString("body-file")
		labels, _ := cmd.Flags().GetStringSlice("label")
		parentID, _ := cmd.Flags().GetString("parent")
		assign, _ := cmd.Flags().GetString("assign")
		blockedBy, _ := cmd.Flags().GetString("blocked-by")

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

		// Add assignment label
		if assign != "" {
			labels = append(labels, fmt.Sprintf("to:agent-%s", assign))
		}

		id, err := cpClient.CreateShardWithMetadata(ctx, title, content, "task", nil, labels, nil)
		if err != nil {
			return err
		}

		// Create child-of edge to parent
		if parentID != "" {
			if err := cpClient.CreateEdgeSimple(ctx, id, parentID, "child-of"); err != nil {
				return fmt.Errorf("created task %s but failed to set parent: %v", id, err)
			}
		}

		// Create blocked-by edge
		if blockedBy != "" {
			if err := cpClient.CreateEdgeSimple(ctx, id, blockedBy, "blocked-by"); err != nil {
				return fmt.Errorf("created task %s but failed to set blocked-by: %v", id, err)
			}
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":         id,
				"type":       "task",
				"title":      title,
				"created_at": time.Now().UTC().Format(time.RFC3339),
			}
			if parentID != "" {
				out["parent"] = parentID
			}
			if assign != "" {
				out["assigned_to"] = assign
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Created task %s: %q\n", id, title)
		if parentID != "" {
			fmt.Printf("  Parent: %s\n", parentID)
		}
		if assign != "" {
			fmt.Printf("  Assigned to: agent-%s\n", assign)
		}
		if blockedBy != "" {
			fmt.Printf("  Blocked by: %s\n", blockedBy)
		}
		return nil
	},
}

func init() {
	taskCreateCmd.Flags().String("body", "", "Task content (inline)")
	taskCreateCmd.Flags().String("body-file", "", "Read content from file")
	taskCreateCmd.Flags().StringSlice("label", nil, "Labels (repeatable)")
	taskCreateCmd.Flags().String("parent", "", "Parent shard ID (creates child-of edge)")
	taskCreateCmd.Flags().String("assign", "", "Agent to assign (adds to:agent-{value} label)")
	taskCreateCmd.Flags().String("blocked-by", "", "Shard ID this task is blocked by")

	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskGetCmd, taskClaimCmd, taskProgressCmd, taskCloseCmd, taskCreateCmd)
}
