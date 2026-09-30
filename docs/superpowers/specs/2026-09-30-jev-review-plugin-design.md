# jev-review Plugin and `jev review --dry-run`: Design

Date: 2026-09-30

## Goal

Ship a Claude Code plugin that writes the two files `jev review` needs, the
Markdown spec and `jev-review.json`, from a codebase, and turn this
repository into a plugin marketplace so users can install it with two
commands. Add a `--dry-run` mode to `jev review` so the plugin, and anyone
else, can check a config without an API key or a paid call.

Out of scope: changing the questions `jev review` asks, changing thresholds,
running the paid review from inside the plugin, and CI integration of
`jev review` itself.

## What the plugin does

Given a repository, `/jev-review:setup`:

1. Finds or writes a Markdown spec with one heading per unit-sized concern
   and declarative requirement sentences under each.
2. Pairs each spec section with the files that implement it and the files
   that test it, with evidence for each pairing.
3. Stops once, to show the proposed unit map and get approval.
4. Writes `jev-review.json` with five to fifteen concrete behaviors per unit.
5. Runs `jev review --dry-run` until it passes.
6. Prints what it wrote and the `jev review` command to run next.

The plugin never runs the paid review. That is the user's call, made after
the hand-off.

## Layout

```
.claude-plugin/
  marketplace.json                     marketplace "typesafe-go", one plugin entry
plugins/jev-review/
  .claude-plugin/plugin.json           name "jev-review", version 0.1.0
  README.md                            install, requirements, usage, what it writes
  commands/setup.md                    /jev-review:setup
  agents/repo-mapper.md                read-only codebase survey
  skills/setting-up-jev-review/
    SKILL.md
    references/writing-the-spec.md
    references/pairing-files.md
    references/writing-behaviors.md
    references/config-schema.md
    references/language-conventions.md
tools/checkplugin.sh                   zsh; see "Repository integration"
```

The plugin lives in a subdirectory so the Go module and the plugin stay
separate and a second plugin can be added later. The marketplace manifest
is at the repository root because that is where `/plugin marketplace add`
looks.

Install:

```
/plugin marketplace add therealbill/typesafe-go
/plugin install jev-review@typesafe-go
```

### `marketplace.json`

```json
{
  "name": "typesafe-go",
  "description": "Claude Code plugins for the typesafe-go SDK and the jev command-line tool.",
  "owner": { "name": "Bill" },
  "plugins": [
    {
      "name": "jev-review",
      "source": "./plugins/jev-review",
      "description": "Write the spec and jev-review.json that jev review needs, from your codebase.",
      "version": "0.1.0",
      "keywords": ["jev", "typesafe", "review", "spec", "tests"]
    }
  ]
}
```

### `plugin.json`

`name` `jev-review`, `version` `0.1.0`, `description` as above, `author`
`{ "name": "Bill" }`, `homepage` the published docs how-to, `repository`
`https://github.com/therealbill/typesafe-go`, `license` `BSD-3-Clause`,
`keywords` as above. No custom component paths; the default directories are
used.

The plugin version is independent of `jev` release tags. It starts at
`0.1.0` and moves on its own semver.

## Components

### Skill: `setting-up-jev-review`

Fully qualified name `jev-review:setting-up-jev-review`. Auto-activates when
a request mentions jev review, `jev-review.json`, `spec_heading`, writing
behaviors for a review, writing or fixing a spec for `jev review`, or
reviewing a codebase with Jev.

`SKILL.md` is under 120 lines and holds:

- The scope statement from the docs, verbatim: what `jev review` judges and
  what it does not, and that readings drift.
- The procedure, one line per step, matching the command's workflow, so the
  skill can drive the same work when a user asks in their own words instead
  of running the command.
- A pointer to each reference file and when to open it.
- The triage rule for a flag after a real run: add a test, fix the spec or
  the code, or record an acceptance with a note that names the behavior's
  exact text.

The references are written for an agent: terse, imperative, no narrative.
They are derived from the docs pages, not copied, and each ends with the
URL of the docs page it condenses.

