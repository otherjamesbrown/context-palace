# Critic Review

## Overall Judgment

The proposal identifies a real and important problem, but it is not yet a convincing product proposal. It bundles too many major bets together, treats several of them as mutually reinforcing without proving they are, and does not clearly define the primary user, the core workflow, or the minimum product outcome. As written, it reads more like a directionally interesting framework than a sharply argued product case.

The biggest weakness is that the proposal keeps saying "AI coding agents need better context" while avoiding the harder product question: what specific failure in an agent's workflow is the product meant to fix first, and why is this layered, evidence-aware, automation-heavy model better than simpler alternatives?

## Major Concerns

- The proposal is too broad to evaluate as a single product thesis. It is simultaneously arguing for hot/warm/cold context, index-first lazy loading, evidence-aware retrieval, trust/freshness scoring, metadata enrichment, PR analysis, test linking, drift review, KB coverage monitoring, and promotion suggestions. That is not one proposal. It is a stack of loosely connected bets. Because of that, it is hard to tell what is foundational, what is optional, and what should be validated first.

- The user and product outcome are underdefined. "AI coding agents working in real software projects" is too broad to anchor product design. A repo-embedded autonomous coding agent, an IDE assistant, and a human-supervised task runner do not have the same context needs or interaction model. The proposal also does not define the primary success metric. Is the goal faster first useful action, fewer stale-doc mistakes, higher task completion, lower token usage, or better human trust in agent behavior? Without a target user and target outcome, the rest of the design cannot be judged properly.

- The problem statement is directionally correct but too generic. "Too much context, too little context, stale context" is true, but it is true of almost any retrieval system. The proposal does not show that these are the dominant failure modes for AI coding agents relative to simpler issues like bad repo search, poor source-of-truth rules, weak code/test grounding, or lack of task decomposition. It risks over-attributing agent failures to document retrieval structure when many failures come from reasoning, planning, or verification discipline.

- Hot/warm/cold is not yet justified as the right base model. It is plausible, but the proposal mostly argues from intuition. It does not explain why three tiers are meaningfully better than two, why the boundaries would stay legible, or why this mental model maps cleanly onto real coding tasks. Many tasks cross subsystem boundaries immediately. Others need code-local grounding, not knowledge-layer navigation. There is a real risk that the tiering system reflects information architecture neatness more than agent behavior.

- The proposal overestimates the value of `llms.txt`-style index-first lazy loading as a base model for coding work. This model makes sense when documentation is the main discovery surface. But in many software tasks, code search, symbol navigation, git history, and tests are the real index. The proposal acknowledges code/test anchors later, but still frames documentation hierarchy as the main retrieval spine. That may be backwards. For coding agents, docs may be an annotation layer on top of code-grounded retrieval, not the primary navigation model.

- Usage-aware promotion is weakly argued and likely to create complexity faster than value. Repeated access does not necessarily mean durable importance. It can also mean confusing docs, poor top-level routing, temporary project churn, or a pathological hotspot. Promoting frequently accessed grandchildren into higher-level indexes risks turning the index into a popularity cache rather than a reliable conceptual map. The proposal does not define the failure mode it is fixing, how promotion stays bounded, or how it avoids making the hierarchy less comprehensible over time.

- The proposal is not convincing on why this is better than simpler alternatives. It gestures at "single large guide plus search" but does not seriously compare against concrete simpler models such as:
  a repo-level rules file plus source-of-truth precedence,
  code-and-test-first retrieval with stale-doc warnings,
  a flat set of focused notes with good tagging/search,
  or task-scoped context packs generated from code ownership and git history.
  Right now, the proposal assumes the layered hierarchy is the right answer before earning that conclusion.

- The greenfield and legacy stories are both individually plausible but jointly too convenient. Greenfield wants lightweight capture and promotion because structure does not exist yet. Legacy wants mapping, triage, contradiction handling, and freshness scoring because too much structure already exists and is dirty. Those are meaningfully different product problems. The proposal currently treats them as two modes of the same system rather than proving they can share a coherent product surface without becoming abstract and overbuilt.

- The automation story is not yet credible as a product commitment. The proposal assumes background AI jobs can continuously infer links, detect drift, identify proof surfaces, and suggest promotions in a way that is accurate enough to matter and quiet enough to be trusted. That is a very strong assumption. The document says these jobs can "observe, infer, and flag," but does not address false positives, review burden, noisy suggestions, or the common fate of automated knowledge-maintenance systems: teams start ignoring them. The proposal relies heavily on automation while underplaying the operational trust problem.

- Trust and freshness signals are underspecified where they matter most: behavior change. The proposal says the system should emit warnings like fresh, stale, volatile, or conceptually useful but detail drift suspected. But it never explains how these signals change what the agent actually does. Does the agent stop using the doc, downgrade it, ask for verification, or route to tests first? If the signals do not produce different retrieval and reasoning behavior, they are just metadata ornamentation.

