package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/otherjamesbrown/context-palace/cxp/internal/workflows"
	"github.com/spf13/cobra"
)

var (
	scheduleCronFlag     string
	scheduleConfigFlag   string
	scheduleWorkflowFlag string
	scheduleOverlapFlag  string
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage scheduled workflows for the current project",
}

var scheduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List schedules for the current project",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		schedules, err := cpClient.ListSchedules(ctx, cpClient.Config.Project)
		if err != nil {
			return err
		}

		if outputFormat == "json" || outputFormat == "yaml" {
			s, _ := client.FormatOutput(schedules, outputFormat)
			fmt.Println(s)
			return nil
		}

		if len(schedules) == 0 {
			fmt.Printf("No schedules found for %s\n", cpClient.Config.Project)
			return nil
		}

		table := client.NewTable("NAME", "WORKFLOW", "CRON", "ENABLED", "NEXT RUN")
		for _, schedule := range schedules {
			nextRun := "-"
			if schedule.NextRunAt != nil {
				nextRun = schedule.NextRunAt.Local().Format("2006-01-02 15:04")
			}
			table.AddRow(
				schedule.Name,
				schedule.WorkflowType,
				schedule.ScheduleExpr,
				fmt.Sprintf("%t", schedule.Enabled),
				nextRun,
			)
		}
		fmt.Print(table.String())
		return nil
	},
}

var scheduleCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a schedule for the current project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		if scheduleCronFlag == "" {
			return fmt.Errorf("--cron is required")
		}

		config, err := parseScheduleConfig(scheduleConfigFlag)
		if err != nil {
			return err
		}

		workflowType := scheduleWorkflowFlag
		if workflowType == "" {
			workflowType = args[0]
		}

		schedule, err := cpClient.CreateSchedule(ctx, client.CreateScheduleInput{
			Project:       cpClient.Config.Project,
			Name:          args[0],
			WorkflowType:  workflowType,
			ScheduleExpr:  scheduleCronFlag,
			OverlapPolicy: scheduleOverlapFlag,
			Config:        config,
		})
		if err != nil {
			return err
		}

		return printScheduleOutput(schedule, "Created schedule")
	},
}

var scheduleEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Enable a schedule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		schedule, err := cpClient.EnableSchedule(ctx, cpClient.Config.Project, args[0])
		if err != nil {
			return err
		}
		return printScheduleOutput(schedule, "Enabled schedule")
	},
}

var scheduleDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Disable a schedule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		schedule, err := cpClient.DisableSchedule(ctx, cpClient.Config.Project, args[0])
		if err != nil {
			return err
		}
		return printScheduleOutput(schedule, "Disabled schedule")
	},
}

var scheduleRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Run a schedule in the foreground and record the execution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		schedule, err := cpClient.GetSchedule(ctx, cpClient.Config.Project, args[0])
		if err != nil {
			return err
		}

		if schedule.OverlapPolicy == client.ScheduleOverlapSkip {
			running, err := cpClient.HasRunningScheduleRun(ctx, schedule.ID)
			if err != nil {
				return err
			}
			if running {
				if outputFormat == "json" || outputFormat == "yaml" {
					out := map[string]any{
						"schedule": schedule.Name,
						"status":   "skipped",
						"reason":   "another run is already in progress",
					}
					s, _ := client.FormatOutput(out, outputFormat)
					fmt.Println(s)
					return nil
				}
				fmt.Printf("Skipped schedule %s: another run is already in progress\n", schedule.Name)
				return nil
			}
		}

		run, err := cpClient.CreateScheduleRun(ctx, client.CreateScheduleRunInput{
			ScheduleID:   schedule.ID,
			Project:      schedule.Project,
			WorkflowType: schedule.WorkflowType,
			Status:       client.ScheduleRunStatusRunning,
			Result:       json.RawMessage(`{}`),
		})
		if err != nil {
			return err
		}

		summary, result, workflowErr := runScheduleWorkflow(ctx, schedule, run.ID)
		status := client.ScheduleRunStatusCompleted
		if workflowErr != nil {
			status = client.ScheduleRunStatusFailed
			if summary == "" {
				summary = workflowErr.Error()
			}
			if len(result) == 0 {
				result = scheduleRunErrorResult(workflowErr)
			}
		}

		run, err = cpClient.UpdateScheduleRun(ctx, run.ID, client.UpdateScheduleRunInput{
			Status:     status,
			FinishedAt: timePtr(time.Now().UTC()),
			Summary:    summary,
			Result:     result,
		})
		if err != nil {
			return err
		}

		if outputFormat == "json" || outputFormat == "yaml" {
			out := map[string]any{
				"schedule": schedule,
				"run":      run,
			}
			s, _ := client.FormatOutput(out, outputFormat)
			fmt.Println(s)
			return nil
		}

		if workflowErr != nil {
			fmt.Printf("Schedule %s run %d failed: %s\n", schedule.Name, run.ID, summary)
			return nil
		}

		fmt.Printf("Schedule %s run %d completed\n", schedule.Name, run.ID)
		return nil
	},
}

var scheduleHistoryCmd = &cobra.Command{
	Use:   "history <name>",
	Short: "Show recent runs for a schedule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		schedule, err := cpClient.GetSchedule(ctx, cpClient.Config.Project, args[0])
		if err != nil {
			return err
		}

		runs, err := cpClient.ListScheduleRuns(ctx, schedule.ID, limitFlag)
		if err != nil {
			return err
		}

		if outputFormat == "json" || outputFormat == "yaml" {
			s, _ := client.FormatOutput(runs, outputFormat)
			fmt.Println(s)
			return nil
		}

		if len(runs) == 0 {
			fmt.Printf("No runs found for schedule %s\n", schedule.Name)
			return nil
		}

		table := client.NewTable("RUN ID", "STATUS", "STARTED", "FINISHED", "SUMMARY")
		for _, run := range runs {
			finished := "-"
			if run.FinishedAt != nil {
				finished = run.FinishedAt.Local().Format("2006-01-02 15:04:05")
			}
			table.AddRow(
				fmt.Sprintf("%d", run.ID),
				run.Status,
				run.StartedAt.Local().Format("2006-01-02 15:04:05"),
				finished,
				client.Truncate(run.Summary, 60),
			)
		}
		fmt.Print(table.String())
		return nil
	},
}

