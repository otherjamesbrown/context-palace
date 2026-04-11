package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var boardCmd = &cobra.Command{
	Use:   "board",
	Short: "Show open shards grouped by area with token estimates",
	Example: `  cxp board
  cxp board --since 7d
  cxp board --area Pipeline
  cxp board --all
  cxp board --types bug,task
  cxp board -o json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		opts := client.BoardOpts{}

		sinceStr, _ := cmd.Flags().GetString("since")
		if sinceStr != "" {
			d, err := parseBoardDuration(sinceStr)
			if err != nil {
				return fmt.Errorf("invalid --since value: %v", err)
			}
			t := time.Now().Add(-d)
			opts.Since = &t
		}

		opts.Area, _ = cmd.Flags().GetString("area")
		opts.Agent, _ = cmd.Flags().GetString("agent")
		opts.Budget, _ = cmd.Flags().GetInt("budget")
		opts.All, _ = cmd.Flags().GetBool("all")
		boardFormat, _ := cmd.Flags().GetString("format")

		typesStr, _ := cmd.Flags().GetString("types")
		if typesStr != "" {
			opts.Types = strings.Split(typesStr, ",")
		}

		result, err := cpClient.GetBoardShards(ctx, opts)
		if err != nil {
			return err
		}

		// --format=compact takes precedence over config-defaulted output format;
		// only -o json explicitly passed by the user overrides it.
		jsonExplicit := cmd.Flags().Changed("output") && outputFormat == "json"
		if jsonExplicit || (outputFormat == "json" && boardFormat == "verbose") {
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		if boardFormat == "compact" {
			fmt.Print(client.FormatBoardCompact(result))
			return nil
		}

		hasContent := len(result.Focus) > 0 || len(result.NeedsReview) > 0 ||
			len(result.Blocked) > 0 || len(result.RecentActivity) > 0 || len(result.Groups) > 0

		if !hasContent {
			fmt.Println("No shards to display.")
		} else {
			printed := false

			// FOCUS section
			if len(result.Focus) > 0 {
				fmt.Printf("# FOCUS (%d items, ~%s) #\n\n", len(result.Focus), formatBoardTokens(result.FocusTokens))
				for _, e := range result.Focus {
					printBoardEntry(e)
				}
				printed = true
			}

			// Needs Review section
			if len(result.NeedsReview) > 0 {
				if printed {
					fmt.Println()
				}
				fmt.Printf("# NEEDS REVIEW (%d items) #\n\n", len(result.NeedsReview))
				for _, e := range result.NeedsReview {
					printBoardEntry(e)
				}
				printed = true
			}

			// Blocked section
			if len(result.Blocked) > 0 {
				if printed {
					fmt.Println()
				}
				fmt.Printf("# BLOCKED (%d items) #\n\n", len(result.Blocked))
				for _, e := range result.Blocked {
					printBoardEntry(e)
				}
				printed = true
			}

			// Recent Activity section
			if len(result.RecentActivity) > 0 {
				if printed {
					fmt.Println()
				}
				fmt.Printf("# Recent Activity (%d items, ~%s) #\n\n",
					len(result.RecentActivity), formatBoardTokens(result.RecentTokens))
				for _, e := range result.RecentActivity {
					printBoardEntry(e)
				}
				printed = true
			}

			// Type groups
			for _, g := range result.Groups {
				if len(g.Items) == 0 {
					continue
				}
				if printed {
					fmt.Println()
				}
				fmt.Printf("# Open %s (%d items, ~%s) #\n\n",
					strings.ToUpper(g.Type+"s"), len(g.Items), formatBoardTokens(g.TotalTokens))
				for _, e := range g.Items {
					printBoardEntryNoType(e)
				}
				printed = true
			}
		}

		fmt.Println()
		fmt.Printf("Inbox: %d unread messages\n", result.UnreadCount)
		fmt.Printf("Memories: %d active\n", result.MemoryCount)
		return nil
	},
}

func printBoardEntry(e client.BoardEntry) {
	fmt.Printf("  %-7s %-90s  %5dt  %5s  %s\n",
		e.Type, client.Truncate(e.Title, 90),
		e.TokenEstimate, client.FormatAgeHours(e.AgeHours), e.ID)
}

func printBoardEntryNoType(e client.BoardEntry) {
	fmt.Printf("  %-100s  %5dt  %5s  %s\n",
		client.Truncate(e.Title, 100),
		e.TokenEstimate, client.FormatAgeHours(e.AgeHours), e.ID)
}

func parseBoardDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		s = strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(s, "%d", &days); err != nil {
			return 0, fmt.Errorf("invalid day count: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func formatBoardTokens(t int) string {
	if t >= 1000 {
		return fmt.Sprintf("%.1fk tokens", float64(t)/1000)
	}
	return fmt.Sprintf("%dt", t)
}

func init() {
	boardCmd.Flags().String("since", "", "Include recently closed shards (e.g., 7d, 24h)")
	boardCmd.Flags().String("area", "", "Filter to area prefix")
	boardCmd.Flags().String("agent", "", "Filter by creator agent")
	boardCmd.Flags().Int("budget", 0, "Token budget highlight threshold")
	boardCmd.Flags().Bool("all", false, "Show all shard types including knowledge")
	boardCmd.Flags().String("types", "", "Shard types to show (comma-separated)")
	boardCmd.Flags().String("format", "verbose", "Output format: verbose (default) or compact")

	rootCmd.AddCommand(boardCmd)
}
