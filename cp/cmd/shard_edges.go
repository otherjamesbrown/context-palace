package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

// -- shard edges --

var shardEdgesCmd = &cobra.Command{
	Use:   "edges <shard-id>",
	Short: "Show edges for a shard",
	Args:  cobra.ExactArgs(1),
	Example: `  cp shard edges pf-abc123
  cp shard edges pf-abc123 --direction outgoing
  cp shard edges pf-abc123 --edge-type implements,references
  cp shard edges pf-req-01 --follow
  cp shard edges pf-req-01 --follow --max-depth 3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		direction, _ := cmd.Flags().GetString("direction")
		edgeTypeFlag, _ := cmd.Flags().GetString("edge-type")
		follow, _ := cmd.Flags().GetBool("follow")
		maxDepth, _ := cmd.Flags().GetInt("max-depth")

		if maxDepth < 1 || maxDepth > 5 {
			return fmt.Errorf("max-depth must be 1-5")
		}

		var edgeTypes []string
		if edgeTypeFlag != "" {
			edgeTypes = strings.Split(edgeTypeFlag, ",")
		}

		// Verify shard exists
		exists, err := cpClient.ShardExists(ctx, id)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("Shard %s not found", id)
		}

		if follow {
			return showEdgeTree(ctx, id, direction, edgeTypes, maxDepth)
		}

		edges, err := cpClient.GetShardEdges(ctx, id, direction, edgeTypes)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(edges)
			fmt.Println(s)
			return nil
		}

		if len(edges) == 0 {
			fmt.Println("No edges found.")
			return nil
		}

		tbl := client.NewTable("DIRECTION", "EDGE TYPE", "SHARD", "TYPE", "STATUS", "TITLE")
		for _, e := range edges {
			tbl.AddRow(e.Direction, e.EdgeType, e.ShardID, e.Type, e.Status, client.Truncate(e.Title, 40))
		}
		fmt.Print(tbl.String())
		return nil
	},
}

func showEdgeTree(ctx context.Context, id, direction string, edgeTypes []string, maxDepth int) error {
	nodes, err := cpClient.GetShardEdgesFollow(ctx, id, direction, edgeTypes, maxDepth)
	if err != nil {
		return err
	}

	if outputFormat == "json" {
		s, _ := client.FormatJSON(nodes)
		fmt.Println(s)
		return nil
	}

	// Get root shard info
	detail, err := cpClient.GetShardDetail(ctx, id)
	if err != nil {
		return err
	}
	fmt.Printf("%s \"%s\"\n", id, detail.Title)

	printTreeNodes(nodes, "")
	return nil
}

func printTreeNodes(nodes []*client.EdgeTreeNode, prefix string) {
	for i, n := range nodes {
		isLast := i == len(nodes)-1
		connector := "├── "
		childPrefix := "│   "
		if isLast {
			connector = "└── "
			childPrefix = "    "
		}

		if n.IsCycle {
			fmt.Printf("%s%s%s (cycle: %s already shown)\n", prefix, connector, n.EdgeType, n.ShardID)
			continue
		}

		dirSuffix := ""
		if n.Direction == "incoming" {
			dirSuffix = " (incoming)"
		}

		fmt.Printf("%s%s%s%s\n", prefix, connector, n.EdgeType, dirSuffix)

		// Print the shard under the edge type
		shardConnector := "└── "
		shardPrefix := "    "
		if len(n.Children) > 0 {
			shardConnector = "├── "
			shardPrefix = "│   "
		}

		statusStr := ""
		if n.Status != "open" {
			statusStr = fmt.Sprintf(" (%s)", n.Status)
		}
		fmt.Printf("%s%s%s%s \"%s\"%s\n", prefix+childPrefix, shardConnector, n.ShardID, "", client.Truncate(n.Title, 40), statusStr)

		if len(n.Children) > 0 {
			printTreeNodes(n.Children, prefix+childPrefix+shardPrefix)
		}
	}
}

// -- shard link --

var shardLinkCmd = &cobra.Command{
	Use:   "link <from-shard-id>",
	Short: "Create a typed edge between shards",
	Args:  cobra.ExactArgs(1),
	Example: `  cp shard link pf-task-123 --implements pf-req-01
  cp shard link pf-bug-03 --references pf-task-456
  cp shard link pf-req-05 --blocked-by pf-req-03`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		fromID := args[0]

		// Find which edge type flag was set
		var edgeType, toID string
		flagCount := 0
		for _, et := range client.ValidEdgeTypes {
			val, _ := cmd.Flags().GetString(et)
			if val != "" {
				edgeType = et
				toID = val
				flagCount++
			}
		}

		if flagCount == 0 {
			return fmt.Errorf("specify an edge type flag. Valid types: %s", strings.Join(client.ValidEdgeTypes, ", "))
		}
		if flagCount > 1 {
			return fmt.Errorf("exactly one edge type flag allowed")
		}

		err := cpClient.CreateEdge(ctx, fromID, toID, edgeType, nil)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"from":      fromID,
				"to":        toID,
				"edge_type": edgeType,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Created edge: %s --%s--> %s\n", fromID, edgeType, toID)
		return nil
	},
}

// -- shard unlink --

var shardUnlinkCmd = &cobra.Command{
	Use:   "unlink <from-shard-id>",
	Short: "Remove a typed edge between shards",
	Args:  cobra.ExactArgs(1),
	Example: `  cp shard unlink pf-task-123 --implements pf-req-01
  cp shard unlink pf-task-123 --implements pf-req-01 --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		fromID := args[0]
		force, _ := cmd.Flags().GetBool("force")

		// Find which edge type flag was set
		var edgeType, toID string
		flagCount := 0
		for _, et := range client.ValidEdgeTypes {
			val, _ := cmd.Flags().GetString(et)
			if val != "" {
				edgeType = et
				toID = val
				flagCount++
			}
		}

		if flagCount == 0 {
			return fmt.Errorf("specify an edge type flag. Valid types: %s", strings.Join(client.ValidEdgeTypes, ", "))
		}
		if flagCount > 1 {
			return fmt.Errorf("exactly one edge type flag allowed")
		}

		if !force {
			fmt.Printf("Remove %s edge from %s to %s? (y/n) ", edgeType, fromID, toID)
			var answer string
			fmt.Scanln(&answer)
			if answer != "y" && answer != "Y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		err := cpClient.DeleteEdge(ctx, fromID, toID, edgeType)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"from":      fromID,
				"to":        toID,
				"edge_type": edgeType,
				"removed":   true,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Removed edge: %s --%s--> %s\n", fromID, edgeType, toID)
		return nil
	},
}

func init() {
	// shard edges flags
	shardEdgesCmd.Flags().String("direction", "", "Filter: outgoing, incoming, or both")
	shardEdgesCmd.Flags().String("edge-type", "", "Comma-separated edge types")
	shardEdgesCmd.Flags().Bool("follow", false, "Tree view showing N-hop graph")
	shardEdgesCmd.Flags().Int("max-depth", 2, "Max hops for --follow (1-5)")

	// shard link flags — one flag per edge type
	for _, et := range client.ValidEdgeTypes {
		shardLinkCmd.Flags().String(et, "", fmt.Sprintf("Target shard for %s edge", et))
		shardUnlinkCmd.Flags().String(et, "", fmt.Sprintf("Target shard for %s edge", et))
	}
	shardUnlinkCmd.Flags().Bool("force", false, "Skip confirmation prompt")

	shardCmd.AddCommand(shardEdgesCmd)
	shardCmd.AddCommand(shardLinkCmd)
	shardCmd.AddCommand(shardUnlinkCmd)
}
