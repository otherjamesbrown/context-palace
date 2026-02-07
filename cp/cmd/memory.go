package cmd

import (
	"context"
	"fmt"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Agent memory",
	Long:  `Commands for managing agent memory — add, list, search, resolve, and defer memories.`,
}

var memoryAddCmd = &cobra.Command{
	Use:     "add <content>",
	Short:   "Add a memory",
	Args:    cobra.ExactArgs(1),
	Example: `  cp memory add "AI client timeout was hardcoded at 120s, not configurable"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		content := args[0]

		// Use content as title (truncated) and full text as content
		title := client.Truncate(content, 200)
		id, err := cpClient.CreateShard(ctx, title, content, "memory", nil, nil)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"id": "%s"}`+"\n", id)
			return nil
		}

		fmt.Printf("Created memory %s\n", id)
		return nil
	},
}

var memoryListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List memories",
	Example: "  cp memory list",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		memories, err := cpClient.ListShardsByType(ctx, "memory", "open", limitFlag)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(memories)
			fmt.Println(s)
			return nil
		}

		if len(memories) == 0 {
			fmt.Println("No memories found.")
			return nil
		}

		tbl := client.NewTable("ID", "CREATED", "CONTENT")
		for _, m := range memories {
			tbl.AddRow(m.ID, m.CreatedAt.Format("2006-01-02"), client.Truncate(m.Title, 60))
		}
		fmt.Print(tbl.String())
		return nil
	},
}

var memorySearchCmd = &cobra.Command{
	Use:     "search <query>",
	Short:   "Search memories",
	Args:    cobra.ExactArgs(1),
	Example: `  cp memory search "timeout"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		results, err := cpClient.SearchShards(ctx, args[0], "memory", limitFlag)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(results)
			fmt.Println(s)
			return nil
		}

		if len(results) == 0 {
			fmt.Println("No matching memories found.")
			return nil
		}

		tbl := client.NewTable("ID", "CREATED", "CONTENT")
		for _, m := range results {
			tbl.AddRow(m.ID, m.CreatedAt.Format("2006-01-02"), client.Truncate(m.Title, 60))
		}
		fmt.Print(tbl.String())
		return nil
	},
}

var memoryResolveCmd = &cobra.Command{
	Use:     "resolve <shard-id>",
	Short:   "Resolve (close) a memory",
	Args:    cobra.ExactArgs(1),
	Example: "  cp memory resolve pf-mem-123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		err := cpClient.UpdateShardStatus(ctx, args[0], "closed")
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"success": true, "id": "%s", "status": "closed"}`+"\n", args[0])
			return nil
		}

		fmt.Printf("Resolved memory %s\n", args[0])
		return nil
	},
}

var memoryDeferCmd = &cobra.Command{
	Use:     "defer <shard-id>",
	Short:   "Defer a memory for later review",
	Args:    cobra.ExactArgs(1),
	Example: "  cp memory defer pf-mem-123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		// Defer = set status to deferred (or add a deferred label)
		err := cpClient.UpdateShardStatus(ctx, args[0], "deferred")
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"success": true, "id": "%s", "status": "deferred"}`+"\n", args[0])
			return nil
		}

		fmt.Printf("Deferred memory %s\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(memoryCmd)
	memoryCmd.AddCommand(memoryAddCmd)
	memoryCmd.AddCommand(memoryListCmd)
	memoryCmd.AddCommand(memorySearchCmd)
	memoryCmd.AddCommand(memoryResolveCmd)
	memoryCmd.AddCommand(memoryDeferCmd)
}
