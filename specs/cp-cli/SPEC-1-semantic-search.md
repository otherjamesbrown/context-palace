# SPEC-1: Semantic Search

**Status:** Draft
**Depends on:** SPEC-0
**Blocks:** SPEC-5

---

## Goal

Add vector embedding and semantic search to Context Palace. Agents search by meaning,
not just keywords. "Pipeline timeout issues" finds shards about Nomad allocation
failures, heartbeat configuration, and AI request timeouts — even if they use different
words.

## What Exists

- `search_vector` tsvector column — keyword/full-text search
- `cp memory search "query"` — text search, memory type only

## What to Build

1. **pgvector extension** in Context Palace database
2. **Embedding column** on shards table
3. **Embed-on-write** — generate embedding when shard is created/updated
4. **Semantic search function** — cosine similarity with filters
5. **`cp recall` command** — semantic search CLI
6. **Backfill command** — embed existing shards

## Database Changes

### Migration: `003_pgvector.sql`

```sql
-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Add embedding column (768 dimensions for text-embedding-004)
ALTER TABLE shards ADD COLUMN embedding vector(768);

-- Similarity search index
-- ivfflat for approximate nearest neighbor search
-- lists = 50 is appropriate for <10k rows
CREATE INDEX idx_shards_embedding ON shards
    USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 50);

-- Semantic search function
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
        1 - (s.embedding <=> p_query_embedding) AS similarity,
        LEFT(s.content, 200) AS snippet,
        s.labels,
        s.created_at
    FROM shards s
    WHERE s.project = p_project
      AND s.embedding IS NOT NULL
      AND 1 - (s.embedding <=> p_query_embedding) >= p_min_similarity
      AND (p_types IS NULL OR s.type = ANY(p_types))
      AND (p_labels IS NULL OR s.labels && p_labels)
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
```

## Embedding Pipeline

### On shard create/update (in `cp` CLI)

1. Build text: `"{type}: {title}\n\n{content}"` (type prefix provides context)
2. Truncate to 8,000 tokens (approximate: 32,000 chars) if needed
3. Call embedding provider API
4. Store result in `embedding` column
5. If API unavailable: store NULL, log warning, shard creation still succeeds

### Embedding Provider Abstraction

```go
// internal/embedding/embed.go
type Provider interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    Dimensions() int
}

// internal/embedding/google.go
type GoogleProvider struct {
    APIKey string
    Model  string // "text-embedding-004"
}

// internal/embedding/openai.go
type OpenAIProvider struct {
    APIKey string
    Model  string // "text-embedding-3-small"
}
```

### Configuration

```yaml
# ~/.cp/config.yaml
embedding:
  provider: google          # google, openai, or local
  model: text-embedding-004
  api_key_env: GOOGLE_API_KEY  # env var name containing API key
  # Or for local:
  # provider: local
  # endpoint: http://localhost:8080/embed
```

### Backfill

```bash
cp admin embed-backfill
# Embedding 587 shards... [=====>    ] 312/587
# Rate: 50/min | ETA: 5m 30s
# Done: 587 embedded, 0 failed

# Options
cp admin embed-backfill --dry-run          # Show count, don't embed
cp admin embed-backfill --batch-size 10    # Shards per API call (if batching supported)
cp admin embed-backfill --type task        # Only embed task shards
```

## CLI Surface

```bash
# Semantic search across all types
cp recall "pipeline timeout issues"
# Output:
#   SIMILARITY  TYPE         STATUS  ID          TITLE
#   0.92        bug          open    pf-c74eea   Fixes STILL not working
#   0.87        requirement  draft   pf-req-04   Structured Error Codes
#   0.85        task         closed  pf-3acaf1   Wire timeout config
#   0.81        memory       open    pf-mem-12   Lesson: AI client timeout

# Filter by type
cp recall "entity resolution" --type requirement,bug

# Filter by label
cp recall "CLIC" --label architecture

# Filter by status
cp recall "timeout" --status open

# Since filter (content created after date)
cp recall "deployment" --since 7d

# Adjust similarity threshold
cp recall "vague query" --min-similarity 0.5

# JSON output
cp recall "entity" -o json

# Limit results
cp recall "deployment" --limit 5

# Include closed/expired shards
cp recall "old decisions" --include-closed
```

## Success Criteria

1. **pgvector installed:** `SELECT 1 FROM pg_extension WHERE extname = 'vector'` returns 1.
2. **Embedding on create:** `cp shard create` embeds content within 5 seconds.
3. **Embedding on update:** `cp shard update` (content change) regenerates embedding.
4. **Semantic search works:** "deployment problems" finds shards about "Nomad allocation
   failures" even without the word "deployment".
