---
title: "How Answers Are Decoded"
description: "Why decoding a System One answer is a two-pass process keyed on its wire type, and how unrecognized types and decode failures are handled."
diataxis: explanation
weight: 50
---

# How Answers Are Decoded

A `SystemOneResponse` carries a map of answers, and each answer can be a
different shape depending on what kind of question it answers. Turning the
raw JSON the server sends into `NoulAnswer`, `ChoiceAnswer`, or `ScoreAnswer`
values is not a single `json.Unmarshal` call — it can't be, because the shape
you need to decode into depends on a field you haven't read yet. This page
walks through how `response.go` resolves that, and what falls out of the
design: preserved unknown answers, deterministic error reporting, and error
messages that speak the API's vocabulary rather than Go's.

## The problem: the shape depends on the type

A noul answer's wire JSON looks like `{"type": "noul", "noul": 0.8}`. A
choice answer looks like `{"type": "choice", "choice": "angry", "confidence":
0.9, "probabilities": {...}}`. A score answer adds a `legend`. These aren't
variations on one struct — they're different sets of required fields
entirely, and a single Go struct with `json:"noul,omitempty"` alongside
`json:"choice,omitempty"` couldn't tell "this field is absent because it's a
choice answer" from "this field is absent and the response is malformed."

## Two passes: type first, then shape

`decodeAnswer` in `response.go` resolves this by decoding twice. The first
pass unmarshals only a header:

```go
var head struct {
    Type string `json:"type"`
}
json.Unmarshal(raw, &head)
```

`encoding/json` ignores fields a struct doesn't declare, so this first pass
costs nothing beyond finding `"type"` — every other key in the object is
simply skipped. Only once `head.Type` is known does `decodeAnswer` switch on
it and run a second, type-specific unmarshal into an anonymous struct shaped
for exactly that answer type: a `noul` object decodes into `struct{ Noul
*float64 }`, a `choice` object into `struct{ Choice *string; Confidence
*float64; Probabilities map[string]float64 }`, and so on. Each of those
per-type structs uses pointer fields for its required values specifically so
the decoder can tell "the field was absent from the JSON" (a nil pointer)
apart from "the field was present with a legitimate zero value" (a `noul` of
exactly `0.0` decodes to a non-nil pointer to `0.0`, and passes). A field
that comes back nil after the second unmarshal produces a
`ResponseValidationError` naming that field's path, rather than silently
treating a missing probability as zero.

This is the only way to decode a type-tagged union in `encoding/json`
without either guessing at the shape or requiring the caller to pre-declare
which type they expect. The first pass answers "what shape do I need"; the
second pass decodes that shape and validates it's complete.

## `UnknownAnswer`: preserving what the library doesn't know

The type switch in `decodeAnswer` has three known cases — `noul`, `choice`,
`score` — and a default. That default doesn't fail the decode:

```go
default:
    return UnknownAnswer{Type: head.Type, Raw: append(json.RawMessage(nil), raw...)}, nil
```

`UnknownAnswer` stores the wire type and a copy of the exact bytes the
server sent. Its `MarshalJSON` returns `Raw` unchanged — or, if `Raw` is
empty (an `UnknownAnswer` built by hand rather than decoded), a minimal
`{"type": a.Type}`. The result is that decoding an answer of a type this
version of the library doesn't model, and then re-marshaling it, round-trips
byte for byte.

That matters for anyone whose code path is "receive a response, maybe touch
a few fields, pass it along" — a proxy, a logging wrapper, a cache. If the
API adds a fourth answer type tomorrow, an application built against
today's library doesn't corrupt or silently drop that answer when it
happens to flow through a marshal/unmarshal cycle; it comes out exactly as
it went in. The alternative — failing to decode, or decoding to a zero value
and losing the original bytes — would make every caller's upgrade path
brittle to the server's own release schedule, not just the library's.

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

Decoding stops at the first bad answer it encounters, and because the
iteration order is fixed, "first" means the same thing every time: the
alphabetically first malformed key. Without the sort, two runs against the
identical malformed response body could report two different field paths,
because Go deliberately randomizes map iteration order between runs. With
it, a response with answers `"a"` and `"z"` both malformed always reports
`answers.a...`, never `answers.z...` on one run and `answers.a...` on the
next.

That determinism is worth the small fixed cost of sorting because of what it
protects downstream: a bug report that says "decoding fails at
`answers.tone.confidence`" describes the same failure every time it's
reproduced, and a test asserting on that field path doesn't flake depending
on map iteration order.

## The typed decode path: `unmarshalAs`

`NoulAnswer`, `ChoiceAnswer`, and `ScoreAnswer` each also implement
`UnmarshalJSON` directly, for callers who already know which type they
expect — for example `SystemOneAs` decoding into a caller-defined struct
field typed as `typesafe.NoulAnswer`. All three route through one generic
helper:

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

`unmarshalAs` runs the same two-pass `decodeAnswer` and then does a type
assertion. When the assertion fails — a caller declared a field as
`NoulAnswer` but the server sent a choice answer for that question key — the
error reads `expected noul answer, got choice`. Notice what that message
names: the wire vocabulary (`"noul"`, `"choice"`), not the Go type names
(`typesafe.NoulAnswer`, `typesafe.ChoiceAnswer`). Someone debugging a live
API response is looking at `curl` output or a captured request body with a
`"type": "choice"` field in it; a message built from that same vocabulary
tells them directly what to check next. A message built from Go type names
would require them to first map `typesafe.ChoiceAnswer` back to `"choice"`
in their head before the error meant anything.

## The raw body as escape hatch

Whatever the typed decode does or doesn't capture, `SystemOneResponse.Raw`
is populated on every successful call:

```go
out.Raw = &RawResponse{Status: resp.StatusCode, Header: resp.Header.Clone(), Body: raw}
```

`RawResponse` holds the exact status, headers, and body bytes the server
returned, independent of how the typed decode into `Answers` went. This is
the same instinct behind `UnknownAnswer`: the typed model is a convenience
over the wire format, not a replacement for it. A caller who hits a decode
limitation the typed API doesn't handle — a field the library doesn't
expose, a response shape from a newer API version — can always fall back to
`Raw.Body` and parse it themselves, without making a second network call.

## Related documentation

- For the full set of answer types, their wire shapes, and the decode-error
  determinism described above, see the
  [response types reference](../reference/response-types.md).
- For the error types raised during decoding — `ResponseValidationError` in
  particular — see [Errors and exit codes](../reference/errors-and-exit-codes.md).
- For a task-oriented walkthrough of decoding answers into your own struct
  via `SystemOneAs`, see
  [Decode answers into your own struct](../how-to/decode-answers-into-your-own-struct.md).
