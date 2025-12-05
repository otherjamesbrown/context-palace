package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/otherjamesbrown/context-palace/internal/cxpdir"
	"github.com/otherjamesbrown/context-palace/internal/memo"
	"github.com/otherjamesbrown/context-palace/internal/output"
	"github.com/spf13/cobra"
)

var memosJSON bool

var memosCmd = &cobra.Command{
	Use:   "memos",
	Short: "List all memos (tree view)",
	Long: `List all memos in tree format.

Examples:
  cxp memos         # Show memo tree
  cxp memos --json  # Output as JSON`,
	RunE: runMemos,
}

func init() {
	memosCmd.Flags().BoolVar(&memosJSON, "json", false, "Output as JSON")
}

func runMemos(cmd *cobra.Command, args []string) error {
	// Find .cxp root
	root, err := cxpdir.FindRoot()
	if err != nil {
		if errors.Is(err, cxpdir.ErrNotInitialized) {
			output.PrintError(".cxp not initialized. Run 'cxp init' first.")
		}
		return err
	}

	// List all memos
	memoNames := listAllMemos(root)

	if len(memoNames) == 0 {
		fmt.Println("No memos found. Create one with 'cxp create <name>'")
		return nil
	}

	// Build tree structure
	tree := buildMemoTree(memoNames)

	if memosJSON {
		return output.Print(tree, true)
	}

	// Print tree
	printMemoTree(tree, "", true)
	return nil
}

// listAllMemos returns all memo names (dot notation) in the memos directory.
func listAllMemos(root string) []string {
	memosPath := filepath.Join(root, cxpdir.DirName, cxpdir.MemosDir)
	var memos []string

	filepath.Walk(memosPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".yaml") {
			return nil
		}

		// Convert path to dot notation
		relPath, err := filepath.Rel(memosPath, path)
		if err != nil {
			return nil
		}

		// Remove .yaml extension
		relPath = strings.TrimSuffix(relPath, ".yaml")

		// Convert path separators to dots
		name := strings.ReplaceAll(relPath, string(filepath.Separator), ".")

		memos = append(memos, name)
		return nil
	})

	sort.Strings(memos)
	return memos
}

// MemoTreeNode represents a node in the memo tree.
type MemoTreeNode struct {
	Name     string          `json:"name"`
	Children []*MemoTreeNode `json:"children,omitempty"`
}

// buildMemoTree creates a tree structure from flat memo names.
func buildMemoTree(names []string) []*MemoTreeNode {
	// Map to track nodes by full path
	nodeMap := make(map[string]*MemoTreeNode)
	var roots []*MemoTreeNode

	for _, name := range names {
		parts := strings.Split(name, ".")

		// Build path up the tree
		for i := range parts {
			fullPath := strings.Join(parts[:i+1], ".")

			if _, exists := nodeMap[fullPath]; !exists {
				node := &MemoTreeNode{Name: fullPath}
				nodeMap[fullPath] = node

				if i == 0 {
					// Root level
					roots = append(roots, node)
				} else {
					// Child level - add to parent
					parentPath := strings.Join(parts[:i], ".")
					if parent, ok := nodeMap[parentPath]; ok {
						parent.Children = append(parent.Children, node)
					}
				}
			}
		}
	}

	return roots
}

// printMemoTree prints the tree in a nice format.
func printMemoTree(nodes []*MemoTreeNode, prefix string, isLast bool) {
	for i, node := range nodes {
		isLastNode := i == len(nodes)-1

		// Get just the last part of the name for display
		displayName := node.Name
		if idx := strings.LastIndex(node.Name, "."); idx >= 0 {
			displayName = node.Name[idx+1:]
		}

		// Print connector
		connector := "├── "
		if isLastNode {
			connector = "└── "
		}

		fmt.Printf("%s%s%s\n", prefix, connector, displayName)

		// Calculate prefix for children
		childPrefix := prefix
		if isLastNode {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}

		// Recursively print children
		if len(node.Children) > 0 {
			printMemoTree(node.Children, childPrefix, isLastNode)
		}
	}
}

// getMemoMeta returns metadata for a memo (for JSON output).
func getMemoMeta(root, name string) *memo.MemoMeta {
	meta := &memo.MemoMeta{
		Name:     name,
		Children: []string{},
	}

	// Get parent
	if parent := cxpdir.GetParentCategory(name); parent != "" {
		meta.Parent = &parent
	}

	// Get children
	allMemos := listAllMemos(root)
	for _, m := range allMemos {
		if isDirectChild(name, m) {
			meta.Children = append(meta.Children, m)
		}
	}

	return meta
}
