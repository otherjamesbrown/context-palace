package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var taskReviewCmd = &cobra.Command{
	Use:   "review <task-id>",
	Short: "Spawn a PR review session for a task",
	Long:  `Reads the task shard, fetches the PR diff, and spawns a Claude session in tmux to review the PR against the task spec and design context.`,
	Args:  cobra.ExactArgs(1),
	Example: `  cxp task review pf-abc123
  cxp task review pf-abc123 --dry-run
  cxp task review pf-abc123 --agent mycroft`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		taskID := args[0]
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		agentOverride, _ := cmd.Flags().GetString("agent")

		agent := cpClient.Config.Agent
		if agentOverride != "" {
			agent = agentOverride
		}

		// 1. Read the task shard
		task, err := cpClient.GetTask(ctx, taskID)
		if err != nil {
			return fmt.Errorf("task not found: %s", taskID)
		}

		// 2. Get PR URL from task metadata
		prURL, err := cpClient.GetMetadataField(ctx, taskID, []string{"pr_url"})
		if err != nil || prURL == "" {
			return fmt.Errorf("no PR found for task %s (use 'cxp task pr create' first)", taskID)
		}

		// 3. Get PR diff
		var diff string
		if dryRun {
			diff = "[dry-run: would fetch diff from " + prURL + "]"
		} else {
			ghCmd := exec.CommandContext(ctx, "gh", "pr", "diff", prURL)
			diffOut, err := ghCmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to get PR diff: %w\n%s", err, string(diffOut))
			}
			diff = string(diffOut)
		}

		// 4. Get parent design shard (follow child-of edge)
		var designTitle, designContent string
		parentEdges, err := cpClient.GetShardEdges(ctx, taskID, "outgoing", []string{"child-of"})
		if err == nil && len(parentEdges) > 0 {
			parentID := parentEdges[0].ShardID
			parent, err := cpClient.GetShard(ctx, parentID)
			if err == nil {
				designTitle = parent.Title
				designContent = parent.Content
			}
		}

		// 5. Get worktree path for the repo root
		worktreePath, _ := cpClient.GetMetadataField(ctx, taskID, []string{"worktree_path"})
		if worktreePath == "" {
			worktreePath = "."
		}

		// 6. Build reviewer prompt
		var pb strings.Builder
		pb.WriteString(fmt.Sprintf("You are reviewing PR for task %s: %s\n\n", taskID, task.Title))
		pb.WriteString("## Task Spec\n")
		pb.WriteString(task.Content)
		pb.WriteString("\n\n")

		if designTitle != "" || designContent != "" {
			pb.WriteString("## Design Context\n")
			if designTitle != "" {
				pb.WriteString(fmt.Sprintf("**%s**\n\n", designTitle))
			}
			if designContent != "" {
				pb.WriteString(designContent)
			}
			pb.WriteString("\n\n")
		}

		pb.WriteString("## PR Diff\n")
		pb.WriteString(diff)
		pb.WriteString("\n\n")

		pb.WriteString("## Review Instructions\n")
		pb.WriteString("Answer three questions:\n")
		pb.WriteString("1. Does it match the task spec? Acceptance criteria met?\n")
		pb.WriteString("2. Does it fit the overall design?\n")
		pb.WriteString("3. Will it break anything?\n\n")
		pb.WriteString(fmt.Sprintf("Write your verdict using: cxp task review-verdict %s approve|request-changes|escalate --body \"<explanation>\"\n", taskID))

		prompt := pb.String()

		// 7. Build tmux command
		escapedPrompt := strings.ReplaceAll(prompt, "'", "'\\''")
		windowName := fmt.Sprintf("review-%s", taskID)
		tmuxCmd := fmt.Sprintf("cd %s && claude --print '%s'", worktreePath, escapedPrompt)
		tmuxArgs := []string{"new-window", "-n", windowName, "-t", "main", tmuxCmd}

		// Dry-run: print everything and exit
		if dryRun {
			fmt.Printf("=== Task ===\n")
			fmt.Printf("ID:     %s\n", task.ID)
			fmt.Printf("Title:  %s\n", task.Title)
			fmt.Printf("PR:     %s\n", prURL)
			fmt.Printf("Agent:  %s\n\n", agent)

			if designTitle != "" {
				fmt.Printf("=== Design Context ===\n")
				fmt.Printf("Parent: %s\n\n", designTitle)
			}

			fmt.Printf("=== Prompt ===\n")
			fmt.Printf("%s\n\n", prompt)

			fmt.Printf("=== tmux Command ===\n")
			fmt.Printf("tmux %s\n", strings.Join(tmuxArgs, " "))
			return nil
		}

		// 8. Spawn tmux window
		spawnCmd := exec.CommandContext(ctx, "tmux", tmuxArgs...)
		tmuxOut, err := spawnCmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(tmuxOut), "no server running") || strings.Contains(string(tmuxOut), "no current client") {
				return fmt.Errorf("no tmux session found. Start one with: tmux new-session -s main")
			}
			return fmt.Errorf("failed to spawn tmux window: %w\n%s", err, string(tmuxOut))
		}

		fmt.Printf("Review dispatched for %s\n", taskID)
		fmt.Printf("  PR:     %s\n", prURL)
		fmt.Printf("  Agent:  %s\n", agent)
		fmt.Printf("  Window: %s\n", windowName)
		return nil
	},
}

