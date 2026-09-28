---
title: "How to Send Fields the SDK Does Not Model Yet"
description: "Use RawQuestion and WithExtraBody to reach API fields this package doesn't have typed support for, and know exactly which parts of a request stay validated."
diataxis: how-to
weight: 80
---

# How to Send Fields the SDK Does Not Model Yet

**Goal**: Send a question shape or a top-level request field that
`typesafe-go` doesn't model with a Go type yet, from both the Go SDK
(`RawQuestion`, `WithExtraBody`) and `jev ask`'s JSON mode, and know exactly
where validation still applies and where it doesn't.

## Prerequisites

- A working `SystemOne` call, see
  [Your First Judgment in Go](../tutorials/first-judgment-in-go.md)
- Familiarity with `Questions` and the typed question kinds, see
  [Ask several questions in one request](./ask-several-questions-in-one-request.md)
  and the [question types reference](../reference/question-types.md)

## Steps

### 1. Use `RawQuestion` for a question type this package doesn't model

`RawQuestion` is `map[string]any`, sent to the API unchanged. The only
validation this package applies is that it carries a non-empty string
`"type"` field. Everything else is the API's call, not this package's:

```go
questions := typesafe.Questions{
	"is_billing": typesafe.Noul{
		Instructions: "Is this ticket about a billing problem?",
	},
	"sentiment_v2": typesafe.RawQuestion{
		"type":         "sentiment_v2",
		"instructions": "Overall sentiment of the message.",
		"labels":       []string{"positive", "negative", "mixed"},
	},
}
```

It drops straight into the same `Questions` map as typed questions, as
covered in
[Ask several questions in one request](./ask-several-questions-in-one-request.md).

### 2. Use `WithExtraBody` for a top-level request field this package doesn't model

`WithExtraBody` merges a map into the top level of the request body, for a
single call, alongside the SDK's own `state`, `model`, and `questions`
fields:

```go
res, err := client.SystemOne(ctx, state, questions,
	typesafe.WithExtraBody(map[string]any{"routing_tag": "priority-support"}),
)
```

Running this against a fake server and printing what the server
received shows the shallow merge:

```
server received body: {"model":"jev-latest","questions":{"is_billing":{"instructions":"Is this about billing?","type":"noul"}},"routing_tag":"priority-support","state":"I was charged twice."}
```

The map passed to `WithExtraBody` is copied at construction time, so
mutating it afterward doesn't change a call already built with it.

### 3. Know which three field names `WithExtraBody` will always reject

`state`, `model`, and `questions` are set by the client itself and cannot be
overridden through `WithExtraBody`. This is checked before any network
call:

```go
_, err := client.SystemOne(ctx, state, questions,
	typesafe.WithExtraBody(map[string]any{"model": "not-allowed"}),
)
// err is a *typesafe.ValidationError with Path "extra_body.model"
```

That check runs even when the client is pointed at an address nothing is
listening on, proving it never gets as far as opening a connection:

```
Path:  extra_body.model
Error: typesafe: invalid request: extra_body.model: field is set by the client and must not be overridden
```

### 4. Know the three-way validation split in `jev ask`'s JSON mode

