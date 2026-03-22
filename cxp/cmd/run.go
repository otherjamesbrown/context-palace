package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var (
	runTaskID            string
	runBranch            string
	runRuntime           string
	runArtifactType      string
	runArtifactRef       string
	runStartStatus       string
	runStartSummary      string
	runObservationRole   string
	runObservationSource string
	runObservationDetail string
	runObservationConf   float64
	runCompleteStatus    string
	runCompleteSummary   string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Task-run evidence recording and inspection",
	Long:  `Commands for starting, recording, inspecting, and completing task runs.`,
}

var runStartCmd = &cobra.Command{
	Use:     "start <shard-id>",
	Short:   "Start a task run for a shard",
	Args:    cobra.ExactArgs(1),
	Example: `  cxp run start pf-123 --runtime claude_code --artifact-type launch_pack`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		run, err := cpClient.StartTaskRun(ctx, client.StartTaskRunInput{
			Project:            cpClient.Config.Project,
			TaskID:             runTaskID,
			ShardID:            args[0],
			Branch:             runBranch,
			AgentRuntime:       runRuntime,
			AgentName:          cpClient.Config.Agent,
			LaunchArtifactType: runArtifactType,
			LaunchArtifactRef:  runArtifactRef,
			Status:             runStartStatus,
			Summary:            runStartSummary,
		})
		if err != nil {
			return err
		}
		return printRunOutput(run)
	},
}

var runObserveCmd = &cobra.Command{
	Use:     "observe <run-id> <type> <subject-type> <subject-ref>",
	Short:   "Record an observation for a task run",
	Args:    cobra.ExactArgs(4),
	Example: `  cxp run observe tr-123 correct_subsystem_reached package pkg/digest --role final_route --detail '{"reason":"shared digest behavior"}'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		var details json.RawMessage
		if runObservationDetail != "" {
			if !json.Valid([]byte(runObservationDetail)) {
				return fmt.Errorf("--detail must be valid JSON")
			}
			details = json.RawMessage(runObservationDetail)
		}

		var confidence *float64
		if cmd.Flags().Changed("confidence") {
			confidence = &runObservationConf
		}

		obs, err := cpClient.RecordTaskObservation(ctx, client.RecordObservationInput{
			TaskRunID:       args[0],
			ObservationType: args[1],
			SubjectType:     args[2],
			SubjectRef:      args[3],
			Role:            runObservationRole,
			Confidence:      confidence,
			Source:          runObservationSource,
			Details:         details,
		})
		if err != nil {
			return err
		}
		return printObservationOutput(obs)
	},
}

var runShowCmd = &cobra.Command{
	Use:     "show <run-id>",
	Short:   "Show a task run and its observations",
	Args:    cobra.ExactArgs(1),
	Example: `  cxp run show tr-123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		run, err := cpClient.GetTaskRun(ctx, args[0])
		if err != nil {
			return err
		}
		observations, err := cpClient.ListTaskRunObservations(ctx, args[0])
		if err != nil {
			return err
		}

		if outputFormat == "json" || outputFormat == "yaml" {
			out := map[string]any{
				"run":          run,
				"observations": observations,
			}
			s, _ := client.FormatOutput(out, outputFormat)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Run:       %s\n", run.ID)
		fmt.Printf("Project:   %s\n", run.Project)
		if run.ShardID != "" {
			fmt.Printf("Shard:     %s\n", run.ShardID)
		}
		if run.TaskID != "" {
			fmt.Printf("Task ID:   %s\n", run.TaskID)
		}
		fmt.Printf("Runtime:   %s\n", run.AgentRuntime)
		if run.AgentName != "" {
			fmt.Printf("Agent:     %s\n", run.AgentName)
		}
		fmt.Printf("Artifact:  %s\n", run.LaunchArtifactType)
		if run.LaunchArtifactRef != "" {
			fmt.Printf("Artifact Ref: %s\n", run.LaunchArtifactRef)
		}
		fmt.Printf("Status:    %s\n", run.Status)
		fmt.Printf("Started:   %s\n", run.StartedAt.Format("2006-01-02 15:04:05"))
		if run.FinishedAt != nil {
			fmt.Printf("Finished:  %s\n", run.FinishedAt.Format("2006-01-02 15:04:05"))
		}
		if run.Summary != "" {
			fmt.Printf("Summary:   %s\n", run.Summary)
		}

		fmt.Printf("\nObservations (%d)\n", len(observations))
		if len(observations) == 0 {
			fmt.Println("  (none)")
			return nil
		}
		table := client.NewTable("TIME", "TYPE", "SUBJECT", "ROLE", "SOURCE")
		for _, obs := range observations {
			table.AddRow(
				obs.ObservedAt.Format("15:04:05"),
				obs.ObservationType,
				fmt.Sprintf("%s:%s", obs.SubjectType, client.Truncate(obs.SubjectRef, 40)),
				obs.Role,
				obs.Source,
			)
		}
		fmt.Print(table.String())
		return nil
	},
}

