---
title: "Self-Review Tool"
description: "The tools/selfreview command: flags, the units.json schema, default thresholds, the JSON report shape, exit codes, and the printed Markdown summary."
diataxis: reference
weight: 80
---

# Self-Review Tool

`tools/selfreview` is a Go `main` package (run as `go run ./tools/selfreview`
or built separately) that asks Jev, through the `jev` CLI, whether each
"unit" of this repository's tests covers the behaviors its spec section
requires and whether its implementation contradicts the spec. Source:
`tools/selfreview/main.go`.

## Flags

Captured by running `go run ./tools/selfreview -h`:

```
Usage of selfreview:
  -jev string
    	path to the jev binary (default "./bin/jev")
  -max-contradict float
    	flag a unit whose contradiction probability is above this (default 0.4)
  -max-state-bytes int
    	byte cap for implementation and tests together (default 100000)
  -min-cover float
    	flag a behavior whose coverage probability is below this (default 0.6)
  -min-thorough float
    	flag a unit whose thoroughness score is below this (default 2)
  -only string
    	run a single unit by name
  -report string
    	where to write the JSON report (default "selfreview-report.json")
  -unit-timeout duration
    	time allowed for one jev ask call (default 2m0s)
  -units string
    	units file (default "tools/selfreview/units.json")
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `-jev` | string | `./bin/jev` | Path to the `jev` binary invoked for each unit's `ask` call. |
| `-max-contradict` | float | `0.4` | Flag a unit whose contradiction probability is above this. |
| `-max-state-bytes` | int | `100000` | Byte cap for implementation and tests together. |
| `-min-cover` | float | `0.6` | Flag a behavior whose coverage probability is below this. |
| `-min-thorough` | float | `2` | Flag a unit whose thoroughness score is below this. |
| `-only` | string | `""` | Run a single unit by name. |
| `-report` | string | `selfreview-report.json` | Where to write the JSON report. |
| `-unit-timeout` | duration | `2m0s` | Time allowed for one `jev ask` call. |
| `-units` | string | `tools/selfreview/units.json` | Units file. |

## `units.json` schema

```go
type config struct {
    Spec  string `json:"spec"`
    Units []unit `json:"units"`
}

