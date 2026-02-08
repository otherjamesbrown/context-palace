package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/otherjamesbrown/context-palace/cp/internal/summary"
	"github.com/spf13/cobra"
)

var memoryAddSubCmd = &cobra.Command{
	Use:   "add-sub <parent-id>",
	Short: "Create a sub-memory under a parent",
	Args:  cobra.ExactArgs(1),
	Example: `  cp memory add-sub pf-aa1 --title "Troubleshooting" --body "If the service fails..."
  cp memory add-sub pf-aa1 --title "Troubleshooting" --body-file troubleshoot.md
  cp memory add-sub pf-aa1 --title "X" --body "Y" --no-ai --summary "When X happens"
  cp memory add-sub pf-aa1 --title "X" --body-file x.md --auto-approve`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		parentID := args[0]

		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")
		bodyFile, _ := cmd.Flags().GetString("body-file")
		labelFlag, _ := cmd.Flags().GetString("label")
		summaryFlag, _ := cmd.Flags().GetString("summary")
		noAI, _ := cmd.Flags().GetBool("no-ai")
		autoApprove, _ := cmd.Flags().GetBool("auto-approve")

		if title == "" {
			return fmt.Errorf("--title is required")
		}
		if body == "" && bodyFile == "" {
			return fmt.Errorf("--body or --body-file is required")
		}
		if body != "" && bodyFile != "" {
			return fmt.Errorf("cannot use both --body and --body-file")
		}
		if noAI && summaryFlag == "" {
			return fmt.Errorf("--summary required when using --no-ai")
		}

		// Read content
		var content string
		if bodyFile != "" {
			data, err := os.ReadFile(bodyFile)
			if err != nil {
				return fmt.Errorf("cannot read file '%s': %v", bodyFile, err)
			}
			content = string(data)
		} else {
			content = body
		}

		// Parse labels
		var labels []string
		if labelFlag != "" {
			labels = strings.Split(labelFlag, ",")
		}

		// Verify parent exists and is a memory
		parent, err := cpClient.GetShard(ctx, parentID)
		if err != nil {
			return fmt.Errorf("parent %s not found: %v", parentID, err)
		}
		if parent.Type != "memory" {
			return fmt.Errorf("parent %s is type '%s', expected 'memory'", parentID, parent.Type)
		}

		// Check depth warning
		path, err := cpClient.GetMemoryPath(ctx, parentID)
		if err == nil && len(path) > 0 {
			parentDepth := 0
			for _, n := range path {
				if n.ID == parentID {
					parentDepth = n.Depth
					break
				}
			}
			childDepth := parentDepth + 1
			if childDepth >= 5 && !autoApprove {
				fmt.Fprintf(os.Stderr, "Warning: This memory will be at depth %d. Deep hierarchies increase access latency. Continue? (y/n) ", childDepth)
				scanner := bufio.NewScanner(os.Stdin)
				if scanner.Scan() {
					if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
						return fmt.Errorf("cancelled")
					}
				}
			}
		}

		// Generate summary
		triggerSummary := summaryFlag

		if !noAI && triggerSummary == "" {
			if cpClient.Generator == nil {
				return fmt.Errorf("AI summary generation requires generation config. Use --no-ai --summary \"...\" to bypass")
			}

			prompt := summary.BuildSummaryPrompt(parentID, parent.Content, title, content)
			response, err := cpClient.Generator.Generate(ctx, prompt)
			if err != nil {
				return fmt.Errorf("summary generation failed: %v", err)
			}

			parsed, err := summary.ParseSummaryResponse(response)
			if err != nil {
				return fmt.Errorf("failed to parse AI response: %v", err)
			}

			triggerSummary = parsed.Summary

			if !autoApprove {
				// Show approval prompt
				fmt.Printf("\nSub-memory: %q → parent %q (%s)\n\n", title, parent.Title, parentID)
				fmt.Printf("Summary: %s\n", triggerSummary)

				if parsed.ParentNeedsUpdate && parsed.ParentEdits != nil {
					fmt.Printf("\nParent update suggested (review only — not auto-applied):\n")
					fmt.Printf("  %s\n", *parsed.ParentEdits)
				}

				fmt.Printf("\n[A]pprove summary  [E]dit summary  [C]ancel\n> ")

				approved, edited := promptApproval(triggerSummary)
				if !approved {
					return fmt.Errorf("cancelled by user")
				}
				triggerSummary = edited
			}
		}

		// Pre-compute embedding (outside transaction)
		vector := cpClient.PrecomputeEmbedding(ctx, title, content)

		// Atomic transaction
		result, err := cpClient.AddSubMemory(ctx, parentID, client.AddSubOpts{
			Title:   title,
			Body:    content,
			Labels:  labels,
			Summary: triggerSummary,
			Vector:  vector,
		})
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Created sub-memory %s %q under %s %q\n", result.ChildID, title, parentID, parent.Title)
		fmt.Printf("Summary: %s\n", triggerSummary)
		return nil
	},
}

var memoryDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Short:   "Delete a memory",
	Args:    cobra.ExactArgs(1),
	Example: "  cp memory delete pf-aa2 --force\n  cp memory delete pf-aa2 --recursive --force",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		force, _ := cmd.Flags().GetBool("force")
		recursive, _ := cmd.Flags().GetBool("recursive")

		// Get shard info for confirmation
		shard, err := cpClient.GetShard(ctx, id)
		if err != nil {
			return fmt.Errorf("shard %s not found", id)
		}
		if shard.Type != "memory" {
			return fmt.Errorf("shard %s is type '%s', expected 'memory'", id, shard.Type)
		}

		if !force {
			fmt.Printf("Delete memory %q (%s)?\n", shard.Title, id)

			// Count children
			children, err := cpClient.GetMemoryChildren(ctx, id)
			if err == nil && len(children) > 0 {
				fmt.Printf("  Children: %d", len(children))
				if recursive {
					fmt.Printf(" (will be deleted recursively)")
				} else {
					fmt.Printf(" (use --recursive to delete them too)")
				}
				fmt.Println()
			}

			fmt.Printf("[y/N] ")
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
					return fmt.Errorf("cancelled")
				}
			}
		}

		result, err := cpClient.DeleteMemory(ctx, id, recursive)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		for _, d := range result.Deleted {
			fmt.Printf("Deleted memory %s\n", d)
		}
		if result.ParentUpdated != nil {
			fmt.Printf("Removed pointer from parent %s\n", *result.ParentUpdated)
		}
		return nil
	},
}

var memoryMoveCmd = &cobra.Command{
	Use:   "move <id> [new-parent-id]",
	Short: "Re-parent a memory",
	Args:  cobra.RangeArgs(1, 2),
	Example: `  cp memory move pf-aa2 pf-xx1
  cp memory move pf-aa2 --root`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		toRoot, _ := cmd.Flags().GetBool("root")

		var newParentID string
		if toRoot {
			if len(args) > 1 {
				return fmt.Errorf("cannot specify both new-parent-id and --root")
			}
		} else {
			if len(args) < 2 {
				return fmt.Errorf("new-parent-id required (or use --root)")
			}
			newParentID = args[1]
		}

		result, err := cpClient.MoveMemory(ctx, id, newParentID, toRoot)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		oldStr := "root"
		if result.OldParent != nil {
			oldStr = *result.OldParent
		}
		newStr := "root"
		if result.NewParent != nil {
			newStr = *result.NewParent
		}
		fmt.Printf("Moved %s from %s to %s\n", id, oldStr, newStr)
		return nil
	},
}

var memoryPromoteCmd = &cobra.Command{
	Use:     "promote <id>",
	Short:   "Move a memory up one level",
	Args:    cobra.ExactArgs(1),
	Example: "  cp memory promote pf-aa2",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		result, err := cpClient.PromoteMemory(ctx, id)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		newStr := "root"
		if result.NewParent != nil {
			newStr = *result.NewParent
		}
		fmt.Printf("Promoted %s to depth %d (%s)\n", id, result.NewDepth, newStr)
		return nil
	},
}

// promptApproval handles the interactive A/E/C approval loop.
// Returns (approved, finalSummary).
func promptApproval(currentSummary string) (bool, string) {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		if !scanner.Scan() {
			return false, ""
		}
		input := strings.ToLower(strings.TrimSpace(scanner.Text()))

		switch input {
		case "a", "approve":
			return true, currentSummary
		case "c", "cancel":
			return false, ""
		case "e", "edit":
			edited := editInEditor(currentSummary)
			if edited != "" {
				currentSummary = edited
			}
			fmt.Printf("Summary: %s\n", currentSummary)
			fmt.Printf("[A]pprove  [E]dit  [C]ancel\n> ")
		default:
			fmt.Printf("[A]pprove  [E]dit  [C]ancel\n> ")
		}
	}
}

// editInEditor opens $EDITOR with the given text and returns the edited result.
func editInEditor(text string) string {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	tmpFile, err := os.CreateTemp("", "cp-summary-*.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp file: %v\n", err)
		return text
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(text); err != nil {
		tmpFile.Close()
		return text
	}
	tmpFile.Close()

	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Editor exited with error: %v\n", err)
		return text
	}

	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return text
	}

	result := strings.TrimSpace(string(data))
	if result == "" {
		return text
	}
	return result
}

func init() {
	// add-sub flags
	memoryAddSubCmd.Flags().String("title", "", "Child memory title (required)")
	memoryAddSubCmd.Flags().String("body", "", "Child content (inline)")
	memoryAddSubCmd.Flags().String("body-file", "", "Child content (from file)")
	memoryAddSubCmd.Flags().String("label", "", "Labels (comma-separated)")
	memoryAddSubCmd.Flags().String("summary", "", "Manual trigger summary (skips AI)")
	memoryAddSubCmd.Flags().Bool("no-ai", false, "Skip AI summary generation (requires --summary)")
	memoryAddSubCmd.Flags().Bool("auto-approve", false, "Accept AI suggestion without review")

	// delete flags
	memoryDeleteCmd.Flags().Bool("force", false, "Skip confirmation")
	memoryDeleteCmd.Flags().Bool("recursive", false, "Delete all descendants")

	// move flags
	memoryMoveCmd.Flags().Bool("root", false, "Move to root (no parent)")

	memoryCmd.AddCommand(memoryAddSubCmd)
	memoryCmd.AddCommand(memoryDeleteCmd)
	memoryCmd.AddCommand(memoryMoveCmd)
	memoryCmd.AddCommand(memoryPromoteCmd)
}
