package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

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
	Short: "Create a new shard",
	Example: `  cp shard create --type design --title "Entity filter architecture" --body "## Design"
  cp shard create --type task --title "Fix bug" --body-file design.md --label urgent,backend
  cp shard create --type bug --title "Missing names" --meta '{"severity":"high"}'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		title, _ := cmd.Flags().GetString("title")
		shardType, _ := cmd.Flags().GetString("type")
		body, _ := cmd.Flags().GetString("body")
		bodyFile, _ := cmd.Flags().GetString("body-file")
		metaFlag, _ := cmd.Flags().GetString("meta")
		labelFlag, _ := cmd.Flags().GetString("label")

		if shardType == "" {
			return fmt.Errorf("--type is required")
		}
		if title == "" {
			return fmt.Errorf("--title is required")
		}
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

		var labels []string
		if labelFlag != "" {
			labels = strings.Split(labelFlag, ",")
		}

		var metadata json.RawMessage
		if metaFlag != "" {
			if !json.Valid([]byte(metaFlag)) {
				parsed, err := parseMetaFlag(metaFlag)
				if err != nil {
					return fmt.Errorf("invalid JSON in --meta: %v", err)
				}
				metadata, _ = json.Marshal(parsed)
			} else {
				metadata = json.RawMessage(metaFlag)
			}
		}

		id, err := cpClient.CreateShardWithMetadata(ctx, title, content, shardType, nil, labels, metadata)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			result := map[string]any{
				"id":         id,
				"type":       shardType,
				"title":      title,
				"created_at": time.Now().UTC().Format(time.RFC3339),
			}
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Created shard %s (%s)\n", id, shardType)
		return nil
	},
}

// -- shard list --

var shardListCmd = &cobra.Command{
	Use:   "list",
	Short: "List shards with filters",
	Example: `  cp shard list
  cp shard list --type task --status open
  cp shard list --type requirement,bug --label architecture
  cp shard list --search "timeout" --since 7d`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		typeFlag, _ := cmd.Flags().GetString("type")
		statusFlag, _ := cmd.Flags().GetString("status")
		labelFlag, _ := cmd.Flags().GetString("label")
		creatorFlag, _ := cmd.Flags().GetString("creator")
		searchFlag, _ := cmd.Flags().GetString("search")
		sinceFlag, _ := cmd.Flags().GetString("since")
		offset, _ := cmd.Flags().GetInt("offset")

		if limitFlag < 1 || limitFlag > 1000 {
			return fmt.Errorf("limit must be 1-1000")
		}

		opts := client.ListShardsOpts{
			Limit:  limitFlag,
			Offset: offset,
		}
		if typeFlag != "" {
			opts.Types = strings.Split(typeFlag, ",")
		}
		if statusFlag != "" {
			opts.Status = strings.Split(statusFlag, ",")
		}
		if labelFlag != "" {
			opts.Labels = strings.Split(labelFlag, ",")
		}
		if creatorFlag != "" {
			opts.Creator = creatorFlag
		}
		if searchFlag != "" {
			opts.Search = searchFlag
		}
		if sinceFlag != "" {
			cutoff, err := parseSince(sinceFlag)
			if err != nil {
				return err
			}
			opts.Since = &cutoff
		}

		results, err := cpClient.ListShardsFiltered(ctx, opts)
		if err != nil {
			return err
		}

		total, err := cpClient.ListShardsCount(ctx, opts)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"total":   total,
				"offset":  offset,
				"limit":   limitFlag,
				"results": results,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		if len(results) == 0 {
			fmt.Println("No shards found.")
			return nil
		}

		tbl := client.NewTable("ID", "TYPE", "STATUS", "CREATED", "TITLE")
		for _, r := range results {
			tbl.AddRow(r.ID, r.Type, r.Status, r.CreatedAt.Format("2006-01-02"), client.Truncate(r.Title, 50))
		}
		fmt.Print(tbl.String())

		start := offset + 1
		end := offset + len(results)
		fmt.Printf("\nShowing %d-%d of %d results\n", start, end, total)
		return nil
	},
}

// -- shard show --

var shardShowCmd = &cobra.Command{
	Use:     "show <shard-id>",
	Short:   "Show shard detail",
	Args:    cobra.ExactArgs(1),
	Example: "  cp shard show pf-c74eea",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		detail, err := cpClient.GetShardDetail(ctx, id)
		if err != nil {
			return err
		}

		// Get edges
		edges, err := cpClient.GetShardEdges(ctx, id, "", nil)
		if err != nil {
			return err
		}
		detail.Edges = edges

		if outputFormat == "json" {
			s, _ := client.FormatJSON(detail)
			fmt.Println(s)
			return nil
		}

		// Text output
		fmt.Println(detail.Title)
		fmt.Println(strings.Repeat("─", len(detail.Title)))
		fmt.Printf("ID:       %s\n", detail.ID)
		fmt.Printf("Type:     %s\n", detail.Type)
		fmt.Printf("Status:   %s\n", detail.Status)
		fmt.Printf("Creator:  %s\n", detail.Creator)
		fmt.Printf("Created:  %s\n", detail.CreatedAt.Format("2006-01-02 15:04"))

		if len(detail.Labels) > 0 {
			fmt.Printf("Labels:   %s\n", strings.Join(detail.Labels, ", "))
		}

		// Metadata
		if detail.Metadata != nil && string(detail.Metadata) != "{}" {
			fmt.Println("\nMetadata:")
			var meta map[string]any
			if json.Unmarshal(detail.Metadata, &meta) == nil {
				for k, v := range meta {
					fmt.Printf("  %s: %v\n", k, v)
				}
			}
		}

		// Content
		if detail.Content != "" {
			fmt.Println("\nContent:")
			fmt.Printf("  %s\n", detail.Content)
		}

		// Edges
		if len(edges) > 0 {
			fmt.Println("\nEdges:")
			tbl := client.NewTable("DIRECTION", "EDGE TYPE", "SHARD", "TYPE", "TITLE")
			for _, e := range edges {
				tbl.AddRow(e.Direction, e.EdgeType, e.ShardID, e.Type, client.Truncate(e.Title, 40))
			}
			fmt.Print("  ")
			fmt.Print(strings.ReplaceAll(tbl.String(), "\n", "\n  "))
		}

		return nil
	},
}

// -- shard update --

var shardUpdateCmd = &cobra.Command{
	Use:   "update <shard-id>",
	Short: "Update shard content or title",
	Args:  cobra.ExactArgs(1),
	Example: `  cp shard update pf-abc123 --body "Updated content"
  cp shard update pf-abc123 --body-file updated.md
  cp shard update pf-abc123 --title "New Title"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		body, _ := cmd.Flags().GetString("body")
		bodyFile, _ := cmd.Flags().GetString("body-file")
		title, _ := cmd.Flags().GetString("title")

		if body == "" && bodyFile == "" && title == "" {
			return fmt.Errorf("at least one of --body, --body-file, or --title is required")
		}
		if body != "" && bodyFile != "" {
			return fmt.Errorf("cannot use both --body and --body-file")
		}

		var contentPtr *string
		if bodyFile != "" {
			data, err := os.ReadFile(bodyFile)
			if err != nil {
				return fmt.Errorf("cannot read file '%s': %v", bodyFile, err)
			}
			content := string(data)
			contentPtr = &content
		} else if body != "" {
			contentPtr = &body
		}

		var titlePtr *string
		if title != "" {
			titlePtr = &title
		}

		result, err := cpClient.UpdateShardFields(ctx, id, titlePtr, contentPtr)
		if err != nil {
			return err
		}

		// Warn if knowledge doc
		if result.ShardType == "knowledge" {
			fmt.Fprintf(os.Stderr, "Warning: This is a knowledge document. Use `cp knowledge update` to preserve version history.\n")
		}

		if outputFormat == "json" {
			var updatedFields []string
			if result.ContentChanged {
				updatedFields = append(updatedFields, "content")
			}
			if result.TitleChanged {
				updatedFields = append(updatedFields, "title")
			}
			out := map[string]any{
				"id":             id,
				"updated_fields": updatedFields,
				"updated_at":     result.UpdatedAt.Format(time.RFC3339),
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Updated %s\n", id)
		return nil
	},
}

