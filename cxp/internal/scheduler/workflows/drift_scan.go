package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/generation"
	"github.com/otherjamesbrown/context-palace/cxp/internal/scheduler"
)

type DriftScanRunner struct{}

func (DriftScanRunner) Name() string { return "drift-scan" }

type DriftScanConfig struct {
	RepoPath             string `json:"repo_path"`
	GapsShard            string `json:"gaps_shard"`
	MaxArticles          int    `json:"max_articles,omitempty"`
	ClaimExtractionModel string `json:"claim_extraction_model,omitempty"` // default: "gemini/gemini-2.0-flash"
	MaxClaimsPerArticle  int    `json:"max_claims_per_article,omitempty"`  // default: 50
	ConfigKeyQuery       string `json:"config_key_query,omitempty"`        // SQL base for config_key verification; value appended as = '<value>'
	DBConnStr            string `json:"db_conn_str,omitempty"`             // libpq conn string for db_table/db_column/config_key

	// Layer 2 semantic judge (rolling check of N articles per run)
	JudgeModel          string `json:"judge_model,omitempty"`           // default: "claude/claude-haiku-4-5"
	JudgeArticlesPerRun int    `json:"judge_articles_per_run,omitempty"` // default: 5; set to -1 to disable
}

type DriftScanResult struct {
	ArticlesScanned        int            `json:"articles_scanned"`
	AnchorsChecked         int            `json:"anchors_checked"`
	AnchorsBroken          int            `json:"anchors_broken"`
	GapsLogged             int            `json:"gaps_logged"`
	BrokenAnchors          []BrokenAnchor `json:"broken_anchors,omitempty"`
	ClaimsExtractedByLLM   int            `json:"claims_extracted_by_llm"`
	ClaimsExtractedByRegex int            `json:"claims_extracted_by_regex"`

	SemanticChecksRun int              `json:"semantic_checks_run"`
	SemanticFindings  []SemanticFinding `json:"semantic_findings,omitempty"`
}

// SemanticFinding records a Layer 2 judge result for an article.
// Category is one of: "drift_interpretation", "drift_coverage", "drift_scope".
// OK results are not recorded in this list.
type SemanticFinding struct {
	ArticleID string `json:"article_id"`
	Category  string `json:"category"`
	Detail    string `json:"detail"`
}

type BrokenAnchor struct {
	ArticleID string `json:"article_id"`
	Type      string `json:"type"`
	Value     string `json:"value"`
	Reason    string `json:"reason"`
}

