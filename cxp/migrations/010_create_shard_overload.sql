-- Migration 010: Add convenience overload for create_shard
-- Fixes cp-019b72: create_shard(text,text,text,unknown,text,text) signature mismatch
-- Accepts a single text label instead of text[], wraps it in an array and delegates.

CREATE OR REPLACE FUNCTION create_shard(
    p_project text,
    p_creator text,
    p_title text,
    p_content text,
    p_type text,
    p_label text
) RETURNS text LANGUAGE sql AS $$
    SELECT create_shard(p_project, p_creator, p_title, p_content, p_type, ARRAY[p_label]);
$$;
