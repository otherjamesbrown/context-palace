package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage KB maintenance schedules",
	Long:  `List, create, enable, disable, and inspect KB maintenance schedules for the current project.`,
}

var scheduleListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List schedules for the current project",
	Example: "  cxp schedule list",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		schedules, err := cpClient.ListSchedules(ctx)
		if err != nil {
			return err
		}

		if outputFormat == "json" || outputFormat == "yaml" {
			s, err := client.FormatOutput(schedules, outputFormat)
			if err != nil {
				return err
			}
			fmt.Println(s)
			return nil
		}

		if len(schedules) == 0 {
			fmt.Println("No schedules found.")
			return nil
		}

		tbl := client.NewTable("NAME", "WORKFLOW", "CRON", "ENABLED", "LAST RUN", "NEXT RUN")
		for _, schedule := range schedules {
			tbl.AddRow(
				schedule.Name,
				schedule.WorkflowType,
				schedule.ScheduleExpr,
				fmt.Sprintf("%t", schedule.Enabled),
				formatScheduleTime(schedule.LastRunAt),
				formatScheduleTime(schedule.NextRunAt),
			)
		}
		fmt.Print(tbl.String())
		return nil
	},
}

var scheduleCreateCmd = &cobra.Command{
	Use:   "create <workflow-type>",
	Short: "Create a schedule for a built-in workflow",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp schedule create drift-scan --cron "0 3 * * *" \
    --config '{"repo_path":"/path/to/repo","judge_articles_per_run":5}'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		workflowType := args[0]
		cronExpr, _ := cmd.Flags().GetString("cron")
		configFlag, _ := cmd.Flags().GetString("config")

		if err := client.ValidateScheduleWorkflowType(workflowType); err != nil {
			return err
		}
		if cronExpr == "" {
			return fmt.Errorf("--cron is required")
		}

		nextRunAt, err := parseScheduleExpr(cronExpr, time.Now())
		if err != nil {
			return err
		}

		configJSON, err := validateScheduleConfig(workflowType, configFlag)
		if err != nil {
			return err
		}

		schedule, err := cpClient.CreateSchedule(ctx, workflowType, workflowType, cronExpr, configJSON, &nextRunAt)
		if err != nil {
			return err
		}

		if outputFormat == "json" || outputFormat == "yaml" {
			s, err := client.FormatOutput(schedule, outputFormat)
			if err != nil {
				return err
			}
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Created schedule %s for project %s\n", schedule.Name, schedule.Project)
		return nil
	},
}

var scheduleEnableCmd = &cobra.Command{
	Use:     "enable <name>",
	Short:   "Enable a schedule",
	Args:    cobra.ExactArgs(1),
	Example: "  cxp schedule enable drift-scan",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		schedules, err := cpClient.ListSchedules(ctx)
		if err != nil {
			return err
		}

		var target *client.Schedule
		for i := range schedules {
			if schedules[i].Name == name {
				target = &schedules[i]
				break
			}
		}
		if target == nil {
			return fmt.Errorf("schedule not found: %s", name)
		}

		nextRunAt, err := parseScheduleExpr(target.ScheduleExpr, time.Now())
		if err != nil {
			return err
		}
		if err := cpClient.SetScheduleEnabled(ctx, name, true, &nextRunAt); err != nil {
			return err
		}

		fmt.Printf("Enabled schedule %s\n", name)
		return nil
	},
}

var scheduleDisableCmd = &cobra.Command{
	Use:     "disable <name>",
	Short:   "Disable a schedule",
	Args:    cobra.ExactArgs(1),
	Example: "  cxp schedule disable canary",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		if err := cpClient.SetScheduleEnabled(ctx, name, false, nil); err != nil {
			return err
		}

		fmt.Printf("Disabled schedule %s\n", name)
		return nil
	},
}

var scheduleHistoryCmd = &cobra.Command{
	Use:     "history <name>",
	Short:   "Show recent run history for a schedule",
	Args:    cobra.ExactArgs(1),
	Example: "  cxp schedule history drift-scan --limit 10",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]
		historyLimit, _ := cmd.Flags().GetInt("limit")

		runs, err := cpClient.GetScheduleHistory(ctx, name, historyLimit)
		if err != nil {
			return err
		}

		if outputFormat == "json" || outputFormat == "yaml" {
			s, err := client.FormatOutput(runs, outputFormat)
			if err != nil {
				return err
			}
			fmt.Println(s)
			return nil
		}

		if len(runs) == 0 {
			fmt.Println("No runs found.")
			return nil
		}

		tbl := client.NewTable("ID", "STATUS", "STARTED", "FINISHED", "SUMMARY")
		for _, run := range runs {
			tbl.AddRow(
				fmt.Sprintf("%d", run.ID),
				run.Status,
				run.StartedAt.Format(time.RFC3339),
				formatScheduleTime(run.FinishedAt),
				nullableString(run.Summary),
			)
		}
		fmt.Print(tbl.String())
		return nil
	},
}

