# Synthesis Review

## Current Core Thesis

AI coding agents handling a short bug-fix or investigation task in an unfamiliar repo need a small task-specific context pack that gets them to the right code and proving tests quickly while calibrating how much to trust surrounding prose.

## Current Assessment

The proposal is clearer than in the prior cycle because it now distinguishes core claim, likely mechanisms, and exploratory extensions more explicitly. That is real progress. But the main editorial problem is still unresolved: the draft has not narrowed hard enough around one wedge, one primary workflow, one top-line outcome, and one fair baseline.

The critic is right about the unresolved narrowing problem. The defender is right that some criticisms are asking proposal-stage questions at design-spec depth. The right synthesis is to keep the core product claim, keep code/tests as the truth hierarchy, and stop letting candidate mechanisms read like commitments the proposal has not yet earned.

## Repeated Feedback From Earlier Runs
- Repeated and still unresolved: the proposal needs a sharper primary user, not just "an AI coding agent in a repo it does not know well."
- Repeated and still unresolved: the proposal needs a sharper first workflow. The draft still implies short-task bug fix/investigation without fully committing to it.
- Repeated and still unresolved: the proposal needs one primary outcome. "Faster routing, better proof anchoring, and trust calibration" still reads as a bundle rather than a ranked promise.
- Repeated and still unresolved: the proposal must state more plainly that code and tests are the proof surfaces and that prose is subordinate.
- Repeated and still unresolved: mechanism sprawl remains a problem. The draft is better tiered than before, but still too willing to carry hot/warm/cold, index-first loading, automation, promotion, and greenfield/legacy scope together.
- Repeated and still unresolved: the proposal still needs a stronger comparison against the best simpler baseline, not a softened alternative.

## New Feedback In This Run
- New and important: the stronger baseline is now articulated clearly enough to matter. The proposal should compare itself against "tiny repo map + source-of-truth rules + task classification + code/test-first retrieval + lightweight prose fallback," not against generic search.
- New and important: the document should identify the product artifact more crisply. The context pack is the product-facing artifact; the retrieval architecture is a candidate means.
- New but secondary: the proposal would benefit from anti-goals and first-version exclusions so the reader can see what is deliberately not being solved.
- New but mostly deferable: the proposal could name one or two essential automations rather than treating automation as a large support cloud.

## What Seems Clearly True
- The core problem is real: agents in unfamiliar repos do not just need more retrieval; they need orientation, routing, and trust calibration under drift.
- The proposal is getting clearer across review cycles. The explicit separation between core claim, likely mechanisms, and exploratory extensions is an improvement and should be preserved.
- Critic and defender actually agree on the most important revision needs: sharpen the user, sharpen the workflow, sharpen the primary outcome, tighten the context-pack definition, and compare against a stronger baseline.
- The best primary user/workflow is now fairly clear: a short-task coding agent doing bug-fix or investigation work in an unfamiliar repo.
- The proposal is strongest when read as a system for routing the agent to executable evidence, not as a new documentation architecture.
- Usage-aware promotion is not core. It remains exploratory and should not carry argumentative weight in the next draft.

## Open Disagreements
- The main disagreement is about how much the proposal has earned its preferred mechanisms. The critic thinks hot/warm/cold and index-first are still mostly asserted; the defender thinks they are acceptable as leading candidates in a proposal. The synthesis view: keep them, but demote their confidence level and stop writing as if they are already the obvious retrieval spine.
- There is a live disagreement about whether the document is still too doc-centric. The critic sees too much prose-organization thinking; the defender sees necessary orientation structure around code/test-first work. The synthesis view: this is resolved editorially by making the pack definition more proof-first and by saying when prose should be absent.
- There is a smaller disagreement about greenfield and legacy scope. This does not need resolution at the thesis level, but the next draft should pick one as the entry story and demote the other to expansion.

## Must Fix Now
1. Narrow the wedge. State plainly that the proposal is optimizing for a short-task bug-fix/investigation agent in an unfamiliar repo, and make "time to correct code/test working area" the primary outcome.
2. Rewrite the baseline comparison. Test the proposal against the strongest simpler alternative and explain exactly what failure remains without explicit context-pack assembly and trust-calibrated routing.
3. Cut mechanism sprawl in the prose. Keep the core claim, keep the proof hierarchy, keep hot/warm/cold and index-first only as candidate mechanisms, and demote usage-aware promotion, broader automation ambitions, and dual-scope storytelling.

## Defer
- Exact decision policies such as when to return only code/tests, when to include prose, or how agents should react to each freshness label. Those belong in follow-on design or evaluation work once the wedge is clear.
- Detailed automation design. For now, it is enough to name the one or two maintenance capabilities the system depends on and defer the rest.
- Full reconciliation of greenfield and legacy operating modes. Choose one wedge first; treat the other as later applicability.

## What Not to Change Yet
- Do not back away from the core claim that context packs should include trust calibration around drifting knowledge. Repeated criticism has not invalidated that point.
- Do not drop the truth hierarchy. The proposal is right to treat code and tests as proof surfaces and prose as routing and warning material.
- Do not remove hot/warm/cold or index-first entirely. The criticism is not that these ideas are useless; it is that they are currently over-credited relative to the evidence.
- Do not spend another draft expanding promotion mechanics. That topic is still non-core and should remain demoted.
