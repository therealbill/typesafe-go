---
title: "What the Self-Review Measures"
description: "How tools/selfreview asks Jev to judge its own spec coverage, why its thresholds have to sit inside the model's own measurement noise, and why that makes it a developer tool rather than a CI gate."
diataxis: explanation
weight: 80
---

`tools/selfreview` sends this repository's own code to its AI provider and
asks two questions about each unit of the codebase: does the test suite
exercise what the spec requires, and does the implementation contradict the
spec anywhere. This page describes how the tool builds those questions, what
its thresholds are calibrated against, and what its numbers do and do not
support.

## How the tool turns code into questions

`runUnit`, in `tools/selfreview/main.go`, reads a `units.json`
configuration. Each unit there names a spec heading, a set of implementation
files, a set of test files, and a list of behaviors the spec is understood
to require. For one unit, `runUnit` extracts the named section of the spec
with `extractSection`, which bounds the section by heading level and is
aware of fenced code blocks. It bundles the implementation and test file
contents, truncating each half of a shared byte budget when needed and
listing the source files by path. It assembles `{"spec": ...,
"implementation": ..., "tests": ...}` as `state` and pairs it with the
questions `buildQuestions` returns. It then pipes that request as JSON into
`jev ask` as a subprocess, which makes `selfreview` an ordinary agent-facing
consumer of the CLI contract described in
[The agent-facing CLI contract](the-agent-facing-cli-contract.md).

`buildQuestions` asks four kinds of question, all drawn from the source:

- One noul question per named behavior, `covers_NN`, asking: "Do the Go
  tests in `tests` exercise the behavior below, which `spec` requires of the
  code in `implementation`?" It comes with explicit guidance attached:
  "Answer yes only if at least one test would fail if this behavior were
  removed or broken. A test that merely mentions the feature without
  asserting on it does not count."
- A `contradicts_spec` noul question: "Does the code in `implementation`
  contradict a requirement stated in `spec`? Count only actual conflicts
  such as a different default value, a different error type, a different
  wire field name, or opposite behavior. Ignore omissions and ignore
  anything the spec leaves unspecified."
- A `thoroughness` score question, with four rubric levels running from "No
  test exercises this section's behavior" up to "Success, error cases, and
  the boundary values, cancellation, or concurrency conditions named in
  `spec` are all tested."
- A `weakest_area` choice question, picking which of validation, error
  mapping, retry, decoding, encoding, logging, or none is "least covered by
  `tests` relative to what `spec` requires."

`evaluate` compares each answer against a threshold: `minCover` (default
0.6) per behavior, `maxContradict` (default 0.4), and `minThorough` (default
2.0). It flags any answer that crosses one, unless that specific flag has
been accepted.

## Where the thresholds come from

No formula produces these thresholds. They come from running the tool
repeatedly against unchanged code and watching how far the answers move.
`units.json` records that movement directly, as notes attached to accepted
flags. Two notes on the `questions` unit:

> contradicts_spec: 0.44–0.49 across three runs with unrelated input
> changes; no spec/code conflict found on review

> covers_00: 0.52–0.68 across five runs on byte-identical input;
> question_test.go pins it in the noul plain and noul no instructions cases
> and in TestNoulCriteriaOmittedWhenNil

The second note covers byte-identical input: the same spec section, the same
file contents, nothing changed between five runs, and coverage
probabilities for one behavior ranging from 0.52 to 0.68. The `responses`
unit records the same spread on `thoroughness` rather than
`contradicts_spec`:

> thoroughness: 1.87, 2.00, 1.98 across three runs with all six behaviors at
> 0.93 or above; threshold sits inside the model's drift band

`minThorough` is 2.0, and that unit scored 1.87 on one run of three with all
six behaviors at 0.93 or above. The model's sampling variance moves a
judgment about unchanged code by that much between runs, so a single run's
number near a threshold does not establish which side of it the code belongs
on. `minCover: 0.6` and `minThorough: 2.0` draw a line that answers near it
will cross in both directions from noise alone. The `accepted` mechanism
handles those crossings; it sits alongside the thresholds rather than
replacing them.

