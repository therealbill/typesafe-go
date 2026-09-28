---
title: "How to Run the Self-Review"
description: "Run make selfreview, read its Markdown report, and choose between adding a test and recording an accepted exception when Jev flags a behavior."
diataxis: how-to
weight: 100
---

# How to Run the Self-Review

**Goal**: Run `make selfreview` to have Jev judge whether your tests cover
what the spec requires, read its Markdown report, and decide when to add a
test versus record an accepted exception.

## Prerequisites

- `TYPESAFE_API_KEY` set (`make selfreview` sends real requests)
- A built `jev` binary (`make selfreview` depends on `make build`)
- Familiarity with `Noul` and `Score` answers, see
  [Act on probabilities and confidence](./act-on-probabilities-and-confidence.md)

## Steps

### 1. Run `make selfreview`

```
make selfreview
```

This builds `bin/jev`, then runs `go run ./tools/selfreview -jev ./bin/jev`.
Each "unit" in `tools/selfreview/units.json` (a spec section, the
implementation files it governs, and the test files meant to cover it)
becomes one `jev ask` call. Jev is asked a `noul` question per listed
behavior (does a test exercise it), a `noul` for whether the
implementation contradicts the spec, a `score` for overall thoroughness, and
a `choice` for the weakest-covered area.

### 2. Read the printed Markdown table

A real run prints a summary table, one row per unit:

```
# Self-review

| Unit | Behaviors covered | Contradicts | Thoroughness | Weakest | Flags |
|---|---|---|---|---|---|
| questions | 9/9 | 0.44 | 2.41 (0.58) | error_mapping | 1 |
| responses | 6/6 | 0.44 | 2.18 (0.61) | encoding | 1 |
| errors | 5/5 | 0.21 | 2.77 (0.77) | validation | 0 |
| retry | 8/8 | 0.33 | 3.00 (1.00) | encoding | 0 |
| client | 16/16 | 0.42 | 2.77 (0.77) | decoding | 1 |
| typed | 4/4 | 0.23 | 1.93 (0.76) | validation | 1 |
| otel | 7/7 | 0.11 | 2.60 (0.60) | retry | 0 |
| cli | 14/14 | 0.28 | 2.97 (0.97) | retry | 0 |
```

"Behaviors covered" counts how many of a unit's listed behaviors scored at
or above `-min-cover` (default 0.6). "Contradicts" is the
`contradicts_spec` `noul` answer, flagged above `-max-contradict` (default
0.4). "Thoroughness" is the `score` answer with its confidence in
parentheses, flagged below `-min-thorough` (default 2.0). "Flags" is the
count of behaviors that tripped a threshold and were not already accepted;
any non-zero, un-accepted flag exits 1.

### 3. Read the per-unit detail for anything with flags

Below the table, each unit with a flag or a stale acceptance gets its own
section. A real run of this repo's own `questions` unit:

```
## questions

note: contradicts_spec: 0.44–0.49 across three runs with unrelated input changes; no spec/code conflict found on review
note: covers_00: 0.52–0.68 across five runs on byte-identical input; question_test.go pins it in the noul plain and noul no instructions cases and in TestNoulCriteriaOmittedWhenNil

- contradicts_spec: 0.44 > 0.40 (accepted)
- acceptance did not fire: Noul.Criteria is omitted from the wire JSON when nil
```

`(accepted)` means this flag fired but was pre-recorded in `units.json` as
tolerated, so it doesn't fail the run. "acceptance did not fire" is a
**stale acceptance**, covered in Step 5.

### 4. Decide: add a test, or record an acceptance

When a flag isn't already accepted and you agree with it, the direct fix is
a real test. Add a case that would fail if the flagged behavior broke, then
rerun `make selfreview` (or `-only <unit>` for only that unit) to confirm the
flag clears.

When you've reviewed the flag and disagree (the model's judgment doesn't
hold up, or the number sits inside noise you've already measured), record it
in `units.json`'s `accepted` array for that unit, with a note explaining
why. This repo's own `questions` unit does exactly that:

