package cmd

import (
	"context"
	"fmt"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var childrenCmd = &cobra.Command{
	Use:   "children",
	Short: "Manage knowledge document child relationships",
	Long:  `Add, list, update, and remove child document relationships on knowledge docs.`,
}

var childrenListCmd = &cobra.Command{
	Use:     "list <parent-id>",
	Short:   "List children of a knowledge document",
	Args:    cobra.ExactArgs(1),
	Example: "  cxp knowledge children list pf-playbook-001",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		parentID := args[0]

		children, err := cpClient.ListKnowledgeChildren(ctx, parentID)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(children)
			fmt.Println(s)
			return nil
		}

		if len(children) == 0 {
			fmt.Println("No children found.")
			return nil
		}

		tbl := client.NewTable("ID", "TITLE", "DESCRIPTION", "TRIGGER")
		for _, ch := range children {
			tbl.AddRow(ch.ID, client.Truncate(ch.Title, 40),
				client.Truncate(ch.Description, 40), client.Truncate(ch.Trigger, 40))
		}
		fmt.Print(tbl.String())
		return nil
	},
}

var childrenAddCmd = &cobra.Command{
	Use:   "add <parent-id> <child-id>",
	Short: "Add a child document relationship",
	Args:  cobra.ExactArgs(2),
	Example: `  cxp knowledge children add pf-playbook-001 pf-deploy-guide \
    --description "Deployment procedures" --trigger "When deploying to production"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		parentID := args[0]
		childID := args[1]

		description, _ := cmd.Flags().GetString("description")
		trigger, _ := cmd.Flags().GetString("trigger")

		if description == "" {
			return fmt.Errorf("--description is required")
		}
		if trigger == "" {
			return fmt.Errorf("--trigger is required")
		}

		if err := cpClient.AddKnowledgeChild(ctx, parentID, childID, description, trigger); err != nil {
			return err
		}

		if outputFormat == "json" {
			result := map[string]any{
				"parent_id":   parentID,
				"child_id":    childID,
				"description": description,
				"trigger":     trigger,
			}
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Added %s as child of %s\n", childID, parentID)
		return nil
	},
}

var childrenUpdateCmd = &cobra.Command{
	Use:   "update <parent-id> <child-id>",
	Short: "Update a child document relationship",
	Args:  cobra.ExactArgs(2),
	Example: `  cxp knowledge children update pf-playbook-001 pf-deploy-guide \
    --description "Updated deployment procedures"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		parentID := args[0]
		childID := args[1]

		description, _ := cmd.Flags().GetString("description")
		trigger, _ := cmd.Flags().GetString("trigger")

		if description == "" && trigger == "" {
			return fmt.Errorf("at least one of --description or --trigger is required")
		}

		if err := cpClient.UpdateKnowledgeChild(ctx, parentID, childID, description, trigger); err != nil {
			return err
		}

		if outputFormat == "json" {
			result := map[string]any{
				"parent_id": parentID,
				"child_id":  childID,
				"updated":   true,
			}
			if description != "" {
				result["description"] = description
			}
			if trigger != "" {
				result["trigger"] = trigger
			}
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Updated child relationship %s -> %s\n", parentID, childID)
		return nil
	},
}

var childrenRemoveCmd = &cobra.Command{
	Use:     "remove <parent-id> <child-id>",
	Short:   "Remove a child document relationship",
	Args:    cobra.ExactArgs(2),
	Example: "  cxp knowledge children remove pf-playbook-001 pf-deploy-guide",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		parentID := args[0]
		childID := args[1]

		if err := cpClient.RemoveKnowledgeChild(ctx, parentID, childID); err != nil {
			return err
		}

		if outputFormat == "json" {
			result := map[string]any{
				"parent_id": parentID,
				"child_id":  childID,
				"removed":   true,
			}
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Removed child relationship %s -> %s\n", parentID, childID)
		return nil
	},
}

func init() {
	// add flags
	childrenAddCmd.Flags().String("description", "", "Description of the child document (required)")
	childrenAddCmd.Flags().String("trigger", "", "When to load this child document (required)")

	// update flags
	childrenUpdateCmd.Flags().String("description", "", "Updated description")
	childrenUpdateCmd.Flags().String("trigger", "", "Updated trigger condition")

	// Wire command tree
	childrenCmd.AddCommand(childrenListCmd)
	childrenCmd.AddCommand(childrenAddCmd)
	childrenCmd.AddCommand(childrenUpdateCmd)
	childrenCmd.AddCommand(childrenRemoveCmd)

	knowledgeCmd.AddCommand(childrenCmd)
}
