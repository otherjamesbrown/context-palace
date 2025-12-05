package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/otherjamesbrown/context-palace/internal/cxpdir"
	"github.com/otherjamesbrown/context-palace/internal/output"
	"github.com/otherjamesbrown/context-palace/internal/trigger"
	"github.com/spf13/cobra"
)

var addTriggerJSON bool

var addTriggerCmd = &cobra.Command{
	Use:   "add-trigger <memo> <description>",
	Short: "Add trigger to CLAUDE.md",
	Long: `Add a trigger entry to CLAUDE.md for a memo.

Examples:
  cxp add-trigger build "Build, test, or run commands"
  cxp add-trigger ci-cd "CI/CD or deployment changes"
  cxp add-trigger deploy "Deploying to production" --json`,
	Args: cobra.ExactArgs(2),
	RunE: runAddTrigger,
}

func init() {
	addTriggerCmd.Flags().BoolVar(&addTriggerJSON, "json", false, "Output as JSON")
}

type AddTriggerResult struct {
	Memo        string `json:"memo"`
	Trigger     string `json:"trigger"`
	ClaudeMD    string `json:"claude_md"`
	Added       bool   `json:"added"`
	TableExists bool   `json:"table_exists"`
}

func runAddTrigger(cmd *cobra.Command, args []string) error {
	category := args[0]
	triggerDesc := args[1]

	// Find .cxp root
	root, err := cxpdir.FindRoot()
	if err != nil {
		if errors.Is(err, cxpdir.ErrNotInitialized) {
			output.PrintError(".cxp not initialized. Run 'cxp init' first.")
		}
		return err
	}

	// Validate memo exists
	if !cxpdir.MemoExists(root, category) {
		output.PrintError("Memo '%s' not found. Create it first with 'cxp create %s'", category, category)
		return fmt.Errorf("memo '%s' not found", category)
	}

	// Load config to get CLAUDE.md path
	cfg, err := cxpdir.LoadConfig(root)
	if err != nil {
		output.PrintError("Could not load config: %v", err)
		return err
	}

	claudePath := filepath.Join(root, cfg.ClaudeMD)

	// Check if table already exists (for result reporting)
	tableInfo, _ := trigger.FindTable(claudePath)
	tableExists := tableInfo != nil && tableInfo.Found

	// Add the trigger
	if err := trigger.AddTrigger(claudePath, triggerDesc, category); err != nil {
		if strings.Contains(err.Error(), "not found") {
			output.PrintError("CLAUDE.md not found at %s", claudePath)
		} else if strings.Contains(err.Error(), "already exists") {
			output.PrintWarning("Trigger for '%s' already exists in CLAUDE.md", category)
			if addTriggerJSON {
				result := AddTriggerResult{
					Memo:        category,
					Trigger:     triggerDesc,
					ClaudeMD:    claudePath,
					Added:       false,
					TableExists: tableExists,
				}
				return output.Print(result, true)
			}
			return nil
		} else {
			output.PrintError("Could not add trigger: %v", err)
		}
		return err
	}

	result := AddTriggerResult{
		Memo:        category,
		Trigger:     triggerDesc,
		ClaudeMD:    claudePath,
		Added:       true,
		TableExists: tableExists,
	}

	if addTriggerJSON {
		return output.Print(result, true)
	}

	fmt.Printf("Added trigger to %s\n", cfg.ClaudeMD)
	return nil
}
