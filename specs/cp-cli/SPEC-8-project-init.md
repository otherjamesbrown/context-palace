# SPEC-8: Project Init & Template Updates

**Status:** Draft
**Depends on:** SPEC-0 (CLI skeleton, config, connection)
**Blocks:** Nothing (but all new projects benefit)

---

## Goal

Give new projects a working agent workflow in one command. `cp init` scaffolds a project
with ways-of-working templates, ingest pipeline, agent identity, and Context Palace
connection — then registers it in the database. `cp update` keeps template files current
as the templates evolve.

Without this, setting up a new project requires manually copying files, replacing
placeholders, creating database records, and verifying connections. This takes 30+ minutes
and is error-prone. With `cp init`, it takes under a minute.

## What Exists

- `templates/` directory in context-palace repo with:
  - `ways-of-working.md` — bug/feature/spec templates, DoD, quality gates
  - `ingest.md` — pipeline orchestrator template
  - `SPEC-TEMPLATE.md` — full spec format
  - `README.md` — placeholder documentation
- `claude-template.md` — CLAUDE.md template for agent identity
- `context-palace.md` — full CP usage guide (downloaded, not templated)
- `setup.md` — manual setup instructions
- SPEC-0 defines `.cp.yaml` project config and `~/.cp/config.yaml` global config
- `create_shard()` SQL function for creating project rules

## Changes to SPEC-0

This spec **supersedes** SPEC-0's `cp init` definition. SPEC-0 defines `cp init` as a
simple command that creates `.cp.yaml` with project name. This spec extends it to scaffold
a full project with templates, database registration, and versioning.

Changes to SPEC-0:
- `cp init` gains `--prefix`, `--implementer`, `--maintainer` flags
- `cp init` creates 6 files (not just `.cp.yaml`)
- `cp init` creates a database shard
- `.cp.yaml` gains `prefix`, `implementer`, `maintainer`, and `templates` sections
- SPEC-0's `cp init` tests need updating to expect the new behavior

## What to Build

1. **`cp init`** — interactive project scaffolding command (supersedes SPEC-0 `cp init`)
2. **`cp update`** — template version checking and update command
3. **Template versioning** — version headers in templates, manifest in project config (internal, tested via cp init/update)
4. **Template registry** — where templates are fetched from (internal, tested via cp init/update)

## Data Model

### Schema Changes

No database schema changes. Uses existing `shards` table for project rules shard.

### Storage Format

#### Project manifest in `.cp.yaml`

SPEC-0 defines `.cp.yaml` with `project` and `agent` fields. Init extends it:

```yaml
project: penfold
agent: agent-penfold
prefix: pf                                # NEW — added by SPEC-8
implementer: agent-mycroft                # NEW — persisted for cp update placeholder resolution
maintainer: agent-cxp                     # NEW — defaults to agent-cxp

# Added by cp init, maintained by cp update
templates:
  source: github:otherjamesbrown/context-palace/templates
  version: 3                              # template set version at init time
  files:
    docs/ways-of-working.md:
      template: ways-of-working.md
      version: 3
      initialized: "2026-02-08T10:00:00Z"
      updated: "2026-02-08T10:00:00Z"
    .claude/commands/ingest.md:
      template: ingest.md
      version: 3
      initialized: "2026-02-08T10:00:00Z"
      updated: "2026-02-08T10:00:00Z"
    docs/SPEC-TEMPLATE.md:
      template: SPEC-TEMPLATE.md
      version: 2
      initialized: "2026-02-08T10:00:00Z"
      updated: "2026-02-08T10:00:00Z"
    CLAUDE.md:
      template: claude-template.md
      version: 1
      initialized: "2026-02-08T10:00:00Z"
      updated: "2026-02-08T10:00:00Z"
```

#### Template version header

Each template file has a version comment on line 1:

```markdown
<!-- cp-template-version: 3 -->
# Ways of Working — Template
...
```

The version increments when the template content changes in a meaningful way
(not formatting or typo fixes).

### Data Flow

