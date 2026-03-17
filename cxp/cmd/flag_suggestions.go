package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// flagSuggestion maps an unknown flag to a helpful message, scoped to a command path.
type flagSuggestion struct {
	cmdPath string // e.g. "shard update", "shard link", "" for any
	flag    string // the unknown flag name (without --)
	message string
}

var flagSuggestions = []flagSuggestion{
	{"shard update", "append", "To append content use: cxp shard append <id> --body \"text\""},
	{"shard update", "label", "To manage labels use: cxp shard label add <id> <labels>"},
	{"shard update", "content", "Did you mean --body?"},
	{"shard link", "type", "Edge types are flags: cxp shard link <from> --child-of <to>\n  Or positional: cxp shard link <from> child-of <to>"},
	{"knowledge update", "change-summary", "Did you mean --summary?"},
	{"knowledge update", "content", "Did you mean --body?"},
	{"message send", "to", "Recipient is a positional arg: cxp message send <recipient> <subject>"},
	// Generic (match any command)
	{"", "content", "Did you mean --body?"},
}

// setupFlagSuggestions registers a flag error function on a command that provides
// "did you mean?" hints for common mistakes.
func setupFlagSuggestions(cmd *cobra.Command) {
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		// Extract flag name from error like "unknown flag: --something"
		flagName := extractUnknownFlag(err.Error())
		if flagName == "" {
			return err
		}

		// Build command path for matching (e.g. "shard update")
		cmdPath := buildCmdPath(c)

		// Check for matching suggestion
		for _, s := range flagSuggestions {
			if s.flag != flagName {
				continue
			}
			if s.cmdPath != "" && s.cmdPath != cmdPath {
				continue
			}
			return fmt.Errorf("unknown flag: --%s\n  Hint: %s", flagName, s.message)
		}

		return err
	})
}

func init() {
	// Register flag suggestions on commands where agents commonly use wrong flags
	setupFlagSuggestions(shardUpdateCmd)
	setupFlagSuggestions(shardLinkCmd)
	setupFlagSuggestions(shardCloseCmd)
	setupFlagSuggestions(kdUpdateCmd)
	setupFlagSuggestions(messageSendCmd)
}

// extractUnknownFlag pulls the flag name from cobra's "unknown flag: --xxx" error.
func extractUnknownFlag(msg string) string {
	// Cobra formats: "unknown flag: --flagname" or "unknown shorthand flag: '-x'"
	if strings.HasPrefix(msg, "unknown flag: --") {
		return strings.TrimPrefix(msg, "unknown flag: --")
	}
	return ""
}

// buildCmdPath returns the command path relative to root, e.g. "shard update".
func buildCmdPath(c *cobra.Command) string {
	parts := []string{}
	for p := c; p != nil && p.Parent() != nil; p = p.Parent() {
		parts = append([]string{p.Name()}, parts...)
	}
	return strings.Join(parts, " ")
}