var scheduleLastCmd = &cobra.Command{
	Use:     "last <name>",
	Short:   "Show the most recent run for a schedule",
	Args:    cobra.ExactArgs(1),
	Example: "  cxp schedule last drift-scan",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		run, err := cpClient.GetLastScheduleRun(ctx, name)
		if err != nil {
			return err
		}

		if outputFormat == "json" || outputFormat == "yaml" {
			s, err := client.FormatOutput(run, outputFormat)
			if err != nil {
				return err
			}
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Schedule: %s\n", name)
		fmt.Printf("Run ID: %d\n", run.ID)
		fmt.Printf("Workflow: %s\n", run.WorkflowType)
		fmt.Printf("Status: %s\n", run.Status)
		fmt.Printf("Started: %s\n", run.StartedAt.Format(time.RFC3339))
		fmt.Printf("Finished: %s\n", formatScheduleTime(run.FinishedAt))
		fmt.Printf("Summary: %s\n", nullableString(run.Summary))
		fmt.Printf("Result:\n%s\n", prettyJSON(run.Result))
		return nil
	},
}

func parseScheduleExpr(expr string, now time.Time) (time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(expr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid cron expression %q: %v", expr, err)
	}
	return schedule.Next(now.UTC()), nil
}

func validateScheduleConfig(workflowType, raw string) (json.RawMessage, error) {
	if raw == "" {
		return nil, fmt.Errorf("--config is required")
	}

	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()

	switch workflowType {
	case client.ScheduleWorkflowDriftScan:
		var cfg struct {
			RepoPath            string `json:"repo_path,omitempty"`
			FactcheckModel      string `json:"factcheck_model,omitempty"`
			JudgeModel          string `json:"judge_model,omitempty"`
			JudgeArticlesPerRun *int   `json:"judge_articles_per_run,omitempty"`
		}
		if err := decoder.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("invalid drift-scan config: %v", err)
		}
	case client.ScheduleWorkflowCanary:
		var cfg struct {
			CanaryShard string   `json:"canary_shard,omitempty"`
			AgentModel  string   `json:"agent_model,omitempty"`
			AgentTools  []string `json:"agent_tools,omitempty"`
		}
		if err := decoder.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("invalid canary config: %v", err)
		}
	case client.ScheduleWorkflowTriage:
		var cfg struct {
			GapsShard        string `json:"gaps_shard,omitempty"`
			EscalationsShard string `json:"escalations_shard,omitempty"`
			TriageModel      string `json:"triage_model,omitempty"`
		}
		if err := decoder.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("invalid triage config: %v", err)
		}
	default:
		return nil, fmt.Errorf("invalid workflow type %q", workflowType)
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(raw)); err != nil {
		return nil, fmt.Errorf("invalid config JSON: %v", err)
	}
	return json.RawMessage(compact.Bytes()), nil
}

func formatScheduleTime(ts *time.Time) string {
	if ts == nil || ts.IsZero() {
		return "-"
	}
	return ts.UTC().Format(time.RFC3339)
}

func nullableString(v *string) string {
	if v == nil || *v == "" {
		return "-"
	}
	return *v
}

func prettyJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return string(raw)
	}
	return out.String()
}

func init() {
	scheduleCreateCmd.Flags().String("cron", "", "Cron expression for the schedule")
	scheduleCreateCmd.Flags().String("config", "", "Workflow config JSON")
	scheduleHistoryCmd.Flags().Int("limit", 10, "Maximum number of runs to show")

	scheduleCmd.AddCommand(scheduleListCmd)
	scheduleCmd.AddCommand(scheduleCreateCmd)
	scheduleCmd.AddCommand(scheduleEnableCmd)
	scheduleCmd.AddCommand(scheduleDisableCmd)
	scheduleCmd.AddCommand(scheduleHistoryCmd)
	scheduleCmd.AddCommand(scheduleLastCmd)
	rootCmd.AddCommand(scheduleCmd)
}
