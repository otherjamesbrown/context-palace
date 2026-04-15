package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/otherjamesbrown/context-palace/cxp/internal/daemon"
	_ "github.com/otherjamesbrown/context-palace/cxp/internal/scheduler/workflows"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the background schedule daemon",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the schedule daemon in the foreground (runs until SIGTERM)",
	Long: `Start the Context Palace schedule daemon in the foreground.

The daemon loads all enabled schedules from the database and fires each one
on its configured cron expression. Schedule changes made via 'cxp schedule'
(create/enable/disable) are picked up automatically within a few seconds via
PostgreSQL LISTEN/NOTIFY — no restart required.

The daemon runs until it receives SIGTERM or SIGINT. On shutdown it finishes
any in-flight workflow run before exiting.

PID is written to ~/.cxp/daemon.pid. Use 'cxp daemon status' to check state.

To run as a background service, see the templates in docs/scheduler/:
  macOS (launchd):  docs/scheduler/cxp-daemon.plist
  Linux (systemd):  docs/scheduler/cxp-daemon.service`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := log.New(os.Stderr, "", log.LstdFlags)
		d, err := daemon.New(cpClient, logger)
		if err != nil {
			return err
		}
		return d.Start(context.Background())
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report the daemon running state and PID",
	RunE: func(cmd *cobra.Command, args []string) error {
		pidPath, err := daemon.DefaultPIDPath()
		if err != nil {
			return err
		}

		pid, readErr := daemon.ReadPID(pidPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				if outputFormat == "json" {
					fmt.Println(`{"running":false}`)
					return nil
				}
				fmt.Println("daemon: stopped (no PID file)")
				return nil
			}
			return fmt.Errorf("cannot read PID file: %w", readErr)
		}

		running := daemon.IsProcessRunning(pid)
		if outputFormat == "json" {
			fmt.Printf("{\"running\":%t,\"pid\":%d}\n", running, pid)
			return nil
		}
		if running {
			fmt.Printf("daemon: running (PID %d)\n", pid)
		} else {
			fmt.Printf("daemon: stopped (stale PID file, last PID %d)\n", pid)
		}
		return nil
	},
}

func init() {
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	rootCmd.AddCommand(daemonCmd)
}
