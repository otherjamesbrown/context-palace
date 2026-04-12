package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage KB maintenance schedules",
	Long: `Manage scheduled KB maintenance workflows (drift-scan, canary, triage).

Schedules are per-project. Each schedule binds a workflow type to a cron
expression and a config block that controls model selection and shard routing.

WORKFLOW TYPES:
  drift-scan   Nightly factcheck of KB articles (default: 03:00)
  canary       Nightly retrieval quality test (default: 06:00)
  triage       Weekly gap analysis and task creation (default: Monday 05:00)

EXAMPLES:
  cxp schedule list
  cxp schedule create drift-scan --cron "0 3 * * *" --config '{"repo_path":"/path/to/repo"}'
  cxp schedule enable drift-scan
  cxp schedule disable canary
  cxp schedule run drift-scan
  cxp schedule history drift-scan --limit 20
  cxp schedule last drift-scan`,
}

// -- schedule list --

var scheduleListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List schedules for the current project",
	Args:    cobra.NoArgs,
	Example: "  cxp schedule list\n  cxp --output json schedule list",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		schedules, err := cpClient.ListSchedules(ctx)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			if schedules == nil {
				schedules = []client.Schedule{}
			}
			out, _ := client.FormatJSON(schedules)
			fmt.Println(out)
			return nil
		}

		if len(schedules) == 0 {
			fmt.Printf("No schedules for project %q\n", cpClient.Config.Project)
			return nil
		}

		tbl := client.NewTable("NAME", "WORKFLOW", "CRON", "ENABLED", "LAST RUN", "NEXT RUN")
		for _, s := range schedules {
			enabled := "yes"
			if !s.Enabled {
				enabled = "no"
			}
			lastRun := "-"
			if s.LastRunAt != nil {
				lastRun = s.LastRunAt.Local().Format("2006-01-02 15:04")
			}
			nextRun := "-"
			if s.NextRunAt != nil {
				nextRun = s.NextRunAt.Local().Format("2006-01-02 15:04")
			}
			tbl.AddRow(s.Name, s.WorkflowType, s.ScheduleExpr, enabled, lastRun, nextRun)
		}
		fmt.Print(tbl)
		return nil
	},
}

// -- schedule create --

var scheduleCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new schedule",
	Long: `Create a schedule binding a workflow to a cron expression.

NAME must be unique within the project. WORKFLOW_TYPE must be one of:
  drift-scan, canary, triage

The --config flag accepts a JSON object with workflow-specific settings.`,
	Args: cobra.ExactArgs(1),
	Example: `  cxp schedule create drift-scan --cron "0 3 * * *" --config '{"repo_path":"/path/to/repo","judge_articles_per_run":5}'
  cxp schedule create canary --cron "0 6 * * *" --config '{"canary_shard":"pf-kb-canaries"}'
  cxp schedule create triage --cron "0 5 * * MON" --config '{"gaps_shard":"pf-kb-gaps"}'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cronExpr, _ := cmd.Flags().GetString("cron")
		configJSON, _ := cmd.Flags().GetString("config")

		if cronExpr == "" {
			return fmt.Errorf("--cron is required")
		}

		// workflow_type defaults to the schedule name if it matches a valid type
		workflowType, _ := cmd.Flags().GetString("workflow-type")
		if workflowType == "" {
			workflowType = name
		}
		if err := client.ValidateWorkflowType(workflowType); err != nil {
			return err
		}

		ctx := context.Background()
		sched, err := cpClient.CreateSchedule(ctx, name, workflowType, cronExpr, configJSON)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out, _ := client.FormatJSON(sched)
			fmt.Println(out)
			return nil
		}

		fmt.Printf("Created schedule %q (workflow: %s, cron: %s)\n", sched.Name, sched.WorkflowType, sched.ScheduleExpr)
		return nil
	},
}

// -- schedule enable --

var scheduleEnableCmd = &cobra.Command{
	Use:     "enable <name>",
	Short:   "Enable a schedule",
	Args:    cobra.ExactArgs(1),
	Example: "  cxp schedule enable drift-scan",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		if err := cpClient.SetScheduleEnabled(ctx, args[0], true); err != nil {
			return err
		}
		fmt.Printf("Schedule %q enabled\n", args[0])
		return nil
	},
}

// -- schedule disable --

var scheduleDisableCmd = &cobra.Command{
	Use:     "disable <name>",
	Short:   "Disable a schedule",
	Args:    cobra.ExactArgs(1),
	Example: "  cxp schedule disable canary",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		if err := cpClient.SetScheduleEnabled(ctx, args[0], false); err != nil {
			return err
		}
		fmt.Printf("Schedule %q disabled\n", args[0])
		return nil
	},
}

// -- schedule history --

var scheduleHistoryCmd = &cobra.Command{
	Use:   "history <name>",
	Short: "Show recent runs for a schedule",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp schedule history drift-scan
  cxp schedule history drift-scan --limit 20
  cxp --output json schedule history canary`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		limit, _ := cmd.Flags().GetInt("limit")
		if limit <= 0 {
			limit = 10
		}

		ctx := context.Background()
		runs, err := cpClient.ScheduleHistory(ctx, name, limit)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			if runs == nil {
				runs = []client.ScheduleRun{}
			}
			out, _ := client.FormatJSON(runs)
			fmt.Println(out)
			return nil
		}

		if len(runs) == 0 {
			fmt.Printf("No runs found for schedule %q\n", name)
			return nil
		}

		tbl := client.NewTable("ID", "STATUS", "STARTED", "FINISHED", "SUMMARY")
		for _, r := range runs {
			finished := "-"
			if r.FinishedAt != nil {
				finished = r.FinishedAt.Local().Format("2006-01-02 15:04:05")
			}
			summary := "-"
			if r.Summary != nil {
				summary = client.Truncate(*r.Summary, 60)
			}
			tbl.AddRow(
				fmt.Sprintf("%d", r.ID),
				r.Status,
				r.StartedAt.Local().Format("2006-01-02 15:04:05"),
				finished,
				summary,
			)
		}
		fmt.Print(tbl)
		return nil
	},
}

