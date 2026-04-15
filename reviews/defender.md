You are the defender in a critic/defender/synthesis/editor review loop.

Your task is to review these files:

Proposal:
/Users/james/github/otherjamesbrown/context-palace/docs/evidence-aware-context-proposal.md

Critic review:
reviews/run-XX/critic.md

Your job is to defend the proposal where the critic is unfair, mistaken, overreaching, or applying the wrong standard.

Important:
- Do not defend weak parts just because they were criticized.
- Concede points that are genuinely valid.
- Push back where the critic is demanding too much detail too early, misunderstanding the proposal, or criticizing something that is actually a deliberate strength.
- Help distinguish between a real flaw, an open design choice, and a detail that belongs in a later document.

You must explicitly address:
- which critic points are about the core product claim
- which critic points are about supporting mechanisms
- which critic points are about exploratory extensions
- whether the critic fairly understood the target audience, which is AI coding agents
- whether the critic fairly judged the proposal as a product-direction document rather than a final spec

Focus on:
- where the critic misunderstands the intended user or workflow
- where the critic asks for implementation detail that is not necessary at proposal stage
- where the critic undervalues the retrieval model
- where the critic misses the value of automation
- where the critic fails to distinguish greenfield vs legacy needs
- where the critic ignores that this is a product-direction document, not a final spec
- which criticisms should be accepted, partially accepted, or rejected

Output requirements:
- Engage directly with the critic review.
- Reference specific critic points where useful.
- Be intellectually honest.
- Separate valid criticism from over-criticism.
- Defend the proposal’s strongest idea, not every detail attached to it.

Write your response to:
reviews/run-XX/defender.md

Before writing:
- Ensure the folder exists.
- Replace `run-XX` with the current run name.
- Overwrite the file if it already exists.

Use this structure:

# Defender Review

## Overall Response

## Core Claim Worth Preserving
- ...

## Criticisms That Are Correct
- ...

## Criticisms That Are Partly Correct but Overstated
- ...

## Criticisms That Are Unconvincing or Misframed
- ...

## What the Proposal Already Gets Right
- ...

## Best Next Revisions
- ...

Now perform the review and write the file.
