package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var taskPRMergeCmd = &cobra.Command{
	Use:     "merge <task-id>",
	Short:   "Merge the PR for a task, clean up worktree, and close",
	Args:    cobra.ExactArgs(1),
	Example: "  cxp task pr merge pf-abc123\n  cxp task pr merge pf-abc123 --merge\n  cxp task pr merge pf-abc123 --dry-run",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		taskID := args[0]
		useMerge, _ := cmd.Flags().GetBool("merge")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		// 1. Read task shard, get pr_url from metadata
		prURL, err := cpClient.GetMetadataField(ctx, taskID, []string{"pr_url"})
		if err != nil || prURL == "" {
			return fmt.Errorf("no PR found for this task, run `cxp task pr create` first")
		}

		mergeStrategy := "--squash"
		if useMerge {
			mergeStrategy = "--merge"
		}

		if dryRun {
			fmt.Printf("Would merge PR:        gh pr merge %s %s --delete-branch\n", prURL, mergeStrategy)
			fmt.Printf("Would get commit:      gh pr view %s --json mergeCommit -q .mergeCommit.oid\n", prURL)
			fmt.Printf("Would store metadata:  merge_commit -> <hash>\n")
			fmt.Printf("Would remove worktree: cxp task worktree remove %s\n", taskID)
			fmt.Printf("Would close task:      cxp shard status %s closed\n", taskID)
			return nil
		}

		// 2. Merge the PR
		ghMergeArgs := []string{"pr", "merge", prURL, mergeStrategy, "--delete-branch"}
		mergeOut, err := exec.CommandContext(ctx, "gh", ghMergeArgs...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to merge PR: %w\n%s", err, string(mergeOut))
		}
		fmt.Printf("Merged PR: %s\n", prURL)

		// 3. Get merge commit hash
		viewOut, err := exec.CommandContext(ctx, "gh", "pr", "view", prURL,
			"--json", "mergeCommit", "-q", ".mergeCommit.oid").CombinedOutput()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: PR merged but could not get merge commit: %v\n%s\n", err, string(viewOut))
		} else {
			mergeCommit := strings.TrimSpace(string(viewOut))
			if mergeCommit != "" {
				// 4. Store merge commit in task metadata
				patch, _ := json.Marshal(map[string]string{"merge_commit": mergeCommit})
				if _, err := cpClient.UpdateMetadata(ctx, taskID, patch); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not store merge_commit in metadata: %v\n", err)
				} else {
					fmt.Printf("Merge commit: %s\n", mergeCommit)
				}
			}
		}

		// 5. Clean up worktree
		wtOut, err := exec.CommandContext(ctx, "cxp", "task", "worktree", "remove", taskID).CombinedOutput()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not remove worktree: %v\n%s\n", err, string(wtOut))
		} else {
			fmt.Printf("Removed worktree for %s\n", taskID)
		}

		// 6. Close task
		closeOut, err := exec.CommandContext(ctx, "cxp", "shard", "status", taskID, "closed").CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to close task: %w\n%s", err, string(closeOut))
		}
		fmt.Printf("Closed task %s\n", taskID)

		return nil
	},
}

func init() {
	taskPRMergeCmd.Flags().Bool("merge", false, "Use merge commit instead of squash (default: squash)")
	taskPRMergeCmd.Flags().Bool("dry-run", false, "Show what would happen without executing")
	taskPRCmd.AddCommand(taskPRMergeCmd)
}
