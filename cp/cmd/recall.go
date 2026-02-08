package cmd

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/otherjamesbrown/context-palace/cp/internal/embedding"
	"github.com/spf13/cobra"
)

var recallCmd = &cobra.Command{
	Use:   "recall <query>",
	Short: "Semantic search across all shard types",
	Long: `Search Context Palace by meaning, not just keywords.
Uses vector embeddings to find semantically similar shards.

By default, only open shards are searched.
Use --include-closed to also search closed shards.`,
	Args:    cobra.ExactArgs(1),
	Example: `  cp recall "pipeline timeout issues"
  cp recall "entity resolution" --type requirement,bug
  cp recall "deployment" --label architecture
  cp recall "timeout" --include-closed
  cp recall "vague query" --min-similarity 0.5
  cp recall "deployment" --limit 5 --since 7d
  cp recall "entity" --show-snippet`,
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		// Parse flags
		typeFlag, _ := cmd.Flags().GetString("type")
		labelFlag, _ := cmd.Flags().GetString("label")
		statusFlag, _ := cmd.Flags().GetString("status")
		sinceFlag, _ := cmd.Flags().GetString("since")
		minSim, _ := cmd.Flags().GetFloat64("min-similarity")
		includeClosed, _ := cmd.Flags().GetBool("include-closed")
		showSnippet, _ := cmd.Flags().GetBool("show-snippet")

		// Validate mutually exclusive flags
		if statusFlag != "" && includeClosed {
			return fmt.Errorf("--status and --include-closed are mutually exclusive")
		}
		if minSim < 0.0 || minSim > 1.0 {
			return fmt.Errorf("min-similarity must be between 0.0 and 1.0")
		}
		if limitFlag < 1 || limitFlag > 1000 {
			return fmt.Errorf("limit must be 1-1000")
		}

		if cpClient.EmbedProvider == nil {
			return fmt.Errorf("semantic search requires embedding config. Use `cp shard list --search` for text search")
		}

		ctx := context.Background()

		// Embed the query
		vec, err := cpClient.EmbedProvider.Embed(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to embed query: %v", err)
		}

		var types []string
		if typeFlag != "" {
			types = strings.Split(typeFlag, ",")
		}

		var labels []string
		if labelFlag != "" {
			labels = strings.Split(labelFlag, ",")
		}

		var status []string
		if statusFlag != "" {
			status = strings.Split(statusFlag, ",")
		} else if !includeClosed {
			status = []string{"open"}
		}
		// If --include-closed and no --status, status stays nil (no filter)

		// Parse --since into time cutoff
		var since *time.Time
		if sinceFlag != "" {
			cutoff, err := parseSince(sinceFlag)
			if err != nil {
				return err
			}
			since = &cutoff
		}

		results, err := cpClient.SemanticSearchWithSince(ctx, vec, types, labels, status, limitFlag, minSim, since)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(results)
			fmt.Println(s)
			return nil
		}

		if len(results) == 0 {
			fmt.Printf("No results above %.2f similarity.\n", minSim)
			return nil
		}

		tbl := client.NewTable("SIMILARITY", "TYPE", "STATUS", "ID", "TITLE")
		for _, r := range results {
			tbl.AddRow(
				fmt.Sprintf("%.2f", r.Similarity),
				r.Type,
				r.Status,
				r.ID,
				client.Truncate(r.Title, 50),
			)
		}
		fmt.Print(tbl.String())

		if showSnippet {
			fmt.Println()
			for _, r := range results {
				if r.Snippet != "" {
					snippet := strings.ReplaceAll(r.Snippet, "\n", " ")
					fmt.Printf("%s  \"%s\"\n", r.ID, client.Truncate(snippet, 100))
				}
			}
		}

		fmt.Printf("\n%d results (min similarity: %.2f)\n", len(results), minSim)
		return nil
	},
}

// parseSince parses duration strings like "7d", "24h", "30m", "2w" or ISO dates "2026-01-01"
func parseSince(s string) (time.Time, error) {
	s = strings.TrimSpace(s)

	// Try ISO date first: YYYY-MM-DD
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, s); matched {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid date: %q", s)
		}
		return t, nil
	}

	// Duration format: Nd, Nh, Nm, Nw
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("invalid duration: '%s'. Use format like '7d', '24h', or '2026-01-01'", s)
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	var num int
	if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
		return time.Time{}, fmt.Errorf("invalid duration: '%s'. Use format like '7d', '24h', or '2026-01-01'", s)
	}

	now := time.Now()
	switch unit {
	case 'd':
		return now.AddDate(0, 0, -num), nil
	case 'h':
		return now.Add(-time.Duration(num) * time.Hour), nil
	case 'm':
		return now.Add(-time.Duration(num) * time.Minute), nil
	case 'w':
		return now.AddDate(0, 0, -num*7), nil
	default:
		return time.Time{}, fmt.Errorf("invalid duration: '%s'. Use format like '7d', '24h', or '2026-01-01'", s)
	}
}

// embedShard embeds a shard and updates its embedding in the database.
// Used by both embed-on-write and backfill.
func embedShard(ctx context.Context, cl *client.Client, provider embedding.Provider, shardID, shardType, title, content string) error {
	text := embedding.BuildEmbeddingText(shardType, title, content)
	if text == "" {
		return nil // Nothing to embed
	}

	vec, err := provider.Embed(ctx, text)
	if err != nil {
		return err
	}

	return cl.UpdateEmbedding(ctx, shardID, vec)
}

func init() {
	recallCmd.Flags().String("type", "", "Filter by shard type (comma-separated)")
	recallCmd.Flags().String("label", "", "Filter by label (comma-separated)")
	recallCmd.Flags().String("status", "", "Filter by status (comma-separated)")
	recallCmd.Flags().String("since", "", "Time filter: duration (7d, 24h, 2w, 30m) or date (2026-01-01)")
	recallCmd.Flags().Float64("min-similarity", 0.3, "Minimum similarity threshold (0.0-1.0)")
	recallCmd.Flags().Bool("include-closed", false, "Include all statuses")
	recallCmd.Flags().Bool("show-snippet", false, "Show content preview under each result")

	rootCmd.AddCommand(recallCmd)
}
