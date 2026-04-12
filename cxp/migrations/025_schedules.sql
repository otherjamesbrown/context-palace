-- Schedules and schedule_runs tables for the CP scheduler.
-- Supports periodic KB maintenance workflows: drift-scan, canary, triage.

CREATE TABLE IF NOT EXISTS schedules (
    id              SERIAL PRIMARY KEY,
    project         TEXT NOT NULL,
    name            TEXT NOT NULL,
    workflow_type   TEXT NOT NULL CHECK (workflow_type IN ('drift-scan', 'canary', 'triage')),
    schedule_expr   TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    overlap_policy  TEXT NOT NULL DEFAULT 'skip' CHECK (overlap_policy IN ('skip', 'allow', 'cancel_running')),
    config          JSONB NOT NULL DEFAULT '{}',
    last_run_at     TIMESTAMPTZ,
    next_run_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(project, name)
);

CREATE TABLE IF NOT EXISTS schedule_runs (
    id              SERIAL PRIMARY KEY,
    schedule_id     INTEGER NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    project         TEXT NOT NULL,
    workflow_type   TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'completed', 'failed')),
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at     TIMESTAMPTZ,
    summary         TEXT,
    result          JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_schedule_runs_schedule ON schedule_runs(schedule_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_schedule_runs_project ON schedule_runs(project, started_at DESC);
