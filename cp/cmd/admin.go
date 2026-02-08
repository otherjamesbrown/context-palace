package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Administrative commands",
	Long:  `Administrative commands for database maintenance and data management.`,
}

var adminEmbedBackfillCmd = &cobra.Command{
	Use:   "embed-backfill",
	Short: "Generate embeddings for shards that don't have them",
	Long: `Fetches shards without embeddings and generates them using the configured
embedding provider. Rate-limited to ~50 requests/minute.`,
	Example: `  cp admin embed-backfill
  cp admin embed-backfill --dry-run
  cp admin embed-backfill --batch-size 20 --type task`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cpClient.EmbedProvider == nil {
			return fmt.Errorf("embedding provider not configured.\nAdd an `embedding:` section to ~/.cp/config.yaml")
		}

		ctx := context.Background()
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		batchSize, _ := cmd.Flags().GetInt("batch-size")
		typeFilter, _ := cmd.Flags().GetString("type")

		// Fetch all shards needing embedding (large limit for backfill)
		shards, err := cpClient.GetShardsNeedingEmbedding(ctx, 10000)
		if err != nil {
			return err
		}

		// Apply type filter if specified
		if typeFilter != "" {
			filtered := shards[:0]
			for _, s := range shards {
				if s.Type == typeFilter {
					filtered = append(filtered, s)
				}
			}
			shards = filtered
		}

		total := len(shards)
		if total == 0 {
			fmt.Println("No shards need embedding.")
			return nil
		}

		if dryRun {
			fmt.Printf("%d shards to embed.\n", total)
			if outputFormat == "json" {
				s, _ := client.FormatJSON(shards)
				fmt.Println(s)
			}
			return nil
		}

		fmt.Printf("Embedding %d shards...\n", total)

		var embedded, failed int
		for i, s := range shards {
			fmt.Printf("Embedding shard %d/%d: %s (%s)\n", i+1, total, s.ID, s.Type)

			// Fetch full content for embedding
			shardType, title, content, err := cpClient.GetShardContentForEmbedding(ctx, s.ID)
			if err != nil {
				fmt.Printf("  Error fetching content: %v\n", err)
				failed++
				continue
			}

			err = embedShard(ctx, cpClient, cpClient.EmbedProvider, s.ID, shardType, title, content)
			if err != nil {
				fmt.Printf("  Error embedding: %v\n", err)
				failed++
				continue
			}

			embedded++

			// Rate limit: ~50/min = 1.2s between calls
			if i < total-1 {
				time.Sleep(1200 * time.Millisecond)
			}

			// Batch boundary logging
			if batchSize > 0 && (i+1)%batchSize == 0 {
				fmt.Printf("  Batch complete: %d/%d\n", i+1, total)
			}
		}

		fmt.Printf("Done: %d embedded, %d failed\n", embedded, failed)
		return nil
	},
}

func init() {
	adminEmbedBackfillCmd.Flags().Bool("dry-run", false, "Show count without embedding")
	adminEmbedBackfillCmd.Flags().Int("batch-size", 10, "Log progress every N shards")
	adminEmbedBackfillCmd.Flags().String("type", "", "Only embed shards of this type")

	adminCmd.AddCommand(adminEmbedBackfillCmd)
	rootCmd.AddCommand(adminCmd)
}
