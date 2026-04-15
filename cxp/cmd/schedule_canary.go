package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/otherjamesbrown/context-palace/cxp/internal/scheduler/workflows"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	canaryQuestionFlag string
	canaryExpectedFlag string
	canarySourceFlag   string
)

var scheduleCanaryCmd = &cobra.Command{
	Use:   "canary",
	Short: "Manage canary questions for the project",
}

var scheduleSeedCanariesCmd = &cobra.Command{
	Use:   "seed-canaries",
	Short: "Seed the project's canaries shard with the default question pack",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		shardID, err := findCanaryShardID(ctx)
		if err != nil {
			return err
		}

		existing, rawContent, err := loadCanaryQuestionsWithContent(ctx, shardID)
		if err != nil {
			return fmt.Errorf("load existing canary questions: %w", err)
		}

		defaults, err := workflows.DefaultCanaryQuestions()
		if err != nil {
			return fmt.Errorf("load default canary questions: %w", err)
		}

		merged, skipped := mergeCanaryQuestions(existing, defaults)

		added := len(merged) - len(existing)
		if added == 0 {
			fmt.Printf("Seeded 0 questions (%d already present, skipped)\n", skipped)
			return nil
		}

		updatedContent, err := replaceCanaryYAML(rawContent, merged)
		if err != nil {
			return fmt.Errorf("marshal canary questions: %w", err)
		}

		if err := cpClient.UpdateShardContent(ctx, shardID, updatedContent); err != nil {
			return fmt.Errorf("update canary shard: %w", err)
		}

		fmt.Printf("Seeded %d questions (%d already present, skipped)\n", added, skipped)
		return nil
	},
}

var scheduleCanaryAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Append a single canary question to the project's canaries shard",
	RunE: func(cmd *cobra.Command, args []string) error {
		if canaryQuestionFlag == "" {
			return fmt.Errorf("--question is required")
		}
		if canaryExpectedFlag == "" {
			return fmt.Errorf("--expected is required")
		}

		ctx := context.Background()

		shardID, err := findCanaryShardID(ctx)
		if err != nil {
			return err
		}

		newQ := workflows.CanaryQuestion{
			Q:             canaryQuestionFlag,
			ExpectedFacts: parseCommaList(canaryExpectedFlag),
			SourceKB:      canarySourceFlag,
		}

		existing, rawContent, err := loadCanaryQuestionsWithContent(ctx, shardID)
		if err != nil {
			return fmt.Errorf("load existing canary questions: %w", err)
		}

		all := append(existing, newQ)
		updatedContent, err := replaceCanaryYAML(rawContent, all)
		if err != nil {
			return fmt.Errorf("marshal canary questions: %w", err)
		}

		if err := cpClient.UpdateShardContent(ctx, shardID, updatedContent); err != nil {
			return fmt.Errorf("update canary shard: %w", err)
		}

		fmt.Printf("Added canary question: %s\n", canaryQuestionFlag)
		return nil
	},
}

var scheduleCanaryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List canary questions with validation flags",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		shardID, err := findCanaryShardID(ctx)
		if err != nil {
			return err
		}

		questions, err := loadAllCanaryQuestions(ctx, shardID)
		if err != nil {
			return fmt.Errorf("load canary questions: %w", err)
		}

		if len(questions) == 0 {
			fmt.Println("No canary questions found. Run: cxp schedule seed-canaries")
			return nil
		}

		// Build seen-count map for duplicate detection.
		seen := make(map[string]int, len(questions))
		for _, q := range questions {
			seen[strings.ToLower(q.Q)]++
		}

		table := client.NewTable("INDEX", "QUESTION", "FACTS", "SOURCE", "FLAGS")
		for i, q := range questions {
			flags := canaryValidationFlags(q, seen)
			table.AddRow(
				fmt.Sprintf("%d", i+1),
				client.Truncate(q.Q, 55),
				strings.Join(q.ExpectedFacts, ", "),
				q.SourceKB,
				strings.Join(flags, "; "),
			)
		}
		fmt.Print(table.String())
		return nil
	},
}

// findCanaryShardID lists schedules for the current project and returns the
// canary_shard ID from the first schedule with workflow_type == "canary".
func findCanaryShardID(ctx context.Context) (string, error) {
	schedules, err := cpClient.ListSchedules(ctx, cpClient.Config.Project)
	if err != nil {
		return "", fmt.Errorf("list schedules: %w", err)
	}

	for _, s := range schedules {
		if s.WorkflowType != "canary" {
			continue
		}
		var cfg workflows.CanaryConfig
		if err := json.Unmarshal(s.Config, &cfg); err != nil {
			continue
		}
		if cfg.CanaryShard != "" {
			return cfg.CanaryShard, nil
		}
	}

	return "", fmt.Errorf(
		"no canary schedule found for project %q\n"+
			"  Create one with: cxp schedule create canary --workflow canary --cron '0 8 * * *' "+
			"--config '{\"canary_shard\": \"<shard-id>\", \"gaps_shard\": \"<shard-id>\"}'",
		cpClient.Config.Project,
	)
}

