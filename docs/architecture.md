# Architecture Overview: ContextPalace

This document outlines the technology stack, data flow, and database schema for the ContextPalace service, designed for a simple, single-VM deployment on Akamai Linode.

## 1. Guiding Principles

* **Simplicity:** Avoid all unnecessary complexity (No Kubernetes, no microservices).
* **Single-Server:** The core application logic (API + Worker) is designed to run on a single Linode VM.
* **Managed Services:** Use managed services for data persistence (Postgres, Object Storage) to reduce operational overhead.
* **Open Source:** Use Go and other open-source tools to avoid vendor lock-in.

## 2. Core Components & Tech Stack

| Component | Technology | Host | Description |
| :--- | :--- | :--- | :--- |
| **Compute** | 1 x **Linode VM** | Linode | A single Linux VM (e.g., Ubuntu 22.04) runs the core app. |
| **Reverse Proxy** | **Nginx** | Linode VM | Handles all incoming HTTP traffic, provides SSL/HTTPS, and routes requests to the Go API or the static UI. |
| **API Server** | **Go (Golang)** | Linode VM | The MCP server. A single binary run as a `systemd` service. Handles all API logic, auth, and job queuing. |
* **Job Worker** | **Go (Golang) + `asynq`** | Linode VM | A second Go binary run as a `systemd` service. Pulls jobs from Redis and performs summarization. |
| **Job Queue** | **Redis** | Docker (on VM) | A lightweight Redis container running on the VM, used by the `asynq` Go library for job queuing. |
| **Index Database** | **PostgreSQL (DBaaS)**| Linode | The Linode Managed Postgres service. Stores all metadata: users, keys, and memory summaries/URLs. |
| **Data Storage** | **Linode Object Storage** | Linode | S3-compatible storage for the *full* memory content (large text files, JSON blobs, etc.). |
| **Auth** | **Clerk / Auth0** | External | A managed, external service for user signup, login, and JWT management. |
| **Secrets** | **Environment File** | Linode VM | A secure `.env` file (`/etc/context-palace/config.env`) locked down with file permissions. Loaded by `systemd`. |

## 3. Data Flow

### Flow 1: `commit_memory` (Asynchronous)

1.  **User Agent** `POST /v1/commit` with API Key + `content` + `context_id`.
2.  **Nginx** receives the request, terminates SSL, and forwards it to the **Go API Server** (e.g., `localhost:8080`).
3.  **Go API (Middleware):**
    a. Authenticates the API Key (by comparing its hash in the `api_keys` table).
    b. Fetches the `user` and `tier` from the DB.
    c. Checks the `usage_logs` table to see if the user is over quota.
4.  **Go API (Logic):**
    a. Generates a unique ID (e.g., UUID) for the memory.
    b. **Saves the *full* `content`** to Linode Object Storage (e.g., `/{user_id}/{uuid}.txt`).
    c. Creates a "summarize" job and pushes it to **Redis** (e.g., `task: "summarize", content: "...", memory_id: "..."`).
    d. **Inserts a *preliminary* row** into the `memories` table: `(user_id, context_id, object_url, status: "pending")`.
    e. Returns a `202 Accepted` response to the user immediately.
5.  **Go Worker (Async):**
    a. The `asynq` worker pulls the job from Redis.
    b. It calls the configured LLM (e.g., OpenAI API) with the `content`.
    c. It receives the `summary` string back from the LLM.
    d. It updates the row in the **Postgres DB**: `UPDATE memories SET summary = "...", status = "complete" WHERE memory_id = "..."`.

### Flow 2: `get_index` (Synchronous)

1.  **User Agent** `GET /v1/index?context_id=agent-123` with API Key.
2.  **Nginx** forwards the request to the **Go API Server**.
3.  **Go API (Middleware):** Authenticates the key and checks the quota (same as above).
4.  **Go API (Logic):**
    a. Gets the `user_id` associated with the API key.
    b. Runs a **secure SQL query** against the **Postgres
    c. Returns a `200 OK` with the JSON list of memories.

