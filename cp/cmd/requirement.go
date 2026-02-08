package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/otherjamesbrown/context-palace/cp/internal/client"
	"github.com/spf13/cobra"
)

var requirementCmd = &cobra.Command{
	Use:     "requirement",
	Aliases: []string{"req"},
	Short:   "Requirement management",
	Long:    `Commands for creating, tracking, and managing requirements through their lifecycle.`,
}

var reqCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new requirement",
	Args:  cobra.ExactArgs(1),
	Example: `  cp requirement create "Entity Lifecycle Management" --priority 2 --category entity-management
  cp requirement create "Test Req" --priority 1 --body "Success criteria here"
  cp requirement create "From File" --priority 3 --body-file req.md`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		title := args[0]

		priority, _ := cmd.Flags().GetInt("priority")
		if priority < 1 || priority > 7 {
			return fmt.Errorf("priority must be 1-7. Got: %d", priority)
		}

		category, _ := cmd.Flags().GetString("category")
		body, _ := cmd.Flags().GetString("body")
		bodyFile, _ := cmd.Flags().GetString("body-file")

		if bodyFile != "" {
			data, err := os.ReadFile(bodyFile)
			if err != nil {
				return fmt.Errorf("failed to read body file: %v", err)
			}
			body = string(data)
		}

		id, err := cpClient.CreateRequirement(ctx, title, body, priority, category)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"id": "%s"}`+"\n", id)
			return nil
		}

		fmt.Printf("Created requirement %s\n", id)
		return nil
	},
}

var reqListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List requirements",
	Example: "  cp requirement list\n  cp requirement list --status approved\n  cp requirement list --status draft,approved --category testing",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		statusFlag, _ := cmd.Flags().GetString("status")
		categoryFlag, _ := cmd.Flags().GetString("category")

		var statusFilter []string
		if statusFlag != "" {
			statusFilter = strings.Split(statusFlag, ",")
		}

		reqs, err := cpClient.ListRequirements(ctx, statusFilter, categoryFlag, limitFlag)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(reqs)
			fmt.Println(s)
			return nil
		}

		if len(reqs) == 0 {
			fmt.Println("No requirements found.")
			return nil
		}

		tbl := client.NewTable("ID", "STATUS", "PRI", "CATEGORY", "TITLE", "TASKS", "TESTS")
		for _, r := range reqs {
			cat := ""
			if r.Category != nil {
				cat = *r.Category
			}
			tasks := fmt.Sprintf("%d/%d", r.TaskCountClosed, r.TaskCountTotal)
			tbl.AddRow(r.ID, r.LifecycleStatus, fmt.Sprintf("%d", r.Priority),
				client.Truncate(cat, 20), client.Truncate(r.Title, 40),
				tasks, fmt.Sprintf("%d", r.TestCount))
		}
		fmt.Print(tbl.String())
		return nil
	},
}

var reqShowCmd = &cobra.Command{
	Use:     "show <id>",
	Short:   "Show requirement details",
	Args:    cobra.ExactArgs(1),
	Example: "  cp requirement show pf-req-01",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		shard, edges, taskTotal, taskClosed, testCount, err := cpClient.ShowRequirement(ctx, id)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			result := map[string]any{
				"shard":        shard,
				"edges":        edges,
				"task_total":   taskTotal,
				"task_closed":  taskClosed,
				"test_count":   testCount,
			}
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		// Parse metadata for display
		lifecycleStatus := "draft"
		priority := ""
		category := ""
		if shard.Metadata != nil {
			var meta map[string]any
			if err := json.Unmarshal(shard.Metadata, &meta); err == nil {
				if ls, ok := meta["lifecycle_status"].(string); ok {
					lifecycleStatus = ls
				}
				if p, ok := meta["priority"].(float64); ok {
					priority = fmt.Sprintf("%d", int(p))
				}
				if c, ok := meta["category"].(string); ok {
					category = c
				}
			}
		}

		fmt.Println(shard.Title)
		fmt.Println(strings.Repeat("─", len(shard.Title)))
		fmt.Printf("ID:       %s\n", shard.ID)
		fmt.Printf("Status:   %s\n", lifecycleStatus)
		if priority != "" {
			fmt.Printf("Priority: %s\n", priority)
		}
		if category != "" {
			fmt.Printf("Category: %s\n", category)
		}
		fmt.Printf("Created:  %s by %s\n", shard.CreatedAt.Format("2006-01-02 15:04:05"), shard.Creator)

		// Task summary
		if taskTotal == 0 {
			fmt.Printf("\nTasks: 0/0 (no implementation tasks linked)\n")
		} else {
			fmt.Printf("\nTasks: %d/%d\n", taskClosed, taskTotal)
		}
		fmt.Printf("Tests: %d\n", testCount)

		// Content
		if shard.Content != "" {
			fmt.Printf("\n%s\n", shard.Content)
		}

		// Edges
		if len(edges) > 0 {
			fmt.Printf("\nEdges:\n")
			for _, e := range edges {
				otherID := e.FromID
				if otherID == id {
					otherID = e.ToID
				}
				ls := ""
				if e.LifecycleStatus != "" {
					ls = fmt.Sprintf(" (%s)", e.LifecycleStatus)
				}
				fmt.Printf("  %-14s %s  %q%s\n", e.EdgeType, otherID, e.Title, ls)
			}
		}

		return nil
	},
}

var reqApproveCmd = &cobra.Command{
	Use:     "approve <id>",
	Short:   "Approve a requirement (draft → approved)",
	Args:    cobra.ExactArgs(1),
	Example: "  cp requirement approve pf-req-01",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		if err := cpClient.ApproveRequirement(ctx, args[0]); err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"id": "%s", "lifecycle_status": "approved"}`+"\n", args[0])
			return nil
		}

		fmt.Printf("Approved requirement %s\n", args[0])
		return nil
	},
}

