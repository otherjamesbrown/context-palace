package summary

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/context-palace/cp/internal/pointer"
)

// SummaryResult holds the parsed AI response for summary generation.
type SummaryResult struct {
	Summary          string  `json:"summary"`
	ParentNeedsUpdate bool   `json:"parent_needs_update"`
	ParentEdits      *string `json:"parent_edits"`
}

// BuildSummaryPrompt creates the prompt for AI summary generation.
// Strips the sub-memories block from parent content before including it.
func BuildSummaryPrompt(parentID, parentContent, childTitle, childContent string) string {
	// Strip sub-memories block from parent content
	cleanParent, _, _ := pointer.ParseSubMemories(parentContent)

	return fmt.Sprintf(`You are writing a pointer summary for a hierarchical memory system used by AI agents.

PARENT MEMORY (ID: %s):
---
%s
---

NEW CHILD MEMORY (title: %s):
---
%s
---

Generate TWO things:

1. TRIGGER SUMMARY (max 120 chars):
   Write a one-line summary that tells the AI agent WHEN they would need to read
   this child memory. Focus on symptoms, situations, or questions — not just what
   the memory contains.

   Good: "If deploy succeeds but service unchanged, or allocations don't restart"
   Bad:  "Deployment troubleshooting information"

   Good: "When entity counts look wrong or junk entities appear"
   Bad:  "Entity quality issues and fixes"

2. PARENT REVIEW:
   Does the parent memory's prose need updating given this new child? If the parent
   says something that the child contradicts, clarifies, or significantly extends,
   suggest specific edits. If the parent is fine as-is, say "No changes needed."

   Example: If parent says "deployment is straightforward" but the child documents
   5 troubleshooting scenarios, suggest softening the parent text.

Respond as JSON:
{
  "summary": "trigger phrase here",
  "parent_needs_update": true/false,
  "parent_edits": "description of suggested edits, or null"
}`, parentID, cleanParent, childTitle, childContent)
}

// ParseSummaryResponse parses the AI-generated JSON response.
// Handles markdown code fences that models sometimes wrap around JSON.
func ParseSummaryResponse(response string) (*SummaryResult, error) {
	response = strings.TrimSpace(response)

	// Strip markdown code fences if present
	if strings.HasPrefix(response, "```") {
		lines := strings.Split(response, "\n")
		// Remove first line (```json or ```)
		if len(lines) > 2 {
			lines = lines[1:]
		}
		// Remove last line (```)
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			lines = lines[:len(lines)-1]
		}
		response = strings.Join(lines, "\n")
		response = strings.TrimSpace(response)
	}

	var result SummaryResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse AI response as JSON: %w\nRaw response: %s", err, response)
	}

	if result.Summary == "" {
		return nil, fmt.Errorf("AI returned empty summary")
	}

	return &result, nil
}
