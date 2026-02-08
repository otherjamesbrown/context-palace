-- test_shard_ops.sql
-- Tests for shard_close, update_shard, add_shard_labels, remove_shard_labels, label_summary
-- Runs inside a transaction that rolls back — no persistent side effects.

BEGIN;

-- Test 1: shard_close marks shard as closed
DO $$
DECLARE
    v_id TEXT;
    v_row RECORD;
    v_result RECORD;
BEGIN
    SELECT create_shard('penfold', 'agent-test', 'Close Test', 'Body', 'task') INTO v_id;

    SELECT * INTO v_result FROM shard_close('penfold', v_id, 'agent-test', NULL);

    SELECT * INTO v_row FROM shards WHERE id = v_id;
    IF v_row.status != 'closed' THEN
        RAISE EXCEPTION 'Test 1 FAIL: expected status=closed, got %', v_row.status;
    END IF;
    IF v_row.closed_at IS NULL THEN
        RAISE EXCEPTION 'Test 1 FAIL: closed_at should not be NULL';
    END IF;
    IF v_row.closed_by != 'agent-test' THEN
        RAISE EXCEPTION 'Test 1 FAIL: expected closed_by=agent-test, got %', v_row.closed_by;
    END IF;
    IF v_result.closed_title != 'Close Test' THEN
        RAISE EXCEPTION 'Test 1 FAIL: expected closed_title=Close Test, got %', v_result.closed_title;
    END IF;

    RAISE NOTICE 'Test 1 PASS: shard_close marks shard as closed with agent and timestamp';
END $$;

-- Test 2: shard_close with reason
DO $$
DECLARE
    v_id TEXT;
    v_row RECORD;
BEGIN
    SELECT create_shard('penfold', 'agent-test', 'Close Reason', 'Body', 'task') INTO v_id;

    PERFORM shard_close('penfold', v_id, 'agent-test', 'No longer needed');

    SELECT * INTO v_row FROM shards WHERE id = v_id;
    IF v_row.closed_reason != 'No longer needed' THEN
        RAISE EXCEPTION 'Test 2 FAIL: expected closed_reason, got %', v_row.closed_reason;
    END IF;

    RAISE NOTICE 'Test 2 PASS: shard_close stores reason';
END $$;

-- Test 3: shard_close is idempotent (closing already-closed shard)
DO $$
DECLARE
    v_id TEXT;
    v_result RECORD;
BEGIN
    SELECT create_shard('penfold', 'agent-test', 'Idempotent Close', 'Body', 'task') INTO v_id;

    PERFORM shard_close('penfold', v_id, 'agent-test', NULL);
    -- Close again — should not error
    SELECT * INTO v_result FROM shard_close('penfold', v_id, 'agent-test', NULL);

    IF v_result.closed_title != 'Idempotent Close' THEN
        RAISE EXCEPTION 'Test 3 FAIL: expected title on re-close, got %', v_result.closed_title;
    END IF;

    RAISE NOTICE 'Test 3 PASS: shard_close is idempotent';
END $$;

-- Test 4: shard_close on nonexistent shard raises error
DO $$
BEGIN
    BEGIN
        PERFORM shard_close('penfold', 'pf-nonexistent', 'agent-test', NULL);
        RAISE EXCEPTION 'Test 4 FAIL: expected error for nonexistent shard';
    EXCEPTION WHEN OTHERS THEN
        IF SQLERRM NOT LIKE '%not found%' THEN
            RAISE EXCEPTION 'Test 4 FAIL: wrong error message: %', SQLERRM;
        END IF;
    END;

    RAISE NOTICE 'Test 4 PASS: shard_close raises error for nonexistent shard';
END $$;

-- Test 5: update_shard changes title
DO $$
DECLARE
    v_id TEXT;
    v_result RECORD;
    v_row RECORD;
BEGIN
    SELECT create_shard('penfold', 'agent-test', 'Original Title', 'Original Content', 'note') INTO v_id;

    SELECT * INTO v_result FROM update_shard(v_id, 'penfold', 'New Title', NULL);

    IF NOT v_result.title_changed THEN
        RAISE EXCEPTION 'Test 5 FAIL: title_changed should be true';
    END IF;
    IF v_result.content_changed THEN
        RAISE EXCEPTION 'Test 5 FAIL: content_changed should be false';
    END IF;

    SELECT * INTO v_row FROM shards WHERE id = v_id;
    IF v_row.title != 'New Title' THEN
        RAISE EXCEPTION 'Test 5 FAIL: expected title=New Title, got %', v_row.title;
    END IF;
    IF v_row.content != 'Original Content' THEN
        RAISE EXCEPTION 'Test 5 FAIL: content should be unchanged, got %', v_row.content;
    END IF;

    RAISE NOTICE 'Test 5 PASS: update_shard changes title only';
END $$;

-- Test 6: update_shard changes content
DO $$
DECLARE
    v_id TEXT;
    v_result RECORD;
    v_row RECORD;
