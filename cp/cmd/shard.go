package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var shardCmd = &cobra.Command{
	Use:   "shard",
	Short: "Shard operations",
	Long:  `Commands for shard metadata, querying, and creation.`,
}

// -- shard metadata --

var shardMetadataCmd = &cobra.Command{
	Use:   "metadata",
	Short: "Shard metadata operations",
	Long:  `Get, set, and delete metadata fields on shards.`,
}

var shardMetadataGetCmd = &cobra.Command{
	Use:     "get <shard-id> [field]",
	Short:   "Get shard metadata",
	Args:    cobra.RangeArgs(1, 2),
	Example: "  cp shard metadata get pf-123\n  cp shard metadata get pf-123 lifecycle_status\n  cp shard metadata get pf-123 test_coverage.unit",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		if len(args) == 1 {
			// Get all metadata
			meta, err := cpClient.GetMetadata(ctx, id)
			if err != nil {
				return err
			}

			if outputFormat == "json" {
				fmt.Println(string(meta))
				return nil
			}

			// Pretty-print JSON
			var pretty json.RawMessage
			if json.Unmarshal(meta, &pretty) == nil {
				b, _ := json.MarshalIndent(pretty, "", "  ")
				fmt.Println(string(b))
			} else {
				fmt.Println(string(meta))
			}
			return nil
		}

		// Get specific field by dot-path
		path := parseDotPath(args[1])
		value, err := cpClient.GetMetadataField(ctx, id, path)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			// Try to output as JSON value
			if json.Valid([]byte(value)) {
				fmt.Println(value)
			} else {
				b, _ := json.Marshal(value)
				fmt.Println(string(b))
			}
			return nil
		}

		fmt.Println(value)
		return nil
	},
}

var shardMetadataSetCmd = &cobra.Command{
	Use:     "set <shard-id> <key> <value>",
	Short:   "Set a metadata field",
	Args:    cobra.ExactArgs(3),
	Example: "  cp shard metadata set pf-123 lifecycle_status approved\n  cp shard metadata set pf-123 test_coverage.unit 5\n  cp shard metadata set pf-123 test_coverage '{\"unit\": 5, \"integration\": 2}'",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]
		key := args[1]
		rawValue := args[2]

		path := parseDotPath(key)

		// Convert the value to JSON
		jsonValue, err := toJSONValue(rawValue)
		if err != nil {
			return fmt.Errorf("invalid value: %v", err)
		}

		result, err := cpClient.SetMetadataPath(ctx, id, path, jsonValue)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Println(string(result))
			return nil
		}

		fmt.Printf("Updated metadata for %s\n", id)
		return nil
	},
}

var shardMetadataDeleteCmd = &cobra.Command{
	Use:     "delete <shard-id> <key>",
	Short:   "Delete a metadata key",
	Args:    cobra.ExactArgs(2),
	Example: "  cp shard metadata delete pf-123 deprecated_field",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]
		key := args[1]

		result, err := cpClient.DeleteMetadataKey(ctx, id, key)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Println(string(result))
			return nil
		}

		fmt.Printf("Deleted key '%s' from %s\n", key, id)
		return nil
	},
}

// -- shard query --

var shardQueryCmd = &cobra.Command{
	Use:     "query",
	Short:   "Query shards by type and metadata",
	Example: "  cp shard query --type requirement --meta \"lifecycle_status=approved\"\n  cp shard query --meta \"priority=1\"",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		shardType, _ := cmd.Flags().GetString("type")
		metaFlag, _ := cmd.Flags().GetString("meta")

		var metaFilters map[string]interface{}
		if metaFlag != "" {
			var err error
			metaFilters, err = parseMetaFlag(metaFlag)
			if err != nil {
				return err
			}
		}

		shards, err := cpClient.QueryByMetadata(ctx, shardType, metaFilters, limitFlag)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(shards)
			fmt.Println(s)
			return nil
		}

		if len(shards) == 0 {
			fmt.Println("No shards found.")
			return nil
		}

		tbl := client.NewTable("ID", "TYPE", "STATUS", "TITLE")
		for _, s := range shards {
			tbl.AddRow(s.ID, s.Type, s.Status, client.Truncate(s.Title, 50))
		}
		fmt.Print(tbl.String())
		return nil
	},
}

// -- shard create --

var shardCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new shard with metadata",
	Example: `  cp shard create --type requirement --title "Test Req" --meta '{"lifecycle_status":"draft","priority":2}'
  cp shard create --type task --title "Fix bug" --priority 1 --label urgent --label backend`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		title, _ := cmd.Flags().GetString("title")
		shardType, _ := cmd.Flags().GetString("type")
		content, _ := cmd.Flags().GetString("content")
		metaFlag, _ := cmd.Flags().GetString("meta")
		labels, _ := cmd.Flags().GetStringSlice("label")

		if title == "" {
			return fmt.Errorf("--title is required")
		}

		var pri *int
		if cmd.Flags().Changed("priority") {
			p, _ := cmd.Flags().GetInt("priority")
			pri = &p
		}

		var metadata json.RawMessage
		if metaFlag != "" {
			// Validate JSON
			if !json.Valid([]byte(metaFlag)) {
				// Try parsing as key=value pairs
				parsed, err := parseMetaFlag(metaFlag)
				if err != nil {
					return fmt.Errorf("invalid JSON in --meta: %v", err)
				}
				metadata, _ = json.Marshal(parsed)
			} else {
				metadata = json.RawMessage(metaFlag)
			}

			// Check size (1MB limit)
			if len(metadata) > 1024*1024 {
				return fmt.Errorf("metadata too large (%.1fMB). Maximum 1MB", float64(len(metadata))/(1024*1024))
			}
		}

		id, err := cpClient.CreateShardWithMetadata(ctx, title, content, shardType, pri, labels, metadata)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"id": "%s"}`+"\n", id)
			return nil
		}

		fmt.Printf("Created shard %s\n", id)
		return nil
	},
}

// toJSONValue converts a CLI string value to a JSON-encoded value
func toJSONValue(s string) (json.RawMessage, error) {
	// If it's already valid JSON (object, array, or quoted string), use as-is
	if json.Valid([]byte(s)) {
		var v interface{}
		if json.Unmarshal([]byte(s), &v) == nil {
			// Check if it's a complex type (object/array)
			if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") || strings.HasPrefix(s, "\"") {
				return json.RawMessage(s), nil
			}
		}
	}

	// Infer type and marshal
	val := inferJSONValue(s)
	b, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// parseDotPath and parseMetaFlag are defined in client/metadata.go
// but we need local wrappers for the cmd package

func parseDotPath(s string) []string {
	return strings.Split(s, ".")
}

func parseMetaFlag(s string) (map[string]interface{}, error) {
	s = strings.TrimSpace(s)

	// Try JSON first
	if strings.HasPrefix(s, "{") {
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(s), &result); err != nil {
			return nil, fmt.Errorf("invalid JSON: %v", err)
		}
		return result, nil
	}

	// Try key=value
	parts := strings.SplitN(s, "=", 2)
	if len(parts) == 2 && parts[0] != "" {
		return map[string]interface{}{
			parts[0]: inferJSONValue(parts[1]),
		}, nil
	}

	return nil, fmt.Errorf("invalid metadata format. Use key=value or JSON")
}

func inferJSONValue(s string) interface{} {
	// Try integer
	if i, err := fmt.Sscanf(s, "%d", new(int)); err == nil && i == 1 {
		var v int64
		fmt.Sscanf(s, "%d", &v)
		// Make sure the entire string was consumed
		if fmt.Sprintf("%d", v) == s {
			return v
		}
	}
	// Try float
	if strings.Contains(s, ".") {
		var f float64
		if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
			if fmt.Sprintf("%g", f) == s || fmt.Sprintf("%f", f) == s {
				return f
			}
		}
	}
	// Try bool
	switch strings.ToLower(s) {
	case "true":
		return true
	case "false":
		return false
	}
	// Default to string
	return s
}

func init() {
	// shard query flags
	shardQueryCmd.Flags().String("type", "", "Shard type filter")
	shardQueryCmd.Flags().String("meta", "", "Metadata filter (key=value or JSON)")

	// shard create flags
	shardCreateCmd.Flags().String("title", "", "Shard title (required)")
	shardCreateCmd.Flags().String("type", "", "Shard type")
	shardCreateCmd.Flags().String("content", "", "Shard content")
	shardCreateCmd.Flags().String("meta", "", "Metadata as JSON")
	shardCreateCmd.Flags().Int("priority", 0, "Priority (0-4)")
	shardCreateCmd.Flags().StringSlice("label", nil, "Labels (repeatable)")

	// Wire command tree
	shardMetadataCmd.AddCommand(shardMetadataGetCmd)
	shardMetadataCmd.AddCommand(shardMetadataSetCmd)
	shardMetadataCmd.AddCommand(shardMetadataDeleteCmd)

	shardCmd.AddCommand(shardMetadataCmd)
	shardCmd.AddCommand(shardQueryCmd)
	shardCmd.AddCommand(shardCreateCmd)

	rootCmd.AddCommand(shardCmd)
}
