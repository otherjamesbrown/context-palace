# Product Requirements Document: ContextPalace

## 1. Overview

**ContextPalace** is a multi-tenant, API-first service that provides a simple and persistent long-term memory for LLM (Large Language Model) agents.

The core problem it solves is agent state and context. As AI agents perform complex, multi-step tasks, they need a "scratchpad" or "ledger" to store their thoughts, logs, and reference documents. This memory must be retrievable across sessions, days, or even months.

Unlike solutions that focus on complex semantic (vector) search, ContextPalace V1 focuses on **deterministic retrieval**. It provides a "palace" where an agent can store a piece of information and a simple "tag" (`context_id`). It can then retrieve that exact piece of information later by referencing the same tag.

## 2. Product Goals

* **Provide a Simple, Robust API:** Offer two primary endpoints: `commit_memory` (write) and `get_index` (read).
* **Ensure Multi-Tenant Security:** A user must *only* ever be able to access their own data. Tenant isolation is a non-negotiable V1 requirement.
* **Support Agent Workflows:** The API design must be optimized for an agent's "save state" / "load state" workflow using `context_id` tags.
* **Offer a "Hands-off" Experience:** The service will automatically summarize large documents on commit, abstracting this complexity from the user.
* **Provide a Generous Free Tier:** Allow developers to sign up and use the service for personal projects and prototypes without immediate cost.

## 3. Target Audience

* **AI/LLM Developers:** Developers building applications with frameworks like LangChain, LlamaIndex, or custom agentic logic.
* **Solo Developers & Hobbyists:** Individuals who need a simple "memory" for their coding assistants or personal projects without setting up their own database.
* **Small Teams:** Teams that need a shared, centralized memory for their internal AI tools and prototypes.

## 4. Key Features (Epics)

### Epic 1: User Account & API Key Management
Users need a way to sign up for the service and get secure credentials.

* **User Signup/Login:** A simple web UI (static site) that allows users to sign up and log in using an external auth provider (e.g., Clerk, Auth0).
* **API Key Generation:** Logged-in users can generate one or more API keys for their agents.
* **API Key Security:** The key is shown to the user *only once* upon creation. The system stores a secure hash of the key.
* **API Key Management:** Users can see a list of their keys (by prefix, e.g., `cp_abc...`) and revoke (delete) them.

### Epic 2: Core Memory API (MCP Server)
The core functionality of the service, exposed via a simple, secure API.

* **API Authentication:** All API endpoints must be protected. The user must provide their API key in the `Authorization` header.
* **`commit_memory` Endpoint (Async):**
    * Allows a user to `POST` a large block of text content.
    * Accepts an optional `context_id` (string) for tagging.
    * Saves the full content to an object store.
    * Queues an "summarization" job.
    * Returns an immediate "Pending" or "Success" response.
* **`get_index` Endpoint (Sync):**
    * Allows a user to `GET` a list of memories.
    * **Requires** a `context_id` as a query parameter.
    * Returns a JSON list of all memories matching that `context_id` *and* the authenticated `user_id`.
    * The response is the "llms.txt"-style report: a list of `{summary, url, created_at}`.

### Epic 3: Asynchronous Summarization Service
Abstracts the complexity of summarizing large documents from the user.

* **Summarization Worker:** A separate background process (Go Worker) that pulls jobs from a queue (Redis).
* **LLM Abstraction:** The worker can be configured to call an external LLM (OpenAI) or a self-hosted one (Ollama) to generate a summary.
* **Database Update:** Upon successful summarization, the worker updates the `summary` and `status` fields in the `memories` database table.

### Epic 4: Usage Metering & Free Tier
Provides a "freemium" model to encourage adoption.

* **Usage Logging:** Every API call (`commit_memory`, `get_index`) is logged against the `api_key_id` and `user_id`.
* **Quota Enforcement:** A middleware on the API checks the user's recent usage against their plan's quota (e.g., 1,000 commits/month for "free" tier).
* **Rate Limiting:** If the quota is exceeded, the API returns a `429 Too Many Requests` error.

## 5. User Stories

### As a New Developer
* **Signup:** "As a new developer, I want to sign up for an account with my Google/GitHub account so I can get started quickly."
* **Create Key:** "As a developer, I want to create a new API key from a dashboard so I can authenticate my agent."
* **Secure Key:** "As a developer, I want to see my new API key displayed *once* so I can copy it to my `.env` file, and I trust it will be stored securely."

### As an Agent Developer (Core Loop)
* **Save State:** "As an agent developer, I want my agent to call `commit_memory` with a large JSON blob of its current state and a `context_id` of `agent-run-123` so it can save its work."
* **Save Document:** "As an agent developer, I want my agent to `commit_memory` with the entire content of a 10,000-word document and have the service automatically generate a summary for me."
* **Fast Write:** "As an agent developer, I want the `commit_memory` call to return in < 500ms, even if the summary takes 30 seconds to generate."
* **Load State:** "As an agent developer, I want my agent to call `get_index(context_id="agent-run-123")` at the start of its next run so it can retrieve the URL to its previous state and resume its work."
* **Security:** "As an agent developer, I want to be 100% confident that when I call `get_index`, I will *only* see memories that *my user* created, even if another user uses the same `context_id`."

### As a Developer (Account Admin)
* **Leaked Key:** "As a developer, I want to revoke (delete) an API key immediately if I accidentally commit it to GitHub."
* **Usage:** "As a developer, I want to see a list of my API key prefixes (e.g., `cp_abc...`) and when they were last used so I can clean up old, unused keys."
* **Quota:** "As a developer, I want to receive a clear `429` error when my free-tier agent goes haywire and exceeds its monthly quota."
