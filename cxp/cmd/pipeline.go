package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Pipeline automation commands",
}

var pipelinePollerCmd = &cobra.Command{
	Use:   "poller",
	Short: "Poll for actionable pipeline state and spawn M sessions",
	Long: `Runs a polling loop that checks for three trigger conditions every interval:

  1. New design ready — open designs without pipeline metadata
  2. Task needs review — tasks in needs-review with pipeline parent in implement phase
  3. Waiting condition satisfied — pipeline waiting_for shards all closed

For each trigger, spawns an M session in a tmux window (unless --dry-run).`,
	Example: `  cxp pipeline poller
  cxp pipeline poller --interval 60
  cxp pipeline poller --once --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		interval, _ := cmd.Flags().GetInt("interval")
		once, _ := cmd.Flags().GetBool("once")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		if interval < 1 {
			interval = 30
		}

		// Resolve repo from registry, falling back to cwd
		repoRoot, err := client.RepoForProject(cpClient.Config.Project)
		if err != nil {
			// Fall back to git repo root or cwd
			repoRoot = findRepoRoot()
			fmt.Fprintf(os.Stderr, "[poller] No repo registered for project %q, using %s\n", cpClient.Config.Project, repoRoot)
			fmt.Fprintf(os.Stderr, "[poller] Run `cxp pipeline setup` in the repo to register it.\n")
		}

		// Load pipeline config for this repo
		pipelineCfg, cfgErr := client.LoadPipelineConfig(repoRoot)
		if cfgErr != nil {
			pipelineCfg = client.DefaultPipelineConfig()
		}

		for {
			runTriggerChecks(ctx, repoRoot, pipelineCfg, dryRun)
			if once {
				break
			}
			time.Sleep(time.Duration(interval) * time.Second)
		}
		return nil
	},
}

// findRepoRoot returns the git repo root, falling back to cwd.
func findRepoRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	cwd, _ := os.Getwd()
	return cwd
}

func runTriggerChecks(ctx context.Context, repoRoot string, cfg *client.PipelineConfig, dryRun bool) {
	fmt.Printf("[%s] Polling for triggers...\n", time.Now().Format("15:04:05"))

	// Trigger 1: New design ready
	triggerNewDesigns(ctx, repoRoot, cfg, dryRun)

	// Trigger 2: Task needs review
	triggerTasksNeedingReview(ctx, repoRoot, cfg, dryRun)

	// Trigger 3: Waiting condition satisfied
	triggerSatisfiedWaits(ctx, repoRoot, cfg, dryRun)
}

func triggerNewDesigns(ctx context.Context, repoRoot string, cfg *client.PipelineConfig, dryRun bool) {
	designs, err := cpClient.FindNewDesigns(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [trigger:new-design] error: %v\n", err)
		return
	}
	for _, d := range designs {
		if !shouldSpawn(ctx, d.ID, dryRun) {
			continue
		}

		if dryRun {
			fmt.Printf("  [trigger:new-design] Would init pipeline and spawn M for %s (%s)\n", d.ID, d.Title)
			continue
		}

		// Init pipeline metadata
		if _, err := cpClient.PipelineInit(ctx, d.ID); err != nil {
			fmt.Fprintf(os.Stderr, "  [trigger:new-design] pipeline init %s: %v\n", d.ID, err)
			continue
		}

		spawnM(ctx, repoRoot, cfg, d.ID, "new-design")
	}
}

func triggerTasksNeedingReview(ctx context.Context, repoRoot string, cfg *client.PipelineConfig, dryRun bool) {
	tasks, err := cpClient.FindTasksNeedingReview(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [trigger:needs-review] error: %v\n", err)
		return
	}
	for _, t := range tasks {
		// Find parent design for lock check
		designID := findParentDesign(ctx, t.ID)
		if designID == "" {
			fmt.Fprintf(os.Stderr, "  [trigger:needs-review] no parent design for %s\n", t.ID)
			continue
		}

		if !shouldSpawn(ctx, designID, dryRun) {
			continue
		}

		if dryRun {
			fmt.Printf("  [trigger:needs-review] Would spawn M for review of %s (design: %s)\n", t.ID, designID)
			continue
		}

		spawnM(ctx, repoRoot, cfg, designID, "needs-review")
	}
}

func triggerSatisfiedWaits(ctx context.Context, repoRoot string, cfg *client.PipelineConfig, dryRun bool) {
	waits, err := cpClient.FindSatisfiedWaits(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [trigger:wait-satisfied] error: %v\n", err)
		return
	}
	for _, w := range waits {
		if !shouldSpawn(ctx, w.DesignID, dryRun) {
			continue
		}

		if dryRun {
			fmt.Printf("  [trigger:wait-satisfied] Would spawn M for %s (%s), all %d waiting_for closed\n",
				w.DesignID, w.Title, len(w.WaitingFor))
			continue
		}

		spawnM(ctx, repoRoot, cfg, w.DesignID, "wait-satisfied")
	}
}

// shouldSpawn checks the pipeline lock. Returns true if unlocked or stale.
func shouldSpawn(ctx context.Context, designID string, dryRun bool) bool {
	status, _, err := cpClient.PipelineLockCheck(ctx, designID)
	if err != nil {
		// If there's no pipeline metadata yet (e.g. trigger 1 before init), allow spawn
		return true
	}
	if status == "locked" {
		if dryRun {
			fmt.Printf("  [lock] %s is locked, skipping\n", designID)
		}
		return false
	}
	// "unlocked" or "stale" — proceed
	return true
}

// findParentDesign follows child-of edges from a task to find its parent design ID.
func findParentDesign(ctx context.Context, taskID string) string {
	edges, err := cpClient.GetShardEdges(ctx, taskID, "outgoing", []string{"child-of"})
	if err != nil {
		return ""
	}
	for _, e := range edges {
		if e.Type == "design" {
			return e.ShardID
		}
	}
	return ""
}

// spawnM spawns an M session in a tmux window.
func spawnM(ctx context.Context, repoRoot string, cfg *client.PipelineConfig, designID, trigger string) {
	// Resolve playbook path
	skillsDir := cfg.SkillsDir
	if skillsDir == "" {
		skillsDir = "skills"
	}
	playbookPath := filepath.Join(skillsDir, "m-playbook.md")

	tmuxSession := cfg.Dispatch.TmuxSession
	if tmuxSession == "" {
		tmuxSession = "main"
	}

	claudeFlags := cfg.Dispatch.ClaudeFlags
	if claudeFlags == "" {
		claudeFlags = "--print"
	}

	prompt := fmt.Sprintf(
		"You are M. Read your playbook at %s. Your pipeline shard is %s. Current trigger: %s.",
		playbookPath, designID, trigger,
	)
	escapedPrompt := strings.ReplaceAll(prompt, "'", "'\\''")
	shellCmd := fmt.Sprintf("cd '%s' && claude %s '%s'", strings.ReplaceAll(repoRoot, "'", "'\\''"), claudeFlags, escapedPrompt)

	windowName := fmt.Sprintf("m-%s", designID)
	tmuxArgs := []string{"new-window", "-n", windowName, "-t", tmuxSession, shellCmd}

	out, err := exec.CommandContext(ctx, "tmux", tmuxArgs...).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [spawn] failed for %s: %v\n%s\n", designID, err, string(out))
		return
	}

	fmt.Printf("  [spawn] M session started: window=%s trigger=%s\n", windowName, trigger)
}

func init() {
	pipelinePollerCmd.Flags().Int("interval", 30, "Polling interval in seconds")
	pipelinePollerCmd.Flags().Bool("once", false, "Run one check cycle and exit")
	pipelinePollerCmd.Flags().Bool("dry-run", false, "Print what would be spawned without executing")

	pipelineCmd.AddCommand(pipelinePollerCmd)
	rootCmd.AddCommand(pipelineCmd)
}
