-- SPEC-011: Custom Shard IDs
-- Allow callers to specify custom IDs instead of auto-generated pf-* IDs

-- Drop old create_shard signature to avoid ambiguity
DROP FUNCTION IF EXISTS create_shard(text, text, text, text, text, text[], text, integer, jsonb);

-- Updated create_shard with optional custom ID parameter
CREATE OR REPLACE FUNCTION create_shard(
    p_project TEXT,
    p_creator TEXT,
    p_title TEXT,
    p_content TEXT DEFAULT NULL,
    p_type TEXT DEFAULT NULL,
    p_labels TEXT[] DEFAULT '{}',
    p_parent_id TEXT DEFAULT NULL,
    p_priority INT DEFAULT NULL,
    p_metadata JSONB DEFAULT '{}',
    p_custom_id TEXT DEFAULT NULL
) RETURNS TEXT AS $$
DECLARE
    new_id TEXT;
BEGIN
    -- Use custom ID if provided, otherwise auto-generate
    IF p_custom_id IS NOT NULL AND p_custom_id != '' THEN
        new_id := p_custom_id;

        -- Check if custom ID already exists
        IF EXISTS (SELECT 1 FROM shards WHERE id = new_id) THEN
            RAISE EXCEPTION 'Shard with ID % already exists', new_id;
        END IF;
    ELSE
        new_id := gen_shard_id(p_project);
    END IF;

    INSERT INTO shards (id, project, title, content, type, creator, labels, parent_id, priority, metadata)
    VALUES (new_id, p_project, p_title, p_content, p_type, p_creator, p_labels, p_parent_id, p_priority, p_metadata);

    RETURN new_id;
END;
$$ LANGUAGE plpgsql;
