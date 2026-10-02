---
title: "Review Your Codebase with Jev"
description: "Configure jev-review.json for your spec and tests, run jev review, and decide whether to add a test, fix a contradiction, or record an accepted exception for each flag."
diataxis: how-to
weight: 100
aliases: ["/docs/how-to/run-the-self-review/"]
---

`jev review` judges whether your tests exercise the behaviors you listed and whether your implementation contradicts the spec section you named. It sends those files to Jev and answers with probabilities. It does not find bugs, review style, check security, or verify that the code is correct, and its readings drift between runs on the same input.

**Goal**: review your own codebase with `jev review`.

## Prerequisites

- A `jev` binary: a release download, or `make build` in this repository.
- `TYPESAFE_API_KEY` set in the environment. `jev review` sends real requests to Jev.
- A Markdown spec file with headings describing the behavior you want reviewed. See [How to write a spec jev review can use](./write-a-spec-jev-review-can-use.md) for how to structure and word it.

## Steps

### 1. Write a starter config with `jev review init`

Run it in the directory where `jev-review.json` should live:

```
$ jev review init
{"wrote":"jev-review.json"}
```

It refuses to overwrite an existing file:

```
$ jev review init
{"error":{"kind":"usage","message":"jev-review.json already exists; remove it or pass --units"}}
jev: jev-review.json already exists; remove it or pass --units
$ echo $?
1
```

The file it writes is a template with one example unit:

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

### 2. Edit `jev-review.json` for your own spec sections

Replace the example unit with one unit per spec section you want reviewed. Every path in the file, including `spec`, resolves relative to the directory containing `jev-review.json`, regardless of where you run the command from. `spec_heading` must match a heading line in the spec file exactly, including the `#` characters: `"## Retry policy"`, not `"Retry policy"`.

Each unit needs a `name` that is unique across the file, a `spec_heading`, at least one `implementation` file, and at least one `behaviors` entry. `tests` and `notes` are optional. `accepted` defaults to empty.

### 3. Pair each section with the files that implement it

Read the spec section and list the functions, types, and options it names.

Find the non-test files that define them and use those as `implementation`:

```
grep -l 'func .*Name' *.go
```

Find the test files that reference those functions and types and use those as `tests`. A file with a similar name is not evidence. The exporter for a feature is not the feature.

When a section's API spans several files, include them all. The state budget is 100 KB by default, split evenly between `implementation` and `tests`, and the report's `truncated` field is `true` when a bundle is cut.

A unit that reads 0 of N behaviors with a real test file present usually means the wrong pairing, not missing tests. Fix the pairing before adding a test. A `tracker.go` feature paired with `exporter_test.go` is this mistake: the exporter's tests exercise the exporter, not the tracker, so every `covers_NN` reads near zero regardless of how well the tracker is actually tested elsewhere.

Use `--only NAME` while you fix one unit's pairing, so each run judges only that unit.

When the spec is spread across several documents, set `spec` on the unit that needs a different file than the top-level `spec`. It resolves relative to the config file, the same as every other path in it:

```json
{
  "spec": "docs/design.md",
  "units": [
    {
      "name": "storage",
      "spec_heading": "## Storage",
      "implementation": ["storage.go"],
      "tests": ["storage_test.go"],
      "behaviors": ["A write past the configured limit is rejected"]
    },
    {
      "name": "cache",
      "spec": "internal/cache/README.md",
      "spec_heading": "## Eviction",
      "implementation": ["internal/cache/lru.go"],
      "tests": ["internal/cache/lru_test.go"],
      "behaviors": ["The least recently used entry is evicted first"]
    }
  ]
}
```

The `cache` unit's own `spec` overrides the top-level `spec` for that unit only; `storage` still reads from `docs/design.md`.

### 4. Write behaviors as concrete, testable claims

List five to fifteen behaviors per unit, one per line. A good behavior names a specific, checkable outcome. A bad one is vague or untestable.

Good:

- "A failed publish is retried up to three times": a test can assert the exact call count.
- "An empty widget name is rejected before any publish attempt is made": a test can assert an error return with zero calls to the sender.

Bad:

- "Retries work correctly": nothing specific to assert.
- "The code handles errors well": no concrete outcome named.

### 5. Check the config with `--dry-run`

