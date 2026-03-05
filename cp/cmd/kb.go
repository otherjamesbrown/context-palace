package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var kbCmd = &cobra.Command{
	Use:   "kb",
	Short: "Knowledge base search and navigation",
	Long: `Search and browse the knowledge base tree.

The knowledge base root is configured in ~/.cp/config.yaml:

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
			return fmt.Errorf("knowledge base root not configured. Set --root or add knowledge_base.root to ~/.cp/config.yaml")
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
			return fmt.Errorf("knowledge base root not configured. Set --root or add knowledge_base.root to ~/.cp/config.yaml")
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

	kbCmd.AddCommand(kbSearchCmd)
	kbCmd.AddCommand(kbTreeCmd)
	rootCmd.AddCommand(kbCmd)
}
