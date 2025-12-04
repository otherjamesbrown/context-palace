# Design Rationale: V1 Retrieval Model

This document explains the intentional V1 design decision to **exclusively use a simple tag-based retrieval system (`context_id`)** and explicitly *avoid* using a vector database or database-native full-text search (FTS) capabilities.

## 1. The Core Decision

For V1, ContextPalace will **not** implement semantic (vector) search or keyword-based (FTS) search. All retrieval will be handled by a single, mandatory query parameter: `get_index(context_id="...")`.

This is a conscious trade-off, prioritizing simplicity, performance, and deterministic behavior over "smarter" but more complex search paradigms.

## 2. Rationale 1: The User is an Intelligent Agent

The primary user of this API is not a human browsing a website, but an **LLM agent**. This agent is, by definition, an intelligent entity.

* **We are a "Context Palace," not a "Search Engine":** A context palace is a "dumb" storage system. The "intelligence" lies in the person (or agent) who places and retrieves the memories.
* **Shifting Complexity:** We shift the burden of "search" to the agent itself. The agent is perfectly capable of managing its own indexes by generating and remembering its own `context_id` tags.
    * **Bad:** `search(query="logs from that failed job")` (Fuzzy, slow, complex)
    * **Good:** `commit(content="...", context_id="job-45b-logs-fail")` -> `get_index(context_id="job-45b-logs-fail")` (Deterministic, fast, simple)
* **In-Context Search:** If an agent *does* need to find a specific memory, it can retrieve a broad category (e.g., `get_index(context_id="project-foo-logs")`), get 100 summaries back, and use its *own* LLM reasoning (in-context) to find the one it needs. The service doesn't need to provide this.

## 3. Rationale 2: Architectural and Operational Simplicity

This is a primary driver given our "single Linode VM" constraint.

* **No Added Dependencies:** Adding `pgvector` (a Postgres extension) may not be available or optimized on the Linode DBaaS. Adding a separate vector database (Chroma, Weaviate) means another service to manage, secure, and scale.
* **Reduced Cost:** Running embedding models (to create the vectors) costs compute time and money. Vector databases require significant RAM. Our simple design runs on a minimal VM.
* **"Dumb" Storage is Fast:** Querying a Postgres table by `user_id` and `context_id` (on an index) is one of the fastest operations a database can perform. It's predictable, lightweight, and scales massively.

## 4. Rationale 3: A Predictable, Deterministic API

Fuzzy search is often the *wrong tool* for agent state.

When an agent saves its state, it doesn't want to "find a memory *like* its state"; it wants to **retrieve its exact state**.

A `context_id` is a primary key. It's a deterministic pointer. A semantic search query is a "best guess" that can fail, return the wrong data, and break agent workflows. By *not* offering this, we force developers into a more robust and predictable pattern of tagging their data properly.

## 5. The Future: When to Re-evaluate (V2)

We will re-evaluate this decision when we want to build features for **human-driven discovery** or **agent-driven-discovery**, rather than agent-driven-retrieval.

* **V1 (Retrieval):** "Get me the exact file I saved for `agent-run-123`." -> `get_index()`
* **V2 (Discovery):** "Find all memories *related to* the CI/CD pipeline, even if I didn't tag them." -> `search(query="...")`

When we build a full web UI where a *human* wants to browse their team's memories, or when an agent needs to "brainstorm" by finding related but untagged info, that is the time to implement a secondary vector search index.
