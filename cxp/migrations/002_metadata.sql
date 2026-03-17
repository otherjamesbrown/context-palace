-- SPEC-2: Structured Shard Metadata
-- Add JSONB metadata column and supporting functions

-- Add metadata column with empty object default
ALTER TABLE shards ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}';

-- GIN index for JSONB containment queries (@>, ?, ?&, ?|)
CREATE INDEX IF NOT EXISTS idx_shards_metadata ON shards USING gin (metadata);

-- Functional index for common lifecycle_status queries
CREATE INDEX IF NOT EXISTS idx_shards_metadata_lifecycle ON shards ((metadata->>'lifecycle_status'))
    WHERE metadata ? 'lifecycle_status';

-- Functional index for priority queries
CREATE INDEX IF NOT EXISTS idx_shards_metadata_priority ON shards ((metadata->>'priority'))
    WHERE metadata ? 'priority';

-- Helper function: merge metadata (don't replace, merge keys)
CREATE OR REPLACE FUNCTION update_metadata(
    p_shard_id TEXT,
    p_metadata JSONB
) RETURNS JSONB AS $$
DECLARE
    result JSONB;
BEGIN
    UPDATE shards
    SET metadata = COALESCE(metadata, '{}') || p_metadata
    WHERE id = p_shard_id
    RETURNING metadata INTO result;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    RETURN result;
END;
$$ LANGUAGE plpgsql;

-- Helper function: set nested metadata value (creates intermediate objects)
CREATE OR REPLACE FUNCTION set_metadata_path(
    p_shard_id TEXT,
    p_path TEXT[],
    p_value JSONB
) RETURNS JSONB AS $$
DECLARE
    result JSONB;
    current_meta JSONB;
    i INT;
    partial_path TEXT[];
BEGIN
    SELECT COALESCE(metadata, '{}') INTO current_meta FROM shards WHERE id = p_shard_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    -- Ensure all intermediate paths exist as objects
    FOR i IN 1..array_length(p_path, 1) - 1 LOOP
        partial_path := p_path[1:i];
        IF current_meta #> partial_path IS NULL OR jsonb_typeof(current_meta #> partial_path) != 'object' THEN
            current_meta := jsonb_set(current_meta, partial_path, '{}', true);
        END IF;
    END LOOP;

    -- Set the actual value
    current_meta := jsonb_set(current_meta, p_path, p_value, true);

    UPDATE shards
    SET metadata = current_meta
    WHERE id = p_shard_id
    RETURNING metadata INTO result;

    RETURN result;
END;
$$ LANGUAGE plpgsql;

-- Helper function: delete metadata key
CREATE OR REPLACE FUNCTION delete_metadata_key(
    p_shard_id TEXT,
    p_key TEXT
) RETURNS JSONB AS $$
DECLARE
    result JSONB;
BEGIN
    UPDATE shards
    SET metadata = COALESCE(metadata, '{}') - p_key
    WHERE id = p_shard_id
    RETURNING metadata INTO result;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Shard % not found', p_shard_id;
    END IF;

    RETURN result;
END;
$$ LANGUAGE plpgsql;

-- Drop old create_shard signature to avoid ambiguity
DROP FUNCTION IF EXISTS create_shard(text, text, text, text, text, text, integer, text);

-- Updated create_shard with metadata parameter
CREATE OR REPLACE FUNCTION create_shard(
    p_project TEXT,
    p_creator TEXT,
    p_title TEXT,
    p_content TEXT DEFAULT NULL,
    p_type TEXT DEFAULT NULL,
    p_labels TEXT[] DEFAULT '{}',
    p_parent_id TEXT DEFAULT NULL,
    p_priority INT DEFAULT NULL,
    p_metadata JSONB DEFAULT '{}'
) RETURNS TEXT AS $$
DECLARE
    new_id TEXT;
BEGIN
    new_id := gen_shard_id(p_project);
    INSERT INTO shards (id, project, title, content, type, creator, labels, parent_id, priority, metadata)
    VALUES (new_id, p_project, p_title, p_content, p_type, p_creator, p_labels, p_parent_id, p_priority, p_metadata);
    RETURN new_id;
END;
$$ LANGUAGE plpgsql;
