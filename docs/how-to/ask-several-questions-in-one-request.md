---
title: "How to Ask Several Questions in One Request"
description: "Build a Questions map mixing Noul, Choice, and Score in one SystemOne call, and recognize when a follow-up question needs a second call instead."
diataxis: how-to
weight: 60
---

**Goal**: Put several independent questions about the same state into one
`Questions` map so they're answered in a single `SystemOne` call, and
recognize the point where a question depends on another's answer and needs a
second call instead.

## Prerequisites

- A working `SystemOne` call, see
  [Your First Judgment in Go](../tutorials/first-judgment-in-go.md)
- Familiarity with `Noul`, `Choice`, and `Score`, see the
  [question types reference](../reference/question-types.md)

## Steps

### 1. Collect every independent question into one `Questions` map

`Questions` is `map[string]Question`. Any mix of `Noul`, `Choice`, and
`Score` values (or pointers to them) can live in the same map, and they're
all answered from the same `state` in one HTTP call:

```go
questions := typesafe.Questions{
	"is_billing": typesafe.Noul{
		Instructions: "Is this ticket about a billing problem?",
	},
	"tone": typesafe.Choice{
		Instructions: "What is the customer's tone?",
		Criteria: map[string]typesafe.JSONContent{
			"neutral":    nil,
			"frustrated": nil,
			"angry":      nil,
		},
	},
	"urgency": typesafe.Score{
		Instructions: "How urgent is this ticket, from 0 (can wait) to 2 (needs immediate attention)?",
		Criteria: []typesafe.JSONContent{
			"can wait",
			"handle today",
			"needs immediate attention",
		},
	},
}
```

### 2. Choose identifiers for what they mean to your code, not the model

The map's keys (`"is_billing"`, `"tone"`, `"urgency"` above) name the
answers in the response. They are not shown to the model, so they carry no
instructional weight; the model only ever sees each question's
`Instructions` and `Criteria`. Pick keys that fit naturally in your own code
(`res.Nouls()["is_billing"]`), not keys that try to prime the model.

### 3. Call `SystemOne` once and read the typed answers back

```go
res, err := client.SystemOne(ctx, ticket, questions)
if err != nil {
	// handle err
}
isBilling := res.Nouls()["is_billing"].Noul
tone := res.Choices()["tone"]
urgency := res.Scores()["urgency"]
```

`Nouls()`, `Choices()`, and `Scores()` each partition `res.Answers` by
answer type, so you don't have to type-assert every entry yourself. See
[How to Act on Probabilities and Confidence](./act-on-probabilities-and-confidence.md)
for what to do with the values once you have them.

### 4. Use a second call when a question depends on another's answer

Every question in a `Questions` map is answered independently against the
same `state`. None of them can see another's answer, because there isn't a
sequencing or dependency concept inside one call. The model receives the
whole map and answers all of it at once. If what you want to ask next
depends on what came back (ask `category` only when `is_billing` is true,
say), that dependency has to live in your Go code between two calls, not
inside one `Questions` map:

```go
res1, err := client.SystemOne(ctx, ticket, questions)
if err != nil {
	// handle err
}
isBilling := res1.Nouls()["is_billing"].Noul

if isBilling > 0.5 {
	followUp := typesafe.Questions{
		"category": typesafe.Choice{
			Instructions: "What kind of billing issue is this?",
			Criteria: map[string]typesafe.JSONContent{
				"billing_error":    nil,
				"duplicate_charge": nil,
				"other":            nil,
			},
		},
	}
	res2, err := client.SystemOne(ctx, ticket, followUp)
	if err != nil {
		// handle err
	}
	category := res2.Choices()["category"]
	_ = category
}
```

When designing a `Questions` map, check what each question needs. Questions
that need only the original `state` go in one call. A question that needs an
answer you don't have yet goes in a second call.

### 5. Keep an eye on how many questions you're bundling into one call

Each call's question mix, the total count and the per-type breakdown,
reaches instrumentation as `RequestInfo.QuestionCount`, `.NoulCount`,
`.ChoiceCount`, and `.ScoreCount`. Nothing in the SDK caps how many
questions you put in one map. Use those counts from an `Instrumentation`
implementation or your own logging to track payload size and per-call
latency as a `Questions` map grows.

### 6. Mix in a `RawQuestion` for a shape this package doesn't model yet

`RawQuestion` implements `Question` too, so it drops into the same map
alongside typed values:

```go
questions := typesafe.Questions{
	"is_billing": typesafe.Noul{Instructions: "Is this ticket about a billing problem?"},
	"sentiment_v2": typesafe.RawQuestion{
		"type":   "sentiment_v2",
		"labels": []string{"positive", "negative", "mixed"},
	},
}
```

See
[How to Send Fields the SDK Does Not Model Yet](./send-fields-the-sdk-does-not-model-yet.md)
for what validation `RawQuestion` does and doesn't get.

## Verify it works

Running the call-1/call-2 program from Step 4 against a fake server that
returns a confident `is_billing` answer prints:

```
is_billing=0.93 tone=frustrated (confidence 0.88)
is_billing=0.93 -> asking follow-up: category
category=duplicate_charge (confidence 0.91)
```

The first line comes entirely out of call 1's single `Questions` map; the
follow-up only happens, and only asks `category`, because Go code inspected
`res1` first.

✅ Independent questions about the same state travel together in one call;
anything that depends on an answer waits for a second one.

## Troubleshooting

### Problem: a question seems to "know" the answer to another question in the same call
**Symptom**: you expected one question's instructions to reference another
question's answer, and it doesn't behave that way.
**Cause**: there's no ordering or data flow between entries in one
`Questions` map. The API answers the whole map against the same `state` in
one pass.
**Solution**: split the dependent question into a second `SystemOne` call, as
in Step 4, and build its `state` or `Instructions` from the first call's
result if needed.

### Problem: unsure whether to send five questions in one call or five separate calls
**Symptom**: no error, only a design question.
**Cause**: both are valid; the SDK doesn't push you either way.
**Solution**: default to one call for every question that only needs the
original `state`. Only split when a question's instructions would need to
reference an answer you don't have yet.

### Problem: a question type this page doesn't cover is rejected
**Symptom**: a `RawQuestion`'s `"type"` value is rejected, either by the SDK
or by the API.
**Cause**: `RawQuestion` skips this package's own validation beyond
requiring a non-empty string `"type"` field. Anything past that is the
API's call.
**Solution**: see
[How to Send Fields the SDK Does Not Model Yet](./send-fields-the-sdk-does-not-model-yet.md).

## Next steps

- [Act on probabilities and confidence](./act-on-probabilities-and-confidence.md)
  in the answers you get back.
- [Send fields the SDK does not model yet](./send-fields-the-sdk-does-not-model-yet.md)
  for the full `RawQuestion` and `WithExtraBody` story.

## See also

- [Your First Judgment in Go](../tutorials/first-judgment-in-go.md)
- [Question types](../reference/question-types.md)
- [Response types](../reference/response-types.md)
