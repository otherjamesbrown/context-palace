-- Migration 004: pgvector extension + semantic search
-- Requires: pgvector extension installed on the PostgreSQL server

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Add embedding column (768 dimensions via gemini-embedding-001 with outputDimensionality=768)
ALTER TABLE shards ADD COLUMN IF NOT EXISTS embedding vector(768);

-- IVFFlat index for approximate nearest neighbor search
-- lists = 50 is appropriate for <10k rows
CREATE INDEX IF NOT EXISTS idx_shards_embedding ON shards
    USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 50);

-- Semantic search function
-- Note: labels are in a separate table, so we use subqueries
CREATE OR REPLACE FUNCTION semantic_search(
    p_project TEXT,
    p_query_embedding vector(768),
    p_types TEXT[] DEFAULT NULL,
    p_labels TEXT[] DEFAULT NULL,
    p_status TEXT[] DEFAULT NULL,
    p_limit INT DEFAULT 20,
    p_min_similarity FLOAT DEFAULT 0.3
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    type TEXT,
    status TEXT,
    similarity FLOAT,
    snippet TEXT,
    labels TEXT[],
    created_at TIMESTAMPTZ
) AS $$
    SELECT
        s.id, s.title, s.type, s.status,
        (1 - (s.embedding <=> p_query_embedding))::FLOAT AS similarity,
        LEFT(s.content, 200) AS snippet,
        ARRAY(SELECT l.label FROM labels l WHERE l.shard_id = s.id) AS labels,
        s.created_at
    FROM shards s
    WHERE s.project = p_project
      AND s.embedding IS NOT NULL
      AND 1 - (s.embedding <=> p_query_embedding) >= p_min_similarity
      AND (p_types IS NULL OR s.type = ANY(p_types))
      AND (p_labels IS NULL OR EXISTS (
          SELECT 1 FROM labels l WHERE l.shard_id = s.id AND l.label = ANY(p_labels)
      ))
      AND (p_status IS NULL OR s.status = ANY(p_status))
    ORDER BY s.embedding <=> p_query_embedding
    LIMIT p_limit;
$$ LANGUAGE sql STABLE;

-- Track which shards need embedding (for retry/backfill)
-- Null embedding + non-null content = needs embedding
CREATE OR REPLACE FUNCTION shards_needing_embedding(p_project TEXT, p_limit INT DEFAULT 100)
RETURNS TABLE (id TEXT, title TEXT, type TEXT) AS $$
    SELECT id, title, type
    FROM shards
    WHERE project = p_project
      AND embedding IS NULL
      AND (content IS NOT NULL OR title IS NOT NULL)
    ORDER BY created_at DESC
    LIMIT p_limit;
$$ LANGUAGE sql STABLE;