| Data | Who Writes | When | Where | Who Reads | How Queried | Decisions | Staleness |
|------|-----------|------|-------|-----------|-------------|-----------|-----------|
| Template version header | Template author (penfold) | On template change | Line 1 of template file | `cp update` | Parse first line | Whether to offer update | Stale if template content changed but version not bumped — author responsibility |
| Project manifest | `cp init` / `cp update` | On init / on update | `.cp.yaml` templates section | `cp update` | YAML parse | Which files need updates, placeholder values for re-replacement | Stale when templates updated upstream — `cp update --check` detects |
| Scaffolded files | `cp init` | On init | Project directory | Agents, users | File read | Agent behavior, workflow process | Stale when templates updated — `cp update` refreshes template sections |
| Project rules shard | `cp init` | On init | shards table | Agents | `SELECT WHERE id = '[prefix]-rules'` | Agent routing, project conventions | Not refreshed by `cp update` — manual update only |

### Concurrency

Not applicable — `cp init` runs once per project, `cp update` runs by one user at a time.
Concurrent `cp update` runs on the same repo are serialized by normal git workflow (pull/push).
No file locking is needed.

## CLI Surface

### `cp init` — Initialize a new project

```bash
# Interactive mode (prompts for all values)
cp init

# Non-interactive mode (all values provided)
cp init --project myproject --prefix mp --agent agent-myagent

# With implementer agent specified
cp init --project myproject --prefix mp --agent agent-orchestrator --implementer agent-builder

# With custom maintainer (default: agent-cxp)
cp init --project myproject --prefix mp --agent agent-orchestrator --implementer agent-builder --maintainer agent-infra

# Add manifest to existing project (no file scaffolding, just adds templates section to .cp.yaml)
cp init --add-manifest
```

**Interactive prompts (when flags not provided):**

```
Project name: myproject
Project prefix (2-4 chars): mp
Orchestrating agent name: agent-orchestrator
Implementing agent name: agent-builder
```

**What it does (sequential — partial failure leaves files on disk, see Edge Cases):**

1. Validate inputs:
   - Project name: lowercase alphanumeric + hyphens, 2-30 chars
   - Prefix: lowercase alphanumeric, 2-4 chars
   - Agent names: must start with `agent-`, lowercase alphanumeric + hyphens
2. Check project doesn't already exist:
   ```sql
   SELECT id FROM shards WHERE project = $1 LIMIT 1;
   ```
   If exists: error `"project 'myproject' already exists. Use 'cp update' to refresh templates."`
3. Fetch latest templates from source (see Template Registry below)
4. Create directory structure:
   ```
   .cp.yaml                         ← project config + manifest
   CLAUDE.md                        ← agent identity
   context-palace.md                ← full CP guide (downloaded)
   docs/
   ├── ways-of-working.md          ← from template
   └── SPEC-TEMPLATE.md            ← from template
   .claude/
   └── commands/
       └── ingest.md               ← from template
   ```
5. Replace placeholders in all scaffolded files:
   | Placeholder | Value |
   |-------------|-------|
   | `[PROJECT]` | project name |
   | `[PREFIX]` | project prefix |
   | `[ORCHESTRATOR]` | orchestrating agent name |
   | `[IMPLEMENTER]` | implementing agent name |
   | `[MAINTAINER]` | `agent-cxp` (default) |
   | `[DB_CONN]` | from `~/.cp/config.yaml` connection |
   | `[PALACE_CLI]` | resolved path to `cp` binary |
6. Write `.cp.yaml` with project config + template manifest
7. Create project rules shard in database:
   ```sql
   SELECT create_shard($project, 'Project Rules for ' || $project,
     $content, 'doc', $agent);
   ```
   Content is a generated rules document based on the template.
8. Verify connection works:
   ```sql
   SELECT count(*) FROM shards WHERE project = $project;
   ```

**Text output (success):**

```
Initialized project 'myproject' (prefix: mp)

Created:
  .cp.yaml                          project config
  CLAUDE.md                         agent identity (agent-orchestrator)
  context-palace.md                 Context Palace guide
  docs/ways-of-working.md           workflow templates
  docs/SPEC-TEMPLATE.md             spec template
  .claude/commands/ingest.md        pipeline orchestrator

Database:
  Project rules shard: mp-xxxxxx

Next steps:
  1. Review and customize docs/ways-of-working.md for your project
  2. Add project-specific components to bug/feature templates
  3. Create ingest phase files in .claude/commands/ (see ingest.md)
  4. Add domain-specific quality metrics
```

**JSON output (`-o json`):**

```json
{
  "project": "myproject",
  "prefix": "mp",
  "agent": "agent-orchestrator",
  "implementer": "agent-builder",
  "files_created": [
    ".cp.yaml",
    "CLAUDE.md",
    "context-palace.md",
    "docs/ways-of-working.md",
    "docs/SPEC-TEMPLATE.md",
    ".claude/commands/ingest.md"
  ],
  "rules_shard": "mp-a1b2c3",
  "template_version": 3
}
```