func (r DriftScanRunner) Run(ctx context.Context, configRaw json.RawMessage) (string, json.RawMessage, error) {
	var cfg DriftScanConfig
	if len(configRaw) > 0 {
		if err := json.Unmarshal(configRaw, &cfg); err != nil {
			return "", nil, fmt.Errorf("invalid drift_scan config: %w", err)
		}
	}
	if cfg.RepoPath == "" {
		return "", nil, fmt.Errorf("drift_scan config requires repo_path")
	}
	if cfg.GapsShard == "" {
		return "", nil, fmt.Errorf("drift_scan config requires gaps_shard")
	}
	if cfg.MaxClaimsPerArticle == 0 {
		cfg.MaxClaimsPerArticle = 50
	}
	if cfg.ClaimExtractionModel == "" {
		cfg.ClaimExtractionModel = "gemini/gemini-2.0-flash"
	}
	if cfg.JudgeModel == "" {
		cfg.JudgeModel = "claude/claude-haiku-4-5"
	}
	if cfg.JudgeArticlesPerRun == 0 {
		cfg.JudgeArticlesPerRun = 5
	}
	if err := generation.ValidateDifferentFamilies(cfg.ClaimExtractionModel, cfg.JudgeModel); err != nil {
		return "", nil, fmt.Errorf("drift_scan config: %w", err)
	}

	var gen generation.Generator
	if g, err := newGeneratorFromModel(cfg.ClaimExtractionModel); err != nil {
		log.Printf("drift-scan: LLM extractor unavailable (%v); falling back to regex only", err)
	} else {
		gen = g
	}

	articles, err := listKBArticles(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("list articles: %w", err)
	}
	if cfg.MaxArticles > 0 && len(articles) > cfg.MaxArticles {
		articles = articles[:cfg.MaxArticles]
	}

	result := DriftScanResult{ArticlesScanned: len(articles)}

	for _, articleID := range articles {
		content, err := getShardContent(ctx, articleID)
		if err != nil {
			continue
		}

		regexAnchors := extractAnchors(content)
		result.ClaimsExtractedByRegex += len(regexAnchors)

		var llmAnchors []Anchor
		if gen != nil {
			var llmErr error
			llmAnchors, llmErr = extractClaimsLLM(ctx, content, gen, cfg.MaxClaimsPerArticle)
			if llmErr != nil {
				log.Printf("drift-scan: LLM extraction failed for article %s: %v; continuing with regex", articleID, llmErr)
			}
		}
		result.ClaimsExtractedByLLM += len(llmAnchors)

		anchors := mergeAndDedup(regexAnchors, llmAnchors)
		result.AnchorsChecked += len(anchors)

		for _, anchor := range anchors {
			ok, reason := verifyAnchor(ctx, cfg, anchor)
			if !ok {
				result.AnchorsBroken++
				broken := BrokenAnchor{
					ArticleID: articleID,
					Type:      anchor.Type,
					Value:     anchor.Value,
					Reason:    reason,
				}
				result.BrokenAnchors = append(result.BrokenAnchors, broken)
				if err := appendGap(ctx, cfg.GapsShard, broken); err == nil {
					result.GapsLogged++
				}
			}
		}
	}

	if cfg.JudgeArticlesPerRun > 0 {
		runSemanticJudge(ctx, cfg, articles, &result)
	}

	summary := fmt.Sprintf("scanned %d articles, %d anchors broken, %d gaps logged (llm=%d regex=%d); %d semantic checks, %d findings",
		result.ArticlesScanned, result.AnchorsBroken, result.GapsLogged,
		result.ClaimsExtractedByLLM, result.ClaimsExtractedByRegex,
		result.SemanticChecksRun, len(result.SemanticFindings))

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return summary, nil, err
	}
	return summary, resultJSON, nil
}

// Anchor represents a verifiable claim extracted from article content.
type Anchor struct {
	Type  string // "file_path" | "function_name" | "type_name" | "db_table" | "db_column" | "config_key" | "shard_id"
	Value string
}

var (
	filePathRE = regexp.MustCompile("`([a-zA-Z0-9_\\-/]+\\.(go|sql|yaml|yml|md|json|sh))`")
	funcNameRE = regexp.MustCompile("`([A-Z][a-zA-Z0-9_]+)\\(\\)`")
)

// extractAnchors extracts file_path and function_name anchors via regex.
// LLM-based extraction for all 7 claim types is done separately.
func extractAnchors(content string) []Anchor {
	seen := map[string]bool{}
	var anchors []Anchor

	for _, m := range filePathRE.FindAllStringSubmatch(content, -1) {
		key := "file:" + m[1]
		if !seen[key] {
			seen[key] = true
			anchors = append(anchors, Anchor{Type: "file_path", Value: m[1]})
		}
	}
	for _, m := range funcNameRE.FindAllStringSubmatch(content, -1) {
		key := "func:" + m[1]
		if !seen[key] {
			seen[key] = true
			anchors = append(anchors, Anchor{Type: "function_name", Value: m[1]})
		}
	}
	return anchors
}

