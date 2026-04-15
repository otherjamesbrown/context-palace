package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/otherjamesbrown/context-palace/cxp/internal/scheduler/workflows"
	"github.com/spf13/cobra"
)

var (
	canaryShardFlag    string
	canaryQuestionFlag string
	canaryExpectedFlag string
	canarySourceFlag   string
)

// scheduleCanaryCmd is the parent for canary subcommands (add, list).
var scheduleCanaryCmd = &cobra.Command{
	Use:   "canary",
	Short: "Manage canary questions for retrieval-quality testing",
}

// scheduleSeedCanariesCmd seeds the default canary question pack into the project's canary shard.
var scheduleSeedCanariesCmd = &cobra.Command{
	Use:   "seed-canaries",
	Short: "Seed the default canary question pack into the project's canary shard",
	Long: `Merges the built-in generic starter pack (13 questions) into the project's
canary shard. Idempotent: existing questions are preserved; only new questions
(by question text) are appended.

The shard is identified by --shard. If not provided, the command looks up the
"canary" schedule for the current project and reads canary_shard from its config.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		shardID, err := resolveCanaryShard(ctx, canaryShardFlag)
		if err != nil {
			return err
		}

		existing, err := workflows.LoadCanaryQuestionsFromShard(ctx, shardID)
		if err != nil {
			return fmt.Errorf("load existing canary questions from %s: %w", shardID, err)
		}

		defaults := workflows.DefaultCanaryQuestions()
		merged := workflows.MergeCanaryQuestions(existing, defaults)
		added := len(merged) - len(existing)

		if added == 0 {
			fmt.Printf("Canary shard %s already contains all default questions (%d questions, nothing added)\n",
				shardID, len(existing))
			return nil
		}

		body, err := workflows.FormatCanaryQuestionsYAML(merged)
		if err != nil {
			return fmt.Errorf("format canary questions: %w", err)
		}

		summary := fmt.Sprintf("seed-canaries: add %d default questions (total: %d)", added, len(merged))
		if _, err := cpClient.UpdateKnowledgeDoc(ctx, shardID, body, summary); err != nil {
			return fmt.Errorf("update canary shard %s: %w", shardID, err)
		}

		fmt.Printf("Seeded %d default canary questions into %s (total: %d)\n", added, shardID, len(merged))
		return nil
	},
}

// scheduleCanaryAddCmd appends a single canary question to the project's canary shard.
var scheduleCanaryAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Append a canary question to the project's canary shard",
	Example: `  cxp schedule canary add \
    --question "What model does the classify_project stage use?" \
    --expected "gemini-2.5-flash,classify_project,ai_routing_rules" \
    --source pf-861f0c`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if canaryQuestionFlag == "" {
			return fmt.Errorf("--question is required")
		}
		if canaryExpectedFlag == "" {
			return fmt.Errorf("--expected is required (comma-separated list of expected facts)")
		}

		shardID, err := resolveCanaryShard(ctx, canaryShardFlag)
		if err != nil {
			return err
		}

		parts := strings.Split(canaryExpectedFlag, ",")
		facts := make([]string, 0, len(parts))
		for _, p := range parts {
			if f := strings.TrimSpace(p); f != "" {
				facts = append(facts, f)
			}
		}
		if len(facts) == 0 {
			return fmt.Errorf("--expected must contain at least one fact")
		}

		newQ := workflows.CanaryQuestion{
			Q:             strings.TrimSpace(canaryQuestionFlag),
			ExpectedFacts: facts,
			SourceKB:      strings.TrimSpace(canarySourceFlag),
		}

		existing, err := workflows.LoadCanaryQuestionsFromShard(ctx, shardID)
		if err != nil {
			return fmt.Errorf("load existing canary questions from %s: %w", shardID, err)
		}

		merged := workflows.MergeCanaryQuestions(existing, []workflows.CanaryQuestion{newQ})
		if len(merged) == len(existing) {
			fmt.Printf("Question already exists in %s (not added)\n", shardID)
			return nil
		}

		body, err := workflows.FormatCanaryQuestionsYAML(merged)
		if err != nil {
			return fmt.Errorf("format canary questions: %w", err)
		}

		summary := fmt.Sprintf("canary add: %q", newQ.Q)
		if _, err := cpClient.UpdateKnowledgeDoc(ctx, shardID, body, summary); err != nil {
			return fmt.Errorf("update canary shard %s: %w", shardID, err)
		}

		fmt.Printf("Added canary question to %s (total: %d)\n", shardID, len(merged))
		return nil
	},
}

// scheduleCanaryListCmd prints the current canary question set with validation flags.
var scheduleCanaryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List canary questions with validation flags",
	Long: `Lists all canary questions in the project's canary shard.
Flags validation issues:
  [WARN: no source_kb]      — question has no source article linked
  [WARN: no expected_facts] — question will always fail (filtered out at run time)
  [WARN: duplicate]         — duplicate question text detected`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		shardID, err := resolveCanaryShard(ctx, canaryShardFlag)
		if err != nil {
			return err
		}

		questions, err := workflows.LoadCanaryQuestionsFromShard(ctx, shardID)
		if err != nil {
			return fmt.Errorf("load canary questions from %s: %w", shardID, err)
		}

		if outputFormat == "json" || outputFormat == "yaml" {
			s, _ := client.FormatOutput(questions, outputFormat)
			fmt.Println(s)
			return nil
		}

		if len(questions) == 0 {
			fmt.Printf("No canary questions in %s\n", shardID)
			fmt.Printf("Run: cxp schedule seed-canaries --shard %s\n", shardID)
			return nil
		}

		seen := make(map[string]bool)
		warnings := 0

		fmt.Printf("Canary questions in %s (%d total):\n\n", shardID, len(questions))
		for i, q := range questions {
			key := strings.ToLower(strings.TrimSpace(q.Q))
			var flags []string
			if q.SourceKB == "" {
				flags = append(flags, "WARN: no source_kb")
			}
			if len(q.ExpectedFacts) == 0 {
				flags = append(flags, "WARN: no expected_facts")
			}
			if seen[key] {
				flags = append(flags, "WARN: duplicate")
			}
			seen[key] = true

			prefix := fmt.Sprintf("%d.", i+1)
			fmt.Printf("  %s %s\n", prefix, q.Q)
			fmt.Printf("     facts:  %s\n", strings.Join(q.ExpectedFacts, ", "))
			if q.SourceKB != "" {
				fmt.Printf("     source: %s\n", q.SourceKB)
			}
			if len(flags) > 0 {
				fmt.Printf("     [%s]\n", strings.Join(flags, "] ["))
				warnings++
			}
			fmt.Println()
		}

		if warnings > 0 {
			fmt.Printf("%d validation warning(s) — update source_kb values with: cxp schedule canary add\n", warnings)
		}
		return nil
	},
}

// resolveCanaryShard returns the canary shard ID from the flag, or auto-detects it from
// the "canary" schedule config for the current project.
func resolveCanaryShard(ctx context.Context, flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}

	schedule, err := cpClient.GetSchedule(ctx, cpClient.Config.Project, "canary")
	if err != nil {
		return "", fmt.Errorf("--shard not specified and no 'canary' schedule found for project %s: %w", cpClient.Config.Project, err)
	}

	var cfg struct {
		CanaryShard string `json:"canary_shard"`
	}
	if len(schedule.Config) > 0 {
		if err := json.Unmarshal(schedule.Config, &cfg); err != nil {
			return "", fmt.Errorf("parse canary schedule config: %w", err)
		}
	}
	if cfg.CanaryShard == "" {
		return "", fmt.Errorf("canary schedule found but canary_shard is not set in its config; use --shard")
	}
	return cfg.CanaryShard, nil
}

func init() {
	scheduleSeedCanariesCmd.Flags().StringVar(&canaryShardFlag, "shard", "", "Canary shard ID (auto-detected from canary schedule if omitted)")

	scheduleCanaryAddCmd.Flags().StringVar(&canaryShardFlag, "shard", "", "Canary shard ID (auto-detected from canary schedule if omitted)")
	scheduleCanaryAddCmd.Flags().StringVar(&canaryQuestionFlag, "question", "", "Question text")
	scheduleCanaryAddCmd.Flags().StringVar(&canaryExpectedFlag, "expected", "", "Comma-separated list of expected facts")
	scheduleCanaryAddCmd.Flags().StringVar(&canarySourceFlag, "source", "", "Source KB shard ID (the article this question tests)")

	scheduleCanaryListCmd.Flags().StringVar(&canaryShardFlag, "shard", "", "Canary shard ID (auto-detected from canary schedule if omitted)")

	scheduleCanaryCmd.AddCommand(scheduleCanaryAddCmd)
	scheduleCanaryCmd.AddCommand(scheduleCanaryListCmd)

	scheduleCmd.AddCommand(scheduleSeedCanariesCmd)
	scheduleCmd.AddCommand(scheduleCanaryCmd)
}
