package trigger

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// TableInfo contains parsed markdown table information.
type TableInfo struct {
	Found       bool
	StartLine   int // Line number where table header is (0-indexed)
	EndLine     int // Line number where table ends
	WhenCol     int // Column index for "When" (0-indexed)
	CommandCol  int // Column index for "Command" (0-indexed)
}

// FindTable scans a CLAUDE.md file for a trigger table.
// Looks for table with BOTH "When" AND "Command" in header (case-insensitive).
func FindTable(path string) (*TableInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	info := &TableInfo{}

	for scanner.Scan() {
		line := scanner.Text()

		// Look for table header row: | ... | ... |
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			cols := parseTableRow(line)
			whenIdx := findColumnIndex(cols, "when")
			cmdIdx := findColumnIndex(cols, "command")

			if whenIdx >= 0 && cmdIdx >= 0 {
				info.Found = true
				info.StartLine = lineNum
				info.WhenCol = whenIdx
				info.CommandCol = cmdIdx

				// Find table end (skip separator, then count data rows)
				info.EndLine = lineNum
				for scanner.Scan() {
					lineNum++
					nextLine := strings.TrimSpace(scanner.Text())
					if strings.HasPrefix(nextLine, "|") {
						info.EndLine = lineNum
					} else {
						break
					}
				}
				return info, nil
			}
		}
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return info, nil
}

// parseTableRow splits a markdown table row into columns.
func parseTableRow(line string) []string {
	// Remove leading/trailing pipes and split
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	cols := make([]string, 0, len(parts))
	for _, p := range parts {
		cols = append(cols, strings.TrimSpace(p))
	}
	return cols
}

// findColumnIndex returns the index of a column containing the text (case-insensitive).
func findColumnIndex(cols []string, text string) int {
	text = strings.ToLower(text)
	for i, col := range cols {
		if strings.Contains(strings.ToLower(col), text) {
			return i
		}
	}
	return -1
}

// TriggerExists checks if a trigger for this category already exists.
func TriggerExists(path, category string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	// Look for `cxp memo <category>` in the file
	pattern := regexp.MustCompile(fmt.Sprintf("`cxp memo %s`", regexp.QuoteMeta(category)))
	return pattern.Match(data), nil
}

// AddTrigger appends a row to existing table or creates new section.
func AddTrigger(path, trigger, category string) error {
	// Check if file exists
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("CLAUDE.md not found")
	}
	if err != nil {
		return err
	}

	// Check for duplicate
	exists, err := TriggerExists(path, category)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("trigger for '%s' already exists in CLAUDE.md", category)
	}

	// Find existing table
	tableInfo, err := FindTable(path)
	if err != nil {
		return err
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")

	if tableInfo.Found {
		// Insert new row at end of table
		newRow := FormatRow(trigger, category)
		// Insert after EndLine
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:tableInfo.EndLine+1]...)
		newLines = append(newLines, newRow)
		newLines = append(newLines, lines[tableInfo.EndLine+1:]...)
		lines = newLines
	} else {
		// Append new section at end of file
		section := CreateSection(trigger, category)
		// Ensure there's a blank line before new section
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, section)
	}

	// Write back
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// FormatRow creates a markdown table row.
func FormatRow(trigger, category string) string {
	return fmt.Sprintf("| %s | `cxp memo %s` |", trigger, category)
}

// CreateSection creates the "## Context Memos" section with table.
func CreateSection(trigger, category string) string {
	return fmt.Sprintf(`## Context Memos

| When | Command |
|------|---------|
| %s | `+"`cxp memo %s`"+` |`, trigger, category)
}
