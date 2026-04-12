package workflows

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultCanaryModel = "gemini/gemini-2.5-flash"
)

var defaultCanaryTools = []string{"cxp kb search", "cxp kb show"}

type CanaryConfig struct {
	Project     string   `json:"project"`
	CanaryShard string   `json:"canary_shard"`
	AgentModel  string   `json:"agent_model"`
	AgentTools  []string `json:"agent_tools"`
	GapsShard   string   `json:"gaps_shard"`
	RunID       int      `json:"-"`
}

type CanaryQuestion struct {
	Question      string   `yaml:"question"`
	ExpectedFacts []string `yaml:"expected_facts"`
}

type CanaryFailure struct {
	Question     string   `json:"question"`
	Expected     []string `json:"expected"`
	MissingFacts []string `json:"missing_facts"`
}

type CanaryResult struct {
	QuestionsTested int             `json:"questions_tested"`
	Passed          int             `json:"passed"`
	Failed          int             `json:"failed"`
	Failures        []CanaryFailure `json:"failures,omitempty"`
}

var (
	loadShardContentFunc   = loadShardContent
	appendShardContentFunc = appendShardContent
	runCanaryAgentFunc     = runCanaryAgent
	nowFunc                = time.Now
)

func RunCanary(ctx context.Context, db *sql.DB, cfg CanaryConfig) (CanaryResult, error) {
	cfg = normalizeCanaryConfig(cfg)
	if db == nil {
		return CanaryResult{}, errors.New("db is required")
	}
	if strings.TrimSpace(cfg.Project) == "" {
		return CanaryResult{}, errors.New("project is required")
	}
	if strings.TrimSpace(cfg.CanaryShard) == "" {
		return CanaryResult{}, errors.New("canary shard is required")
	}

	body, err := loadShardContentFunc(ctx, db, cfg.CanaryShard)
	if err != nil {
		return CanaryResult{}, err
	}

	questions, err := parseCanaryQuestions(body)
	if err != nil {
		return CanaryResult{}, fmt.Errorf("parse canary questions: %w", err)
	}

	result := CanaryResult{
		QuestionsTested: len(questions),
	}
	for _, question := range questions {
		response, err := runCanaryAgentFunc(ctx, cfg, question.Question)
		if err != nil {
			return result, fmt.Errorf("run canary agent for %q: %w", question.Question, err)
		}

		missing := missingFacts(response, question.ExpectedFacts)
		if len(missing) == 0 {
			result.Passed++
			continue
		}

		result.Failed++
		result.Failures = append(result.Failures, CanaryFailure{
			Question:     question.Question,
			Expected:     append([]string(nil), question.ExpectedFacts...),
			MissingFacts: missing,
		})

		if err := appendShardContentFunc(ctx, db, cfg.GapsShard, buildFailureLogLine(cfg, question, response)); err != nil {
			return result, fmt.Errorf("append canary failure to gaps shard: %w", err)
		}
	}

	return result, nil
}

func normalizeCanaryConfig(cfg CanaryConfig) CanaryConfig {
	if strings.TrimSpace(cfg.AgentModel) == "" {
		cfg.AgentModel = defaultCanaryModel
	}
	if len(cfg.AgentTools) == 0 {
		cfg.AgentTools = append([]string(nil), defaultCanaryTools...)
	}
	if strings.TrimSpace(cfg.GapsShard) == "" && strings.TrimSpace(cfg.Project) != "" {
		cfg.GapsShard = fmt.Sprintf("%s-kb-gaps", cfg.Project)
	}
	return cfg
}

func parseCanaryQuestions(body string) ([]CanaryQuestion, error) {
	if strings.TrimSpace(body) == "" {
		return []CanaryQuestion{}, nil
	}

	var questions []CanaryQuestion
	if err := yaml.Unmarshal([]byte(body), &questions); err != nil {
		return nil, err
	}
	return questions, nil
}

func missingFacts(response string, expectedFacts []string) []string {
	responseLower := strings.ToLower(response)
	var missing []string
	for _, fact := range expectedFacts {
		if !strings.Contains(responseLower, strings.ToLower(strings.TrimSpace(fact))) {
			missing = append(missing, fact)
		}
	}
	return missing
}

func buildFailureLogLine(cfg CanaryConfig, question CanaryQuestion, response string) string {
	date := nowFunc().UTC().Format("2006-01-02")
	runLabel := "canary"
	if cfg.RunID > 0 {
		runLabel = fmt.Sprintf("canary-%d", cfg.RunID)
	}
	return fmt.Sprintf(
		"%s | %s | retrieval-failure | Q: %q expected: %s got: %q",
		date,
		runLabel,
		question.Question,
		formatExpectedFacts(question.ExpectedFacts),
		summarizeResponse(response),
	)
}

func formatExpectedFacts(facts []string) string {
	if len(facts) == 0 {
		return "[]"
	}
	quoted := make([]string, 0, len(facts))
	for _, fact := range facts {
		quoted = append(quoted, fmt.Sprintf("%q", fact))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func summarizeResponse(response string) string {
	cleaned := strings.Join(strings.Fields(response), " ")
	const limit = 200
	if len(cleaned) <= limit {
		return cleaned
	}
	return cleaned[:limit-3] + "..."
}

func loadShardContent(ctx context.Context, db *sql.DB, shardID string) (string, error) {
	var content string
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(content, '') FROM shards WHERE id = $1`, shardID).Scan(&content); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("shard not found: %s", shardID)
		}
		return "", err
	}
	return content, nil
}

func appendShardContent(ctx context.Context, db *sql.DB, shardID, content string) error {
	result, err := db.ExecContext(ctx, `
		UPDATE shards
		SET content = CASE
			WHEN COALESCE(content, '') = '' THEN $2
			ELSE content || E'\n' || $2
		END,
		updated_at = NOW()
		WHERE id = $1
	`, shardID, content)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("shard not found: %s", shardID)
	}
	return nil
}

func runCanaryAgent(ctx context.Context, cfg CanaryConfig, question string) (string, error) {
	command, args, err := canaryCommand(cfg)
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = strings.NewReader(question)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s failed: %w: %s", command, err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func canaryCommand(cfg CanaryConfig) (string, []string, error) {
	model := strings.TrimSpace(cfg.AgentModel)
	if model == "" {
		model = defaultCanaryModel
	}
	tools := strings.Join(cfg.AgentTools, ",")

	if strings.HasPrefix(model, "gemini/") {
		command := "gemini"
		if _, err := exec.LookPath(command); err != nil {
			return "", nil, fmt.Errorf("gemini CLI not found for model %q", model)
		}
		args := []string{
			"--model", strings.TrimPrefix(model, "gemini/"),
			"--allowedTools", tools,
			"--print",
		}
		return command, args, nil
	}

	command := "claude"
	if _, err := exec.LookPath(command); err != nil {
		return "", nil, fmt.Errorf("claude CLI not found for model %q", model)
	}
	args := []string{
		"--model", model,
		"--allowedTools", tools,
		"--print",
	}
	return command, args, nil
}
