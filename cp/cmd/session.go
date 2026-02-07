package cmd

import (
	"context"
	"fmt"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Work sessions",
	Long:  `Commands for managing work sessions — start, checkpoint, show, and end.`,
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

	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionStartCmd)
	sessionCmd.AddCommand(sessionCheckpointCmd)
	sessionCmd.AddCommand(sessionShowCmd)
	sessionCmd.AddCommand(sessionEndCmd)
}
