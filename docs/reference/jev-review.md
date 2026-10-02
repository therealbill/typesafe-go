---
title: "jev review"
description: "The jev review and jev review init subcommands: flags, the jev-review.json config schema, the four questions sent per unit, the JSON report shape, the Markdown summary, default thresholds, and exit codes."
diataxis: reference
weight: 80
aliases: ["/docs/reference/self-review-tool/"]
---

`jev review` judges whether your tests exercise the behaviors you listed and
whether your implementation contradicts the spec section you named. It sends
those files to Jev and answers with probabilities. It does not find bugs,
review style, check security, or verify that the code is correct, and its
readings drift between runs on the same input.

Binary: `jev` (built at `./bin/jev`). Source: `internal/review` and
`internal/cli/review.go`.

## Synopsis

```
jev review [--units jev-review.json] [--only NAME] [--report jev-review-report.json]
           [--json] [--dry-run] [--min-cover 0.6] [--max-contradict 0.4]
           [--min-thorough 2] [--max-state-bytes 100000] [--unit-timeout 2m]
jev review init [--units jev-review.json]
```

## `jev review`

Verified against the built binary:

```
$ ./bin/jev review --help
Review a codebase against its specification.

For each unit in the config file, jev review sends the named spec section and
the implementation and test files to Jev and asks whether the tests exercise
each listed behavior, whether the implementation contradicts the spec, how
thorough the tests are, and which area is weakest. It reports probabilities
and flags readings past the thresholds.

It does not find bugs, review style, check security, or verify that the code
is correct. Readings drift between runs on the same input.

Start with: jev review init

Usage:
  jev review [flags]
  jev review [command]

Available Commands:
  init        Write a starter jev-review.json

Flags:
      --dry-run                 check the config, spec headings, and files without calling the API
  -h, --help                    help for review
      --json                    print the report JSON on stdout instead of the Markdown summary
      --max-contradict float    flag a unit whose contradiction probability is above this (default 0.4)
      --max-state-bytes int     byte cap for implementation and tests together (default 100000)
      --min-cover float         flag a behavior whose coverage probability is below this (default 0.6)
      --min-thorough float      flag a unit whose thoroughness score is below this (default 2)
      --only string             run a single unit by name
      --report string           where to write the JSON report (default "jev-review-report.json")
      --unit-timeout duration   time allowed for one unit's call (default 2m0s)
      --units string            config file; paths inside it resolve relative to the file (default "jev-review.json")

Global Flags:
      --api-key string     TypeSafe API key (env TYPESAFE_API_KEY)
      --base-url string    API base URL (env TYPESAFE_BASE_URL)
      --log-level string   debug|info|warning|error|off (env TYPESAFE_LOG_LEVEL)
      --max-retries int    retries after the first attempt (default 2)
      --model string       model name (env TYPESAFE_DEFAULT_MODEL; default jev-latest)
      --no-trace           disable tracing even when HONEYCOMB_API_KEY or OTEL_* is set
      --pretty             indent JSON output
      --timeout duration   per-attempt HTTP timeout (default 10s)
      --trace              force OpenTelemetry tracing on

Use "jev review [command] --help" for more information about a command.
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--dry-run` | bool | `false` | check the config, spec headings, and files without calling the API |
| `--json` | bool | `false` | print the report JSON on stdout instead of the Markdown summary |
| `--max-contradict` | float | `0.4` | flag a unit whose contradiction probability is above this |
| `--max-state-bytes` | int | `100000` | byte cap for implementation and tests together |
| `--min-cover` | float | `0.6` | flag a behavior whose coverage probability is below this |
| `--min-thorough` | float | `2` | flag a unit whose thoroughness score is below this |
| `--only` | string | `""` | run a single unit by name |
| `--report` | string | `jev-review-report.json` | where to write the JSON report |
| `--unit-timeout` | duration | `2m0s` | time allowed for one unit's call |
| `--units` | string | `jev-review.json` | config file; paths inside it resolve relative to the file |

