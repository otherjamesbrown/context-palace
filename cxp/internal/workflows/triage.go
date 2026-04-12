package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var whitespacePattern = regexp.MustCompile(`\s+`)

var supportedTriageCategories = map[string]string{
	"hallucination":     "Suggest how the KB writer can get better source material or stronger evidence before publishing.",
	"omission":          "Identify what missing context should be added or expanded in the KB.",
	"drift-detected":    "Look for drift patterns and suggest whether a change class needs new triggers or maintenance checks.",
	"retrieval-failure": "Suggest retrieval tuning, article restructure, naming changes, or search improvements.",
	"coverage-hole":     "Propose a new KB article outline or a concrete expansion to close the gap.",
}

type TriageWorkflow struct {
	runner CommandRunner
	now    func() time.Time
}

type TriageConfig struct {
	GapsShard        string `json:"gaps_shard"`
	EscalationsShard string `json:"escalations_shard"`
	TriageModel      string `json:"triage_model"`
}

type TriageResult struct {
	GapsReviewed        int    `json:"gaps_reviewed"`
	NewGapsSinceLastRun int    `json:"new_gaps_since_last_run"`
	TasksCreated        int    `json:"tasks_created"`
	Escalations         int    `json:"escalations"`
	ReportShard         string `json:"report_shard"`
	LastTriageAt        string `json:"last_triage_at"`
}

type gapEntry struct {
	Date        time.Time
	DateText    string
	Source      string
	Category    string
	Description string
}

type triageProposal struct {
	Summary string           `json:"summary"`
	Actions []proposedAction `json:"actions"`
}

type proposedAction struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type createdTask struct {
	ID       string
	Category string
	Title    string
}

type recurringGap struct {
	Category     string
	Pattern      string
	Count        int
	Descriptions []string
}

func NewTriageWorkflow(runner CommandRunner, now func() time.Time) *TriageWorkflow {
	if runner == nil {
		runner = ExecRunner{}
	}
	if now == nil {
		now = time.Now
	}
	return &TriageWorkflow{runner: runner, now: now}
}

func (w *TriageWorkflow) Name() string {
	return WorkflowTypeTriage
}

func (w *TriageWorkflow) Run(ctx context.Context, req Request) (json.RawMessage, error) {
	cfg, err := parseTriageConfig(req.Config)
	if err != nil {
		return nil, err
	}

	lastRunAt, err := parseLastTriageAt(req.PreviousResult)
	if err != nil {
		return nil, err
	}

	allEntries, err := w.loadGapEntries(ctx, cfg.GapsShard)
	if err != nil {
		return nil, err
	}

	newEntries := filterEntriesSince(allEntries, lastRunAt)
	grouped := groupEntriesByCategory(newEntries)

	var tasks []createdTask
	groupSummaries := make(map[string]string, len(grouped))
	for _, category := range orderedGroupKeys(grouped) {
		entries := grouped[category]
		if len(entries) == 0 {
			continue
		}

		proposal, err := w.proposeActions(ctx, cfg.TriageModel, category, entries)
		if err != nil {
			return nil, err
		}
		groupSummaries[category] = proposal.Summary

		for _, action := range proposal.Actions {
			taskID, err := w.createTask(ctx, action)
			if err != nil {
				return nil, err
			}
			tasks = append(tasks, createdTask{
				ID:       taskID,
				Category: category,
				Title:    action.Title,
			})
		}
	}

	recurring := detectRecurringGaps(allEntries)
	if len(recurring) > 0 {
		if err := w.appendEscalations(ctx, cfg.EscalationsShard, recurring); err != nil {
			return nil, err
		}
	}

	runAt := w.now().UTC()
	reportID, err := w.createReport(ctx, runAt, allEntries, newEntries, grouped, groupSummaries, tasks, recurring)
	if err != nil {
		return nil, err
	}

	result := TriageResult{
		GapsReviewed:        len(allEntries),
		NewGapsSinceLastRun: len(newEntries),
		TasksCreated:        len(tasks),
		Escalations:         len(recurring),
		ReportShard:         reportID,
		LastTriageAt:        runAt.Format(time.RFC3339),
	}

	return json.Marshal(result)
}

func parseTriageConfig(raw json.RawMessage) (TriageConfig, error) {
	var cfg TriageConfig
	if len(raw) == 0 {
		return cfg, fmt.Errorf("triage workflow requires config")
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parse triage config: %w", err)
	}
	if strings.TrimSpace(cfg.GapsShard) == "" {
		return cfg, fmt.Errorf("triage workflow requires gaps_shard")
	}
	if strings.TrimSpace(cfg.EscalationsShard) == "" {
		return cfg, fmt.Errorf("triage workflow requires escalations_shard")
	}
	if strings.TrimSpace(cfg.TriageModel) == "" {
		cfg.TriageModel = "gemini/gemini-2.5-pro"
	}
	return cfg, nil
}

