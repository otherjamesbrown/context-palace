// Package daemon implements the CP schedule daemon: a long-running process that
// fires enabled workflows on their cron expressions and reloads schedule changes
// via PostgreSQL LISTEN/NOTIFY within seconds.
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/otherjamesbrown/context-palace/cxp/internal/scheduler"
	"github.com/robfig/cron/v3"
)

// Daemon is a long-running process that fires scheduled workflows on their cron
// expressions and reloads schedule changes via PostgreSQL LISTEN/NOTIFY.
type Daemon struct {
	client  *client.Client
	cron    *cron.Cron
	logger  *log.Logger
	pidPath string

	mu      sync.RWMutex
	entries map[int]*schedEntry // scheduleID → entry

	wg sync.WaitGroup // tracks in-flight runs for graceful shutdown
}

// schedEntry tracks the cron registration and in-flight run state for one schedule.
type schedEntry struct {
	cronID   cron.EntryID
	mu       sync.Mutex
	running  bool
	cancelFn context.CancelFunc // non-nil while a run is in progress
}

// DefaultPIDPath returns the canonical PID file path (~/.cxp/daemon.pid).
func DefaultPIDPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".cxp", "daemon.pid"), nil
}

// New creates a new Daemon backed by the given client.
func New(c *client.Client, logger *log.Logger) (*Daemon, error) {
	if logger == nil {
		logger = log.Default()
	}
	pidPath, err := DefaultPIDPath()
	if err != nil {
		return nil, err
	}
	return &Daemon{
		client:  c,
		cron:    cron.New(),
		logger:  logger,
		pidPath: pidPath,
		entries: make(map[int]*schedEntry),
	}, nil
}

// PIDPath returns the path to the daemon's PID file.
func (d *Daemon) PIDPath() string { return d.pidPath }

// EntryCount returns the number of registered cron entries. Used in tests.
func (d *Daemon) EntryCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.entries)
}

// HasEntry reports whether a schedule is currently registered. Used in tests.
func (d *Daemon) HasEntry(scheduleID int) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, ok := d.entries[scheduleID]
	return ok
}

