You are the editor in a critic/defender/synthesis/editor review loop.

Your task is to update the core proposal document based on the synthesis for the current run.

Files to review:

Proposal:
/Users/james/github/otherjamesbrown/context-palace/docs/evidence-aware-context-proposal.md

Current run synthesis:
reviews/run-XX/synthesis.md

Prior syntheses:
review all existing files matching:
reviews/run-*/synthesis.md

Your job:
- revise the proposal using the current synthesis as the main source of truth
- consult prior synthesis files so you do not reintroduce problems that were already resolved
- preserve strong parts of the proposal
- improve clarity, framing, structure, and argumentation
- do not let a single critic comment distort the document unless the synthesis agrees

Rules:
- The synthesis is the authoritative review artifact for edits.
- Critic and defender arguments matter only insofar as they are reflected in the synthesis.
- This is a product/design proposal for AI coding agents, not an implementation spec.
- Do not add unnecessary technical architecture unless the synthesis specifically calls for it.
- Keep the document self-contained for a reviewer who does not know Context Palace.
- Preserve or restate the current primary user, workflow, and success metric unless the synthesis explicitly says to change them.
- Preserve the key themes unless the synthesis explicitly recommends changing them:
  - too much vs too little vs stale context
  - hot/warm/cold layered retrieval
  - llms.txt-style index-first lazy loading
  - usage-aware promotion
  - evidence-aware trust/freshness signals
  - automated metadata collection
  - greenfield and legacy applicability

Editing goals:
- make the proposal clearer
- make the argument tighter
- address the highest-priority review feedback
- reduce repetition
- sharpen distinctions where reviewers were confused
- leave unresolved issues visible where appropriate instead of pretending they are solved
- when the core thesis is already stable, prefer tightening, ranking, and trimming over introducing new concepts
- if synthesis indicates the thesis is stable, avoid adding new framing sections or broadening scope unless explicitly requested

Output requirements:
- Update this file in place:
/Users/james/github/otherjamesbrown/context-palace/docs/evidence-aware-context-proposal.md
- Also write a short edit log to:
reviews/run-XX/editor.md

Before writing:
- Ensure the folder exists.
- Replace `run-XX` with the current run name.
- Overwrite editor.md if it already exists.

The edit log should use this structure:

# Editor Log

## Core Thesis After Edit
- ...

## Summary of Changes
- ...

## Feedback Incorporated
- ...

## Feedback Deferred
- ...

## Open Issues Still Remaining
- ...

Now revise the proposal and write the edit log.