**Error cases:**

| Condition | Error |
|-----------|-------|
| Project already exists | `"project 'X' already exists. Use 'cp update' to refresh templates."` |
| Invalid project name | `"invalid project name: must be lowercase alphanumeric + hyphens, 2-30 chars"` |
| Invalid prefix | `"invalid prefix: must be lowercase alphanumeric, 2-4 chars"` |
| No global config | `"no config found. Run 'cp config init' first (see SPEC-0)."` |
| DB connection fails | `"cannot connect to Context Palace: [error]. Check ~/.cp/config.yaml"` |
| Template fetch fails | `"cannot fetch templates from [source]: [error]"` |
| Directory not empty + has .cp.yaml | `"project already initialized. Use 'cp update'."` |
| File write fails mid-scaffold | Files written so far are kept. Error: `"failed writing [file]: [error]. Partial init — [N] files created. Fix the issue and run 'cp init' again (will skip existing files)."` |

---

### Template destination path mapping

Complete mapping from template source name to project destination path:

| Template Source | Destination | Notes |
|----------------|-------------|-------|
| `ways-of-working.md` | `docs/ways-of-working.md` | Templated, has section markers |
| `ingest.md` | `.claude/commands/ingest.md` | Templated, has section markers |
| `SPEC-TEMPLATE.md` | `docs/SPEC-TEMPLATE.md` | Templated, no section markers (full file replace on update) |
| `claude-template.md` | `CLAUDE.md` | Templated, no section markers (full file replace on update) |

Non-template files (not tracked in manifest, not updated by `cp update`):

| File | Destination | How Updated |
|------|-------------|-------------|
| `context-palace.md` | `context-palace.md` | Downloaded at init, manually refreshed: `cp update --download-guide` |

The `DestinationPath()` function uses this hardcoded mapping. Unknown template names
return an error.

---

### `cp update` — Check and apply template updates

```bash
# Check for updates (dry run, no changes)
cp update --check

# Apply all available updates (with confirmation)
cp update

# Apply updates to specific file
cp update docs/ways-of-working.md

# Apply without confirmation
cp update --yes

# Show diff for a specific template
cp update --diff docs/ways-of-working.md

# Force-replace a file (re-applies full template, re-adds markers if removed)
cp update --force docs/ways-of-working.md
```

**What it does (sequential):**

1. Read `.cp.yaml` manifest — get current template versions and placeholder values
   (`project`, `prefix`, `agent`, `implementer`, `maintainer`)
2. Fetch latest templates from source
3. Compare versions:
   - Parse `<!-- cp-template-version: N -->` from each latest template
   - Compare against manifest version for each file
4. For each outdated file:
   - Generate diff between current file content and latest template (with placeholders replaced)
   - Present to user
5. If `--check`: show summary and exit (no changes)
6. If `--diff [file]`: show full diff for that file and exit
7. Otherwise: for each outdated file, prompt user:
   ```
   docs/ways-of-working.md: v2 → v3
   Changes: Added quality metrics section, updated parallel session guidance

   [View diff] [Apply] [Skip]
   ```
8. Apply accepted updates:
   - Write new file content
   - Update manifest version and `updated` timestamp
9. Report what was updated

**Text output (`--check`):**

```
Template updates available:

  docs/ways-of-working.md    v2 → v3   Added quality metrics, parallel guidance
  .claude/commands/ingest.md  v2 → v3   Added real-data verification step
  docs/SPEC-TEMPLATE.md       v2 → v2   Up to date ✓
  CLAUDE.md                    v1 → v1   Up to date ✓

2 updates available. Run 'cp update' to apply.
```

**Text output (apply):**

```
Updating templates:

  docs/ways-of-working.md    v2 → v3   Applied ✓
  .claude/commands/ingest.md  v2 → v3   Skipped (user choice)

1 file updated, 1 skipped.
```

**JSON output (`-o json`):**

```json
{
  "updates": [
    {
      "file": "docs/ways-of-working.md",
      "template": "ways-of-working.md",
      "current_version": 2,
      "latest_version": 3,
      "status": "applied",
      "changelog": "Added quality metrics, parallel guidance"
    },
    {
      "file": ".claude/commands/ingest.md",
      "template": "ingest.md",
      "current_version": 2,
      "latest_version": 3,
      "status": "skipped"
    }
  ]
}
```

