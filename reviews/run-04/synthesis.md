# Synthesis Review

## Current Core Thesis

AI coding agents doing short bug-fix or investigation work in unfamiliar repositories need a small, trust-aware task launch pack that gets them to the right code and nearest executable evidence faster than a strong launcher built from repo rules, task classification, code/test-first retrieval, and warnings alone.

## Current Assessment

The thesis is now stable. The proposal no longer needs help finding its shape; it needs help proving its differentiation. The wedge is clear, the truth hierarchy is sensible, and the context-pack framing is now a credible product claim. The remaining question is whether explicit launch-pack assembly is materially better than a high-quality launcher plus code/test-first routing and warnings.

The proposal is therefore in the differentiation/validation phase rather than the shaping phase. The next draft should spend less energy on introducing or defending the mechanics and more energy on showing exactly what failure remains after the simpler baseline is in place.

## Review Phase
- Shaping: mostly complete
- Differentiation/validation: now dominant

## Repeated Feedback From Earlier Runs
- Repeated and still valid: the proposal must compare itself against the strongest simpler baseline, not generic search. This is still the central unresolved issue.
- Repeated and improved: the wedge is now clear and specific. The unfamiliar-repo short-task story is no longer the problem.
- Repeated and improved: the truth hierarchy is now explicit and coherent. Code, tests, and nearby executable evidence are clearly the proof surfaces.
- Repeated and still valid: mechanism sprawl remains a risk. Hot/warm/cold, index-first loading, usage-aware promotion, automation, and greenfield/legacy scope still sit close enough to the thesis that the reader can confuse them for the product.
- Repeated and still valid: the primary outcome still needs a more exact operational definition, even though it is much better than before.

## New Feedback In This Run
- New and important: the proposal still has not made the decisive causal case for why a pre-assembled launch pack is necessary rather than merely helpful.
- New and important: the artifact risks reading like a renamed retrieval bundle unless the document makes the user-visible or workflow-visible difference sharper.
- New and important: the baseline comparison should be framed around concrete short-task scenarios and the stitching burden that remains after a strong launcher does its job.
- New and important: the evaluation target is still too loose. "Reduced false starts" is directionally right, but the proposal needs one primary measurement.
- New but secondary: broadening from tests to nearest executable evidence is probably right, but the proposal should keep that proof surface disciplined.

## Resolution Status of Prior Issues
- Resolved:
- The primary user is clear.
- The near-term workflow is clear.
- The context pack is now clearly the product-facing artifact.
- The truth hierarchy is explicit.
- The unfamiliar existing-repo wedge is now dominant.
- Mostly resolved:
- Hot/warm/cold and index-first are now demoted, but they still carry a little too much conceptual weight.
- Automation is scoped more credibly, though its reliability bar still needs sharpening.
- Greenfield applicability is now subordinate, but it still appears more often than the wedge strictly needs.
- Still unresolved:
- The proposal has not yet shown, concretely enough, why a strong launcher still leaves a meaningful gap that only a pack closes.
- The primary success metric remains underdefined.
- The context-pack distinction still needs one operational sentence that makes it obviously different from a thin launcher.

## What Seems Clearly True
- The core claim is now strong enough to review on its own terms.
- Critic and defender are now arguing mostly about differentiation, not about basic framing.
- The real product question is whether launch-pack assembly saves enough stitching and wrong turns to justify a distinct artifact.
- The proposal is strongest when it is framed as launch into executable evidence, not as a retrieval architecture debate.
- Repeated criticism is pointing to a real unresolved issue, not reviewer noise.

## Open Disagreements
- The main disagreement is whether the context pack is already meaningfully distinct from a high-quality launcher or whether it still risks collapsing into better packaging of the same ingredients.
- There is a secondary disagreement about how much weight hot/warm/cold and index-first should carry. The synthesis view remains that they are plausible mechanisms, not the thesis.
- There is a smaller disagreement about automation. Conservative, explainable drift signals seem useful, but they should stay tightly scoped.

## Must Fix Now
1. Make the causal case for launch-pack assembly explicit. Use 2-3 concrete short-task scenarios to show exactly where "tiny map + routing + code/test-first retrieval + warnings" still leaves too much stitching or too many wrong turns.
2. Pick one primary success metric and state it plainly. If the measure is false starts, define it tightly; if it is time to first correct subsystem plus executable check, make that the sole primary outcome.
3. Tighten the product-distinctness story. Add one short, operational sentence that says what the context pack does that a strong launcher does not.

## Defer
- Detailed decision policies for when to include prose versus only code/tests and warnings.
- Richer promotion or usage-aware adaptation; it remains non-core.
- Broader automation and maintenance workflows beyond conservative drift flagging and linkage to changed code/tests.
- Full greenfield/legacy unification beyond keeping the unfamiliar existing-repo wedge first.

## What Not to Change Yet
- Do not retreat from the core claim that agents need trust-aware launch context around code/tests or nearby executable evidence.
- Do not remove the truth hierarchy.
- Do not delete hot/warm/cold or index-first entirely; they are still plausible candidates, just not the heart of the argument.
- Do not reopen the wedge or primary user framing unless the proposal regresses.
