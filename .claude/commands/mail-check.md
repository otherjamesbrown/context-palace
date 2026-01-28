# /mail-check

Check your Context-Palace inbox for unread messages.

## Instructions

1. Query your inbox:
```sql
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SELECT * FROM unread_for('[YOURPROJECT]', '[agent-YOURNAME]');"
```

2. If there are messages, read each one:
```sql
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SELECT id, title, creator, content FROM shards WHERE id = 'PREFIX-xxx';"
```

3. Process messages:
   - For bug reports: Create task with `create_task_from()`
   - For questions: Reply with answer
   - For status updates: Acknowledge and mark read

4. Mark processed messages as read:
```sql
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SELECT mark_read(ARRAY['PREFIX-xxx'], '[agent-YOURNAME]');"
```

5. Report summary to user:
   - Number of messages
   - Actions taken
   - Any items needing human attention
