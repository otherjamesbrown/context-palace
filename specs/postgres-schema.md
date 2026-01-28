# Context-Palace PostgreSQL Schema

## Projects

Each project has its own ID prefix. Register projects before use.

```sql
CREATE TABLE projects (
  name       TEXT PRIMARY KEY,
  prefix     TEXT UNIQUE NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add a project
INSERT INTO projects (name, prefix) VALUES ('penfold', 'pf');
INSERT INTO projects (name, prefix) VALUES ('context-palace', 'cp');
```

## ID Generation

IDs are generated per-project with the project's prefix:

```sql
CREATE OR REPLACE FUNCTION gen_shard_id(p_project TEXT) RETURNS TEXT AS $$
  SELECT prefix || '-' || substr(md5(random()::text), 1, 6)
  FROM projects WHERE name = p_project;
$$ LANGUAGE sql;

-- Usage
SELECT gen_shard_id('penfold');  -- Returns: pf-a1b2c3
```

## Create Shard Helper

Simplifies creating shards with auto-generated IDs:

```sql
CREATE OR REPLACE FUNCTION create_shard(
  p_project TEXT,
  p_title TEXT,
  p_content TEXT,
  p_type TEXT,
  p_creator TEXT,
  p_owner TEXT DEFAULT NULL,
  p_priority INT DEFAULT NULL,
  p_status TEXT DEFAULT 'open'
) RETURNS TEXT AS $$
DECLARE
  new_id TEXT;
BEGIN
  new_id := gen_shard_id(p_project);
  INSERT INTO shards (id, project, title, content, type, creator, owner, priority, status)
  VALUES (new_id, p_project, p_title, p_content, p_type, p_creator, p_owner, p_priority, p_status);
  RETURN new_id;
END;
$$ LANGUAGE plpgsql;

-- Usage
SELECT create_shard('penfold', 'Fix bug', 'Details...', 'task', 'agent-cxp');
-- Returns: pf-b2c3d4
```

## Tables

### shards

The core table. Everything is a shard.

```sql
CREATE TABLE shards (
  id          TEXT PRIMARY KEY DEFAULT gen_shard_id(),
  project     TEXT NOT NULL,                 -- Project namespace: 'penfold', 'context-palace', etc.
  title       TEXT NOT NULL CHECK (char_length(title) <= 500),
  content     TEXT,
  type        TEXT,                          -- 'task', 'message', 'log', 'config', 'doc', etc.
  status      TEXT NOT NULL DEFAULT 'open',  -- 'open', 'in_progress', 'closed'
  priority    INTEGER CHECK (priority >= 0 AND priority <= 4),
  creator     TEXT NOT NULL,
  owner       TEXT,
  parent_id   TEXT REFERENCES shards(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  closed_at   TIMESTAMPTZ,
  closed_reason TEXT
);

-- Indexes
CREATE INDEX idx_shards_project ON shards(project);
CREATE INDEX idx_shards_project_status ON shards(project, status);
CREATE INDEX idx_shards_project_type ON shards(project, type);
CREATE INDEX idx_shards_status ON shards(status);
CREATE INDEX idx_shards_type ON shards(type);
CREATE INDEX idx_shards_creator ON shards(creator);
CREATE INDEX idx_shards_owner ON shards(owner);
CREATE INDEX idx_shards_parent_id ON shards(parent_id);
CREATE INDEX idx_shards_created_at ON shards(created_at);
```

### edges

Relationships between shards.

```sql
CREATE TABLE edges (
  from_id     TEXT NOT NULL REFERENCES shards(id) ON DELETE CASCADE,
  to_id       TEXT NOT NULL REFERENCES shards(id) ON DELETE CASCADE,
  edge_type   TEXT NOT NULL,  -- 'blocks', 'parent-child', 'replies-to', 'relates-to', etc.
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  metadata    JSONB,
  PRIMARY KEY (from_id, to_id, edge_type)
);

-- Indexes
CREATE INDEX idx_edges_to_id ON edges(to_id);
CREATE INDEX idx_edges_type ON edges(edge_type);
```

### labels

Tags on shards.

```sql
CREATE TABLE labels (
  shard_id    TEXT NOT NULL REFERENCES shards(id) ON DELETE CASCADE,
  label       TEXT NOT NULL,
  PRIMARY KEY (shard_id, label)
);

CREATE INDEX idx_labels_label ON labels(label);
```

### read_receipts

Per-agent read status.

```sql
CREATE TABLE read_receipts (
  shard_id    TEXT NOT NULL REFERENCES shards(id) ON DELETE CASCADE,
  agent_id    TEXT NOT NULL,
  read_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (shard_id, agent_id)
);

CREATE INDEX idx_read_receipts_agent ON read_receipts(agent_id);
```

