package workflows

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/scheduler"
)

type TriageRunner struct{}

func (TriageRunner) Name() string { return "triage" }

type TriageConfig struct {
	GapsShard           string `json:"gaps_shard"`
	EscalationsShard    string `json:"escalations_shard"`
	EscalationThreshold int    `json:"escalation_threshold,omitempty"`
}

type TriageResult struct {
	GapsReviewed   int            `json:"gaps_reviewed"`
	GapsByCategory map[string]int `json:"gaps_by_category"`
	TasksCreated   []string       `json:"tasks_created,omitempty"`
	Escalations    []string       `json:"escalations,omitempty"`
	ReportShard    string         `json:"report_shard,omitempty"`
}

func (r TriageRunner) Run(ctx context.Context, configRaw json.RawMessage) (string, json.RawMessage, error) {
	var cfg TriageConfig
	if len(configRaw) > 0 {
		if err := json.Unmarshal(configRaw, &cfg); err != nil {
			return "", nil, fmt.Errorf("invalid triage config: %w", err)
		}
	}
	if cfg.GapsShard == "" || cfg.EscalationsShard == "" {
		return "", nil, fmt.Errorf("triage config requires gaps_shard and escalations_shard")
	}
	if cfg.EscalationThreshold == 0 {
		cfg.EscalationThreshold = 3
	}

	// 1. Read gap entries
	content, err := getShardContent(ctx, cfg.GapsShard)
	if err != nil {
		return "", nil, fmt.Errorf("read gaps: %w", err)
	}
	entries := parseGapEntries(content)

	result := TriageResult{
		GapsReviewed:   len(entries),
		GapsByCategory: map[string]int{},
	}

	// 2. Group by category
	byCategory := map[string][]GapEntry{}
	for _, e := range entries {
		byCategory[e.Category] = append(byCategory[e.Category], e)
		result.GapsByCategory[e.Category]++
	}

	// 3. Detect recurring patterns and propose actions
	for category, items := range byCategory {
		proposals := proposeActions(category, items, cfg.EscalationThreshold)
		for _, p := range proposals {
			switch p.Kind {
			case "task":
				if id, err := createTask(ctx, p.Title, p.Body); err == nil {
					result.TasksCreated = append(result.TasksCreated, id)
				}
			case "escalation":
				if err := appendEscalation(ctx, cfg.EscalationsShard, p.Body); err == nil {
					result.Escalations = append(result.Escalations, p.Title)
				}
			}
		}
	}

	// 4. Create a triage report shard
	if reportID, err := createReport(ctx, result); err == nil {
		result.ReportShard = reportID
	}

	summary := fmt.Sprintf("reviewed %d gaps, created %d tasks, escalated %d issues",
		result.GapsReviewed, len(result.TasksCreated), len(result.Escalations))

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return summary, nil, err
	}
	return summary, resultJSON, nil
}

func init() {
	scheduler.DefaultRegistry.Register(TriageRunner{})
}

// GapEntry represents a single parsed line from the gaps shard.
type GapEntry struct {
	Date        string
	Source      string
	Category    string
	Description string
	Raw         string
}

