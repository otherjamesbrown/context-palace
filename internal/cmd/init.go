package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/otherjamesbrown/context-palace/internal/cxpdir"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize .cxp directory structure",
	Long: `Creates the .cxp/ directory structure in the current directory:
  .cxp/
  ├── config.yaml    (default configuration)
  ├── memos/         (memo storage)
  └── logs/          (access and write logs)

Also adds ContextPalace instructions to CLAUDE.md.

This command is idempotent - safe to run multiple times.`,
	RunE: runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}

	alreadyInit := cxpdir.Exists(cwd)

	if alreadyInit {
		fmt.Println("ContextPalace already initialized in this directory")
	} else {
		if err := cxpdir.Initialize(cwd); err != nil {
			return fmt.Errorf("failed to initialize: %w", err)
		}
		fmt.Println("Initialized ContextPalace in .cxp/")
	}

	// Update CLAUDE.md
	if err := ensureClaudeMD(cwd); err != nil {
		fmt.Printf("Warning: could not update CLAUDE.md: %v\n", err)
	}

	checkPathAndPrintHint()
	return nil
}

const claudeMDSection = `## Context Memos

Before starting tasks, check if there's relevant context in the table below.
Run the command to see project-specific guidance that prevents repeated mistakes.

| When | Command |
|------|---------|
`

// ensureClaudeMD adds ContextPalace instructions to CLAUDE.md.
func ensureClaudeMD(cwd string) error {
	claudePath := filepath.Join(cwd, "CLAUDE.md")

	// Check if file exists
	content, err := os.ReadFile(claudePath)
	if os.IsNotExist(err) {
		// Create new CLAUDE.md
		if err := os.WriteFile(claudePath, []byte(claudeMDSection), 0644); err != nil {
			return err
		}
		fmt.Println("Created CLAUDE.md with ContextPalace instructions")
		return nil
	}
	if err != nil {
		return err
	}

	// Check if already has ContextPalace section
	if strings.Contains(string(content), "## Context Memos") {
		fmt.Println("CLAUDE.md already has Context Memos section")
		return nil
	}

	// Append section to existing file
	f, err := os.OpenFile(claudePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Add blank line before section if file doesn't end with newlines
	prefix := "\n\n"
	if len(content) > 0 && content[len(content)-1] == '\n' {
		prefix = "\n"
	}

	if _, err := f.WriteString(prefix + claudeMDSection); err != nil {
		return err
	}

	fmt.Println("Added Context Memos section to CLAUDE.md")
	return nil
}

// checkPathAndPrintHint checks if cxp is in PATH and prints install hint if not.
func checkPathAndPrintHint() {
	_, err := exec.LookPath("cxp")
	if err != nil {
		// cxp not in PATH, find current executable and suggest adding to PATH
		exePath, err := os.Executable()
		if err != nil {
			fmt.Println("\nNote: 'cxp' is not in your PATH.")
			fmt.Println("Add the directory containing cxp to your PATH, or install with:")
			fmt.Println("  go install github.com/otherjamesbrown/context-palace/cmd/cxp@latest")
			return
		}

		exeDir := filepath.Dir(exePath)
		fmt.Println()
		fmt.Println("Note: 'cxp' is not in your PATH. To fix, either:")
		fmt.Printf("  1. Add to PATH:  export PATH=\"$PATH:%s\"\n", exeDir)
		fmt.Println("  2. Install:      go install github.com/otherjamesbrown/context-palace/cmd/cxp@latest")
	}
}
