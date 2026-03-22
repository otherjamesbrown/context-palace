package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/spf13/cobra"
)

var taskDispatchAllCmd = &cobra.Command{
	Use:   "dispatch-all <design-id>",
	Short: "Dispatch all ready tasks for a design",
	Long:  `Reads the dependency graph for a design and dispatches all tasks that are ready (not blocked, not already running).`,
	Args:  cobra.ExactArgs(1),
	Example: `  cxp task dispatch-all pf-design-123
  cxp task dispatch-all pf-design-123 --dry-run
  cxp task dispatch-all pf-design-123 --max 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		designID := args[0]
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		maxConcurrent, _ := cmd.Flags().GetInt("max")

		// 1. Get the dependency graph
		result, err := cpClient.GetTaskDeps(ctx, designID)
		if err != nil {
			return fmt.Errorf("failed to get task deps for %s: %w", designID, err)
		}

		if len(result.Dispatchable) == 0 {
			fmt.Println("No tasks ready to dispatch.")
			return nil
		}

		// 2. Build a status map from all waves for reporting
		statusCounts := map[string]int{}
		totalTasks := 0
		for _, wave := range result.Waves {
			for _, task := range wave.Tasks {
				statusCounts[task.Status]++
				totalTasks++
			}
		}

		// 3. Filter dispatchable: skip tasks already in_progress, needs-review, or closed
		// Build a quick lookup of task status from the waves
		taskStatus := make(map[string]string)
		for _, wave := range result.Waves {
			for _, task := range wave.Tasks {
				taskStatus[task.ID] = task.Status
			}
		}

		var toDispatch []string
		var skipped []string
		for _, id := range result.Dispatchable {
			status := taskStatus[id]
			if status == "in_progress" || status == "needs-review" || status == "closed" {
				skipped = append(skipped, id)
				continue
			}
			toDispatch = append(toDispatch, id)
		}

		if len(toDispatch) == 0 {
			fmt.Println("All dispatchable tasks are already in progress, under review, or closed.")
			return nil
		}

		// Dry run: show what would happen
		if dryRun {
			fmt.Printf("Would dispatch %d task(s):\n", len(toDispatch))
			for _, id := range toDispatch {
				fmt.Printf("  cxp task dispatch %s\n", id)
			}
			if len(skipped) > 0 {
				fmt.Printf("\nSkipped %d task(s) (already in_progress/needs-review/closed):\n", len(skipped))
				for _, id := range skipped {
					fmt.Printf("  %s (%s)\n", id, taskStatus[id])
				}
			}
			fmt.Printf("\nWaiting: %d  Done: %d  Total: %d\n",
				totalTasks-statusCounts["closed"]-len(toDispatch),
				statusCounts["closed"],
				totalTasks)
			return nil
		}

		// 4. Dispatch with concurrency limit
		sem := make(chan struct{}, maxConcurrent)
		var mu sync.Mutex
		var dispatched []string
		var failures []string

		var wg sync.WaitGroup
		for _, id := range toDispatch {
			wg.Add(1)
			sem <- struct{}{} // acquire semaphore
			go func(taskID string) {
				defer wg.Done()
				defer func() { <-sem }() // release semaphore

				out, err := exec.CommandContext(ctx, "cxp", "task", "dispatch", taskID).CombinedOutput()
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					failures = append(failures, fmt.Sprintf("%s: %s", taskID, string(out)))
				} else {
					dispatched = append(dispatched, taskID)
				}
			}(id)
		}
		wg.Wait()

		// 5. Report
		waiting := totalTasks - statusCounts["closed"] - len(dispatched)
		done := statusCounts["closed"]

		fmt.Printf("%d tasks dispatched, %d waiting, %d done\n",
			len(dispatched), waiting, done)

		if len(failures) > 0 {
			fmt.Printf("\nFailed to dispatch %d task(s):\n", len(failures))
			for _, f := range failures {
				fmt.Printf("  %s\n", f)
			}
		}

		return nil
	},
}

func init() {
	taskDispatchAllCmd.Flags().Bool("dry-run", false, "Show what would be dispatched without executing")
	taskDispatchAllCmd.Flags().Int("max", 3, "Max concurrent dispatches")
	taskCmd.AddCommand(taskDispatchAllCmd)
}
