---
title: "How to Act on Probabilities and Confidence"
description: "Read NoulAnswer, ChoiceAnswer, and ScoreAnswer as probabilities rather than labels, pick your own thresholds, and handle a none-style label explicitly."
diataxis: how-to
weight: 70
---

# How to Act on Probabilities and Confidence

**Goal**: Turn `NoulAnswer`, `ChoiceAnswer`, and `ScoreAnswer` values into
decisions your code can act on, without treating a probability near the
middle as a third label or a confident answer as more certain than it is.

## Prerequisites

- A working `SystemOne` call and familiarity with the answer types — see
  [Your First Judgment in Go](../tutorials/first-judgment-in-go.md) and the
  [response types reference](../reference/response-types.md)
- TypeSafe's own confidence documentation at
  [docs.typesafe.ai/confidence](https://docs.typesafe.ai/confidence) covers
  how these numbers are produced; this page covers what to do with them in Go

## Steps

### 1. Treat `NoulAnswer.Noul` as a probability, not a yes/no

`Noul` is the probability, from 0 to 1, that the condition holds — not a
boolean and not a three-way "yes/no/maybe". A value near 0.5 means the model
is genuinely undecided about *this specific state*, not that the answer is
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
`0.99` — confidently billing. Against a deliberately ambiguous ticket that
mentioned both a payment and a login problem, the same question came back
`0.47` — inside the undecided band, and correctly so: the ticket really
doesn't lean either way.

### 2. Pick the threshold against your own data, not a universal number

There's no single "right" cutoff for `billingThreshold` above. A routing
decision that's cheap to get wrong (route to a queue a human can
re-triage) can use a looser threshold than one that's expensive to get wrong
(auto-refund a charge). Run representative states through the question, look
at the distribution of scores you actually get back, and set the threshold
— and the width of the "undecided" band — from that, not from a number that
felt right in the abstract.

### 3. Use `ChoiceAnswer.Confidence` to decide how much to trust `.Choice`

`Confidence` (0–1) summarizes how concentrated the label distribution is —
high when one label dominates, low when the model is split between two or
more. Don't act on `.Choice` alone without checking it:

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
`0.98`, with `probabilities: {"angry":0, "frustrated":0.01, "neutral":0.99}`
— a genuinely lopsided distribution, safe to trust directly. A lower
`Confidence` on a different state would mean the top two labels are close,
which `.Probabilities` makes visible even though `.Choice` alone would hide
it.

### 4. Handle a `none`-style label as its own branch

If your `Criteria` includes an explicit "doesn't apply" label — `none`,
`other`, whatever fits your domain — check for it by name instead of
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

A real run on a ticket that was just a thank-you message returned
`topic: "none"` at confidence `1.0` — the model was completely sure the
ticket didn't concern billing or login, which is a different (and more
useful) signal than "the model couldn't decide."

### 5. Read `ScoreAnswer` the same way: `Score`, `Confidence`, `Probabilities`

`Score` is the probability-weighted position on your rubric — not
necessarily an integer, since it's a weighted average across levels — and
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
`legend: {"0":"low","1":"medium","2":"high"}` — the model leaned toward
"high" but wasn't concentrated there, which `Score` alone (1.67, between
medium and high) shows but `Confidence` makes explicit: don't treat this one
as settled.

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
**Cause**: 0.5 isn't a "maybe" label baked into the type — it's the natural
midpoint of a probability, and treating it as meaningfully different from
0.49 or 0.51 overstates the model's precision there.
**Solution**: define an explicit undecided band (Step 1) and route it to a
fallback — a human queue, a default action, a second question — instead of
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