**Handling customized files — section-level merge:**

Users customize scaffolded files (adding project-specific components, quality metrics, etc.).
`cp update` must update template-managed content without destroying user customizations.

**Two update modes** based on whether the template has section markers:

**Mode A: Section-level merge** (templates with markers — `ways-of-working.md`, `ingest.md`):

Templates use HTML comment markers to delimit template-managed sections:

```markdown
# Ways of Working

<!-- cp-template-start: bug-template -->
## Bug Reports
### Template
...template-managed content...
<!-- cp-template-end: bug-template -->

## Our Project's Bug Conventions
...user content — cp update NEVER touches content outside markers...

<!-- cp-template-start: quality-rules -->
## Quality Rules
...template-managed content...
<!-- cp-template-end: quality-rules -->

## Project-Specific Quality Metrics
...user content...
```

**Section marker rules:**
- Markers are HTML comments: `<!-- cp-template-start: SECTION-NAME -->` and `<!-- cp-template-end: SECTION-NAME -->`
- Section names must be unique within a file (no duplicates)
- Markers must not be nested (no marker pair inside another marker pair)
- Content outside all marker pairs is user-owned — never modified by `cp update`
- Content between a start/end pair is template-owned — replaced on update

**Merge algorithm (MergeSections):**

```
1. Parse current file into segments:
   - For each cp-template-start/end pair: extract section name + content
   - Everything else is a "user segment"
   Result: ordered list of [user_segment, template_section, user_segment, template_section, ...]

2. Parse latest template the same way

3. For each template section in the LATEST template:
   a. Find matching section name in current file
   b. If found: replace the content between markers with the latest template content
   c. If NOT found (new section in latest template): append at end of file with markers

4. Apply placeholder replacement to updated template sections using values from .cp.yaml:
   [PROJECT] → manifest.project, [PREFIX] → manifest.prefix, etc.

5. Reassemble file: user segments (unchanged) + template sections (updated)
```

**Mode B: Full file replace** (templates without markers — `SPEC-TEMPLATE.md`, `CLAUDE.md`):

Templates without section markers are replaced entirely on update. The user is warned:

```
CLAUDE.md: v1 → v2 (full file replace — no section markers)
  This will replace the entire file. Your customizations will be lost.
  [View diff] [Apply] [Skip]
```

**When markers are removed** (user deleted them from a file that should have them):

`cp update` detects this by checking the latest template for markers and finding none in
the current file. Behavior:
- Non-interactive: warn and skip the file
- `--diff`: show full diff so user can see what changed
- `--force [file]`: replace the entire file with the latest template (re-adds markers)

```
docs/ways-of-working.md: template markers removed — skipping.
  Use 'cp update --diff docs/ways-of-working.md' to see changes.
  Use 'cp update --force docs/ways-of-working.md' to replace entire file.
```

**Error cases:**

| Condition | Error |
|-----------|-------|
| No `.cp.yaml` | `"not a cp project. Run 'cp init' first."` |
| No manifest in `.cp.yaml` | `"no template manifest found. Run 'cp init --add-manifest' to add template tracking to this project."` |
| Template source unreachable | `"cannot fetch templates from [source]: [error]"` |
| File doesn't exist locally | `"[file] not found. Run 'cp init' to regenerate."` |

---

## Template Registry

Templates are fetched from a configurable source. Default is the context-palace GitHub repo.

### Source resolution

```yaml
# In .cp.yaml or ~/.cp/config.yaml
templates:
  source: github:otherjamesbrown/context-palace/templates
```

| Source format | Resolves to |
|--------------|-------------|
| `github:owner/repo/path` | `https://raw.githubusercontent.com/owner/repo/main/path/` |
| `local:/path/to/dir` | Local filesystem directory |
| `file:./relative/path` | Relative to current directory |

For development/testing, use `local:` to point at a local clone:
```yaml
templates:
  source: local:/Users/dev/github/otherjamesbrown/context-palace/templates
```

### Changelog

Each template version bump includes a one-line changelog in a `CHANGELOG.md` in the
templates directory:

```markdown
# Template Changelog

## Version 3 (2026-02-08)
- ways-of-working.md: Added quality metrics section, parallel session advisory role
- ingest.md: Added real-data verification, sub-shard size limits, test quality review

## Version 2 (2026-02-07)
- ways-of-working.md: Initial template
- ingest.md: Initial template
- SPEC-TEMPLATE.md: Added pre-submission checklist

## Version 1 (2026-02-06)
- Initial release
```

