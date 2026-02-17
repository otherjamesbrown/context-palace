package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Work sessions",
	Long:  `Commands for managing work sessions — start, checkpoint, show, and end.`,
}

var sessionEnsureCmd = &cobra.Command{
	Use:     "ensure",
	Short:   "Ensure an open session exists (idempotent)",
	Example: "  cp session ensure\n  cp session ensure -o json",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		result, err := cpClient.EnsureSession(ctx)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		fmt.Println(result.ID)
		return nil
	},
}

var sessionStartCmd = &cobra.Command{
	Use:     "start [title]",
	Short:   "Start a new work session",
	Args:    cobra.MaximumNArgs(1),
	Example: "  cp session start\n  cp session start \"Debugging timeout issues\"",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		title := ""
		if len(args) > 0 {
			title = args[0]
		}

		id, err := cpClient.StartSession(ctx, title)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"id": "%s"}`+"\n", id)
			return nil
		}

		fmt.Printf("Started session %s\n", id)
		return nil
	},
}

var sessionCheckpointCmd = &cobra.Command{
	Use:     "checkpoint <note>",
	Short:   "Add a checkpoint to the current session",
	Args:    cobra.ExactArgs(1),
	Example: `  cp session checkpoint "Found root cause: hardcoded timeout"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		sessionID, _ := cmd.Flags().GetString("session")
		if sessionID == "" {
			// Find current open session
			session, err := cpClient.GetCurrentSession(ctx)
			if err != nil {
				return fmt.Errorf("no open session. Start one with: cp session start")
			}
			sessionID = session.ID
		}

		err := cpClient.Checkpoint(ctx, sessionID, args[0])
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"success": true, "session_id": "%s"}`+"\n", sessionID)
			return nil
		}

		fmt.Printf("Added checkpoint to %s\n", sessionID)
		return nil
	},
}

var sessionShowCmd = &cobra.Command{
	Use:     "show [session-id]",
	Short:   "Show a session",
	Args:    cobra.MaximumNArgs(1),
	Example: "  cp session show\n  cp session show pf-abc123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var session *client.Session
		var err error

		if len(args) > 0 {
			session, err = cpClient.ShowSession(ctx, args[0])
		} else {
			session, err = cpClient.GetCurrentSession(ctx)
		}
		if err != nil {
			if errors.Is(err, client.ErrNoSession) {
				if outputFormat == "json" {
					fmt.Println(`{"session": null}`)
				} else {
					fmt.Println("No open session.")
				}
				return nil
			}
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(session)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("ID:      %s\n", session.ID)
		fmt.Printf("Title:   %s\n", session.Title)
		fmt.Printf("Status:  %s\n", session.Status)
		fmt.Printf("Started: %s\n", session.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Updated: %s\n", session.UpdatedAt.Format("2006-01-02 15:04:05"))
		if session.Content != "" {
			fmt.Printf("\n%s\n", session.Content)
		}
		return nil
	},
}

var sessionBoardCmd = &cobra.Command{
	Use:   "board",
	Short: "Show open shards grouped by area with token estimates",
	Example: `  cp session board
  cp session board --since 7d
  cp session board --area Pipeline
  cp session board -o json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		opts := client.BoardOpts{}

		sinceStr, _ := cmd.Flags().GetString("since")
		if sinceStr != "" {
			d, err := parseDuration(sinceStr)
			if err != nil {
				return fmt.Errorf("invalid --since value: %v", err)
			}
			t := time.Now().Add(-d)
			opts.Since = &t
		}

		opts.Area, _ = cmd.Flags().GetString("area")
		opts.Agent, _ = cmd.Flags().GetString("agent")
		opts.Budget, _ = cmd.Flags().GetInt("budget")

		result, err := cpClient.GetBoardShards(ctx, opts)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		if len(result.Groups) == 0 {
			fmt.Println("No shards to display.")
		} else {
			for i, g := range result.Groups {
				if i > 0 {
					fmt.Println()
				}
				fmt.Printf("# Current Open %s (%d items, ~%s) #\n\n",
					strings.ToUpper(g.Type+"s"), len(g.Items), formatTokens(g.TotalTokens))
				for _, e := range g.Items {
					priority := ""
					if e.Priority != nil {
						switch *e.Priority {
						case 0:
							priority = "CRIT"
						case 1:
							priority = "HIGH"
						}
					}
					status := e.Status
					fmt.Printf("  %-100s  %5dt  %-4s  %-6s  %s\n",
						client.Truncate(e.Title, 100),
						e.TokenEstimate, priority, status, e.ID)
				}
			}
		}

		fmt.Println()
		fmt.Printf("Inbox: %d unread messages\n", result.UnreadCount)
		fmt.Printf("Memories: %d active\n", result.MemoryCount)
		return nil
	},
}

