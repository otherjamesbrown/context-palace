package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var taskWorktreeCmd = &cobra.Command{
	Use:   "worktree",
	Short: "Manage git worktrees for tasks",
	Long:  `Create, show, and remove git worktrees linked to task shards.`,
}

var taskWorktreeCreateCmd = &cobra.Command{
	Use:     "create <task-id>",
	Short:   "Create a git worktree for a task",
	Args:    cobra.ExactArgs(1),
	Example: "  cxp task worktree create pf-abc123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		taskID := args[0]

		// Get task to read its title
		task, err := cpClient.GetTask(ctx, taskID)
		if err != nil {
			return fmt.Errorf("cannot get task %s: %w", taskID, err)
		}

		// Find the git repo root from cwd
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot get working directory: %w", err)
		}
		repoDir, err := client.GitRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("must be run from within a git repository: %w", err)
		}

		project := cpClient.Config.Project

		// Create worktree
		info, err := client.CreateWorktree(repoDir, project, taskID, task.Title)
		if err != nil {
			return err
		}

		// Store worktree_path in task metadata
		patch, _ := json.Marshal(map[string]string{
			"worktree_path": info.WorktreePath,
			"worktree_branch": info.Branch,
		})
		if _, err := cpClient.UpdateMetadata(ctx, taskID, patch); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: worktree created but failed to update metadata: %v\n", err)
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(info)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Created worktree for %s\n", taskID)
		fmt.Printf("  Branch: %s\n", info.Branch)
		fmt.Printf("  Path:   %s\n", info.WorktreePath)
		return nil
	},
}

var taskWorktreeShowCmd = &cobra.Command{
	Use:     "show <task-id>",
	Short:   "Show worktree info for a task",
	Args:    cobra.ExactArgs(1),
	Example: "  cxp task worktree show pf-abc123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		taskID := args[0]

		wtPath, err := cpClient.GetMetadataField(ctx, taskID, []string{"worktree_path"})
		if err != nil {
			return fmt.Errorf("no worktree found for %s: %w", taskID, err)
		}

		branch, _ := cpClient.GetMetadataField(ctx, taskID, []string{"worktree_branch"})

		info := &client.WorktreeInfo{
			TaskID:       taskID,
			Branch:       branch,
			WorktreePath: wtPath,
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(info)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Task:   %s\n", taskID)
		fmt.Printf("Branch: %s\n", info.Branch)
		fmt.Printf("Path:   %s\n", info.WorktreePath)
		return nil
	},
}

var taskWorktreeRemoveCmd = &cobra.Command{
	Use:     "remove <task-id>",
	Short:   "Remove worktree and branch for a task",
	Args:    cobra.ExactArgs(1),
	Example: "  cxp task worktree remove pf-abc123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		taskID := args[0]

		// Get worktree info from metadata
		wtPath, err := cpClient.GetMetadataField(ctx, taskID, []string{"worktree_path"})
		if err != nil {
			return fmt.Errorf("no worktree found for %s: %w", taskID, err)
		}
		branch, _ := cpClient.GetMetadataField(ctx, taskID, []string{"worktree_branch"})

		// Find the git repo root from cwd
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot get working directory: %w", err)
		}
		repoDir, err := client.GitRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("must be run from within a git repository: %w", err)
		}

		// Remove worktree
		if err := client.RemoveWorktree(repoDir, cpClient.Config.Project, taskID); err != nil {
			// If the worktree dir doesn't exist, that's fine — it may have been manually removed
			if _, statErr := os.Stat(wtPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "Note: worktree directory already removed\n")
			} else {
				return err
			}
		}

		// Remove branch
		if branch != "" {
			if err := client.RemoveWorktreeBranch(repoDir, branch); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to delete branch %s: %v\n", branch, err)
			}
		}

		// Clear metadata
		if _, err := cpClient.DeleteMetadataKey(ctx, taskID, "worktree_path"); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to clear worktree_path metadata: %v\n", err)
		}
		if _, err := cpClient.DeleteMetadataKey(ctx, taskID, "worktree_branch"); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to clear worktree_branch metadata: %v\n", err)
		}

		if outputFormat == "json" {
			fmt.Printf(`{"success": true, "task_id": "%s"}`+"\n", taskID)
			return nil
		}

		fmt.Printf("Removed worktree for %s\n", taskID)
		return nil
	},
}

func init() {
	taskCmd.AddCommand(taskWorktreeCmd)
	taskWorktreeCmd.AddCommand(taskWorktreeCreateCmd, taskWorktreeShowCmd, taskWorktreeRemoveCmd)
}
