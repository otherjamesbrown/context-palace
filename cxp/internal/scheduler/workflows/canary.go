package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/scheduler"
	"gopkg.in/yaml.v3"
)

// CanaryRunner executes end-to-end retrieval tests against the knowledge base.
type CanaryRunner struct{}

func (CanaryRunner) Name() string { return "canary" }

// CanaryConfig holds the configuration for a canary workflow run.
type CanaryConfig struct {
	CanaryShard string `json:"canary_shard"`          // required: shard with question YAML
	GapsShard   string `json:"gaps_shard"`            // required: shard for retrieval-failure entries
	MaxResults  int    `json:"max_results,omitempty"` // optional: cap kb search results per question (default 5)
}

// CanaryQuestion represents a single canary test case.
type CanaryQuestion struct {
	Q             string   `yaml:"q"              json:"q"`
	ExpectedFacts []string `yaml:"expected_facts" json:"expected_facts"`
	SourceKB      string   `yaml:"source_kb"      json:"source_kb"`
}

// CanaryResult is the structured output of a canary run.
type CanaryResult struct {
	QuestionsTotal int             `json:"questions_total"`
	Passed         int             `json:"passed"`
	Failed         int             `json:"failed"`
	GapsLogged     int             `json:"gaps_logged"`
	Failures       []CanaryFailure `json:"failures,omitempty"`
}

// CanaryFailure records a single failed canary question.
type CanaryFailure struct {
	Question      string   `json:"question"`
	ExpectedFacts []string `json:"expected_facts"`
	MissingFacts  []string `json:"missing_facts"`
	SourceKB      string   `json:"source_kb,omitempty"`
}

func (r CanaryRunner) Run(ctx context.Context, configRaw json.RawMessage) (string, json.RawMessage, error) {
	var cfg CanaryConfig
	if len(configRaw) > 0 {
		if err := json.Unmarshal(configRaw, &cfg); err != nil {
			return "", nil, fmt.Errorf("invalid canary config: %w", err)
		}
	}
	if cfg.CanaryShard == "" || cfg.GapsShard == "" {
		return "", nil, fmt.Errorf("canary config requires canary_shard and gaps_shard")
	}
	if cfg.MaxResults == 0 {
		cfg.MaxResults = 5
	}

	questions, err := loadCanaryQuestions(ctx, cfg.CanaryShard)
	if err != nil {
		return "", nil, fmt.Errorf("load canaries: %w", err)
	}

	if len(questions) == 0 {
		summary := fmt.Sprintf("no questions seeded — run `cxp schedule seed-canaries --shard %s` to populate the canary question set", cfg.CanaryShard)
		result, _ := json.Marshal(CanaryResult{})
		return summary, result, nil
	}

	result := CanaryResult{QuestionsTotal: len(questions)}

	for _, q := range questions {
		response, err := runKBSearch(ctx, q.Q, cfg.MaxResults)
		if err != nil {
			response = "" // treat search error as retrieval failure
		}
		missing := findMissingFacts(response, q.ExpectedFacts)
		if len(missing) == 0 {
			result.Passed++
			continue
		}
		result.Failed++
		failure := CanaryFailure{
			Question:      q.Q,
			ExpectedFacts: q.ExpectedFacts,
			MissingFacts:  missing,
			SourceKB:      q.SourceKB,
		}
		result.Failures = append(result.Failures, failure)
		if err := appendCanaryGap(ctx, cfg.GapsShard, failure); err == nil {
			result.GapsLogged++
		}
	}

	summary := fmt.Sprintf("%d questions: %d passed, %d failed, %d gaps logged",
		result.QuestionsTotal, result.Passed, result.Failed, result.GapsLogged)

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return summary, nil, err
	}
	return summary, resultJSON, nil
}

// loadCanaryQuestions reads the canary shard body and parses YAML question entries.
func loadCanaryQuestions(ctx context.Context, shardID string) ([]CanaryQuestion, error) {
	content, err := getShardContent(ctx, shardID)
	if err != nil {
		return nil, err
	}
	yamlBlock := extractYAMLBlock(content)
	var questions []CanaryQuestion
	if err := yaml.Unmarshal([]byte(yamlBlock), &questions); err != nil {
		return nil, fmt.Errorf("parse canary YAML: %w", err)
	}
	valid := questions[:0]
	for _, q := range questions {
		if q.Q != "" && len(q.ExpectedFacts) > 0 {
			valid = append(valid, q)
		}
	}
	return valid, nil
}

// extractYAMLBlock extracts the content of a fenced ```yaml block, or returns
// the full content if no fenced block is found.
func extractYAMLBlock(content string) string {
	if start := strings.Index(content, "```yaml"); start >= 0 {
		rest := content[start+len("```yaml"):]
		if end := strings.Index(rest, "```"); end >= 0 {
			return rest[:end]
		}
	}
	return content
}

