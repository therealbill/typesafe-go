# `jev review` — Design

Date: 2026-09-28

## Goal

Promote the repository's self-review tool to a `jev review` subcommand that
anyone can run against their own codebase, and document it for that audience
with an explicit statement of what it judges and what it does not.

Out of scope: changing the questions the tool asks, changing thresholds, any
form of general code review, and CI integration.

## What the tool judges

For each unit the user defines (a spec section, a set of implementation
files, a set of test files, and a list of behaviors the spec requires),
`jev review` sends the spec section and the file contents to Jev as state and
asks:

- one Noul per behavior: do the tests exercise this behavior?
- one Noul: does the implementation contradict the spec section?
- one Score: how thorough are the tests against the spec section, on a
  four-level rubric?
- one Choice: which area is weakest, from a fixed label set?

It reports probabilities and flags those below or above thresholds. It does
not find bugs, review style, check security, or verify that the code is
correct. Its readings drift between runs on identical input, so it is a
development aid and not a CI gate. Every page about it states this in its
first paragraph.

## Code

### Layout

```
internal/review/            review logic, moved from tools/selfreview/main.go
  review.go                 Config, Unit, Options, Run, evaluate, extractSection,
                            bundle, buildQuestions, markdown
  review_test.go            moved and extended tests
  template.go               the init template
internal/cli/review.go      `jev review` and `jev review init`
jev-review.json             this repository's own config (moved from
                            tools/selfreview/units.json)
```

`tools/selfreview/` is deleted. `Makefile`'s `selfreview` target becomes
`review` and runs `./bin/jev review`.

### Package `internal/review`

```go
type Config struct {
    Description string `json:"description,omitempty"`
    Spec        string `json:"spec"`
    Units       []Unit `json:"units"`
}
type Unit struct {
    Name           string   `json:"name"`
    SpecHeading    string   `json:"spec_heading"`
    Implementation []string `json:"implementation"`
    Tests          []string `json:"tests"`
    Behaviors      []string `json:"behaviors"`
    Accepted       []string `json:"accepted,omitempty"`
    Notes          []string `json:"notes,omitempty"`
}
type Options struct {
    Only          string
    MaxStateBytes int           // default 100000
    MinCover      float64       // default 0.6
    MaxContradict float64       // default 0.4
    MinThorough   float64       // default 2.0
    UnitTimeout   time.Duration // default 2m
}
type Asker interface {
    SystemOne(ctx context.Context, state any, questions typesafe.Questions,
        opts ...typesafe.RequestOption) (*typesafe.SystemOneResponse, error)
}

func Load(path string) (*Config, string /*baseDir*/, error)
func Run(ctx context.Context, cfg *Config, baseDir string, asker Asker,
    opts Options) (Report, error)
func (r Report) Markdown(opts Options) string
func (r Report) Failing() bool
func Template() []byte
```

`Load` reads and validates the config: at least one unit, non-empty names,
unique names, non-empty spec heading, at least one implementation file, at
least one behavior, and `accepted` entries that are either a non-coverage
question id (`contradicts_spec`, `thoroughness`, `weakest_area`) or a
behavior string verbatim; a `covers_NN` id is rejected with a message
explaining the rule. All file paths in the config, including `spec`, resolve
relative to the config file's directory. `Run` runs units sequentially with a
per-unit timeout, never reads the environment, and returns a `Report` (the
existing `[]unitReport` shape, exported as `UnitReport`) with the same fields
as today plus `Notes` and `AcceptanceDidNotFire []string`. Errors reading a
file or a spec section are unit errors in the report, not `Run` errors; `Run`
returns an error only when zero units matched `Only`.

The `*typesafe.Client` satisfies `Asker`, and tests use a fake.

### Subcommand

```
jev review [--units jev-review.json] [--only NAME] [--report jev-review-report.json]
           [--json] [--min-cover 0.6] [--max-contradict 0.4] [--min-thorough 2]
           [--max-state-bytes 100000] [--unit-timeout 2m]
jev review init [--units jev-review.json]
```

`review` builds the client with the root flags (so `--api-key`, `--model`,
`--timeout`, `--max-retries`, `--log-level`, tracing flags all apply), calls
`Load` then `Run`, writes the report file, prints the Markdown to stdout (or
the report JSON with `--json`), prints `review: ran N units` to stderr, and
exits:

