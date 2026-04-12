package workflows

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pmezard/go-difflib/difflib"
)

const (
	defaultFactcheckModel      = "gemini/gemini-2.0-flash"
	defaultJudgeModel          = "claude/claude-haiku-4-5"
	defaultJudgeArticlesPerRun = 5
)

type DriftScanConfig struct {
	Project             string `json:"project"`
	RepoPath            string `json:"repo_path"`
	FactcheckModel      string `json:"factcheck_model"`
	JudgeModel          string `json:"judge_model"`
	JudgeArticlesPerRun int    `json:"judge_articles_per_run"`
	GapsShard           string `json:"gaps_shard"`
	RunID               int    `json:"-"`
}

type DriftScanResult struct {
	ArticlesScanned int `json:"articles_scanned"`
	ClaimsChecked   int `json:"claims_checked"`
	ClaimsBroken    int `json:"claims_broken"`
	SemanticChecks  int `json:"semantic_checks"`
	SemanticIssues  int `json:"semantic_issues"`
	GapsLogged      int `json:"gaps_logged"`
}

type DriftScanRunOutput struct {
	DriftScanResult
	LastSemanticCheck map[string]string `json:"last_semantic_check,omitempty"`
}

type driftArticle struct {
	ID      string
	Title   string
	Content string
	Version int
}

type driftClaim struct {
	Kind        string
	Value       string
	Description string
}

type driftJudgeVerdict struct {
	Verdict string `json:"verdict"`
	Issues  []struct {
		Severity    string `json:"severity"`
		Description string `json:"description"`
	} `json:"issues"`
	GapsIdentified []string `json:"gaps_identified"`
}

type driftScanDeps interface {
	listOpenArticles(context.Context, string) ([]driftArticle, error)
	resolveGapsShard(context.Context, DriftScanConfig) (string, error)
	verifyClaim(context.Context, DriftScanConfig, driftClaim) (bool, error)
	appendGap(context.Context, string, string) error
	lastSemanticChecks(context.Context, string) (map[string]time.Time, error)
	articleDiff(context.Context, string) (string, error)
	judgeDiff(context.Context, DriftScanConfig, string) (driftJudgeVerdict, error)
	now() time.Time
}

type sqlDriftScanDeps struct {
	db *sql.DB
}

var (
	filePathClaimRegex = regexp.MustCompile(`(?im)(?:^|\s)(?:[-*]\s*)?file_path\s*[:=]\s*["'` + "`" + `]?([A-Za-z0-9_./\\-]+)["'` + "`" + `]?`)
	dbTableClaimRegex  = regexp.MustCompile(`(?im)(?:^|\s)(?:[-*]\s*)?db_table\s*[:=]\s*["'` + "`" + `]?([A-Za-z0-9_.-]+)["'` + "`" + `]?`)
	dbColumnClaimRegex = regexp.MustCompile(`(?im)(?:^|\s)(?:[-*]\s*)?db_column\s*[:=]\s*["'` + "`" + `]?([A-Za-z0-9_.-]+)["'` + "`" + `]?`)
	migrationClaimRGX  = regexp.MustCompile(`(?im)(?:^|\s)(?:[-*]\s*)?migration_number\s*[:=]\s*["'` + "`" + `]?([0-9]+)["'` + "`" + `]?`)
)

func RunDriftScan(ctx context.Context, db *sql.DB, cfg DriftScanConfig) (DriftScanResult, error) {
	output, err := RunDriftScanDetailed(ctx, db, cfg)
	if err != nil {
		return DriftScanResult{}, err
	}
	return output.DriftScanResult, nil
}

func RunDriftScanDetailed(ctx context.Context, db *sql.DB, cfg DriftScanConfig) (DriftScanRunOutput, error) {
	return runDriftScan(ctx, sqlDriftScanDeps{db: db}, cfg)
}