// parseDuration parses duration strings like "7d", "24h", "30d"
func parseDuration(s string) (time.Duration, error) {
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

// formatTokens formats token count as "1.2k" or "450"
func formatTokens(t int) string {
	if t >= 1000 {
		return fmt.Sprintf("%.1fk tokens", float64(t)/1000)
	}
	return fmt.Sprintf("%dt", t)
}

var sessionContextCmd = &cobra.Command{
	Use:     "context <shard-id> [shard-id...]",
	Short:   "Load shards as compact context blocks",
	Args:    cobra.MinimumNArgs(1),
	Example: "  cp session context pf-4b7bca pf-7313ed",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		totalTokens := 0
		for i, id := range args {
			shard, err := cpClient.GetShard(ctx, id)
			if err != nil {
				return err
			}

			if i > 0 {
				fmt.Println()
			}

			// Header: --- pf-xxx (type, status, PRIORITY) ---
			header := fmt.Sprintf("--- %s (%s, %s", shard.ID, shard.Type, shard.Status)
			if shard.Priority != nil {
				switch *shard.Priority {
				case 0:
					header += ", CRIT"
				case 1:
					header += ", HIGH"
				}
			}
			header += ") ---"
			fmt.Println(header)

			// Title
			fmt.Println(shard.Title)

			// Content
			if shard.Content != "" {
				fmt.Println(shard.Content)
			}

			tokens := client.EstimateTokens(shard.Content)
			totalTokens += tokens
		}

		fmt.Printf("\n[%d shards, ~%s]\n", len(args), formatTokens(totalTokens))
		return nil
	},
}

var sessionInjectCmd = &cobra.Command{
	Use:     "inject",
	Short:   "Output formatted context for Claude Code hooks",
	Example: "  cp session inject\n  cp session inject --tag main",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tag, _ := cmd.Flags().GetString("tag")

		output, err := cpClient.InjectContext(ctx, tag)
		if err != nil {
			return err
		}

		fmt.Print(output)
		return nil
	},
}

var sessionLastCheckpointCmd = &cobra.Command{
	Use:     "last-checkpoint",
	Short:   "Show the last checkpoint from the current session",
	Example: "  cp session last-checkpoint\n  cp session last-checkpoint --tag main\n  cp session last-checkpoint -o json",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tag, _ := cmd.Flags().GetString("tag")

		entry, err := cpClient.LastCheckpoint(ctx, tag)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(entry)
			fmt.Println(s)
			return nil
		}

		if entry.Tag != "" {
			fmt.Printf("[%s] at %s:\n", entry.Tag, entry.Timestamp)
		} else {
			fmt.Printf("at %s:\n", entry.Timestamp)
		}
		fmt.Println(entry.Content)
		return nil
	},
}

var sessionEndCmd = &cobra.Command{
	Use:     "end [session-id]",
	Short:   "End a session",
	Args:    cobra.MaximumNArgs(1),
	Example: "  cp session end\n  cp session end pf-abc123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		sessionID := ""
		if len(args) > 0 {
			sessionID = args[0]
		} else {
			session, err := cpClient.GetCurrentSession(ctx)
			if err != nil {
				return fmt.Errorf("no open session to end")
			}
			sessionID = session.ID
		}

		err := cpClient.EndSession(ctx, sessionID)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"success": true, "session_id": "%s", "status": "closed"}`+"\n", sessionID)
			return nil
		}

		fmt.Printf("Ended session %s\n", sessionID)
		return nil
	},
}

func init() {
	sessionCheckpointCmd.Flags().String("session", "", "Session ID (default: current open session)")
	sessionInjectCmd.Flags().String("tag", "", "Filter by checkpoint tag")
	sessionLastCheckpointCmd.Flags().String("tag", "", "Filter by checkpoint tag")
	sessionBoardCmd.Flags().String("since", "", "Include recently closed shards (e.g., 7d, 24h)")
	sessionBoardCmd.Flags().String("area", "", "Filter to area prefix")
	sessionBoardCmd.Flags().String("agent", "", "Filter by creator agent")
	sessionBoardCmd.Flags().Int("budget", 0, "Token budget highlight threshold")

	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionBoardCmd)
	sessionCmd.AddCommand(sessionContextCmd)
	sessionCmd.AddCommand(sessionEnsureCmd)
	sessionCmd.AddCommand(sessionStartCmd)
	sessionCmd.AddCommand(sessionCheckpointCmd)
	sessionCmd.AddCommand(sessionShowCmd)
	sessionCmd.AddCommand(sessionInjectCmd)
	sessionCmd.AddCommand(sessionLastCheckpointCmd)
	sessionCmd.AddCommand(sessionEndCmd)
}