BEGIN
    SELECT create_shard('penfold', 'agent-test', 'Title', 'Old Content', 'note') INTO v_id;

    SELECT * INTO v_result FROM update_shard(v_id, 'penfold', NULL, 'New Content');

    IF v_result.title_changed THEN
        RAISE EXCEPTION 'Test 6 FAIL: title_changed should be false';
    END IF;
    IF NOT v_result.content_changed THEN
        RAISE EXCEPTION 'Test 6 FAIL: content_changed should be true';
    END IF;

    SELECT * INTO v_row FROM shards WHERE id = v_id;
    IF v_row.content != 'New Content' THEN
        RAISE EXCEPTION 'Test 6 FAIL: expected content=New Content, got %', v_row.content;
    END IF;

    RAISE NOTICE 'Test 6 PASS: update_shard changes content only';
END $$;

-- Test 7: update_shard on nonexistent shard raises error
DO $$
BEGIN
    BEGIN
        PERFORM update_shard('pf-nonexistent', 'penfold', 'Title', NULL);
        RAISE EXCEPTION 'Test 7 FAIL: expected error for nonexistent shard';
    EXCEPTION WHEN OTHERS THEN
        IF SQLERRM NOT LIKE '%not found%' THEN
            RAISE EXCEPTION 'Test 7 FAIL: wrong error message: %', SQLERRM;
        END IF;
    END;

    RAISE NOTICE 'Test 7 PASS: update_shard raises error for nonexistent shard';
END $$;

-- Test 8: add_shard_labels adds labels and deduplicates
DO $$
DECLARE
    v_id TEXT;
    v_labels TEXT[];
BEGIN
    SELECT create_shard('penfold', 'agent-test', 'Label Test', 'Body', 'note', ARRAY['a', 'b']) INTO v_id;

    SELECT add_shard_labels(v_id, ARRAY['b', 'c', 'd']) INTO v_labels;

    -- Should have {a, b, c, d} (sorted, no duplicates)
    IF NOT (v_labels @> ARRAY['a', 'b', 'c', 'd']) THEN
        RAISE EXCEPTION 'Test 8 FAIL: expected all labels present, got %', v_labels;
    END IF;
    IF array_length(v_labels, 1) != 4 THEN
        RAISE EXCEPTION 'Test 8 FAIL: expected 4 labels, got %', array_length(v_labels, 1);
    END IF;

    RAISE NOTICE 'Test 8 PASS: add_shard_labels adds and deduplicates';
END $$;

-- Test 9: remove_shard_labels removes specified labels
DO $$
DECLARE
    v_id TEXT;
    v_labels TEXT[];
BEGIN
    SELECT create_shard('penfold', 'agent-test', 'Remove Label', 'Body', 'note', ARRAY['x', 'y', 'z']) INTO v_id;

    SELECT remove_shard_labels(v_id, ARRAY['y']) INTO v_labels;

    IF v_labels @> ARRAY['y'] THEN
        RAISE EXCEPTION 'Test 9 FAIL: label y should have been removed, got %', v_labels;
    END IF;
    IF NOT (v_labels @> ARRAY['x', 'z']) THEN
        RAISE EXCEPTION 'Test 9 FAIL: labels x and z should remain, got %', v_labels;
    END IF;

    RAISE NOTICE 'Test 9 PASS: remove_shard_labels removes specified labels';
END $$;

-- Test 10: remove_shard_labels with non-existing label is harmless
DO $$
DECLARE
    v_id TEXT;
    v_labels TEXT[];
BEGIN
    SELECT create_shard('penfold', 'agent-test', 'Safe Remove', 'Body', 'note', ARRAY['a', 'b']) INTO v_id;

    SELECT remove_shard_labels(v_id, ARRAY['z']) INTO v_labels;

    IF NOT (v_labels @> ARRAY['a', 'b']) THEN
        RAISE EXCEPTION 'Test 10 FAIL: original labels should be preserved, got %', v_labels;
    END IF;

    RAISE NOTICE 'Test 10 PASS: remove_shard_labels with non-existing label is harmless';
END $$;

-- Test 11: label_summary returns correct counts
DO $$
DECLARE
    v_count INT;
    v_found BOOLEAN := FALSE;
BEGIN
    -- Create a few shards with a unique label
    PERFORM create_shard('penfold', 'agent-test', 'LS1', 'Body', 'note', ARRAY['test-label-summary-unique']);
    PERFORM create_shard('penfold', 'agent-test', 'LS2', 'Body', 'note', ARRAY['test-label-summary-unique']);
    PERFORM create_shard('penfold', 'agent-test', 'LS3', 'Body', 'note', ARRAY['test-label-summary-unique']);

    SELECT shard_count INTO v_count
    FROM label_summary('penfold')
    WHERE label = 'test-label-summary-unique';

    IF v_count IS NULL OR v_count < 3 THEN
        RAISE EXCEPTION 'Test 11 FAIL: expected at least 3 shards with label, got %', v_count;
    END IF;

    RAISE NOTICE 'Test 11 PASS: label_summary returns correct counts (found % shards)', v_count;
END $$;

ROLLBACK;