## What `accepted` records

An `accepted` entry on a unit names one flag that may keep firing without
failing the run: a behavior's exact text, or the fixed id
`"contradicts_spec"` or `"thoroughness"`. `validateUnits` rejects an
acceptance that names a `covers_NN` id by position, matched by the
`coversID` regex, so an acceptance must name the behavior's own text.
Reordering or inserting a behavior in `units.json` therefore cannot silently
reassign an old acceptance to a different, unreviewed claim. Each acceptance
carries a note recording the review that produced it; the
`contradicts_spec: 0.44–0.49 ... no spec/code conflict found on review` note
above records a reviewer looking at the flagged discrepancy and concluding
it was noise, not a defect.

Accepting a flag leaves the test suite and the code unchanged, which makes
it a different outcome from fixing the underlying issue. It records a
judgment that this signal, at this threshold, is not worth chasing further
right now. `evaluate` tracks the reverse case as well: `StaleAcceptances`
lists any accepted entry whose flag did not fire on a given run, so an
acceptance that stopped being needed appears in the report instead of
sitting unused in the file.

## The `weakest_area` categories and the unit boundaries

The `weakest_area` categories do not line up with the unit boundaries. The
`retry` unit (`spec_heading: "### Retry policy"`) lists eight behaviors, all
about backoff and header handling: the default policy's shape, delay
doubling and capping, jitter, `retry-after-ms` taking precedence over
`Retry-After`, `Retry-After` parsing (seconds, HTTP dates, a past date
yielding zero), `HonorRetryAfter=false`, `ShouldRetry` overriding all other
decisions, and `RetryOnConnErr`/`RetryOnTimeout` being respected. Retry
budgets and cancellation are not among them. Those two behaviors ("The retry
budget prevents a retry whose delay would exceed it", "Cancelling the
context during backoff returns promptly with context.Canceled") sit under
the separate `client` unit (`spec_heading: "### Client"`).

`buildQuestions` describes the `weakest_area` option `"retry"` to the model
as: "Backoff, Retry-After, budget, and cancellation." That description spans
both units, bundling the `retry` unit's own behaviors with two that live in
the `client` unit's spec section. A `weakest_area: retry` answer on the
`client` unit's own run therefore points at material tested under `client`,
not only at the unit literally named `retry`. The `weakest_area` category
boundaries were drawn once, by hand, in `buildQuestions`, and do not track
the per-unit boundaries `units.json` uses everywhere else in the same tool.

## Where the tool runs

No GitHub Actions workflow in this repository runs `selfreview`. The
`make selfreview` target is the only entry point; it depends on `build`, and
`main()` exits immediately when `TYPESAFE_API_KEY` is absent. Three
properties keep it out of CI. It calls the live TypeSafe API, so every run
costs real tokens and needs a live credential that a CI job would have to be
trusted with. It reports token usage per unit in its own output, through
`InputTokens` on each report entry, as a visible reminder of that cost. Its
findings are also not deterministic: the drift notes above record 0.52 to
0.68 swings on byte-identical input, so the same code, judged twice, can
land on different sides of a threshold. A required gate that returns
different verdicts for the same input trains developers to ignore it as
flaky. `make selfreview` is a target a developer runs by choice, for a
second opinion on whether a change's tests back up what the spec claims,
with a human in the loop to read the flags, accept the noise, and act on the
rest.

## Related documentation

- For why `jev`'s stdin/stdout/exit-code shape is what a subprocess caller
  like `selfreview` relies on, see
  [The agent-facing CLI contract](the-agent-facing-cli-contract.md).
- For the noul, choice, and score question types `selfreview` builds and
  sends, see the [question types reference](../reference/question-types.md).
- For how those answers are decoded on the way back, see
  [How answers are decoded](how-answers-are-decoded.md).
