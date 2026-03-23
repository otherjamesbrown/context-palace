ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS purpose TEXT NOT NULL DEFAULT 'implementation';
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS parent_shard_id TEXT;
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS token_usage INTEGER;
ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS output_shard_id TEXT;
CREATE INDEX IF NOT EXISTS idx_task_runs_parent_purpose ON task_runs(parent_shard_id, purpose);
