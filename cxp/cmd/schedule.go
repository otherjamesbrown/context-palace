package cmd

import (
	"context"
	"fmt"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/otherjamesbrown/context-palace/cxp/internal/workflows"
	"github.com/spf13/cobra"
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Scheduled KB maintenance workflows",
}

var scheduleRunCmd = &cobra.Command{
	Use:     "run <name>",
	Short:   "Run a configured schedule immediately",
	Args:    cobra.ExactArgs(1),
	Example: "  cxp schedule run drift-scan",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		schedule, err := cpClient.GetScheduleByName(ctx, args[0])
		if err != nil {
			return err
		}

		if _, ok := workflows.Lookup(schedule.WorkflowType); !ok {
			return fmt.Errorf("schedule %s uses unsupported workflow type %s", schedule.Name, schedule.WorkflowType)
		}

		run, err := cpClient.StartScheduleRun(ctx, schedule.ID, schedule.Project, schedule.WorkflowType)
		if err != nil {
			return err
		}

		workflowClient := cloneScheduleClient(cpClient, schedule.Project)
		result, runErr := workflows.Run(ctx, workflowClient, schedule.Project, schedule.WorkflowType, schedule.Config)
		if runErr != nil {
			failPayload := map[string]any{"error": runErr.Error()}
			_ = cpClient.CompleteScheduleRun(ctx, run.ID, "failed", runErr.Error(), failPayload)
			return runErr
		}

		if err := cpClient.CompleteScheduleRun(ctx, run.ID, "completed", result.Summary, result.Result); err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"run_id":        run.ID,
				"schedule_name": schedule.Name,
				"workflow_type": schedule.WorkflowType,
				"status":        "completed",
				"summary":       result.Summary,
				"result":        result.Result,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Println(result.Summary)
		return nil
	},
}

func cloneScheduleClient(base *client.Client, project string) *client.Client {
	cfgCopy := *base.Config
	cfgCopy.Project = project
	next := client.NewClient(&cfgCopy)
	next.EmbedProvider = base.EmbedProvider
	next.Generator = base.Generator
	return next
}

func init() {
	scheduleCmd.AddCommand(scheduleRunCmd)
	rootCmd.AddCommand(scheduleCmd)
}