`jev review --dry-run` loads the config, extracts every unit's spec section, and reads and bundles every listed file, then stops before the API call. It needs no API key and sends nothing. A heading that does not match a line in the spec, or a file that does not exist, shows up here instead of in a paid run.

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

Fix the heading text in the config or the spec and rerun until the exit code is 0:

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

Each row shows the extracted section's size and each bundle's size before truncation, against the budget line above the table. A `yes` under Truncated means the bundle was cut; split the unit or raise `--max-state-bytes`.

### 6. Run `jev review`

A two-unit project with a retry policy and a validation rule, where the tests don't yet cover two of the five listed behaviors:

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

### 7. Read the table and the flagged lines

"Behaviors covered" counts behaviors whose coverage probability is at or above `--min-cover` (default 0.6). "Contradicts" is the contradiction probability, flagged above `--max-contradict` (default 0.4). "Thoroughness" is the score with its confidence in parentheses, flagged below `--min-thorough` (default 2.0). Below the table, each flagged unit gets its own section naming the exact behavior or question that crossed a threshold.

### 8. Decide how to respond to each flag

For each flag, choose one of three responses.

**Add a test** when the behavior genuinely isn't tested. The run above is missing coverage for the retry delay and the name-length check. After adding a test that asserts the retry delay stays at a fixed ~100ms and doesn't grow, and a test that asserts a name over 64 bytes is rejected, a rerun of the same two units produces:

```
$ jev review
review: ran 2 units
# jev review

| Unit | Behaviors covered | Contradicts | Thoroughness | Weakest | Flags |
|---|---|---|---|---|---|
| retry | 3/3 | 0.29 | 2.13 (0.74) | none | 0 |
| validation | 2/2 | 0.05 | 2.09 (0.89) | none | 0 |

$ echo $?
0
```

**Fix the spec or the code** when they genuinely disagree, such as a `contradicts_spec` flag pointing at a real conflict.

**Record an acceptance with a note** when you've reviewed the flag and it's noise rather than a defect. Add the flag's key, either the exact behavior text or one of `contradicts_spec`, `thoroughness`, `weakest_area`, to that unit's `accepted` array, and a note to `notes` explaining the review. This repository's own `jev-review.json` does exactly that for its `questions` unit:

```json
{
  "accepted": [
    "contradicts_spec",
    "Noul.Criteria is omitted from the wire JSON when nil"
  ],
  "notes": [
    "contradicts_spec: 0.44–0.49 across three runs with unrelated input changes; no spec/code conflict found on review",
    "covers_00: 0.52–0.68 across five runs on byte-identical input; question_test.go pins it in the noul plain and noul no instructions cases and in TestNoulCriteriaOmittedWhenNil"
  ]
}
```

An acceptance always names the behavior's exact text. `jev review` rejects a `covers_NN`-shaped acceptance at load time, with an error explaining that renumbering the behaviors list would otherwise silently move the acceptance to a different claim.

### 9. Iterate on one unit with `--only`

```
jev review --only retry
```

This reruns a single named unit instead of the whole config, useful while you work through that unit's flags.

### 10. Calibrate thresholds against your own drift

`--min-cover`, `--max-contradict`, and `--min-thorough` all carry defaults, but none of them has a universally correct value. The right value is one calibrated against your own repeated runs on unchanged code. Watch a few runs of your own suite before changing any of them. See [What Jev Review Measures](../explanation/what-jev-review-measures.md) for why the defaults sit where they do.

## Verify it works

A clean run exits 0:

```
jev review; echo $?
```

`jev review` also writes a JSON report, default path `jev-review-report.json` (add it to `.gitignore`), holding the same data as the Markdown table, one object per unit. `jev review --only retry --json` on the fixed scratch example from step 8:

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

✅ A clean run exits 0, and a flagged run's per-unit section names the exact
behavior or question that crossed its threshold.

## See also

- [jev review](../reference/jev-review.md) for the full flag and JSON report reference.
- [What Jev Review Measures](../explanation/what-jev-review-measures.md) for why the thresholds sit where they do and what drift means.
- [Act on probabilities and confidence](./act-on-probabilities-and-confidence.md) for reading `Noul`/`Score` answers in general, including the drift that shows up in `contradicts_spec` and `thoroughness`.
- [Cut a release](./cut-a-release.md), which runs a review before tagging.
