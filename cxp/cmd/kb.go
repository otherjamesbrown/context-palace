package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var kbCmd = &cobra.Command{
	Use:   "kb",
	Short: "Knowledge base search and navigation",
	Long: `Navigate and search the knowledge base tree.

The KB tree is designed for structured navigation first, search second. Your
playbook (always loaded at session start) lists branches with triggers — follow
triggers to load branches, then find articles. Only use kb search when the
tree doesn't cover your question.

The knowledge base root is configured in ~/.cxp/config.yaml:

  knowledge_base:
    root: pf-34494b
    default_mode: hybrid`,
}

var kbSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the knowledge base tree",
	Long: `Search within the knowledge base using text, semantic, or hybrid search.

Modes:
  text      Full-text search using tsvector
  semantic  Vector similarity search using embeddings
  hybrid    Both text and semantic (default)`,
	Args: cobra.ExactArgs(1),
	Example: `  cxp kb search "pipeline throttling"
  cxp kb search "model selection" --mode semantic
  cxp kb search "concurrency" --show-path
  cxp kb search "throttling" --root pf-34494b`,
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		rootID, _ := cmd.Flags().GetString("root")
		mode, _ := cmd.Flags().GetString("mode")
		includeClosed, _ := cmd.Flags().GetBool("include-closed")
		showPath, _ := cmd.Flags().GetBool("show-path")
		minSim, _ := cmd.Flags().GetFloat64("min-similarity")

		// Resolve root from flag or config
		if rootID == "" {
			rootID = resolveKBRoot()
		}
		if rootID == "" {
			return fmt.Errorf("knowledge base root not configured. Set --root or add knowledge_base.root to ~/.cxp/config.yaml")
		}

		// Resolve mode from flag or config
		if mode == "" {
			mode = resolveKBMode()
		}

		ctx := context.Background()

		var queryText string
		var queryEmb []float32

		switch mode {
		case "text":
			queryText = query
		case "semantic":
			if cpClient.EmbedProvider == nil {
				return fmt.Errorf("semantic search requires embedding config")
			}
			emb, err := cpClient.EmbedProvider.Embed(ctx, query)
			if err != nil {
				return fmt.Errorf("failed to embed query: %v", err)
			}
			queryEmb = emb
		case "hybrid":
			queryText = query
			if cpClient.EmbedProvider != nil {
				emb, err := cpClient.EmbedProvider.Embed(ctx, query)
				if err != nil {
					if debugFlag {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: embedding failed, falling back to text: %v\n", err)
					}
				} else {
					queryEmb = emb
				}
			}
			// If no embed provider, fall back to text-only silently
		default:
			return fmt.Errorf("invalid mode %q: use text, semantic, or hybrid", mode)
		}

		results, err := cpClient.KBSearch(ctx, rootID, queryText, queryEmb, includeClosed, limitFlag, minSim)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(results)
			fmt.Println(s)
			return nil
		}

		if len(results) == 0 {
			fmt.Println("No results found in knowledge base.")
			return nil
		}

		if showPath {
			for _, r := range results {
				score := formatScore(r)
				path := formatPath(r.ParentPath)
				fmt.Printf("%s  %s  (depth:%d %s)\n", r.ID, client.Truncate(r.Title, 50), r.Depth, score)
				if path != "" {
					fmt.Printf("  via %s\n", path)
				}
				if r.Description != "" {
					fmt.Printf("  %s\n", client.Truncate(r.Description, 80))
				}
				fmt.Println()
			}
		} else {
			tbl := client.NewTable("SCORE", "ID", "DEPTH", "TITLE")
			for _, r := range results {
				tbl.AddRow(
					formatScore(r),
					r.ID,
					fmt.Sprintf("%d", r.Depth),
					client.Truncate(r.Title, 50),
				)
			}
			fmt.Print(tbl.String())
		}

		fmt.Printf("%d results (mode: %s)\n", len(results), mode)
		return nil
	},
}