`jev ask`'s JSON input mode (`{"state": ..., "questions": {...}, "model":
...}` plus anything else) treats three kinds of unrecognized content
differently:

- An unrecognized **top-level** field passes through unchanged, becoming an
  extra body field (fed to `WithExtraBody` internally).
- A question whose `"type"` isn't `noul`, `choice`, or `score` passes
  through unchanged as a `RawQuestion`.
- An unrecognized field **inside** a `noul`, `choice`, or `score` object is
  rejected client-side, before any request is sent. `jev` decodes those
  three question shapes strictly (`(*json.Decoder).DisallowUnknownFields()`),
  so a typo or an extra key inside one of them is a JSON decode error, not a
  pass-through.

A request combining the first two, an unrecognized top-level field
(`routing_tag`) and a question with an unrecognized `type`
(`sentiment_v2`), passes `jev`'s own validation entirely and reaches the
API, which then makes its own decision:

```json
{
  "state": "Ticket: \"My invoice for last month shows a charge I don't recognize.\"",
  "questions": {
    "is_billing": { "type": "noul", "instructions": "Is this ticket about a billing or invoice problem?" },
    "sentiment_raw": { "type": "sentiment_v2", "instructions": "Overall sentiment of the message." }
  },
  "routing_tag": "tier-1-support"
}
```

```
$ ./bin/jev ask -f request.json --pretty
{
  "error": {
    "kind": "request",
    "status": 400,
    "request_id": "req_01a0e93e58dd7500a89974c792038021",
    "message": "api_usage_error: Invalid request."
  }
}
```

That `"kind": "request"` and a populated `request_id` show that `jev`
sent this request and the *API* rejected it (exit code 4, per the
[errors and exit codes reference](../reference/errors-and-exit-codes.md)).
This particular API account doesn't recognize `routing_tag` or the
`sentiment_v2` question type, but `jev` itself raised no objection to
either. A gateway or a future API version that *does* recognize one or both
would accept the same request unchanged.

### 5. See the client-side rejection for a bogus field inside a typed question

Contrast that with a field misplaced inside a `noul` object:

```json
{
  "state": "Ticket: \"My invoice for last month shows a charge I don't recognize.\"",
  "questions": {
    "is_billing": {
      "type": "noul",
      "instructions": "Is this ticket about a billing or invoice problem?",
      "confidence_threshold": 0.8
    }
  }
}
```

```
$ ./bin/jev ask -f request.json --pretty
{
  "error": {
    "kind": "usage",
    "message": "invalid request: questions.is_billing: json: unknown field \"confidence_threshold\""
  }
}
```

No `request_id`, `"kind": "usage"`, and exit code 1 mean this never left the
process. `confidence_threshold` isn't a real field of `noul`, `choice`, or
`score` in this SDK. There's no equivalent of `RawQuestion`'s leniency once
you're inside one of the three typed shapes.

## Verify it works

The three real runs above cover all three cases:

| Case | Where it's checked | `kind` | `request_id` |
|---|---|---|---|
| Unrecognized top-level field | API | `"request"` | present |
| Question with unrecognized `type` | API (as `RawQuestion`) | `"request"` | present |
| Unrecognized field inside `noul`/`choice`/`score` | `jev` itself | `"usage"` | absent |

✅ Top-level fields and whole question shapes reach the API unfiltered;
fields inside a typed question do not.

## Troubleshooting

### Problem: `WithExtraBody` field never shows up in the request
**Symptom**: the server doesn't see a field you passed to `WithExtraBody`.
**Cause**: the map was mutated after being passed to `WithExtraBody`. The
option copies it once, at construction time, not on every call.
**Solution**: build a fresh map (or a fresh call to `WithExtraBody`) for
each request instead of reusing and mutating one.

### Problem: `*typesafe.ValidationError` with `Path` like `"extra_body.state"`
**Symptom**: a call using `WithExtraBody` fails immediately with a
`ValidationError`.
**Cause**: the map passed to `WithExtraBody` used one of the three reserved
keys: `state`, `model`, or `questions`.
**Solution**: rename the field, or set it through the SDK's own parameters
(`state`, `WithRequestModel`, `questions`) instead.

### Problem: `jev ask` rejects a field you expected to pass through
**Symptom**: `"kind": "usage"` with a `json: unknown field` message.
**Cause**: the field is inside a `noul`, `choice`, or `score` question
object, where `jev` decodes strictly. This is the one place pass-through
doesn't apply.
**Solution**: if the API needs that field on a typed question,
there's no way to send it through `jev ask`'s flag or typed-JSON path today;
open an issue, or send it as a fully custom `RawQuestion`/raw JSON document
instead of a `noul`/`choice`/`score` object.

## Next steps

- See the exact JSON shapes `jev ask` accepts in the
  [jev ask wire format reference](../reference/jev-ask-wire-format.md).
- [Ask several questions in one request](./ask-several-questions-in-one-request.md)
  for how `RawQuestion` fits alongside typed questions.

## See also

- [Question types](../reference/question-types.md)
- [Client options and environment](../reference/client-options-and-environment.md)
- [Errors and exit codes](../reference/errors-and-exit-codes.md)
