package client

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// FormatJSON marshals data as indented JSON
func FormatJSON(data interface{}) (string, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// FormatYAML marshals data as YAML
func FormatYAML(data interface{}) (string, error) {
	b, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Table formats data as an aligned text table
type Table struct {
	Headers []string
	Rows    [][]string
	MaxCols []int
}

// NewTable creates a new table with the given headers
func NewTable(headers ...string) *Table {
	maxCols := make([]int, len(headers))
	for i, h := range headers {
		maxCols[i] = len(h)
	}
	return &Table{
		Headers: headers,
		MaxCols: maxCols,
	}
}

// AddRow adds a row to the table
func (t *Table) AddRow(cells ...string) {
	// Pad if fewer cells than headers
	for len(cells) < len(t.Headers) {
		cells = append(cells, "")
	}
	for i, c := range cells {
		if i < len(t.MaxCols) && len(c) > t.MaxCols[i] {
			t.MaxCols[i] = len(c)
		}
	}
	t.Rows = append(t.Rows, cells)
}

// String renders the table as aligned text
func (t *Table) String() string {
	if len(t.Rows) == 0 {
		return ""
	}

	var sb strings.Builder

	// Header
	for i, h := range t.Headers {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(padRight(h, t.MaxCols[i]))
	}
	sb.WriteString("\n")

	// Rows
	for _, row := range t.Rows {
		for i, cell := range row {
			if i >= len(t.Headers) {
				break
			}
			if i > 0 {
				sb.WriteString("  ")
			}
			// Last column doesn't need padding
			if i == len(t.Headers)-1 {
				sb.WriteString(cell)
			} else {
				sb.WriteString(padRight(cell, t.MaxCols[i]))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// Truncate truncates a string to maxLen with "..." suffix
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// FormatOutput formats data according to the output format
func FormatOutput(data interface{}, format string) (string, error) {
	switch format {
	case "json":
		return FormatJSON(data)
	case "yaml":
		return FormatYAML(data)
	default:
		return fmt.Sprintf("%v", data), nil
	}
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