var kbTreeCmd = &cobra.Command{
	Use:   "tree [root-id]",
	Short: "Browse the knowledge base tree structure",
	Args:  cobra.MaximumNArgs(1),
	Example: `  cxp kb tree
  cxp kb tree pf-c66536
  cxp kb tree --depth 2`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rootID := ""
		if len(args) > 0 {
			rootID = args[0]
		}
		if rootID == "" {
			rootID = resolveKBRoot()
		}
		if rootID == "" {
			return fmt.Errorf("knowledge base root not configured. Set --root or add knowledge_base.root to ~/.cxp/config.yaml")
		}

		depth, _ := cmd.Flags().GetInt("depth")
		includeClosed, _ := cmd.Flags().GetBool("include-closed")

		ctx := context.Background()

		nodes, err := cpClient.KBTree(ctx, rootID, depth, includeClosed)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(nodes)
			fmt.Println(s)
			return nil
		}

		if len(nodes) == 0 {
			fmt.Println("No children found.")
			return nil
		}

		for _, n := range nodes {
			indent := strings.Repeat("  ", n.Depth)
			childStr := ""
			if n.ChildCount > 0 {
				childStr = fmt.Sprintf(" — %d children", n.ChildCount)
			}
			desc := ""
			if n.Description != "" {
				desc = " — " + client.Truncate(n.Description, 60)
			}
			fmt.Printf("%s%s  %s%s%s\n", indent, n.ID, client.Truncate(n.Title, 40), childStr, desc)
		}

		return nil
	},
}

// kbTypeError returns the helpful error message when a shard is not a knowledge shard.
func kbTypeError(id, actualType, subcmd string) error {
	// Map type to its command namespace (if one exists)
	typeCmd := actualType
	msg := fmt.Sprintf("Error: %s is a %s, not a knowledge shard.\n", id, actualType)
	msg += fmt.Sprintf("  Use: cxp shard %s %s\n", subcmd, id)
	msg += fmt.Sprintf("  Or:  cxp %s %s %s", typeCmd, subcmd, id)
	return fmt.Errorf("%s", msg)
}

var kbShowCmd = &cobra.Command{
	Use:     "show <id>",
	Short:   "Show a knowledge shard",
	Args:    cobra.ExactArgs(1),
	Example: "  cxp kb show pf-34494b",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		// Type-check before delegating
		shard, err := cpClient.GetShard(ctx, id)
		if err != nil {
			return err
		}
		if shard.Type != "knowledge" {
			return kbTypeError(id, shard.Type, "show")
		}

		// Delegate to knowledge show logic
		doc, err := cpClient.ShowKnowledgeDoc(ctx, id)
		if err != nil {
			return err
		}

		_ = cpClient.KnowledgeTouch(ctx, id, cpClient.Config.Agent)
		children, _ := cpClient.ListKnowledgeChildren(ctx, id)

		if outputFormat == "json" {
			s, _ := client.FormatJSON(doc)
			fmt.Println(s)
			return nil
		}

		if shard.Status == "closed" {
			fmt.Printf("%s (%s) (closed)\n", doc.Title, doc.ID)
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

		if len(children) > 0 {
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

		return nil
	},
}

var kbListCmd = &cobra.Command{
	Use:   "list",
	Short: "List knowledge shards",
	Example: `  cxp kb list
  cxp kb list --parent pf-34494b
  cxp kb list --label core`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		parentID, _ := cmd.Flags().GetString("parent")
		labelFilter, _ := cmd.Flags().GetString("label")

		if parentID != "" {
			// List children of a parent knowledge shard
			children, err := cpClient.ListKnowledgeChildren(ctx, parentID)
			if err != nil {
				return err
			}

			if outputFormat == "json" {
				s, _ := client.FormatJSON(children)
				fmt.Println(s)
				return nil
			}

			if len(children) == 0 {
				fmt.Printf("No children found for %s.\n", parentID)
				return nil
			}

			tbl := client.NewTable("ID", "TITLE", "TRIGGER")
			for _, ch := range children {
				hint := ch.Trigger
				if hint == "" {
					hint = ch.Description
				}
				tbl.AddRow(ch.ID, client.Truncate(ch.Title, 50), client.Truncate(hint, 50))
			}
			fmt.Print(tbl.String())
			return nil
		}

		// List all knowledge shards
		docs, err := cpClient.ListKnowledgeDocs(ctx, "", labelFilter, limitFlag)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(docs)
			fmt.Println(s)
			return nil
		}

		if len(docs) == 0 {
			fmt.Println("No knowledge shards found.")
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

var kbCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a knowledge shard",
	Example: `  cxp kb create --title "My Article" --body "Content here"
  cxp kb create --title "My Article" --body-file article.md`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		title, _ := cmd.Flags().GetString("title")
		if title == "" {
			return fmt.Errorf("--title is required")
		}

		body, _ := cmd.Flags().GetString("body")
		bodyFile, _ := cmd.Flags().GetString("body-file")

		content, err := resolveBody(body, bodyFile)
		if err != nil {
			return err
		}

		labels, _ := cmd.Flags().GetStringSlice("label")

		id, err := cpClient.CreateKnowledgeDoc(ctx, title, content, "reference", labels)
		if err != nil {
			return err
		}

		parentID, _ := cmd.Flags().GetString("parent")
		trigger, _ := cmd.Flags().GetString("trigger")
		childDesc, _ := cmd.Flags().GetString("description")
		if parentID == "" && cpClient.Config.KnowledgeBase != nil && cpClient.Config.KnowledgeBase.Unsorted != "" {
			parentID = cpClient.Config.KnowledgeBase.Unsorted
		}
		if parentID != "" {
			if trigger != "" || childDesc != "" {
				if err := cpClient.AddKnowledgeChild(ctx, parentID, id, childDesc, trigger); err != nil {
					return fmt.Errorf("created knowledge shard %s but failed to link to parent: %v", id, err)
				}
			} else {
				if err := cpClient.CreateEdgeSimple(ctx, id, parentID, "child-of"); err != nil {
					return fmt.Errorf("created knowledge shard %s but failed to set parent: %v", id, err)
				}
			}
		}

		if outputFormat == "json" {
			doc, _ := cpClient.ShowKnowledgeDoc(ctx, id)
			s, _ := client.FormatJSON(doc)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Created knowledge shard %s\n", id)
		return nil
	},
}

var kbUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a knowledge shard (versioned)",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp kb update pf-34494b --body-file updated.md --summary "Added section"
  cxp kb update pf-34494b --body "New content" --summary "Rewrote"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		shard, err := cpClient.GetShard(ctx, id)
		if err != nil {
			return err
		}
		if shard.Type != "knowledge" {
			return kbTypeError(id, shard.Type, "update")
		}

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

var kbAppendCmd = &cobra.Command{
	Use:   "append <id>",
	Short: "Append content to a knowledge shard (versioned)",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp kb append pf-34494b --body "## New section" --summary "Added new section"
  cxp kb append pf-34494b --body-file addition.md --summary "Added entry"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		shard, err := cpClient.GetShard(ctx, id)
		if err != nil {
			return err
		}
		if shard.Type != "knowledge" {
			return kbTypeError(id, shard.Type, "append")
		}

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

var kbDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a knowledge shard",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp kb delete pf-34494b`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		shard, err := cpClient.GetShard(ctx, id)
		if err != nil {
			return err
		}
		if shard.Type != "knowledge" {
			return kbTypeError(id, shard.Type, "delete")
		}

		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			fmt.Printf("Delete knowledge shard %s (%s)? [y/N] ", id, client.Truncate(shard.Title, 50))
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				response := strings.TrimSpace(strings.ToLower(scanner.Text()))
				if response != "y" && response != "yes" {
					fmt.Println("Aborted.")
					return nil
				}
			}
		}

		if err := cpClient.DeleteShard(ctx, id); err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(map[string]string{"id": id, "status": "deleted"})
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Deleted knowledge shard %s\n", id)
		return nil
	},
}