// Start runs the daemon in the foreground until SIGTERM, SIGINT, or ctx is cancelled.
// It writes a PID file, loads all enabled schedules, starts the cron loop,
// and listens for schedule changes via PostgreSQL LISTEN/NOTIFY.
func (d *Daemon) Start(ctx context.Context) error {
	if err := d.writePID(); err != nil {
		return fmt.Errorf("cannot write PID file: %w", err)
	}
	defer d.removePID()

	d.logger.Printf("[daemon] loading enabled schedules")
	if err := d.loadAllSchedules(ctx); err != nil {
		return fmt.Errorf("failed to load schedules: %w", err)
	}

	d.cron.Start()

	notifyCtx, notifyCancel := context.WithCancel(ctx)
	go d.listenForChanges(notifyCtx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	d.mu.RLock()
	count := len(d.entries)
	d.mu.RUnlock()
	d.logger.Printf("[daemon] started (PID %d), %d schedule(s) registered", os.Getpid(), count)

	select {
	case sig := <-sigCh:
		d.logger.Printf("[daemon] received signal %s, shutting down", sig)
	case <-ctx.Done():
		d.logger.Printf("[daemon] context cancelled, shutting down")
	}

	// Stop the cron ticker; in-flight jobs continue to completion.
	notifyCancel()
	stopCtx := d.cron.Stop()
	select {
	case <-stopCtx.Done():
	case <-time.After(5 * time.Second):
		d.logger.Printf("[daemon] timeout waiting for cron to drain")
	}

	// Wait for all in-flight runs to finish.
	waitDone := make(chan struct{})
	go func() { d.wg.Wait(); close(waitDone) }()
	select {
	case <-waitDone:
		d.logger.Printf("[daemon] all runs finished, stopped cleanly")
	case <-time.After(10 * time.Minute):
		d.logger.Printf("[daemon] timed out waiting for in-flight runs; forcing exit")
	}

	return nil
}

// --- schedule management ---

// loadAllSchedules queries the DB for every enabled schedule and registers each one.
func (d *Daemon) loadAllSchedules(ctx context.Context) error {
	conn, err := d.client.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT id, project, name, workflow_type, schedule_expr, enabled,
		       overlap_policy, config, last_run_at, next_run_at, created_at, updated_at
		FROM schedules
		WHERE enabled = true
	`)
	if err != nil {
		return fmt.Errorf("cannot query enabled schedules: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		s, err := scanScheduleRow(rows)
		if err != nil {
			return fmt.Errorf("scan schedule: %w", err)
		}
		if err := d.registerSchedule(s); err != nil {
			d.logger.Printf("[daemon] warning: cannot register schedule %s (id=%d): %v",
				s.Name, s.ID, err)
		}
	}
	return rows.Err()
}

// registerSchedule adds or replaces a schedule in the cron.
func (d *Daemon) registerSchedule(s client.Schedule) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Remove any existing entry for this schedule.
	if e, ok := d.entries[s.ID]; ok {
		d.cron.Remove(e.cronID)
		delete(d.entries, s.ID)
	}

	if !s.Enabled {
		return nil
	}

	entry := &schedEntry{}
	schedID := s.ID
	schedCopy := s // capture for closure

	cronID, err := d.cron.AddFunc(s.ScheduleExpr, func() {
		d.fireSchedule(schedID, schedCopy)
	})
	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", s.ScheduleExpr, err)
	}

	entry.cronID = cronID
	d.entries[s.ID] = entry
	d.logger.Printf("[daemon] registered schedule %s (id=%d, cron=%q)",
		s.Name, s.ID, s.ScheduleExpr)
	return nil
}

// unregisterSchedule removes a schedule from the cron.
func (d *Daemon) unregisterSchedule(scheduleID int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if e, ok := d.entries[scheduleID]; ok {
		d.cron.Remove(e.cronID)
		delete(d.entries, scheduleID)
		d.logger.Printf("[daemon] unregistered schedule id=%d", scheduleID)
	}
}

// reloadSchedule fetches a schedule by ID and re-registers or unregisters it.
// Called from the LISTEN/NOTIFY goroutine.
func (d *Daemon) reloadSchedule(ctx context.Context, scheduleID int) {
	conn, err := d.client.Connect(ctx)
	if err != nil {
		d.logger.Printf("[daemon] reload id=%d: cannot connect: %v", scheduleID, err)
		return
	}
	defer conn.Close(ctx)

	row := conn.QueryRow(ctx, `
		SELECT id, project, name, workflow_type, schedule_expr, enabled,
		       overlap_policy, config, last_run_at, next_run_at, created_at, updated_at
		FROM schedules WHERE id = $1
	`, scheduleID)

	s, err := scanScheduleRow(row)
	if err == pgx.ErrNoRows {
		// Schedule was deleted — remove it.
		d.unregisterSchedule(scheduleID)
		return
	}
	if err != nil {
		d.logger.Printf("[daemon] reload id=%d: scan error: %v", scheduleID, err)
		return
	}

	if s.Enabled {
		if err := d.registerSchedule(s); err != nil {
			d.logger.Printf("[daemon] reload id=%d: register error: %v", scheduleID, err)
		} else {
			d.logger.Printf("[daemon] reload: registered schedule %s (id=%d)", s.Name, s.ID)
		}
	} else {
		d.unregisterSchedule(s.ID)
		d.logger.Printf("[daemon] reload: unregistered schedule %s (id=%d, disabled)", s.Name, s.ID)
	}
}

// --- run execution ---

// fireSchedule is called by the cron at each tick. It enforces the overlap policy
// and starts the workflow run in a goroutine tracked by d.wg.
func (d *Daemon) fireSchedule(scheduleID int, s client.Schedule) {
	d.mu.RLock()
	entry, ok := d.entries[scheduleID]
	d.mu.RUnlock()
	if !ok {
		return // schedule removed while we were waiting
	}

	entry.mu.Lock()

	switch s.OverlapPolicy {
	case client.ScheduleOverlapSkip:
		if entry.running {
			entry.mu.Unlock()
			d.logger.Printf("[daemon] schedule %s: skip (run already in progress)", s.Name)
			return
		}
	case client.ScheduleOverlapCancelRunning:
		if entry.running && entry.cancelFn != nil {
			entry.cancelFn()
			d.logger.Printf("[daemon] schedule %s: cancelled previous run", s.Name)
		}
		// fall through to start new run
	// ScheduleOverlapAllow: fall through unconditionally
	}

	runCtx, cancelFn := context.WithCancel(context.Background())
	entry.running = true
	entry.cancelFn = cancelFn
	entry.mu.Unlock()

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer func() {
			cancelFn() // always release the context
			entry.mu.Lock()
			entry.running = false
			entry.cancelFn = nil
			entry.mu.Unlock()
		}()
		d.executeRun(runCtx, s)
	}()
}

// executeRun creates a schedule_runs row, invokes the workflow runner, and
// finalises the row with the outcome.
func (d *Daemon) executeRun(ctx context.Context, s client.Schedule) {
	run, err := d.client.CreateScheduleRun(ctx, client.CreateScheduleRunInput{
		ScheduleID:   s.ID,
		Project:      s.Project,
		WorkflowType: s.WorkflowType,
		Status:       client.ScheduleRunStatusRunning,
		Result:       json.RawMessage(`{}`),
	})
	if err != nil {
		d.logger.Printf("[daemon] schedule %s: cannot create run row: %v", s.Name, err)
		return
	}
	d.logger.Printf("[daemon] schedule %s: starting run %d", s.Name, run.ID)

	runner, registryErr := scheduler.DefaultRegistry.Get(s.WorkflowType)
	if registryErr != nil {
		d.finalizeRun(run.ID, s.Name, "", errorResult(registryErr), registryErr)
		return
	}

	summary, result, workflowErr := runner.Run(ctx, s.Config)
	if workflowErr != nil && len(result) == 0 {
		result = errorResult(workflowErr)
	}
	d.finalizeRun(run.ID, s.Name, summary, result, workflowErr)
}

// finalizeRun updates the schedule_run row and the parent schedule's timestamps.
// Always uses a background context so SIGTERM doesn't abort the DB write.
func (d *Daemon) finalizeRun(runID int, scheduleName, summary string, result json.RawMessage, runErr error) {
	status := client.ScheduleRunStatusCompleted
	if runErr != nil {
		status = client.ScheduleRunStatusFailed
		if summary == "" {
			summary = runErr.Error()
		}
	}
	if len(result) == 0 {
		result = json.RawMessage(`{}`)
	}

	now := time.Now().UTC()
	if _, err := d.client.UpdateScheduleRun(context.Background(), runID, client.UpdateScheduleRunInput{
		Status:     status,
		FinishedAt: &now,
		Summary:    summary,
		Result:     result,
	}); err != nil {
		d.logger.Printf("[daemon] run %d: failed to update row: %v", runID, err)
		return
	}

	if runErr != nil {
		d.logger.Printf("[daemon] run %d (%s): failed: %v", runID, scheduleName, runErr)
	} else {
		d.logger.Printf("[daemon] run %d (%s): completed: %s", runID, scheduleName, summary)
	}
}

// --- LISTEN/NOTIFY ---

// listenForChanges runs the LISTEN loop, reconnecting on error until ctx is cancelled.
func (d *Daemon) listenForChanges(ctx context.Context) {
	for {
		if err := d.listenLoop(ctx); err != nil {
			if ctx.Err() != nil {
				return // clean shutdown
			}
			d.logger.Printf("[daemon] LISTEN error, reconnecting in 5s: %v", err)
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}
		} else {
			return // clean exit
		}
	}
}

// listenLoop opens a dedicated LISTEN connection and dispatches reload goroutines
// for each schedule_changed notification until ctx is cancelled or the connection drops.
func (d *Daemon) listenLoop(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, d.client.ConnectionString())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(context.Background())

	if _, err := conn.Exec(ctx, "LISTEN schedule_changed"); err != nil {
		return fmt.Errorf("LISTEN: %w", err)
	}
	d.logger.Printf("[daemon] LISTEN/NOTIFY active on 'schedule_changed'")

	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		scheduleID, parseErr := strconv.Atoi(strings.TrimSpace(n.Payload))
		if parseErr != nil {
			d.logger.Printf("[daemon] invalid notification payload %q: %v", n.Payload, parseErr)
			continue
		}
		d.logger.Printf("[daemon] schedule change notification: id=%d", scheduleID)
		// Reload in a separate goroutine so we don't block the notification loop.
		go d.reloadSchedule(ctx, scheduleID)
	}
}

// --- PID management ---

func (d *Daemon) writePID() error {
	if err := os.MkdirAll(filepath.Dir(d.pidPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(d.pidPath, []byte(strconv.Itoa(os.Getpid())), 0o600)
}

func (d *Daemon) removePID() {
	_ = os.Remove(d.pidPath)
}

// ReadPID reads the PID from a PID file and returns it.
func ReadPID(pidPath string) (int, error) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// IsProcessRunning returns true if a process with the given PID exists and is running.
func IsProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// --- helpers ---

// scanScheduleRow scans a pgx row into a client.Schedule. The column order must match
// the SELECT list used in loadAllSchedules and reloadSchedule.
func scanScheduleRow(scanner interface{ Scan(...any) error }) (client.Schedule, error) {
	var s client.Schedule
	err := scanner.Scan(
		&s.ID, &s.Project, &s.Name, &s.WorkflowType, &s.ScheduleExpr, &s.Enabled,
		&s.OverlapPolicy, &s.Config, &s.LastRunAt, &s.NextRunAt, &s.CreatedAt, &s.UpdatedAt,
	)
	return s, err
}

// errorResult encodes an error into a schedule_runs.result JSON value.
func errorResult(err error) json.RawMessage {
	payload, marshalErr := json.Marshal(map[string]any{"error": err.Error()})
	if marshalErr != nil {
		return json.RawMessage(`{"error":"workflow failed"}`)
	}
	return payload
}