## Auto-update updated_at

```sql
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER shards_updated_at
  BEFORE UPDATE ON shards
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at();
```

## Helper Functions

Simplify common queries - agents can use these instead of writing complex SQL.

```sql
-- Get unread messages for an agent (checks both to: and cc: labels)
CREATE OR REPLACE FUNCTION unread_for(p_project TEXT, p_agent TEXT)
RETURNS TABLE (id TEXT, title TEXT, creator TEXT, kind TEXT, created_at TIMESTAMPTZ) AS $$
  SELECT
    s.id, s.title, s.creator,
    (SELECT label FROM labels WHERE shard_id = s.id AND label LIKE 'kind:%' LIMIT 1),
    s.created_at
  FROM shards s
  JOIN labels l ON l.shard_id = s.id
  WHERE s.project = p_project
    AND s.type = 'message'
    AND s.status = 'open'
    AND l.label IN ('to:' || p_agent, 'cc:' || p_agent)
    AND s.id NOT IN (SELECT shard_id FROM read_receipts WHERE agent_id = p_agent)
  ORDER BY s.created_at;
$$ LANGUAGE sql;

-- Get tasks assigned to an agent
CREATE OR REPLACE FUNCTION tasks_for(p_project TEXT, p_agent TEXT)
RETURNS TABLE (id TEXT, title TEXT, priority INT, status TEXT, created_at TIMESTAMPTZ) AS $$
  SELECT id, title, priority, status, created_at
  FROM shards
  WHERE project = p_project AND type = 'task' AND owner = p_agent AND status != 'closed'
  ORDER BY priority, created_at;
$$ LANGUAGE sql;

-- Get ready tasks (open, not blocked)
CREATE OR REPLACE FUNCTION ready_tasks(p_project TEXT)
RETURNS TABLE (id TEXT, title TEXT, priority INT, owner TEXT, created_at TIMESTAMPTZ) AS $$
  SELECT s.id, s.title, s.priority, s.owner, s.created_at
  FROM shards s
  WHERE s.project = p_project AND s.type = 'task' AND s.status = 'open'
    AND NOT EXISTS (
      SELECT 1 FROM edges e JOIN shards blocker ON e.to_id = blocker.id
      WHERE e.from_id = s.id AND e.edge_type = 'blocks' AND blocker.status != 'closed'
    )
  ORDER BY s.priority, s.created_at;
$$ LANGUAGE sql;

-- Get conversation thread from root message
CREATE OR REPLACE FUNCTION get_thread(p_root_id TEXT)
RETURNS TABLE (id TEXT, title TEXT, creator TEXT, content TEXT, depth INT, created_at TIMESTAMPTZ) AS $$
  WITH RECURSIVE thread AS (
    SELECT s.id, s.title, s.creator, s.content, 0 AS depth, s.created_at
    FROM shards s WHERE s.id = p_root_id
    UNION ALL
    SELECT s.id, s.title, s.creator, s.content, t.depth + 1, s.created_at
    FROM shards s
    JOIN edges e ON e.from_id = s.id
    JOIN thread t ON e.to_id = t.id
    WHERE e.edge_type = 'replies-to'
  )
  SELECT * FROM thread ORDER BY depth, created_at;
$$ LANGUAGE sql;
```

### send_message()

Atomic message creation with labels and edges:

```sql
CREATE OR REPLACE FUNCTION send_message(
  p_project TEXT,
  p_sender TEXT,
  p_recipients TEXT[],
  p_subject TEXT,
  p_body TEXT,
  p_cc TEXT[] DEFAULT NULL,
  p_kind TEXT DEFAULT NULL,
  p_reply_to TEXT DEFAULT NULL
) RETURNS TEXT AS $$
DECLARE
  new_id TEXT;
  recipient TEXT;
BEGIN
  new_id := gen_shard_id(p_project);
  INSERT INTO shards (id, project, title, content, type, status, creator)
  VALUES (new_id, p_project, p_subject, p_body, 'message', 'open', p_sender);

  FOREACH recipient IN ARRAY p_recipients LOOP
    INSERT INTO labels (shard_id, label) VALUES (new_id, 'to:' || recipient);
  END LOOP;

  IF p_cc IS NOT NULL THEN
    FOREACH recipient IN ARRAY p_cc LOOP
      INSERT INTO labels (shard_id, label) VALUES (new_id, 'cc:' || recipient);
    END LOOP;
  END IF;

  IF p_kind IS NOT NULL THEN
    INSERT INTO labels (shard_id, label) VALUES (new_id, 'kind:' || p_kind);
  END IF;

  IF p_reply_to IS NOT NULL THEN
    INSERT INTO edges (from_id, to_id, edge_type) VALUES (new_id, p_reply_to, 'replies-to');
    INSERT INTO read_receipts (shard_id, agent_id) VALUES (p_reply_to, p_sender) ON CONFLICT DO NOTHING;
  END IF;

  RETURN new_id;
END;
$$ LANGUAGE plpgsql;
```

