---
title: "What jev review Measures"
description: "How jev review turns a spec section and its code into four graded questions, why its thresholds sit inside the model's own measurement noise, and what its readings can and cannot show."
diataxis: explanation
weight: 80
aliases: ["/docs/explanation/what-the-self-review-measures/"]
---

`jev review` judges whether your tests exercise the behaviors you listed and whether your implementation contradicts the spec section you named. It sends those files to Jev and answers with probabilities. It does not find bugs, review style, check security, or verify that the code is correct, and its readings drift between runs on the same input. This page describes what each question type measures, why its thresholds sit inside the model's own drift, what an acceptance records, and why the tool runs by choice rather than in CI.

## What jev review asks

Each unit `jev review` evaluates names a spec section, a set of implementation files, a set of test files, and a list of behaviors the spec is understood to require. `buildQuestions`, in `internal/review/review.go`, builds four kinds of question from that unit.

One `noul` question per listed behavior, `covers_NN`, asks: "Do the tests in `tests` exercise the behavior below, which `spec` requires of the code in `implementation`?" It carries guidance that a test must be capable of failing if the behavior broke; merely mentioning a feature without asserting on it does not count. The answer is a probability, 0 to 1, that a real, failure-capable test exists for that specific claim, not a coverage percentage of lines or branches.

One `noul` question, `contradicts_spec`, asks whether the implementation contradicts a requirement stated in the spec. It counts only actual conflicts: a different default, a different error type, a different wire field name, opposite behavior. It ignores omissions and anything the spec leaves unspecified. The answer is a probability of an active conflict, not a probability of incompleteness.

One `score` question, `thoroughness`, has four ordered levels, 0 to 3, from "no test exercises this section's behavior" up through "success, error cases, and the boundary values, cancellation, or concurrency conditions named in spec are all tested." It is a single holistic judgment about the whole unit, not an aggregate of the individual `covers_NN` answers.

One `choice` question, `weakest_area`, picks which of seven fixed categories, validation, error_mapping, retry, decoding, encoding, logging, or none, is least covered by tests relative to what the spec requires.

## Why thresholds sit inside the model's own measurement noise

The same code, judged twice with no relevant change, does not always produce the same number. Each answer is a probabilistic judgment, not a deterministic check, so repeated runs on identical input land at different points rather than at one fixed value.

This repository's own `jev-review.json` records that movement directly, in the notes attached to its accepted flags, each written after a human reviewed the discrepancy and found no actual defect.

The `questions` unit's `contradicts_spec` reading moved from 0.44 to 0.49 across three runs with unrelated input changes elsewhere in the repository; no spec/code conflict was found on review. Its `covers_00` behavior, about `Noul.Criteria` being omitted from the wire JSON when nil, moved from 0.52 to 0.68 across five runs on byte-identical input, despite that exact behavior being pinned by three separate test cases in the suite.

The `responses` unit's `contradicts_spec` reading sat at 0.41 to 0.43, within noise of the 0.40 default threshold, with no conflict found on review. Its `thoroughness` score read 1.87, 2.00, and 1.98 across three runs, with all six of its behaviors independently scored at 0.93 probability or above each time. The default `--min-thorough` threshold is 2.0, so one of those three runs landed below the line on unchanged, well-covered code.

The `client` unit's `contradicts_spec` reading moved from 0.35 to 0.47 on byte-identical input, attributed to sampling variance. The `typed` unit's `thoroughness` score read 1.98, 2.05, 2.06, 2.06, and 1.94 across five runs, with all four of its behaviors at 0.87 probability or above each time.

A default threshold of 0.6 for coverage, 0.4 for contradiction, and 2.0 for thoroughness draws a line that a reading near it will cross in both directions from sampling noise alone, on code that has not changed. A single run's number close to a threshold does not, by itself, establish which side of that line the code actually belongs on.

## How unit boundaries change a reading

What a unit's `implementation` and `tests` lists include shapes a reading as much as the code's own state does. Reviewing this repository's own `### Retry policy` spec section against its actual `retry.go` and `retry_test.go` source shows the effect directly.

Reviewed with only `retry.go` and `retry_test.go`, the unit's own backoff, jitter, and Retry-After logic and that file's own tests, the unit's `thoroughness` score came back 1.75, below the 2.0 default threshold, and `contradicts_spec` at 0.42, above the 0.4 default threshold. Both flagged; the unit failed.

Reviewed with `transport.go` and `client_test.go` also included, this repository's actual, checked-in configuration for this unit, which folds in cancellation, retry-budget, and connection-error-handling coverage exercised through those files, the same unit's `thoroughness` came back 3.00, the maximum level, with `contradicts_spec` at 0.29. No flags.

Nothing about the code changed between these two runs. Only which files the unit's `implementation` and `tests` lists named changed. The behaviors listed for a unit, and the files bundled to answer them, define what the reading can see. A unit whose file lists do not include the tests that actually exercise a behavior reads as under-tested even when that behavior is well tested elsewhere in the same codebase.

A unit's own `spec` follows the same principle for the spec side of the reading: it lets a unit read the document that actually describes it, when the code it covers is documented apart from the top-level spec.

## What accepted and stale-acceptance lines record

An `accepted` entry on a unit names one flag that may keep firing without making the run fail: a behavior's exact text, or one of the fixed ids `contradicts_spec`, `thoroughness`, `weakest_area`. Config loading rejects an acceptance written as a positional id like `covers_00`, requiring the behavior's own text instead, so reordering or inserting a behavior cannot silently reassign an old acceptance to a different, unreviewed claim.

Each acceptance in this repository's own config carries a note recording the review that produced it. The drift observations above are those notes. Accepting a flag leaves the test suite and the code unchanged. It records a judgment that a specific signal, at a specific threshold, was reviewed and found not to indicate a defect, not that the underlying question was answered differently.

`jev review` also tracks the reverse case: an accepted entry whose flag does not fire on a given run is reported as a stale acceptance, `"acceptance did not fire: <name>"`, rather than sitting silently unused in the config. An acceptance that stopped being needed is reported rather than left in place.

## Why it is a development aid, not a CI gate

No CI workflow in this repository runs `jev review`. It is a target a developer runs by choice.

Three properties follow from the mechanism described above. It calls a live API, so every run has a real cost and needs a live credential. The report records that cost per unit, as `input_tokens`. Its readings are not deterministic, as the drift observations above show directly: the same code, judged twice, can land on different sides of a threshold.

A gate that returns different verdicts for the same input on repeated runs is a different kind of tool than a deterministic CI check. `jev review` puts a human in the loop to read the flags, decide, and record that decision, rather than pass or fail a build automatically.

## What it cannot tell you

`jev review` does not run the code, so it cannot find a bug that a passing test suite doesn't reveal. It is not a style or security check. A `contradicts_spec` reading of 0 does not mean the code is correct; it means the model found no conflict with what the spec explicitly states. Anything the spec leaves unspecified is outside what this question can see.

## Related documentation

- [Review your codebase with jev](../how-to/review-your-codebase-with-jev.md)
- [jev review reference](../reference/jev-review.md)
