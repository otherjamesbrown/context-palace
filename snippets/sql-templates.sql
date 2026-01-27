-- Context-Palace SQL Templates
-- Replace YOUR_AGENT_ID with your agent ID (e.g., agent-backend)
-- Replace cp-xxxxx with actual shard IDs

-- ============================================================
-- INBOX & MESSAGES
-- ============================================================

-- Check inbox (unread messages to you)
SELECT id, title, creator, created_at
FROM shards
WHERE type = 'message'
  AND id IN (SELECT shard_id FROM labels WHERE label = 'to:YOUR_AGENT_ID')
  AND id NOT IN (SELECT shard_id FROM read_receipts WHERE agent_id = 'YOUR_AGENT_ID')
ORDER BY created_at;

-- Read a specific message
SELECT * FROM shards WHERE id = 'cp-xxxxx';

-- Mark message as read
INSERT INTO read_receipts (shard_id, agent_id)
VALUES ('cp-xxxxx', 'YOUR_AGENT_ID')
ON CONFLICT DO NOTHING;

-- Send a message
INSERT INTO shards (title, content, type, creator)
VALUES (
  'Subject line here',
  'Message body here. Supports markdown.',
  'message',
  'YOUR_AGENT_ID'
)
RETURNING id;

-- After sending, add recipient label (use the returned id)
INSERT INTO labels (shard_id, label) VALUES ('cp-newid', 'to:recipient-agent-id');

-- Add message kind label
INSERT INTO labels (shard_id, label) VALUES ('cp-newid', 'kind:status-update');
-- Kinds: bug-report, feature-request, status-update, question, completion

-- Reply to a message
INSERT INTO shards (title, content, type, creator)
VALUES ('Re: Original subject', 'Reply content', 'message', 'YOUR_AGENT_ID')
RETURNING id;

INSERT INTO edges (from_id, to_id, edge_type) VALUES ('cp-reply-id', 'cp-original-id', 'replies-to');
INSERT INTO labels (shard_id, label) VALUES ('cp-reply-id', 'to:original-sender');

-- ============================================================
-- TASKS
-- ============================================================

-- Get your assigned tasks
SELECT id, title, priority, status, created_at
FROM shards
WHERE type = 'task'
  AND owner = 'YOUR_AGENT_ID'
  AND status != 'closed'
ORDER BY priority, created_at;

-- Get all open tasks (ready for claiming)
SELECT id, title, priority, owner, created_at
FROM shards
WHERE type = 'task'
  AND status = 'open'
ORDER BY priority, created_at;

-- Create a task
INSERT INTO shards (title, content, type, status, creator, owner, priority)
VALUES (
  'Task title',
  '## Description\nWhat needs to be done\n\n## Acceptance Criteria\n- Criterion 1\n- Criterion 2',
  'task',
  'open',
  'YOUR_AGENT_ID',  -- creator
  'target-agent',   -- owner (who should do it), NULL if unassigned
  2                 -- priority: 0=critical, 1=high, 2=normal, 3=low, 4=backlog
)
RETURNING id;

-- Add labels to task
INSERT INTO labels (shard_id, label) VALUES ('cp-taskid', 'backend');
INSERT INTO labels (shard_id, label) VALUES ('cp-taskid', 'auth');

-- Claim a task (assign to yourself)
UPDATE shards
SET owner = 'YOUR_AGENT_ID', status = 'in_progress'
WHERE id = 'cp-xxxxx'
  AND (owner IS NULL OR owner = 'YOUR_AGENT_ID');

-- Update task status
UPDATE shards SET status = 'in_progress' WHERE id = 'cp-xxxxx';

-- Close a task
UPDATE shards
SET status = 'closed', closed_at = NOW(), closed_reason = 'Completed: brief description of what was done'
WHERE id = 'cp-xxxxx';

-- ============================================================
-- RELATIONSHIPS (EDGES)
-- ============================================================

-- Link message to task
INSERT INTO edges (from_id, to_id, edge_type)
VALUES ('cp-message', 'cp-task', 'relates-to');

-- Bug report became a task
INSERT INTO edges (from_id, to_id, edge_type)
VALUES ('cp-bugreport', 'cp-newtask', 'discovered-from');

-- Task B blocks Task A
INSERT INTO edges (from_id, to_id, edge_type)
VALUES ('cp-taskA', 'cp-taskB', 'blocks');

-- Find what blocks a task
SELECT s.id, s.title, s.status
FROM shards s
JOIN edges e ON e.to_id = s.id
WHERE e.from_id = 'cp-xxxxx' AND e.edge_type = 'blocks';

-- ============================================================
-- SEARCH & QUERIES
-- ============================================================

-- Full-text search
SELECT id, title, status, ts_rank(search_vector, query) AS rank
FROM shards, to_tsquery('english', 'oauth & redirect') query
WHERE search_vector @@ query
ORDER BY rank DESC
LIMIT 10;

-- Find shards by label
SELECT s.* FROM shards s
JOIN labels l ON l.shard_id = s.id
WHERE l.label = 'backend';

-- Find shards with multiple labels (AND)
SELECT s.* FROM shards s
WHERE EXISTS (SELECT 1 FROM labels WHERE shard_id = s.id AND label = 'backend')
  AND EXISTS (SELECT 1 FROM labels WHERE shard_id = s.id AND label = 'auth');

-- ============================================================
-- LOGGING
-- ============================================================

-- Log an action
INSERT INTO shards (title, content, type, status, creator)
VALUES (
  'Completed database migration',
  'Ran migration 003_add_users.sql\nResult: Success\nRows affected: 42',
  'log',
  'closed',  -- logs are born closed
  'YOUR_AGENT_ID'
)
RETURNING id;

-- Link log to task
INSERT INTO edges (from_id, to_id, edge_type) VALUES ('cp-logid', 'cp-taskid', 'relates-to');
