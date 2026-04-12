package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"gopkg.in/yaml.v3"
)

const defaultCanaryModel = "gemini/gemini-2.5-flash"

var defaultCanaryTools = []string{"cxp kb search", "cxp kb show"}

// CanaryWorkflow executes the KB retrieval canary.
type CanaryWorkflow struct {
	runner CommandRunner
	gaps   GapAppender
	now    func() time.Time
}

// CanaryConfig is the per-schedule config for the canary workflow.
type CanaryConfig struct {
	CanaryShard string   `json:"canary_shard"`
	AgentModel  string   `json:"agent_model"`
	AgentTools  []string `json:"agent_tools"`
	GapsShard   string   `json:"gaps_shard"`
}

// CanaryQuestion is one test prompt and its required facts.
type CanaryQuestion struct {
	Question      string   `json:"question" yaml:"question"`
	ExpectedFacts []string `json:"expected_facts" yaml:"expected_facts"`
}

// CanaryFailure describes one failed canary.
type CanaryFailure struct {
	Question     string   `json:"question" yaml:"question"`
	Expected     []string `json:"expected" yaml:"expected"`
	MissingFacts []string `json:"missing_facts" yaml:"missing_facts"`
}

// CanaryResult is the structured workflow output.
type CanaryResult struct {
	QuestionsTested int             `json:"questions_tested" yaml:"questions_tested"`
	Passed          int             `json:"passed" yaml:"passed"`
	Failed          int             `json:"failed" yaml:"failed"`
	Failures        []CanaryFailure `json:"failures,omitempty" yaml:"failures,omitempty"`
}

type canaryShardPayload struct {
	Content string `json:"content"`
}

type canaryQuestionSet struct {
	Questions []CanaryQuestion `yaml:"questions"`
}

// NewCanaryWorkflow constructs the canary workflow.
func NewCanaryWorkflow(deps Dependencies) CanaryWorkflow {
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	runner := deps.Runner
	if runner == nil {
		runner = execRunner{}
	}
	return CanaryWorkflow{
		runner: runner,
		gaps:   deps.Gaps,
		now:    now,
	}
}

// Run executes the canary workflow for one schedule run.
func (w CanaryWorkflow) Run(ctx context.Context, schedule client.Schedule, run client.ScheduleRun) (any, error) {
	cfg, err := parseCanaryConfig(schedule)
	if err != nil {
		return nil, err
	}

	questions, err := w.loadQuestions(ctx, cfg.CanaryShard)
	if err != nil {
		return nil, err
	}

	result := CanaryResult{
		QuestionsTested: len(questions),
	}
	if len(questions) == 0 {
		return result, nil
	}

	gapsShard := cfg.GapsShard
	if gapsShard == "" {
		gapsShard = fmt.Sprintf("%s-kb-gaps", schedule.Project)
	}

	for _, question := range questions {
		response, err := w.askQuestion(ctx, cfg, question.Question)
		missing := missingFacts(response, question.ExpectedFacts)
		if err != nil && len(missing) == 0 {
			missing = append([]string(nil), question.ExpectedFacts...)
		}

		if len(missing) == 0 {
			result.Passed++
			continue
		}

		result.Failed++
		failure := CanaryFailure{
			Question:     question.Question,
			Expected:     append([]string(nil), question.ExpectedFacts...),
			MissingFacts: missing,
		}
		result.Failures = append(result.Failures, failure)

		if err := w.logFailure(ctx, gapsShard, run.ID, failure); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func parseCanaryConfig(schedule client.Schedule) (CanaryConfig, error) {
	var cfg CanaryConfig
	if len(schedule.Config) > 0 {
		if err := json.Unmarshal(schedule.Config, &cfg); err != nil {
			return CanaryConfig{}, fmt.Errorf("parse canary config: %w", err)
		}
	}
	if cfg.CanaryShard == "" {
		return CanaryConfig{}, fmt.Errorf("canary workflow requires config.canary_shard")
	}
	if strings.TrimSpace(cfg.AgentModel) == "" {
		cfg.AgentModel = defaultCanaryModel
	}
	if len(cfg.AgentTools) == 0 {
		cfg.AgentTools = append([]string(nil), defaultCanaryTools...)
	}
	return cfg, nil
}

func (w CanaryWorkflow) loadQuestions(ctx context.Context, shardID string) ([]CanaryQuestion, error) {
	output, err := w.runner.Run(ctx, "cxp", "--output", "json", "shard", "show", shardID)
	if err != nil {
		return nil, fmt.Errorf("load canary shard %s: %w", shardID, err)
	}

	var payload canaryShardPayload
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return nil, fmt.Errorf("parse canary shard %s: %w", shardID, err)
	}

	return parseCanaryQuestions(payload.Content)
}

func parseCanaryQuestions(content string) ([]CanaryQuestion, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, nil
	}
	if strings.HasPrefix(trimmed, "```") {
		trimmed = stripCodeFence(trimmed)
	}

	var questions canaryQuestionSet
	if err := yaml.Unmarshal([]byte(trimmed), &questions); err != nil {
		return nil, fmt.Errorf("parse canary questions yaml: %w", err)
	}
	return questions.Questions, nil
}

func stripCodeFence(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) >= 2 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		lines = lines[1:]
	}
	if len(lines) >= 1 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func (w CanaryWorkflow) askQuestion(ctx context.Context, cfg CanaryConfig, question string) (string, error) {
	return w.runner.Run(
		ctx,
		"claude",
		"--model", cfg.AgentModel,
		"--allowedTools", strings.Join(cfg.AgentTools, ","),
		"--print", question,
	)
}

func missingFacts(response string, expected []string) []string {
	responseLower := strings.ToLower(response)
	var missing []string
	for _, fact := range expected {
		if !strings.Contains(responseLower, strings.ToLower(fact)) {
			missing = append(missing, fact)
		}
	}
	return missing
}

func (w CanaryWorkflow) logFailure(ctx context.Context, gapsShard string, runID int, failure CanaryFailure) error {
	if w.gaps == nil {
		return fmt.Errorf("canary workflow requires a gap appender")
	}

	entry := fmt.Sprintf(
		`%s | canary-%d | retrieval-failure | Q: %q expected: %s missing: %s`,
		w.now().UTC().Format("2006-01-02"),
		runID,
		failure.Question,
		formatFactList(failure.Expected),
		formatFactList(failure.MissingFacts),
	)
	return w.gaps.AppendShardContent(ctx, gapsShard, entry)
}

func formatFactList(facts []string) string {
	if len(facts) == 0 {
		return "[]"
	}
	return "[" + strings.Join(facts, ", ") + "]"
}
