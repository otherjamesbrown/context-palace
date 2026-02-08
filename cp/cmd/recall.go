package cmd

import (
	"context"
	"fmt"
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

By default, only open and in_progress shards are searched.
Use --include-closed to also search closed shards.`,
	Args:    cobra.ExactArgs(1),
	Example: `  cp recall "pipeline timeout issues"
  cp recall "entity resolution" --type requirement,bug
  cp recall "deployment" --label architecture
  cp recall "timeout" --include-closed
  cp recall "vague query" --min-similarity 0.5
  cp recall "deployment" --limit 5 --since 7d`,
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		if cpClient.EmbedProvider == nil {
			return fmt.Errorf("semantic search requires embedding config.\nAdd an `embedding:` section to ~/.cp/config.yaml\nUse `cp memory search '%s'` for text search", query)
		}

		ctx := context.Background()

		// Embed the query
		vec, err := cpClient.EmbedProvider.Embed(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to embed query: %v", err)
		}

		// Parse flags
		typeFlag, _ := cmd.Flags().GetString("type")
		labelFlag, _ := cmd.Flags().GetString("label")
		statusFlag, _ := cmd.Flags().GetString("status")
		sinceFlag, _ := cmd.Flags().GetString("since")
		minSim, _ := cmd.Flags().GetFloat64("min-similarity")
		includeClosed, _ := cmd.Flags().GetBool("include-closed")

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
			status = []string{"open", "in_progress"}
		}
		// If --include-closed and no --status, status stays nil (no filter)

		results, err := cpClient.SemanticSearch(ctx, vec, types, labels, status, limitFlag, minSim)
		if err != nil {
			return err
		}

		// Post-filter by --since if specified
		if sinceFlag != "" {
			cutoff, err := parseSince(sinceFlag)
			if err != nil {
				return err
			}
			filtered := results[:0]
			for _, r := range results {
				if r.CreatedAt.After(cutoff) {
					filtered = append(filtered, r)
				}
			}
			results = filtered
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(results)
			fmt.Println(s)
			return nil
		}

		if len(results) == 0 {
			fmt.Println("No matching shards found.")
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
		return nil
	},
}

// parseSince parses duration strings like "7d", "24h", "30m"
func parseSince(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("invalid --since value: %q (use e.g. 7d, 24h, 30m)", s)
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	var num int
	if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
		return time.Time{}, fmt.Errorf("invalid --since value: %q (use e.g. 7d, 24h, 30m)", s)
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
		return time.Time{}, fmt.Errorf("invalid --since unit %q (use d, h, m, w)", string(unit))
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
	recallCmd.Flags().String("since", "", "Only show shards created since (e.g. 7d, 24h)")
	recallCmd.Flags().Float64("min-similarity", 0.3, "Minimum similarity threshold")
	recallCmd.Flags().Bool("include-closed", false, "Include closed/expired shards")

	rootCmd.AddCommand(recallCmd)
}
