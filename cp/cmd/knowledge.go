package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"encoding/json"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var knowledgeCmd = &cobra.Command{
	Use:     "knowledge",
	Aliases: []string{"kd"},
	Short:   "Knowledge document management",
	Long: `Knowledge documents form a navigable tree — the agent's institutional memory.
The tree is structured so agents can navigate (warm retrieval) before searching
(cold retrieval). Every knowledge shard should be linked into the tree with a
trigger and description so other agents can find it by following the tree.`,
}

var kdCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new knowledge document",
	Args:  cobra.ExactArgs(1),
	Example: `  cp knowledge create "System Architecture" --doc-type architecture --body "## Components"
  cp knowledge create "Decisions Log" --doc-type decision --body-file decisions.md --label core`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		title := args[0]

		docType, _ := cmd.Flags().GetString("doc-type")
		if docType == "" {
			return fmt.Errorf("--doc-type is required")
		}
		if err := client.ValidateDocType(docType); err != nil {
			return err
		}

		body, _ := cmd.Flags().GetString("body")
		bodyFile, _ := cmd.Flags().GetString("body-file")

		content, err := resolveBody(body, bodyFile)
		if err != nil {
			return err
		}

		labels, _ := cmd.Flags().GetStringSlice("label")

		id, err := cpClient.CreateKnowledgeDoc(ctx, title, content, docType, labels)
		if err != nil {
			return err
		}

		// Determine parent: explicit flag > config unsorted default
		parentID, _ := cmd.Flags().GetString("parent")
		trigger, _ := cmd.Flags().GetString("trigger")
		childDesc, _ := cmd.Flags().GetString("description")
		if parentID == "" && cpClient.Config.KnowledgeBase != nil && cpClient.Config.KnowledgeBase.Unsorted != "" {
			parentID = cpClient.Config.KnowledgeBase.Unsorted
		}
		if parentID != "" {
			if trigger != "" || childDesc != "" {
				// Use AddKnowledgeChild to set edge metadata (trigger + description)
				if err := cpClient.AddKnowledgeChild(ctx, parentID, id, childDesc, trigger); err != nil {
					return fmt.Errorf("created knowledge doc %s but failed to link to parent: %v", id, err)
				}
			} else {
				if err := cpClient.CreateEdgeSimple(ctx, id, parentID, "child-of"); err != nil {
					return fmt.Errorf("created knowledge doc %s but failed to set parent: %v", id, err)
				}
			}
		}

		if outputFormat == "json" {
			result := map[string]any{
				"id":         id,
				"title":      title,
				"doc_type":   docType,
				"version":    1,
				"created_at": "now",
			}
			// Re-fetch for accurate timestamp
			doc, fetchErr := cpClient.ShowKnowledgeDoc(ctx, id)
			if fetchErr == nil {
				result["created_at"] = doc.CreatedAt
			}
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Created knowledge document %s (%s, v1)\n", id, docType)
		return nil
	},
}

var kdListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List knowledge documents",
	Example: "  cp knowledge list\n  cp knowledge list --doc-type architecture",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		docType, _ := cmd.Flags().GetString("doc-type")
		labelFilter, _ := cmd.Flags().GetString("label")

		docs, err := cpClient.ListKnowledgeDocs(ctx, docType, labelFilter, limitFlag)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(docs)
			fmt.Println(s)
			return nil
		}

		if len(docs) == 0 {
			fmt.Println("No knowledge documents found.")
			return nil
		}

		tbl := client.NewTable("ID", "DOC TYPE", "VERSION", "READS", "UPDATED", "TITLE")
		for _, d := range docs {
			reads := "-"
			if d.AccessCount > 0 {
				reads = fmt.Sprintf("%d", d.AccessCount)
			}
			tbl.AddRow(d.ID, d.DocType, fmt.Sprintf("%d", d.Version),
				reads, d.UpdatedAt.Format("2006-01-02"), client.Truncate(d.Title, 50))
		}
		fmt.Print(tbl.String())
		return nil
	},
}

