package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otherjamesbrown/context-palace/cxp/internal/client"
	"github.com/otherjamesbrown/context-palace/cxp/internal/scheduler"
)

// ─── PID file tests ─────────────────────────────────────────────────────────

func TestPID_WriteReadRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.pid")

	if err := WritePID(path); err != nil {
		t.Fatalf("WritePID: %v", err)
	}

	pid, err := ReadPID(path)
	if err != nil {
		t.Fatalf("ReadPID: %v", err)
	}
	if pid != os.Getpid() {
		t.Errorf("ReadPID = %d, want %d", pid, os.Getpid())
	}

	if err := RemovePID(path); err != nil {
		t.Fatalf("RemovePID: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("PID file still exists after RemovePID")
	}
}

func TestPID_RemoveNonexistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.pid")
	if err := RemovePID(path); err != nil {
		t.Errorf("RemovePID on missing file should not error, got: %v", err)
	}
}

func TestPID_AlreadyRunning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.pid")

	if err := WritePID(path); err != nil {
		t.Fatalf("first WritePID: %v", err)
	}
	defer RemovePID(path) //nolint:errcheck

	err := WritePID(path)
	if err == nil {
		t.Fatal("expected error when daemon is already running, got nil")
	}
}

func TestPID_StalePIDFileReplaced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.pid")

	// Write a PID of a dead process (PID 1 is always init/launchd, so use a high
	// number that is almost certainly free).
	if err := os.WriteFile(path, []byte("999999\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WritePID(path); err != nil {
		t.Fatalf("WritePID with stale file: %v", err)
	}
	defer RemovePID(path) //nolint:errcheck

	pid, err := ReadPID(path)
	if err != nil {
		t.Fatalf("ReadPID: %v", err)
	}
	if pid != os.Getpid() {
		t.Errorf("stale PID file was not replaced; got PID %d, want %d", pid, os.Getpid())
	}
}

// ─── Mock helpers ────────────────────────────────────────────────────────────

type mockDB struct {
	mu sync.Mutex

	schedules []client.Schedule // returned by ListAllEnabledSchedules
	hasRunning bool              // returned by HasRunningScheduleRun
	listErr   error

	cancelledIDs []int // scheduleIDs passed to CancelRunningScheduleRuns
	cancelErr    error

	nextRunID  int
	createdRuns map[int]*client.ScheduleRun
	updatedRuns map[int]client.UpdateScheduleRunInput
}

func newMockDB(schedules []client.Schedule) *mockDB {
	return &mockDB{
		nextRunID:   1,
		createdRuns: make(map[int]*client.ScheduleRun),
		updatedRuns: make(map[int]client.UpdateScheduleRunInput),
		schedules:   schedules,
	}
}

func (m *mockDB) ListAllEnabledSchedules(_ context.Context) ([]client.Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.schedules, m.listErr
}

func (m *mockDB) HasRunningScheduleRun(_ context.Context, _ int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hasRunning, nil
}

func (m *mockDB) CancelRunningScheduleRuns(_ context.Context, scheduleID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelledIDs = append(m.cancelledIDs, scheduleID)
	return m.cancelErr
}

func (m *mockDB) CreateScheduleRun(_ context.Context, in client.CreateScheduleRunInput) (*client.ScheduleRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run := &client.ScheduleRun{
		ID:           m.nextRunID,
		ScheduleID:   in.ScheduleID,
		Project:      in.Project,
		WorkflowType: in.WorkflowType,
		Status:       in.Status,
		StartedAt:    time.Now(),
		Result:       json.RawMessage(`{}`),
	}
	m.createdRuns[run.ID] = run
	m.nextRunID++
	return run, nil
}

func (m *mockDB) UpdateScheduleRun(_ context.Context, runID int, in client.UpdateScheduleRunInput) (*client.ScheduleRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updatedRuns[runID] = in
	run, ok := m.createdRuns[runID]
	if !ok {
		return nil, errors.New("run not found")
	}
	run.Status = in.Status
	return run, nil
}

func (m *mockDB) finalizedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.updatedRuns)
}

// mockRunner records calls and optionally blocks until released.
type mockRunner struct {
	name    string
	called  atomic.Int32
	summary string
	result  json.RawMessage
	runErr  error
	block   chan struct{} // if non-nil, Run blocks until this is closed
}

func (r *mockRunner) Name() string { return r.name }