var taskReviewVerdictCmd = &cobra.Command{
	Use:   "review-verdict <task-id> <verdict>",
	Short: "Record a review verdict on a task",
	Long: `Record a review verdict for a task PR.

Valid verdicts:
  approve          Mark PR as approved
  request-changes  Request changes from implementer
  escalate         Escalate for human review`,
	Args:    cobra.ExactArgs(2),
	Example: `  cxp task review-verdict pf-abc123 approve --body "LGTM, all criteria met"
  cxp task review-verdict pf-abc123 request-changes --body "Missing error handling in auth.go"
  cxp task review-verdict pf-abc123 escalate --body "Security implications need human review"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		taskID := args[0]
		verdict := args[1]
		body, _ := cmd.Flags().GetString("body")

		// Validate verdict
		validVerdicts := map[string]bool{
			"approve":         true,
			"request-changes": true,
			"escalate":        true,
		}
		if !validVerdicts[verdict] {
			return fmt.Errorf("invalid verdict %q: must be approve, request-changes, or escalate", verdict)
		}

		// Build the verdict entry
		agent := cpClient.Config.Agent
		if agent == "" {
			agent = "unknown"
		}
		timestamp := time.Now().UTC().Format("2006-01-02 15:04:05 UTC")

		var entry strings.Builder
		entry.WriteString(fmt.Sprintf("\n\n---\n**Review Verdict:** %s\n", verdict))
		entry.WriteString(fmt.Sprintf("**Reviewer:** %s\n", agent))
		entry.WriteString(fmt.Sprintf("**Date:** %s\n", timestamp))
		if body != "" {
			entry.WriteString(fmt.Sprintf("\n%s\n", body))
		}

		// Append verdict to task shard
		if err := cpClient.AppendShardContent(ctx, taskID, entry.String()); err != nil {
			return fmt.Errorf("failed to append verdict to task %s: %w", taskID, err)
		}

		switch verdict {
		case "approve":
			// Add approved label
			if _, err := cpClient.AddShardLabels(ctx, taskID, []string{"approved"}); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: verdict recorded but failed to add label: %v\n", err)
			}
			fmt.Printf("Approved. Run `cxp task pr merge %s` to merge.\n", taskID)

		case "request-changes":
			// Set task status back to in_progress
			if _, err := cpClient.TransitionShardStatus(ctx, taskID, "in_progress"); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: verdict recorded but failed to transition status: %v\n", err)
			}
			fmt.Println("Changes requested. Implementer will be notified.")

		case "escalate":
			// Add blocked label
			if _, err := cpClient.AddShardLabels(ctx, taskID, []string{"blocked"}); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: verdict recorded but failed to add label: %v\n", err)
			}
			fmt.Println("Escalated. James will be notified.")
		}

		return nil
	},
}

func init() {
	taskReviewCmd.Flags().Bool("dry-run", false, "Show what would be done without executing")
	taskReviewCmd.Flags().String("agent", "", "Override reviewer agent")

	taskReviewVerdictCmd.Flags().String("body", "", "Explanation/details for the verdict")

	taskCmd.AddCommand(taskReviewCmd)
	taskCmd.AddCommand(taskReviewVerdictCmd)
}