| File | Contents |
|---|---|
| `writing-the-spec.md` | The section-extraction rule: exact heading match including `#` characters, section runs to the next heading of the same or higher level, headings inside fences are ignored. One heading per unit-sized concern, consistent levels. Declarative sentences that state a default, a limit, an error type, a field name, an ordering, or a failure behavior. Requirements in prose, code blocks as illustration only. Size a section to what one unit's files implement. The three routes: from READMEs, from code and tests, from a design spec. |
| `pairing-files.md` | List the symbols a section names. Find the non-test files that define them. Find the test files that reference them. A similar file name is not evidence. The exporter for a feature is not the feature. A unit reading 0 of N with a real test file present is a mis-pairing. The 100 KB budget, split in half between implementation and tests. Use `--only` while fixing one unit. |
| `writing-behaviors.md` | One behavior per requirement sentence, five to fifteen per unit. Each names one checkable outcome a test could assert. Good and bad examples from the docs. Under five means the section is thin: add requirement sentences or merge units. Over fifteen means split the section into sub-headings and sub-units. Acceptances name the behavior's exact text, never `covers_NN`. |
| `config-schema.md` | Every field of `Config` and `Unit` with type, requiredness, and meaning. Path resolution relative to the config file. The per-unit `spec` override. The `init` template. The `--dry-run` check. |
| `language-conventions.md` | A table per language: where tests live, how to find a symbol's defining file, how to find tests that reference it. Go in depth: package layout, `_test.go` siblings, `go list -f '{{.GoFiles}} {{.TestGoFiles}}' ./...`, external test packages. Python: `test_*.py`, `tests/`, pytest imports. TypeScript: `*.test.ts`, `*.spec.ts`, `__tests__/`. Rust: `#[cfg(test)]` modules in the same file, so `tests` may name the implementation file itself, and `tests/*.rs` integration tests. |

### Command: `/jev-review:setup [spec-path] [config-path]`

File `commands/setup.md`. Frontmatter: `name: setup`, `description`, and
`argument-hint: "[spec-path] [config-path]"`. Both arguments are optional.
`config-path` defaults to `jev-review.json` in the current directory, which
is where `jev review` looks by default.

Steps, in order:

1. **Preconditions.** If `jev` is not on `PATH`, print the two install
   routes from the README (`go install` and the release download) and stop.
   If `jev review --help` does not mention `--dry-run`, say the installed
   `jev` predates this feature, name the first release that ships it, and
   stop. Record
   whether `TYPESAFE_API_KEY` is set; this only changes the hand-off text.
   If `config-path` exists and was not passed explicitly, stop and say so;
   an explicit path may be overwritten.
2. **Inventory.** Resolve the spec. The `spec-path` argument wins. Otherwise
   collect candidates matching `docs/**/*design*.md`, `docs/spec.md`,
   `SPEC.md`, and `docs/**/*spec*.md`, excluding `docs/superpowers/plans/`.
   One candidate is the spec. Several are passed to the mapper, which picks
   the one whose headings best match the code and lists the rest; the
   checkpoint table names the chosen file so the user can redirect. None
   means new-spec mode. *New spec*: write one from code, tests, and READMEs,
   at `docs/spec.md` when `docs/` exists and `SPEC.md` otherwise. *Existing
   spec*: audit it against the spec rules and apply targeted edits only;
   never reorder, remove, or rewrite existing content.
3. **Map.** Dispatch the `jev-review:repo-mapper` agent with the repository
   root, the spec candidates if any, and the mode. Wait for its proposal.
4. **Checkpoint.** Render the proposal as one table: unit, heading with
   `existing` or `new`, implementation, tests, bytes as
   `implementation/tests` against the 50 KB half-budget, and the evidence
   note. Ask for approval with `AskUserQuestion`: approve as is, or describe
   changes. Apply changes and re-present until approved. This is the only
   stop.
5. **Spec.** For each approved unit, ensure the heading exists at a level
   consistent with its siblings and that the section holds declarative
   requirement sentences. In new-spec mode every sentence is written from
   code; in existing-spec mode only the edits the audit proposed are made.
   Collect every sentence written from code with a file and line reference
   for the hand-off, so the user can confirm it states intent and not just
   current behavior. The spec file itself carries no markers.
6. **Config.** Write `config-path` with `description`, top-level `spec`
   (relative to the config file), and one unit per approved row. Set a
   per-unit `spec` only when a unit's spec file differs from the top-level
   one. Behaviors follow `writing-behaviors.md`. `accepted` and `notes`
   start as empty arrays. Every path is relative to the config file's
   directory.
7. **Dry run.** Run `jev review --dry-run --units <config-path>`. On a
   heading mismatch or missing file, fix the config or the spec and rerun
   until exit 0.
8. **Hand-off.** Print the files written, the sentences written from code,
   the exact `jev review --units <config-path>` command, and a one-line
   note that the paid run sends the listed files to the TypeSafe API. If
   `TYPESAFE_API_KEY` is unset, say so. Offer to triage flags after the user
   runs it.

### Agent: `repo-mapper`