| Exit | Meaning |
|---|---|
| 0 | every unit ran and nothing unaccepted was flagged |
| 1 | config missing or invalid, report not writable, zero units matched, or bad flags (`kind` `usage`) |
| 3–7 | a unit's API call failed and the failure was not recoverable, classified as for `ask` (the report still lists the unit with its error) |
| 8 | at least one unaccepted flag fired (`kind` `flagged`) |

A unit whose call fails with an API or transport error is recorded in the
report with `error` set and counts as failing; the process exit code is the
classifier's code for the first such error if any occurred, otherwise 8 if
flagged, otherwise 0. On any non-zero exit the stderr line and, for codes 1
and 3–7, the stdout JSON envelope follow the existing CLI rules; for exit 8
stdout carries the report (Markdown or JSON) and stderr the summary line.

`init` writes `Template()` to the `--units` path, refuses to overwrite an
existing file (exit 1), and prints the path written.

The template:

```json
{
  "description": "Units for jev review. Paths are relative to this file.",
  "spec": "docs/design.md",
  "units": [
    {
      "name": "example",
      "spec_heading": "## Retry policy",
      "implementation": ["retry.go"],
      "tests": ["retry_test.go"],
      "behaviors": [
        "A 429 response is retried after the Retry-After delay",
        "Retries stop after MaxRetries and the last error is returned"
      ],
      "accepted": [],
      "notes": []
    }
  ]
}
```

### Repository's own config

`tools/selfreview/units.json` moves to `jev-review.json` at the repository
root with paths unchanged (they are already root-relative) and gains the
`description` field. `make review` runs `./bin/jev review`. The plan's Task
26 semantics (run, triage, accept with notes) are unchanged.

## Documentation

Three pages are rewritten for a reader reviewing their own codebase, with
this repository as the worked example. Each opens with the scope statement
from "What the tool judges" above, in plain prose. Old paths become Hugo
aliases in the new pages' front matter.

| Old page | New page | Kind |
|---|---|---|
| `docs/how-to/run-the-self-review.md` | `docs/how-to/review-your-codebase-with-jev.md` | how-to: install `jev`, `jev review init`, choose spec headings, write behaviors (one concrete testable claim each, five to fifteen per unit), run, read the table, add a test or record an acceptance with a note, tune thresholds on your own data |
| `docs/reference/self-review-tool.md` | `docs/reference/jev-review.md` | reference: subcommands, flags, config schema, report schema, exit codes, thresholds, the four question texts verbatim from `internal/review` |
| `docs/explanation/what-the-self-review-measures.md` | `docs/explanation/what-jev-review-measures.md` | explanation: what each question type measures, drift, acceptances, unit boundaries, why it is not a CI gate |

Also updated: `docs/reference/jev-cli.md` (new subcommand and exit code 8),
`docs/reference/errors-and-exit-codes.md` (exit 8, kind `flagged`),
`docs/reference/makefile-and-repository-layout.md` (`review` target, no
`tools/`), `docs/how-to/cut-a-release.md` if it mentions the tool,
`docs/explanation/the-agent-facing-cli-contract.md` (the reviewer now calls
the library directly rather than consuming the CLI as a subprocess), the
three section `_index.md` lists, and the README's how-to line. The design
spec for the SDK (`2026-09-23`) gets a one-line note pointing here.

## Verification

- `go test -race ./...`, lint, `make build`.
- `./bin/jev review init --units /tmp/x/jev-review.json` writes the template
  and a second run exits 1.
- `make review` on this repository exits 0 with the same accepted flags as
  before the move (or reports drift, which is triaged as in Task 26).
- `./bin/jev review --units jev-review.json --only questions --json | jq .`
  parses.
- A scratch repository with one unit exercises the how-to end to end.
- `make docs` passes; the three old URLs redirect on the built site.

## Execution

Agent team: `cli` (go-architect, opus) for `internal/review`,
`internal/cli/review.go`, Makefile, config move, tool deletion; `docs`
(general-purpose, sonnet, driving the Diátaxis writers with the prose
rubric from the earlier feedback) for the pages. Lead runs the final gate,
push, and deploy watch.