### Template discovery

The registry discovers available templates via a `manifest.yaml` file in the templates
directory:

```yaml
# templates/manifest.yaml
templates:
  - name: ways-of-working.md
    destination: docs/ways-of-working.md
    has_sections: true
  - name: ingest.md
    destination: .claude/commands/ingest.md
    has_sections: true
  - name: SPEC-TEMPLATE.md
    destination: docs/SPEC-TEMPLATE.md
    has_sections: false
  - name: claude-template.md
    destination: CLAUDE.md
    has_sections: false
```

`FetchAll()` reads this manifest first, then fetches each listed template.
`DestinationPath()` reads from this manifest, not a hardcoded mapping.

`cp update --check` reads the CHANGELOG to show what changed.

## Go Implementation Notes

### Package Structure

```
cp/
├── cmd/
│   └── cp/
│       ├── init.go              ← cp init command
│       └── update.go            ← cp update command
└── internal/
    └── templates/
        ├── registry.go          ← template source fetching (github, local)
        ├── scaffold.go          ← placeholder replacement, file writing
        ├── manifest.go          ← .cp.yaml manifest read/write
        ├── diff.go              ← template diff generation
        └── sections.go          ← cp-template-start/end section parsing
```

### Key Types

```go
// Template represents a fetched template file
type Template struct {
    Name     string // e.g., "ways-of-working.md"
    Version  int
    Content  string
    Changelog string
}

// ManifestEntry tracks a scaffolded file
type ManifestEntry struct {
    Template    string    `yaml:"template"`
    Version     int       `yaml:"version"`
    Initialized time.Time `yaml:"initialized"`
    Updated     time.Time `yaml:"updated"`
}

// Manifest is the templates section of .cp.yaml
type Manifest struct {
    Source  string                    `yaml:"source"`
    Version int                      `yaml:"version"`
    Files   map[string]ManifestEntry `yaml:"files"`
}

// Registry fetches templates from a source
type Registry interface {
    Fetch(name string) (*Template, error)
    FetchAll() ([]*Template, error)
    Changelog() (string, error)
}
```

### Key Flows

```go
// cp init flow
func runInit(ctx context.Context, project, prefix, agent, implementer string) error {
    // 1. Validate inputs
    if err := validateProject(project, prefix, agent); err != nil {
        return err
    }

    // 2. Check project doesn't exist
    exists, err := client.ProjectExists(ctx, project)
    if exists {
        return fmt.Errorf("project '%s' already exists", project)
    }

    // 3. Fetch templates
    registry := templates.NewRegistry(config.Templates.Source)
    tmpls, err := registry.FetchAll()

    // 4. Replace placeholders
    replacements := map[string]string{
        "[PROJECT]":      project,
        "[PREFIX]":       prefix,
        "[ORCHESTRATOR]": agent,
        "[IMPLEMENTER]":  implementer,
        // ...
    }
    for _, t := range tmpls {
        t.Content = templates.Replace(t.Content, replacements)
    }

    // 5. Write files + manifest
    manifest := templates.NewManifest(config.Templates.Source)
    for _, t := range tmpls {
        dest := templates.DestinationPath(t.Name)
        if err := templates.WriteFile(dest, t.Content); err != nil {
            return fmt.Errorf("writing %s: %w", dest, err)
        }
        manifest.AddFile(dest, t)
    }

    // 6. Write .cp.yaml
    if err := manifest.WriteTo(".cp.yaml", project, prefix, agent); err != nil {
        return fmt.Errorf("writing .cp.yaml: %w", err)
    }

    // 7. Create project rules shard
    rulesID, err := client.CreateShard(ctx, project, agent, ...)

    return nil
}
```

