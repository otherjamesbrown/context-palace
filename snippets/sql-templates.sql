-- Context-Palace SQL Templates
-- Replace: PROJECT, AGENT_ID, cp-xxxxx with actual values

-- ============================================================
-- CONNECTION
-- ============================================================

-- Interactive
-- psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full"

-- Single command
-- psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SQL"

-- ============================================================
-- INBOX (MESSAGES)
-- ============================================================

-- Check inbox (unread messages)
SELECT * FROM unread_for('PROJECT', 'AGENT_ID');

-- Read full message
SELECT * FROM shards WHERE id = 'cp-xxxxx';

-- Mark as read
INSERT INTO read_receipts (shard_id, agent_id)
VALUES ('cp-xxxxx', 'AGENT_ID')
ON CONFLICT DO NOTHING;

-- Send a message
INSERT INTO shards (project, title, content, type, creator)
VALUES ('PROJECT', 'Subject line', 'Message body', 'message', 'AGENT_ID')
RETURNING id;

-- Add recipient (after sending)
INSERT INTO labels (shard_id, label) VALUES ('cp-xxxxx', 'to:recipient-agent');

-- Add message kind
INSERT INTO labels (shard_id, label) VALUES ('cp-xxxxx', 'kind:status-update');
-- Kinds: bug-report, feature-request, status-update, question, completion

-- Reply to a message
INSERT INTO shards (project, title, content, type, creator)
VALUES ('PROJECT', 'Re: Original subject', 'Reply body', 'message', 'AGENT_ID')
RETURNING id;

INSERT INTO edges (from_id, to_id, edge_type)
VALUES ('cp-reply-id', 'cp-original-id', 'replies-to');

INSERT INTO labels (shard_id, label) VALUES ('cp-reply-id', 'to:original-sender');

-- Get conversation thread
SELECT * FROM get_thread('cp-root-message-id');

-- ============================================================
-- TASKS
-- ============================================================

-- Get your tasks
SELECT * FROM tasks_for('PROJECT', 'AGENT_ID');

-- Get all ready tasks (open, not blocked)
SELECT * FROM ready_tasks('PROJECT');

-- Create a task
INSERT INTO shards (project, title, content, type, status, creator, owner, priority)
VALUES (
  'PROJECT',
  'Task title',
  '## Description\nDetails here\n\n## Acceptance Criteria\n- Criterion 1',
  'task',
  'open',
  'AGENT_ID',     -- creator
  'target-agent', -- owner (or NULL if unassigned)
  2               -- priority: 0=critical, 1=high, 2=normal, 3=low
)
RETURNING id;

-- Claim a task
UPDATE shards
SET owner = 'AGENT_ID', status = 'in_progress'
WHERE id = 'cp-xxxxx';

-- Update task status
UPDATE shards SET status = 'in_progress' WHERE id = 'cp-xxxxx';

-- Complete a task
UPDATE shards
SET status = 'closed', closed_at = NOW(), closed_reason = 'Completed: summary'
WHERE id = 'cp-xxxxx';

-- ============================================================
-- LABELS
-- ============================================================

-- Add label
INSERT INTO labels (shard_id, label)
VALUES ('cp-xxxxx', 'backend')
ON CONFLICT DO NOTHING;

-- Remove label
DELETE FROM labels WHERE shard_id = 'cp-xxxxx' AND label = 'backend';

-- Find shards by label
SELECT s.* FROM shards s
JOIN labels l ON l.shard_id = s.id
WHERE s.project = 'PROJECT' AND l.label = 'backend';

-- ============================================================
-- EDGES (RELATIONSHIPS)
-- ============================================================

-- Bug report → Task (discovered-from)
INSERT INTO edges (from_id, to_id, edge_type)
VALUES ('cp-bugreport', 'cp-task', 'discovered-from');

-- Task A blocked by Task B
INSERT INTO edges (from_id, to_id, edge_type)
VALUES ('cp-taskA', 'cp-taskB', 'blocks');

-- Message relates to task
INSERT INTO edges (from_id, to_id, edge_type)
VALUES ('cp-message', 'cp-task', 'relates-to');

-- Find blockers for a task
SELECT s.id, s.title, s.status FROM shards s
JOIN edges e ON e.to_id = s.id
WHERE e.from_id = 'cp-xxxxx' AND e.edge_type = 'blocks';

-- ============================================================
-- LOGS
-- ============================================================

-- Log an action
INSERT INTO shards (project, title, content, type, status, creator)
VALUES ('PROJECT', 'Action completed', 'Details of what was done', 'log', 'closed', 'AGENT_ID')
RETURNING id;

-- Link log to task
INSERT INTO edges (from_id, to_id, edge_type)
VALUES ('cp-log-id', 'cp-task-id', 'relates-to');

-- ============================================================
-- SEARCH
-- ============================================================

-- Full-text search
SELECT id, title, status, ts_rank(search_vector, query) AS rank
FROM shards, to_tsquery('english', 'oauth & redirect') query
WHERE project = 'PROJECT' AND search_vector @@ query
ORDER BY rank DESC
LIMIT 10;

-- ============================================================
-- HELPER FUNCTIONS REFERENCE
-- ============================================================

-- unread_for(project, agent) → id, title, creator, kind, created_at
-- tasks_for(project, agent)  → id, title, priority, status, created_at
-- ready_tasks(project)       → id, title, priority, owner, created_at
-- get_thread(shard_id)       → id, title, creator, content, depth, created_at