func runDriftScan(ctx context.Context, deps driftScanDeps, cfg DriftScanConfig) (DriftScanRunOutput, error) {
	cfg = cfg.withDefaults()
	if strings.TrimSpace(cfg.Project) == "" {
		return DriftScanRunOutput{}, fmt.Errorf("project is required")
	}
	if strings.TrimSpace(cfg.RepoPath) == "" {
		return DriftScanRunOutput{}, fmt.Errorf("repo_path is required")
	}

	articles, err := deps.listOpenArticles(ctx, cfg.Project)
	if err != nil {
		return DriftScanRunOutput{}, err
	}

	gapsShard, err := deps.resolveGapsShard(ctx, cfg)
	if err != nil {
		return DriftScanRunOutput{}, err
	}

	history, err := deps.lastSemanticChecks(ctx, cfg.Project)
	if err != nil {
		return DriftScanRunOutput{}, err
	}

	result := DriftScanRunOutput{
		DriftScanResult: DriftScanResult{
			ArticlesScanned: len(articles),
		},
		LastSemanticCheck: map[string]string{},
	}

	runToken := cfg.runToken(deps.now())
	runDate := deps.now().UTC().Format("2006-01-02")

	for _, article := range articles {
		claims := extractDriftClaims(article.Content)
		for _, claim := range claims {
			result.ClaimsChecked++
			ok, verifyErr := deps.verifyClaim(ctx, cfg, claim)
			if verifyErr != nil {
				return DriftScanRunOutput{}, verifyErr
			}
			if ok {
				continue
			}

			result.ClaimsBroken++
			result.GapsLogged++
			line := fmt.Sprintf("%s | %s | drift-detected | %s (%s)", runDate, runToken, claim.Description, article.ID)
			if err := deps.appendGap(ctx, gapsShard, line); err != nil {
				return DriftScanRunOutput{}, err
			}
		}
	}

	for _, article := range selectSemanticArticles(articles, history, cfg.JudgeArticlesPerRun) {
		result.SemanticChecks++

		diffText, diffErr := deps.articleDiff(ctx, article.ID)
		if diffErr != nil {
			return DriftScanRunOutput{}, diffErr
		}
		result.LastSemanticCheck[article.ID] = deps.now().UTC().Format(time.RFC3339)
		if !isNonTrivialDiff(diffText) {
			continue
		}

		verdict, judgeErr := deps.judgeDiff(ctx, cfg, diffText)
		if judgeErr != nil {
			continue
		}

		for _, issue := range verdict.Issues {
			if strings.TrimSpace(issue.Description) == "" {
				continue
			}
			result.SemanticIssues++
			result.GapsLogged++
			line := fmt.Sprintf("%s | %s | drift-detected | semantic issue for %s: %s", runDate, runToken, article.ID, issue.Description)
			if err := deps.appendGap(ctx, gapsShard, line); err != nil {
				return DriftScanRunOutput{}, err
			}
		}
		for _, gap := range verdict.GapsIdentified {
			if strings.TrimSpace(gap) == "" {
				continue
			}
			result.SemanticIssues++
			result.GapsLogged++
			line := fmt.Sprintf("%s | %s | drift-detected | semantic gap for %s: %s", runDate, runToken, article.ID, gap)
			if err := deps.appendGap(ctx, gapsShard, line); err != nil {
				return DriftScanRunOutput{}, err
			}
		}
	}

	return result, nil
}

func (cfg DriftScanConfig) withDefaults() DriftScanConfig {
	cfg.Project = strings.TrimSpace(cfg.Project)
	cfg.RepoPath = strings.TrimSpace(cfg.RepoPath)
	cfg.GapsShard = strings.TrimSpace(cfg.GapsShard)
	cfg.FactcheckModel = strings.TrimSpace(cfg.FactcheckModel)
	cfg.JudgeModel = strings.TrimSpace(cfg.JudgeModel)
	if cfg.FactcheckModel == "" {
		cfg.FactcheckModel = defaultFactcheckModel
	}
	if cfg.JudgeModel == "" {
		cfg.JudgeModel = defaultJudgeModel
	}
	if cfg.JudgeArticlesPerRun <= 0 {
		cfg.JudgeArticlesPerRun = defaultJudgeArticlesPerRun
	}
	return cfg
}

func (cfg DriftScanConfig) runToken(now time.Time) string {
	if cfg.RunID > 0 {
		return fmt.Sprintf("drift-scan-%d", cfg.RunID)
	}
	return "drift-scan-" + now.UTC().Format("20060102150405")
}

func extractDriftClaims(content string) []driftClaim {
	var claims []driftClaim
	seen := map[string]bool{}

	appendClaims := func(kind string, rgx *regexp.Regexp, description func(string) string) {
		for _, match := range rgx.FindAllStringSubmatch(content, -1) {
			value := strings.TrimSpace(match[1])
			if value == "" {
				continue
			}
			key := kind + ":" + value
			if seen[key] {
				continue
			}
			seen[key] = true
			claims = append(claims, driftClaim{
				Kind:        kind,
				Value:       value,
				Description: description(value),
			})
		}
	}

	appendClaims("file_path", filePathClaimRegex, func(value string) string {
		return fmt.Sprintf("Missing file path claim %s", value)
	})
	appendClaims("db_table", dbTableClaimRegex, func(value string) string {
		return fmt.Sprintf("Missing database table claim %s", value)
	})
	appendClaims("db_column", dbColumnClaimRegex, func(value string) string {
		return fmt.Sprintf("Missing database column claim %s", value)
	})
	appendClaims("migration_number", migrationClaimRGX, func(value string) string {
		return fmt.Sprintf("Missing migration claim %s", value)
	})

	return claims
}