var kdShowCmd = &cobra.Command{
	Use:     "show <id>",
	Short:   "Show a knowledge document",
	Args:    cobra.ExactArgs(1),
	Example: "  cp knowledge show pf-arch-001\n  cp knowledge show pf-arch-001 --version 2",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		versionFlag, _ := cmd.Flags().GetInt("version")

		var doc *client.KnowledgeDoc
		var err error

		if versionFlag > 0 {
			doc, err = cpClient.GetKnowledgeVersion(ctx, id, versionFlag)
		} else {
			doc, err = cpClient.ShowKnowledgeDoc(ctx, id)
		}
		if err != nil {
			return err
		}

		// Touch telemetry for current version reads (fire-and-forget)
		if versionFlag == 0 {
			_ = cpClient.KnowledgeTouch(ctx, id, cpClient.Config.Agent)
		}

		// Extract access stats from metadata
		var accessCount int
		var lastAccessed string
		if doc.Metadata != nil {
			if v, ok := doc.Metadata["access_count"]; ok {
				if f, ok := v.(float64); ok {
					accessCount = int(f)
				}
			}
			if v, ok := doc.Metadata["last_accessed"]; ok {
				if s, ok := v.(string); ok {
					lastAccessed = s
				}
			}
		}

		// Fetch children (fire-and-forget on error)
		children, _ := cpClient.ListKnowledgeChildren(ctx, id)

		if outputFormat == "json" {
			// Build output map from doc, then add children
			raw, _ := json.Marshal(doc)
			var out map[string]any
			json.Unmarshal(raw, &out)
			if len(children) > 0 {
				out["children"] = children
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		// Check if closed
		if versionFlag == 0 {
			// For current version, check shard status
			shard, sErr := cpClient.GetShard(ctx, id)
			if sErr == nil && shard.Status == "closed" {
				fmt.Printf("%s (%s) (closed)\n", doc.Title, doc.ID)
			} else {
				fmt.Printf("%s (%s)\n", doc.Title, doc.ID)
			}
		} else {
			fmt.Printf("%s (%s)\n", doc.Title, doc.ID)
		}
		displayDate := doc.UpdatedAt
		if displayDate.IsZero() {
			displayDate = doc.CreatedAt
		}
		fmt.Printf("Type: %s | Version: %d | Updated: %s\n", doc.DocType, doc.Version, displayDate.Format("2006-01-02"))
		if len(doc.Labels) > 0 {
			fmt.Printf("Labels: %s\n", strings.Join(doc.Labels, ", "))
		}
		if accessCount > 0 {
			accessStr := fmt.Sprintf("%d times", accessCount)
			if lastAccessed != "" {
				accessStr += fmt.Sprintf(" (last: %s)", lastAccessed)
			}
			fmt.Printf("Accessed: %s\n", accessStr)
		}
		fmt.Printf("\n%s\n", doc.Content)

		// Render children block if any exist
		if len(children) > 0 {
			if outputFormat == "json" {
				fmt.Println("\n--- children ---")
				childJSON, _ := json.MarshalIndent(map[string]any{
					"children": children,
				}, "", "  ")
				fmt.Println(string(childJSON))
			} else {
				fmt.Printf("\n--- Children (%d) ---\n", len(children))
				for _, ch := range children {
					hint := ch.Trigger
					if hint == "" {
						hint = ch.Description
					}
					if len(hint) > 40 {
						hint = hint[:37] + "..."
					}
					title := ch.Title
					if len(title) > 30 {
						title = title[:27] + "..."
					}
					if hint != "" {
						fmt.Printf("  %-24s %-30s → %s\n", ch.ID, title, hint)
					} else {
						fmt.Printf("  %-24s %s\n", ch.ID, title)
					}
				}
			}
		}

		return nil
	},
}

var kdUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update document content (versioned)",
	Args:  cobra.ExactArgs(1),
	Example: `  cp knowledge update pf-arch-001 --body-file updated-arch.md --summary "Added pipeline stage diagram"
  cp knowledge update pf-arch-001 --body "New content" --summary "Rewrote section"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		summary, _ := cmd.Flags().GetString("summary")
		if summary == "" {
			return fmt.Errorf("update requires --summary to describe the change")
		}

		body, _ := cmd.Flags().GetString("body")
		bodyFile, _ := cmd.Flags().GetString("body-file")

		content, err := resolveBody(body, bodyFile)
		if err != nil {
			return err
		}

		result, err := cpClient.UpdateKnowledgeDoc(ctx, id, content, summary)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Updated %s to v%d\n", result.ID, result.Version)
		fmt.Printf("Previous version preserved as %s\n", result.PreviousVersionID)
		return nil
	},
}

var kdAppendCmd = &cobra.Command{
	Use:   "append <id>",
	Short: "Append content to document (versioned)",
	Args:  cobra.ExactArgs(1),
	Example: `  cp knowledge append pf-dec-001 --summary "Decision: Split CLI" --body "## Decision: Split CLI"
  cp knowledge append pf-dec-001 --summary "Added entry" --body-file entry.md`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		summary, _ := cmd.Flags().GetString("summary")
		if summary == "" {
			return fmt.Errorf("append requires --summary to describe the change")
		}

		body, _ := cmd.Flags().GetString("body")
		bodyFile, _ := cmd.Flags().GetString("body-file")

		content, err := resolveBody(body, bodyFile)
		if err != nil {
			return err
		}

		result, err := cpClient.AppendKnowledgeDoc(ctx, id, content, summary)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Appended to %s, now v%d\n", result.ID, result.Version)
		fmt.Printf("Previous version preserved as %s\n", result.PreviousVersionID)
		return nil
	},
}

var kdHistoryCmd = &cobra.Command{
	Use:     "history <id>",
	Short:   "Show version history",
	Args:    cobra.ExactArgs(1),
	Example: "  cp knowledge history pf-arch-001",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		entries, err := cpClient.KnowledgeHistory(ctx, id)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(entries)
			fmt.Println(s)
			return nil
		}

		if len(entries) == 0 {
			fmt.Println("No version history found.")
			return nil
		}

		tbl := client.NewTable("VERSION", "DATE", "CHANGED BY", "SUMMARY")
		for _, e := range entries {
			tbl.AddRow(
				fmt.Sprintf("%d", e.Version),
				e.ChangedAt.Format("2006-01-02"),
				e.ChangedBy,
				client.Truncate(e.ChangeSummary, 60),
			)
		}
		fmt.Print(tbl.String())
		return nil
	},
}

var kdDiffCmd = &cobra.Command{
	Use:   "diff <id>",
	Short: "Diff between versions",
	Args:  cobra.ExactArgs(1),
	Example: `  cp knowledge diff pf-arch-001
  cp knowledge diff pf-arch-001 --from 1 --to 3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		fromVer, _ := cmd.Flags().GetInt("from")
		toVer, _ := cmd.Flags().GetInt("to")

		// If neither specified, get current version and diff N vs N-1
		if fromVer == 0 && toVer == 0 {
			doc, err := cpClient.ShowKnowledgeDoc(ctx, id)
			if err != nil {
				return err
			}
			if doc.Version <= 1 {
				return fmt.Errorf("document has only 1 version. Nothing to diff")
			}
			fromVer = doc.Version - 1
			toVer = doc.Version
		} else if fromVer == 0 {
			fromVer = toVer - 1
			if fromVer < 1 {
				fromVer = 1
			}
		} else if toVer == 0 {
			doc, err := cpClient.ShowKnowledgeDoc(ctx, id)
			if err != nil {
				return err
			}
			toVer = doc.Version
		}

		// Swap if from > to
		if fromVer > toVer {
			fromVer, toVer = toVer, fromVer
		}

		diffText, err := cpClient.DiffVersions(ctx, id, fromVer, toVer)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			result := map[string]any{
				"id":           id,
				"from_version": fromVer,
				"to_version":   toVer,
				"diff":         diffText,
			}
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		if diffText == "" {
			fmt.Println("No differences found.")
			return nil
		}

		fmt.Print(diffText)
		return nil
	},
}

