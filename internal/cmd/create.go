package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/otherjamesbrown/context-palace/internal/cxpdir"
	"github.com/otherjamesbrown/context-palace/internal/logging"
	"github.com/otherjamesbrown/context-palace/internal/memo"
	"github.com/otherjamesbrown/context-palace/internal/output"
	"github.com/spf13/cobra"
)

var (
	createParent string
	createEdit   bool
	createJSON   bool
)

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create memo file (template)",
	Long: `Create a new memo file with a template structure.

Examples:
  cxp create build              # Create build.yaml
  cxp create docker --parent ci-cd  # Create ci-cd.docker.yaml
  cxp create deploy --edit      # Create and open in $EDITOR
  cxp create test --json        # Output result as JSON`,
	Args: cobra.ExactArgs(1),
	RunE: runCreate,
}

func init() {
	createCmd.Flags().StringVar(&createParent, "parent", "", "Parent memo (creates child memo)")
	createCmd.Flags().BoolVar(&createEdit, "edit", false, "Open in $EDITOR after creation")
	createCmd.Flags().BoolVar(&createJSON, "json", false, "Output as JSON")
}

type CreateResult struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Created bool   `json:"created"`
}

func runCreate(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Find .cxp root
	root, err := cxpdir.FindRoot()
	if err != nil {
		if errors.Is(err, cxpdir.ErrNotInitialized) {
			output.PrintError(".cxp not initialized. Run 'cxp init' first.")
		}
		return err
	}

	// Build full category name
	category := name
	if createParent != "" {
		// Validate parent exists
		if !cxpdir.MemoExists(root, createParent) {
			output.PrintError("Parent memo '%s' not found", createParent)
			return fmt.Errorf("parent memo '%s' not found", createParent)
		}
		category = createParent + "." + name
	}

	// Check if memo already exists
	memoPath := cxpdir.GetMemoPath(root, category)
	if _, err := os.Stat(memoPath); err == nil {
		output.PrintError("Memo '%s' already exists at %s", category, memoPath)
		return fmt.Errorf("memo '%s' already exists", category)
	}

	// Create template memo
	m := &memo.Memo{
		Content: map[string]interface{}{
			"summary": "TODO: Brief description of this memo",
			"rules":   []string{"TODO: Add rules here"},
		},
	}

	if createParent != "" {
		m.Parent = createParent
	}

	// Ensure parent directory exists
	dir := filepath.Dir(memoPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		output.PrintError("Cannot create directory: %v", err)
		return err
	}

	// Save memo
	if err := cxpdir.SaveMemo(memoPath, m); err != nil {
		output.PrintError("Cannot write memo: %v", err)
		return err
	}

	result := CreateResult{
		Name:    category,
		Path:    memoPath,
		Created: true,
	}

	// Log write
	cfg, _ := cxpdir.LoadConfig(root)
	if cfg != nil && cfg.Logging.Enabled {
		entry := logging.WriteEntry{
			Timestamp: time.Now(),
			Operation: "create",
			Memo:      category,
			Parent:    createParent,
		}
		if err := cxpdir.AppendLog(root, cfg.Logging.WritesLog, entry); err != nil {
			output.PrintWarning("Could not write log: %v", err)
		}
	}

	if createJSON {
		return output.Print(result, true)
	}

	fmt.Printf("Created %s\n", memoPath)

	// Open in editor if requested
	if createEdit {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		cmd := exec.Command(editor, memoPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			output.PrintWarning("Could not open editor: %v", err)
		}
	}

	return nil
}
