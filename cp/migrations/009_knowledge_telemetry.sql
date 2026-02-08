-- SPEC-9: Knowledge document access telemetry
-- Clone of memory_touch() adapted for flat knowledge docs (no depth parameter).
-- Tracks: metadata.access_count, metadata.last_accessed, metadata.access_log (last 50 reads).

CREATE OR REPLACE FUNCTION knowledge_touch(
    p_doc_id TEXT,
    p_agent TEXT
) RETURNS VOID AS $$
DECLARE
    new_entry JSONB;
    old_log JSONB;
    new_log JSONB;
BEGIN
    new_entry := jsonb_build_object(
        'at', now()::text,
        'by', p_agent
    );

    SELECT COALESCE(metadata->'access_log', '[]'::jsonb)
    INTO old_log
    FROM shards WHERE id = p_doc_id;

    IF NOT FOUND THEN
        RETURN;
    END IF;

    SELECT COALESCE(to_jsonb(array_agg(elem ORDER BY ord)), '[]'::jsonb)
    INTO new_log
    FROM (
        SELECT new_entry AS elem, 0 AS ord
        UNION ALL
        SELECT value, ordinality::int
        FROM jsonb_array_elements(old_log) WITH ORDINALITY
        WHERE ordinality <= 49
    ) sub;

    UPDATE shards
    SET metadata = jsonb_set(
        jsonb_set(
            jsonb_set(
                COALESCE(metadata, '{}'::jsonb),
                '{access_count}',
                to_jsonb(COALESCE((metadata->>'access_count')::int, 0) + 1)
            ),
            '{last_accessed}',
            to_jsonb(now()::text)
        ),
        '{access_log}',
        new_log
    )
    WHERE id = p_doc_id;
END;
$$ LANGUAGE plpgsql VOLATILE;