func selectSemanticArticles(articles []driftArticle, history map[string]time.Time, limit int) []driftArticle {
	if limit <= 0 || len(articles) == 0 {
		return nil
	}
	copied := append([]driftArticle(nil), articles...)
	sort.SliceStable(copied, func(i, j int) bool {
		left := history[copied[i].ID]
		right := history[copied[j].ID]
		if left.Equal(right) {
			return copied[i].ID < copied[j].ID
		}
		if left.IsZero() {
			return true
		}
		if right.IsZero() {
			return false
		}
		return left.Before(right)
	})
	if limit > len(copied) {
		limit = len(copied)
	}
	return copied[:limit]
}

func isNonTrivialDiff(diffText string) bool {
	for _, line := range strings.Split(diffText, "\n") {
		if line == "" || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "@@") {
			continue
		}
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			if strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "+"), "-")) != "" {
				return true
			}
		}
	}
	return false
}

func (d sqlDriftScanDeps) now() time.Time {
	return time.Now().UTC()
}

func (d sqlDriftScanDeps) listOpenArticles(ctx context.Context, project string) ([]driftArticle, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, title, COALESCE(content, ''), COALESCE((metadata->>'version')::int, 1)
		FROM shards
		WHERE project = $1 AND type = 'knowledge' AND status = 'open'
		ORDER BY updated_at DESC, id ASC
	`, project)
	if err != nil {
		return nil, fmt.Errorf("list open knowledge articles: %w", err)
	}
	defer rows.Close()

	var articles []driftArticle
	for rows.Next() {
		var article driftArticle
		if err := rows.Scan(&article.ID, &article.Title, &article.Content, &article.Version); err != nil {
			return nil, fmt.Errorf("scan open knowledge article: %w", err)
		}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate open knowledge articles: %w", err)
	}
	return articles, nil
}

func (d sqlDriftScanDeps) resolveGapsShard(ctx context.Context, cfg DriftScanConfig) (string, error) {
	candidate := cfg.GapsShard
	if candidate == "" {
		candidate = cfg.Project + "-kb-gaps"
	}

	var found string
	err := d.db.QueryRowContext(ctx, `
		SELECT id
		FROM shards
		WHERE project = $1 AND id = $2
	`, cfg.Project, candidate).Scan(&found)
	if err == nil {
		return found, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("resolve gaps shard: %w", err)
	}
	return "", fmt.Errorf("gaps shard not found: %s", candidate)
}

func (d sqlDriftScanDeps) verifyClaim(ctx context.Context, cfg DriftScanConfig, claim driftClaim) (bool, error) {
	switch claim.Kind {
	case "file_path":
		return verifyFileClaim(ctx, cfg.RepoPath, claim.Value)
	case "db_table":
		return d.verifyTableClaim(ctx, claim.Value)
	case "db_column":
		return d.verifyColumnClaim(ctx, claim.Value)
	case "migration_number":
		return verifyMigrationClaim(cfg.RepoPath, claim.Value)
	default:
		return true, nil
	}
}

func verifyFileClaim(ctx context.Context, repoPath, value string) (bool, error) {
	cleanPath := filepath.Clean(value)
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "ls-files", "--error-unmatch", "--", cleanPath)
	if err := cmd.Run(); err == nil {
		return true, nil
	}
	fullPath := filepath.Join(repoPath, cleanPath)
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("check file claim %s: %w", value, err)
}

func (d sqlDriftScanDeps) verifyTableClaim(ctx context.Context, value string) (bool, error) {
	schema, table := splitSchemaAndName(value)
	var exists bool
	err := d.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = $1 AND table_name = $2
		)
	`, schema, table).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check db_table claim %s: %w", value, err)
	}
	return exists, nil
}