// loadCanaryQuestionsWithContent returns the valid questions and the full raw shard
// content so callers can rebuild and update the shard body.
func loadCanaryQuestionsWithContent(ctx context.Context, shardID string) ([]workflows.CanaryQuestion, string, error) {
	shard, err := cpClient.GetShard(ctx, shardID)
	if err != nil {
		return nil, "", fmt.Errorf("get shard %s: %w", shardID, err)
	}

	content := shard.Content
	yamlBlock := extractYAMLBlockFrom(content)

	var questions []workflows.CanaryQuestion
	if yamlBlock != "" {
		if err := yaml.Unmarshal([]byte(yamlBlock), &questions); err != nil {
			return nil, content, fmt.Errorf("parse canary YAML: %w", err)
		}
	}

	// Return only valid questions (same filter as the canary runner).
	valid := questions[:0]
	for _, q := range questions {
		if q.Q != "" && len(q.ExpectedFacts) > 0 {
			valid = append(valid, q)
		}
	}
	return valid, content, nil
}

// loadAllCanaryQuestions returns every question in the shard, including partially
// invalid ones, so they can be shown with validation warnings.
func loadAllCanaryQuestions(ctx context.Context, shardID string) ([]workflows.CanaryQuestion, error) {
	shard, err := cpClient.GetShard(ctx, shardID)
	if err != nil {
		return nil, fmt.Errorf("get shard %s: %w", shardID, err)
	}

	yamlBlock := extractYAMLBlockFrom(shard.Content)
	if yamlBlock == "" {
		return nil, nil
	}

	var questions []workflows.CanaryQuestion
	if err := yaml.Unmarshal([]byte(yamlBlock), &questions); err != nil {
		return nil, fmt.Errorf("parse canary YAML: %w", err)
	}
	return questions, nil
}

// mergeCanaryQuestions merges newQuestions into existing, skipping any whose
// Q text already appears (case-insensitive). Returns the merged slice and the
// number of questions that were skipped as duplicates.
func mergeCanaryQuestions(existing, newQuestions []workflows.CanaryQuestion) ([]workflows.CanaryQuestion, int) {
	seen := make(map[string]struct{}, len(existing))
	for _, q := range existing {
		seen[strings.ToLower(q.Q)] = struct{}{}
	}

	skipped := 0
	result := append([]workflows.CanaryQuestion(nil), existing...)
	for _, q := range newQuestions {
		if _, ok := seen[strings.ToLower(q.Q)]; ok {
			skipped++
			continue
		}
		result = append(result, q)
		seen[strings.ToLower(q.Q)] = struct{}{}
	}
	return result, skipped
}

// replaceCanaryYAML replaces the first ```yaml block in content with a YAML
// serialisation of questions. If no block is found, one is appended.
func replaceCanaryYAML(content string, questions []workflows.CanaryQuestion) (string, error) {
	yamlBytes, err := yaml.Marshal(questions)
	if err != nil {
		return "", err
	}
	newBlock := string(yamlBytes)

	const fence = "```yaml"
	if start := strings.Index(content, fence); start >= 0 {
		rest := content[start+len(fence):]
		if end := strings.Index(rest, "```"); end >= 0 {
			before := content[:start]
			after := rest[end+len("```"):]
			return before + fence + "\n" + newBlock + "```" + after, nil
		}
	}

	// No existing block — append one.
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + "\n" + fence + "\n" + newBlock + "```\n", nil
}

// extractYAMLBlockFrom returns the content of the first ```yaml fence, or ""
// if none is found.
func extractYAMLBlockFrom(content string) string {
	const fence = "```yaml"
	if start := strings.Index(content, fence); start >= 0 {
		rest := content[start+len(fence):]
		if end := strings.Index(rest, "```"); end >= 0 {
			return rest[:end]
		}
	}
	return ""
}

// canaryValidationFlags returns warning strings for a question at index i.
func canaryValidationFlags(q workflows.CanaryQuestion, seen map[string]int) []string {
	var flags []string
	if len(q.ExpectedFacts) == 0 {
		flags = append(flags, "WARN: missing expected_facts")
	}
	if q.SourceKB == "" {
		flags = append(flags, "WARN: empty source_kb")
	}
	if seen[strings.ToLower(q.Q)] > 1 {
		flags = append(flags, "WARN: duplicate")
	}
	return flags
}

// parseCommaList splits a comma-separated string and trims whitespace.
func parseCommaList(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			result = append(result, p)
		}
	}
	return result
}

func init() {
	scheduleCanaryAddCmd.Flags().StringVar(&canaryQuestionFlag, "question", "", "Question text (required)")
	scheduleCanaryAddCmd.Flags().StringVar(&canaryExpectedFlag, "expected", "", "Comma-separated expected facts (required)")
	scheduleCanaryAddCmd.Flags().StringVar(&canarySourceFlag, "source", "", "Source KB shard ID (optional)")

	scheduleCanaryCmd.AddCommand(scheduleCanaryAddCmd)
	scheduleCanaryCmd.AddCommand(scheduleCanaryListCmd)

	scheduleCmd.AddCommand(scheduleCanaryCmd)
	scheduleCmd.AddCommand(scheduleSeedCanariesCmd)
}