func parseLastTriageAt(raw json.RawMessage) (*time.Time, error) {
	if len(raw) == 0 || string(raw) == "{}" {
		return nil, nil
	}

	var payload struct {
		LastTriageAt string `json:"last_triage_at"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse previous triage result: %w", err)
	}
	if strings.TrimSpace(payload.LastTriageAt) == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, payload.LastTriageAt)
	if err != nil {
		return nil, fmt.Errorf("parse previous last_triage_at %q: %w", payload.LastTriageAt, err)
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func (w *TriageWorkflow) loadGapEntries(ctx context.Context, shardID string) ([]gapEntry, error) {
	out, err := w.runner.Run(ctx, "cxp", "shard", "show", shardID, "-o", "json")
	if err != nil {
		return nil, err
	}

	var payload struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse gaps shard %s: %w", shardID, err)
	}

	return parseGapEntries(payload.Content)
}

func parseGapEntries(content string) ([]gapEntry, error) {
	lines := strings.Split(content, "\n")
	entries := make([]gapEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entry, ok, err := parseGapEntry(line)
		if err != nil {
			return nil, err
		}
		if ok {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func parseGapEntry(line string) (gapEntry, bool, error) {
	if strings.Count(line, "|") < 3 {
		return gapEntry{}, false, nil
	}

	parts := strings.SplitN(line, "|", 4)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) != 4 {
		return gapEntry{}, false, nil
	}

	dateValue, err := parseGapDate(parts[0])
	if err != nil {
		return gapEntry{}, false, fmt.Errorf("parse gap entry %q: %w", line, err)
	}

	category := strings.ToLower(parts[2])
	if _, ok := supportedTriageCategories[category]; !ok {
		return gapEntry{}, false, nil
	}

	return gapEntry{
		Date:        dateValue,
		DateText:    parts[0],
		Source:      parts[1],
		Category:    category,
		Description: parts[3],
	}, true, nil
}

func parseGapDate(raw string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported date format %q", raw)
}

func filterEntriesSince(entries []gapEntry, lastRunAt *time.Time) []gapEntry {
	if lastRunAt == nil {
		return append([]gapEntry(nil), entries...)
	}

	filtered := make([]gapEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Date.After(*lastRunAt) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func groupEntriesByCategory(entries []gapEntry) map[string][]gapEntry {
	grouped := make(map[string][]gapEntry)
	for _, entry := range entries {
		grouped[entry.Category] = append(grouped[entry.Category], entry)
	}
	return grouped
}

func orderedGroupKeys(grouped map[string][]gapEntry) []string {
	keys := make([]string, 0, len(grouped))
	for category, entries := range grouped {
		if len(entries) == 0 {
			continue
		}
		keys = append(keys, category)
	}
	sort.Strings(keys)
	return keys
}

func (w *TriageWorkflow) proposeActions(ctx context.Context, model, category string, entries []gapEntry) (triageProposal, error) {
	prompt := buildTriagePrompt(category, entries)
	out, err := w.runner.Run(ctx, "claude", "--model", model, "--print", prompt)
	if err != nil {
		return triageProposal{}, err
	}

	var proposal triageProposal
	if err := json.Unmarshal(extractJSONObject(out), &proposal); err != nil {
		return triageProposal{}, fmt.Errorf("parse triage proposal for %s: %w", category, err)
	}

	cleaned := make([]proposedAction, 0, len(proposal.Actions))
	for _, action := range proposal.Actions {
		action.Title = strings.TrimSpace(action.Title)
		action.Body = strings.TrimSpace(action.Body)
		if action.Title == "" || action.Body == "" {
			continue
		}
		cleaned = append(cleaned, action)
	}
	proposal.Actions = cleaned
	proposal.Summary = strings.TrimSpace(proposal.Summary)
	return proposal, nil
}

func buildTriagePrompt(category string, entries []gapEntry) string {
	var b strings.Builder
	b.WriteString("You are triaging accumulated KB gaps.\n")
	b.WriteString("Return strict JSON only with this shape:\n")
	b.WriteString("{\"summary\":\"short summary\",\"actions\":[{\"title\":\"...\",\"body\":\"...\"}]}\n")
	b.WriteString("Do not include markdown fences.\n")
	b.WriteString(fmt.Sprintf("Category: %s\n", category))
	b.WriteString(fmt.Sprintf("Guidance: %s\n", supportedTriageCategories[category]))
	b.WriteString("Entries:\n")
	for _, entry := range entries {
		b.WriteString(fmt.Sprintf("- %s | %s | %s\n", entry.DateText, entry.Source, entry.Description))
	}
	b.WriteString("Create concrete, actionable follow-up tasks. Prefer 0-3 actions.")
	return b.String()
}

func extractJSONObject(raw []byte) []byte {
	trimmed := strings.TrimSpace(string(raw))
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end >= start {
		return []byte(trimmed[start : end+1])
	}
	return []byte(trimmed)
}

func (w *TriageWorkflow) createTask(ctx context.Context, action proposedAction) (string, error) {
	out, err := w.runner.Run(ctx, "cxp", "task", "create", action.Title, "--body", action.Body, "-o", "json")
	if err != nil {
		return "", err
	}

	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parse created task response: %w", err)
	}
	if payload.ID == "" {
		return "", fmt.Errorf("created task response missing id")
	}
	return payload.ID, nil
}

func detectRecurringGaps(entries []gapEntry) []recurringGap {
	type aggregate struct {
		category     string
		pattern      string
		descriptions []string
	}

	aggregates := make(map[string]*aggregate)
	for _, entry := range entries {
		pattern := normalizeIssuePattern(entry.Description)
		if pattern == "" {
			continue
		}

		key := entry.Category + "|" + pattern
		current := aggregates[key]
		if current == nil {
			current = &aggregate{category: entry.Category, pattern: pattern}
			aggregates[key] = current
		}
		current.descriptions = append(current.descriptions, entry.Description)
	}

	recurring := make([]recurringGap, 0)
	for _, item := range aggregates {
		if len(item.descriptions) < 3 {
			continue
		}
		recurring = append(recurring, recurringGap{
			Category:     item.category,
			Pattern:      item.pattern,
			Count:        len(item.descriptions),
			Descriptions: dedupeStrings(item.descriptions),
		})
	}

	sort.Slice(recurring, func(i, j int) bool {
		if recurring[i].Count == recurring[j].Count {
			if recurring[i].Category == recurring[j].Category {
				return recurring[i].Pattern < recurring[j].Pattern
			}
			return recurring[i].Category < recurring[j].Category
		}
		return recurring[i].Count > recurring[j].Count
	})

	return recurring
}

func normalizeIssuePattern(description string) string {
	normalized := strings.ToLower(strings.TrimSpace(description))
	normalized = whitespacePattern.ReplaceAllString(normalized, " ")
	return normalized
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func (w *TriageWorkflow) appendEscalations(ctx context.Context, shardID string, recurring []recurringGap) error {
	body := buildEscalationBody(w.now().UTC(), recurring)
	_, err := w.runner.Run(ctx, "cxp", "shard", "append", shardID, "--body", body, "-o", "json")
	return err
}

func buildEscalationBody(at time.Time, recurring []recurringGap) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n## Recurring KB gap escalation %s\n", at.Format("2006-01-02")))
	for _, item := range recurring {
		b.WriteString(fmt.Sprintf("- `%s` occurred %d times: %s\n", item.Category, item.Count, item.Pattern))
		if len(item.Descriptions) > 0 {
			b.WriteString(fmt.Sprintf("  Examples: %s\n", strings.Join(item.Descriptions, " | ")))
		}
	}
	return b.String()
}

func (w *TriageWorkflow) createReport(ctx context.Context, runAt time.Time, allEntries, newEntries []gapEntry, grouped map[string][]gapEntry, summaries map[string]string, tasks []createdTask, recurring []recurringGap) (string, error) {
	body := buildTriageReport(runAt, allEntries, newEntries, grouped, summaries, tasks, recurring)
	title := fmt.Sprintf("KB Triage Report %s", runAt.Format("2006-01-02"))
	out, err := w.runner.Run(ctx, "cxp", "knowledge", "create", title, "--doc-type", "reference", "--body", body, "-o", "json")
	if err != nil {
		return "", err
	}

	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parse triage report response: %w", err)
	}
	if payload.ID == "" {
		return "", fmt.Errorf("triage report response missing id")
	}
	return payload.ID, nil
}

func buildTriageReport(runAt time.Time, allEntries, newEntries []gapEntry, grouped map[string][]gapEntry, summaries map[string]string, tasks []createdTask, recurring []recurringGap) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# KB Triage Report %s\n\n", runAt.Format("2006-01-02")))
	b.WriteString(fmt.Sprintf("- Gaps reviewed: %d\n", len(allEntries)))
	b.WriteString(fmt.Sprintf("- New gaps since last run: %d\n", len(newEntries)))
	b.WriteString(fmt.Sprintf("- Tasks created: %d\n", len(tasks)))
	b.WriteString(fmt.Sprintf("- Recurring escalations: %d\n\n", len(recurring)))

	for _, category := range orderedGroupKeys(grouped) {
		entries := grouped[category]
		b.WriteString(fmt.Sprintf("## %s\n", category))
		b.WriteString(fmt.Sprintf("New entries: %d\n\n", len(entries)))
		if summary := strings.TrimSpace(summaries[category]); summary != "" {
			b.WriteString(summary + "\n\n")
		}
		for _, entry := range entries {
			b.WriteString(fmt.Sprintf("- %s | %s | %s\n", entry.DateText, entry.Source, entry.Description))
		}
		b.WriteString("\n")
	}

	if len(tasks) > 0 {
		b.WriteString("## Tasks created\n")
		for _, task := range tasks {
			b.WriteString(fmt.Sprintf("- %s | %s | %s\n", task.ID, task.Category, task.Title))
		}
		b.WriteString("\n")
	}

	if len(recurring) > 0 {
		b.WriteString("## Recurring issues\n")
		for _, item := range recurring {
			b.WriteString(fmt.Sprintf("- %s | %d occurrences | %s\n", item.Category, item.Count, item.Pattern))
		}
		b.WriteString("\n")
	}

	if len(newEntries) == 0 {
		b.WriteString("No new KB gaps were logged since the last triage run.\n")
	}

	return b.String()
}
