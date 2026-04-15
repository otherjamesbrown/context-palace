# Synthesis Review

## Current Assessment

The proposal has a real thesis, but it is still trying to win too many arguments at once. Its strongest idea is not the hot/warm/cold hierarchy by itself; it is the claim that AI coding agents need task-specific context packs that are both selectively loaded and calibrated by implementation evidence. The next draft should center that claim and treat the rest as supporting mechanisms with different confidence levels.

Right now the critic is correct about the main weakness: product framing is still too loose. The defender is correct that the proposal already contains a plausible workflow and should not be treated as empty abstraction. The synthesis is therefore simple: do not throw away the retrieval model, but stop presenting every attached mechanism as equally central.

## Repeated Feedback From Earlier Runs
- No earlier `reviews/run-*/synthesis.md` files exist yet, so there is no prior synthesis history to deduplicate against.
- Even so, the current run already shows one repeated internal theme across critic and defender: the proposal needs a sharper user, workflow, and success metric.
- Another repeated theme inside the document and reviews is that code/tests appear to be the proof surface, but the proposal still does not state that hierarchy plainly enough.

## New Feedback In This Run
- The strongest new feedback is that the proposal should be narrowed to one product claim: helping an AI coding agent reach the right code, tests, and warnings faster from a short task in an unfamiliar repo.
- The proposal currently bundles core thesis, candidate retrieval shape, adaptive promotion, automation layer, and maintenance workflows into one stack. That makes it harder to judge and should be untangled.
- Trust/freshness signals are still underspecified at the behavioral level. The draft says what the labels are, but not what the agent should do differently when it sees them.
- The comparison against simpler alternatives is missing and now clearly matters. Without that, the proposal reads more elaborate than justified.
- Usage-aware promotion is not carrying its argumentative weight. It reads as an optimization, not as part of the core case.

## What Seems Clearly True
- The core problem is real: agents in unfamiliar repos need orientation, routing, and trust calibration, not just raw search.
- The proposal gets stronger when framed around task-specific context packs rather than around documentation structure.
- Both critic and defender effectively agree that the next draft must define the primary user, primary workflow, and primary success metric.
- Both sides also agree that code and tests should be treated as the main proof surfaces, with docs/knowledge units acting as routing and summarization layers.
- Automation should stay in scope, but as a reviewable and credibility-sensitive layer, not as hand-waved magic.

## Open Disagreements
- The main disagreement is not whether the problem exists, but how much the current draft has already earned its preferred shape. The critic thinks the hierarchy and lazy-loading model are still mostly asserted; the defender thinks proposal-stage review can legitimately evaluate them as leading candidates rather than settled facts.
- There is also a real disagreement about how central documentation-style indexing should be. The critic worries the draft is still too doc-centric; the defender reasonably argues that agents need conceptual routing that raw search does not provide.
- The critic treats greenfield and legacy as possibly separate products; the defender sees a shared core with different automation emphases. This remains unresolved, but it is not the first thing to fix. The draft can keep both in scope if it names one as the wedge.

## Recommended Changes for the Next Draft
1. Reframe the proposal around one explicit product outcome: an AI coding agent receives a short task in an unfamiliar repo and gets to the right code/tests faster, with fewer stale-doc mistakes. Add a concrete success metric or two.
2. Separate the proposal into confidence tiers: core claim, likely mechanisms, and exploratory extensions. Hot/warm/cold and lazy loading can stay, but as the current leading retrieval shape under review; usage-aware promotion should be demoted.
3. State the truth hierarchy plainly: code and tests are the primary proof surfaces; knowledge units are routing, summary, and acceleration layers around them. Then explain what freshness signals change in agent behavior.
4. Add a short comparison against the strongest simpler baseline: repo rules, source-of-truth precedence, code/test-first retrieval, and stale-doc flags. Explain why that is necessary but insufficient.
5. Tighten the document substantially. Cut repetition, reduce mechanism sprawl, and make the greenfield/legacy split subordinate to the main product story.

## What Not to Change Yet
- Do not abandon the layered retrieval idea just because it is not fully proven yet. It remains a plausible organizing model and should be refined, not removed.
- Do not remove automation from the story. The right revision is to classify automation by confidence and review burden, not to retreat to manual-first maintenance.
- Do not overreact to the critique of `llms.txt` language by deleting the analogy entirely. Just soften it and stop letting it do too much argumentative work.
- Do not spend the next draft trying to fully solve promotion mechanics. Demote that topic unless stronger evidence emerges.
