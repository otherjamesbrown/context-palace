# /mail-send-wait

Send a message and wait for a response (synchronous conversation).

## Arguments

- `$ARGUMENTS` - Format: `recipient subject` (e.g., `agent-backend Bug: API timeout`)

## Instructions

### 1. Parse Arguments

Extract recipient and subject from arguments.

### 2. Generate Session ID

```bash
SESSION_ID=$(uuidgen | tr '[:upper:]' '[:lower:]' | cut -d'-' -f1-2)
```

### 3. Compose Message

Ask the user for the message body. Structure it as:

```
{
  "poll_hint": "continue",
  "type": "bug|question|request",
  "session": "SESSION_ID"
}

## Subject

Message body here...

-- [agent-YOURNAME]
```

### 4. Send Message

```sql
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" <<EOSQL
SELECT send_message(
  '[YOURPROJECT]',
  '[agent-YOURNAME]',
  ARRAY['RECIPIENT'],
  'SUBJECT',
  \$body\$MESSAGE_CONTENT\$body\$
);
EOSQL
```

Then add sync labels:
```sql
SELECT add_labels('PREFIX-NEWID', ARRAY['sync:true', 'sync:session-SESSION_ID']);
```

### 5. Poll for Response

Poll every 5 seconds for replies:

```bash
for i in {1..360}; do  # 30 min max
  RESPONSE=$(psql ... -t -A -c "
    SELECT id, content FROM shards s
    JOIN edges e ON e.from_id = s.id
    WHERE e.to_id = 'PREFIX-ORIGINAL' AND e.edge_type = 'replies-to'
    AND s.id NOT IN (SELECT shard_id FROM read_receipts WHERE agent_id = '[agent-YOURNAME]')
    ORDER BY s.created_at DESC LIMIT 1;
  ")

  if [ -n "$RESPONSE" ]; then
    # Process response
    # Check poll_hint
    break
  fi

  sleep 5
done
```

### 6. Handle poll_hint

- `continue` - Keep polling, show response to user, ask for reply
- `done` - Conversation complete, mark all as read, exit
- `pause` - Sleep for `resume_in` seconds, then continue polling
- `typing` - Reset timeout, continue polling

### 7. Timeout Handling

- **30 minutes**: Warn user "Conversation running long"
- **60 minutes**: Auto-send `poll_hint: done` message and exit

### 8. End Conversation

When done, mark all session messages as read:
```sql
SELECT mark_all_read('[YOURPROJECT]', '[agent-YOURNAME]');
```
