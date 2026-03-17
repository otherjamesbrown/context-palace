package pointer

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	SubMemoryStart = "<!-- sub-memories -->"
	SubMemoryEnd   = "<!-- /sub-memories -->"
)

// SubMemoryEntry represents a child pointer in the sub-memories block.
type SubMemoryEntry struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

// ParseSubMemories extracts the JSON block from memory content.
// Returns the main content (without the block) and the parsed entries.
func ParseSubMemories(content string) (mainContent string, entries []SubMemoryEntry, err error) {
	startIdx := strings.Index(content, SubMemoryStart)
	if startIdx == -1 {
		return content, nil, nil
	}
	endIdx := strings.Index(content, SubMemoryEnd)
	if endIdx == -1 {
		return content, nil, fmt.Errorf("found %s without closing %s", SubMemoryStart, SubMemoryEnd)
	}

	mainContent = strings.TrimRight(content[:startIdx], "\n")
	jsonBlock := content[startIdx+len(SubMemoryStart) : endIdx]
	jsonBlock = strings.TrimSpace(jsonBlock)

	if err := json.Unmarshal([]byte(jsonBlock), &entries); err != nil {
		return mainContent, nil, fmt.Errorf("invalid sub-memories JSON: %w", err)
	}
	return mainContent, entries, nil
}
