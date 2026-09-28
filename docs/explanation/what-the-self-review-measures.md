---
title: "What the Self-Review Measures"
description: "How tools/selfreview asks Jev to judge its own spec coverage, why its thresholds have to sit inside the model's own measurement noise, and why that makes it a developer tool rather than a CI gate."
diataxis: explanation
weight: 80
---

# What the Self-Review Measures

`tools/selfreview` is this repository asking its own AI provider a question
about itself: for each unit of the codebase, does the test suite exercise
what the spec requires, and does the implementation contradict the
spec anywhere? It's an unusual tool to find in a Go module, and unpacking
what it does, and what its numbers do and don't mean, is the point of this
page.

## How the tool turns code into questions

`runUnit`, in `tools/selfreview/main.go`, works from a `units.json`
configuration where each unit names a spec heading, a set of implementation
files, a set of test files, and a list of behaviors the spec is understood
to require. For one unit, `runUnit` extracts the named section of the spec
(`extractSection`, bounded by heading level and aware of fenced code
blocks), bundles the implementation and test file contents (truncating each
half of a shared byte budget if needed, with the source files listed by
path), and assembles a request body: `{"spec": ..., "implementation": ...,
"tests": ...}` as `state`, alongside a set of questions built by
`buildQuestions`. That request is sent to Jev the same way any other caller
would, piped as JSON into `jev ask` as a subprocess, which makes
`selfreview` itself an ordinary agent-facing consumer of the CLI contract
described in
[The agent-facing CLI contract](the-agent-facing-cli-contract.md).

The questions `buildQuestions` asks are concrete and drawn straight from the
source:

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

`evaluate` then compares each answer against a threshold, `minCover`
(default 0.6) per behavior, `maxContradict` (default 0.4), `minThorough`
(default 2.0), and flags anything that crosses it, unless that specific
flag has been accepted (below).

## Why thresholds sit inside the model's own drift band

The thresholds above aren't derived from a formula; they're set by running
the tool repeatedly against unchanged code and watching how much the
answers move. `units.json` records that noise directly, as notes attached
to accepted flags. For the `questions` unit:

> contradicts_spec: 0.44–0.49 across three runs with unrelated input
> changes; no spec/code conflict found on review

> covers_00: 0.52–0.68 across five runs on byte-identical input;
> question_test.go pins it in the noul plain and noul no instructions cases
> and in TestNoulCriteriaOmittedWhenNil

The second note is the sharper illustration: byte-identical input (the
exact same spec section, the exact same file contents, nothing changed
between runs) still produced coverage probabilities ranging from 0.52 to
0.68 for the same behavior. The `responses` unit shows the same pattern from
the other side, on `thoroughness` rather than `contradicts_spec`:

> thoroughness: 1.87, 2.00, 1.98 across three runs with all six behaviors at
> 0.93 or above; threshold sits inside the model's drift band

