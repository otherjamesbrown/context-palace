-- Scheduler tables for built-in periodic workflows.

CREATE TABLE IF NOT EXISTS schedules (
    id SERIAL PRIMARY KEY,
    project TEXT NOT NULL REFERENCES projects(name),
    name TEXT NOT NULL,
    workflow_type TEXT NOT NULL,
    schedule_expr TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    overlap_policy TEXT NOT NULL DEFAULT 'skip',
    config JSONB NOT NULL DEFAULT '{}',
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project, name)
);

CREATE TABLE IF NOT EXISTS schedule_runs (
    id SERIAL PRIMARY KEY,
    schedule_id INTEGER NOT NULL REFERENCES schedules(id),
    project TEXT NOT NULL,
    workflow_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    summary TEXT,
    result JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_schedule_runs_schedule
    ON schedule_runs(schedule_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_schedule_runs_project
    ON schedule_runs(project, started_at DESC);