// resolveBody reads content from --body or --body-file flags
func resolveBody(body, bodyFile string) (string, error) {
	if body == "" && bodyFile == "" {
		return "", fmt.Errorf("either --body or --body-file is required")
	}
	if body != "" && bodyFile != "" {
		return "", fmt.Errorf("specify either --body or --body-file, not both")
	}
	if bodyFile != "" {
		data, err := os.ReadFile(bodyFile)
		if err != nil {
			return "", fmt.Errorf("cannot read file '%s': %v", bodyFile, err)
		}
		return string(data), nil
	}
	return body, nil
}

func init() {
	// create flags
	kdCreateCmd.Flags().String("doc-type", "", "Document type (required): architecture, vision, roadmap, decision, reference")
	kdCreateCmd.Flags().String("body", "", "Document content (inline)")
	kdCreateCmd.Flags().String("body-file", "", "Read document content from file")
	kdCreateCmd.Flags().StringSlice("label", nil, "Labels (repeatable)")
	kdCreateCmd.Flags().String("parent", "", "Parent shard ID (default: unsorted from config)")
	kdCreateCmd.Flags().String("trigger", "", "When to load this doc — routing hint for tree navigation (used with --parent)")
	kdCreateCmd.Flags().String("description", "", "What this doc covers — shown in parent listings (used with --parent)")

	// list flags
	kdListCmd.Flags().String("doc-type", "", "Filter by document type")
	kdListCmd.Flags().String("label", "", "Filter by label")

	// show flags
	kdShowCmd.Flags().Int("version", 0, "Show specific version number")

	// update flags
	kdUpdateCmd.Flags().String("body", "", "New content (inline)")
	kdUpdateCmd.Flags().String("body-file", "", "New content from file")
	kdUpdateCmd.Flags().String("summary", "", "Change summary (required)")

	// append flags
	kdAppendCmd.Flags().String("body", "", "Content to append (inline)")
	kdAppendCmd.Flags().String("body-file", "", "Content to append from file")
	kdAppendCmd.Flags().String("summary", "", "Change summary (required)")

	// diff flags
	kdDiffCmd.Flags().Int("from", 0, "Source version number")
	kdDiffCmd.Flags().Int("to", 0, "Target version number")

	// Wire command tree
	knowledgeCmd.AddCommand(kdCreateCmd)
	knowledgeCmd.AddCommand(kdListCmd)
	knowledgeCmd.AddCommand(kdShowCmd)
	knowledgeCmd.AddCommand(kdUpdateCmd)
	knowledgeCmd.AddCommand(kdAppendCmd)
	knowledgeCmd.AddCommand(kdHistoryCmd)
	knowledgeCmd.AddCommand(kdDiffCmd)

	rootCmd.AddCommand(knowledgeCmd)
}