File `agents/repo-mapper.md`. Frontmatter: `name: repo-mapper`,
`description` naming when to use it (proposing jev review units for a
repository, pairing spec sections with implementation and test files),
`tools: Read, Grep, Glob, Bash`, `model: inherit`. The system prompt
restricts Bash to read-only commands: `git ls-files`, `go list`, `wc -c`,
`find`, `grep`. The agent writes nothing.

Input, in the dispatch prompt: repository root, spec candidates or `none`,
mode.

The agent reads `language-conventions.md` and `pairing-files.md` from
`${CLAUDE_PLUGIN_ROOT}/skills/setting-up-jev-review/references/`, detects
the language and test convention, picks the spec when several candidates
were given, and builds the proposal. Its final message is a fenced JSON block followed by a
short prose summary. The JSON shape:

```json
{
  "language": "go",
  "test_convention": "_test.go siblings; external test packages for client_test.go",
  "spec": {
    "path": "docs/design.md",
    "other_candidates": ["docs/old-spec.md"],
    "headings": [{"line": 12, "level": 2, "text": "## Retry policy"}],
    "unmatched_headings": ["## Roadmap"],
    "vague_sentences": [{"line": 40, "text": "The client retries sensibly."}]
  },
  "units": [
    {
      "name": "retry",
      "heading": "## Retry policy",
      "heading_exists": true,
      "concern": "Backoff, jitter, Retry-After, and the retry budget",
      "implementation": ["retry.go", "transport.go"],
      "tests": ["retry_test.go", "client_test.go"],
      "symbols": ["RetryPolicy", "backoff", "parseRetryAfter"],
      "implementation_bytes": 11520,
      "tests_bytes": 34110,
      "evidence": "client_test.go asserts on retry-after-ms and the budget; retry_test.go covers backoff and jitter"
    }
  ]
}
```

`spec` is `null` when no spec exists. `unmatched_headings` lists headings
with no code behind them. `vague_sentences` lists sentences with nothing
checkable in them. Where evidence is weak, `evidence` says so plainly, for
example `no test file references Tracker; exporter_test.go tests the
exporter only`, so the checkpoint table shows it.

## `jev review --dry-run`

A validate-only mode that runs every step of a unit up to, but not
including, the API call.

### Package `internal/review`

- `Options` gains `DryRun bool`.
- `bundle` also returns the concatenated size before truncation.
- `UnitReport` gains three fields, populated in both modes:

  ```go
  SpecBytes           int `json:"spec_bytes,omitempty"`
  ImplementationBytes int `json:"implementation_bytes,omitempty"`
  TestsBytes          int `json:"tests_bytes,omitempty"`
  ```

  `SpecBytes` is the extracted section's length. The other two are the
  bundle sizes before truncation, so a real run also shows how close each
  unit sits to the half-budget. `Truncated` keeps its meaning.
- `runUnit` fills the three fields once the section and bundles exist. When
  `opts.DryRun` is set it returns there, with `Failing` false, and never
  touches the asker. Errors before that point (unreadable spec, missing
  heading, missing file) are recorded as today.
- `Run` accepts a nil asker when `opts.DryRun` is set, and sets a new
  `Report.DryRun bool` field, excluded from JSON, that `Markdown` reads.
- `Markdown` in dry-run mode prints `# jev review --dry-run` and a table:

  | Unit | Spec | Heading | Implementation | Tests | Bytes | Status |
  |---|---|---|---|---|---|---|

  `Bytes` is `implementation/tests`. `Status` is `ok`, `ok (truncated)`, or
  `error`. Below the table, one `## <unit>` section per errored unit with
  `error: <message>`, and one per truncated unit with the existing
  `note: sources were truncated to fit the state budget` line.

### Subcommand

`jev review` gains `--dry-run` (bool, default false): "check the config,
spec headings, and files without calling the API". When set, `runReview`
loads the config, then skips global flag validation, telemetry, and client
construction, calls `review.Run` with a nil asker, prints
`review: checked N units` to stderr, prints the Markdown or, with `--json`,
the report JSON to stdout, and does not write the report file. `--report`
is ignored in this mode and the reference says so. `--only` works as
today.

Exit codes are unchanged in meaning: 0 when every unit resolved, 8 when any
unit errored (`kind` `flagged`, with `review: k of n units failing` on
stderr), 1 for a missing or invalid config.

No API key or network is needed.

### Tests

`internal/review/review_test.go`: a dry run on a clean unit fills the three
byte fields, leaves `Failing` false, and calls no asker (the asker is nil);
a dry run on a missing heading records the error and `Failing` true; a dry
run on files past the half-budget sets `Truncated` and reports the
pre-truncation sizes; a real run through the fake asker also carries the
byte fields.