func formatScore(r client.KBSearchResult) string {
	if r.CombinedScore > 0 {
		return fmt.Sprintf("%.2f", r.CombinedScore)
	}
	if r.Similarity > 0 {
		return fmt.Sprintf("%.2f", r.Similarity)
	}
	if r.TextRank > 0 {
		return fmt.Sprintf("%.4f", r.TextRank)
	}
	return "0"
}

func formatPath(path []string) string {
	if len(path) == 0 {
		return ""
	}
	return strings.Join(path, " > ")
}

func resolveKBRoot() string {
	if cpClient != nil && cpClient.Config != nil && cpClient.Config.KnowledgeBase != nil {
		return cpClient.Config.KnowledgeBase.Root
	}
	return ""
}

func resolveKBMode() string {
	if cpClient != nil && cpClient.Config != nil && cpClient.Config.KnowledgeBase != nil {
		m := cpClient.Config.KnowledgeBase.DefaultMode
		if m != "" {
			return m
		}
	}
	return "hybrid"
}

var kbExportPlaybookCmd = &cobra.Command{
	Use:   "export-playbook",
	Short: "Export the playbook shard to a file",
	Long: `Reads the configured KB root (or --root shard) and writes its content to a file.

Formats:
  raw     Raw markdown content, no wrapper (default)
  claude  Same as raw (Claude Code reads plain markdown)
  codex   AGENTS.md-compatible: prefix with agent header comment
  cursor  Adds YAML frontmatter suitable for .cursor/rules`,
	Example: `  cxp kb export-playbook --output .playbook.md
  cxp kb export-playbook --format cursor --output .cursor/rules/playbook.md
  cxp kb export-playbook --format codex --output AGENTS.md
  cxp kb export-playbook --root pf-34494b --output /tmp/test.md`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rootID, _ := cmd.Flags().GetString("root")
		outputPath, _ := cmd.Flags().GetString("output")
		format, _ := cmd.Flags().GetString("format")

		if rootID == "" {
			rootID = resolveKBRoot()
		}
		if rootID == "" {
			return fmt.Errorf("knowledge base root not configured. Set --root or add knowledge_base.root to ~/.cxp/config.yaml")
		}

		if outputPath == "" {
			outputPath = ".playbook.md"
		}

		switch format {
		case "raw", "claude", "codex", "cursor":
			// valid
		default:
			return fmt.Errorf("invalid format %q: use raw, claude, codex, or cursor", format)
		}

		ctx := context.Background()
		doc, err := cpClient.ShowKnowledgeDoc(ctx, rootID)
		if err != nil {
			return fmt.Errorf("fetch shard %s: %w", rootID, err)
		}

		content := buildExportContent(doc, format)

		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}

		if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write output: %w", err)
		}

		fmt.Printf("Wrote playbook to %s (%d bytes)\n", outputPath, len(content))
		return nil
	},
}

