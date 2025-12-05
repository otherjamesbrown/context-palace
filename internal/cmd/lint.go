package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/otherjamesbrown/context-palace/internal/cxpdir"
	"github.com/otherjamesbrown/context-palace/internal/lint"
	"github.com/otherjamesbrown/context-palace/internal/output"
	"github.com/spf13/cobra"
)

var lintJSON bool

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Validate structure and links",
	Long: `Validate .cxp structure, memo files, and links.

Checks:
  - All memo files are valid YAML
  - Parent references point to existing memos
  - source_doc links point to existing files
  - Memo size doesn't exceed configured limit

Examples:
  cxp lint         # Run validation
  cxp lint --json  # Output as JSON`,
	RunE: runLint,
}

func init() {
	lintCmd.Flags().BoolVar(&lintJSON, "json", false, "Output as JSON")
}

func runLint(cmd *cobra.Command, args []string) error {
	// Find .cxp root
	root, err := cxpdir.FindRoot()
	if err != nil {
		if errors.Is(err, cxpdir.ErrNotInitialized) {
			output.PrintError(".cxp not initialized. Run 'cxp init' first.")
		}
		return err
	}

	cfg, err := cxpdir.LoadConfig(root)
	if err != nil {
		output.PrintWarning("Could not load config, using defaults: %v", err)
	}

	result := lint.LintResult{
		Valid:    true,
		Errors:   []lint.LintError{},
		Warnings: []lint.LintWarning{},
	}

	// List all memos
	memoNames := listAllMemos(root)

	for _, name := range memoNames {
		path := cxpdir.GetMemoPath(root, name)

		// Try to load memo
		m, err := cxpdir.LoadMemo(path)
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, lint.LintError{
				Memo:     name,
				Error:    fmt.Sprintf("Invalid YAML: %v", err),
				Severity: "error",
			})
			continue
		}

		// Check parent reference
		if m.Parent != "" {
			if !cxpdir.MemoExists(root, m.Parent) {
				result.Valid = false
				result.Errors = append(result.Errors, lint.LintError{
					Memo:     name,
					Field:    "parent",
					Error:    fmt.Sprintf("Parent memo '%s' not found", m.Parent),
					Severity: "error",
				})
			}
		}

		// Check source_doc reference
		if m.SourceDoc != "" {
			docPath := filepath.Join(root, m.SourceDoc)
			if _, err := os.Stat(docPath); os.IsNotExist(err) {
				result.Valid = false
				result.Errors = append(result.Errors, lint.LintError{
					Memo:     name,
					Field:    "source_doc",
					Error:    fmt.Sprintf("Source document '%s' not found", m.SourceDoc),
					Severity: "error",
				})
			}
		}

		// Check memo size
		if cfg != nil && cfg.Limits.MemoLines > 0 {
			data, err := os.ReadFile(path)
			if err == nil {
				lines := strings.Count(string(data), "\n") + 1
				if lines > cfg.Limits.MemoLines {
					result.Warnings = append(result.Warnings, lint.LintWarning{
						Memo:    name,
						Warning: fmt.Sprintf("Memo has %d lines (limit: %d)", lines, cfg.Limits.MemoLines),
					})
				}
			}
		}

		// Check for orphaned child memos (parent in name but no parent field)
		if cxpdir.IsChildCategory(name) {
			expectedParent := cxpdir.GetParentCategory(name)
			if m.Parent == "" {
				result.Warnings = append(result.Warnings, lint.LintWarning{
					Memo:    name,
					Warning: fmt.Sprintf("Child memo has no parent field (expected: %s)", expectedParent),
				})
			} else if m.Parent != expectedParent {
				result.Warnings = append(result.Warnings, lint.LintWarning{
					Memo:    name,
					Warning: fmt.Sprintf("Parent field '%s' doesn't match path (expected: %s)", m.Parent, expectedParent),
				})
			}
		}
	}

	// Check CLAUDE.md exists
	if cfg != nil {
		claudePath := filepath.Join(root, cfg.ClaudeMD)
		if _, err := os.Stat(claudePath); os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, lint.LintWarning{
				Warning: fmt.Sprintf("CLAUDE.md not found at %s", cfg.ClaudeMD),
			})
		}
	}

	// Check config file exists
	configPath := filepath.Join(root, cxpdir.DirName, cxpdir.ConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		result.Warnings = append(result.Warnings, lint.LintWarning{
			Warning: "config.yaml not found",
		})
	}

	if lintJSON {
		return output.Print(result, true)
	}

	// Human-readable output
	if len(result.Errors) == 0 && len(result.Warnings) == 0 {
		fmt.Println("✓ All checks passed")
		return nil
	}

	if len(result.Errors) > 0 {
		fmt.Println("Errors:")
		for _, e := range result.Errors {
			if e.Field != "" {
				fmt.Printf("  ✗ %s [%s]: %s\n", e.Memo, e.Field, e.Error)
			} else {
				fmt.Printf("  ✗ %s: %s\n", e.Memo, e.Error)
			}
		}
	}

	if len(result.Warnings) > 0 {
		if len(result.Errors) > 0 {
			fmt.Println()
		}
		fmt.Println("Warnings:")
		for _, w := range result.Warnings {
			if w.Memo != "" {
				fmt.Printf("  ⚠ %s: %s\n", w.Memo, w.Warning)
			} else {
				fmt.Printf("  ⚠ %s\n", w.Warning)
			}
		}
	}

	if !result.Valid {
		return fmt.Errorf("validation failed")
	}

	return nil
}