The global flags (`--api-key`, `--base-url`, `--model`, `--timeout`,
`--max-retries`, `--log-level`, `--trace`, `--no-trace`, `--pretty`) apply to
`jev review` exactly as documented on the [jev CLI reference](./jev-cli.md).

## `jev review init`

Verified against the built binary:

```
$ ./bin/jev review init --help
Write a starter jev-review.json

Usage:
  jev review init [flags]

Flags:
  -h, --help           help for init
      --units string   path to write (default "jev-review.json")

Global Flags:
      --api-key string     TypeSafe API key (env TYPESAFE_API_KEY)
      --base-url string    API base URL (env TYPESAFE_BASE_URL)
      --log-level string   debug|info|warning|error|off (env TYPESAFE_LOG_LEVEL)
      --max-retries int    retries after the first attempt (default 2)
      --model string       model name (env TYPESAFE_DEFAULT_MODEL; default jev-latest)
      --no-trace           disable tracing even when HONEYCOMB_API_KEY or OTEL_* is set
      --pretty             indent JSON output
      --timeout duration   per-attempt HTTP timeout (default 10s)
      --trace              force OpenTelemetry tracing on
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--units` | string | `jev-review.json` | path to write |

The global flags listed above apply to `jev review init` the same as to
`jev review`; see the [jev CLI reference](./jev-cli.md).

### Behavior

