package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var bugCmd = &cobra.Command{
	Use:   "bug",
	Short: "Bug operations",
	Long:  `Commands for creating bug shards (defects).`,
}

var bugCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a bug (defect)",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp bug create "Missing names in output" --body "Names are null"
  cxp bug create "Crash on startup" --severity critical --parent pf-xxx`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		title := args[0]

		body, _ := cmd.Flags().GetString("body")
		bodyFile, _ := cmd.Flags().GetString("body-file")
		labels, _ := cmd.Flags().GetStringSlice("label")
		severity, _ := cmd.Flags().GetString("severity")
		parentID, _ := cmd.Flags().GetString("parent")

		if body != "" && bodyFile != "" {
			return fmt.Errorf("cannot use both --body and --body-file")
		}

		var content string
		if bodyFile != "" {
			data, err := os.ReadFile(bodyFile)
			if err != nil {
				return fmt.Errorf("cannot read file '%s': %v", bodyFile, err)
			}
			content = string(data)
		} else {
			content = body
		}

		// Validate severity if provided
		validSeverities := map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
		if severity != "" && !validSeverities[severity] {
			return fmt.Errorf("invalid severity %q. Valid: low, medium, high, critical", severity)
		}

		// Build metadata
		var metadata json.RawMessage
		if severity != "" {
			metadata, _ = json.Marshal(map[string]string{"severity": severity})
		}

		id, err := cpClient.CreateShardWithMetadata(ctx, title, content, "bug", nil, labels, metadata)
		if err != nil {
			return err
		}

		// Create child-of edge to parent
		if parentID != "" {
			if err := cpClient.CreateEdgeSimple(ctx, id, parentID, "child-of"); err != nil {
				return fmt.Errorf("created bug %s but failed to set parent: %v", id, err)
			}
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":         id,
				"type":       "bug",
				"title":      title,
				"created_at": time.Now().UTC().Format(time.RFC3339),
			}
			if severity != "" {
				out["severity"] = severity
			}
			if parentID != "" {
				out["parent"] = parentID
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Created bug %s: %q\n", id, title)
		if severity != "" {
			fmt.Printf("  Severity: %s\n", severity)
		}
		if parentID != "" {
			fmt.Printf("  Parent: %s\n", parentID)
		}
		return nil
	},
}

func init() {
	bugCreateCmd.Flags().String("body", "", "Bug content (inline)")
	bugCreateCmd.Flags().String("body-file", "", "Read content from file")
	bugCreateCmd.Flags().StringSlice("label", nil, "Labels (repeatable)")
	bugCreateCmd.Flags().String("severity", "", "Severity (low/medium/high/critical)")
	bugCreateCmd.Flags().String("parent", "", "Parent shard ID (creates child-of edge)")

	bugCmd.AddCommand(bugCreateCmd)
	rootCmd.AddCommand(bugCmd)
}
