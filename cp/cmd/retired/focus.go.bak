package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var focusCmd = &cobra.Command{
	Use:   "focus",
	Short: "Show/set/clear active epic",
	Long:  `Manage the active epic focus for the current agent. Focus persists across sessions.`,
	Example: `  cp focus                          # show current focus
  cp focus set pf-abc123            # set focus to epic
  cp focus set pf-abc123 --note "working on this until EOD"
  cp focus clear                    # clear focus`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default: show current focus
		ctx := context.Background()

		focus, err := cpClient.GetFocus(ctx)
		if err != nil {
			return err
		}

		if focus == nil {
			fmt.Println("No focus set. Use `cp focus set <epic-id>` to set one.")
			return nil
		}

		progress, err := cpClient.GetEpicProgress(ctx, focus.EpicID)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"epic_id":    focus.EpicID,
				"epic_title": focus.EpicTitle,
				"set_at":     focus.SetAt.Format(time.RFC3339),
				"progress":   progress,
			}
			if focus.Note != "" {
				out["note"] = focus.Note
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		bar := renderProgressBar(progress.Completed, progress.Total, 10)
		fmt.Printf("Focus: %s (%s)\n", focus.EpicTitle, focus.EpicID)
		fmt.Printf("  Set: %s\n", timeAgo(focus.SetAt))
		if focus.Note != "" {
			fmt.Printf("  Note: %s\n", focus.Note)
		}
		fmt.Printf("  Progress: %s %d/%d complete\n", bar, progress.Completed, progress.Total)

		// Show in-progress and next items
		children, _ := cpClient.GetEpicChildren(ctx, focus.EpicID)
		for _, ch := range children {
			if ch.Status == "in_progress" && ch.Owner != nil {
				fmt.Printf("  In progress: %s %s (%s)\n", ch.ID, ch.Title, shortAgent(*ch.Owner))
			}
		}
		nextShards, _ := cpClient.GetNextShards(ctx, &focus.EpicID, 1)
		if len(nextShards) > 0 {
			fmt.Printf("  Next: %s %s\n", nextShards[0].ID, nextShards[0].Title)
		}

		return nil
	},
}

var focusSetCmd = &cobra.Command{
	Use:     "set <epic-id>",
	Short:   "Set active epic focus",
	Args:    cobra.ExactArgs(1),
	Example: "  cp focus set pf-abc123\n  cp focus set pf-abc123 --note \"afternoon work\"",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		epicID := args[0]
		noteFlag, _ := cmd.Flags().GetString("note")

		var note *string
		if noteFlag != "" {
			note = &noteFlag
		}

		err := cpClient.SetFocus(ctx, epicID, note)
		if err != nil {
			return err
		}

		// Get epic info for confirmation
		shard, err := cpClient.GetShard(ctx, epicID)
		if err != nil {
			return err
		}

		progress, _ := cpClient.GetEpicProgress(ctx, epicID)

		if outputFormat == "json" {
			out := map[string]any{
				"epic_id":    epicID,
				"epic_title": shard.Title,
				"set_at":     time.Now().UTC().Format(time.RFC3339),
				"progress":   progress,
			}
			if noteFlag != "" {
				out["note"] = noteFlag
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		bar := ""
		if progress != nil {
			bar = fmt.Sprintf(" %s %d/%d", renderProgressBar(progress.Completed, progress.Total, 10), progress.Completed, progress.Total)
		}
		fmt.Printf("Focus set: %s (%s)%s\n", shard.Title, epicID, bar)
		return nil
	},
}

var focusClearCmd = &cobra.Command{
	Use:     "clear",
	Short:   "Clear active epic focus",
	Example: "  cp focus clear",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		cleared, err := cpClient.ClearFocus(ctx)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{"cleared": cleared}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		if cleared {
			fmt.Println("Focus cleared.")
		} else {
			fmt.Println("No focus was set.")
		}
		return nil
	},
}

// timeAgo returns a human-readable duration since t
func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}

func init() {
	// focus set flags
	focusSetCmd.Flags().String("note", "", "Optional context note")

	// Wire command tree
	focusCmd.AddCommand(focusSetCmd)
	focusCmd.AddCommand(focusClearCmd)

	rootCmd.AddCommand(focusCmd)
}
