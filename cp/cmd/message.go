package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
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
	Use:   "send <recipient> <subject> [body]",
	Short: "Send a message",
	Args:  cobra.RangeArgs(2, 3),
	Example: `  cp message send agent-mycroft "Subject" "Short body"
  cp message send agent-mycroft "Bug found" --body "Details here" --kind bug-report
  cp message send agent-mycroft "Re: Bug" --body "Looking into it" --reply-to pf-abc123
  echo "Long body" | cp message send agent-mycroft "Subject"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		recipients := strings.Split(args[0], ",")
		subject := args[1]

		positionalBody := ""
		if len(args) == 3 {
			positionalBody = args[2]
		}
		flagBody, _ := cmd.Flags().GetString("body")

		// Detect stdin: only read if not a terminal
		var stdinReader io.Reader
		if stat, _ := os.Stdin.Stat(); stat != nil && (stat.Mode()&os.ModeCharDevice) == 0 {
			stdinReader = os.Stdin
		}

		body, err := resolveMessageBody(positionalBody, flagBody, stdinReader)
		if err != nil {
			return err
		}

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

// resolveMessageBody determines the message body from three sources in priority order:
// 1. Positional argument (3rd arg)
// 2. --body flag
// 3. stdin (when piped)
// Returns an error if both positional and flag are provided, or if no body is available.
func resolveMessageBody(positional, flag string, stdin io.Reader) (string, error) {
	positional = strings.TrimSpace(positional)
	flag = strings.TrimSpace(flag)

	if positional != "" && flag != "" {
		return "", fmt.Errorf("cannot specify both positional body argument and --body flag")
	}

	if positional != "" {
		return positional, nil
	}
	if flag != "" {
		return flag, nil
	}

	// Try stdin
	if stdin != nil {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		body := strings.TrimSpace(string(data))
		if body != "" {
			return body, nil
		}
	}

	return "", fmt.Errorf("message body is required: provide as 3rd argument, --body flag, or pipe to stdin")
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