var gapLineRE = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})\s*\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|\s*(.+)$`)

func parseGapEntries(content string) []GapEntry {
	var entries []GapEntry
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := gapLineRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		entries = append(entries, GapEntry{
			Date:        m[1],
			Source:      strings.TrimSpace(m[2]),
			Category:    strings.TrimSpace(m[3]),
			Description: strings.TrimSpace(m[4]),
			Raw:         line,
		})
	}
	return entries
}

// Proposal is a proposed action resulting from triage analysis.
type Proposal struct {
	Kind  string // "task" | "escalation"
	Title string
	Body  string
}

func proposeActions(category string, items []GapEntry, escalationThreshold int) []Proposal {
	if len(items) == 0 {
		return nil
	}
	var proposals []Proposal

	// Per-category default action: open one summary task
	title := fmt.Sprintf("Triage: %d %s gap(s)", len(items), category)
	var body strings.Builder
	fmt.Fprintf(&body, "## %s gaps (%d entries)\n\n", category, len(items))
	body.WriteString(actionHintForCategory(category))
	body.WriteString("\n\n## Entries\n\n")
	for _, e := range items {
		fmt.Fprintf(&body, "- %s | %s | %s\n", e.Date, e.Source, e.Description)
	}
	proposals = append(proposals, Proposal{Kind: "task", Title: title, Body: body.String()})

	// Detect recurring identical descriptions → escalate
	counts := map[string]int{}
	for _, e := range items {
		counts[e.Description]++
	}
	for desc, count := range counts {
		if count >= escalationThreshold {
			proposals = append(proposals, Proposal{
				Kind:  "escalation",
				Title: fmt.Sprintf("Recurring %s: %s", category, desc),
				Body: fmt.Sprintf("**Category:** %s\n**Occurrences:** %d\n**Description:** %s\n\nLogged %d times without resolution. Requires human review.",
					category, count, desc, count),
			})
		}
	}
	return proposals
}

func actionHintForCategory(category string) string {
	switch category {
	case "drift-detected":
		return "**Suggested action:** review broken anchors. If a pattern emerges (same file, same module), the underlying code may have moved without kb-sync catching it."
	case "retrieval-failure":
		return "**Suggested action:** investigate whether the KB has the answer at all. If yes, the article needs better searchable terms in title/content. If no, write a new article."
	case "hallucination":
		return "**Suggested action:** repeated hallucinations in the same area suggest the writer agent needs better source material. Consider whether the affected articles need restructuring or whether the prompt needs adjustment."
	case "omission":
		return "**Suggested action:** the kb-sync writer is dropping content. Review the affected articles to confirm what should be preserved."
	case "coverage-hole":
		return "**Suggested action:** an agent couldn't find what it needed. Either the topic isn't covered (write a new article) or the existing coverage isn't discoverable (improve titles, triggers, or restructure)."
	default:
		return "**Suggested action:** review entries for patterns."
	}
}


func createTask(ctx context.Context, title, body string) (string, error) {
	cmd := exec.CommandContext(ctx, "cxp", "task", "create", title, "--body", body, "--output", "json")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func appendEscalation(ctx context.Context, escalationsShard, body string) error {
	line := fmt.Sprintf("\n## %s\n\n%s\n", time.Now().UTC().Format("2006-01-02"), body)
	cmd := exec.CommandContext(ctx, "cxp", "shard", "append", escalationsShard, "--body", line)
	return cmd.Run()
}

func createReport(ctx context.Context, result TriageResult) (string, error) {
	title := fmt.Sprintf("KB Triage Report %s", time.Now().UTC().Format("2006-01-02"))
	var body strings.Builder
	fmt.Fprintf(&body, "# %s\n\n", title)
	fmt.Fprintf(&body, "**Gaps reviewed:** %d\n\n", result.GapsReviewed)
	body.WriteString("## By category\n\n")
	cats := make([]string, 0, len(result.GapsByCategory))
	for c := range result.GapsByCategory {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	for _, c := range cats {
		fmt.Fprintf(&body, "- %s: %d\n", c, result.GapsByCategory[c])
	}
	fmt.Fprintf(&body, "\n## Tasks created\n\n")
	for _, id := range result.TasksCreated {
		fmt.Fprintf(&body, "- %s\n", id)
	}
	if len(result.Escalations) > 0 {
		body.WriteString("\n## Escalations\n\n")
		for _, e := range result.Escalations {
			fmt.Fprintf(&body, "- %s\n", e)
		}
	}

	cmd := exec.CommandContext(ctx, "cxp", "knowledge", "create",
		"--title", title, "--body", body.String(), "--output", "json")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}