var runListCmd = &cobra.Command{
	Use:     "list <shard-id>",
	Short:   "List recent task runs for a shard",
	Args:    cobra.ExactArgs(1),
	Example: `  cxp run list pf-123 --limit 10`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		runs, err := cpClient.ListTaskRunsForShard(ctx, args[0], limitFlag)
		if err != nil {
			return err
		}

		if outputFormat == "json" || outputFormat == "yaml" {
			s, _ := client.FormatOutput(runs, outputFormat)
			fmt.Println(s)
			return nil
		}

		if len(runs) == 0 {
			fmt.Printf("No task runs found for %s\n", args[0])
			return nil
		}

		table := client.NewTable("RUN ID", "RUNTIME", "ARTIFACT", "STATUS", "STARTED")
		for _, run := range runs {
			table.AddRow(
				run.ID,
				run.AgentRuntime,
				run.LaunchArtifactType,
				run.Status,
				timeAgo(run.StartedAt),
			)
		}
		fmt.Print(table.String())
		return nil
	},
}

var runCompleteCmd = &cobra.Command{
	Use:     "complete <run-id>",
	Short:   "Complete a task run",
	Args:    cobra.ExactArgs(1),
	Example: `  cxp run complete tr-123 --summary "Reached correct proof surface"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		if err := cpClient.CompleteTaskRun(ctx, args[0], client.CompleteTaskRunInput{
			Status:  runCompleteStatus,
			Summary: runCompleteSummary,
		}); err != nil {
			return err
		}

		if outputFormat == "json" || outputFormat == "yaml" {
			out := map[string]any{
				"id":      args[0],
				"status":  valueOrDefault(runCompleteStatus, "completed"),
				"summary": runCompleteSummary,
			}
			s, _ := client.FormatOutput(out, outputFormat)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Completed task run %s\n", args[0])
		return nil
	},
}

func printRunOutput(run *client.TaskRun) error {
	if outputFormat == "json" || outputFormat == "yaml" {
		s, _ := client.FormatOutput(run, outputFormat)
		fmt.Println(s)
		return nil
	}

	fmt.Printf("Started task run %s\n", run.ID)
	fmt.Printf("  Project:  %s\n", run.Project)
	fmt.Printf("  Shard:    %s\n", run.ShardID)
	fmt.Printf("  Runtime:  %s\n", run.AgentRuntime)
	fmt.Printf("  Artifact: %s\n", run.LaunchArtifactType)
	if run.LaunchArtifactRef != "" {
		fmt.Printf("  Ref:      %s\n", run.LaunchArtifactRef)
	}
	return nil
}

func printObservationOutput(obs *client.TaskRunObservation) error {
	if outputFormat == "json" || outputFormat == "yaml" {
		s, _ := client.FormatOutput(obs, outputFormat)
		fmt.Println(s)
		return nil
	}

	fmt.Printf("Recorded observation %s\n", obs.ID)
	fmt.Printf("  Run:     %s\n", obs.TaskRunID)
	fmt.Printf("  Type:    %s\n", obs.ObservationType)
	fmt.Printf("  Subject: %s:%s\n", obs.SubjectType, obs.SubjectRef)
	if obs.Role != "" {
		fmt.Printf("  Role:    %s\n", obs.Role)
	}
	if obs.Confidence != nil {
		fmt.Printf("  Confidence: %.3f\n", *obs.Confidence)
	}
	return nil
}

func valueOrDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func init() {
	runStartCmd.Flags().StringVar(&runTaskID, "task-id", "", "External task identifier")
	runStartCmd.Flags().StringVar(&runBranch, "branch", "", "Branch name for this run")
	runStartCmd.Flags().StringVar(&runRuntime, "runtime", "cxp", "Agent runtime (e.g. cxp, claude_code)")
	runStartCmd.Flags().StringVar(&runArtifactType, "artifact-type", client.LaunchArtifactNone, "Launch artifact type (none|launcher|launch_pack|manual)")
	runStartCmd.Flags().StringVar(&runArtifactRef, "artifact-ref", "", "Reference to the compiled launcher or pack")
	runStartCmd.Flags().StringVar(&runStartStatus, "status", "started", "Initial task run status")
	runStartCmd.Flags().StringVar(&runStartSummary, "summary", "", "Optional initial run summary")

	runObserveCmd.Flags().StringVar(&runObservationRole, "role", "", "Role of this observation")
	runObserveCmd.Flags().StringVar(&runObservationSource, "source", "cxp", "Observation source")
	runObserveCmd.Flags().StringVar(&runObservationDetail, "detail", "", "JSON detail payload")
	runObserveCmd.Flags().Float64Var(&runObservationConf, "confidence", 0, "Confidence in the observation (0..1)")

	runCompleteCmd.Flags().StringVar(&runCompleteStatus, "status", "completed", "Terminal task run status")
	runCompleteCmd.Flags().StringVar(&runCompleteSummary, "summary", "", "Completion summary")

	runCmd.AddCommand(runStartCmd)
	runCmd.AddCommand(runObserveCmd)
	runCmd.AddCommand(runShowCmd)
	runCmd.AddCommand(runListCmd)
	runCmd.AddCommand(runCompleteCmd)
	rootCmd.AddCommand(runCmd)
}