func (r *mockRunner) Run(_ context.Context, _ json.RawMessage) (string, json.RawMessage, error) {
	r.called.Add(1)
	if r.block != nil {
		<-r.block
	}
	return r.summary, r.result, r.runErr
}

func testSchedule(id int, expr string, policy string) client.Schedule {
	return client.Schedule{
		ID:            id,
		Project:       "test-project",
		Name:          "test-schedule",
		WorkflowType:  "mock-workflow",
		ScheduleExpr:  expr,
		Enabled:       true,
		OverlapPolicy: policy,
		Config:        json.RawMessage(`{}`),
	}
}

func newTestDaemon(db *mockDB, reg *scheduler.Registry, dir string) *Daemon {
	return newDaemon(db, "", reg, filepath.Join(dir, "daemon.pid"))
}

// ─── LoadSchedules diff tests ────────────────────────────────────────────────

func TestLoadSchedules_AddsNewSchedule(t *testing.T) {
	s := testSchedule(1, "@every 1h", client.ScheduleOverlapSkip)
	db := newMockDB([]client.Schedule{s})
	reg := scheduler.NewRegistry()
	d := newTestDaemon(db, reg, t.TempDir())

	if err := d.LoadSchedules(context.Background()); err != nil {
		t.Fatalf("LoadSchedules: %v", err)
	}

	d.mu.Lock()
	_, registered := d.entries[1]
	d.mu.Unlock()

	if !registered {
		t.Error("schedule id=1 was not registered")
	}
}

func TestLoadSchedules_RemovesDisabledSchedule(t *testing.T) {
	s := testSchedule(1, "@every 1h", client.ScheduleOverlapSkip)
	db := newMockDB([]client.Schedule{s})
	reg := scheduler.NewRegistry()
	d := newTestDaemon(db, reg, t.TempDir())

	// Load once — registers schedule 1.
	if err := d.LoadSchedules(context.Background()); err != nil {
		t.Fatalf("first LoadSchedules: %v", err)
	}

	// Simulate schedule being disabled: return empty list.
	db.mu.Lock()
	db.schedules = nil
	db.mu.Unlock()

	if err := d.LoadSchedules(context.Background()); err != nil {
		t.Fatalf("second LoadSchedules: %v", err)
	}

	d.mu.Lock()
	_, still := d.entries[1]
	d.mu.Unlock()

	if still {
		t.Error("schedule id=1 should have been unregistered")
	}
}

func TestLoadSchedules_IdempotentOnRepeat(t *testing.T) {
	s := testSchedule(1, "@every 1h", client.ScheduleOverlapSkip)
	db := newMockDB([]client.Schedule{s})
	reg := scheduler.NewRegistry()
	d := newTestDaemon(db, reg, t.TempDir())

	for i := 0; i < 3; i++ {
		if err := d.LoadSchedules(context.Background()); err != nil {
			t.Fatalf("LoadSchedules pass %d: %v", i, err)
		}
	}

	d.mu.Lock()
	count := len(d.entries)
	d.mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 registered entry after 3 loads, got %d", count)
	}
}

// ─── Overlap policy tests ────────────────────────────────────────────────────

func TestFireSchedule_SkipPolicy_RunAlreadyActive(t *testing.T) {
	db := newMockDB(nil)
	db.hasRunning = true

	reg := scheduler.NewRegistry()
	runner := &mockRunner{name: "mock-workflow"}
	reg.Register(runner)

	s := testSchedule(1, "@every 1h", client.ScheduleOverlapSkip)
	d := newTestDaemon(db, reg, t.TempDir())
	d.fireSchedule(s)

	if runner.called.Load() != 0 {
		t.Error("runner should not have been called when skip policy is active and run is running")
	}
	if len(db.createdRuns) != 0 {
		t.Error("no run should have been created when skipped")
	}
}

func TestFireSchedule_SkipPolicy_NoActiveRun(t *testing.T) {
	db := newMockDB(nil)
	db.hasRunning = false

	reg := scheduler.NewRegistry()
	runner := &mockRunner{name: "mock-workflow", summary: "ok"}
	reg.Register(runner)

	s := testSchedule(1, "@every 1h", client.ScheduleOverlapSkip)
	d := newTestDaemon(db, reg, t.TempDir())
	d.fireSchedule(s)

	if runner.called.Load() != 1 {
		t.Errorf("runner.called = %d, want 1", runner.called.Load())
	}
}

