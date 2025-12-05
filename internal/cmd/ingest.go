package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/internal/cxpdir"
	"github.com/otherjamesbrown/context-palace/internal/logging"
	"github.com/otherjamesbrown/context-palace/internal/memo"
	"github.com/otherjamesbrown/context-palace/internal/output"
	"github.com/otherjamesbrown/context-palace/internal/prompt"
	"github.com/otherjamesbrown/context-palace/internal/trigger"
	"github.com/spf13/cobra"
)

var (
	ingestCategory  string
	ingestWhat      string
	ingestCause     string
	ingestCorrect   string
	ingestTrigger   string
	ingestJSON      bool
	ingestDryRun    bool
	ingestNoConfirm bool
)

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Create memo from mistake (guided flow)",
	Long: `Guided workflow for creating memos from mistakes.

When an agent makes an error, run 'cxp ingest' to analyze the failure
and create a memo that prevents similar mistakes in the future.

Interactive mode (default):
  cxp ingest

Non-interactive mode (for agents):
  cxp ingest --category build \
    --what "Used wrong build path" \
    --cause "main.go at root not in cmd/" \
    --correct "Build with 'go build -o bin/cli .'" \
    --trigger "Building Go binaries" \
    --no-confirm`,
	RunE: runIngest,
}

func init() {
	ingestCmd.Flags().StringVar(&ingestCategory, "category", "", "Memo category (existing or new)")
	ingestCmd.Flags().StringVar(&ingestWhat, "what", "", "Class of mistake (goes to footguns)")
	ingestCmd.Flags().StringVar(&ingestCause, "cause", "", "Why it happens (goes to rules)")
	ingestCmd.Flags().StringVar(&ingestCorrect, "correct", "", "General rule to follow (goes to rules)")
	ingestCmd.Flags().StringVar(&ingestTrigger, "trigger", "", "CLAUDE.md trigger text (optional)")
	ingestCmd.Flags().BoolVar(&ingestJSON, "json", false, "Output as JSON")
	ingestCmd.Flags().BoolVar(&ingestDryRun, "dry-run", false, "Preview without writing")
	ingestCmd.Flags().BoolVar(&ingestNoConfirm, "no-confirm", false, "Skip confirmation")
}

// IngestInput holds the gathered input.
type IngestInput struct {
	Category string
	What     string
	Cause    string
	Correct  string
	Trigger  string
}

// IngestResult is the JSON output structure.
type IngestResult struct {
	Memo               string       `json:"memo"`
	Path               string       `json:"path"`
	Operation          string       `json:"operation"`
	Added              AddedContent `json:"added"`
	TriggerAdded       bool         `json:"trigger_added"`
	TriggerDescription string       `json:"trigger_description,omitempty"`
}

// AddedContent shows what was added to the memo.
type AddedContent struct {
	Rules    []string `json:"rules,omitempty"`
	Footguns []string `json:"footguns,omitempty"`
}

