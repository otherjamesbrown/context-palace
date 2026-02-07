package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var messageCmd = &cobra.Command{
	Use:   "message",
	Short: "Agent messaging",
	Long:  `Commands for sending and reading agent messages.`,
}

var messageSendCmd = &cobra.Command{
	Use:   "send <recipient> <subject>",
	Short: "Send a message",
	Args:  cobra.ExactArgs(2),
	Example: `  cp message send agent-mycroft "Bug found in entity pipeline"
  cp message send agent-mycroft "Bug found" --body "Details here" --kind bug-report
  cp message send agent-mycroft "Re: Bug" --body "Looking into it" --reply-to pf-abc123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		recipients := strings.Split(args[0], ",")
		subject := args[1]

		body, _ := cmd.Flags().GetString("body")
		ccStr, _ := cmd.Flags().GetString("cc")
		kind, _ := cmd.Flags().GetString("kind")
		replyTo, _ := cmd.Flags().GetString("reply-to")

		var cc []string
		if ccStr != "" {
			cc = strings.Split(ccStr, ",")
		}

		id, err := cpClient.SendMessage(ctx, recipients, subject, body, cc, kind, replyTo)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"id": "%s"}`+"\n", id)
			return nil
		}

		fmt.Printf("Sent message %s to %s\n", id, strings.Join(recipients, ", "))
		return nil
	},
}

var messageInboxCmd = &cobra.Command{
	Use:     "inbox",
	Short:   "Show unread messages",
	Example: "  cp message inbox",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		messages, err := cpClient.GetInbox(ctx)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(messages)
			fmt.Println(s)
			return nil
		}

		if len(messages) == 0 {
			fmt.Println("No unread messages.")
			return nil
		}

		tbl := client.NewTable("ID", "FROM", "KIND", "DATE", "SUBJECT")
		for _, m := range messages {
			kind := ""
			if m.Kind != nil {
				kind = strings.TrimPrefix(*m.Kind, "kind:")
			}
			tbl.AddRow(m.ID, m.Creator, kind, m.CreatedAt.Format("01-02 15:04"), m.Title)
		}
		fmt.Print(tbl.String())
		return nil
	},
}

var messageShowCmd = &cobra.Command{
	Use:     "show <shard-id>",
	Short:   "Show a message",
	Args:    cobra.ExactArgs(1),
	Example: "  cp message show pf-abc123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		msg, err := cpClient.GetMessage(ctx, args[0])
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(msg)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("ID:      %s\n", msg.ID)
		fmt.Printf("From:    %s\n", msg.Creator)
		fmt.Printf("Date:    %s\n", msg.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Subject: %s\n", msg.Title)
		if msg.Content != "" {
			fmt.Printf("\n%s\n", msg.Content)
		}
		return nil
	},
}

var messageReadCmd = &cobra.Command{
	Use:     "read <shard-id> [shard-id...]",
	Short:   "Mark messages as read",
	Args:    cobra.MinimumNArgs(1),
	Example: "  cp message read pf-abc123 pf-def456",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		count, err := cpClient.MarkRead(ctx, args)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"marked_read": %d}`+"\n", count)
			return nil
		}

		fmt.Printf("Marked %d message(s) as read\n", count)
		return nil
	},
}

func init() {
	messageSendCmd.Flags().String("body", "", "Message body")
	messageSendCmd.Flags().String("cc", "", "CC recipients (comma-separated)")
	messageSendCmd.Flags().String("kind", "", "Message kind (e.g., bug-report, feature-request)")
	messageSendCmd.Flags().String("reply-to", "", "Shard ID to reply to")

	rootCmd.AddCommand(messageCmd)
	messageCmd.AddCommand(messageSendCmd)
	messageCmd.AddCommand(messageInboxCmd)
	messageCmd.AddCommand(messageShowCmd)
	messageCmd.AddCommand(messageReadCmd)
}
