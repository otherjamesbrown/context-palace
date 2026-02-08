package pointer

// AppendSubMemory adds an entry to the pointer block.
// Creates the block if it doesn't exist.
func AppendSubMemory(content string, entry SubMemoryEntry) (string, error) {
	mainContent, entries, err := ParseSubMemories(content)
	if err != nil {
		return "", err
	}
	entries = append(entries, entry)
	return RenderWithBlock(mainContent, entries)
}

// RemoveSubMemory removes an entry by ID from the pointer block.
func RemoveSubMemory(content string, childID string) (string, error) {
	mainContent, entries, err := ParseSubMemories(content)
	if err != nil {
		return "", err
	}

	filtered := make([]SubMemoryEntry, 0, len(entries))
	for _, e := range entries {
		if e.ID != childID {
			filtered = append(filtered, e)
		}
	}

	if len(filtered) == 0 {
		return mainContent, nil
	}
	return RenderWithBlock(mainContent, filtered)
}

// ReplaceSubMemories replaces all entries in the pointer block.
// Used by sync to reconcile the block from actual children.
func ReplaceSubMemories(content string, entries []SubMemoryEntry) (string, error) {
	mainContent, _, err := ParseSubMemories(content)
	if err != nil {
		// If parse fails, use content as-is (it has no valid block)
		mainContent = content
	}

	if len(entries) == 0 {
		return mainContent, nil
	}
	return RenderWithBlock(mainContent, entries)
}
