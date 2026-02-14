package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var messageCmd = &cobra.Command{
	Use:   "message",
	Short: "Agent messaging",
	Long:  `Commands for sending and reading agent messages.`,
}

var messageSendCmd = &cobra.Command{
	Use:   "send [recipient] [subject] [body]",
	Short: "Send a message",
	Args:  cobra.RangeArgs(0, 3),
	Example: `  cp message send agent-mycroft "Subject" "Short body"
  cp message send --recipient agent-mycroft --subject "Subject" --body "Body"
  cp message send agent-mycroft "Bug found" --body "Details here" --kind bug-report
  cp message send agent-mycroft "Re: Bug" --body "Looking into it" --reply-to pf-abc123
  echo "Long body" | cp message send agent-mycroft "Subject"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		flagRecipient, _ := cmd.Flags().GetString("recipient")
		flagSubject, _ := cmd.Flags().GetString("subject")
		flagBody, _ := cmd.Flags().GetString("body")

		// Resolve recipient: positional arg takes precedence over flag
		var recipientStr string
		switch {
		case len(args) >= 1 && flagRecipient != "":
			return fmt.Errorf("cannot specify both positional recipient and --recipient flag")
		case len(args) >= 1:
			recipientStr = args[0]
		case flagRecipient != "":
			recipientStr = flagRecipient
		default:
			return fmt.Errorf("recipient is required: provide as 1st argument or --recipient flag")
		}
		recipients := strings.Split(recipientStr, ",")

		// Resolve subject: positional arg takes precedence over flag
		var subject string
		switch {
		case len(args) >= 2 && flagSubject != "":
			return fmt.Errorf("cannot specify both positional subject and --subject flag")
		case len(args) >= 2:
			subject = args[1]
		case flagSubject != "":
			subject = flagSubject
		default:
			return fmt.Errorf("subject is required: provide as 2nd argument or --subject flag")
		}

		positionalBody := ""
		if len(args) == 3 {
			positionalBody = args[2]
		}

		// Detect stdin: only read if not a terminal
		var stdinReader io.Reader
		if stat, _ := os.Stdin.Stat(); stat != nil && (stat.Mode()&os.ModeCharDevice) == 0 {
			stdinReader = os.Stdin
		}

		// Pre-flight validation: if body is already provided via positional arg or flag,
		// don't attempt stdin (prevents hanging in tmux/CI where stdin detection is unreliable)
		if positionalBody != "" || flagBody != "" {
			stdinReader = nil
		}

		// If no body source provided at all, fail immediately
		if positionalBody == "" && flagBody == "" && stdinReader == nil {
			return fmt.Errorf("message body is required: provide as 3rd argument, --body flag, or pipe to stdin")
		}

		// Warn if subject is suspiciously long (might have been intended as body)
		if len(subject) > 100 {
			fmt.Fprintf(os.Stderr, "Warning: subject is very long (%d chars). Did you mean to provide this as body?\n", len(subject))
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

	// Try stdin with a timeout to avoid hanging when os.Stdin.Stat() falsely
	// reports a pipe (common in tmux/CI environments).
	if stdin != nil {
		type readResult struct {
			data []byte
			err  error
		}
		ch := make(chan readResult, 1)
		go func() {
			data, err := io.ReadAll(stdin)
			ch <- readResult{data, err}
		}()
		select {
		case res := <-ch:
			if res.err != nil {
				return "", fmt.Errorf("reading stdin: %w", res.err)
			}
			body := strings.TrimSpace(string(res.data))
			if body != "" {
				return body, nil
			}
		case <-time.After(100 * time.Millisecond):
			// stdin detected but no data arrived — likely a false positive
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
	Short:   "Read messages and mark as read",
	Args:    cobra.MinimumNArgs(1),
	Example: "  cp message read pf-abc123 pf-def456",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Fetch and display each message
		var messages []*client.Message
		for _, id := range args {
			msg, err := cpClient.GetMessage(ctx, id)
			if err != nil {
				return fmt.Errorf("fetching message %s: %w", id, err)
			}
			messages = append(messages, msg)
		}

		// Mark all as read
		count, err := cpClient.MarkRead(ctx, args)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			type readResult struct {
				Messages  []*client.Message `json:"messages"`
				MarkedRead int             `json:"marked_read"`
			}
			s, _ := client.FormatJSON(readResult{Messages: messages, MarkedRead: count})
			fmt.Println(s)
			return nil
		}

		for i, msg := range messages {
			if i > 0 {
				fmt.Println("---")
			}
			fmt.Printf("ID:      %s\n", msg.ID)
			fmt.Printf("From:    %s\n", msg.Creator)
			fmt.Printf("Date:    %s\n", msg.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("Subject: %s\n", msg.Title)
			if msg.Content != "" {
				fmt.Printf("\n%s\n", msg.Content)
			}
		}
		fmt.Printf("\nMarked %d message(s) as read\n", count)
		return nil
	},
}

func init() {
	messageSendCmd.Flags().String("recipient", "", "Recipient agent(s) (comma-separated)")
	messageSendCmd.Flags().String("subject", "", "Message subject")
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
