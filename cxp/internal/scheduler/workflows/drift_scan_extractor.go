package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/context-palace/cxp/internal/generation"
)

const claimExtractionPrompt = `You are a technical claim extractor for knowledge base articles. Analyze the article below and identify all factual technical claims that reference specific artifacts.

Output ONLY a JSON array. Each element must have:
- "type": one of "file_path", "function_name", "type_name", "db_table", "db_column", "config_key", "shard_id"
- "value": the exact artifact value referenced

Definitions:
- file_path: A file path in the codebase (e.g. "internal/scheduler/runner.go")
- function_name: A function or method name (e.g. "RunMigration")
- type_name: A struct, interface, or type alias name (e.g. "DriftScanConfig")
- db_table: A database table name (e.g. "schedules")
- db_column: A database column name (e.g. "workflow_type")
- config_key: A configuration key (e.g. "claim_extraction_model")
- shard_id: A Context Palace shard ID (e.g. "cp-48a2e7")

Article content:
%s`

var validClaimTypes = map[string]bool{
	"file_path":     true,
	"function_name": true,
	"type_name":     true,
	"db_table":      true,
	"db_column":     true,
	"config_key":    true,
	"shard_id":      true,
}

type llmClaim struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// newGeneratorFromModel creates a Generator from a "provider/model" string.
// Supported providers: "gemini", "google" (both map to Google Gemini).
func newGeneratorFromModel(modelStr string) (generation.Generator, error) {
	provider, model, err := parseModelString(modelStr)
	if err != nil {
		return nil, err
	}
	var genProvider, apiKeyEnv string
	switch provider {
	case "gemini", "google":
		genProvider = "google"
		apiKeyEnv = "GEMINI_API_KEY"
	case "claude", "anthropic":
		genProvider = "anthropic"
		apiKeyEnv = "ANTHROPIC_API_KEY"
	default:
		return nil, fmt.Errorf("unsupported model provider %q", provider)
	}
	return generation.NewGenerator(&generation.GenerationConfig{
		Provider:  genProvider,
		Model:     model,
		APIKeyEnv: apiKeyEnv,
	})
}

// parseModelString splits "provider/model" into its components.
func parseModelString(s string) (provider, model string, err error) {
	idx := strings.Index(s, "/")
	if idx <= 0 || idx == len(s)-1 {
		return "", "", fmt.Errorf("model %q must be in provider/model format (e.g. gemini/gemini-2.0-flash)", s)
	}
	return s[:idx], s[idx+1:], nil
}

// extractClaimsLLM sends article content to the LLM and returns structured claims.
// Claims are capped at maxClaims. Returns an error if the LLM call or JSON parse fails;
// callers should log the error and fall back to regex-only extraction.
func extractClaimsLLM(ctx context.Context, content string, gen generation.Generator, maxClaims int) ([]Anchor, error) {
	prompt := fmt.Sprintf(claimExtractionPrompt, content)
	resp, err := gen.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	jsonStr := extractJSONArray(resp)
	var claims []llmClaim
	if err := json.Unmarshal([]byte(jsonStr), &claims); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}

	if maxClaims > 0 && len(claims) > maxClaims {
		claims = claims[:maxClaims]
	}

	var anchors []Anchor
	for _, c := range claims {
		if validClaimTypes[c.Type] && c.Value != "" {
			anchors = append(anchors, Anchor{Type: c.Type, Value: c.Value})
		}
	}
	return anchors, nil
}

// extractJSONArray strips markdown code fences and extracts the first JSON array from text.
func extractJSONArray(s string) string {
	s = strings.TrimSpace(s)
	// Strip ```json ... ``` or ``` ... ``` fences
	if idx := strings.Index(s, "```json"); idx >= 0 {
		s = s[idx+7:]
		if end := strings.Index(s, "```"); end >= 0 {
			s = s[:end]
		}
	} else if idx := strings.Index(s, "```"); idx >= 0 {
		s = s[idx+3:]
		if end := strings.Index(s, "```"); end >= 0 {
			s = s[:end]
		}
	}
	s = strings.TrimSpace(s)
	// Find outermost JSON array bounds
	if start := strings.Index(s, "["); start >= 0 {
		if end := strings.LastIndex(s, "]"); end > start {
			return s[start : end+1]
		}
	}
	return s
}

// mergeAndDedup merges regex anchors and LLM claims into a single deduplicated list.
// Regex anchors appear first; LLM claims that duplicate an existing (type, value) are dropped.
func mergeAndDedup(regexAnchors, llmAnchors []Anchor) []Anchor {
	seen := make(map[string]bool, len(regexAnchors)+len(llmAnchors))
	result := make([]Anchor, 0, len(regexAnchors)+len(llmAnchors))
	for _, a := range append(regexAnchors, llmAnchors...) {
		key := a.Type + "\x00" + a.Value
		if !seen[key] {
			seen[key] = true
			result = append(result, a)
		}
	}
	return result
}