5. **Type filter:** `--type requirement` returns only requirement shards.
6. **Label filter:** `--label architecture` returns only matching shards.
7. **Status filter:** Defaults to open shards only. `--include-closed` adds closed.
8. **Backfill:** `cp admin embed-backfill` embeds all existing shards.
9. **Graceful degradation:** Embedding API down → shard created, embedding NULL, text
   search still works.
10. **Provider-agnostic:** Config switches between Google, OpenAI, local.

## Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Empty content shard | Embed title only. If title also empty, skip (NULL embedding). |
| Content > 8000 tokens | Truncate to 8000 tokens. Full content preserved in shard. |
| Embedding API rate limit | Retry with exponential backoff (1s, 2s, 4s). Max 3 retries. Shard creation not blocked. |
| No results above threshold | Return empty list, not error. |
| Closed/expired shards | Excluded by default. `--include-closed` to include. |
| No embedding config | Error: "No embedding provider configured. Add `embedding:` section to ~/.cp/config.yaml" |
| Embedding dimension mismatch | Error: "Existing embeddings are 768-dim, provider returns 1536-dim. Run `cp admin embed-reset`." |
| Concurrent embed + search | No conflict. Embedding updates are atomic (single column UPDATE). |
| Query embedding fails | Error: "Failed to embed query. Check API key and provider config." |

---

## Test Cases

### SQL Tests: pgvector Extension

```
TEST: pgvector extension is installed
  Given: Migration 003_pgvector.sql applied
  When:  SELECT 1 FROM pg_extension WHERE extname = 'vector'
  Then:  Returns 1 row

TEST: embedding column exists
  Given: Migration applied
  When:  SELECT column_name, data_type FROM information_schema.columns
         WHERE table_name = 'shards' AND column_name = 'embedding'
  Then:  Returns 1 row with data_type containing 'USER-DEFINED'

TEST: embedding column accepts vector
  Given: Shard exists
  When:  UPDATE shards SET embedding = '[0.1, 0.2, ..., 0.768]'::vector WHERE id = <id>
  Then:  Update succeeds

TEST: embedding column accepts NULL
  Given: New shard created without embedding
  When:  SELECT embedding FROM shards WHERE id = <new>
  Then:  Returns NULL
```

### SQL Tests: semantic_search Function

```
TEST: semantic_search returns similar shards
  Given: 3 shards with known embeddings:
         shard-a: embedding close to query vector (similarity ~0.9)
         shard-b: embedding moderate similarity (~0.6)
         shard-c: embedding low similarity (~0.2)
  When:  SELECT * FROM semantic_search('test', <query_vector>)
  Then:  Returns shard-a and shard-b (above 0.3 threshold)
         shard-a ranked first
         shard-c excluded (below threshold)

TEST: semantic_search type filter
  Given: 3 shards: task (sim 0.9), bug (sim 0.8), memory (sim 0.7)
  When:  SELECT * FROM semantic_search('test', <vec>, ARRAY['task','bug'])
  Then:  Returns task and bug only. Memory excluded.

TEST: semantic_search label filter
  Given: 3 shards: A with labels=['arch'], B with labels=['deploy'], C with no labels
  When:  SELECT * FROM semantic_search('test', <vec>, NULL, ARRAY['arch'])
  Then:  Returns only shard A

TEST: semantic_search status filter
  Given: 3 shards: open (sim 0.9), closed (sim 0.8), open (sim 0.7)
  When:  SELECT * FROM semantic_search('test', <vec>, NULL, NULL, ARRAY['open'])
  Then:  Returns only the 2 open shards

TEST: semantic_search with limit
  Given: 10 shards all above threshold
  When:  SELECT * FROM semantic_search('test', <vec>, NULL, NULL, NULL, 3)
  Then:  Returns exactly 3 results (top 3 by similarity)

TEST: semantic_search min_similarity filter
  Given: 3 shards with similarities 0.9, 0.5, 0.2
  When:  SELECT * FROM semantic_search('test', <vec>, NULL, NULL, NULL, 20, 0.6)
  Then:  Returns only the 0.9 shard

TEST: semantic_search with no embeddings
  Given: No shards have embeddings (all NULL)
  When:  SELECT * FROM semantic_search('test', <vec>)
  Then:  Returns empty result set (not error)

TEST: semantic_search project isolation
  Given: project-a has shard (sim 0.9), project-b has shard (sim 0.95)
  When:  SELECT * FROM semantic_search('project-a', <vec>)
  Then:  Returns only project-a's shard
```

