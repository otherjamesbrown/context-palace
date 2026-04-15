package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

const (
	defaultCronDrift   = "0 3 * * *"
	defaultCronCanary  = "0 6 * * *"
	defaultCronTriage  = "0 5 * * 1"

	kbLabel            = "kb-maintenance"
	kbGapsTitle        = "KB Gaps Tracker"
	kbCanariesTitle    = "KB Canary Questions"
	kbEscalationsTitle = "KB Escalations"
)

var (
	scheduleInitRepoPAth    string
	scheduleInitCronDrift   string
	scheduleInitCronCanary  string
	scheduleInitCronTriage  string
	scheduleInitDryRun      bool
)

var scheduleInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Subscribe a project to KB maintenance workflows",
	Long: `Subscribe the current (or --project-flagged) project to the three KB maintenance
workflows: drift-scan, canary, and triage. Idempotent — safe to run multiple times.`,
	Example: `  cxp schedule init --repo-path /path/to/repo
  cxp schedule init --repo-path /path/to/repo --cron-drift "0 2 * * *"
  cxp schedule init --repo-path /path/to/repo --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if scheduleInitRepoPAth == "" {
			return fmt.Errorf("--repo-path is required")
		}

		ctx := context.Background()
		project := cpClient.Config.Project

		// Resolve the project ID prefix for custom shard IDs.
		prefix, err := cpClient.GetProjectIDPrefix(ctx)
		if err != nil {
			return fmt.Errorf("could not determine project ID prefix: %w", err)
		}

		gapsID := prefix + "-kb-gaps"
		canariesID := prefix + "-kb-canaries"
		escalationsID := prefix + "-kb-escalations"

		// ------------------------------------------------------------------
		// Step 1: Resolve or plan the three KB singleton shards.
		// ------------------------------------------------------------------
		type shardPlan struct {
			id     string
			title  string
			action string // "create" | "already exists"
		}

		gapsPlan, err := resolveKBShard(ctx, gapsID, kbGapsTitle, "# KB Gaps\n")
		if err != nil {
			return fmt.Errorf("gaps shard: %w", err)
		}
		canariesPlan, err := resolveKBShard(ctx, canariesID, kbCanariesTitle, "[]\n")
		if err != nil {
			return fmt.Errorf("canaries shard: %w", err)
		}
		escalationsPlan, err := resolveKBShard(ctx, escalationsID, kbEscalationsTitle, "# KB Escalations\n")
		if err != nil {
			return fmt.Errorf("escalations shard: %w", err)
		}

		// ------------------------------------------------------------------
		// Step 2: Resolve or plan the three schedules.
		// ------------------------------------------------------------------
		type schedulePlan struct {
			name      string
			workflow  string
			cron      string
			config    json.RawMessage
			action    string // "create" | "already exists"
			nextRunAt string
		}

		driftConfig := mustJSON(map[string]any{
			"repo_path":  scheduleInitRepoPAth,
			"gaps_shard": gapsPlan.id,
		})
		canaryConfig := mustJSON(map[string]any{
			"canary_shard": canariesPlan.id,
			"gaps_shard":   gapsPlan.id,
		})
		triageConfig := mustJSON(map[string]any{
			"gaps_shard":        gapsPlan.id,
			"escalations_shard": escalationsPlan.id,
		})

		driftPlan := &schedulePlan{name: "drift-scan", workflow: "drift-scan", cron: scheduleInitCronDrift, config: driftConfig}
		canaryPlan := &schedulePlan{name: "canary", workflow: "canary", cron: scheduleInitCronCanary, config: canaryConfig}
		triagePlan := &schedulePlan{name: "triage", workflow: "triage", cron: scheduleInitCronTriage, config: triageConfig}

		for _, sp := range []*schedulePlan{driftPlan, canaryPlan, triagePlan} {
			existing, found, err := cpClient.GetScheduleIfExists(ctx, project, sp.name)
			if err != nil {
				return fmt.Errorf("checking schedule %s: %w", sp.name, err)
			}
			if found {
				sp.action = "already exists"
				if existing.NextRunAt != nil {
					sp.nextRunAt = existing.NextRunAt.Local().Format("2006-01-02 15:04")
				}
			} else {
				sp.action = "create"
			}
		}

		// If all three schedules already exist, report and exit.
		allExist := driftPlan.action == "already exists" &&
			canaryPlan.action == "already exists" &&
			triagePlan.action == "already exists"

		if allExist {
			fmt.Printf("Already subscribed: all 3 schedules exist for %s\n", project)
			return nil
		}

		// ------------------------------------------------------------------
		// Dry-run: print plan and stop.
		// ------------------------------------------------------------------
		if scheduleInitDryRun {
			fmt.Printf("Dry-run — no changes will be written.\n\n")
			fmt.Printf("Project: %s\n\n", project)
			fmt.Printf("Shards:\n")
			fmt.Printf("  gaps shard:        %s (%s)\n", gapsPlan.id, gapsPlan.action)
			fmt.Printf("  canaries shard:    %s (%s)\n", canariesPlan.id, canariesPlan.action)
			fmt.Printf("  escalations shard: %s (%s)\n", escalationsPlan.id, escalationsPlan.action)
			fmt.Printf("\nSchedules:\n")
			fmt.Printf("  drift-scan:        %s  cron=%s\n", driftPlan.action, driftPlan.cron)
			fmt.Printf("  canary:            %s  cron=%s\n", canaryPlan.action, canaryPlan.cron)
			fmt.Printf("  triage:            %s  cron=%s\n", triagePlan.action, triagePlan.cron)
			return nil
		}

		// ------------------------------------------------------------------
		// Step 3: Create shards and schedules (with compensating error report).
		// ------------------------------------------------------------------
		var createdShards []string

		if gapsPlan.action == "create" {
			id, err := cpClient.CreateKnowledgeDocWithID(ctx, gapsPlan.id, gapsPlan.title, "# KB Gaps\n", "reference", []string{kbLabel})
			if err != nil {
				return fmt.Errorf("create gaps shard: %w", err)
			}
			gapsPlan.id = id
			createdShards = append(createdShards, id)
		}

		if canariesPlan.action == "create" {
			id, err := cpClient.CreateKnowledgeDocWithID(ctx, canariesPlan.id, canariesPlan.title, "[]\n", "reference", []string{kbLabel})
			if err != nil {
				reportCompensation(createdShards)
				return fmt.Errorf("create canaries shard: %w", err)
			}
			canariesPlan.id = id
			createdShards = append(createdShards, id)
		}

		if escalationsPlan.action == "create" {
			id, err := cpClient.CreateKnowledgeDocWithID(ctx, escalationsPlan.id, escalationsPlan.title, "# KB Escalations\n", "reference", []string{kbLabel})
			if err != nil {
				reportCompensation(createdShards)
				return fmt.Errorf("create escalations shard: %w", err)
			}
			escalationsPlan.id = id
			createdShards = append(createdShards, id)
		}

		// Update configs now that shard IDs are resolved.
		driftPlan.config = mustJSON(map[string]any{
			"repo_path":  scheduleInitRepoPAth,
			"gaps_shard": gapsPlan.id,
		})
		canaryPlan.config = mustJSON(map[string]any{
			"canary_shard": canariesPlan.id,
			"gaps_shard":   gapsPlan.id,
		})
		triagePlan.config = mustJSON(map[string]any{
			"gaps_shard":        gapsPlan.id,
			"escalations_shard": escalationsPlan.id,
		})

		for _, sp := range []*schedulePlan{driftPlan, canaryPlan, triagePlan} {
			if sp.action != "create" {
				continue
			}
			created, err := cpClient.CreateSchedule(ctx, client.CreateScheduleInput{
				Project:       project,
				Name:          sp.name,
				WorkflowType:  sp.workflow,
				ScheduleExpr:  sp.cron,
				OverlapPolicy: client.ScheduleOverlapSkip,
				Config:        sp.config,
			})
			if err != nil {
				reportCompensation(createdShards)
				return fmt.Errorf("create schedule %s: %w", sp.name, err)
			}
			sp.action = "created"
			if created.NextRunAt != nil {
				sp.nextRunAt = created.NextRunAt.Local().Format("2006-01-02 15:04")
			}
		}

		// ------------------------------------------------------------------
		// Output.
		// ------------------------------------------------------------------
		fmt.Printf("Subscribed %s to KB maintenance workflows\n", project)
		printShardLine("gaps shard", gapsPlan.id, gapsPlan.action)
		printShardLine("canaries shard", canariesPlan.id, canariesPlan.action)
		printShardLine("escalations shard", escalationsPlan.id, escalationsPlan.action)
		printScheduleLine("drift-scan", driftPlan.action, driftPlan.nextRunAt)
		printScheduleLine("canary", canaryPlan.action, canaryPlan.nextRunAt)
		printScheduleLine("triage", triagePlan.action, triagePlan.nextRunAt)
		fmt.Printf("\nRun 'cxp schedule run drift-scan' to test.\n")
		return nil
	},
}

// resolveKBShard checks whether a KB maintenance shard exists by label+title.
// Returns a plan stub — actual creation happens later.
func resolveKBShard(ctx context.Context, candidateID, title, _ string) (*struct {
	id     string
	title  string
	action string
}, error) {
	existing, err := cpClient.FindShardByLabelAndTitle(ctx, kbLabel, title, "knowledge")
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return &struct {
			id     string
			title  string
			action string
		}{id: existing.ID, title: title, action: "already exists"}, nil
	}
	return &struct {
		id     string
		title  string
		action string
	}{id: candidateID, title: title, action: "create"}, nil
}

func printShardLine(label, id, action string) {
	fmt.Printf("  %-20s %s (%s)\n", label+":", id, action)
}

func printScheduleLine(name, action, nextRunAt string) {
	if nextRunAt != "" {
		fmt.Printf("  %-20s %s (next run: %s)\n", name+":", action, nextRunAt)
	} else {
		fmt.Printf("  %-20s %s\n", name+":", action)
	}
}

func reportCompensation(created []string) {
	if len(created) == 0 {
		return
	}
	fmt.Printf("Note: the following shards were created before the error and were left in place:\n")
	for _, id := range created {
		fmt.Printf("  %s\n", id)
	}
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("mustJSON: %v", err))
	}
	return json.RawMessage(b)
}

func init() {
	scheduleInitCmd.Flags().StringVar(&scheduleInitRepoPAth, "repo-path", "", "Path to the repository root (required for drift-scan)")
	scheduleInitCmd.Flags().StringVar(&scheduleInitCronDrift, "cron-drift", defaultCronDrift, "Cron expression for the drift-scan schedule")
	scheduleInitCmd.Flags().StringVar(&scheduleInitCronCanary, "cron-canary", defaultCronCanary, "Cron expression for the canary schedule")
	scheduleInitCmd.Flags().StringVar(&scheduleInitCronTriage, "cron-triage", defaultCronTriage, "Cron expression for the triage schedule")
	scheduleInitCmd.Flags().BoolVar(&scheduleInitDryRun, "dry-run", false, "Print what would be created without writing anything")
}