Source: `internal/cli/review.go`, `newReviewInitCmd`. `jev review init` writes
`Template()` (the JSON shown under [Config schema](#config-schema-jev-reviewjson)
below) to the `--units` path, with file mode `0644`. If the file already
exists, it refuses with a usage error (exit code 1) and does not overwrite
it. On success, it prints `{"wrote":"<path>"}` to stdout.

Verified against the built binary:

```
$ ./bin/jev review init
{"wrote":"jev-review.json"}
$ ./bin/jev review init
{"error":{"kind":"usage","message":"jev-review.json already exists; remove it or pass --units"}}
jev: jev-review.json already exists; remove it or pass --units
$ echo $?
1
```

## `jev review --dry-run`

Source: `internal/cli/review.go`, `runDryRun`, and `internal/review/review.go`, `runUnit`.

With `--dry-run`, `jev review` loads the config, then for each unit reads its
spec document, extracts the section named by `spec_heading`, reads every
`implementation` and `tests` file, and builds both bundles. It stops there. No
client is built, no request is sent, and no API key is needed. `--report` is
ignored and no report file is written. `--only` and `--json` work as they do
without `--dry-run`. The global flags are accepted and unused.

The Markdown summary is a different table from the one a full run prints:

| Column | Content |
|---|---|
| Unit | The unit's `name`. |
| Spec bytes | Length of the extracted section. |
| Implementation bytes | Size of the implementation bundle before truncation. |
| Tests bytes | Size of the tests bundle before truncation. |
| Truncated | `yes` when either bundle was cut to fit half of `--max-state-bytes`. |
| Status | `ok`, or `error` when the spec document, heading, or a file could not be read. |

A `budget:` line above the table states the per-bundle limit. Below the table,
each errored unit gets a `## <unit>` section with `error: <message>`, and each
truncated unit gets one with `note: sources were truncated to fit the state
budget`.

Verified against the built binary, on a two-unit scratch project:

```
$ jev review --dry-run
review: checked 2 units
# jev review --dry-run

budget: 50000 bytes each for implementation and tests

| Unit | Spec bytes | Implementation bytes | Tests bytes | Truncated | Status |
|---|---|---|---|---|---|
| retry | 186 | 269 | 282 | no | ok |
| validation | 132 | 227 | 176 | no | ok |

exit 0
```

The same config with the second unit's heading changed to `## Validating`,
which does not appear in the spec:

```
$ jev review --dry-run
review: checked 2 units
# jev review --dry-run

budget: 50000 bytes each for implementation and tests

| Unit | Spec bytes | Implementation bytes | Tests bytes | Truncated | Status |
|---|---|---|---|---|---|
| retry | 186 | 269 | 282 | no | ok |
| validation | 0 | 0 | 0 | no | error |

## validation

error: heading "## Validating" not found in spec

review: 1 of 2 units failing
exit 8
```

Exit codes with `--dry-run`: 0 when every unit resolved, 8 when any unit
errored (`kind` `flagged`, with `review: k of n units failing` on stderr), 1
when the config is missing or invalid or `--only` matched nothing. Codes 3 to
7 cannot occur.

## Config schema (`jev-review.json`)

Source: `internal/review/review.go`.

```go
type Config struct {
    Description string `json:"description,omitempty"`
    Spec        string `json:"spec"`
    Units       []Unit `json:"units"`
}
type Unit struct {
    Name           string   `json:"name"`
    Spec           string   `json:"spec,omitempty"`
    SpecHeading    string   `json:"spec_heading"`
    Implementation []string `json:"implementation"`
    Tests          []string `json:"tests"`
    Behaviors      []string `json:"behaviors"`
    Accepted       []string `json:"accepted,omitempty"`
    Notes          []string `json:"notes,omitempty"`
}
```

### `Config` fields

| Field | Type | Required | Description |
|---|---|---|---|
| `description` | string | No | Free-text description of the config. Omitted from the wire JSON when empty. |
| `spec` | string | Conditional | Path to the spec file used by any unit that does not set its own `spec`. Required unless every unit sets its own `spec`. |
| `units` | array of `Unit` | Yes | The units to run. Must have at least one entry. |

### `Unit` fields

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | Yes | Identifies the unit in the report and in `--only`. Required, non-empty, and must be unique across the config; a duplicate name is a load error. |
| `spec` | string | No | Path to a spec file that overrides the top-level `spec` for this unit only. Omitted or empty falls back to the top-level `spec`. |
| `spec_heading` | string | Yes | A Markdown heading line (for example `"### Questions"`), matched exactly against a heading in the spec file. The section it names runs from that heading through the line before the next heading of the same or higher level, ignoring headings that appear inside fenced code blocks. See [How to write a spec jev review can use](../how-to/write-a-spec-jev-review-can-use.md). |
| `implementation` | array of string | Yes | File paths making up the implementation bundle. At least one entry is required. |
| `tests` | array of string | No | File paths making up the tests bundle. Omitted or empty is valid. |
| `behaviors` | array of string | Yes | Behavior-description strings; each becomes one `covers_NN` question, in order. At least one entry is required. |
| `accepted` | array of string | No | Acknowledged flag ids and behavior text; see below. Omitted or empty is valid. |
| `notes` | array of string | No | Free-text entries, copied into the report's `notes` field. Omitted or empty is valid. |

Every unit resolves a spec file: its own `spec` when set, otherwise the
top-level `spec`. The top-level `spec` is required only for a unit that does
not set its own; a config with such a unit and no top-level `spec` is a
`Load` error naming the unit, `unit "<name>": "spec" is required at the top
level or on every unit`. A unit whose resolved spec file cannot be read is a
unit error, recorded in the report, not a `Load` error. Units that share a
spec path read it once; the file is not re-read per unit.

All paths, including the top-level `spec`, each unit's own `spec` when set,
and each unit's `implementation` and `tests` entries, resolve relative to
the directory containing the config file; an absolute path is used as-is.

Each `accepted` entry must be either the exact text of one of that unit's
`behaviors`, or one of the fixed ids `contradicts_spec`, `thoroughness`,
`weakest_area`. An entry matching the pattern `covers_\d+` (for example
`covers_00`) is rejected at load time with an explicit error naming the
rule: it must name the behavior text itself, not its position, since
renumbering `behaviors` would otherwise silently reassign the acceptance to
a different claim. An entry matching none of the valid forms is also
rejected at load time.

A missing config file, invalid JSON, or a validation failure described above
is a `Load` error, surfaced by `jev review` as a usage error (exit code 1).

### Schema example

The `init` template, written by `jev review init`:

```json
{
  "description": "Units for jev review. Paths are relative to this file. Each unit names one spec section, the files that implement it, the files that test it, and the behaviors the section requires, one concrete claim per line. A unit may set its own \"spec\" to override the top-level one.",
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

## Questions sent to Jev

Source: `internal/review/review.go`, `buildQuestions`. For each unit, four
kinds of questions are sent:

| id | type | purpose |
|---|---|---|
| `covers_NN` (one per behavior, zero-padded two digits, e.g. `covers_00`, `covers_01`) | `noul` | Whether the tests exercise that behavior. |
| `contradicts_spec` | `noul` | Whether the implementation contradicts a spec requirement. |
| `thoroughness` | `score` (4 levels) | How thoroughly the tests cover the spec's required behaviors. |
| `weakest_area` | `choice` | Which area of the implementation is least covered by the tests. |

The instructions text of each is reproduced verbatim below.

### `covers_NN`

Type `noul`. `instructions` is an object:

- `question`: "Do the tests in `tests` exercise the behavior below, which `spec` requires of the code in `implementation`?"
- `behavior`: the unit's behavior text, inserted per-question.
- `guidance`: "Answer yes only if at least one test would fail if this behavior were removed or broken. A test that merely mentions the feature without asserting on it does not count."

### `contradicts_spec`

Type `noul`. `instructions`: "Does the code in `implementation` contradict a requirement stated in `spec`? Count only actual conflicts such as a different default value, a different error type, a different wire field name, or opposite behavior. Ignore omissions and ignore anything the spec leaves unspecified."

Criteria:

- `true`: "At least one statement in `spec` is contradicted by `implementation`."
- `false`: "`implementation` is consistent with every statement in `spec` that it addresses."

### `thoroughness`

Type `score` (4 levels). `instructions`: "How thoroughly do the tests in `tests` cover the behaviors that `spec` requires of `implementation`?"

Levels, in order (level 0 to level 3):

0. "No test exercises this section's behavior."
1. "Only the success path is tested; error cases named in `spec` are not."
2. "The success path and the error cases named in `spec` are tested."
3. "Success, error cases, and the boundary values, cancellation, or concurrency conditions named in `spec` are all tested."

### `weakest_area`

Type `choice`. `instructions`: "Which area of `implementation` is least covered by `tests` relative to what `spec` requires? Choose none if coverage is even."

Labels, verbatim:

- `validation`: "Input checks and validation error paths."
- `error_mapping`: "Turning failures from dependencies into the error types the code exposes."
- `retry`: "Backoff, retry limits, budgets, and cancellation."
- `decoding`: "Turning input or response data into typed values."
- `encoding`: "Turning typed values into output or wire formats."
- `logging`: "What is and is not logged."
- `none`: "No area stands out."

### Bundling and truncation

`implementation` and `tests` are each built by concatenating the named files
with a `// file: <path>` header line before each, then independently
truncated to half of `--max-state-bytes` (default `100000`, so `50000` bytes
each), appending the marker `// ...(truncated)` when cut. The report's
`truncated` field is `true` if either half was cut.

## Report JSON schema

Source: `internal/review/review.go`.

```go
type BehaviorResult struct {
    ID       string  `json:"id"`
    Behavior string  `json:"behavior"`
    Covered  float64 `json:"covered"`
}
type UnitReport struct {
    Name        string `json:"name"`
    RequestID   string `json:"request_id,omitempty"`
    Model       string `json:"model,omitempty"`
    InputTokens int    `json:"input_tokens,omitempty"`
    Truncated   bool   `json:"truncated"`
    // SpecBytes is the length of the extracted spec section.
    // ImplementationBytes and TestsBytes are the bundle sizes before
    // truncation, so a report shows how close a unit sits to the budget.
    SpecBytes           int              `json:"spec_bytes,omitempty"`
    ImplementationBytes int              `json:"implementation_bytes,omitempty"`
    TestsBytes          int              `json:"tests_bytes,omitempty"`
    Behaviors           []BehaviorResult `json:"behaviors"`
    Contradicts         float64          `json:"contradicts_spec"`
    Thoroughness        float64          `json:"thoroughness"`
    ThoroughConf        float64          `json:"thoroughness_confidence"`
    Weakest             string           `json:"weakest_area"`
    WeakestConf         float64          `json:"weakest_confidence"`
    Flags               []string         `json:"flags"`
    Notes               []string         `json:"notes,omitempty"`
    StaleAcceptances    []string         `json:"stale_acceptances,omitempty"`
    Failing             bool             `json:"failing"`
    Error               string           `json:"error,omitempty"`
}
```

| Field | Type | Presence | Description |
|---|---|---|---|
| `name` | string | Always | The unit's `name` from the config. |
| `request_id` | string | omitempty | The request id from the Jev call for this unit. |
| `model` | string | omitempty | The model used for this unit's call. |
| `input_tokens` | int | omitempty | Input token count for this unit's call. |
| `truncated` | bool | Always | Whether `implementation` or `tests` was cut to fit `--max-state-bytes`. |
| `spec_bytes` | int | omitempty | Length in bytes of the extracted spec section. |
| `implementation_bytes` | int | omitempty | Size in bytes of the implementation bundle before truncation. |
| `tests_bytes` | int | omitempty | Size in bytes of the tests bundle before truncation; absent when `tests` is empty. |
| `behaviors` | array of `{id, behavior, covered}` | Always | One entry per `behaviors` entry in the config, `id` being its `covers_NN` question id and `covered` its answered probability. |
| `contradicts_spec` | float64 | Always | The `contradicts_spec` answer's probability. |
| `thoroughness` | float64 | Always | The `thoroughness` answer's score. |
| `thoroughness_confidence` | float64 | Always | The `thoroughness` answer's confidence. |
| `weakest_area` | string | Always | The `weakest_area` answer's chosen label. |
| `weakest_confidence` | float64 | Always | The `weakest_area` answer's confidence. |
| `flags` | array of string | Always (may be empty) | One message per threshold crossed. A message matching an accepted entry has `" (accepted)"` appended and does not make the unit failing. |
| `notes` | array of string | omitempty | Copied from the unit's `notes` in the config. |
| `stale_acceptances` | array of string | omitempty | Entries from the unit's `accepted` list whose corresponding flag did not fire on this run, sorted. |
| `failing` | bool | Always | `true` when at least one unaccepted flag fired, or the unit errored (missing or unreadable spec document, missing implementation or test file, missing spec heading, API/transport failure, or the unit call exceeding `--unit-timeout`). |
| `error` | string | omitempty | Set instead of most other fields when the unit could not complete. |

In a dry run, `request_id`, `model`, `input_tokens`, `behaviors`, and the
answer fields are absent or zero and `flags` is `null`; the three byte fields,
`truncated`, `notes`, `failing`, and `error` are set as in a full run.

A unit error, of any of the kinds listed above, ends only that unit. `Run`
records it on that unit's `UnitReport` and continues with the remaining
units.

The report file is written by `jev review` to `--report` (default
`jev-review-report.json`) as a JSON array of `UnitReport`, one entry per
unit that ran (respecting `--only`).

Real verified example, from a live run (`--only retry --json`):

```json
[
  {
    "name": "retry",
    "request_id": "req_01a0ea68e2037e40b3016976d67385eb",
    "model": "jev-1.13.0",
    "input_tokens": 1596,
    "truncated": false,
    "behaviors": [
      {"id": "covers_00", "behavior": "A failed publish is retried up to three times", "covered": 0.94},
      {"id": "covers_01", "behavior": "The delay between retries is a fixed 100ms and does not grow", "covered": 0.97},
      {"id": "covers_02", "behavior": "The last error is returned after all retries are exhausted", "covered": 0.97}
    ],
    "contradicts_spec": 0.28,
    "thoroughness": 2.1,
    "thoroughness_confidence": 0.8,
    "weakest_area": "none",
    "weakest_confidence": 0.44,
    "flags": null,
    "failing": false
  }
]
```

## Markdown summary

Printed to stdout when `--json` is not passed (source: `(Report).Markdown`).
Starts with `# jev review`, then a table:

| Column | Content |
|---|---|
| Unit | The unit's `name`. |
| Behaviors covered | `<covered>/<total>`, where `covered` counts behaviors at or above `--min-cover`. |
| Contradicts | The `contradicts_spec` value. |
| Thoroughness | The `thoroughness` value. |
| Weakest | The `weakest_area` value. |
| Flags | The flag count, with `" FAIL"` appended if the unit is failing, or the literal string `error` (overriding the count) if the unit errored. |

Below the table, one `## <unit name>` section is printed for each unit that
errored, has a flag, or has a stale acceptance; a clean unit with none of
those is omitted from this part. An errored unit prints `error: <message>`.
Otherwise, in order: a `note: sources were truncated to fit the state
budget` line if truncated, one `note: <text>` line per entry in `notes`, one
`- <message>` bullet per flag (sorted), then one
`- acceptance did not fire: <name>` bullet per stale acceptance.

Real verified example transcript:

```
$ jev review
review: ran 2 units
# jev review

| Unit | Behaviors covered | Contradicts | Thoroughness | Weakest | Flags |
|---|---|---|---|---|---|
| retry | 2/3 | 0.28 | 1.67 (0.57) | retry | 2 FAIL |
| validation | 1/2 | 0.06 | 1.08 (0.63) | validation | 2 FAIL |

## retry

- covers_01: coverage 0.09 < 0.60 for "The delay between retries is a fixed 100ms and does not grow"
- thoroughness: 1.67 < 2.00

## validation

- covers_01: coverage 0.05 < 0.60 for "A widget name longer than 64 bytes is rejected"
- thoroughness: 1.08 < 2.00

review: 2 of 2 units failing
$ echo $?
8
```

stderr carries `review: ran N units` before the report is written, and (on
failure) `review: <k> of <n> units failing` before the process exits 8.

## Default thresholds

| Flag | Default | Meaning |
|---|---|---|
| `--min-cover` | 0.6 | flags a behavior whose `covers_NN` probability is below this |
| `--max-contradict` | 0.4 | flags a unit whose `contradicts_spec` probability is above this |
| `--min-thorough` | 2.0 | flags a unit whose `thoroughness` score is below this |
| `--max-state-bytes` | 100000 | byte cap for `implementation` and `tests` together (each gets half) |
| `--unit-timeout` | 2m0s | time allowed for one unit's Jev call |

## Exit codes

| Exit | Meaning |
|---|---|
| 0 | every unit ran and nothing unaccepted was flagged |
| 1 | config missing or invalid, report not writable, zero units matched `--only`, or bad flags (`kind` `usage`) |
| 3–7 | a unit's API call failed with an unrecoverable error, classified the same as `jev ask` (see the [errors and exit codes reference](./errors-and-exit-codes.md)); the report still lists that unit with its `error` field set |
| 8 | at least one unit is failing: an unaccepted flag fired, a file or spec heading was missing, or the unit timed out (`kind` `flagged`); the report says which |

With `--dry-run`, only 0, 1, and 8 occur; see
[`jev review --dry-run`](#jev-review---dry-run).

The process exit code is the classifier's code for the first unit call error
if any occurred, otherwise 8 if any unit is failing for any other reason,
otherwise 0. On exit 8, stdout carries the report itself (Markdown or, with
`--json`, the report JSON) rather than an error envelope; stderr carries the
summary line. See the [errors and exit codes reference](./errors-and-exit-codes.md)
for the full classifier table and error JSON shape.

## See also

- [How to review your codebase with jev](../how-to/review-your-codebase-with-jev.md)
- [What jev review measures](../explanation/what-jev-review-measures.md)