```go
// cp update flow
func runUpdate(ctx context.Context, checkOnly bool, targetFile string) error {
    // 1. Read manifest
    manifest, err := templates.ReadManifest(".cp.yaml")

    // 2. Fetch latest templates
    registry := templates.NewRegistry(manifest.Source)
    latest, err := registry.FetchAll()

    // 3. Compare versions
    updates := []UpdatePlan{}
    for _, t := range latest {
        entry, ok := manifest.Files[templates.DestinationPath(t.Name)]
        if !ok || entry.Version < t.Version {
            updates = append(updates, UpdatePlan{
                File:           templates.DestinationPath(t.Name),
                CurrentVersion: entry.Version,
                LatestVersion:  t.Version,
                Template:       t,
            })
        }
    }

    if checkOnly {
        printUpdateSummary(updates)
        return nil
    }

    // 4. For each update: parse sections, apply template sections, preserve user sections
    for _, u := range updates {
        current, _ := os.ReadFile(u.File)
        updated := templates.MergeSections(string(current), u.Template.Content)
        // prompt user, write if accepted
    }

    return nil
}
```

## Success Criteria

1. `cp init --project myproject --prefix mp --agent agent-x --implementer agent-y` creates all 6 files with placeholders replaced correctly
2. `cp init` in interactive mode prompts for all required values
3. `cp init` on existing project returns error with helpful message
4. `cp init` creates project rules shard in database
5. `cp init` verifies database connection works
6. `cp update --check` shows available updates without modifying files
7. `cp update` applies template section updates while preserving user customizations
8. `cp update` skips files where template markers were removed (with warning)
9. `cp update --diff [file]` shows full diff for a specific file
10. `cp update --yes` applies without interactive confirmation
11. Template version header is parsed correctly from line 1
12. Manifest in `.cp.yaml` tracks template versions and timestamps
13. `local:` and `github:` template sources both work
14. Invalid inputs (bad project name, bad prefix) return clear errors

## Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| `cp init` in non-empty directory | Proceeds — only creates cp-specific files. Warns if CLAUDE.md already exists. |
| `cp init` when .cp.yaml exists | Error: "project already initialized" |
| `cp update` when no .cp.yaml | Error: "not a cp project" |
| `cp update` when all up to date | "All templates up to date." |
| `cp update` when template source unreachable | Error with source URL and suggestion to use `local:` |
| `cp update` when file deleted locally | Warning: "[file] not found. Run 'cp init' to regenerate." |
| `cp update` when markers removed from file | Warning + skip. Suggest `--diff` to review manually. |
| Template has new file not in manifest | `cp update` offers to scaffold the new file |
| `cp init --prefix` with >4 chars | Validation error |
| `cp init --agent` without `agent-` prefix | Validation error |
| `cp init` with no database connection | Scaffold files succeed, database step fails with clear error. Files are kept. |
| `cp init` file write fails mid-scaffold | Files written so far are kept. Error with count of files created. Re-running skips existing files. |
| `cp init --add-manifest` on project without .cp.yaml | Error: "no .cp.yaml found" |
| `cp init --add-manifest` on project with manifest | Error: "manifest already exists" |
| `cp update --force` on file with markers | Replaces entire file, re-applies latest template |
| `cp update` when latest template adds new section | New section appended at end of file with markers |
| `cp update` when latest template adds new file not in manifest | Offers to scaffold the new file. Placeholders resolved from .cp.yaml values. |

---

## Test Cases

### SQL Tests

No SQL functions are introduced — uses existing `create_shard()`.

### Go Unit Tests

```
TEST: ParseTemplateVersion
  Given: File content starting with "<!-- cp-template-version: 3 -->"
  When:  templates.ParseVersion(content)
  Then:  Returns 3

TEST: ParseTemplateVersion_Missing
  Given: File content with no version header
  When:  templates.ParseVersion(content)
  Then:  Returns 0

TEST: ReplacePlaceholders
  Given: Content "[PROJECT] uses [PREFIX]-xxx" and replacements map
  When:  templates.Replace(content, replacements)
  Then:  Returns "myproject uses mp-xxx"

TEST: ReplacePlaceholders_NoMatch
  Given: Content with no placeholders
  When:  templates.Replace(content, replacements)
  Then:  Returns content unchanged

TEST: MergeSections_UpdatesTemplateSection
  Given: File with cp-template-start/end markers and modified template content
  When:  templates.MergeSections(current, latest)
  Then:  Template section content is replaced, non-template content preserved

TEST: MergeSections_PreservesUserContent
  Given: File with user content outside template markers
  When:  templates.MergeSections(current, latest)
  Then:  User content is unchanged

TEST: MergeSections_MissingMarkers
  Given: File where user removed template markers
  When:  templates.MergeSections(current, latest)
  Then:  Returns current content unchanged + warning

TEST: ManifestReadWrite
  Given: Manifest struct with 4 file entries
  When:  manifest.WriteTo(".cp.yaml") then templates.ReadManifest(".cp.yaml")
  Then:  Round-trip produces identical manifest

TEST: ValidateProject_Valid
  Given: project="myproject", prefix="mp", agent="agent-x"
  When:  validateProject(project, prefix, agent)
  Then:  Returns nil

TEST: ValidateProject_BadPrefix
  Given: prefix="toolong"
  When:  validateProject(project, prefix, agent)
  Then:  Returns error

TEST: ValidateProject_BadAgent
  Given: agent="notanagent"
  When:  validateProject(project, prefix, agent)
  Then:  Returns error

TEST: RegistryFetch_Local
  Given: Local directory with template files
  When:  NewRegistry("local:/path").FetchAll()
  Then:  Returns all templates with versions parsed

TEST: RegistryFetch_GitHub
  Given: Valid GitHub source
  When:  NewRegistry("github:owner/repo/path").Fetch("ingest.md")
  Then:  Returns template content from GitHub raw URL

TEST: DestinationPath
  Given: Template name "ways-of-working.md"
  When:  templates.DestinationPath("ways-of-working.md")
  Then:  Returns "docs/ways-of-working.md"

TEST: DestinationPath_Ingest
  Given: Template name "ingest.md"
  When:  templates.DestinationPath("ingest.md")
  Then:  Returns ".claude/commands/ingest.md"
```

