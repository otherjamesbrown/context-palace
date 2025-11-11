# ContextPalace

**A simple, multi-tenant memory service for LLM agents and developers.**

ContextPalace provides a persistent, scalable, and secure "long-term memory" for AI applications. Instead of relying on complex vector search, it uses a simple, tag-based index (`context_id`) that allows an agent to retrieve its own state and documents deterministically.

This repository contains the product requirements, architectural overview, and design decisions for the ContextPalace service.

## Core Documents

* **`PRD.md`**: The main Product Requirements Document, including features and user stories.
* **`ARCHITECTURE.md`**: A detailed overview of the technology stack, data flow, and database schema.
* **`DESIGN_DECISIONS.md`**: A critical document explaining the V1 rationale for *not* using a vector database or full-text search.
