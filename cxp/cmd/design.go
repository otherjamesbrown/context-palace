package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var designCmd = &cobra.Command{
	Use:   "design",
	Short: "Design operations",
	Long:  `Commands for creating design shards (plans, features, specs).`,
}

var designCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a design (plan/feature)",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp design create "Auth redesign" --body "## Overview"
  cxp design create "Auth redesign" --body-file design.md --parent pf-xxx
  cxp design create "Auth redesign" --adopt pf-a,pf-b --order "pf-b:pf-a"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		title := args[0]

		body, _ := cmd.Flags().GetString("body")
		bodyFile, _ := cmd.Flags().GetString("body-file")
		labels, _ := cmd.Flags().GetStringSlice("label")
		parentID, _ := cmd.Flags().GetString("parent")
		adoptFlag, _ := cmd.Flags().GetString("adopt")
		orderFlag, _ := cmd.Flags().GetString("order")

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

		var adoptIDs []string
		if adoptFlag != "" {
			adoptIDs = strings.Split(adoptFlag, ",")
			for i := range adoptIDs {
				adoptIDs[i] = strings.TrimSpace(adoptIDs[i])
			}
		}

		var orderEdges []client.OrderEdge
		if orderFlag != "" {
			var err error
			orderEdges, err = parseOrderFlag(orderFlag, adoptIDs)
			if err != nil {
				return err
			}
		}

		id, err := cpClient.CreateDesign(ctx, title, content, labels, parentID, adoptIDs, orderEdges)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":    id,
				"type":  "design",
				"title": title,
			}
			if parentID != "" {
				out["parent"] = parentID
			}
			if len(adoptIDs) > 0 {
				out["adopted"] = adoptIDs
			}
			if len(orderEdges) > 0 {
				out["edges"] = orderEdges
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Created design %s: %q\n", id, title)
		if parentID != "" {
			fmt.Printf("  Parent: %s\n", parentID)
		}
		if len(adoptIDs) > 0 {
			fmt.Printf("  Adopted %d shards\n", len(adoptIDs))
		}
		if len(orderEdges) > 0 {
			fmt.Printf("  Set %d dependency edges\n", len(orderEdges))
		}
		return nil
	},
}

func init() {
	designCreateCmd.Flags().String("body", "", "Design content (inline)")
	designCreateCmd.Flags().String("body-file", "", "Read content from file")
	designCreateCmd.Flags().StringSlice("label", nil, "Labels (repeatable)")
	designCreateCmd.Flags().String("parent", "", "Parent shard ID (creates child-of edge)")
	designCreateCmd.Flags().String("adopt", "", "Comma-separated shard IDs to adopt as children")
	designCreateCmd.Flags().String("order", "", "Dependency edges: child:blocker,child:blocker")

	designCmd.AddCommand(designCreateCmd)
	rootCmd.AddCommand(designCmd)
}
