// Package daemon implements the cxp background scheduler process.
//
// We use github.com/robfig/cron/v3 as the embedded cron scheduler because:
//   - It supports the standard 5-field cron syntax plus @descriptors — the same
//     grammar as client.standardCronParser, so schedule_expr values validated at
//     creation time will always parse correctly at execution time.
//   - Dynamic entry management (Add/Remove by EntryID) lets LISTEN/NOTIFY-driven
//     reloads add and remove individual schedules without restarting the engine.
//   - Cron.Stop() returns a context that resolves only after all in-flight jobs
//     finish, giving us a clean join point for graceful shutdown.
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/robfig/cron/v3"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/otherjamesbrown/context-palace/cxp/internal/scheduler"
)

// scheduleDB is the subset of client.Client methods the Daemon requires.
// Defining it as an interface keeps the daemon unit-testable without a live DB.
type scheduleDB interface {
	ListAllEnabledSchedules(ctx context.Context) ([]client.Schedule, error)
	HasRunningScheduleRun(ctx context.Context, scheduleID int) (bool, error)
	CancelRunningScheduleRuns(ctx context.Context, scheduleID int) error
	CreateScheduleRun(ctx context.Context, input client.CreateScheduleRunInput) (*client.ScheduleRun, error)
	UpdateScheduleRun(ctx context.Context, runID int, input client.UpdateScheduleRunInput) (*client.ScheduleRun, error)
}

// Daemon runs the embedded cron scheduler for Context Palace scheduled workflows.
type Daemon struct {
	db       scheduleDB
	connStr  string // used only for the dedicated LISTEN/NOTIFY connection
	registry *scheduler.Registry
	cron     *cron.Cron
	pidPath  string

	mu      sync.Mutex
	entries map[int]cron.EntryID // scheduleID → cron EntryID
}

// New creates a Daemon from a configured client and workflow registry.
func New(c *client.Client, reg *scheduler.Registry, pidPath string) *Daemon {
	return newDaemon(c, c.ConnectionString(), reg, pidPath)
}

// newDaemon is the internal constructor; tests inject a mock db and skip the real connStr.
func newDaemon(db scheduleDB, connStr string, reg *scheduler.Registry, pidPath string) *Daemon {
	return &Daemon{
		db:      db,
		connStr: connStr,
		registry: reg,
		cron: cron.New(cron.WithParser(cron.NewParser(
			cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
		))),
		pidPath: pidPath,
		entries: make(map[int]cron.EntryID),
	}
}

// Start loads all enabled schedules, starts the cron loop, opens the LISTEN connection,
// and blocks until ctx is cancelled.  On return, the cron scheduler has stopped and all
// in-flight jobs have completed — no schedule_runs rows are left in status='running'.
func (d *Daemon) Start(ctx context.Context) error {
	if err := WritePID(d.pidPath); err != nil {
		return fmt.Errorf("daemon: pid file: %w", err)
	}
	defer func() { _ = RemovePID(d.pidPath) }()

	if err := d.LoadSchedules(ctx); err != nil {
		return fmt.Errorf("daemon: initial schedule load: %w", err)
	}
	d.cron.Start()

	listenDone := make(chan error, 1)
	go func() {
		listenDone <- d.listenForChanges(ctx)
	}()

	<-ctx.Done()

	// Stop cron; blocks until every in-flight job has returned.
	cronCtx := d.cron.Stop()
	<-cronCtx.Done()

	// Give the LISTEN goroutine a moment to clean up its DB connection.
	select {
	case <-listenDone:
	case <-time.After(5 * time.Second):
		log.Printf("daemon: timed out waiting for LISTEN goroutine to exit")
	}

	return nil
}

