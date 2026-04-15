You are the critic in a critic/defender/synthesis/editor review loop.

Your task is to review this proposal document:

/Users/james/github/otherjamesbrown/context-palace/docs/evidence-aware-context-proposal.md

Context:
- The target audience for this feature is AI coding agents working in real software projects.
- The proposal is trying to solve the tension between too much context, too little context, and stale context.
- It is also proposing that hot/warm/cold context, index-first lazy loading in the style of llms.txt, evidence-aware retrieval, and automated metadata collection may be the right direction.
- Your job is not to be nice. Your job is to find weaknesses, ambiguity, missing assumptions, product risks, conceptual flaws, and areas where the proposal is not yet convincing.

Please review the proposal as a product/design reviewer, not as an implementation engineer.

You must explicitly evaluate:
- whether the proposal names one primary user clearly enough
- whether it names one primary workflow clearly enough
- whether it names one primary product outcome clearly enough
- whether it distinguishes the core claim from supporting mechanisms and exploratory extensions
- whether the proposed retrieval model is actually the right one for AI coding agents
- whether hot/warm/cold context is the right base model
- whether llms.txt-style index-first lazy loading is the right base model
- whether usage-aware promotion is convincing or introduces complexity
- whether the greenfield and legacy stories are both realistic
- whether the automation story is credible
- whether the proposal is too abstract, too broad, or missing prioritization
- whether the proposal clearly explains why this is better than simpler alternatives
- whether the proposal is internally consistent

Important review rules:
- Distinguish between flaws in the core product claim and flaws in later mechanisms.
- Do not criticize an exploratory mechanism as if it invalidates the whole proposal.
- Compare the proposal against the strongest simpler baseline, not just a generic simpler alternative.
- Focus on issues that would materially weaken the proposal for AI coding agents.
- Do not propose code architecture or implementation detail unless the lack of detail is itself a product problem.
- If the proposal has clearly improved on a previously criticized point, explicitly acknowledge that before explaining what remains weak.
- If the core framing is now mostly stable, focus your pressure on differentiation, necessity, and product meaning rather than reopening already-settled framing points unless the draft has regressed.

Output requirements:
- Be direct and specific.
- Prioritize the most important problems first.
- Spend very little time summarizing.
- Include concrete suggestions for what the next revision should clarify, tighten, remove, demote, or strengthen.

Write your review to this file:
reviews/run-XX/critic.md

Before writing:
- Create the folder if it does not exist.
- Replace `run-XX` with the current run name.
- Overwrite the file if it already exists.

Your review should use this structure:

# Critic Review

## Overall Judgment

## Assessment of Primary User, Workflow, and Outcome
- ...

## Major Concerns
- ...

## Secondary Concerns
- ...

## Strongest Simpler Alternative
- ...

## Differentiation Test
- ...

## Questions the Proposal Must Answer
- ...

## Recommended Revisions
- ...

Now perform the review and write the file.
