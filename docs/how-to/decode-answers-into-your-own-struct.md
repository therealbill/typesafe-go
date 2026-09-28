---
title: "How to Decode Answers into Your Own Struct"
description: "Use SystemOneAs to decode SystemOne answers directly into a struct you define, choose between pointer and value answer fields, and handle ResponseValidationError."
diataxis: how-to
weight: 20
---

# How to Decode Answers into Your Own Struct

**Goal**: Replace `res.Nouls()["id"]`-style map lookups with a struct of your
own that `SystemOneAs[T]` decodes directly, and handle the validation errors
decoding can produce.

## Prerequisites

- A working `SystemOne` call and familiarity with `Questions`, see
  [Your First Judgment in Go](../tutorials/first-judgment-in-go.md), which
  introduces `SystemOneAs` for a single always-present field
- The [response types reference](../reference/response-types.md) for the
  full shape of `NoulAnswer`, `ChoiceAnswer`, and `ScoreAnswer`

## Steps

### 1. Define a struct tagged with your question identifiers

`SystemOneAs[T]` decodes the response's `answers` object into `T` using
ordinary `encoding/json` tags. Each tag value must match the identifier you
used in `Questions`:

```go
type ReviewTriage struct {
	Spam *typesafe.NoulAnswer  `json:"spam"`
	Tone typesafe.ChoiceAnswer `json:"tone"`
}
```

### 2. Call `SystemOneAs[T]`

```go
triage, resp, err := typesafe.SystemOneAs[ReviewTriage](ctx, client, state, typesafe.Questions{
	"spam": typesafe.Noul{Instructions: "Is this review spam?"},
	"tone": typesafe.Choice{
		Instructions: "What is the tone of this review?",
		Criteria:     map[string]typesafe.JSONContent{"angry": nil, "neutral": nil, "happy": nil},
	},
})
if err != nil {
	// handled in Step 4
}
```

### 3. Choose pointer vs. value fields

Both a pointer field and a value field decode successfully even when their
question is missing from the response. Neither causes an error on its own.
The difference is what you can tell afterward:

- A **pointer** field (`*typesafe.NoulAnswer`) stays `nil` when its question
  is absent, so `if triage.Spam == nil` distinguishes "the model didn't
  answer this" from "the model answered with a value near zero." Use it for
  a question that might be left out of a particular response.
- A **value** field (`typesafe.ChoiceAnswer`) silently decodes to its Go
  zero value (`Choice: ""`, `Confidence: 0`) if its question is ever absent,
  indistinguishable from a real zero-ish answer. Use it only for a
  question you're confident the response always includes, since you can't
  detect its absence afterward.

### 4. Handle `*ResponseValidationError` via `errors.As`

Decoding can still fail. For example, a question's answer comes back as the
wrong type, or a required answer subfield is missing. Both surface as
`*typesafe.ResponseValidationError`:

```go
var validationErr *typesafe.ResponseValidationError
if errors.As(err, &validationErr) {
	fmt.Println("field:", validationErr.FieldPath)
	fmt.Println("error:", validationErr.Error())
}
```

`FieldPath` and `Error()` carry different information depending on what kind
of validation failed, so check both. A type mismatch (your struct expects
`NoulAnswer` but the server answered `"type":"choice"`) sets `FieldPath` to
the constant string `"type"`. It never names the struct field or the
question identifier that failed. The useful diagnostic lives in the error
message: `Error()` names the expected and actual answer
type, for example:

```
typesafe: invalid response field "type": expected noul answer, got choice
```

The accompanying `*SystemOneResponse` is still non-nil for this case,
because decoding into `T` is what failed, not the initial decode of the
response itself. A malformed answer body (a missing required subfield
anywhere in the response, even for a question your struct doesn't ask for)
reports a fully qualified `FieldPath` like `"answers.tone.confidence"`
instead, and the response is `nil`, because `SystemOne` itself failed to
decode. Check `resp == nil` before using it.

### 5. Keep the returned `*SystemOneResponse` for `Usage` and `RequestID`

`SystemOneAs` returns `T` *and* the full response. Don't discard the second
value with `_` unless you truly don't need it:

```go
fmt.Printf("Usage: input=%d output=%d\n", *resp.Usage.InputTokens, *resp.Usage.OutputTokens)
fmt.Printf("RequestID: %s\n", resp.RequestID)
```

Decoding into `T` alone keeps only what your struct's fields ask for.
`Usage` and `RequestID` live on `*SystemOneResponse`, not on `T`.

## Verify it works

A fake server returns a canned response with `RequestID` set via the
`x-typesafe-request-id` response header. The real client
always reads it from there, not from a `request_id` field in the body. Running the
code from Steps 1, 2, and 5 against it prints:

```
Spam: 0.87
Tone: angry (confidence 0.73)
Usage: input=128 output=42
RequestID: req_demo123
```

✅ The struct is populated straight from the response, with `Usage` and
`RequestID` still reachable through the second return value.

## Troubleshooting

### Problem: a pointer field is `nil` and you expected an answer
**Symptom**: `triage.Spam` is `nil` after a successful decode (`err ==
nil`).
**Cause**: the question identifier tagging that field wasn't present in the
response's `answers` object. This is not an error, by design.
**Solution**: check the identifier against what you put in
`Questions`, and confirm the model was asked that question in this call.

### Problem: `*ResponseValidationError` but `resp` is `nil`
**Symptom**: code that assumes `resp` is always usable after `SystemOneAs`
panics on a nil dereference.
**Cause**: a validation failure inside `SystemOne`'s own decode (for
example, a missing required subfield anywhere in the payload) leaves the
whole response undecoded, so `SystemOneAs` returns `nil` for it.
**Solution**: always check `resp == nil` before reading `resp.Usage` or
`resp.RequestID`, even on error paths.

### Problem: `FieldPath` is only `"type"`, not the question identifier
**Symptom**: a validation error's `FieldPath` doesn't say which question
failed.
**Cause**: a type-mismatch error is detected inside the answer type's own
`UnmarshalJSON`, before the enclosing question identifier is known to it.
**Solution**: log the full error string (`validationErr.Error()`), which
does include the surrounding context, alongside `FieldPath`.

## See also

- [Your First Judgment in Go](../tutorials/first-judgment-in-go.md)
- [Response types](../reference/response-types.md)
- [Question types](../reference/question-types.md)