`minThorough` is 2.0, and a well-tested unit still scored 1.87 on one run
of three. A single point estimate from one run cannot be trusted at face
value near a threshold like that: the same judgment, asked about code that
hasn't changed, comes back with a different number each time, from the
model's own sampling variance. Setting `minCover: 0.6` or `minThorough:
2.0` picks a line, aware that judgments near it will sometimes fall on the
wrong side from that noise, rather than claiming precision the underlying
measurement doesn't have. That's why the `accepted` mechanism exists
alongside the thresholds, instead of replacing them.

## What `accepted` records

An `accepted` entry on a unit names a specific flag (a behavior's exact
text, or the fixed ids `"contradicts_spec"`/`"thoroughness"`) that is
allowed to keep firing without failing the run. `validateUnits` specifically
rejects accepting a `covers_NN` id by position (`coversID` regex), forcing
acceptances to name the behavior's own text instead, so that reordering or
inserting a new behavior in `units.json` can't silently reassign an old
acceptance to a different, unreviewed claim. Each acceptance in the file
carries a note explaining why a human reviewed the flag and judged it not to
indicate a real problem: the `contradicts_spec: 0.44–0.49 ... no spec/code
conflict found on review` note above is exactly that, a record that someone
looked at the flagged discrepancy and concluded it was noise, not a defect.

That's a distinct outcome from fixing the underlying issue. Accepting a flag
leaves the test suite and the code unchanged. It is a documented judgment
call that this particular signal, at this particular threshold, isn't worth
chasing further right now. `evaluate` also tracks the reverse case:
`StaleAcceptances` lists any accepted entry whose flag didn't fire on a
given run, so an acceptance that's no longer needed (because the flag
stopped triggering, not because someone removed the entry) stays visible
in the report rather than sitting silently unused.

## The retry unit's boundary doesn't match its own name

Reading the `retry` unit's entry in `units.json` closely surfaces something
worth knowing before trusting a `weakest_area` answer at face value. The
`retry` unit (`spec_heading: "### Retry policy"`) lists eight behaviors, all
about backoff and header handling: the default policy's shape, delay
doubling and capping, jitter, `retry-after-ms` taking precedence over
`Retry-After`, `Retry-After` parsing (seconds, HTTP dates, a past date
yielding zero), `HonorRetryAfter=false`, `ShouldRetry` overriding all other
decisions, and `RetryOnConnErr`/`RetryOnTimeout` being respected. Nothing in
that list mentions retry *budgets* or *cancellation*: those behaviors
("The retry budget prevents a retry whose delay would exceed it",
"Cancelling the context during backoff returns promptly with
context.Canceled") are listed under the separate `client` unit instead
(`spec_heading: "### Client"`).

But `buildQuestions`' `weakest_area` choice offers `"retry"` as one option,
described to the model as: "Backoff, Retry-After, budget, and
cancellation." That description spans both units: it bundles the `retry`
unit's own behaviors with two behaviors that live in the `client`
unit's spec section. As a result, a `weakest_area: retry` answer on the
`client` unit's own run isn't out of place or a sign the tool is confused:
the category covers material tested under `client`, not only under the
unit literally named `retry`. Reading `weakest_area` usefully means
remembering that its category boundaries were drawn once, by hand, in
`buildQuestions`, and don't necessarily line up with the per-unit
boundaries `units.json` uses everywhere else in the same tool.

## Why it's a development tool, not a CI gate

Nothing in this repository's GitHub Actions workflows runs `selfreview`; the
only place it's invoked is the `make selfreview` target, which depends on
`build` and requires `TYPESAFE_API_KEY` to be set: `main()` exits
immediately if that variable is absent. That placement, outside CI, follows
directly from three properties this page has already described. It calls
the live TypeSafe API, which means it costs real tokens on every run and
can't execute without a live credential, something a CI job either has to
be trusted with or can't run at all. It reports token usage per unit in its
own output (`InputTokens` on each report entry) as a visible reminder of
that cost. And, as the drift notes above show directly, its own findings are
not deterministic: the same code, judged twice, can land on different sides
of a threshold. A required CI gate needs to produce the same verdict on the
same input every time, or it trains developers to ignore it as flaky; a tool
whose own documentation records 0.52–0.68 swings on byte-identical input
cannot offer that. It suits the role it's wired up for here: a `make`
target a developer runs by choice, for a second opinion on whether a
change's tests back up what the spec claims, with a human in the loop to
read the flags, accept the noise, and act on the rest.

## Related documentation

- For why `jev`'s stdin/stdout/exit-code shape is what a subprocess caller
  like `selfreview` relies on, see
  [The agent-facing CLI contract](the-agent-facing-cli-contract.md).
- For the noul, choice, and score question types `selfreview` builds and
  sends, see the [question types reference](../reference/question-types.md).
- For how those answers are decoded on the way back, see
  [How answers are decoded](how-answers-are-decoded.md).
