package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var taskPRCmd = &cobra.Command{
	Use:   "pr",
	Short: "Pull request operations for tasks",
	Long:  `Create and manage GitHub pull requests linked to task shards.`,
}

var taskPRCreateCmd = &cobra.Command{
	Use:     "create <task-id>",
	Short:   "Create a GitHub PR from a task's worktree branch",
	Args:    cobra.ExactArgs(1),
	Example: "  cxp task pr create pf-abc123\n  cxp task pr create pf-abc123 --draft\n  cxp task pr create pf-abc123 --dry-run",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		taskID := args[0]
		draft, _ := cmd.Flags().GetBool("draft")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		// 1. Read the task shard
		task, err := cpClient.GetTask(ctx, taskID)
		if err != nil {
			return fmt.Errorf("cannot get task %s: %w", taskID, err)
		}

		// 2. Get worktree path from metadata
		wtPath, err := cpClient.GetMetadataField(ctx, taskID, []string{"worktree_path"})
		if err != nil {
			return fmt.Errorf("no worktree found for %s (use 'cxp task worktree create' first): %w", taskID, err)
		}

		// 3. Get the branch name
		branch, _ := cpClient.GetMetadataField(ctx, taskID, []string{"worktree_branch"})
		if branch == "" {
			out, err := exec.Command("git", "-C", wtPath, "branch", "--show-current").Output()
			if err != nil {
				return fmt.Errorf("cannot determine branch for worktree at %s: %w", wtPath, err)
			}
			branch = strings.TrimSpace(string(out))
		}
		if branch == "" {
			return fmt.Errorf("cannot determine branch for task %s", taskID)
		}

		// Detect repo from git remote
		repo, err := client.DetectOwnerRepo(wtPath)
		if err != nil {
			return fmt.Errorf("cannot detect github repo for worktree %s: %w", wtPath, err)
		}

		// 4. Build PR body
		body := buildPRBody(taskID, task.Title, task.Content)

		// 5. Build gh command args
		ghArgs := []string{"pr", "create",
			"--repo", repo,
			"--head", branch,
			"--base", "main",
			"--title", task.Title,
			"--body", body,
		}
		if draft {
			ghArgs = append(ghArgs, "--draft")
		}

		if dryRun {
			fmt.Printf("Would push branch: git -C %s push -u origin %s\n", wtPath, branch)
			fmt.Printf("Would create PR:   gh %s\n", strings.Join(ghArgs, " "))
			return nil
		}

		// 6. Push the branch
		pushCmd := exec.Command("git", "-C", wtPath, "push", "-u", "origin", branch)
		pushOut, err := pushCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to push branch %s: %w\n%s", branch, err, string(pushOut))
		}

		// 7. Create PR via gh CLI
		ghCmd := exec.Command("gh", ghArgs...)
		ghCmd.Dir = wtPath
		prOut, err := ghCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to create PR: %w\n%s", err, string(prOut))
		}
		prURL := strings.TrimSpace(string(prOut))

		// 8. Store PR URL in task metadata
		patch, _ := json.Marshal(map[string]string{"pr_url": prURL})
		if _, err := cpClient.UpdateMetadata(ctx, taskID, patch); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: PR created but failed to store URL in metadata: %v\n", err)
		}

		// 9. Output
		if outputFormat == "json" {
			out := map[string]string{
				"task_id": taskID,
				"branch":  branch,
				"pr_url":  prURL,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Println(prURL)
		return nil
	},
}

// buildPRBody constructs the PR description from task info.
func buildPRBody(taskID, title, content string) string {
	var sb strings.Builder

	sb.WriteString("## Task\n")
	sb.WriteString(fmt.Sprintf("[%s] %s\n", taskID, title))

	// Extract acceptance criteria section if present
	ac := extractSection(content, "Acceptance Criteria")
	if ac != "" {
		sb.WriteString("\n## Acceptance Criteria\n")
		sb.WriteString(ac)
		sb.WriteString("\n")
	}

	sb.WriteString("\n---\n")
	sb.WriteString(fmt.Sprintf("Pipeline task: %s\n", taskID))

	return sb.String()
}

// extractSection pulls out a markdown section by heading name.
// Returns empty string if not found.
func extractSection(content, heading string) string {
	// Look for ## Heading or # Heading patterns
	pattern := fmt.Sprintf(`(?i)##?\s*%s\s*\n`, regexp.QuoteMeta(heading))
	re := regexp.MustCompile(pattern)
	loc := re.FindStringIndex(content)
	if loc == nil {
		return ""
	}

	// Extract from after the heading to the next heading or end
	rest := content[loc[1]:]
	nextHeading := regexp.MustCompile(`\n##?\s+\S`)
	end := nextHeading.FindStringIndex(rest)
	if end != nil {
		return strings.TrimSpace(rest[:end[0]])
	}
	return strings.TrimSpace(rest)
}

func init() {
	taskPRCreateCmd.Flags().Bool("draft", false, "Create as draft PR")
	taskPRCreateCmd.Flags().Bool("dry-run", false, "Show what would be done without executing")

	taskCmd.AddCommand(taskPRCmd)
	taskPRCmd.AddCommand(taskPRCreateCmd)
}