// -- schedule last --

var scheduleLastCmd = &cobra.Command{
	Use:   "last <name>",
	Short: "Show the last run of a schedule",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp schedule last drift-scan
  cxp --output json schedule last triage`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		run, err := cpClient.ScheduleLastRun(ctx, args[0])
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out, _ := client.FormatJSON(run)
			fmt.Println(out)
			return nil
		}

		finished := "-"
		duration := ""
		if run.FinishedAt != nil {
			finished = run.FinishedAt.Local().Format("2006-01-02 15:04:05")
			duration = fmt.Sprintf(" (%s)", run.FinishedAt.Sub(run.StartedAt).Round(time.Second))
		}
		summary := "-"
		if run.Summary != nil {
			summary = *run.Summary
		}

		fmt.Printf("Run #%d  status: %s\n", run.ID, run.Status)
		fmt.Printf("  started:  %s\n", run.StartedAt.Local().Format("2006-01-02 15:04:05"))
		fmt.Printf("  finished: %s%s\n", finished, duration)
		fmt.Printf("  summary:  %s\n", summary)
		fmt.Printf("  result:   %s\n", strings.TrimSpace(string(run.Result)))
		return nil
	},
}

// -- schedule run --

var scheduleRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Execute a workflow in the foreground and record the run",
	Long: `Trigger a workflow run immediately, synchronously, and record it in schedule_runs.

Useful for testing before the daemon exists. The workflow stubs are no-ops
until the workflow implementations are added in the next wave.`,
	Args:    cobra.ExactArgs(1),
	Example: "  cxp schedule run drift-scan",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		ctx := context.Background()

		fmt.Printf("Running schedule %q...\n", name)

		run, err := cpClient.RunSchedule(ctx, name)
		if err != nil {
			// Even on workflow failure, a run row may have been recorded
			if run != nil {
				fmt.Printf("Run #%d completed with status: %s\n", run.ID, run.Status)
			}
			return err
		}

		if outputFormat == "json" {
			out, _ := client.FormatJSON(run)
			fmt.Println(out)
			return nil
		}

		duration := ""
		if run.FinishedAt != nil {
			duration = fmt.Sprintf(" in %s", run.FinishedAt.Sub(run.StartedAt).Round(time.Millisecond))
		}
		summary := ""
		if run.Summary != nil {
			summary = fmt.Sprintf("\n  summary: %s", *run.Summary)
		}
		fmt.Printf("Run #%d completed%s  status: %s%s\n", run.ID, duration, run.Status, summary)
		return nil
	},
}

func init() {
	// schedule create flags
	scheduleCreateCmd.Flags().String("cron", "", "Cron expression (required)")
	scheduleCreateCmd.Flags().String("config", "{}", "Workflow config as JSON")
	scheduleCreateCmd.Flags().String("workflow-type", "", "Workflow type (drift-scan|canary|triage); defaults to <name>")

	// schedule history flags
	scheduleHistoryCmd.Flags().Int("limit", 10, "Number of runs to show")

	// Wire subcommands to parent
	scheduleCmd.AddCommand(scheduleListCmd)
	scheduleCmd.AddCommand(scheduleCreateCmd)
	scheduleCmd.AddCommand(scheduleEnableCmd)
	scheduleCmd.AddCommand(scheduleDisableCmd)
	scheduleCmd.AddCommand(scheduleHistoryCmd)
	scheduleCmd.AddCommand(scheduleLastCmd)
	scheduleCmd.AddCommand(scheduleRunCmd)

	// Register with root
	rootCmd.AddCommand(scheduleCmd)
}
