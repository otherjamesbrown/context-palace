# /mail-listen

Listen for incoming synchronous messages and respond.

## Instructions

### 1. Poll for Sync Messages

Query for unread messages with `sync:true` label:

```sql
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "
  SELECT s.id, s.title, s.creator, s.content
  FROM shards s
  JOIN labels l ON l.shard_id = s.id
  JOIN labels sync ON sync.shard_id = s.id
  WHERE s.project = '[YOURPROJECT]'
    AND s.type = 'message'
    AND s.status = 'open'
    AND l.label IN ('to:[agent-YOURNAME]', 'cc:[agent-YOURNAME]')
    AND sync.label = 'sync:true'
    AND s.id NOT IN (SELECT shard_id FROM read_receipts WHERE agent_id = '[agent-YOURNAME]')
  ORDER BY s.created_at;
"
```

### 2. Process Each Message

For each message:

1. **Parse JSON frontmatter** to get `type`, `session`, `poll_hint`
2. **Mark as read** immediately
3. **Process based on type:**

| Type | Action |
|------|--------|
| `bug` | Investigate, create task if needed, respond with fix/status |
| `question` | Answer the question |
| `request` | Fulfill request or explain why not |
| `ack` | Acknowledge receipt, continue |

### 3. Compose Response

Structure your response with JSON frontmatter:

```
{
  "poll_hint": "continue|done",
  "type": "response|ack|resolution",
  "session": "SAME_SESSION_ID"
}

## Response

Your response here...

-- [agent-YOURNAME]
```

### 4. Send Response

```sql
SELECT send_message(
  '[YOURPROJECT]',
  '[agent-YOURNAME]',
  ARRAY['ORIGINAL_SENDER'],
  'Re: ORIGINAL_SUBJECT',
  $body$RESPONSE_CONTENT$body$,
  NULL,
  NULL,
  'PREFIX-ORIGINAL'  -- reply_to
);
```

Add sync labels to response:
```sql
SELECT add_labels('PREFIX-REPLY', ARRAY['sync:true', 'sync:session-SESSION_ID']);
```

### 5. Poll Hints

**When to use each:**

| poll_hint | When to use |
|-----------|-------------|
| `continue` | You need more info, or waiting for confirmation |
| `done` | Issue resolved, conversation complete |
| `pause` | Need time to investigate (include `resume_in`) |
| `typing` | Working on response, need more time |

### 6. Continue Loop

If your response has `poll_hint: continue`, keep listening for follow-ups.

If you sent `poll_hint: done`, exit the listen loop.

### 7. Timeout

- **30 minutes**: Send warning message
- **60 minutes**: Auto-send `poll_hint: done` and exit

### 8. Example Flow

```
Initiator                          Responder
    |                                   |
    | --- Bug: API timeout -----------> |
    |     poll_hint: continue           |
    |                                   |
    | <-- Investigating... ------------ |
    |     poll_hint: continue           |
    |                                   |
    | <-- Fixed, deployed ------------- |
    |     poll_hint: done               |
    |                                   |
   END                                 END
```