var reqVerifyCmd = &cobra.Command{
	Use:     "verify <id>",
	Short:   "Verify a requirement (implemented → verified)",
	Args:    cobra.ExactArgs(1),
	Example: "  cp requirement verify pf-req-01\n  cp requirement verify pf-req-01 --force",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		force, _ := cmd.Flags().GetBool("force")

		if err := cpClient.VerifyRequirement(ctx, args[0], force); err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"id": "%s", "lifecycle_status": "verified"}`+"\n", args[0])
			return nil
		}

		fmt.Printf("Verified requirement %s\n", args[0])
		return nil
	},
}

var reqReopenCmd = &cobra.Command{
	Use:     "reopen <id>",
	Short:   "Reopen a requirement (→ approved)",
	Args:    cobra.ExactArgs(1),
	Example: `  cp requirement reopen pf-req-01 --reason "test failures found"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		reason, _ := cmd.Flags().GetString("reason")
		if reason == "" {
			return fmt.Errorf("--reason is required")
		}

		if err := cpClient.ReopenRequirement(ctx, args[0], reason); err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"id": "%s", "lifecycle_status": "approved", "reopen_reason": "%s"}`+"\n", args[0], reason)
			return nil
		}

		fmt.Printf("Reopened requirement %s → approved\n", args[0])
		return nil
	},
}

var reqLinkCmd = &cobra.Command{
	Use:   "link <id>",
	Short: "Link a task, test, or dependency to a requirement",
	Args:  cobra.ExactArgs(1),
	Example: `  cp requirement link pf-req-01 --task pf-task-123
  cp requirement link pf-req-01 --test pf-test-789
  cp requirement link pf-req-05 --depends-on pf-req-03`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		reqID := args[0]

		taskFlag, _ := cmd.Flags().GetString("task")
		testFlag, _ := cmd.Flags().GetString("test")
		depsFlag, _ := cmd.Flags().GetString("depends-on")

		flagCount := 0
		if taskFlag != "" {
			flagCount++
		}
		if testFlag != "" {
			flagCount++
		}
		if depsFlag != "" {
			flagCount++
		}

		if flagCount != 1 {
			return fmt.Errorf("exactly one of --task, --test, or --depends-on is required")
		}

		var err error
		var linkType, targetID string

		if taskFlag != "" {
			err = cpClient.LinkTask(ctx, reqID, taskFlag)
			linkType = "implements"
			targetID = taskFlag
		} else if testFlag != "" {
			err = cpClient.LinkTest(ctx, reqID, testFlag)
			linkType = "has-artifact"
			targetID = testFlag
		} else {
			err = cpClient.LinkDependency(ctx, reqID, depsFlag)
			linkType = "blocked-by"
			targetID = depsFlag
		}

		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"requirement": "%s", "target": "%s", "edge_type": "%s"}`+"\n", reqID, targetID, linkType)
			return nil
		}

		fmt.Printf("Linked %s --%s--> %s\n", targetID, linkType, reqID)
		return nil
	},
}

var reqUnlinkCmd = &cobra.Command{
	Use:   "unlink <id>",
	Short: "Unlink a task, test, or dependency from a requirement",
	Args:  cobra.ExactArgs(1),
	Example: `  cp requirement unlink pf-req-01 --task pf-task-123
  cp requirement unlink pf-req-01 --test pf-test-789
  cp requirement unlink pf-req-05 --depends-on pf-req-03`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		reqID := args[0]

		taskFlag, _ := cmd.Flags().GetString("task")
		testFlag, _ := cmd.Flags().GetString("test")
		depsFlag, _ := cmd.Flags().GetString("depends-on")

		flagCount := 0
		if taskFlag != "" {
			flagCount++
		}
		if testFlag != "" {
			flagCount++
		}
		if depsFlag != "" {
			flagCount++
		}

		if flagCount != 1 {
			return fmt.Errorf("exactly one of --task, --test, or --depends-on is required")
		}

		var err error
		var edgeType, targetID string

		if taskFlag != "" {
			edgeType = "implements"
			targetID = taskFlag
		} else if testFlag != "" {
			edgeType = "has-artifact"
			targetID = testFlag
		} else {
			edgeType = "blocked-by"
			targetID = depsFlag
		}

		err = cpClient.UnlinkEdge(ctx, reqID, targetID, edgeType)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			fmt.Printf(`{"requirement": "%s", "target": "%s", "edge_type": "%s", "action": "unlinked"}`+"\n", reqID, targetID, edgeType)
			return nil
		}

		fmt.Printf("Unlinked %s --%s--> %s\n", targetID, edgeType, reqID)
		return nil
	},
}