var scheduleLastCmd = &cobra.Command{
	Use:   "last <name>",
	Short: "Show the most recent run for a schedule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		schedule, err := cpClient.GetSchedule(ctx, cpClient.Config.Project, args[0])
		if err != nil {
			return err
		}

		run, err := cpClient.GetLastScheduleRun(ctx, schedule.ID)
		if err != nil {
			return err
		}

		if outputFormat == "json" || outputFormat == "yaml" {
			s, _ := client.FormatOutput(run, outputFormat)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Schedule:  %s\n", schedule.Name)
		fmt.Printf("Run ID:    %d\n", run.ID)
		fmt.Printf("Status:    %s\n", run.Status)
		fmt.Printf("Started:   %s\n", run.StartedAt.Local().Format("2006-01-02 15:04:05"))
		if run.FinishedAt != nil {
			fmt.Printf("Finished:  %s\n", run.FinishedAt.Local().Format("2006-01-02 15:04:05"))
		}
		if run.Summary != "" {
			fmt.Printf("Summary:   %s\n", run.Summary)
		}
		if len(run.Result) > 0 {
			fmt.Printf("Result:    %s\n", string(run.Result))
		}
		return nil
	},
}

func runScheduleWorkflow(ctx context.Context, schedule *client.Schedule, runID int) (string, json.RawMessage, error) {
	db, err := openScheduleDB(ctx)
	if err != nil {
		return "", nil, err
	}
	defer db.Close()

	switch schedule.WorkflowType {
	case "canary":
		var cfg workflows.CanaryConfig
		if len(schedule.Config) > 0 {
			if err := json.Unmarshal(schedule.Config, &cfg); err != nil {
				return "", nil, fmt.Errorf("parse canary config: %w", err)
			}
		}
		cfg.Project = schedule.Project
		cfg.RunID = runID

		result, err := workflows.RunCanary(ctx, db, cfg)
		if err != nil {
			return "", nil, err
		}

		payload, err := json.Marshal(result)
		if err != nil {
			return "", nil, fmt.Errorf("marshal canary result: %w", err)
		}

		summary := fmt.Sprintf(
			"Canary completed: %d tested, %d passed, %d failed",
			result.QuestionsTested,
			result.Passed,
			result.Failed,
		)
		return summary, payload, nil
	default:
		return "", nil, errors.New("workflow not implemented")
	}
}

func scheduleRunErrorResult(err error) json.RawMessage {
	payload, marshalErr := json.Marshal(map[string]any{
		"error": err.Error(),
	})
	if marshalErr != nil {
		return json.RawMessage(`{"error":"workflow failed"}`)
	}
	return payload
}

func parseScheduleConfig(raw string) (json.RawMessage, error) {
	if raw == "" {
		return json.RawMessage(`{}`), nil
	}
	if !json.Valid([]byte(raw)) {
		return nil, fmt.Errorf("--config must be valid JSON")
	}
	return json.RawMessage(raw), nil
}

func printScheduleOutput(schedule *client.Schedule, prefix string) error {
	if outputFormat == "json" || outputFormat == "yaml" {
		s, _ := client.FormatOutput(schedule, outputFormat)
		fmt.Println(s)
		return nil
	}

	fmt.Printf("%s %s\n", prefix, schedule.Name)
	fmt.Printf("  Workflow: %s\n", schedule.WorkflowType)
	fmt.Printf("  Enabled:  %t\n", schedule.Enabled)
	fmt.Printf("  Cron:     %s\n", schedule.ScheduleExpr)
	if schedule.NextRunAt != nil {
		fmt.Printf("  Next run: %s\n", schedule.NextRunAt.Local().Format("2006-01-02 15:04:05"))
	}
	return nil
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func openScheduleDB(ctx context.Context) (*sql.DB, error) {
	connConfig, err := pgx.ParseConfig(cpClient.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}
	db := stdlib.OpenDB(*connConfig)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect schedule database: %w", err)
	}
	return db, nil
}

func init() {
	scheduleCreateCmd.Flags().StringVar(&scheduleCronFlag, "cron", "", "Cron expression for the schedule")
	scheduleCreateCmd.Flags().StringVar(&scheduleConfigFlag, "config", "{}", "Schedule config JSON")
	scheduleCreateCmd.Flags().StringVar(&scheduleWorkflowFlag, "workflow", "", "Workflow type override (defaults to schedule name)")
	scheduleCreateCmd.Flags().StringVar(&scheduleOverlapFlag, "overlap-policy", client.ScheduleOverlapSkip, "Overlap policy (skip|allow|cancel_running)")

	scheduleCmd.AddCommand(scheduleListCmd)
	scheduleCmd.AddCommand(scheduleCreateCmd)
	scheduleCmd.AddCommand(scheduleEnableCmd)
	scheduleCmd.AddCommand(scheduleDisableCmd)
	scheduleCmd.AddCommand(scheduleRunCmd)
	scheduleCmd.AddCommand(scheduleHistoryCmd)
	scheduleCmd.AddCommand(scheduleLastCmd)

	rootCmd.AddCommand(scheduleCmd)
}
