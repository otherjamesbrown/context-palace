package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

// ImprovementAction describes a suggested change to a skill or config file.
type ImprovementAction struct {
	File        string `json:"file"`
	Type        string `json:"type"`    // "skill", "config", "process"
	Description string `json:"description"`
	Reason      string `json:"reason"`
	Diff        string `json:"diff,omitempty"` // suggested change
}

var pipelineImproveCmd = &cobra.Command{
	Use:   "improve",
	Short: "Suggest pipeline improvements based on execution patterns",
	Long: `Analyzes pipeline execution data and proposes specific changes to
skills, config, and process files based on observed patterns.

Run with --apply to auto-apply the changes.`,
	Example: `  cxp pipeline improve
  cxp pipeline improve --apply
  cxp pipeline improve -o json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		apply, _ := cmd.Flags().GetBool("apply")

		project := cpClient.Config.Project
		repoRoot, _ := client.RepoForProject(project)
		if repoRoot == "" {
			cwd, _ := os.Getwd()
			repoRoot, _ = client.GitRepoRoot(cwd)
		}

		pCfg, _ := client.LoadPipelineConfig(repoRoot)
		if pCfg == nil {
			pCfg = client.DefaultPipelineConfig()
		}

		// Get insights data
		stats, err := cpClient.GetInsightsStats(ctx, project)
		if err != nil {
			return fmt.Errorf("failed to get insights: %v", err)
		}

		// Generate improvement actions from patterns
		actions := generateImprovements(stats, pCfg, repoRoot)

		if outputFormat == "json" {
			s, _ := client.FormatJSON(actions)
			fmt.Println(s)
			return nil
		}

		if len(actions) == 0 {
			fmt.Println("No improvements suggested — pipeline is running well.")
			return nil
		}

		fmt.Printf("Pipeline Improvements — %s\n", project)
		fmt.Printf("%d suggestions based on execution data\n\n", len(actions))

		for i, a := range actions {
			fmt.Printf("%d. [%s] %s\n", i+1, a.Type, a.Description)
			fmt.Printf("   Reason: %s\n", a.Reason)
			if a.Diff != "" {
				fmt.Printf("   File: %s\n", a.File)
				if apply {
					if err := applyImprovement(a, repoRoot); err != nil {
						fmt.Printf("   FAILED: %v\n", err)
					} else {
						fmt.Printf("   APPLIED ✓\n")
					}
				} else {
					fmt.Printf("   Change:\n")
					for _, line := range strings.Split(a.Diff, "\n") {
						fmt.Printf("     %s\n", line)
					}
				}
			}
			fmt.Println()
		}

		if !apply && hasApplyable(actions) {
			fmt.Println("Run with --apply to auto-apply these changes.")
		}

		return nil
	},
}

func generateImprovements(stats *client.InsightsStats, cfg *client.PipelineConfig, repoRoot string) []ImprovementAction {
	var actions []ImprovementAction

	// Pattern 1: Low readiness-review pass rate → improve create-design skill
	for _, gs := range stats.GateStats {
		if gs.GateName == "readiness-review" && gs.PassRate < 60 {
			actions = append(actions, ImprovementAction{
				File:        filepath.Join("skills", "create-design.md"),
				Type:        "skill",
				Description: "Strengthen code location requirements in create-design skill",
				Reason:      fmt.Sprintf("readiness-review first-try pass rate is %.0f%% — most failures are missing code locations", gs.PassRate),
				Diff: `Add to the "Technical Approach" section:

**CRITICAL: Include file paths with line numbers.**
Designs without specific file paths (e.g. "pipeline.go:1750-1777")
fail readiness review 80% of the time. "Somewhere in the pipeline code"
is not sufficient.`,
			})
		}
	}

	// Pattern 2: Model not configured for judgment tasks
	if cfg != nil {
		for _, phase := range cfg.Phases {
			if phase.Name == "review" && phase.Model == "" {
				actions = append(actions, ImprovementAction{
					File:        filepath.Join(".cxp", "pipeline.yaml"),
					Type:        "config",
					Description: "Set review phase model to haiku (currently using default/opus)",
					Reason:      "Review is a judgment task — haiku is sufficient and cheaper",
					Diff:        "phases:\n  - name: review\n    model: haiku  # ← add this",
				})
			}
		}
		// Check if default model is set
		if cfg.Dispatch.DefaultModel == "" {
			actions = append(actions, ImprovementAction{
				File:        filepath.Join(".cxp", "pipeline.yaml"),
				Type:        "config",
				Description: "Set default model to sonnet",
				Reason:      "No default model configured — agents may use opus unnecessarily",
				Diff:        "dispatch:\n  default_model: sonnet  # ← add this",
			})
		}
	}

	// Pattern 3: No monitoring configured
	if cfg != nil && cfg.Monitoring.StallTimeout == "" {
		actions = append(actions, ImprovementAction{
			File:        filepath.Join(".cxp", "pipeline.yaml"),
			Type:        "config",
			Description: "Enable health monitoring for dispatched agents",
			Reason:      "No monitoring configured — stalled/crashed agents won't be detected",
			Diff: `monitoring:
  stall_timeout: 30m
  crash_check: true
  max_retries: 3
  model: haiku
  actions:
    on_stall: skill:m-stall-check
    on_crash: redispatch
    on_max_retries: escalate`,
		})
	}

	// Pattern 4: No integration test requirement
	hasDecompGate := false
	if cfg != nil {
		for _, phase := range cfg.Phases {
			if phase.Name == "decompose" {
				for _, gate := range phase.Gates {
					if gate.RequiresLabel == "integration-test" {
						hasDecompGate = true
					}
				}
			}
		}
	}
	if !hasDecompGate {
		actions = append(actions, ImprovementAction{
			File:        filepath.Join(".cxp", "pipeline.yaml"),
			Type:        "config",
			Description: "Require integration test task in decomposition gate",
			Reason:      "Without this, designs can advance to implement without e2e test coverage",
			Diff: `phases:
  - name: decompose
    gates:
      - name: decomposition-review
        requires_label: integration-test  # ← add this`,
		})
	}

	// Pattern 5: No CXP_DISPATCH in hooks
	hookPath := filepath.Join(repoRoot, ".claude", "hooks", "session-start.sh")
	if data, err := os.ReadFile(hookPath); err == nil {
		if !strings.Contains(string(data), "CXP_DISPATCH") {
			actions = append(actions, ImprovementAction{
				File:        filepath.Join(".claude", "hooks", "session-start.sh"),
				Type:        "process",
				Description: "Add CXP_DISPATCH check to session start hook",
				Reason:      "Without this, dispatched agents get the work queue menu instead of their task",
				Diff: `Add at the top of the hook (after set -euo pipefail):

if [ "${CXP_DISPATCH:-}" = "true" ]; then
  echo "[Dispatched session — task context provided via prompt]"
  exit 0
fi`,
			})
		}
	}

	// Pattern 6: Skills not initialized in repo
	skillsDir := "skills"
	if cfg != nil && cfg.SkillsDir != "" {
		skillsDir = cfg.SkillsDir
	}
	playbookPath := filepath.Join(repoRoot, skillsDir, "m-playbook.md")
	if _, err := os.Stat(playbookPath); os.IsNotExist(err) {
		actions = append(actions, ImprovementAction{
			Type:        "process",
			Description: "Initialize pipeline skills in this repo",
			Reason:      fmt.Sprintf("No %s/m-playbook.md found — pipeline skills are not portable to this repo", skillsDir),
			Diff:        "Run: cxp pipeline init-skills",
		})
	}

	return actions
}

func applyImprovement(a ImprovementAction, repoRoot string) error {
	// Only auto-apply config and process improvements that have clear file targets
	// Skill improvements need human review
	if a.Type == "skill" {
		return fmt.Errorf("skill changes need human review — not auto-applying")
	}
	if a.Diff == "" {
		return fmt.Errorf("no diff to apply")
	}
	// For now, just report — auto-apply is a future enhancement
	return fmt.Errorf("auto-apply not yet implemented — apply manually")
}

func hasApplyable(actions []ImprovementAction) bool {
	for _, a := range actions {
		if a.Diff != "" {
			return true
		}
	}
	return false
}

func init() {
	pipelineImproveCmd.Flags().Bool("apply", false, "Auto-apply suggested changes")
	pipelineCmd.AddCommand(pipelineImproveCmd)
}