func buildExportContent(doc *client.KnowledgeDoc, format string) string {
	body := doc.Content
	switch format {
	case "codex":
		header := fmt.Sprintf("# AGENTS.md — generated from %s\n# DO NOT EDIT: regenerate with `cxp kb export-playbook --format codex`\n\n", doc.ID)
		return header + body
	case "cursor":
		frontmatter := fmt.Sprintf("---\ndescription: Playbook (generated from %s)\nalwaysApply: true\n---\n\n", doc.ID)
		return frontmatter + body
	default:
		// raw and claude: plain markdown
		return body
	}
}

func init() {
	// kb search flags
	kbSearchCmd.Flags().String("root", "", "Knowledge base root shard ID (overrides config)")
	kbSearchCmd.Flags().String("mode", "", "Search mode: text, semantic, hybrid (default from config)")
	kbSearchCmd.Flags().Bool("include-closed", false, "Include closed shards in results")
	kbSearchCmd.Flags().Bool("show-path", false, "Show breadcrumb path for each result")
	kbSearchCmd.Flags().Float64("min-similarity", 0.3, "Minimum similarity threshold for semantic search")

	// kb tree flags
	kbTreeCmd.Flags().Int("depth", 1, "Maximum tree depth to display")
	kbTreeCmd.Flags().Bool("include-closed", false, "Include closed shards")

	// kb list flags
	kbListCmd.Flags().String("parent", "", "Filter by parent shard ID (lists children)")
	kbListCmd.Flags().String("label", "", "Filter by label")

	// kb create flags
	kbCreateCmd.Flags().String("title", "", "Title of the knowledge shard (required)")
	kbCreateCmd.Flags().String("body", "", "Content (inline)")
	kbCreateCmd.Flags().String("body-file", "", "Read content from file")
	kbCreateCmd.Flags().StringSlice("label", nil, "Labels (repeatable)")
	kbCreateCmd.Flags().String("parent", "", "Parent shard ID")
	kbCreateCmd.Flags().String("trigger", "", "When to load this shard (used with --parent)")
	kbCreateCmd.Flags().String("description", "", "What this shard covers (used with --parent)")

	// kb update flags
	kbUpdateCmd.Flags().String("body", "", "New content (inline)")
	kbUpdateCmd.Flags().String("body-file", "", "New content from file")
	kbUpdateCmd.Flags().String("summary", "", "Change summary (required)")

	// kb append flags
	kbAppendCmd.Flags().String("body", "", "Content to append (inline)")
	kbAppendCmd.Flags().String("body-file", "", "Content to append from file")
	kbAppendCmd.Flags().String("summary", "", "Change summary (required)")

	// kb delete flags
	kbDeleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	// kb export-playbook flags
	kbExportPlaybookCmd.Flags().String("root", "", "Shard ID to export (default: config knowledge_base.root)")
	kbExportPlaybookCmd.Flags().String("output", "", "Output file path (default: .playbook.md)")
	kbExportPlaybookCmd.Flags().String("format", "raw", "Output format: raw, claude, codex, cursor")

	kbCmd.AddCommand(kbSearchCmd)
	kbCmd.AddCommand(kbTreeCmd)
	kbCmd.AddCommand(kbShowCmd)
	kbCmd.AddCommand(kbListCmd)
	kbCmd.AddCommand(kbCreateCmd)
	kbCmd.AddCommand(kbUpdateCmd)
	kbCmd.AddCommand(kbAppendCmd)
	kbCmd.AddCommand(kbDeleteCmd)
	kbCmd.AddCommand(kbExportPlaybookCmd)
	rootCmd.AddCommand(kbCmd)
}
