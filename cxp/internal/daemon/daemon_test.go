package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	_ "github.com/otherjamesbrown/context-palace/cxp/internal/scheduler/workflows"
)

// testSetup returns a real DB client and a daemon with a temp PID file,
// or skips the test if the database or required migrations are not available.
func testSetup(t *testing.T) (*client.Client, *Daemon) {
	t.Helper()

	cfg, err := client.LoadConfig("")
	if err != nil {
		t.Skipf("no DB config: %v", err)
	}
	c := client.NewClient(cfg)

	ctx := context.Background()
	conn, err := c.Connect(ctx)
	if err != nil {
		t.Skipf("DB not reachable: %v", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, "SELECT 1 FROM schedules LIMIT 0"); err != nil {
		t.Skipf("schedules table not migrated: %v", err)
	}

	// Skip tests that require LISTEN/NOTIFY only when the trigger is absent.
	// The trigger-check is deferred to TestDaemon_ListenNotifyReload.

	logger := log.New(os.Stderr, "[daemon-test] ", log.LstdFlags)
	d, err := New(c, logger)
	if err != nil {
		t.Fatalf("daemon.New: %v", err)
	}
	// Use a temp PID file so tests don't interfere with a real daemon.
	d.pidPath = t.TempDir() + "/daemon.pid"

	return c, d
}

// requireNotifyTrigger skips the test if the schedule_changed trigger is not installed.
func requireNotifyTrigger(t *testing.T, c *client.Client) {
	t.Helper()
	ctx := context.Background()
	conn, err := c.Connect(ctx)
	if err != nil {
		t.Skipf("DB not reachable: %v", err)
	}
	defer conn.Close(ctx)

	var exists bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM pg_trigger t
			JOIN pg_class cl ON cl.oid = t.tgrelid
			WHERE cl.relname = 'schedules' AND t.tgname = 'schedules_change_trigger'
		)
	`).Scan(&exists)
	if err != nil || !exists {
		t.Skipf("schedule_changed trigger not installed (run migration 026_scheduler_notify.sql)")
	}
}

// TestDaemon_CronTickFiresWorkflow verifies that the daemon registers an enabled
// schedule and creates at least one completed schedule_runs row within 3 seconds.
func TestDaemon_CronTickFiresWorkflow(t *testing.T) {
	c, d := testSetup(t)
	ctx, cancel := context.WithCancel(context.Background())

	name := fmt.Sprintf("test-daemon-cron-%d", time.Now().UnixNano())
	schedule, err := c.CreateSchedule(ctx, client.CreateScheduleInput{
		Project:       c.Config.Project,
		Name:          name,
		WorkflowType:  "noop",
		ScheduleExpr:  "@every 1s",
		OverlapPolicy: client.ScheduleOverlapSkip,
		Config:        json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- d.Start(ctx) }()

	// Wait for at least one cron tick and run to complete.
	time.Sleep(2500 * time.Millisecond)
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("daemon.Start: %v", err)
	}

	runs, err := c.ListScheduleRuns(context.Background(), schedule.ID, 10)
	if err != nil {
		t.Fatalf("ListScheduleRuns: %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("expected at least one run, got none")
	}
	for _, r := range runs {
		if r.Status == client.ScheduleRunStatusRunning {
			t.Errorf("run %d is stuck in 'running' after daemon stopped", r.ID)
		}
	}
	t.Logf("cron fired %d run(s); first status=%s summary=%q",
		len(runs), runs[0].Status, runs[0].Summary)
}

// TestDaemon_ListenNotifyReload verifies that creating a schedule after the daemon
// has started is picked up via LISTEN/NOTIFY and fired within 3 seconds.
func TestDaemon_ListenNotifyReload(t *testing.T) {
	c, d := testSetup(t)
	requireNotifyTrigger(t, c)

	ctx, cancel := context.WithCancel(context.Background())

	// Start daemon with no schedules matching our test name.
	errCh := make(chan error, 1)
	go func() { errCh <- d.Start(ctx) }()

	// Allow the daemon to start and establish the LISTEN connection.
	time.Sleep(600 * time.Millisecond)

	// Create schedule after daemon is listening — NOTIFY fires on INSERT.
	name := fmt.Sprintf("test-daemon-listen-%d", time.Now().UnixNano())
	schedule, err := c.CreateSchedule(ctx, client.CreateScheduleInput{
		Project:       c.Config.Project,
		Name:          name,
		WorkflowType:  "noop",
		ScheduleExpr:  "@every 1s",
		OverlapPolicy: client.ScheduleOverlapSkip,
		Config:        json.RawMessage(`{}`),
	})
	if err != nil {
		cancel()
		<-errCh
		t.Fatalf("CreateSchedule: %v", err)
	}

	// Give the daemon time to receive the notification, reload, and fire at least once.
	time.Sleep(3 * time.Second)
	cancel()
	<-errCh

	runs, err := c.ListScheduleRuns(context.Background(), schedule.ID, 10)
	if err != nil {
		t.Fatalf("ListScheduleRuns: %v", err)
	}
	if len(runs) == 0 {
		t.Error("expected at least one run after LISTEN/NOTIFY reload — daemon did not pick up the new schedule")
	} else {
		t.Logf("LISTEN/NOTIFY reload: daemon picked up new schedule and fired %d run(s)", len(runs))
	}
}

// TestDaemon_GracefulShutdown verifies that after the daemon context is cancelled
// no schedule_runs rows are left stuck in 'running' status.
func TestDaemon_GracefulShutdown(t *testing.T) {
	c, d := testSetup(t)
	ctx, cancel := context.WithCancel(context.Background())

	name := fmt.Sprintf("test-daemon-shutdown-%d", time.Now().UnixNano())
	schedule, err := c.CreateSchedule(ctx, client.CreateScheduleInput{
		Project:       c.Config.Project,
		Name:          name,
		WorkflowType:  "noop",
		ScheduleExpr:  "@every 1s",
		OverlapPolicy: client.ScheduleOverlapSkip,
		Config:        json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- d.Start(ctx) }()

	// Let a few runs fire.
	time.Sleep(2 * time.Second)
	cancel() // trigger graceful shutdown

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("daemon.Start: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("daemon did not stop within 15 seconds")
	}

	// After shutdown: no rows should be stuck in 'running'.
	runs, err := c.ListScheduleRuns(context.Background(), schedule.ID, 20)
	if err != nil {
		t.Fatalf("ListScheduleRuns: %v", err)
	}
	stuckCount := 0
	for _, r := range runs {
		if r.Status == client.ScheduleRunStatusRunning {
			stuckCount++
			t.Errorf("run %d stuck in 'running' after graceful shutdown", r.ID)
		}
	}
	t.Logf("graceful shutdown: %d total runs, %d stuck", len(runs), stuckCount)
}
