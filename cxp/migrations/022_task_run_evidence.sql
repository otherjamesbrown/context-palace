-- Task-run evidence layer for launch-pack feedback loops.
-- Stores machine-generated run observations separately from curated shard knowledge.

CREATE TABLE IF NOT EXISTS task_runs (
    id TEXT PRIMARY KEY,
    project TEXT NOT NULL,
    task_id TEXT,
    shard_id TEXT REFERENCES shards(id) ON DELETE SET NULL,
    branch TEXT,
    agent_runtime TEXT NOT NULL,
    agent_name TEXT,
    launch_artifact_type TEXT NOT NULL CHECK (launch_artifact_type IN ('none', 'launcher', 'launch_pack', 'manual')),
    launch_artifact_ref TEXT,
    status TEXT NOT NULL CHECK (status IN ('started', 'completed', 'failed', 'cancelled')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_task_runs_project_started_at
    ON task_runs(project, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_task_runs_shard_id
    ON task_runs(shard_id);

CREATE INDEX IF NOT EXISTS idx_task_runs_status
    ON task_runs(status);

CREATE TABLE IF NOT EXISTS task_run_observations (
    id TEXT PRIMARY KEY,
    task_run_id TEXT NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    observation_type TEXT NOT NULL CHECK (observation_type IN (
        'correct_subsystem_reached',
        'executable_proof_surface_found',
        'dead_end',
        'pack_missing_reference'
    )),
    subject_type TEXT NOT NULL CHECK (subject_type IN (
        'file',
        'test',
        'shard',
        'warning',
        'service',
        'command',
        'migration',
        'package',
        'module',
        'subsystem',
        'repro'
    )),
    subject_ref TEXT NOT NULL,
    role TEXT,
    confidence NUMERIC(4,3) CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1)),
    source TEXT NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_task_run_observations_run
    ON task_run_observations(task_run_id);

CREATE INDEX IF NOT EXISTS idx_task_run_observations_subject
    ON task_run_observations(subject_type, subject_ref);

CREATE INDEX IF NOT EXISTS idx_task_run_observations_type
    ON task_run_observations(observation_type);

CREATE TABLE IF NOT EXISTS metadata_patch_proposals (
    id TEXT PRIMARY KEY,
    shard_id TEXT NOT NULL REFERENCES shards(id) ON DELETE CASCADE,
    proposal_type TEXT NOT NULL CHECK (proposal_type IN (
        'add_reference',
        'remove_reference',
        'promote_reference',
        'add_test_link',
        'mark_stability',
        'suggest_related_shard'
    )),
    target_field TEXT NOT NULL,
    target_ref TEXT NOT NULL,
    evidence_count INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'proposed' CHECK (status IN ('proposed', 'accepted', 'rejected', 'applied')),
    rationale TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_metadata_patch_proposals_shard
    ON metadata_patch_proposals(shard_id);

CREATE INDEX IF NOT EXISTS idx_metadata_patch_proposals_status
    ON metadata_patch_proposals(status);

CREATE TABLE IF NOT EXISTS drift_flags (
    id TEXT PRIMARY KEY,
    shard_id TEXT NOT NULL REFERENCES shards(id) ON DELETE CASCADE,
    severity TEXT NOT NULL CHECK (severity IN ('low', 'medium', 'high')),
    flag_type TEXT NOT NULL CHECK (flag_type IN (
        'referenced_file_changed',
        'referenced_test_removed',
        'path_missing',
        'high_runtime_divergence',
        'doc_detail_drift'
    )),
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'accepted', 'dismissed', 'resolved')),
    rationale TEXT NOT NULL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_drift_flags_shard
    ON drift_flags(shard_id);

CREATE INDEX IF NOT EXISTS idx_drift_flags_status
    ON drift_flags(status);