`internal/cli/cli_test.go` or `review_test.go`: `review --dry-run` with no
`TYPESAFE_API_KEY` in the environment exits 0 and prints the dry-run table;
with a bad heading exits 8; `--dry-run --json` prints an array whose entries
carry `spec_bytes`; no report file is written.

## Documentation

| Page | Change |
|---|---|
| `docs/how-to/build-a-jev-review-config-with-claude-code.md` | New. Prerequisites (Claude Code, `jev` with `--dry-run`, optional key), the two install commands, running `/jev-review:setup`, what the checkpoint shows, what gets written, running the dry run and then `jev review`, and a pointer to the review how-to for triage. Opens with the scope statement like every `jev review` page. |
| `docs/how-to/review-your-codebase-with-jev.md` | New step between pairing files and running: check the config with `--dry-run` before spending a call, with a verified transcript. |
| `docs/reference/jev-review.md` | `--dry-run` in the synopsis and flags table; a `## jev review --dry-run` section with the verified `--help` and a transcript; the three byte fields in the report table; `--report` ignored in dry-run; exit-code note. |
| `docs/reference/jev-cli.md` | The verbatim `jev review --help` block and its prose gain the flag. |
| `docs/reference/makefile-and-repository-layout.md` | `make plugin` in the help list; `.claude-plugin/`, `plugins/`, and `tools/checkplugin.sh` in the layout; the `make plugin` step in the `ci.yml` description. |
| `docs/how-to/_index.md`, `README.md` | Link the new how-to. README gains a short paragraph after the `jev review` line with the two install commands. |
| `plugins/jev-review/README.md` | Install, requirements, usage, what it writes, and that the paid run is the user's to start. |

All prose follows the project's rubric: plain declaratives and
imperatives, active voice, no em dashes, no narrated reasoning, one topic
per paragraph.

## Repository integration

`tools/checkplugin.sh`, zsh, run by a new `make plugin` target and by a new
`make plugin` step in the `lint` job of `ci.yml`. It checks:

- `.claude-plugin/marketplace.json` and every plugin's
  `.claude-plugin/plugin.json` parse (via `python3 -m json.tool`, present on
  macOS and the Ubuntu runner image).
- Every `source` in the marketplace exists and contains
  `.claude-plugin/plugin.json`.
- Every `skills/*/` directory has a `SKILL.md` whose frontmatter has `name`
  and `description`; every `commands/*.md` and `agents/*.md` has
  frontmatter with `description`.
- The json tag names on `Config` and `Unit` in `internal/review/review.go`
  all appear as backticked field names in
  `skills/setting-up-jev-review/references/config-schema.md`, and every
  backticked field name in that file's field table is a real tag. This
  catches schema drift between the binary and the plugin.

`.gitignore` is unchanged. `clean` is unchanged.

## Verification

- `make lint test build` pass.
- `./bin/jev review --dry-run` on this repository's own `jev-review.json`
  exits 0 and prints eight `ok` rows; the same with a heading edited to
  mismatch exits 8 and names the unit.
- `./bin/jev review --dry-run --json | jq .` parses and every entry carries
  `spec_bytes`.
- `env -u TYPESAFE_API_KEY ./bin/jev review --dry-run` exits 0.
- `make plugin` passes; breaking a json tag name in `config-schema.md` makes
  it fail.
- End to end: `/plugin marketplace add /Users/bill/Projects/gojev`, install
  `jev-review@typesafe-go`, run `/jev-review:setup` in a scratch copy of
  the retry demo from the spec how-to. The checkpoint table appears, the
  files are written, the dry run exits 0, and a real `jev review` run
  produces readings.
- A second end-to-end pass on a scratch copy of this repository with its
  `jev-review.json` removed. The proposed unit map is compared with the
  checked-in config; differences are reviewed, not necessarily fixed.
- `make docs` passes.

## Execution

Agent team of up to four, no worktrees:

- `go` (go-architect, opus): `internal/review`, `internal/cli`, tests, and
  the reference and how-to edits for `--dry-run`.
- `plugin` (general-purpose, opus, using the plugin-dev skills):
  `.claude-plugin/`, `plugins/jev-review/`, `tools/checkplugin.sh`,
  Makefile and CI step.
- `docs` (general-purpose, sonnet, driving the Diátaxis how-to writer with
  the prose rubric): the new how-to, index, README, layout reference,
  plugin README.
- Lead: end-to-end verification, `make docs`, final gate, commit.

Every agent stages by explicit path.
