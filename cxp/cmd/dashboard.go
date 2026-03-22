package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "High-level pipeline status overview",
	Long: `Shows a dashboard with outcomes progress, active pipelines, blockers, and agent status.

SECTIONS:
  OUTCOMES          Business goals with design completion progress
  ACTIVE PIPELINES  Designs with pipeline metadata showing current phase
  BLOCKERS          Shards labeled "blocked" that need attention
  AGENTS            Agent activity based on in-progress tasks`,
	Example: `  cxp dashboard
  cxp dashboard --project myproject
  cxp dashboard -o json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		data, err := cpClient.GetDashboardData(ctx)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			s, _ := client.FormatJSON(data)
			fmt.Println(s)
			return nil
		}

		if outputFormat == "yaml" {
			s, _ := client.FormatYAML(data)
			fmt.Print(s)
			return nil
		}

		printDashboardText(data)
		return nil
	},
}

func printDashboardText(data *client.DashboardData) {
	// OUTCOMES
	fmt.Println("OUTCOMES")
	if len(data.Outcomes) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, o := range data.Outcomes {
			bullet := "○"
			if o.HasProgress {
				bullet = "●"
			}
			fmt.Printf("  %s %-40s %d/%d designs closed\n",
				bullet, client.Truncate(o.Title, 40), o.DesignClosed, o.DesignTotal)
		}
	}
	fmt.Println()

	// ACTIVE PIPELINES
	fmt.Println("ACTIVE PIPELINES")
	if len(data.Pipelines) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, p := range data.Pipelines {
			typeChar := strings.ToUpper(string(p.Type[0]))
			phase := p.Phase
			if phase == "" {
				phase = "(no phase)"
			}
			line := fmt.Sprintf("  %-12s %s %-28s %s", p.ID, typeChar, client.Truncate(p.Title, 28), phase)
			if p.Note != "" {
				line += "  → " + p.Note
			}
			fmt.Println(line)
		}
	}
	fmt.Println()

	// BLOCKERS
	fmt.Println("BLOCKERS")
	if len(data.Blockers) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, b := range data.Blockers {
			fmt.Printf("  %-12s %-8s %s\n", b.ID, b.Type, b.Title)
		}
	}
	fmt.Println()

	// AGENTS
	fmt.Println("AGENTS")
	if len(data.Agents) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, a := range data.Agents {
			if a.Idle {
				fmt.Printf("  %-12s idle\n", a.Agent)
			} else {
				fmt.Printf("  %-12s working on %-12s %s  (%s)\n",
					a.Agent, a.TaskID, client.Truncate(a.TaskTitle, 30), a.Duration)
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}
