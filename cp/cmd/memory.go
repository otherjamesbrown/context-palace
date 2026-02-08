package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Agent memory",
	Long:  `Commands for managing agent memory — add, list, search, recall, resolve, and defer memories.`,
}

var memoryAddCmd = &cobra.Command{
	Use:   "add <content>",
	Short: "Add a memory",
	Args:  cobra.ExactArgs(1),
	Example: `  cp memory add "AI client timeout was hardcoded at 120s, not configurable"
  cp memory add "Entity names missing" --label entity,pipeline
  cp memory add "Discovered during investigation" --references pf-bug-03,pf-req-01`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		content := args[0]

		labelFlag, _ := cmd.Flags().GetString("label")
		refsFlag, _ := cmd.Flags().GetString("references")

		// Use content as title (truncated) and full text as content
		title := client.Truncate(content, 200)

		var labels []string
		if labelFlag != "" {
			labels = strings.Split(labelFlag, ",")
		}

		id, err := cpClient.CreateShard(ctx, title, content, "memory", nil, labels)
		if err != nil {
			return err
		}

		// Create reference edges if --references specified
		if refsFlag != "" {
			refIDs := strings.Split(refsFlag, ",")
			for _, refID := range refIDs {
				refID = strings.TrimSpace(refID)
				if refID == "" {
					continue
				}
				exists, err := cpClient.ShardExists(ctx, refID)
				if err != nil || !exists {
					fmt.Fprintf(os.Stderr, "Warning: Shard %s not found. Memory created without edge.\n", refID)
					continue
				}
				err = cpClient.CreateEdgeSimple(ctx, id, refID, "references")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Could not create edge to %s: %v\n", refID, err)
				}
			}
		}

		if outputFormat == "json" {
			out := map[string]any{"id": id}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Created memory %s\n", id)
		return nil
	},
}

var memoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List memories",
	Example: `  cp memory list
  cp memory list --label lesson-learned
  cp memory list --since 7d`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		labelFlag, _ := cmd.Flags().GetString("label")
		sinceFlag, _ := cmd.Flags().GetString("since")
		statusFlag, _ := cmd.Flags().GetString("status")
		rootsOnly, _ := cmd.Flags().GetBool("roots")

		opts := client.ListShardsOpts{
			Types:     []string{"memory"},
			Limit:     limitFlag,
			RootsOnly: rootsOnly,
		}

		if statusFlag != "" {
			opts.Status = strings.Split(statusFlag, ",")
		} else {
			opts.Status = []string{"open"}
		}
		if labelFlag != "" {
			opts.Labels = strings.Split(labelFlag, ",")
		}
		if sinceFlag != "" {
			cutoff, err := parseSince(sinceFlag)
			if err != nil {
				return err
			}
			opts.Since = &cutoff
		}

		memories, err := cpClient.ListShardsFiltered(ctx, opts)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(memories)
			fmt.Println(s)
			return nil
		}

		if len(memories) == 0 {
			fmt.Println("No memories found.")
			return nil
		}

		tbl := client.NewTable("ID", "CREATED", "CONTENT")
		for _, m := range memories {
			tbl.AddRow(m.ID, m.CreatedAt.Format("2006-01-02"), client.Truncate(m.Title, 60))
		}
		fmt.Print(tbl.String())
		return nil
	},
}

