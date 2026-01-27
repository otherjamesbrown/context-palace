# Context-Palace PostgreSQL Schema

## ID Generation

```sql
CREATE OR REPLACE FUNCTION gen_shard_id() RETURNS TEXT AS $$
  SELECT 'cp-' || substr(md5(random()::text), 1, 6);
$$ LANGUAGE sql;
```

## Tables

### shards

The core table. Everything is a shard.

```sql
CREATE TABLE shards (
  id          TEXT PRIMARY KEY DEFAULT gen_shard_id(),
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
