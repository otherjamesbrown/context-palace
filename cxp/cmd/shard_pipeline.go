package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var shardPipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Pipeline metadata operations on design shards",
	Long:  `Initialise, view, and update pipeline state stored in design shard metadata.`,
}

var shardPipelineInitCmd = &cobra.Command{
	Use:   "init <design-id>",
	Short: "Initialise pipeline metadata on a design shard",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp shard pipeline init pf-design-123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		state, err := cpClient.PipelineInit(ctx, id)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":       id,
				"pipeline": state,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Initialised pipeline on %s\n", id)
		fmt.Printf("  Phase:    %s\n", state.Phase)
		fmt.Printf("  Progress: %s\n", state.LastProgress)
		return nil
	},
}

var shardPipelineShowCmd = &cobra.Command{
	Use:   "show <design-id>",
	Short: "Display current pipeline state",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp shard pipeline show pf-design-123
  cxp shard pipeline show pf-design-123 -o json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		state, err := cpClient.PipelineGet(ctx, id)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":       id,
				"pipeline": state,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Pipeline: %s\n", id)
		fmt.Printf("  Phase:          %s\n", state.Phase)
		if state.LockedBy != nil {
			fmt.Printf("  Locked by:      %s\n", *state.LockedBy)
		}
		if state.LockExpires != nil {
			fmt.Printf("  Lock expires:   %s\n", state.LockExpires.Format("2006-01-02 15:04"))
		}
		if len(state.WaitingFor) > 0 {
			fmt.Printf("  Waiting for:    %s\n", strings.Join(state.WaitingFor, ", "))
		}
		fmt.Printf("  Last progress:  %s\n", state.LastProgress)
		if len(state.TaskShards) > 0 {
			fmt.Printf("  Task shards:    %s\n", strings.Join(state.TaskShards, ", "))
		}
		fmt.Printf("  Tokens:         %d\n", state.CumulativeTokens)
		if len(state.IterationCounts) > 0 {
			parts := make([]string, 0, len(state.IterationCounts))
			for phase, count := range state.IterationCounts {
				parts = append(parts, fmt.Sprintf("%s=%d", phase, count))
			}
			fmt.Printf("  Iterations:     %s\n", strings.Join(parts, ", "))
		}
		return nil
	},
}

var shardPipelineUpdateCmd = &cobra.Command{
	Use:   "update <design-id>",
	Short: "Update pipeline state on a design shard",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp shard pipeline update pf-design-123 --phase implement
  cxp shard pipeline update pf-design-123 --add-task pf-task-456
  cxp shard pipeline update pf-design-123 --tokens 1500
  cxp shard pipeline update pf-design-123 --waiting-for '["pf-review-1"]'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		phaseFlag, _ := cmd.Flags().GetString("phase")
		waitingForFlag, _ := cmd.Flags().GetString("waiting-for")
		addTaskFlag, _ := cmd.Flags().GetString("add-task")
		tokensFlag, _ := cmd.Flags().GetInt("tokens")

		// Ensure at least one flag is provided
		if phaseFlag == "" && waitingForFlag == "" && addTaskFlag == "" && tokensFlag == 0 {
			return fmt.Errorf("at least one of --phase, --waiting-for, --add-task, or --tokens is required")
		}

		var phase *string
		if phaseFlag != "" {
			phase = &phaseFlag
		}

		var waitingFor *json.RawMessage
		if waitingForFlag != "" {
			raw := json.RawMessage(waitingForFlag)
			waitingFor = &raw
		}

		var addTask *string
		if addTaskFlag != "" {
			addTask = &addTaskFlag
		}

		var addTokens *int
		if tokensFlag != 0 {
			addTokens = &tokensFlag
		}

		state, err := cpClient.PipelineUpdate(ctx, id, phase, waitingFor, addTask, addTokens)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":       id,
				"pipeline": state,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Updated pipeline on %s\n", id)
		fmt.Printf("  Phase:    %s\n", state.Phase)
		fmt.Printf("  Tokens:   %d\n", state.CumulativeTokens)
		if len(state.TaskShards) > 0 {
			fmt.Printf("  Tasks:    %s\n", strings.Join(state.TaskShards, ", "))
		}
		return nil
	},
}