type unit struct {
    Name           string   `json:"name"`
    SpecHeading    string   `json:"spec_heading"`
    Implementation []string `json:"implementation"`
    Tests          []string `json:"tests"`
    Behaviors      []string `json:"behaviors"`
    Accepted       []string `json:"accepted,omitempty"`
    Notes          []string `json:"notes,omitempty"`
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `spec` (top-level) | string | Yes | Path to the spec file the run reads section text from, via `extractSection`. |
| `units` (top-level) | array of unit | Yes | The units to run. |
| `name` | string | Yes | Identifies the unit in the report and in `-only`. |
| `spec_heading` | string | Yes | A Markdown heading line (for example `"### Questions"`), matched exactly, whose section text (up to the next heading of the same or higher level, ignoring fenced code blocks) is sent to Jev as `state.spec`. |
| `implementation` | array of string | Yes | File paths whose contents are concatenated (each preceded by a `// file: <path>` header) into `state.implementation`. |
| `tests` | array of string | Yes | File paths concatenated the same way into `state.tests`. |
| `behaviors` | array of string | Yes | Behavior-description strings; each becomes one `covers_NN` question sent to Jev, in order. |
| `accepted` | array of string | No | Question ids or behavior text whose flag is acknowledged and does not fail the run; see [Report JSON fields](#report-json-fields) below. An entry matching `^covers_\d+$` is rejected at load time (`validateUnits`). Coverage acceptances must name the behavior text itself, not its position, so renumbering `behaviors` cannot silently move an acceptance to a different claim. Every other entry must equal one of `behaviors`, or be `contradicts_spec` or `thoroughness`; an entry matching none of those is also rejected at load time. |
| `notes` | array of string | No | Free-text rationale for the acceptances, copied into the report as `notes`. |

The real `tools/selfreview/units.json` in this repository declares
`"spec": "docs/superpowers/specs/2026-09-23-typesafe-go-sdk-design.md"` and
eight units (`questions`, `responses`, `errors`, `retry`, `client`, `typed`,
`otel`, `cli`). Its `questions` unit, reproduced as an example:

```json
{
  "name": "questions",
  "spec_heading": "### Questions",
  "implementation": ["question.go", "errors.go"],
  "tests": ["question_test.go"],
  "behaviors": [
    "Noul.Criteria is omitted from the wire JSON when nil",
    "A nil Choice label value encodes as JSON null",
    "Choice with zero labels fails validation with path questions.<key>.criteria",
    "Choice with more than 255 labels fails validation",
    "Score with fewer than 2 or more than 10 levels fails validation",
    "An empty questions map fails validation with path questions",
    "RawQuestion without a string type field fails validation",
    "Instructions that are a number fail validation naming the instructions path",
    "Pointers to Noul, Choice, and Score are accepted as questions"
  ],
  "accepted": ["contradicts_spec", "Noul.Criteria is omitted from the wire JSON when nil"],
  "notes": [
    "contradicts_spec: 0.44–0.49 across three runs with unrelated input changes; no spec/code conflict found on review",
    "covers_00: 0.52–0.68 across five runs on byte-identical input; question_test.go pins it in the noul plain and noul no instructions cases and in TestNoulCriteriaOmittedWhenNil"
  ]
}
```

## Default thresholds

Three thresholds gate whether a unit's questions produce a flag:

| Threshold | Default | Meaning |
|---|---|---|
| `min-cover` | `0.6` | Flags a behavior whose `covers_NN` coverage probability is below this value. |
| `max-contradict` | `0.4` | Flags a unit whose `contradicts_spec` probability is above this value. |
| `min-thorough` | `2` | Flags a unit whose `thoroughness` score is below this value. |

These correspond to the `thresholds` struct's `minCover`, `maxContradict`,
and `minThorough` fields in `main.go`, populated from the `-min-cover`,
`-max-contradict`, and `-min-thorough` flags and compared in `evaluate`
against each unit's `covers_NN`, `contradicts_spec`, and `thoroughness`
answers respectively.

## Questions sent to Jev

For each unit, `buildQuestions` constructs one `jev ask` request whose
`state` is `{"spec": <section text>, "implementation": <bundled files>, "tests": <bundled files>}` and whose `questions` are:

| Question id | Type | Purpose |
|---|---|---|
| `covers_NN` (one per behavior, zero-padded two digits) | `noul` | Whether the tests exercise that behavior. |
| `contradicts_spec` | `noul` | Whether the implementation contradicts a spec requirement. |
| `thoroughness` | `score` (4 levels) | How thoroughly the tests cover the spec's required behaviors for the unit. |
| `weakest_area` | `choice` (`validation`, `error_mapping`, `retry`, `decoding`, `encoding`, `logging`, `none`) | Which area of the implementation is least covered by the tests relative to what the spec requires. |

`bundle` truncates `implementation` and `tests` independently, each to half
of `-max-state-bytes`, appending `\n// ...(truncated)\n` when a cut is made;
`Truncated` in the report is true if either half was cut.

## Report JSON fields

Written to `-report` (default `selfreview-report.json`) as a JSON array of
`unitReport`:

```go
type unitReport struct {
    Name         string           `json:"name"`
    RequestID    string           `json:"request_id,omitempty"`
    Model        string           `json:"model,omitempty"`
    InputTokens  int              `json:"input_tokens,omitempty"`
    Truncated    bool             `json:"truncated"`
    Behaviors    []behaviorResult `json:"behaviors"`
    Contradicts  float64          `json:"contradicts_spec"`
    Thoroughness float64          `json:"thoroughness"`
    ThoroughConf float64          `json:"thoroughness_confidence"`
    Weakest      string           `json:"weakest_area"`
    WeakestConf  float64          `json:"weakest_confidence"`
    Flags        []string         `json:"flags"`
    Notes        []string         `json:"notes,omitempty"`
    StaleAcceptances []string     `json:"stale_acceptances,omitempty"`
    Failing      bool             `json:"failing"`
    Error        string           `json:"error,omitempty"`
}

type behaviorResult struct {
    ID       string  `json:"id"`
    Behavior string  `json:"behavior"`
    Covered  float64 `json:"covered"`
}
```

| Field | Type | Presence | Description |
|---|---|---|---|
| `name` | string | Always | The unit's `name` from `units.json`. |
| `request_id` | string | omitempty | The `request_id` from the `jev ask` response for this unit. |
| `model` | string | omitempty | The `model` from the `jev ask` response for this unit. |
| `input_tokens` | int | omitempty | `usage.input_tokens` from the `jev ask` response for this unit. |
| `truncated` | bool | Always | Whether `implementation` or `tests` was cut to fit `-max-state-bytes`. |
| `behaviors` | array of `{id, behavior, covered}` | Always | One entry per `behaviors` entry in `units.json`, `id` being its `covers_NN` question id and `covered` its answered probability. |
| `contradicts_spec` | float64 | Always | The `contradicts_spec` answer's probability. |
| `thoroughness` | float64 | Always | The `thoroughness` answer's score. |
| `thoroughness_confidence` | float64 | Always | The `thoroughness` answer's confidence. |
| `weakest_area` | string | Always | The `weakest_area` answer's chosen label. |
| `weakest_confidence` | float64 | Always | The `weakest_area` answer's confidence. |
| `flags` | array of string | Always (may be empty) | One message per threshold crossed; a message ends with `" (accepted)"` when it matches an entry in the unit's `accepted` list. |
| `notes` | array of string | omitempty | Copied from the unit's `notes` in `units.json`. |
| `stale_acceptances` | array of string | omitempty | Entries from the unit's `accepted` list whose corresponding flag did not fire on this run, sorted. |
| `failing` | bool | Always | True when at least one flag was recorded that was not accepted, or when the unit errored. |
| `error` | string | omitempty | Set instead of the other fields (aside from `name` and `notes`) when the unit could not be run to completion: a missing source file, an unreadable spec heading, a `jev ask` process failure or timeout, or a response the tool could not parse. |

A `covers_NN` behavior flag is keyed in `accepted`/`flags`/`stale_acceptances`
by the behavior's own text, not by `covers_NN`, so reordering `behaviors` in
`units.json` cannot silently reassign an acceptance.

## Exit codes

From `main()`:

| Code | Condition |
|---|---|
| `0` | The report was written and no unit was flagged failing. |
| `1` | The report was written but at least one unit is flagged failing (a flag not covered by `accepted`, or a unit error). |
| `2` | A setup/configuration error: `TYPESAFE_API_KEY` is not set; `-units` could not be read; `-units` content is not valid JSON; `-units` content fails `validateUnits` (an `accepted` entry names a `covers_NN` id, or names something that is neither a behavior nor `contradicts_spec`/`thoroughness`); the spec file named by `spec` could not be read; `-only` matched no unit (zero units ran); or the report file could not be written. |

## Markdown summary

After writing the report, `selfreview` prints a Markdown document to stdout
(`markdown(reports, th)`) and writes `selfreview: ran <n> units` to stderr
before that (or the relevant setup error, for an exit-2 case). The Markdown
starts with a `# Self-review` heading and a table:

| Column | Content |
|---|---|
| Unit | The unit's `name`. |
| Behaviors covered | `<covered>/<total>`, where `covered` counts behaviors whose `covered` probability is at or above `-min-cover` (recomputed from the report's `behaviors` list, independent of which behaviors were flagged or accepted). |
| Contradicts | `contradicts_spec`, formatted `%.2f`. |
| Thoroughness | `thoroughness (thoroughness_confidence)`, formatted `%.2f (%.2f)`. |
| Weakest | `weakest_area`. |
| Flags | The number of entries in `flags`, with `" FAIL"` appended when `failing` is true, or the literal string `error` when `error` is non-empty (which overrides the numeric count). |

Below the table, one `## <name>` subsection is printed for each unit that
either errored, has at least one flag, or has at least one stale acceptance;
a unit with none of those is omitted from this part of the output. An error
subsection prints `error: <message>`. Otherwise it prints, in order: a
`note: sources were truncated to fit the state budget` line when `truncated`
is true, a `note: <text>` line per entry in `notes`, then one `- <message>`
bullet per entry in `flags` (sorted), then one
`- acceptance did not fire: <name>` bullet per entry in `stale_acceptances`.
