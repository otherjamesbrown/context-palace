package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var knowledgeCmd = &cobra.Command{
	Use:     "knowledge",
	Aliases: []string{"kd"},
	Short:   "Knowledge document management",
	Long:    `Commands for creating, updating, and versioning knowledge documents.`,
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

		docs, err := cpClient.ListKnowledgeDocs(ctx, docType, limitFlag)
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

		tbl := client.NewTable("ID", "DOC TYPE", "VERSION", "UPDATED", "TITLE")
		for _, d := range docs {
			tbl.AddRow(d.ID, d.DocType, fmt.Sprintf("%d", d.Version),
				d.UpdatedAt.Format("2006-01-02"), client.Truncate(d.Title, 50))
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

		if outputFormat == "json" {
			s, _ := client.FormatJSON(doc)
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
		fmt.Printf("\n%s\n", doc.Content)
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

	// list flags
	kdListCmd.Flags().String("doc-type", "", "Filter by document type")

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