func runIngest(cmd *cobra.Command, args []string) error {
	// 1. Find .cxp root
	root, err := cxpdir.FindRoot()
	if err != nil {
		if errors.Is(err, cxpdir.ErrNotInitialized) {
			output.PrintError(".cxp not initialized. Run 'cxp init' first.")
		}
		return err
	}

	// 2. Gather input
	input, err := gatherInput(root)
	if err != nil {
		return err
	}

	// 3. Validate child memo parent exists
	if cxpdir.IsChildCategory(input.Category) {
		if !cxpdir.ParentExists(root, input.Category) {
			parent := cxpdir.GetParentCategory(input.Category)
			output.PrintError("Parent memo '%s' not found", parent)
			return fmt.Errorf("parent memo '%s' not found", parent)
		}
	}

	// 4. Load existing memo or create new
	memoPath := cxpdir.GetMemoPath(root, input.Category)
	existingMemo, err := cxpdir.LoadMemo(memoPath)
	isUpdate := err == nil

	// 5. Generate merged content
	result := mergeContent(existingMemo, input)
	result.Name = input.Category
	result.Path = memoPath

	// 6. Build result for output
	ingestResult := IngestResult{
		Memo:      input.Category,
		Path:      memoPath,
		Operation: "create",
		Added: AddedContent{
			Rules:    []string{input.Cause, input.Correct},
			Footguns: []string{input.What},
		},
		TriggerDescription: input.Trigger,
	}
	if isUpdate {
		ingestResult.Operation = "update"
	}

	// 7. Preview
	if !ingestNoConfirm || ingestDryRun {
		printPreview(result, input, isUpdate)
	}

	// 8. Dry run exits here
	if ingestDryRun {
		if ingestJSON {
			return output.Print(ingestResult, true)
		}
		return nil
	}

	// 9. Confirm (unless --no-confirm)
	if !ingestNoConfirm {
		reader := prompt.NewReader()
		confirmed, err := reader.Confirm("\nCreate/update this memo? [y/n] > ")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// 10. Write memo
	if err := cxpdir.SaveMemo(memoPath, result); err != nil {
		output.PrintError("Cannot write to %s: %v", memoPath, err)
		return err
	}
	fmt.Printf("✓ %s %s\n", operationVerb(isUpdate), memoPath)

	// 11. Update CLAUDE.md (if trigger provided)
	triggerAdded := false
	if input.Trigger != "" {
		cfg, err := cxpdir.LoadConfig(root)
		if err != nil {
			output.PrintWarning("Could not load config: %v", err)
		} else {
			claudePath := filepath.Join(root, cfg.ClaudeMD)
			if err := trigger.AddTrigger(claudePath, input.Trigger, input.Category); err != nil {
				// Check if it's a "not found" or "already exists" warning
				if strings.Contains(err.Error(), "not found") {
					output.PrintWarning("CLAUDE.md not found, skipping trigger")
				} else if strings.Contains(err.Error(), "already exists") {
					output.PrintWarning("Trigger for '%s' already exists in CLAUDE.md", input.Category)
				} else {
					output.PrintWarning("Could not update CLAUDE.md: %v", err)
				}
			} else {
				triggerAdded = true
				fmt.Printf("✓ Added trigger to %s\n", cfg.ClaudeMD)
			}
		}
	}
	ingestResult.TriggerAdded = triggerAdded

	// 12. Log write
	cfg, _ := cxpdir.LoadConfig(root)
	if cfg != nil && cfg.Logging.Enabled {
		entry := logging.WriteEntry{
			Timestamp: time.Now(),
			Operation: ingestResult.Operation,
			Memo:      input.Category,
			Parent:    cxpdir.GetParentCategory(input.Category),
		}
		if err := cxpdir.AppendLog(root, cfg.Logging.WritesLog, entry); err != nil {
			output.PrintWarning("Could not write log: %v", err)
		} else {
			fmt.Printf("✓ Logged to .cxp/%s\n", cfg.Logging.WritesLog)
		}
	}

	// 13. Output
	if ingestJSON {
		return output.Print(ingestResult, true)
	}

	fmt.Printf("\nDone. Run 'cxp memo %s' before similar tasks to see this context.\n", input.Category)
	return nil
}

func gatherInput(root string) (*IngestInput, error) {
	// Check if non-interactive mode
	hasContentFlags := ingestWhat != "" || ingestCause != "" || ingestCorrect != ""

	if ingestNoConfirm {
		// Validate all required flags for non-interactive
		missing := []string{}
		if ingestCategory == "" {
			missing = append(missing, "--category")
		}
		if ingestWhat == "" {
			missing = append(missing, "--what")
		}
		if ingestCause == "" {
			missing = append(missing, "--cause")
		}
		if ingestCorrect == "" {
			missing = append(missing, "--correct")
		}
		if len(missing) > 0 {
			output.PrintError("Missing required flags for non-interactive mode: %s", strings.Join(missing, ", "))
			return nil, fmt.Errorf("missing required flags")
		}
		return &IngestInput{
			Category: ingestCategory,
			What:     ingestWhat,
			Cause:    ingestCause,
			Correct:  ingestCorrect,
			Trigger:  ingestTrigger,
		}, nil
	}

	if hasContentFlags && ingestCategory != "" {
		// All flags provided, non-interactive but with confirmation
		return &IngestInput{
			Category: ingestCategory,
			What:     ingestWhat,
			Cause:    ingestCause,
			Correct:  ingestCorrect,
			Trigger:  ingestTrigger,
		}, nil
	}

	// Interactive mode
	existingCategories := listExistingCategories(root)
	promptInput, err := prompt.RunIngestPrompts(existingCategories)
	if err != nil {
		return nil, err
	}
	return &IngestInput{
		Category: promptInput.Category,
		What:     promptInput.What,
		Cause:    promptInput.Cause,
		Correct:  promptInput.Correct,
		Trigger:  promptInput.Trigger,
	}, nil
}

func listExistingCategories(root string) []string {
	memosPath := filepath.Join(root, cxpdir.DirName, cxpdir.MemosDir)
	entries, err := os.ReadDir(memosPath)
	if err != nil {
		return nil
	}

	categories := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".yaml") {
			categories = append(categories, strings.TrimSuffix(name, ".yaml"))
		}
	}
	return categories
}

func mergeContent(existing *memo.Memo, input *IngestInput) *memo.Memo {
	result := &memo.Memo{
		Content: make(map[string]interface{}),
	}

	if existing != nil {
		// Copy existing content
		for k, v := range existing.Content {
			result.Content[k] = v
		}
		result.Parent = existing.Parent
		result.SourceDoc = existing.SourceDoc
	}

	// Append to rules array
	rules := getStringSlice(result.Content, "rules")
	rules = append(rules, input.Cause, input.Correct)
	result.Content["rules"] = rules

	// Append to footguns array
	footguns := getStringSlice(result.Content, "footguns")
	footguns = append(footguns, input.What)
	result.Content["footguns"] = footguns

	return result
}

func getStringSlice(content map[string]interface{}, key string) []string {
	if v, ok := content[key]; ok {
		switch slice := v.(type) {
		case []interface{}:
			result := make([]string, 0, len(slice))
			for _, item := range slice {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		case []string:
			return slice
		}
	}
	return []string{}
}

func printPreview(m *memo.Memo, input *IngestInput, isUpdate bool) {
	fmt.Println("\n=== Preview ===")
	fmt.Println()
	fmt.Printf("Memo: %s\n", input.Category)
	fmt.Printf("File: %s\n", m.Path)
	if isUpdate {
		fmt.Println("Status: UPDATE (adding to existing memo)")
	} else {
		fmt.Println("Status: CREATE (new memo)")
	}
	fmt.Println()
	fmt.Println("New content to add:")
	fmt.Println()
	fmt.Println("  rules:")
	fmt.Printf("    - \"%s\"\n", input.Cause)
	fmt.Printf("    - \"%s\"\n", input.Correct)
	fmt.Println()
	fmt.Println("  footguns:")
	fmt.Printf("    - \"%s\"\n", input.What)

	if input.Trigger != "" {
		fmt.Println()
		fmt.Println("CLAUDE.md update:")
		fmt.Printf("  | %s | `cxp memo %s` |\n", input.Trigger, input.Category)
	}
}

func operationVerb(isUpdate bool) string {
	if isUpdate {
		return "Updated"
	}
	return "Created"
}