func (d sqlDriftScanDeps) verifyColumnClaim(ctx context.Context, value string) (bool, error) {
	parts := strings.Split(value, ".")
	if len(parts) < 2 {
		return false, fmt.Errorf("db_column claim must be table.column or schema.table.column: %s", value)
	}

	schema := "public"
	table := parts[0]
	column := parts[1]
	if len(parts) >= 3 {
		schema = parts[0]
		table = parts[1]
		column = parts[2]
	}

	var exists bool
	err := d.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = $2 AND column_name = $3
		)
	`, schema, table, column).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check db_column claim %s: %w", value, err)
	}
	return exists, nil
}

func verifyMigrationClaim(repoPath, value string) (bool, error) {
	migrationsDir := filepath.Join(repoPath, "migrations")
	prefixes := []string{value}
	if parsed, err := strconv.Atoi(value); err == nil {
		prefixes = append(prefixes,
			fmt.Sprintf("%03d", parsed),
			fmt.Sprintf("%04d", parsed),
		)
	}
	for _, prefix := range prefixes {
		matches, err := filepath.Glob(filepath.Join(migrationsDir, prefix+"_*"))
		if err != nil {
			return false, fmt.Errorf("check migration claim %s: %w", value, err)
		}
		if len(matches) > 0 {
			return true, nil
		}
	}
	return false, nil
}

func splitSchemaAndName(value string) (string, string) {
	parts := strings.Split(value, ".")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "public", value
}

func (d sqlDriftScanDeps) appendGap(ctx context.Context, shardID, line string) error {
	var content string
	err := d.db.QueryRowContext(ctx, `SELECT COALESCE(content, '') FROM shards WHERE id = $1`, shardID).Scan(&content)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("gaps shard not found: %s", shardID)
		}
		return fmt.Errorf("load gaps shard: %w", err)
	}

	if strings.TrimSpace(content) == "" {
		content = line
	} else {
		content += "\n\n---\n\n" + line
	}

	if _, err := d.db.ExecContext(ctx, `
		UPDATE shards
		SET content = $2, updated_at = NOW()
		WHERE id = $1
	`, shardID, content); err != nil {
		return fmt.Errorf("append gap to shard %s: %w", shardID, err)
	}
	return nil
}

func (d sqlDriftScanDeps) lastSemanticChecks(ctx context.Context, project string) (map[string]time.Time, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT result
		FROM schedule_runs
		WHERE project = $1 AND workflow_type = 'drift-scan' AND status = 'completed'
		ORDER BY started_at ASC, id ASC
	`, project)
	if err != nil {
		return nil, fmt.Errorf("load drift scan history: %w", err)
	}
	defer rows.Close()

	history := map[string]time.Time{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan drift scan history: %w", err)
		}
		var payload DriftScanRunOutput
		if err := json.Unmarshal(raw, &payload); err != nil {
			continue
		}
		for articleID, ts := range payload.LastSemanticCheck {
			parsed, err := time.Parse(time.RFC3339, ts)
			if err != nil {
				continue
			}
			history[articleID] = parsed
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate drift scan history: %w", err)
	}
	return history, nil
}

func (d sqlDriftScanDeps) articleDiff(ctx context.Context, articleID string) (string, error) {
	var version int
	var currentContent string
	var project string
	err := d.db.QueryRowContext(ctx, `
		SELECT COALESCE((metadata->>'version')::int, 1), COALESCE(content, ''), project
		FROM shards
		WHERE id = $1
	`, articleID).Scan(&version, &currentContent, &project)
	if err != nil {
		return "", fmt.Errorf("load article %s for diff: %w", articleID, err)
	}
	if version <= 1 {
		return "", nil
	}

	var previousContent string
	err = d.db.QueryRowContext(ctx, `
		SELECT content
		FROM knowledge_version($1, $2, $3)
	`, articleID, version-1, project).Scan(&previousContent)
	if err != nil {
		return "", fmt.Errorf("load previous version for %s: %w", articleID, err)
	}

	return difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(previousContent),
		B:        difflib.SplitLines(currentContent),
		FromFile: fmt.Sprintf("%s v%d", articleID, version-1),
		ToFile:   fmt.Sprintf("%s v%d", articleID, version),
		Context:  3,
	})
}

func (d sqlDriftScanDeps) judgeDiff(ctx context.Context, cfg DriftScanConfig, diffText string) (driftJudgeVerdict, error) {
	args := []string{"--skill", "kb-judge", "--model", cfg.JudgeModel, "--input", diffText}
	cmd := exec.CommandContext(ctx, "claude", args...)
	output, err := cmd.Output()
	if err != nil {
		return driftJudgeVerdict{}, fmt.Errorf("run kb-judge: %w", err)
	}

	var verdict driftJudgeVerdict
	if err := json.Unmarshal(output, &verdict); err != nil {
		return driftJudgeVerdict{}, fmt.Errorf("parse kb-judge output: %w", err)
	}
	return verdict, nil
}
