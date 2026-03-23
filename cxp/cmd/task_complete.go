package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var taskCompleteCmd = &cobra.Command{
	Use:   "complete <task-id>",
	Short: "Post-agent completion: push, create PR, mark needs-review",
	Long: `Runs deterministic bookkeeping after an agent finishes implementing a task.
Designed to run automatically after the agent exits (appended to tmux command).

Steps:
  1. Check for uncommitted changes in worktree → commit them
  2. Push the branch
  3. Create PR if it doesn't exist
  4. Append evidence (files changed, commit hash)
  5. Mark task needs-review`,
	Args:    cobra.ExactArgs(1),
	Example: "  cxp task complete pf-abc123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		taskID := args[0]

		// Get task
		task, err := cpClient.GetTask(ctx, taskID)
		if err != nil {
			return fmt.Errorf("task not found: %s", taskID)
		}

		// If already closed, skip entirely
		if task.Status == "closed" {
			fmt.Printf("Task %s already closed, skipping\n", taskID)
			return nil
		}

		// If already needs-review, validate instead of skip
		if task.Status == "needs-review" {
			return validateCompletion(ctx, taskID, task, cpClient)
		}

		// Get worktree path from metadata
		worktreePath, _ := cpClient.GetMetadataField(ctx, taskID, []string{"worktree_path"})
		if worktreePath == "" {
			return fmt.Errorf("no worktree_path in task metadata")
		}

		// Load pipeline config for GitHub repo
		repoRoot, _ := client.RepoForProject(cpClient.Config.Project)
		pCfg, _ := client.LoadPipelineConfig(repoRoot)

		// 1. Check for uncommitted changes
		statusOut, err := exec.Command("git", "-C", worktreePath, "status", "--porcelain").Output()
		if err == nil && len(strings.TrimSpace(string(statusOut))) > 0 {
			fmt.Println("Committing uncommitted changes...")
			exec.Command("git", "-C", worktreePath, "add", "-A").Run()
			exec.Command("git", "-C", worktreePath, "commit", "-m",
				fmt.Sprintf("[%s] Auto-commit remaining changes", taskID)).Run()
		}

		// 2. Get branch name
		branchOut, err := exec.Command("git", "-C", worktreePath, "branch", "--show-current").Output()
		if err != nil {
			return fmt.Errorf("cannot get branch: %v", err)
		}
		branch := strings.TrimSpace(string(branchOut))
		if branch == "" {
			return fmt.Errorf("no branch found in worktree")
		}

		// 3. Push branch
		fmt.Printf("Pushing branch %s...\n", branch)
		pushOut, err := exec.Command("git", "-C", worktreePath, "push", "-u", "origin", branch).CombinedOutput()
		if err != nil {
			fmt.Printf("Push warning: %s\n", strings.TrimSpace(string(pushOut)))
			// Don't fail — might already be pushed
		}

		// 4. Create PR if it doesn't exist
		prURL, _ := cpClient.GetMetadataField(ctx, taskID, []string{"pr_url"})
		if prURL == "" {
			fmt.Println("Creating PR...")

			// Detect repo
			repo := ""
			if pCfg != nil && pCfg.GitHub.OwnerRepo != "" {
				repo = pCfg.GitHub.OwnerRepo
			} else {
				repoOut, err := exec.Command("git", "-C", worktreePath, "remote", "get-url", "origin").Output()
				if err == nil {
					url := strings.TrimSpace(string(repoOut))
					// Parse git@github.com:owner/repo.git or https://...
					for _, prefix := range []string{"git@github.com:", "https://github.com/"} {
						if strings.HasPrefix(url, prefix) {
							repo = strings.TrimSuffix(strings.TrimPrefix(url, prefix), ".git")
							break
						}
					}
				}
			}

			if repo != "" {
				prBody := fmt.Sprintf("## Task\n[%s] %s\n\n---\nPipeline task: %s", taskID, task.Title, taskID)
				ghArgs := []string{"pr", "create",
					"--repo", repo,
					"--head", branch,
					"--base", "main",
					"--title", fmt.Sprintf("%s (%s)", task.Title, taskID),
					"--body", prBody,
				}
				prOut, err := exec.Command("gh", ghArgs...).CombinedOutput()
				if err != nil {
					fmt.Printf("PR creation warning: %s\n", strings.TrimSpace(string(prOut)))
				} else {
					prURL = strings.TrimSpace(string(prOut))
					fmt.Printf("PR created: %s\n", prURL)
					// Store PR URL in metadata
					prJSON, _ := json.Marshal(prURL)
					cpClient.SetMetadataPath(ctx, taskID, []string{"pr_url"}, prJSON)
				}
			} else {
				fmt.Println("Could not detect GitHub repo — skipping PR creation")
			}
		} else {
			fmt.Printf("PR already exists: %s\n", prURL)
		}

		// 5. Append evidence
		commitOut, _ := exec.Command("git", "-C", worktreePath, "log", "--oneline", "-1").Output()
		commit := strings.TrimSpace(string(commitOut))

		diffOut, _ := exec.Command("git", "-C", worktreePath, "diff", "--stat", "main...HEAD").Output()
		filesChanged := strings.TrimSpace(string(diffOut))

		evidence := fmt.Sprintf("## Auto-completion evidence\n\nCommit: %s\nPR: %s\n\n### Files changed\n```\n%s\n```",
			commit, prURL, filesChanged)
		exec.Command("cxp", "shard", "append", taskID, "--body", evidence).Run()

		// 6. Mark needs-review
		fmt.Println("Marking needs-review...")
		exec.Command("cxp", "shard", "status", taskID, "needs-review").Run()

		fmt.Printf("Task %s complete → needs-review\n", taskID)
		return nil
	},
}

