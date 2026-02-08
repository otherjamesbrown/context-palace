-- test_create_shard.sql
-- Tests for create_shard function and overloads
-- Runs inside a transaction that rolls back — no persistent side effects.

BEGIN;

-- Test 1: Basic create with correct argument ordering (project, creator, title, content, type)
DO $$
DECLARE
    v_id TEXT;
    v_row RECORD;
BEGIN
    SELECT create_shard('penfold', 'agent-test', 'My Title', 'My Content', 'note') INTO v_id;
    SELECT * INTO v_row FROM shards WHERE id = v_id;

    IF v_row.creator != 'agent-test' THEN
        RAISE EXCEPTION 'Test 1 FAIL: expected creator=agent-test, got %', v_row.creator;
    END IF;
    IF v_row.title != 'My Title' THEN
        RAISE EXCEPTION 'Test 1 FAIL: expected title=My Title, got %', v_row.title;
    END IF;
    IF v_row.content != 'My Content' THEN
        RAISE EXCEPTION 'Test 1 FAIL: expected content=My Content, got %', v_row.content;
    END IF;
    IF v_row.type != 'note' THEN
        RAISE EXCEPTION 'Test 1 FAIL: expected type=note, got %', v_row.type;
    END IF;

    RAISE NOTICE 'Test 1 PASS: basic create with correct argument ordering';
END $$;

-- Test 2: Labels array persisted correctly
DO $$
DECLARE
    v_id TEXT;
    v_labels TEXT[];
BEGIN
    SELECT create_shard('penfold', 'agent-test', 'Labels Test', 'Body', 'note', ARRAY['foo', 'bar', 'baz']) INTO v_id;
    SELECT labels INTO v_labels FROM shards WHERE id = v_id;

    IF v_labels != ARRAY['foo', 'bar', 'baz'] THEN
        RAISE EXCEPTION 'Test 2 FAIL: expected labels={foo,bar,baz}, got %', v_labels;
    END IF;

    RAISE NOTICE 'Test 2 PASS: labels array persisted correctly';
END $$;

-- Test 3: Single-label text overload works
DO $$
DECLARE
    v_id TEXT;
    v_labels TEXT[];
BEGIN
    SELECT create_shard('penfold', 'agent-test', 'Single Label', 'Body', 'note', 'my-label') INTO v_id;
    SELECT labels INTO v_labels FROM shards WHERE id = v_id;

    IF v_labels != ARRAY['my-label'] THEN
        RAISE EXCEPTION 'Test 3 FAIL: expected labels={my-label}, got %', v_labels;
    END IF;

    RAISE NOTICE 'Test 3 PASS: single-label text overload works';
END $$;

-- Test 4: NULL labels handled (defaults to empty array)
DO $$
DECLARE
    v_id TEXT;
    v_labels TEXT[];
BEGIN
    SELECT create_shard('penfold', 'agent-test', 'No Labels', 'Body', 'note') INTO v_id;
    SELECT labels INTO v_labels FROM shards WHERE id = v_id;

    IF v_labels != '{}' THEN
        RAISE EXCEPTION 'Test 4 FAIL: expected empty labels, got %', v_labels;
    END IF;

    RAISE NOTICE 'Test 4 PASS: NULL/default labels handled';
END $$;

-- Test 5: Empty labels array handled
DO $$
DECLARE
    v_id TEXT;
    v_labels TEXT[];
BEGIN
    SELECT create_shard('penfold', 'agent-test', 'Empty Labels', 'Body', 'note', ARRAY[]::text[]) INTO v_id;
    SELECT labels INTO v_labels FROM shards WHERE id = v_id;

    IF v_labels != '{}' THEN
        RAISE EXCEPTION 'Test 5 FAIL: expected empty labels, got %', v_labels;
    END IF;

    RAISE NOTICE 'Test 5 PASS: empty labels array handled';
END $$;

-- Test 6: create_impl_shard argument ordering (title vs creator not swapped)
DO $$
DECLARE
    v_id TEXT;
    v_row RECORD;
BEGIN
    SELECT create_impl_shard('penfold', 'agent-test', 'builder', 'Impl Title', 'Impl Content', ARRAY[]::text[], ARRAY[]::text[]) INTO v_id;
    SELECT * INTO v_row FROM shards WHERE id = v_id;

    IF v_row.creator != 'agent-test' THEN
        RAISE EXCEPTION 'Test 6 FAIL: creator wrong — expected agent-test, got % (title/creator likely swapped)', v_row.creator;
    END IF;
    IF v_row.title != 'Impl Title' THEN
        RAISE EXCEPTION 'Test 6 FAIL: title wrong — expected Impl Title, got % (content likely in title slot)', v_row.title;
    END IF;
    IF v_row.content != 'Impl Content' THEN
        RAISE EXCEPTION 'Test 6 FAIL: content wrong — expected Impl Content, got %', v_row.content;
    END IF;
    IF v_row.type != 'task' THEN
        RAISE EXCEPTION 'Test 6 FAIL: type wrong — expected task, got %', v_row.type;
    END IF;

    RAISE NOTICE 'Test 6 PASS: create_impl_shard argument ordering correct';
END $$;

-- Test 7: create_shard with all parameters (parent_id, priority, metadata)
DO $$
DECLARE
    v_parent TEXT;
    v_child TEXT;
    v_row RECORD;
BEGIN
    SELECT create_shard('penfold', 'agent-test', 'Parent', 'Parent body', 'epic') INTO v_parent;
    SELECT create_shard('penfold', 'agent-test', 'Child', 'Child body', 'task', ARRAY['urgent']::text[], v_parent, 1, '{"key": "value"}'::jsonb) INTO v_child;
    SELECT * INTO v_row FROM shards WHERE id = v_child;

    IF v_row.parent_id != v_parent THEN
        RAISE EXCEPTION 'Test 7 FAIL: expected parent_id=%, got %', v_parent, v_row.parent_id;
    END IF;
    IF v_row.priority != 1 THEN
        RAISE EXCEPTION 'Test 7 FAIL: expected priority=1, got %', v_row.priority;
    END IF;
    IF v_row.metadata->>'key' != 'value' THEN
        RAISE EXCEPTION 'Test 7 FAIL: expected metadata key=value, got %', v_row.metadata;
    END IF;
    IF v_row.labels != ARRAY['urgent'] THEN
        RAISE EXCEPTION 'Test 7 FAIL: expected labels={urgent}, got %', v_row.labels;
    END IF;

    RAISE NOTICE 'Test 7 PASS: full parameter create with parent, priority, metadata';
END $$;

ROLLBACK;