func TestFireSchedule_AllowPolicy_FiresWhenRunning(t *testing.T) {
	db := newMockDB(nil)
	db.hasRunning = true

	reg := scheduler.NewRegistry()
	runner := &mockRunner{name: "mock-workflow", summary: "ok"}
	reg.Register(runner)

	s := testSchedule(1, "@every 1h", client.ScheduleOverlapAllow)
	d := newTestDaemon(db, reg, t.TempDir())
	d.fireSchedule(s)

	if runner.called.Load() != 1 {
		t.Errorf("allow policy: runner.called = %d, want 1", runner.called.Load())
	}
}

func TestFireSchedule_CancelRunningPolicy(t *testing.T) {
	db := newMockDB(nil)

	reg := scheduler.NewRegistry()
	runner := &mockRunner{name: "mock-workflow", summary: "ok"}
	reg.Register(runner)

	s := testSchedule(1, "@every 1h", client.ScheduleOverlapCancelRunning)
	d := newTestDaemon(db, reg, t.TempDir())
	d.fireSchedule(s)

	db.mu.Lock()
	cancelled := db.cancelledIDs
	db.mu.Unlock()

	if len(cancelled) != 1 || cancelled[0] != 1 {
		t.Errorf("expected CancelRunningScheduleRuns called with id=1, got %v", cancelled)
	}
	if runner.called.Load() != 1 {
		t.Errorf("runner should still fire after cancel; called = %d", runner.called.Load())
	}
}

func TestFireSchedule_RunnerError_MarksFailed(t *testing.T) {
	db := newMockDB(nil)

	reg := scheduler.NewRegistry()
	runner := &mockRunner{name: "mock-workflow", runErr: errors.New("boom")}
	reg.Register(runner)

	s := testSchedule(1, "@every 1h", client.ScheduleOverlapAllow)
	d := newTestDaemon(db, reg, t.TempDir())
	d.fireSchedule(s)

	db.mu.Lock()
	up, ok := db.updatedRuns[1]
	db.mu.Unlock()

	if !ok {
		t.Fatal("UpdateScheduleRun was not called")
	}
	if up.Status != client.ScheduleRunStatusFailed {
		t.Errorf("run status = %q, want %q", up.Status, client.ScheduleRunStatusFailed)
	}
}

func TestFireSchedule_NoRunner_MarksFailed(t *testing.T) {
	db := newMockDB(nil)
	reg := scheduler.NewRegistry() // empty registry — no runner registered

	s := testSchedule(1, "@every 1h", client.ScheduleOverlapAllow)
	d := newTestDaemon(db, reg, t.TempDir())
	d.fireSchedule(s)

	db.mu.Lock()
	up, ok := db.updatedRuns[1]
	db.mu.Unlock()

	if !ok {
		t.Fatal("UpdateScheduleRun was not called for missing runner")
	}
	if up.Status != client.ScheduleRunStatusFailed {
		t.Errorf("run status = %q, want %q", up.Status, client.ScheduleRunStatusFailed)
	}
}

// ─── Graceful shutdown test ──────────────────────────────────────────────────

// TestStart_GracefulShutdown starts the daemon with a fast-firing schedule,
// cancels the context, and verifies that Start returns and no runs are left
// without a terminal status update.
func TestStart_GracefulShutdown(t *testing.T) {
	// A schedule that fires every second so at least one run happens quickly.
	s := testSchedule(1, "@every 1s", client.ScheduleOverlapAllow)
	db := newMockDB([]client.Schedule{s})

	reg := scheduler.NewRegistry()
	// Blocking runner: we'll release it before cancelling so the run finishes cleanly.
	release := make(chan struct{})
	runner := &mockRunner{name: "mock-workflow", block: release}
	reg.Register(runner)

	dir := t.TempDir()
	d := newTestDaemon(db, reg, dir)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- d.Start(ctx)
	}()

	// Wait for the runner to be invoked.
	deadline := time.After(3 * time.Second)
	for {
		if runner.called.Load() > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("runner was not called within 3 seconds")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Release the in-flight run, then cancel.
	close(release)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within 5 seconds after cancel")
	}

	// Verify every created run has a terminal update.
	db.mu.Lock()
	created := len(db.createdRuns)
	updated := len(db.updatedRuns)
	db.mu.Unlock()

	if created == 0 {
		t.Error("no runs were created during the test")
	}
	if updated != created {
		t.Errorf("runs created=%d but only %d were finalized — stuck running rows", created, updated)
	}

	// PID file should be gone.
	pidPath := filepath.Join(dir, "daemon.pid")
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file was not removed after clean shutdown")
	}
}