var shardPipelineLockCmd = &cobra.Command{
	Use:   "lock <design-id>",
	Short: "Acquire pipeline lock",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp shard pipeline lock pf-design-123
  cxp shard pipeline lock pf-design-123 --session agent-steve-1711100000`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		sessionID, _ := cmd.Flags().GetString("session")
		if sessionID == "" {
			sessionID = fmt.Sprintf("%s-%d", cpClient.Config.Agent, time.Now().Unix())
		}

		state, err := cpClient.PipelineLock(ctx, id, sessionID)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":       id,
				"pipeline": state,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Locked pipeline on %s\n", id)
		fmt.Printf("  Locked by:    %s\n", *state.LockedBy)
		fmt.Printf("  Lock expires: %s\n", state.LockExpires.Format("2006-01-02 15:04"))
		return nil
	},
}

var shardPipelineUnlockCmd = &cobra.Command{
	Use:     "unlock <design-id>",
	Short:   "Release pipeline lock",
	Args:    cobra.ExactArgs(1),
	Example: `  cxp shard pipeline unlock pf-design-123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		state, err := cpClient.PipelineUnlock(ctx, id)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":       id,
				"pipeline": state,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Unlocked pipeline on %s\n", id)
		fmt.Printf("  Phase: %s\n", state.Phase)
		return nil
	},
}

var shardPipelineLockCheckCmd = &cobra.Command{
	Use:     "lock-check <design-id>",
	Short:   "Check pipeline lock status",
	Args:    cobra.ExactArgs(1),
	Example: `  cxp shard pipeline lock-check pf-design-123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		id := args[0]

		status, state, err := cpClient.PipelineLockCheck(ctx, id)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			out := map[string]any{
				"id":          id,
				"lock_status": status,
				"pipeline":    state,
			}
			s, _ := client.FormatJSON(out)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Pipeline %s: %s\n", id, status)
		if state.LockedBy != nil {
			fmt.Printf("  Locked by:    %s\n", *state.LockedBy)
		}
		if state.LockExpires != nil {
			fmt.Printf("  Lock expires: %s\n", state.LockExpires.Format("2006-01-02 15:04"))
		}
		return nil
	},
}

var shardPipelineReviewCmd = &cobra.Command{
	Use:   "review <design-id>",
	Short: "Record Phase 1 readiness review verdict",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp shard pipeline review pf-design-123 --verdict pass --readiness 4 --body "All criteria met."
  cxp shard pipeline review pf-design-123 --verdict fail --readiness 2 --body-file review-notes.md`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		designID := args[0]

		verdict, _ := cmd.Flags().GetString("verdict")
		readiness, _ := cmd.Flags().GetInt("readiness")
		body, _ := cmd.Flags().GetString("body")
		bodyFile, _ := cmd.Flags().GetString("body-file")

		// Validate verdict
		if verdict == "" {
			return fmt.Errorf("--verdict is required")
		}
		if verdict != "pass" && verdict != "fail" {
			return fmt.Errorf("--verdict must be 'pass' or 'fail', got %q", verdict)
		}

		// Validate readiness
		if readiness < 1 || readiness > 5 {
			return fmt.Errorf("--readiness must be 1-5, got %d", readiness)
		}

		// Resolve body content
		content, err := resolveBody(body, bodyFile)
		if err != nil {
			return err
		}

		result, err := cpClient.PipelineReview(ctx, designID, verdict, readiness, content)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		fmt.Printf("Recorded Phase 1 review for %s\n", result.DesignID)
		fmt.Printf("  Review shard: %s\n", result.ReviewShardID)
		fmt.Printf("  Round:        %d\n", result.Round)
		phaseTransition := result.Phase
		if result.Verdict == "pass" {
			phaseTransition = "design → decompose"
		}
		fmt.Printf("  Verdict:      %s (%d/5)\n", result.Verdict, result.Readiness)
		fmt.Printf("  Phase:        %s\n", phaseTransition)
		return nil
	},
}