// verifyAnchor checks whether an anchor claim holds against the repo/DB/service.
// Returns (true, "") if valid or unverifiable (skip), (false, reason) if broken.
func verifyAnchor(ctx context.Context, cfg DriftScanConfig, anchor Anchor) (bool, string) {
	switch anchor.Type {
	case "file_path":
		cmd := exec.CommandContext(ctx, "git", "-C", cfg.RepoPath, "ls-files", "--error-unmatch", anchor.Value)
		if err := cmd.Run(); err != nil {
			return false, "file not tracked in repo"
		}
		return true, ""

	case "function_name":
		cmd := exec.CommandContext(ctx, "git", "-C", cfg.RepoPath, "grep", "-q", "-E",
			fmt.Sprintf("(func|fn|def)[[:space:]]+%s", regexp.QuoteMeta(anchor.Value)))
		if err := cmd.Run(); err != nil {
			return false, "function name not found in repo"
		}
		return true, ""

	case "type_name":
		cmd := exec.CommandContext(ctx, "git", "-C", cfg.RepoPath, "grep", "-q", "-E",
			fmt.Sprintf("type[[:space:]]+%s[[:space:]]+", regexp.QuoteMeta(anchor.Value)))
		if err := cmd.Run(); err != nil {
			return false, "type name not found in repo"
		}
		return true, ""

	case "db_table":
		if cfg.DBConnStr == "" {
			return true, "" // skip — not verifiable without DB connection
		}
		query := fmt.Sprintf(
			"SELECT 1 FROM information_schema.tables WHERE table_name = '%s' LIMIT 1",
			escapeSQLLiteral(anchor.Value),
		)
		return runPsqlCheck(ctx, cfg.DBConnStr, query,
			fmt.Sprintf("table %q not found in database", anchor.Value))

	case "db_column":
		if cfg.DBConnStr == "" {
			return true, ""
		}
		query := fmt.Sprintf(
			"SELECT 1 FROM information_schema.columns WHERE column_name = '%s' LIMIT 1",
			escapeSQLLiteral(anchor.Value),
		)
		return runPsqlCheck(ctx, cfg.DBConnStr, query,
			fmt.Sprintf("column %q not found in database", anchor.Value))

	case "config_key":
		if cfg.ConfigKeyQuery == "" {
			return true, "" // skip — not verifiable without a query
		}
		if cfg.DBConnStr == "" {
			return true, ""
		}
		query := cfg.ConfigKeyQuery + " = '" + escapeSQLLiteral(anchor.Value) + "'"
		return runPsqlCheck(ctx, cfg.DBConnStr, query,
			fmt.Sprintf("config key %q not found", anchor.Value))

	case "shard_id":
		cmd := exec.CommandContext(ctx, "cxp", "shard", "show", anchor.Value)
		if err := cmd.Run(); err != nil {
			return false, fmt.Sprintf("shard %q not found", anchor.Value)
		}
		return true, ""
	}
	return true, ""
}

// runPsqlCheck runs a SELECT query via psql and returns true if at least one row is returned.
func runPsqlCheck(ctx context.Context, connStr, query, failReason string) (bool, string) {
	cmd := exec.CommandContext(ctx, "psql", connStr, "--no-psqlrc", "-t", "-A", "-c", query)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Sprintf("db query error: %v", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return false, failReason
	}
	return true, ""
}

// escapeSQLLiteral escapes a value for safe embedding in a SQL string literal.
func escapeSQLLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func listKBArticles(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "cxp", "kb", "list", "--output", "json")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var items []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, err
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids, nil
}

func appendGap(ctx context.Context, gapsShard string, broken BrokenAnchor) error {
	line := fmt.Sprintf("%s | drift-scan | drift-detected | article=%s type=%s value=%s reason=%s",
		time.Now().UTC().Format("2006-01-02"),
		broken.ArticleID, broken.Type, broken.Value, broken.Reason)
	cmd := exec.CommandContext(ctx, "cxp", "shard", "append", gapsShard, "--body", line)
	return cmd.Run()
}

func init() {
	scheduler.DefaultRegistry.Register(DriftScanRunner{})
}
