package client

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// WorktreeInfo holds information about a task's worktree
type WorktreeInfo struct {
	TaskID       string `json:"task_id"`
	Branch       string `json:"branch"`
	WorktreePath string `json:"worktree_path"`
}

// Slugify converts a title to a URL-safe slug.
// Lowercase, replace non-alphanumeric runs with hyphens, trim hyphens, truncate to maxLen.
func Slugify(title string, maxLen int) string {
	s := strings.ToLower(title)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > maxLen {
		s = s[:maxLen]
		s = strings.TrimRight(s, "-")
	}
	return s
}

// WorktreeBranch returns the branch name for a task worktree.
func WorktreeBranch(taskID, title string) string {
	slug := Slugify(title, 50)
	if slug == "" {
		return taskID
	}
	return taskID + "/" + slug
}

// WorktreeBasePath returns the base directory for worktrees: ~/worktrees/<project>/
func WorktreeBasePath(project string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, "worktrees", project), nil
}

// WorktreePath returns the full worktree directory for a task: ~/worktrees/<project>/<task-id>/
func WorktreePath(project, taskID string) (string, error) {
	base, err := WorktreeBasePath(project)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, taskID), nil
}

// GitRepoRoot returns the root of the git repository containing dir.
func GitRepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CreateWorktree creates a git worktree and feature branch for a task.
// repoDir is the path to the main git repository.
// Returns the WorktreeInfo on success.
func CreateWorktree(repoDir, project, taskID, title string) (*WorktreeInfo, error) {
	branch := WorktreeBranch(taskID, title)
	wtPath, err := WorktreePath(project, taskID)
	if err != nil {
		return nil, err
	}

	// Check if worktree already exists
	if _, err := os.Stat(wtPath); err == nil {
		return nil, fmt.Errorf("worktree already exists at %s", wtPath)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
		return nil, fmt.Errorf("cannot create worktree parent directory: %w", err)
	}

	// Create worktree with new branch from main
	cmd := exec.Command("git", "-C", repoDir, "worktree", "add", "-b", branch, wtPath, "main")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git worktree add failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return &WorktreeInfo{
		TaskID:       taskID,
		Branch:       branch,
		WorktreePath: wtPath,
	}, nil
}

// RemoveWorktree removes a git worktree and its feature branch.
func RemoveWorktree(repoDir, project, taskID string) error {
	wtPath, err := WorktreePath(project, taskID)
	if err != nil {
		return err
	}

	// Remove worktree
	cmd := exec.Command("git", "-C", repoDir, "worktree", "remove", wtPath, "--force")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}

// RemoveWorktreeBranch deletes the feature branch associated with a worktree.
func RemoveWorktreeBranch(repoDir, branch string) error {
	cmd := exec.Command("git", "-C", repoDir, "branch", "-D", branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git branch -D failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