var reqDashboardCmd = &cobra.Command{
	Use:     "dashboard",
	Short:   "Show requirements dashboard",
	Example: "  cp requirement dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		rows, err := cpClient.RequirementDashboard(ctx)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(rows)
			fmt.Println(s)
			return nil
		}

		// Count by status
		counts := map[string]int{}
		for _, r := range rows {
			counts[r.LifecycleStatus]++
		}
		total := len(rows)

		fmt.Println("REQUIREMENTS DASHBOARD")
		fmt.Println("──────────────────────")
		fmt.Printf("Total: %d    Draft: %d    Approved: %d    In Progress: %d    Implemented: %d    Verified: %d\n\n",
			total, counts["draft"], counts["approved"], counts["in_progress"],
			counts["implemented"], counts["verified"])

		// Blocked
		fmt.Println("BLOCKED:")
		hasBlocked := false
		for _, r := range rows {
			if len(r.BlockedByIDs) > 0 {
				hasBlocked = true
				fmt.Printf("  %s %q ← blocked by %s\n", r.ID, r.Title, strings.Join(r.BlockedByIDs, ", "))
			}
		}
		if !hasBlocked {
			fmt.Println("  (none)")
		}

		// Ready for implementation
		fmt.Println("\nREADY FOR IMPLEMENTATION (approved, unblocked):")
		hasReady := false
		for _, r := range rows {
			if r.LifecycleStatus == "approved" && len(r.BlockedByIDs) == 0 {
				hasReady = true
				cat := ""
				if r.Category != nil {
					cat = fmt.Sprintf(", %s", *r.Category)
				}
				fmt.Printf("  %s %q (pri %d%s)\n", r.ID, r.Title, r.Priority, cat)
			}
		}
		if !hasReady {
			fmt.Println("  (none)")
		}

		// Needs verification
		fmt.Println("\nNEEDS VERIFICATION (implemented, untested):")
		hasNeedsVerify := false
		for _, r := range rows {
			if r.LifecycleStatus == "implemented" && r.TestCount == 0 {
				hasNeedsVerify = true
				fmt.Printf("  %s %q\n", r.ID, r.Title)
			}
		}
		if !hasNeedsVerify {
			fmt.Println("  (none)")
		}

		// Test coverage
		tested := 0
		for _, r := range rows {
			if r.TestCount > 0 {
				tested++
			}
		}
		pct := 0
		if total > 0 {
			pct = tested * 100 / total
		}
		fmt.Printf("\nTEST COVERAGE: %d/%d (%d%%)\n", tested, total, pct)
		for _, r := range rows {
			if r.TestCount > 0 {
				fmt.Printf("  %s %q — %d tests\n", r.ID, r.Title, r.TestCount)
			}
		}

		return nil
	},
}

func init() {
	// create flags
	reqCreateCmd.Flags().Int("priority", 3, "Priority (1-7)")
	reqCreateCmd.Flags().String("category", "", "Requirement category")
	reqCreateCmd.Flags().String("body", "", "Requirement body/content")
	reqCreateCmd.Flags().String("body-file", "", "Read body from file")

	// list flags
	reqListCmd.Flags().String("status", "", "Filter by lifecycle status (comma-separated)")
	reqListCmd.Flags().String("category", "", "Filter by category")

	// verify flags
	reqVerifyCmd.Flags().Bool("force", false, "Verify without test coverage")

	// reopen flags
	reqReopenCmd.Flags().String("reason", "", "Reason for reopening (required)")

	// link flags
	reqLinkCmd.Flags().String("task", "", "Task shard ID to link")
	reqLinkCmd.Flags().String("test", "", "Test shard ID to link")
	reqLinkCmd.Flags().String("depends-on", "", "Requirement ID this depends on")

	// unlink flags
	reqUnlinkCmd.Flags().String("task", "", "Task shard ID to unlink")
	reqUnlinkCmd.Flags().String("test", "", "Test shard ID to unlink")
	reqUnlinkCmd.Flags().String("depends-on", "", "Requirement ID to remove dependency")

	// Wire command tree
	requirementCmd.AddCommand(reqCreateCmd)
	requirementCmd.AddCommand(reqListCmd)
	requirementCmd.AddCommand(reqShowCmd)
	requirementCmd.AddCommand(reqApproveCmd)
	requirementCmd.AddCommand(reqVerifyCmd)
	requirementCmd.AddCommand(reqReopenCmd)
	requirementCmd.AddCommand(reqLinkCmd)
	requirementCmd.AddCommand(reqUnlinkCmd)
	requirementCmd.AddCommand(reqDashboardCmd)

	rootCmd.AddCommand(requirementCmd)
}