var shardPipelineDecomposeCmd = &cobra.Command{
	Use:   "decompose <design-id>",
	Short: "Record Phase 2 decomposition verdict",
	Args:  cobra.ExactArgs(1),
	Example: `  cxp shard pipeline decompose pf-design-123 --verdict pass --body "Tasks are well-defined."
  cxp shard pipeline decompose pf-design-123 --verdict fail --body-file decompose-notes.md`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		designID := args[0]

		verdict, _ := cmd.Flags().GetString("verdict")
		body, _ := cmd.Flags().GetString("body")
		bodyFile, _ := cmd.Flags().GetString("body-file")

		// Validate verdict
		if verdict == "" {
			return fmt.Errorf("--verdict is required")
		}
		if verdict != "pass" && verdict != "fail" {
			return fmt.Errorf("--verdict must be 'pass' or 'fail', got %q", verdict)
		}

		// Resolve body content
		content, err := resolveBody(body, bodyFile)
		if err != nil {
			return err
		}

		result, err := cpClient.PipelineDecompose(ctx, designID, verdict, content)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(result)
			fmt.Println(s)
			return nil
		}

		phaseTransition := result.Phase
		if result.Verdict == "pass" {
			phaseTransition = "decompose → implement"
		}
		fmt.Printf("Recorded Phase 2 decomposition for %s\n", result.DesignID)
		fmt.Printf("  Decompose shard: %s\n", result.DecomposeShardID)
		fmt.Printf("  Round:           %d\n", result.Round)
		fmt.Printf("  Verdict:         %s\n", result.Verdict)
		fmt.Printf("  Tasks:           %d\n", result.TaskCount)
		fmt.Printf("  Phase:           %s\n", phaseTransition)
		return nil
	},
}

func init() {
	// pipeline update flags
	shardPipelineUpdateCmd.Flags().String("phase", "", "Pipeline phase (design, decompose, implement, review, deploy, done)")
	shardPipelineUpdateCmd.Flags().String("waiting-for", "", "JSON array of shard IDs to wait for")
	shardPipelineUpdateCmd.Flags().String("add-task", "", "Shard ID to append to task_shards")
	shardPipelineUpdateCmd.Flags().Int("tokens", 0, "Token count to add to cumulative_tokens")

	// pipeline review flags
	shardPipelineReviewCmd.Flags().String("verdict", "", "Review verdict: 'pass' or 'fail' (required)")
	shardPipelineReviewCmd.Flags().Int("readiness", 0, "Readiness score 1-5 (required)")
	shardPipelineReviewCmd.Flags().String("body", "", "Findings text")
	shardPipelineReviewCmd.Flags().String("body-file", "", "Read findings from file")
	_ = shardPipelineReviewCmd.MarkFlagRequired("verdict")
	_ = shardPipelineReviewCmd.MarkFlagRequired("readiness")

	// pipeline decompose flags
	shardPipelineDecomposeCmd.Flags().String("verdict", "", "Decomposition verdict: 'pass' or 'fail' (required)")
	shardPipelineDecomposeCmd.Flags().String("body", "", "Findings text")
	shardPipelineDecomposeCmd.Flags().String("body-file", "", "Read findings from file")
	_ = shardPipelineDecomposeCmd.MarkFlagRequired("verdict")

	// pipeline lock flags
	shardPipelineLockCmd.Flags().String("session", "", "Session ID for the lock (defaults to agent name + timestamp)")

	// Wire command tree
	shardPipelineCmd.AddCommand(shardPipelineInitCmd)
	shardPipelineCmd.AddCommand(shardPipelineShowCmd)
	shardPipelineCmd.AddCommand(shardPipelineUpdateCmd)
	shardPipelineCmd.AddCommand(shardPipelineLockCmd)
	shardPipelineCmd.AddCommand(shardPipelineUnlockCmd)
	shardPipelineCmd.AddCommand(shardPipelineLockCheckCmd)
	shardPipelineCmd.AddCommand(shardPipelineReviewCmd)
	shardPipelineCmd.AddCommand(shardPipelineDecomposeCmd)

	shardCmd.AddCommand(shardPipelineCmd)
}