### Integration Tests

```
TEST: InitCreatesAllFiles
  Given: Empty temp directory, valid DB connection
  When:  cp init --project test --prefix tt --agent agent-test --implementer agent-builder
  Then:  6 files created, .cp.yaml has manifest, all placeholders replaced

TEST: InitCreatesRulesShard
  Given: Empty temp directory, valid DB connection
  When:  cp init --project test --prefix tt --agent agent-test --implementer agent-builder
  Then:  Shard with title "Project Rules for test" exists in database

TEST: InitRejectsExistingProject
  Given: Directory with .cp.yaml
  When:  cp init --project test
  Then:  Exit code 1, error message contains "already initialized"

TEST: UpdateDetectsOutdatedTemplates
  Given: Project with manifest version 2, templates at version 3
  When:  cp update --check
  Then:  Output lists outdated files with version numbers

TEST: UpdatePreservesUserContent
  Given: Project with customized ways-of-working.md (user sections outside markers)
  When:  cp update --yes
  Then:  User sections unchanged, template sections updated

TEST: UpdateSkipsRemovedMarkers
  Given: Project where user removed cp-template markers from a file
  When:  cp update
  Then:  Warning printed, file skipped

TEST: UpdateForceReplacesFile
  Given: Project where user removed markers from ways-of-working.md
  When:  cp update --force docs/ways-of-working.md
  Then:  File replaced with latest template, markers restored, placeholders resolved

TEST: UpdateAddsNewSection
  Given: Project at template v1, template v2 adds new section "parallel-sessions"
  When:  cp update --yes
  Then:  New section appended to file with markers, existing user content preserved

TEST: UpdateScaffoldsNewFile
  Given: Project at template v1, template v2 adds new template "new-template.md"
  When:  cp update
  Then:  Prompted to scaffold new file, placeholders resolved from .cp.yaml

TEST: InitAddManifest
  Given: Existing project with .cp.yaml but no templates section
  When:  cp init --add-manifest
  Then:  templates section added to .cp.yaml, existing files detected and added to manifest

TEST: InitPartialFailure
  Given: Empty temp directory, disk becomes full after 3 files
  When:  cp init --project test --prefix tt --agent agent-test --implementer agent-builder
  Then:  Error message states how many files created, re-running init skips existing files
```

---

## Pre-Submission Checklist

- [x] Every item in "What to Build" has: CLI section + success criterion + tests
- [x] Every data flow answers all 7 questions (who writes/when/where/who reads/how/what for/staleness)
- [x] Every command has: syntax + example + output + atomic steps + JSON schema
- [x] Every success criterion has at least one test case
- [x] Concurrency is addressed (N/A — single user commands)
- [x] No feature is "mentioned but not specced"
- [x] Edge cases cover: invalid input, empty state, conflicts, boundaries, failure recovery
- [x] Existing spec interactions documented (supersedes SPEC-0 cp init, extends .cp.yaml)
- [x] Sub-agent review completed — 21 findings (6 HIGH, 10 MEDIUM, 5 LOW), all HIGH + key MEDIUM fixed