- The proposal is internally conflicted about where truth lives. It argues that docs drift, tests are stronger evidence, code is a proof surface, and source-of-truth precedence matters. But it still centers a document hierarchy as the navigation backbone. If implementation evidence is the key differentiator, then the document should say much more clearly whether docs are primary guidance, secondary summaries, or merely retrieval accelerators. Right now it tries to have it both ways.

## Secondary Concerns

- The proposal repeats the same claims in multiple sections instead of tightening them into sharper decisions. That creates apparent completeness while masking missing tradeoffs.

- "Evidence-aware" remains too abstract. The document lists possible signals but does not define what counts as evidence versus correlation versus heuristic guesswork.

- "Knowledge units" are treated as obviously desirable, but the proposal does not address fragmentation risk. Breaking knowledge into small units can reduce overload, but it can also destroy narrative coherence and increase lookup overhead.

- The hot layer sounds overloaded. It is supposed to stay small, but it is also expected to include rules, project map, source-of-truth precedence, and retrieval guidance. That may already be too much for something that is always loaded.

- The proposal talks about "task-specific context packs" as if they naturally fall out of the model, but that is really the product. The document should center that outcome much more clearly and treat the hierarchy as one possible mechanism, not the main story.

- The document does not separate authoring burden from review burden. Even if metadata is inferred automatically, someone still has to validate drift warnings, approve links, and judge promotions. That is still labor, and it may concentrate on the people least likely to do it.

- There is no serious discussion of failure modes where the system becomes self-reinforcing in a bad way. Example: a stale but frequently accessed note gets promoted, referenced more often, and becomes more "important" due to its own visibility.

- The proposal assumes that "small navigable index" is inherently better for LLMs, but that may depend heavily on the agent runtime. Some agents are much better at active search, iterative repo exploration, and tool calling than others. The design may be overfitting one interaction style.

## Questions the Proposal Must Answer

- Who exactly is the primary user? A mostly autonomous coding agent, a human-supervised assistant, or the human who maintains the repo context?

- What is the first product outcome to optimize for? Faster onboarding, fewer incorrect changes, fewer stale-doc mistakes, reduced token cost, or something else?

- What is the minimum viable version of this idea? Which parts are essential, and which are optional later enhancements?

- Why is hot/warm/cold the right base model for coding tasks specifically, not just for information architecture generally?

- Why is documentation-first indexing the right spine when code, tests, and git history are often the most trustworthy discovery surfaces?

- In what situations would a simpler model outperform this one, and why is that acceptable?

- What observable behavior should change when a doc is marked "possibly stale" versus "likely current"?

- How much noise can the automation layer produce before the system becomes ignored? What is the review burden budget?

- What makes usage-aware promotion a product win rather than a clever mechanism? What specific user pain does it measurably remove?

- How does this system avoid reinforcing the wrong artifacts, such as popular but weak notes or frequently touched but poorly scoped areas?

- Can one product really serve greenfield and legacy repos well, or should the proposal admit that one is the true wedge and the other is a later expansion?

- Why should a team adopt this instead of starting with a much simpler package: repo rules, source-of-truth precedence, code/test anchors, and stale-doc flags?

## Recommended Revisions

- Narrow the proposal to a single core product claim. A stronger version would say: "We help coding agents generate better task-specific context packs by grounding documentation in code/test evidence and freshness signals." Then treat hot/warm/cold and lazy loading as candidate mechanisms, not the headline.

- Define the primary user and workflow explicitly. Pick one main scenario and stay disciplined about it, for example: "an agent receives a short task in an unfamiliar repo and must reach correct code and proof surfaces quickly."

- State the primary outcome in measurable terms. Examples: reduce time to first relevant file, reduce stale-doc-led misrouting, increase correct first-pass task scoping, or reduce unnecessary context loaded per task.

- Separate the proposal into tiers of confidence:
  1. core belief you are confident in,
  2. supporting mechanisms you believe are promising,
  3. speculative extensions that should not be mistaken for the MVP.

- Either defend hot/warm/cold with more rigor or soften it. Show why three layers matter, what each layer uniquely contributes, and where the model breaks. If you cannot do that, stop presenting it as the natural retrieval spine.

- Reframe `llms.txt`-style lazy loading more carefully. Make clear whether it is a direct analogy, an inspiration, or a limited design pattern. Right now the analogy is doing too much argumentative work.

- Cut or demote usage-aware promotion unless you can make a much stronger case for it. It currently reads like optional optimization complexity, not part of the core value proposition.

- Clarify the role of documents versus code/tests. Say explicitly whether docs are the navigation layer, the summary layer, or one evidence source among several. This is currently blurred.

- Be more skeptical and concrete about automation. Specify which automated inferences are critical, which are advisory only, and what level of noise is acceptable. A proposal that depends on continuous background agents needs a much stronger trust story.

- Decide whether greenfield and legacy are both in scope for the first version. If yes, explain the shared core and the different workflows. If not, choose one as the wedge and say so.

- Add a serious comparison against simpler alternatives and explain why they are insufficient. Without that, the proposal feels over-designed.

- Reduce repetition and increase tradeoff density. The next revision should spend less time restating benefits and more time defending controversial assumptions.
