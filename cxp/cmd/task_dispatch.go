package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var taskDispatchCmd = &cobra.Command{
	Use:   "dispatch <task-id>",
	Short: "Dispatch a task to an agent via tmux",
	Long:  `Spawns a Claude Code session in a tmux window with full context from the task and its parent design shard.`,
	Args:  cobra.ExactArgs(1),
	Example: `  cxp task dispatch pf-abc123
  cxp task dispatch pf-abc123 --agent mycroft
  cxp task dispatch pf-abc123 --dry-run`,
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

		// 2. Verify task is dispatchable
		if task.Status == "in_progress" {
			return fmt.Errorf("task already dispatched (status: in_progress)")
		}
		if task.Status != "open" && task.Status != "ready" {
			return fmt.Errorf("task not dispatchable (status: %s, must be open or ready)", task.Status)
		}

		// Check blocked-by edges: all blockers must be closed
		blockerEdges, err := cpClient.GetShardEdges(ctx, taskID, "outgoing", []string{"blocked-by"})
		if err != nil {
			return fmt.Errorf("failed to check blockers: %w", err)
		}
		var unsatisfied []string
		for _, e := range blockerEdges {
			if e.Status != "closed" {
				unsatisfied = append(unsatisfied, fmt.Sprintf("%s (%s)", e.ShardID, e.Status))
			}
		}
		if len(unsatisfied) > 0 {
			return fmt.Errorf("blockers not satisfied:\n  %s", strings.Join(unsatisfied, "\n  "))
		}

		// 3. Get or create worktree
		worktreePath, _ := cpClient.GetMetadataField(ctx, taskID, []string{"worktree_path"})
		if worktreePath == "" {
			if dryRun {
				fmt.Println("[dry-run] Would create worktree via: cxp task worktree create " + taskID)
				worktreePath = fmt.Sprintf("~/worktrees/<project>/%s", taskID)
			} else {
				out, err := exec.CommandContext(ctx, "cxp", "task", "worktree", "create", taskID).CombinedOutput()
				if err != nil {
					return fmt.Errorf("failed to create worktree: %s\n%s", err, string(out))
				}
				// Re-read the worktree path from metadata
				worktreePath, err = cpClient.GetMetadataField(ctx, taskID, []string{"worktree_path"})
				if err != nil || worktreePath == "" {
					return fmt.Errorf("worktree created but could not read worktree_path from metadata")
				}
			}
		}

		// 4. Get parent design shard (follow child-of edge)
		var designContext string
		parentEdges, err := cpClient.GetShardEdges(ctx, taskID, "outgoing", []string{"child-of"})
		if err == nil && len(parentEdges) > 0 {
			parentID := parentEdges[0].ShardID
			parent, err := cpClient.GetShard(ctx, parentID)
			if err == nil {
				designContext = fmt.Sprintf("## Design Context (from %s)\n\n**%s**\n\n%s",
					parent.ID, parent.Title, parent.Content)
			}
		}

		// 5. Build the prompt
		var promptBuilder strings.Builder
		promptBuilder.WriteString(fmt.Sprintf("# Task: %s\n\n", task.Title))
		promptBuilder.WriteString(fmt.Sprintf("**Task ID:** %s\n", task.ID))
		promptBuilder.WriteString(fmt.Sprintf("**Agent:** %s\n\n", agent))
		promptBuilder.WriteString("## Task Content\n\n")
		promptBuilder.WriteString(task.Content)
		promptBuilder.WriteString("\n\n")
		if designContext != "" {
			promptBuilder.WriteString(designContext)
			promptBuilder.WriteString("\n\n")
		}
		// Load pipeline config — try repo root from registry, fall back to worktree path
		repoRoot, _ := client.RepoForProject(cpClient.Config.Project)
		pCfg, _ := client.LoadPipelineConfig(repoRoot)
		if pCfg == nil {
			pCfg, _ = client.LoadPipelineConfig(worktreePath)
		}
		if pCfg == nil {
			pCfg = client.DefaultPipelineConfig()
		}

		promptBuilder.WriteString("## Instructions\n\n")
		promptBuilder.WriteString("Implement this task following the acceptance criteria above.\n\n")
		promptBuilder.WriteString("### On completion\n\n")

		step := 1
		if len(pCfg.Test) > 0 {
			promptBuilder.WriteString(fmt.Sprintf("%d. Run tests: `%s`\n", step, strings.Join(pCfg.Test, " && ")))
			step++
		}
		if len(pCfg.Build) > 0 {
			promptBuilder.WriteString(fmt.Sprintf("%d. Build: `%s`\n", step, strings.Join(pCfg.Build, " && ")))
			step++
		}
		promptBuilder.WriteString(fmt.Sprintf("%d. Append evidence:\n", step))
		promptBuilder.WriteString("   ```\n")
		promptBuilder.WriteString("   cxp task evidence " + taskID + " --files \"<files changed>\" --commit <hash> --body \"<verification notes>\"\n")
		promptBuilder.WriteString("   ```\n")
		step++
		promptBuilder.WriteString(fmt.Sprintf("%d. Create PR: `cxp task pr create %s`\n", step, taskID))
		step++
		promptBuilder.WriteString(fmt.Sprintf("%d. Mark for review: `cxp shard status %s needs-review`\n", step, taskID))

		prompt := promptBuilder.String()

		// 6. Build tmux command
		// Escape single quotes in prompt for shell
		escapedPrompt := strings.ReplaceAll(prompt, "'", "'\\''")
		// Read tmux session from pipeline config
		tmuxSession := "main"
		if pCfg != nil && pCfg.Dispatch.TmuxSession != "" {
			tmuxSession = pCfg.Dispatch.TmuxSession
		}

		tmuxCmd := fmt.Sprintf("cd '%s' && claude --print '%s'", strings.ReplaceAll(worktreePath, "'", "'\\''"), escapedPrompt)
		tmuxArgs := []string{"new-window", "-n", taskID, "-t", tmuxSession, tmuxCmd}

		// Dry-run: print everything and exit
		if dryRun {
			fmt.Printf("=== Task ===\n")
			fmt.Printf("ID:     %s\n", task.ID)
			fmt.Printf("Title:  %s\n", task.Title)
			fmt.Printf("Status: %s\n", task.Status)
			fmt.Printf("Agent:  %s\n\n", agent)

			fmt.Printf("=== Worktree ===\n")
			fmt.Printf("Path: %s\n\n", worktreePath)

			if designContext != "" {
				fmt.Printf("=== Design Context ===\n")
				fmt.Printf("%s\n\n", designContext)
			}

			fmt.Printf("=== Prompt ===\n")
			fmt.Printf("%s\n\n", prompt)

			fmt.Printf("=== tmux Command ===\n")
			fmt.Printf("tmux %s\n", strings.Join(tmuxArgs, " "))
			return nil
		}

		// 7. Set task status to in_progress
		statusOut, err := exec.CommandContext(ctx, "cxp", "shard", "status", taskID, "in_progress").CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to set status to in_progress: %s\n%s", err, string(statusOut))
		}

		// 8. Spawn tmux window
		tmuxOut, err := exec.CommandContext(ctx, "tmux", tmuxArgs...).CombinedOutput()
		if err != nil {
			// Revert status on tmux failure
			_ = exec.CommandContext(ctx, "cxp", "shard", "status", taskID, task.Status).Run()
			if strings.Contains(string(tmuxOut), "no server running") || strings.Contains(string(tmuxOut), "no current client") {
				return fmt.Errorf("no tmux session found. Start one with: tmux new-session -s main")
			}
			return fmt.Errorf("failed to spawn tmux window: %s\n%s", err, string(tmuxOut))
		}

		// 9. Record dispatch metadata
		dispatchInfo := map[string]string{
			"dispatched_at": time.Now().UTC().Format(time.RFC3339),
			"agent":         agent,
			"tmux_window":   taskID,
		}
		patch, _ := json.Marshal(dispatchInfo)
		if _, err := cpClient.UpdateMetadata(ctx, taskID, patch); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: dispatched but failed to update metadata: %v\n", err)
		}

		if outputFormat == "json" {
			out := map[string]any{
				"task_id":       taskID,
				"agent":         agent,
				"worktree_path": worktreePath,
				"tmux_window":   taskID,
				"dispatched_at": dispatchInfo["dispatched_at"],
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Dispatched %s to %s\n", taskID, agent)
		fmt.Printf("  Worktree: %s\n", worktreePath)
		fmt.Printf("  Window:   %s\n", taskID)
		return nil
	},
}

func init() {
	taskDispatchCmd.Flags().String("agent", "", "Override agent (default: from config)")
	taskDispatchCmd.Flags().Bool("dry-run", false, "Show what would be done without executing")
	taskCmd.AddCommand(taskDispatchCmd)
}
