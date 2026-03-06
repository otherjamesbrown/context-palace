package cmd

import (
	"context"
	"fmt"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var (
	backlogPriority int
)

var backlogCmd = &cobra.Command{
	Use:   "backlog",
	Short: "Dev backlog",
	Long:  `Commands for managing the development backlog — add, list, show, update, and close items.`,
}

var backlogAddCmd = &cobra.Command{
	Use:     "add <title>",
	Short:   "Add a backlog item",
	Args:    cobra.ExactArgs(1),
	Example: `  cp backlog add "Refactor entity pipeline" --priority 2`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		title := args[0]

		body, _ := cmd.Flags().GetString("body")

		var pri *int
		if cmd.Flags().Changed("priority") {
			pri = &backlogPriority
		}

		id, err := cpClient.CreateShard(ctx, title, body, "backlog", pri, nil)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"id": "%s"}`+"\n", id)
			return nil
		}

		fmt.Printf("Created backlog item %s\n", id)
		return nil
	},
}

var backlogListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List backlog items",
	Example: "  cp backlog list",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		items, err := cpClient.ListShardsByType(ctx, "backlog", "open", limitFlag)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(items)
			fmt.Println(s)
			return nil
		}

		if len(items) == 0 {
			fmt.Println("No backlog items found.")
			return nil
		}

		tbl := client.NewTable("ID", "PRI", "STATUS", "TITLE")
		for _, item := range items {
			pri := "-"
			if item.Priority != nil {
				pri = fmt.Sprintf("%d", *item.Priority)
			}
			tbl.AddRow(item.ID, pri, item.Status, client.Truncate(item.Title, 50))
		}
		fmt.Print(tbl.String())
		return nil
	},
}

var backlogShowCmd = &cobra.Command{
	Use:     "show <shard-id>",
	Short:   "Show backlog item details",
	Args:    cobra.ExactArgs(1),
	Example: "  cp backlog show pf-123",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		task, err := cpClient.GetTask(ctx, args[0])
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(task)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("ID:       %s\n", task.ID)
		fmt.Printf("Title:    %s\n", task.Title)
		fmt.Printf("Status:   %s\n", task.Status)
		if task.Priority != nil {
			fmt.Printf("Priority: %d\n", *task.Priority)
		}
		fmt.Printf("Created:  %s\n", task.CreatedAt.Format("2006-01-02 15:04:05"))
		if task.Content != "" {
			fmt.Printf("\n%s\n", task.Content)
		}
		return nil
	},
}

var backlogUpdateCmd = &cobra.Command{
	Use:     "update <shard-id> <note>",
	Short:   "Update a backlog item",
	Args:    cobra.ExactArgs(2),
	Example: `  cp backlog update pf-123 "Revised scope to include API changes"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		err := cpClient.AddProgress(ctx, args[0], args[1])
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"success": true, "id": "%s"}`+"\n", args[0])
			return nil
		}

		fmt.Printf("Updated backlog item %s\n", args[0])
		return nil
	},
}

var backlogCloseCmd = &cobra.Command{
	Use:     "close <shard-id>",
	Short:   "Close a backlog item",
	Args:    cobra.ExactArgs(1),
	Example: "  cp backlog close pf-123",
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

		fmt.Printf("Closed backlog item %s\n", args[0])
		return nil
	},
}

func init() {
	backlogAddCmd.Flags().IntVar(&backlogPriority, "priority", 0, "Priority (0-4)")
	backlogAddCmd.Flags().String("body", "", "Item description")

	rootCmd.AddCommand(backlogCmd)
	backlogCmd.AddCommand(backlogAddCmd)
	backlogCmd.AddCommand(backlogListCmd)
	backlogCmd.AddCommand(backlogShowCmd)
	backlogCmd.AddCommand(backlogUpdateCmd)
	backlogCmd.AddCommand(backlogCloseCmd)
}
