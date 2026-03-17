package pointer

import (
	"encoding/json"
	"fmt"
)

// RenderWithBlock serializes entries as indented JSON and wraps in delimiters.
func RenderWithBlock(mainContent string, entries []SubMemoryEntry) (string, error) {
	jsonBytes, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize sub-memories: %w", err)
	}
	return fmt.Sprintf("%s\n\n%s\n%s\n%s\n",
		mainContent, SubMemoryStart, string(jsonBytes), SubMemoryEnd), nil
}
