---
title: "How Answers Are Decoded"
description: "Why decoding a System One answer is a two-pass process keyed on its wire type, and how unrecognized types and decode failures are handled."
diataxis: explanation
weight: 50
---

A `SystemOneResponse` carries a map of answers, and each answer's shape
depends on the kind of question it answers. A single `json.Unmarshal` cannot
turn that raw JSON into `NoulAnswer`, `ChoiceAnswer`, or `ScoreAnswer`
values, because the shape to decode into depends on a field the decoder has
not read yet. This page covers how `response.go` resolves that and what
follows from it: preserved unknown answers, deterministic error reporting,
and error messages in the API's vocabulary rather than Go's.

## The shape depends on the type

A noul answer's wire JSON is `{"type": "noul", "noul": 0.8}`. A choice
answer is `{"type": "choice", "choice": "angry", "confidence": 0.9,
"probabilities": {...}}`. A score answer adds a `legend`. Each type requires
a different set of fields. One Go struct carrying `json:"noul,omitempty"`
alongside `json:"choice,omitempty"` would decode all three, but an absent
`noul` field would then mean either that the answer is a choice answer or
that the response is malformed, and the struct could not distinguish the
two.

## Two passes: type first, then shape

`decodeAnswer` in `response.go` decodes twice. The first pass unmarshals
only a header:

```go
var head struct {
    Type string `json:"type"`
}
json.Unmarshal(raw, &head)
```

`encoding/json` ignores fields a struct doesn't declare, so this first pass
costs nothing beyond finding `"type"` and skips every other key in the
object. `decodeAnswer` then switches on `head.Type` and runs a second,
type-specific unmarshal into an anonymous struct shaped for exactly that
answer type: a `noul` object into `struct{ Noul *float64 }`, a `choice`
object into `struct{ Choice *string; Confidence *float64; Probabilities
map[string]float64 }`, and so on.

Each per-type struct uses pointer fields for its required values. A nil
pointer after the second unmarshal means the field was absent from the JSON;
a non-nil pointer to `0.0` means the server sent a `noul` of exactly `0.0`,
which passes. A nil required field produces a `ResponseValidationError`
naming that field's path, so a missing probability never decodes silently to
zero.

Decoding a type-tagged union in `encoding/json` takes these two passes,
short of guessing at the shape or requiring the caller to pre-declare which
type they expect. The first pass determines the shape; the second decodes
that shape and validates that it is complete.

## `UnknownAnswer`

The type switch in `decodeAnswer` has three known cases, `noul`, `choice`,
and `score`, plus a default. The default does not fail the decode:

```go
default:
    return UnknownAnswer{Type: head.Type, Raw: append(json.RawMessage(nil), raw...)}, nil
```

`UnknownAnswer` stores the wire type and a copy of the exact bytes the
server sent. Its `MarshalJSON` returns `Raw` unchanged, or a minimal
`{"type": a.Type}` when `Raw` is empty, which happens for an
`UnknownAnswer` built by hand rather than decoded. An answer of a type this
version of the library doesn't model round-trips byte for byte through a
decode and a re-marshal.

A proxy, a logging wrapper, or a cache receives a response, touches a few
fields, and passes it along. When the API adds a fourth answer type,
an application built against today's library passes that answer through a
marshal/unmarshal cycle unchanged rather than dropping or corrupting it.
Failing the decode, or decoding to a zero value and losing the original
bytes, would tie every caller's upgrade path to the server's release
schedule and the library's.

## Deterministic field paths

`decodeSystemOne` decodes the `answers` map's entries in sorted key order,
not Go's randomized map iteration order:

```go
keys := make([]string, 0, len(w.Answers))
for key := range w.Answers {
    keys = append(keys, key)
}
sort.Strings(keys)
for _, key := range keys {
    a, err := decodeAnswer(joinPath("answers", key), w.Answers[key])
    ...
}
```

Decoding stops at the first bad answer it encounters. The fixed iteration
order makes that the alphabetically first malformed key on every run. Go
randomizes map iteration order by design, so without the sort two runs
against the identical malformed response body could report two different
field paths. With it, a response whose `"a"` and `"z"` answers are both
malformed always reports `answers.a...`.

A bug report naming `answers.tone.confidence` describes the same failure
every time it is reproduced, and a test asserting on that field path does
not flake with map iteration order.

## `unmarshalAs`

`NoulAnswer`, `ChoiceAnswer`, and `ScoreAnswer` each also implement
`UnmarshalJSON` directly, for callers that already know which type they
expect, such as `SystemOneAs` decoding into a caller-defined struct field
typed as `typesafe.NoulAnswer`. All three route through one generic helper:

```go
func unmarshalAs[T Answer](b []byte, want string) (T, error) {
    var zero T
    a, err := decodeAnswer("", b)
    if err != nil {
        return zero, err
    }
    v, ok := a.(T)
    if !ok {
        return zero, &ResponseValidationError{FieldPath: "type", Err: errors.New("expected " + want + " answer, got " + a.AnswerType())}
    }
    return v, nil
}
```

`unmarshalAs` runs the same two-pass `decodeAnswer` and then asserts the
type. A caller that declared a field as `NoulAnswer` for a question key the
server answered with a choice gets `expected noul answer, got choice`. That
message uses the wire vocabulary, `"noul"` and `"choice"`, not the Go type
names `typesafe.NoulAnswer` and `typesafe.ChoiceAnswer`. Someone debugging a
live API response is reading `curl` output or a captured request body
carrying a `"type": "choice"` field, and the error names the same token they
are looking at. Go type names would need mapping back to the wire type
before the error pointed anywhere.

## `RawResponse`

`SystemOneResponse.Raw` is populated on every successful call, whatever the
typed decode did or did not capture:

```go
out.Raw = &RawResponse{Status: resp.StatusCode, Header: resp.Header.Clone(), Body: raw}
```

`RawResponse` holds the exact status, headers, and body bytes the server
returned, independent of how the typed decode into `Answers` went. It serves
the same purpose as `UnknownAnswer`: the typed model layers over the wire
format without replacing it. A caller that hits a decode limitation the
typed API doesn't handle, such as a field the library doesn't expose or a
response shape from a newer API version, parses `Raw.Body` directly and
makes no second network call.

## Related documentation

- For the full set of answer types, their wire shapes, and the decode-error
  determinism described above, see the
  [response types reference](../reference/response-types.md).
- For the error types raised during decoding, `ResponseValidationError` in
  particular, see [Errors and exit codes](../reference/errors-and-exit-codes.md).
- For a task-oriented walkthrough of decoding answers into your own struct
  via `SystemOneAs`, see
  [Decode answers into your own struct](../how-to/decode-answers-into-your-own-struct.md).
