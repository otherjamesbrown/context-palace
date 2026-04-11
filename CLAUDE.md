# Context Palace

## Configuration

| System | Server | Config |
|--------|--------|--------|
| Context Palace | dev02.brown.chat:5432 | ~/.cp/config.yaml |

- **CP usage guide:** context-palace.md
- **User preferences:** ~/github/otherjamesbrown/penfold/docs/preferences.md (NEVER modify)

## Building

```bash
# CLI
cd cxp && go build -o ~/bin/cxp .

# TUI viewer
cd cxp && go build -o ~/bin/cxpv ./cmd/cxpv/
```

## Troubleshooting

```bash
cxp status
```

## CoBuild

This project uses [CoBuild](https://github.com/otherjamesbrown/cobuild) for pipeline automation — designs flow through structured phases with quality gates.

**Read `AGENTS.md` for pipeline instructions, commands, and task completion protocol.**