### SQL Tests: shards_needing_embedding

```
TEST: returns shards without embedding
  Given: 3 shards: A has embedding, B and C don't
  When:  SELECT * FROM shards_needing_embedding('test')
  Then:  Returns B and C only

TEST: excludes shards with no content
  Given: Shard with NULL content and NULL title, no embedding
  When:  SELECT * FROM shards_needing_embedding('test')
  Then:  Does not include the empty shard

TEST: respects limit
  Given: 50 shards without embeddings
  When:  SELECT * FROM shards_needing_embedding('test', 10)
  Then:  Returns exactly 10
```

### Go Unit Tests: Embedding Provider

```
TEST: GoogleProvider.Embed returns correct dimensions
  Given: Mock HTTP server returning 768-dim vector
  When:  provider.Embed(ctx, "test text")
  Then:  Returns []float32 with len = 768

TEST: GoogleProvider.Embed truncates long text
  Given: Text with 50,000 characters
  When:  provider.Embed(ctx, longText)
  Then:  Request body contains truncated text (< 32,000 chars)

TEST: GoogleProvider.Embed handles rate limit
  Given: Mock HTTP server returning 429
  When:  provider.Embed(ctx, "test")
  Then:  Returns error containing "rate limit"

TEST: GoogleProvider.Embed handles timeout
  Given: Mock HTTP server that hangs
  When:  provider.Embed(ctx, "test") with 5s timeout
  Then:  Returns error containing "timeout" after ~5s

TEST: buildEmbeddingText includes type
  Given: Shard with type="requirement", title="Entity Management", content="..."
  When:  buildEmbeddingText(shard)
  Then:  Returns "requirement: Entity Management\n\n..."

TEST: buildEmbeddingText handles empty content
  Given: Shard with type="memory", title="Remember this", content=""
  When:  buildEmbeddingText(shard)
  Then:  Returns "memory: Remember this"
```

### Go Unit Tests: Recall Command

```
TEST: parseRecallFlags defaults
  Given: `cp recall "query"` (no flags)
  When:  parseRecallFlags()
  Then:  types=nil, labels=nil, status=["open"], limit=20, minSimilarity=0.3

TEST: parseRecallFlags with type filter
  Given: `cp recall "query" --type requirement,bug`
  When:  parseRecallFlags()
  Then:  types=["requirement", "bug"]

TEST: parseRecallFlags with include-closed
  Given: `cp recall "query" --include-closed`
  When:  parseRecallFlags()
  Then:  status=nil (no status filter)

TEST: formatRecallResults text output
  Given: 3 results with similarity, type, id, title
  When:  formatRecallResults(results, "text")
  Then:  Aligned table with SIMILARITY, TYPE, STATUS, ID, TITLE columns
```

### Integration Tests: Embed and Recall

```
TEST: create shard embeds content
  Given: Valid embedding config
  When:  `cp shard create --type memory --title "Timeout lesson" --body "AI client timeout is 120s"`
  Then:  Shard has non-NULL embedding column

TEST: recall finds embedded shard
  Given: Shard created with "Nomad deployment failed because allocation didn't restart"
  When:  `cp recall "deployment problems"`
  Then:  Shard appears in results with similarity > 0.5

TEST: recall type filter works
  Given: Task shard and bug shard both about "timeout"
  When:  `cp recall "timeout" --type bug`
  Then:  Only bug shard returned

TEST: recall with no matches
  Given: Shards about software development
  When:  `cp recall "medieval castle architecture"`
  Then:  Empty result set, exit code 0

TEST: embed-backfill processes all shards
  Given: 10 shards without embeddings
  When:  `cp admin embed-backfill`
  Then:  All 10 shards now have embeddings
  And:   Output shows "10 embedded, 0 failed"

TEST: embed-backfill skips already-embedded
  Given: 10 shards, 5 already have embeddings
  When:  `cp admin embed-backfill`
  Then:  Only 5 newly embedded
  And:   Output shows "5 embedded, 0 failed, 5 skipped"

TEST: embed-backfill dry-run
  Given: 10 shards without embeddings
  When:  `cp admin embed-backfill --dry-run`
  Then:  Output shows "10 shards to embed" but no actual embedding done

TEST: shard update re-embeds
  Given: Shard with existing embedding
  When:  Content is updated via `cp shard update`
  Then:  Embedding changes (different vector from before)

TEST: recall without embedding config falls back
  Given: No embedding section in config
  When:  `cp recall "query"`
  Then:  Error: "Semantic search requires embedding config."
         Suggests: "Use `cp shard list --search 'query'` for text search."
```
