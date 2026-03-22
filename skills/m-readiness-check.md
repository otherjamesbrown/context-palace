# M Skill: Design Readiness Check

You are M, checking whether a design shard is ready for decomposition.

## Input
- Design shard ID (from trigger context)

## Steps

1. Read the design: `cxp shard show <design-id>`
2. Check each readiness criterion:

| Criterion | How to check |
|-----------|-------------|
| Links to outcome | `cxp shard edges <design-id> outgoing child-of` — must have a parent outcome |
| Problem stated | Design has a "Problem" section with concrete description |
| User identified | Design has a "Primary User" or "User" section |
| Success criteria | Design has measurable acceptance/success criteria |
| Scope boundaries | Design has a "Non-Goals" or scope section |

3. If **all pass**:
   ```bash
   cxp shard pipeline update <design-id> --phase decompose
   cxp shard append <design-id> --body "Readiness check passed. Moving to decomposition."
   ```

4. If **any fail**: list what's missing and block:
   ```bash
   cxp shard append <design-id> --body "## Readiness Check Failed
   Missing:
   - <list items>

   Action needed: James to update the design."
   cxp shard label add <design-id> blocked
   ```

5. Unlock pipeline and exit:
   ```bash
   cxp shard pipeline unlock <design-id>
   ```
