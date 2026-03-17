-- Migration 012: Browse tree functions for cxpv TUI
-- Provides type listing, tree roots, and children queries.
-- Hierarchy from BOTH parent_id column AND child-of edges.

-- shard_types: returns distinct types with open/closed counts for a project
CREATE OR REPLACE FUNCTION shard_types(
    p_project TEXT,
    p_include_closed BOOLEAN DEFAULT FALSE
)
RETURNS TABLE(shard_type TEXT, shard_count BIGINT, open_count BIGINT, closed_count BIGINT)
LANGUAGE sql STABLE AS $$
    SELECT type,
        count(*),
        count(*) FILTER (WHERE status != 'closed'),
        count(*) FILTER (WHERE status = 'closed')
    FROM shards
    WHERE project = p_project
      AND (p_include_closed OR status != 'closed')
    GROUP BY type
    ORDER BY type;
$$;

-- Helper: count children from both parent_id and child-of edges (deduplicated)
CREATE OR REPLACE FUNCTION _shard_child_count(
    p_shard_id TEXT,
    p_include_closed BOOLEAN
)
RETURNS BIGINT
LANGUAGE sql STABLE AS $$
    SELECT count(*) FROM (
        -- Children via parent_id
        SELECT c.id FROM shards c
        WHERE c.parent_id = p_shard_id
          AND (p_include_closed OR c.status != 'closed')
        UNION
        -- Children via child-of edge (from_id is the child, to_id is the parent)
        SELECT e.from_id FROM edges e
        JOIN shards c ON c.id = e.from_id
        WHERE e.to_id = p_shard_id
          AND e.edge_type = 'child-of'
          AND (p_include_closed OR c.status != 'closed')
    ) sub;
$$;

-- shard_tree_roots: root shards (no parent via either mechanism) of a given type
CREATE OR REPLACE FUNCTION shard_tree_roots(
    p_project TEXT,
    p_type TEXT,
    p_include_closed BOOLEAN DEFAULT FALSE
)
RETURNS TABLE(
    id TEXT,
    title TEXT,
    shard_type TEXT,
    status TEXT,
    labels TEXT[],
    child_count BIGINT,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
)
LANGUAGE sql STABLE AS $$
    SELECT
        s.id,
        s.title,
        s.type,
        s.status,
        s.labels,
        _shard_child_count(s.id, p_include_closed),
        s.created_at,
        s.updated_at
    FROM shards s
    WHERE s.project = p_project
      AND s.type = p_type
      AND s.parent_id IS NULL
      -- No child-of edge pointing from this shard to a parent
      AND NOT EXISTS (
          SELECT 1 FROM edges e
          WHERE e.from_id = s.id AND e.edge_type = 'child-of'
      )
      AND (p_include_closed OR s.status != 'closed')
    ORDER BY s.updated_at DESC;
$$;

-- shard_tree_children: direct children of a parent (any type) via both mechanisms
CREATE OR REPLACE FUNCTION shard_tree_children(
    p_project TEXT,
    p_parent_id TEXT,
    p_include_closed BOOLEAN DEFAULT FALSE
)
RETURNS TABLE(
    id TEXT,
    title TEXT,
    shard_type TEXT,
    status TEXT,
    labels TEXT[],
    child_count BIGINT,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
)
LANGUAGE sql STABLE AS $$
    SELECT
        s.id,
        s.title,
        s.type,
        s.status,
        s.labels,
        _shard_child_count(s.id, p_include_closed),
        s.created_at,
        s.updated_at
    FROM shards s
    WHERE s.project = p_project
      AND (p_include_closed OR s.status != 'closed')
      AND s.id IN (
          -- Via parent_id
          SELECT c.id FROM shards c WHERE c.parent_id = p_parent_id
          UNION
          -- Via child-of edge
          SELECT e.from_id FROM edges e
          WHERE e.to_id = p_parent_id AND e.edge_type = 'child-of'
      )
    ORDER BY s.updated_at DESC;
$$;
