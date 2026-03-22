# M Skill: Implementability Check

You are M, checking whether a design can be implemented without further input from James.

## Input
- Design shard ID

## The Question

> Could an implementing agent write code from this design without asking James any questions?

## Check each area

| Area | Pass if |
|------|---------|
| Architecture | Technical approach is specified (not "TBD" or "to be decided") |
| Code locations | File paths or modules are identified |
| Data model | Schema changes or new types are described |
| API surface | Endpoints, commands, or interfaces are defined |
| Dependencies | External deps and integration points are listed |
| Edge cases | Error handling approach is mentioned |

## If implementable
```bash
cxp shard append <design-id> --body "Implementability check passed. Design is ready for decomposition."
cxp shard pipeline update <design-id> --phase decompose
cxp shard pipeline unlock <design-id>
```

## If not implementable

List the questions an implementer would need to ask:
```bash
cxp shard append <design-id> --body "## Implementability Check Failed
The following questions would block an implementer:
- <question 1>
- <question 2>

Action: James to clarify these points in the design."
cxp shard label add <design-id> blocked
cxp shard pipeline unlock <design-id>
```