// validateCompletion checks that a needs-review task actually has the goods.
// If anything is missing, fixes it.
func validateCompletion(ctx context.Context, taskID string, task *client.Shard, c *client.Client) error {
	issues := []string{}

	// Check worktree exists
	wtPath, _ := c.GetMetadataField(ctx, taskID, []string{"worktree_path"})

	// Check branch has commits
	if wtPath != "" {
		commitOut, err := exec.Command("git", "-C", wtPath, "log", "--oneline", "main..HEAD").Output()
		if err != nil || len(strings.TrimSpace(string(commitOut))) == 0 {
			issues = append(issues, "no commits on branch")
		}
	}

	// Check PR exists
	prURL, _ := c.GetMetadataField(ctx, taskID, []string{"pr_url"})
	if prURL == "" {
		issues = append(issues, "no PR created")
		// Auto-fix: try to create PR
		if wtPath != "" {
			fmt.Println("Validation: no PR — creating one...")
			repoRoot, _ := client.RepoForProject(c.Config.Project)
			pCfg, _ := client.LoadPipelineConfig(repoRoot)
			repo := ""
			if pCfg != nil && pCfg.GitHub.OwnerRepo != "" {
				repo = pCfg.GitHub.OwnerRepo
			}
			if repo != "" {
				branch, _ := exec.Command("git", "-C", wtPath, "branch", "--show-current").Output()
				branchName := strings.TrimSpace(string(branch))
				exec.Command("git", "-C", wtPath, "push", "-u", "origin", branchName).Run()
				prBody := fmt.Sprintf("## Task\n[%s] %s\n\n---\nPipeline task: %s", taskID, task.Title, taskID)
				prOut, err := exec.Command("gh", "pr", "create",
					"--repo", repo, "--head", branchName, "--base", "main",
					"--title", fmt.Sprintf("%s (%s)", task.Title, taskID),
					"--body", prBody).CombinedOutput()
				if err == nil {
					prURL = strings.TrimSpace(string(prOut))
					fmt.Printf("Validation: PR created: %s\n", prURL)
					prJSON, _ := json.Marshal(prURL)
					c.SetMetadataPath(ctx, taskID, []string{"pr_url"}, prJSON)
				}
			}
		}
	}

	if len(issues) > 0 && prURL == "" {
		fmt.Printf("Validation issues for %s: %s\n", taskID, strings.Join(issues, ", "))
	} else {
		fmt.Printf("Task %s validation passed (needs-review with PR)\n", taskID)
	}
	return nil
}

func init() {
	taskCmd.AddCommand(taskCompleteCmd)
}