### create_task_from()

Create task from a source message with auto-linking:

```sql
CREATE OR REPLACE FUNCTION create_task_from(
  p_project TEXT,
  p_creator TEXT,
  p_source_id TEXT,
  p_title TEXT,
  p_description TEXT,
  p_priority INT DEFAULT 2,
  p_owner TEXT DEFAULT NULL,
  p_labels TEXT[] DEFAULT NULL
) RETURNS TEXT AS $$
DECLARE
  new_id TEXT;
  lbl TEXT;
  source_label TEXT;
BEGIN
  new_id := gen_shard_id(p_project);
  INSERT INTO shards (id, project, title, content, type, status, creator, owner, priority)
  VALUES (new_id, p_project, p_title, p_description, 'task', 'open', p_creator, p_owner, p_priority);

  INSERT INTO edges (from_id, to_id, edge_type) VALUES (new_id, p_source_id, 'discovered-from');

  -- Copy component labels from source
  FOR source_label IN
    SELECT label FROM labels
    WHERE shard_id = p_source_id
    AND label NOT LIKE 'to:%' AND label NOT LIKE 'cc:%' AND label NOT LIKE 'kind:%'
  LOOP
    INSERT INTO labels (shard_id, label) VALUES (new_id, source_label) ON CONFLICT DO NOTHING;
  END LOOP;

  IF p_labels IS NOT NULL THEN
    FOREACH lbl IN ARRAY p_labels LOOP
      INSERT INTO labels (shard_id, label) VALUES (new_id, lbl) ON CONFLICT DO NOTHING;
    END LOOP;
  END IF;

  -- Close source message
  UPDATE shards SET status = 'closed' WHERE id = p_source_id AND type = 'message';

  RETURN new_id;
END;
$$ LANGUAGE plpgsql;
```

### Usage

```sql
-- Send a message
SELECT send_message('penfold', 'agent-cli', ARRAY['agent-backend'], 'Bug found', 'Details...', NULL, 'bug-report');

-- Reply to a message
SELECT send_message('penfold', 'agent-backend', ARRAY['agent-cli'], 'Re: Bug found', 'Looking into it', NULL, 'ack', 'pf-original');

-- Create task from bug report
SELECT create_task_from('penfold', 'agent-backend', 'pf-bug-msg', 'fix: Bug title', 'Description', 1, 'agent-backend');

-- Other helpers
SELECT * FROM unread_for('penfold', 'agent-backend');
SELECT * FROM tasks_for('penfold', 'agent-backend');
SELECT * FROM ready_tasks('penfold');
SELECT * FROM get_thread('pf-abc123');
```

## Common Queries

### Create a task

```sql
INSERT INTO shards (title, content, type, status, creator, owner, priority)
VALUES ('Fix authentication bug', 'Users getting logged out randomly', 'task', 'open', 'human-james', 'agent-1', 1)
RETURNING id;
```

### Send a message

```sql
-- Create the message
INSERT INTO shards (title, content, type, status, creator)
VALUES ('Need help with auth', 'Can you review the OAuth flow?', 'message', 'open', 'agent-1')
RETURNING id;

-- Link it as a reply (if replying to something)
INSERT INTO edges (from_id, to_id, edge_type)
VALUES ('cp-new123', 'cp-original', 'replies-to');
```

### Get unread messages for an agent

```sql
SELECT s.*
FROM shards s
WHERE s.type = 'message'
  AND s.id NOT IN (
    SELECT shard_id FROM read_receipts WHERE agent_id = 'agent-1'
  )
ORDER BY s.created_at;
```

### Mark as read

```sql
INSERT INTO read_receipts (shard_id, agent_id)
VALUES ('cp-abc123', 'agent-1')
ON CONFLICT DO NOTHING;
```

### Get ready tasks (open, not blocked)

```sql
SELECT s.*
FROM shards s
WHERE s.type = 'task'
  AND s.status = 'open'
  AND NOT EXISTS (
    SELECT 1 FROM edges e
    JOIN shards blocker ON e.to_id = blocker.id
    WHERE e.from_id = s.id
      AND e.edge_type = 'blocks'
      AND blocker.status != 'closed'
  )
ORDER BY s.priority, s.created_at;
```