var memorySearchCmd = &cobra.Command{
	Use:     "search <query>",
	Short:   "Text search memories",
	Args:    cobra.ExactArgs(1),
	Example: `  cp memory search "timeout"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		results, err := cpClient.SearchShards(ctx, args[0], "memory", limitFlag)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(results)
			fmt.Println(s)
			return nil
		}

		if len(results) == 0 {
			fmt.Println("No matching memories found.")
			return nil
		}

		tbl := client.NewTable("ID", "CREATED", "CONTENT")
		for _, m := range results {
			tbl.AddRow(m.ID, m.CreatedAt.Format("2006-01-02"), client.Truncate(m.Title, 60))
		}
		fmt.Print(tbl.String())
		return nil
	},
}

var memoryRecallCmd = &cobra.Command{
	Use:   "recall <query>",
	Short: "Semantic search over memories",
	Args:  cobra.ExactArgs(1),
	Example: `  cp memory recall "deployment issues"
  cp memory recall "timeout" --label pipeline --limit 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		if cpClient.EmbedProvider == nil {
			return fmt.Errorf("semantic recall requires embedding config. Use `cp memory search` for text search")
		}

		ctx := context.Background()

		minSim, _ := cmd.Flags().GetFloat64("min-similarity")
		labelFlag, _ := cmd.Flags().GetString("label")

		vec, err := cpClient.EmbedProvider.Embed(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to embed query: %v", err)
		}

		var labels []string
		if labelFlag != "" {
			labels = strings.Split(labelFlag, ",")
		}

		results, err := cpClient.MemoryRecall(ctx, vec, labels, limitFlag, minSim)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(results)
			fmt.Println(s)
			return nil
		}

		if len(results) == 0 {
			fmt.Printf("No memories above %.2f similarity.\n", minSim)
			return nil
		}

		tbl := client.NewTable("SIMILARITY", "ID", "CONTENT")
		for _, r := range results {
			tbl.AddRow(
				fmt.Sprintf("%.2f", r.Similarity),
				r.ID,
				client.Truncate(r.Content, 60),
			)
		}
		fmt.Print(tbl.String())
		fmt.Printf("\n%d results (min similarity: %.2f)\n", len(results), minSim)
		return nil
	},
}

var memoryResolveCmd = &cobra.Command{
	Use:     "resolve <shard-id>",
	Short:   "Resolve (close) a memory",
	Args:    cobra.ExactArgs(1),
	Example: "  cp memory resolve pf-mem-123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		err := cpClient.UpdateShardStatus(ctx, args[0], "closed")
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"success": true, "id": "%s", "status": "closed"}`+"\n", args[0])
			return nil
		}

		fmt.Printf("Resolved memory %s\n", args[0])
		return nil
	},
}

var memoryDeferCmd = &cobra.Command{
	Use:     "defer <shard-id>",
	Short:   "Defer a memory for later review",
	Args:    cobra.ExactArgs(1),
	Example: "  cp memory defer pf-mem-123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		err := cpClient.UpdateShardStatus(ctx, args[0], "deferred")
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"success": true, "id": "%s", "status": "deferred"}`+"\n", args[0])
			return nil
		}

		fmt.Printf("Deferred memory %s\n", args[0])
		return nil
	},
}

func init() {
	// memory add flags
	memoryAddCmd.Flags().String("label", "", "Comma-separated labels")
	memoryAddCmd.Flags().String("references", "", "Shard IDs to create references edges to (comma-separated)")

	// memory list flags
	memoryListCmd.Flags().String("label", "", "Filter by label (comma-separated)")
	memoryListCmd.Flags().String("since", "", "Time filter: duration or date")
	memoryListCmd.Flags().String("status", "open", "Filter by status")
	memoryListCmd.Flags().Bool("roots", false, "Show only root memories (no parent)")

	// memory recall flags
	memoryRecallCmd.Flags().String("label", "", "Filter by label (comma-separated)")
	memoryRecallCmd.Flags().Float64("min-similarity", 0.3, "Minimum cosine similarity (0.0-1.0)")

	rootCmd.AddCommand(memoryCmd)
	memoryCmd.AddCommand(memoryAddCmd)
	memoryCmd.AddCommand(memoryListCmd)
	memoryCmd.AddCommand(memorySearchCmd)
	memoryCmd.AddCommand(memoryRecallCmd)
	memoryCmd.AddCommand(memoryResolveCmd)
	memoryCmd.AddCommand(memoryDeferCmd)
}
