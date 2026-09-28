---
title: "How to Act on Probabilities and Confidence"
description: "Read NoulAnswer, ChoiceAnswer, and ScoreAnswer as probabilities rather than labels, pick your own thresholds, and handle a none-style label explicitly."
diataxis: how-to
weight: 70
---

**Goal**: Turn `NoulAnswer`, `ChoiceAnswer`, and `ScoreAnswer` values into
decisions your code can act on, without treating a probability near the
middle as a third label or a confident answer as more certain than it is.

## Prerequisites

- A working `SystemOne` call and familiarity with the answer types, see
  [Your First Judgment in Go](../tutorials/first-judgment-in-go.md) and the
  [response types reference](../reference/response-types.md)
- TypeSafe's own confidence documentation at
  [docs.typesafe.ai/confidence](https://docs.typesafe.ai/confidence) covers
  how these numbers are produced; this page covers what to do with them in Go

## Steps

### 1. Treat `NoulAnswer.Noul` as a probability, not a yes/no

`Noul` is the probability, from 0 to 1, that the condition holds. It is not a
boolean and not a three-way "yes/no/maybe". A value near 0.5 means the model
is undecided about *this specific state*, not that the answer is
"medium":

```go
const billingThreshold = 0.8
const undecidedLow, undecidedHigh = 0.4, 0.6

p := res.Nouls()["is_billing"].Noul
switch {
case p >= undecidedLow && p <= undecidedHigh:
	// genuinely undecided — don't round this to a label
case p >= billingThreshold:
	// confident yes
default:
	// confident no
}
```

Against a real ticket ("My invoice for last month shows a charge I don't
recognize, and I'm getting really frustrated...") `is_billing` came back
`0.99`. Against an intentionally ambiguous ticket that mentioned both a
payment and a login problem, the same question came back `0.47`, inside the
undecided band.

### 2. Pick the threshold against your own data, not a universal number

No single cutoff fits every use of `billingThreshold`. A routing decision
that is cheap to get wrong (route to a queue a human can re-triage) can use
a looser threshold than one that is expensive to get wrong (auto-refund a
charge). Run representative states through the question, look at the
distribution of scores that come back, and set both the threshold and the
width of the undecided band from that distribution.

### 3. Use `ChoiceAnswer.Confidence` to decide how much to trust `.Choice`

`Confidence` (0–1) summarizes how concentrated the label distribution is. It
is high when one label dominates, low when the model is split between two or
more labels. Don't act on `.Choice` alone without checking it:

```go
const choiceConfidenceThreshold = 0.7

tone := res.Choices()["tone"]
if tone.Confidence >= choiceConfidenceThreshold {
	// trust tone.Choice directly
} else {
	// low confidence — inspect tone.Probabilities for a close second place
	for label, p := range tone.Probabilities {
		_ = label
		_ = p
	}
}
```

A real run on "hey so, this might be nothing, but I wanted to flag it just
in case. Not mad or anything..." returned `tone: "neutral"` at confidence
`0.98`, with `probabilities: {"angry":0, "frustrated":0.01, "neutral":0.99}`.
That distribution concentrates on one label, so `.Choice` can be trusted
directly. A lower `Confidence` on a different state means the top two labels
are close, which `.Probabilities` shows and `.Choice` alone does not.

### 4. Handle a `none`-style label as its own branch

If your `Criteria` includes an explicit "doesn't apply" label (`none`,
`other`, whatever fits your domain), check for it by name instead of
lumping it in with "no answer" or a low-confidence case:

```go
topic := res.Choices()["topic"]
switch {
case topic.Choice == "none":
	// no identifiable topic — this is itself useful information
case topic.Confidence >= choiceConfidenceThreshold:
	// trust topic.Choice directly
default:
	// low confidence — inspect topic.Probabilities
}
```

A real run on a thank-you-only ticket returned `topic: "none"` at confidence
`1.0`. The model was certain the ticket concerned neither billing nor login.
That is a different signal from a low-confidence answer, where the model did
not settle on a label.

### 5. Read `ScoreAnswer` the same way: `Score`, `Confidence`, `Probabilities`

`Score` is the probability-weighted position on your rubric (not
necessarily an integer, since it's a weighted average across levels), and
`Confidence`/`Probabilities` work exactly like `Choice`'s:

```go
const scoreConfidenceThreshold = 0.7

urgency := res.Scores()["urgency"]
if urgency.Confidence < scoreConfidenceThreshold {
	for level, p := range urgency.Probabilities {
		_ = urgency.Legend[level]
		_ = p
	}
}
```

A real run returned `urgency.Score = 1.67` at `Confidence = 0.5`, with
`probabilities: {"0":0, "1":0.33, "2":0.67}` against
`legend: {"0":"low","1":"medium","2":"high"}`. The distribution leans toward
"high" without concentrating there. `Score` of 1.67 sits between medium and
high, and `Confidence` of 0.5 reports the same lack of concentration
directly. Don't treat this answer as settled.

## Verify it works

Running Steps 1, 3, 4, and 5 together against a fake server returning the
real values captured above prints:

```
is_billing: 0.99 >= 0.80 threshold -> treat as billing
is_billing_ambiguous: undecided (0.47), treating as unknown
tone: frustrated (confidence 1.00, trusted directly)
topic: none (confidence 1.00) -- ticket has no identifiable topic
urgency: 1.67 (confidence 0.50, low -- inspecting level probabilities)
  level 0 (low): 0.00
  level 1 (medium): 0.33
  level 2 (high): 0.67
```

✅ Every answer is read as a probability with a confidence, not collapsed to
a bare label before your code gets a chance to look at it.

## Troubleshooting

### Problem: a `Noul` near 0.5 keeps getting treated as a real answer
**Symptom**: downstream logic branches on `p > 0.5` as if it were a clean
boolean, and behaves inconsistently on borderline states.
**Cause**: 0.5 is the natural midpoint of a probability, not a "maybe" label
baked into the type. Treating it as meaningfully different from
0.49 or 0.51 overstates the model's precision there.
**Solution**: define an explicit undecided band (Step 1) and route it to a
fallback (a human queue, a default action, a second question) instead of
forcing a binary decision out of it.

### Problem: `ChoiceAnswer.Choice` looks wrong even though `Confidence` was low
**Symptom**: code that only reads `.Choice` picks the "wrong" label on a
close call.
**Cause**: `.Choice` is always the top label, even when the distribution is
nearly flat across two or three labels.
**Solution**: gate on `.Confidence` before trusting `.Choice` alone (Step 3),
and fall back to `.Probabilities` when it's low.

### Problem: a `none`-style label never shows up in your metrics
**Symptom**: states that clearly don't fit any real category get force-fit
into one anyway.
**Cause**: `none` wasn't offered as a label, or was offered but handled the
same as every other label instead of as its own signal.
**Solution**: include an explicit "doesn't apply" label in `Criteria` when
that's a real possible outcome, and branch on it by name (Step 4).

## Next steps

- Read TypeSafe's own confidence documentation at
  [docs.typesafe.ai/confidence](https://docs.typesafe.ai/confidence) for how
  these numbers are computed.
- [Decode answers into your own struct](./decode-answers-into-your-own-struct.md)
  once you're ready to move off map lookups.

## See also

- [Response types](../reference/response-types.md)
- [Ask several questions in one request](./ask-several-questions-in-one-request.md)
- [Your First Judgment in Go](../tutorials/first-judgment-in-go.md)