### Close a task

```sql
UPDATE shards
SET status = 'closed', closed_at = NOW(), closed_reason = 'Completed'
WHERE id = 'cp-abc123';
```

### Add a label

```sql
INSERT INTO labels (shard_id, label)
VALUES ('cp-abc123', 'backend')
ON CONFLICT DO NOTHING;
```

### Get conversation thread

```sql
-- Get all messages in a thread (following replies-to chain)
WITH RECURSIVE thread AS (
  SELECT s.*, 0 AS depth
  FROM shards s
  WHERE s.id = 'cp-root-message'

  UNION ALL

  SELECT s.*, t.depth + 1
  FROM shards s
  JOIN edges e ON e.from_id = s.id
  JOIN thread t ON e.to_id = t.id
  WHERE e.edge_type = 'replies-to'
)
SELECT * FROM thread ORDER BY depth, created_at;
```

### Create a blocking dependency

```sql
-- task-B blocks task-A (A cannot proceed until B is closed)
INSERT INTO edges (from_id, to_id, edge_type)
VALUES ('cp-taskA', 'cp-taskB', 'blocks');
```

### Full-text search (duplicate detection)

```sql
-- Find existing tasks matching keywords
SELECT id, title, status,
  ts_rank(search_vector, query) AS rank
FROM shards, to_tsquery('english', 'search & timeout') query
WHERE type = 'task'
  AND status != 'closed'
  AND search_vector @@ query
ORDER BY rank DESC
LIMIT 5;
```

## Full Setup Script

```sql
-- Run this to set up a fresh Context-Palace database

-- ID generation
CREATE OR REPLACE FUNCTION gen_shard_id() RETURNS TEXT AS $$
  SELECT 'cp-' || substr(md5(random()::text), 1, 6);
$$ LANGUAGE sql;

-- Updated_at trigger
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Tables
CREATE TABLE shards (
  id          TEXT PRIMARY KEY DEFAULT gen_shard_id(),
  project     TEXT NOT NULL,
  title       TEXT NOT NULL CHECK (char_length(title) <= 500),
  content     TEXT,
  type        TEXT,
  status      TEXT NOT NULL DEFAULT 'open',
  priority    INTEGER CHECK (priority >= 0 AND priority <= 4),
  creator     TEXT NOT NULL,
  owner       TEXT,
  parent_id   TEXT REFERENCES shards(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  closed_at   TIMESTAMPTZ,
  closed_reason TEXT
);

CREATE TABLE edges (
  from_id     TEXT NOT NULL REFERENCES shards(id) ON DELETE CASCADE,
  to_id       TEXT NOT NULL REFERENCES shards(id) ON DELETE CASCADE,
  edge_type   TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  metadata    JSONB,
  PRIMARY KEY (from_id, to_id, edge_type)
);

CREATE TABLE labels (
  shard_id    TEXT NOT NULL REFERENCES shards(id) ON DELETE CASCADE,
  label       TEXT NOT NULL,
  PRIMARY KEY (shard_id, label)
);

CREATE TABLE read_receipts (
  shard_id    TEXT NOT NULL REFERENCES shards(id) ON DELETE CASCADE,
  agent_id    TEXT NOT NULL,
  read_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (shard_id, agent_id)
);

-- Indexes
CREATE INDEX idx_shards_project ON shards(project);
CREATE INDEX idx_shards_project_status ON shards(project, status);
CREATE INDEX idx_shards_project_type ON shards(project, type);
CREATE INDEX idx_shards_status ON shards(status);
CREATE INDEX idx_shards_type ON shards(type);
CREATE INDEX idx_shards_creator ON shards(creator);
CREATE INDEX idx_shards_owner ON shards(owner);
CREATE INDEX idx_shards_parent_id ON shards(parent_id);
CREATE INDEX idx_shards_created_at ON shards(created_at);
CREATE INDEX idx_edges_to_id ON edges(to_id);
CREATE INDEX idx_edges_type ON edges(edge_type);
CREATE INDEX idx_labels_label ON labels(label);
CREATE INDEX idx_read_receipts_agent ON read_receipts(agent_id);

-- Trigger
CREATE TRIGGER shards_updated_at
  BEFORE UPDATE ON shards
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at();

-- Full-text search (optional but recommended)
ALTER TABLE shards ADD COLUMN search_vector tsvector
  GENERATED ALWAYS AS (to_tsvector('english', coalesce(title,'') || ' ' || coalesce(content,''))) STORED;
CREATE INDEX idx_shards_search ON shards USING GIN(search_vector);
```
