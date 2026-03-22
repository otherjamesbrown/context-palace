package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var taskEvidenceCmd = &cobra.Command{
	Use:   "evidence <task-id>",
	Short: "Append structured evidence to a task",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp task evidence pf-123 --files "auth.go, token.go" --commit abc123
  cxp task evidence pf-123 --test-output "PASS ok 0.5s" --tokens 15000
  cxp task evidence pf-123 --decisions "Used retry with backoff" --body "All tests green"
  cxp task evidence pf-123 --test-output-file /tmp/test-results.txt`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		files, _ := cmd.Flags().GetString("files")
		testOutput, _ := cmd.Flags().GetString("test-output")
		testOutputFile, _ := cmd.Flags().GetString("test-output-file")
		commit, _ := cmd.Flags().GetString("commit")
		decisions, _ := cmd.Flags().GetString("decisions")
		tokens, _ := cmd.Flags().GetInt("tokens")
		body, _ := cmd.Flags().GetString("body")

		if testOutput != "" && testOutputFile != "" {
			return fmt.Errorf("cannot use both --test-output and --test-output-file")
		}

		// Read test output from file if specified
		if testOutputFile != "" {
			data, err := os.ReadFile(testOutputFile)
			if err != nil {
				return fmt.Errorf("cannot read file '%s': %v", testOutputFile, err)
			}
			testOutput = string(data)
		}

		// Ensure at least one flag is provided
		if files == "" && testOutput == "" && commit == "" && decisions == "" && tokens == 0 && body == "" {
			return fmt.Errorf("at least one evidence flag is required (--files, --test-output, --test-output-file, --commit, --decisions, --tokens, --body)")
		}

		// Build the markdown evidence block
		var sb strings.Builder
		sb.WriteString("\n## Evidence\n\n")

		if files != "" {
			sb.WriteString(fmt.Sprintf("**Files:** %s\n", files))
		}
		if commit != "" {
			sb.WriteString(fmt.Sprintf("**Commit:** %s\n", commit))
		}
		if tokens > 0 {
			sb.WriteString(fmt.Sprintf("**Tokens:** %d\n", tokens))
		}

		if testOutput != "" {
			sb.WriteString("\n### Test output\n")
			sb.WriteString(testOutput)
			if !strings.HasSuffix(testOutput, "\n") {
				sb.WriteString("\n")
			}
		}

		if decisions != "" {
			sb.WriteString("\n### Decisions\n")
			sb.WriteString(decisions)
			if !strings.HasSuffix(decisions, "\n") {
				sb.WriteString("\n")
			}
		}

		if body != "" {
			sb.WriteString("\n### Notes\n")
			sb.WriteString(body)
			if !strings.HasSuffix(body, "\n") {
				sb.WriteString("\n")
			}
		}

		content := sb.String()

		err := cpClient.AppendShardContent(ctx, id, content)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":       id,
				"appended": true,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Appended evidence to %s\n", id)
		return nil
	},
}

func init() {
	taskEvidenceCmd.Flags().String("files", "", "Files changed (comma-separated)")
	taskEvidenceCmd.Flags().String("test-output", "", "Test results (inline)")
	taskEvidenceCmd.Flags().String("test-output-file", "", "Read test output from file")
	taskEvidenceCmd.Flags().String("commit", "", "Commit hash")
	taskEvidenceCmd.Flags().String("decisions", "", "Implementation decisions")
	taskEvidenceCmd.Flags().Int("tokens", 0, "Token usage")
	taskEvidenceCmd.Flags().String("body", "", "Additional notes (free form)")

	taskCmd.AddCommand(taskEvidenceCmd)
}