```json
{
  "accepted": ["contradicts_spec", "Noul.Criteria is omitted from the wire JSON when nil"],
  "notes": [
    "contradicts_spec: 0.44–0.49 across three runs with unrelated input changes; no spec/code conflict found on review",
    "covers_00: 0.52–0.68 across five runs on byte-identical input; question_test.go pins it in the noul plain and noul no instructions cases and in TestNoulCriteriaOmittedWhenNil"
  ]
}
```

An entry in `accepted` is either `"contradicts_spec"`, `"thoroughness"`, or
the exact text of one of that unit's `behaviors`, never a positional id
like `covers_03`, since renumbering the behaviors list would silently move
the acceptance to a different claim (`validateUnits` rejects that form
outright).

### 5. Watch for the stale-acceptance line

`unitReport.StaleAcceptances`, printed as `"acceptance did not fire: ..."`,
names an acceptance whose flag didn't trigger on this run. It's
not a failure, but it's a prompt: either the behavior it was guarding is
reliably fine now and the acceptance can be deleted, or, as in the real
example above, it's still measurement noise that happens not to have
crossed the threshold *this* run, in which case leaving the acceptance in
place (with its note) is the right call. Either way, an acceptance that sits
stale run after run without anyone looking at it is worth revisiting.

### 6. Remember the drift caveat: the note is itself a model judgment

`contradicts_spec` and `thoroughness` are themselves Jev's own probability
and score answers, so they carry the same run-to-run drift as any other
`Noul`/`Score` answer, see
[Act on probabilities and confidence](./act-on-probabilities-and-confidence.md).
This repo's own recorded note for `questions`' `contradicts_spec`
acceptance says exactly that: *"0.44–0.49 across three runs with unrelated
input changes; no spec/code conflict found on review"*. The number moves
between runs even when nothing relevant changed, because it's a judgment,
not a deterministic check. Don't read a single run's flag as more precise
than the drift band your own notes have already measured.

## Verify it works

`make selfreview` writes `selfreview-report.json` (gitignored, inspect it
locally, it isn't meant to be committed) alongside the printed table. Its
shape is one object per unit:

```json
{
  "name": "questions",
  "request_id": "req_01a0e93eb4167ec7ab3ea19b8f4e6613",
  "model": "jev-1.13.0",
  "contradicts_spec": 0.44,
  "thoroughness": 2.41,
  "weakest_area": "error_mapping",
  "flags": ["contradicts_spec: 0.44 > 0.40 (accepted)"]
}
```

A run with no un-accepted flags anywhere exits 0; run
`make selfreview; echo $?` and confirm.

✅ You can read the table, tell an accepted flag from a real regression, and
know what a stale acceptance is telling you.

## Troubleshooting

### Problem: `selfreview: TYPESAFE_API_KEY is not set`
**Symptom**: the tool exits 2 immediately with this message.
**Cause**: `tools/selfreview/main.go` checks for the key before reading
`units.json` at all.
**Solution**: export `TYPESAFE_API_KEY` before running `make selfreview`.

### Problem: a unit fails with `jev ask did not finish within 2m0s`
**Symptom**: a specific unit times out instead of completing.
**Cause**: `-unit-timeout` defaults to 2 minutes per `jev ask` call; a large
`implementation`/`tests` bundle for that unit can push close to it.
**Solution**: pass `-unit-timeout` with a longer duration, or narrow the run
to that unit with `-only <name>` while investigating.

### Problem: the run exits 1 but every row in the table looks fine
**Symptom**: nonzero exit despite a clean-looking table.
**Cause**: the table's "Flags" column counts *all* flags including accepted
ones; only an un-accepted flag fails the run, and the table alone doesn't
distinguish them. Check the per-unit detail sections below it.
**Solution**: read the detail section for any unit with a nonzero flag
count, and check for a line without `(accepted)` at the end.

## Next steps

- See the tool's full flag and JSON report reference in
  [Self-Review Tool](../reference/self-review-tool.md).
- Read [What the Self-Review Measures](../explanation/what-the-self-review-measures.md)
  for why its thresholds are set the way they are.

## See also

- [Act on probabilities and confidence](./act-on-probabilities-and-confidence.md)
- [Cut a release](./cut-a-release.md)