// LoadSchedules diffs enabled DB schedules against the registered cron entries.
// New schedules are added; disabled or deleted schedules are removed.
// Safe to call concurrently with cron ticks (protected by mu).
func (d *Daemon) LoadSchedules(ctx context.Context) error {
	schedules, err := d.db.ListAllEnabledSchedules(ctx)
	if err != nil {
		return fmt.Errorf("list enabled schedules: %w", err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	enabled := make(map[int]bool, len(schedules))
	for _, s := range schedules {
		enabled[s.ID] = true
	}

	// Remove entries for disabled or deleted schedules.
	for id, entryID := range d.entries {
		if !enabled[id] {
			d.cron.Remove(entryID)
			delete(d.entries, id)
			log.Printf("daemon: unregistered schedule id=%d", id)
		}
	}

	// Register newly-enabled schedules.
	for _, s := range schedules {
		if _, exists := d.entries[s.ID]; exists {
			continue
		}
		s := s // capture loop variable for the closure
		entryID, err := d.cron.AddFunc(s.ScheduleExpr, func() { d.fireSchedule(s) })
		if err != nil {
			log.Printf("daemon: register %s (id=%d): %v", s.Name, s.ID, err)
			continue
		}
		d.entries[s.ID] = entryID
		log.Printf("daemon: registered %s (id=%d expr=%q)", s.Name, s.ID, s.ScheduleExpr)
	}

	return nil
}

// fireSchedule applies the overlap policy for s, creates a run record, executes
// the workflow runner, then updates the run to its terminal status.
func (d *Daemon) fireSchedule(s client.Schedule) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	switch s.OverlapPolicy {
	case client.ScheduleOverlapSkip:
		running, err := d.db.HasRunningScheduleRun(ctx, s.ID)
		if err != nil {
			log.Printf("daemon: overlap check %s: %v", s.Name, err)
			return
		}
		if running {
			log.Printf("daemon: skip %s — previous run still active", s.Name)
			return
		}

	case client.ScheduleOverlapCancelRunning:
		if err := d.db.CancelRunningScheduleRuns(ctx, s.ID); err != nil {
			log.Printf("daemon: cancel running for %s: %v", s.Name, err)
			// Best-effort — proceed even if the cancel query failed.
		}

	case client.ScheduleOverlapAllow:
		// Fire unconditionally regardless of any in-flight runs.

	default:
		log.Printf("daemon: unknown overlap policy %q for %s — defaulting to skip", s.OverlapPolicy, s.Name)
		running, _ := d.db.HasRunningScheduleRun(ctx, s.ID)
		if running {
			return
		}
	}

	run, err := d.db.CreateScheduleRun(ctx, client.CreateScheduleRunInput{
		ScheduleID:   s.ID,
		Project:      s.Project,
		WorkflowType: s.WorkflowType,
		Status:       client.ScheduleRunStatusRunning,
	})
	if err != nil {
		log.Printf("daemon: create run for %s: %v", s.Name, err)
		return
	}

	runner, err := d.registry.Get(s.WorkflowType)
	if err != nil {
		log.Printf("daemon: no runner for %q (%s): %v", s.WorkflowType, s.Name, err)
		d.finalizeRun(ctx, run.ID, client.ScheduleRunStatusFailed,
			fmt.Sprintf("no runner registered for workflow_type %q", s.WorkflowType), nil)
		return
	}

	summary, result, runErr := runner.Run(ctx, s.Config)
	status := client.ScheduleRunStatusCompleted
	if runErr != nil {
		status = client.ScheduleRunStatusFailed
		if summary == "" {
			summary = runErr.Error()
		}
		log.Printf("daemon: %s run failed: %v", s.Name, runErr)
	}
	d.finalizeRun(ctx, run.ID, status, summary, result)
}

func (d *Daemon) finalizeRun(ctx context.Context, runID int, status, summary string, result json.RawMessage) {
	if len(result) == 0 {
		result = json.RawMessage(`{}`)
	}
	if _, err := d.db.UpdateScheduleRun(ctx, runID, client.UpdateScheduleRunInput{
		Status:  status,
		Summary: summary,
		Result:  result,
	}); err != nil {
		log.Printf("daemon: update run %d → %s: %v", runID, status, err)
	}
}

// listenForChanges opens a dedicated DB connection, issues LISTEN schedules_changed,
// and reloads schedules on each notification.  Returns nil on clean shutdown
// (ctx cancelled), or an error if the connection is lost.
func (d *Daemon) listenForChanges(ctx context.Context) error {
	if d.connStr == "" {
		// No connection string — skip LISTEN (used in unit tests).
		<-ctx.Done()
		return nil
	}

	conn, err := pgx.Connect(ctx, d.connStr)
	if err != nil {
		// Non-fatal: daemon continues without dynamic reload until restart.
		log.Printf("daemon: LISTEN connection failed: %v — schedule changes require restart", err)
		<-ctx.Done()
		return err
	}
	defer conn.Close(context.Background()) //nolint:errcheck

	if _, err := conn.Exec(ctx, "LISTEN schedules_changed"); err != nil {
		log.Printf("daemon: LISTEN schedules_changed: %v", err)
		<-ctx.Done()
		return err
	}

	for {
		if _, err := conn.WaitForNotification(ctx); err != nil {
			if ctx.Err() != nil {
				return nil // clean shutdown via context cancellation
			}
			log.Printf("daemon: LISTEN notification error: %v", err)
			return err
		}
		if err := d.LoadSchedules(ctx); err != nil {
			// Transient DB blip — log and keep listening.
			log.Printf("daemon: reload after notify: %v", err)
		}
	}
}