// -- shard close --

var shardCloseCmd = &cobra.Command{
	Use:     "close <shard-id>",
	Short:   "Close a shard with optional reason",
	Args:    cobra.ExactArgs(1),
	Example: "  cp shard close pf-abc123\n  cp shard close pf-abc123 --reason \"Done: implemented and tested\"",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		reason, _ := cmd.Flags().GetString("reason")

		result, err := cpClient.CloseShard(ctx, id, reason)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":        result.ID,
				"status":    result.Status,
				"closed_at": result.ClosedAt.Format(time.RFC3339),
			}
			if result.Reason != "" {
				out["reason"] = result.Reason
			}
			if len(result.Unblocked) > 0 {
				var ids []string
				for _, u := range result.Unblocked {
					ids = append(ids, u.ID)
				}
				out["unblocked"] = ids
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Closed %s %q\n", result.ID, result.Title)
		if result.Reason != "" {
			fmt.Printf("  Reason: %s\n", result.Reason)
		}
		for _, u := range result.Unblocked {
			fmt.Printf("  Unblocked: %s %q\n", u.ID, u.Title)
		}
		return nil
	},
}

// -- shard reopen --

var shardReopenCmd = &cobra.Command{
	Use:     "reopen <shard-id>",
	Short:   "Reopen a closed shard",
	Args:    cobra.ExactArgs(1),
	Example: "  cp shard reopen pf-abc123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		// Check current status
		shard, err := cpClient.GetShard(ctx, id)
		if err != nil {
			return fmt.Errorf("Shard %s not found", id)
		}
		if shard.Status == "open" {
			fmt.Printf("Shard %s is already open.\n", id)
			return nil
		}

		err = cpClient.UpdateShardStatus(ctx, id, "open")
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":         id,
				"status":     "open",
				"updated_at": time.Now().UTC().Format(time.RFC3339),
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Reopened %s\n", id)
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
	shardCreateCmd.Flags().String("type", "", "Shard type (required)")
	shardCreateCmd.Flags().String("body", "", "Inline content")
	shardCreateCmd.Flags().String("body-file", "", "Content from file")
	shardCreateCmd.Flags().String("label", "", "Comma-separated labels")
	shardCreateCmd.Flags().String("meta", "", "Metadata as JSON")

	// shard list flags
	shardListCmd.Flags().String("type", "", "Comma-separated shard types")
	shardListCmd.Flags().String("status", "", "Comma-separated statuses")
	shardListCmd.Flags().String("label", "", "Comma-separated labels (OR)")
	shardListCmd.Flags().String("creator", "", "Filter by creator")
	shardListCmd.Flags().String("search", "", "Text search (tsvector)")
	shardListCmd.Flags().String("since", "", "Time filter: duration or date")
	shardListCmd.Flags().Int("offset", 0, "Skip N results for pagination")

	// shard update flags
	shardUpdateCmd.Flags().String("body", "", "New content (inline)")
	shardUpdateCmd.Flags().String("body-file", "", "New content (from file)")
	shardUpdateCmd.Flags().String("title", "", "New title")

	// shard close flags
	shardCloseCmd.Flags().String("reason", "", "Closure reason")

	// Wire command tree
	shardMetadataCmd.AddCommand(shardMetadataGetCmd)
	shardMetadataCmd.AddCommand(shardMetadataSetCmd)
	shardMetadataCmd.AddCommand(shardMetadataDeleteCmd)

	shardCmd.AddCommand(shardMetadataCmd)
	shardCmd.AddCommand(shardQueryCmd)
	shardCmd.AddCommand(shardCreateCmd)
	shardCmd.AddCommand(shardListCmd)
	shardCmd.AddCommand(shardShowCmd)
	shardCmd.AddCommand(shardUpdateCmd)
	shardCmd.AddCommand(shardCloseCmd)
	shardCmd.AddCommand(shardReopenCmd)

	rootCmd.AddCommand(shardCmd)
}