// runKBSearch invokes cxp kb search and concatenates snippet+title pairs into a blob.
func runKBSearch(ctx context.Context, query string, maxResults int) (string, error) {
	cmd := exec.CommandContext(ctx, "cxp", "kb", "search", query,
		"--limit", fmt.Sprintf("%d", maxResults),
		"--output", "json")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var results []struct {
		Snippet string `json:"snippet"`
		Title   string `json:"title"`
	}
	if err := json.Unmarshal(out, &results); err != nil {
		return string(out), nil
	}
	var sb strings.Builder
	for _, r := range results {
		sb.WriteString(r.Title)
		sb.WriteString("\n")
		sb.WriteString(r.Snippet)
		sb.WriteString("\n\n")
	}
	return sb.String(), nil
}

// findMissingFacts returns the expected facts not found in response (case-insensitive).
func findMissingFacts(response string, expected []string) []string {
	haystack := strings.ToLower(response)
	var missing []string
	for _, fact := range expected {
		if !strings.Contains(haystack, strings.ToLower(fact)) {
			missing = append(missing, fact)
		}
	}
	return missing
}

// appendCanaryGap appends a structured retrieval-failure line to the gaps shard.
func appendCanaryGap(ctx context.Context, gapsShard string, failure CanaryFailure) error {
	line := fmt.Sprintf("%s | canary | retrieval-failure | q=%q expected=%v missing=%v source=%s",
		time.Now().UTC().Format("2006-01-02"),
		failure.Question, failure.ExpectedFacts, failure.MissingFacts, failure.SourceKB)
	cmd := exec.CommandContext(ctx, "cxp", "shard", "append", gapsShard, "--body", line)
	return cmd.Run()
}

// LoadCanaryQuestionsFromShard is the exported wrapper for loading and parsing
// canary questions from a shard, used by the seed and canary subcommands.
func LoadCanaryQuestionsFromShard(ctx context.Context, shardID string) ([]CanaryQuestion, error) {
	content, err := getShardContent(ctx, shardID)
	if err != nil {
		return nil, err
	}
	yamlBlock := extractYAMLBlock(content)
	var questions []CanaryQuestion
	if err := yaml.Unmarshal([]byte(yamlBlock), &questions); err != nil {
		return nil, fmt.Errorf("parse canary YAML: %w", err)
	}
	return questions, nil
}

// MergeCanaryQuestions merges incoming questions into existing ones, deduplicating
// on question text (case-insensitive). Returns the merged slice with existing questions
// first, then any new questions from incoming.
func MergeCanaryQuestions(existing, incoming []CanaryQuestion) []CanaryQuestion {
	seen := make(map[string]bool, len(existing))
	for _, q := range existing {
		seen[strings.ToLower(strings.TrimSpace(q.Q))] = true
	}
	merged := make([]CanaryQuestion, len(existing))
	copy(merged, existing)
	for _, q := range incoming {
		key := strings.ToLower(strings.TrimSpace(q.Q))
		if !seen[key] {
			seen[key] = true
			merged = append(merged, q)
		}
	}
	return merged
}

// DefaultCanaryQuestions returns the built-in generic starter pack of canary questions.
// These cover common KB article shapes: playbook lookup, architecture, subsystem retrieval,
// config lookup, and operational runbooks. Customize expected_facts and source_kb for your project.
func DefaultCanaryQuestions() []CanaryQuestion {
	return []CanaryQuestion{
		{
			Q:             "What are the main knowledge domains or sections in this project's KB?",
			ExpectedFacts: []string{"playbook", "branch"},
		},
		{
			Q:             "How do I build or compile this project locally?",
			ExpectedFacts: []string{"build", "go build"},
		},
		{
			Q:             "Where are the tests located and how do I run them?",
			ExpectedFacts: []string{"test", "go test"},
		},
		{
			Q:             "What database does this project use and how is the connection configured?",
			ExpectedFacts: []string{"database", "config"},
		},
		{
			Q:             "What is the overall architecture of this system and its main components?",
			ExpectedFacts: []string{"architecture", "component"},
		},
		{
			Q:             "What are the core data entities or domain models in this system?",
			ExpectedFacts: []string{"entity", "model"},
		},
		{
			Q:             "How is the project deployed to production?",
			ExpectedFacts: []string{"deploy"},
		},
		{
			Q:             "What are the main integration points or external service dependencies?",
			ExpectedFacts: []string{"integration"},
		},
		{
			Q:             "Where does configuration live and how is it structured?",
			ExpectedFacts: []string{"config"},
		},
		{
			Q:             "How do I troubleshoot or debug common errors in this system?",
			ExpectedFacts: []string{"troubleshoot", "error"},
		},
		{
			Q:             "What is the status lifecycle for work items or tasks?",
			ExpectedFacts: []string{"status", "open"},
		},
		{
			Q:             "How are knowledge articles organized and navigated in this KB?",
			ExpectedFacts: []string{"knowledge", "playbook"},
		},
		{
			Q:             "What conventions or standards does this project follow for code or process?",
			ExpectedFacts: []string{"convention"},
		},
	}
}

// FormatCanaryQuestionsYAML serialises a slice of questions as a fenced YAML block
// suitable for embedding in a shard body.
func FormatCanaryQuestionsYAML(questions []CanaryQuestion) (string, error) {
	data, err := yaml.Marshal(questions)
	if err != nil {
		return "", fmt.Errorf("marshal canary questions: %w", err)
	}
	return "```yaml\n" + string(data) + "```\n", nil
}

func init() {
	scheduler.DefaultRegistry.Register(CanaryRunner{})
}
