package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/internal/cxpdir"
	"github.com/otherjamesbrown/context-palace/internal/logging"
	"github.com/otherjamesbrown/context-palace/internal/memo"
	"github.com/otherjamesbrown/context-palace/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	memoDepth string
	memoJSON  bool
)

var memoCmd = &cobra.Command{
	Use:   "memo <name>",
	Short: "Show memo content",
	Long: `Show the content of a memo by name.

Examples:
  cxp memo build            # Show build memo
  cxp memo ci-cd --depth 1  # Show ci-cd with immediate children
  cxp memo ci-cd --depth all # Show ci-cd with all descendants
  cxp memo build --json     # Output as JSON`,
	Args: cobra.ExactArgs(1),
	RunE: runMemo,
}

func init() {
	memoCmd.Flags().StringVar(&memoDepth, "depth", "0", "Include children: 0 (none), 1, 2, ... or 'all'")
	memoCmd.Flags().BoolVar(&memoJSON, "json", false, "Output as JSON")
}

func runMemo(cmd *cobra.Command, args []string) error {
	category := args[0]

	// Find .cxp root
	root, err := cxpdir.FindRoot()
	if err != nil {
		if errors.Is(err, cxpdir.ErrNotInitialized) {
			output.PrintError(".cxp not initialized. Run 'cxp init' first.")
		}
		return err
	}

	// Load the memo
	memoPath := cxpdir.GetMemoPath(root, category)
	m, err := cxpdir.LoadMemo(memoPath)
	if err != nil {
		output.PrintError("Memo '%s' not found", category)
		return err
	}
	m.Name = category
	m.Path = memoPath

	// Determine depth
	depth := parseDepth(memoDepth)

	// Load children if requested
	if depth != 0 {
		children, err := loadChildren(root, category, depth, 1)
		if err != nil {
			output.PrintWarning("Could not load children: %v", err)
		}
		for _, child := range children {
			m.Children = append(m.Children, child.Name)
		}
	}

	// Log access
	cfg, _ := cxpdir.LoadConfig(root)
	if cfg != nil && cfg.Logging.Enabled {
		entry := logging.AccessEntry{
			Timestamp: time.Now(),
			Memo:      category,
			Depth:     memoDepth,
		}
		if err := cxpdir.AppendLog(root, cfg.Logging.AccessLog, entry); err != nil {
			output.PrintWarning("Could not write access log: %v", err)
		}
	}

	// Output
	if memoJSON {
		return output.Print(m, true)
	}

	// Pretty print for human consumption
	printMemo(m, depth, root)
	return nil
}

func parseDepth(s string) int {
	if s == "all" {
		return -1 // -1 means unlimited
	}
	var d int
	fmt.Sscanf(s, "%d", &d)
	return d
}

func loadChildren(root, parent string, maxDepth, currentDepth int) ([]*memo.Memo, error) {
	if maxDepth != -1 && currentDepth > maxDepth {
		return nil, nil
	}

	children := findChildMemos(root, parent)
	var result []*memo.Memo

	for _, childName := range children {
		childPath := cxpdir.GetMemoPath(root, childName)
		child, err := cxpdir.LoadMemo(childPath)
		if err != nil {
			continue
		}
		child.Name = childName
		child.Path = childPath

		// Recursively load grandchildren
		grandchildren, _ := loadChildren(root, childName, maxDepth, currentDepth+1)
		for _, gc := range grandchildren {
			child.Children = append(child.Children, gc.Name)
		}

		result = append(result, child)
		result = append(result, grandchildren...)
	}

	return result, nil
}

func findChildMemos(root, parent string) []string {
	// List all memos and find ones that are direct children of parent
	allMemos := listAllMemos(root)
	var children []string

	for _, name := range allMemos {
		if isDirectChild(parent, name) {
			children = append(children, name)
		}
	}

	return children
}

func isDirectChild(parent, candidate string) bool {
	// "ci-cd.docker" is a direct child of "ci-cd"
	// "ci-cd.docker.build" is NOT a direct child of "ci-cd"
	if !strings.HasPrefix(candidate, parent+".") {
		return false
	}
	suffix := strings.TrimPrefix(candidate, parent+".")
	return !strings.Contains(suffix, ".")
}

func printMemo(m *memo.Memo, depth int, root string) {
	fmt.Printf("# %s\n", m.Name)
	if m.SourceDoc != "" {
		fmt.Printf("source: %s\n", m.SourceDoc)
	}
	fmt.Println()

	// Print content as YAML
	if len(m.Content) > 0 {
		data, err := yaml.Marshal(m.Content)
		if err == nil {
			fmt.Print(string(data))
		}
	}

	// Print children if we have depth
	if depth != 0 && len(m.Children) > 0 {
		fmt.Println()
		fmt.Println("---")
		for _, childName := range m.Children {
			childPath := cxpdir.GetMemoPath(root, childName)
			child, err := cxpdir.LoadMemo(childPath)
			if err != nil {
				continue
			}
			child.Name = childName
			fmt.Printf("\n## %s\n\n", childName)
			if len(child.Content) > 0 {
				data, _ := yaml.Marshal(child.Content)
				fmt.Print(string(data))
			}
		}
	}
}
