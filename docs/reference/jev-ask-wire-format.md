---
title: "jev ask Wire Format"
description: "The exact JSON shapes jev ask reads and writes: the request document, each question type, the response document, each answer type, and the error envelope."
diataxis: reference
weight: 70
---

# jev ask Wire Format

This page documents the JSON document shapes for `jev ask`: the request
document it parses, the response document it prints, and the error envelope
it prints on failure. Flags, input modes, and subcommand-level
behavior are documented on the [jev CLI reference](jev-cli.md). Source:
`internal/cli/request.go` (`parseRequestJSON`, `parseQuestion`), `question.go`
(`Noul`, `Choice`, `Score`, their `MarshalJSON` methods), `response.go`
(`SystemOneResponse`, `decodeAnswer`), and `internal/cli/exit.go`
(`errorPayload`, `classify`).

## Request document

```json
{
  "state": "<any JSON value>",
  "questions": { "<id>": { "type": "...", "...": "..." } },
  "model": "<string, optional>"
}
```

| Field | Required | Type | Description |
|---|---|---|---|
| `state` | Yes | any JSON value | Decoded into `req.State` (a Go `any`). `parseRequestJSON` requires the key to be present; if absent the error is `invalid request: "state" is required`. |
| `questions` | Yes | object | Keyed by an arbitrary identifier string; each value is one question object (below). `parseRequestJSON` requires the key to be present; if absent the error is `invalid request: "questions" is required`. |
| `model` | No | string | If present, becomes `req.Model`. A non-string value produces `invalid request: model: <json error>`. |
| any other key | No | any JSON value | Collected into `req.Extra`, keyed by its own name, and passed to `typesafe.WithExtraBody(req.Extra)` when the request is sent, but only when `req.Extra` is non-empty. `parseRequestJSON` never puts `state`, `questions`, or `model` into `req.Extra`, so `WithExtraBody`'s own rejection of those three key names (see the [client options and environment reference](client-options-and-environment.md)) cannot be triggered through this path. |

`parseRequestJSON` decodes the top level into `map[string]json.RawMessage`
before processing individual fields, so malformed top-level JSON is reported
as `invalid request JSON: <json error>` before any field-specific error.

## Question objects

Every entry in `questions` must be a JSON object carrying a string `"type"`
field (`parseQuestion`). A missing or non-string `type` is
`invalid request: questions.<id>: must be an object with a "type"`; an empty
string `type` is `invalid request: questions.<id>.type: is required`.

`type` of `"noul"`, `"choice"`, or `"score"` is decoded with a strict decoder
(`json.NewDecoder(...).DisallowUnknownFields()`, applied recursively to
nested objects such as a `noul` question's `criteria`); an unrecognized field
anywhere in the object is rejected:
`invalid request: questions.<id>: json: unknown field "<field>"`. Any other
`type` value is decoded with a plain `json.Unmarshal` into a
`typesafe.RawQuestion` (`map[string]any`), a forward-compatible question
type not subject to the strict decoder, passed through unchanged.

Each parsed question is held as a `typesafe.Noul`, `typesafe.Choice`,
`typesafe.Score`, or `typesafe.RawQuestion` value. Before the request is sent,
the client library's `Client.SystemOne` re-encodes these values to JSON via
each type's own `MarshalJSON` method. The bytes transmitted follow
the wire shapes below, which can differ from the original input bytes (for
example, `criteria` appearing as an explicit JSON `null` when omitted from a
`choice` or `score` question, because `json.Marshal` of a nil Go map or slice
produces `null`, not `{}` or `[]`).

`Client.SystemOne` also runs `validateQuestions` on the parsed `Questions` map
immediately before the network call, after `jev ask`'s own JSON parsing has
already succeeded and after `Instrumentation.RequestStart` has been called.
The per-type limits below (`choice` label count, `score` level count) are
enforced there, as a `*typesafe.ValidationError`, not during
`parseRequestJSON`/`parseQuestion` itself: a `choice` or `score` question
whose `criteria` violates its limit parses without error and only fails once
the request is sent.

### noul

```go
type Noul struct {
    Instructions JSONContent
    Criteria     *NoulCriteria
}

type NoulCriteria struct {
    True  JSONContent
    False JSONContent
}
```

Input shape:

```json
{ "type": "noul", "instructions": "...", "criteria": { "true": "...", "false": "..." } }
```

`instructions` and `criteria` are both optional; within `criteria`, `true`
and `false` are each independently optional. No count or presence limit is
enforced on a `noul` question by either `parseQuestion` or
`validateQuestion` beyond `instructions` (when present) being text, an
object, or an array.

`Noul.MarshalJSON` produces `{"type":"noul"}` plus `instructions` when
non-nil, plus `criteria` when non-nil (itself containing `true`/`false` only
when each is non-nil). Omitted fields are absent from the object rather than
present as `null`.

### choice

```go
type Choice struct {
    Instructions JSONContent
    Criteria     map[string]JSONContent
}
```

Input shape:

```json
{ "type": "choice", "instructions": "...", "criteria": { "<label>": "<description or null>", "...": "..." } }
```

`instructions` is optional. `criteria` maps each label to a description, or
to JSON `null` to leave it undescribed; `parseQuestion` accepts `criteria`
being entirely absent (the resulting `typesafe.Choice` then has a nil
`Criteria` map). `validateQuestion`, run at request time as described above,
requires between 1 and 255 labels (`maxChoiceLabels = 255`). Zero labels
(including an absent or empty `criteria`) fails with
`&typesafe.ValidationError{Path: "questions.<id>.criteria", Err: errors.New("choice requires at least one label")}`,
and more than 255 fails naming the count.

`Choice.MarshalJSON` produces `{"type":"choice","criteria":<Criteria>}` plus
`instructions` when non-nil. `criteria` is always present in the marshaled
object, even when `Criteria` is nil (it then marshals as `null`).

### score

```go
type Score struct {
    Instructions JSONContent
    Criteria     []JSONContent
}
```

Input shape:

```json
{ "type": "score", "instructions": "...", "criteria": ["<level 0 description>", "<level 1 description>", "..."] }
```

`instructions` is optional. `criteria` is an ordered array where index `i`
describes score level `i`; `parseQuestion` accepts `criteria` being entirely
absent (the resulting `typesafe.Score` then has a nil `Criteria` slice) and
does not itself check how many entries it holds. `validateQuestion`, run at
request time as described above, requires between `minScoreLevels = 2` and
`maxScoreLevels = 10` entries. A count outside that range fails with
`&typesafe.ValidationError{Path: "questions.<id>.criteria", Err: fmt.Errorf("score requires between 2 and 10 levels, got <n>")}`.
Each individual level value is validated as required text, an object, or an
array (not optional, unlike `noul`/`choice` content).

`Score.MarshalJSON` produces `{"type":"score","criteria":<Criteria>}` plus
`instructions` when non-nil. As with `choice`, `criteria` is always present,
marshaling as `null` when `Criteria` is nil.

### Any other type

A `type` other than `"noul"`, `"choice"`, or `"score"` is decoded as-is into a
`typesafe.RawQuestion` (`map[string]any`) with a plain `json.Unmarshal`. No
field is rejected, no strict decoding applies, and no library-side validation
runs beyond confirming `type` is a non-empty string (already guaranteed by
`parseQuestion` before the value becomes a `RawQuestion`). It is sent to the
API unchanged.

## Response document

```go
type SystemOneResponse struct {
    Model     string            `json:"model"`
    Answers   map[string]Answer `json:"answers"`
    Usage     Usage             `json:"usage"`
    RequestID string            `json:"request_id,omitempty"`
}

type Usage struct {
    InputTokens  *int `json:"input_tokens"`
    OutputTokens *int `json:"output_tokens"`
}
```

```json
{
  "model": "...",
  "answers": { "<id>": { "type": "noul|choice|score|...", "...": "..." } },
  "usage": { "input_tokens": null, "output_tokens": null },
  "request_id": "..."
}
```

`Usage.InputTokens` and `Usage.OutputTokens` are pointers, so a field the
server omits decodes as Go `nil` (JSON `null`) rather than as `0`.
`request_id` is omitted entirely from the marshaled response when empty.

Each entry in `answers` is decoded by `decodeAnswer` according to its own
`"type"` field:

| Type | Go type | Wire fields |
|---|---|---|
| `"noul"` | `NoulAnswer` | `noul` (float64, required) |
| `"choice"` | `ChoiceAnswer` | `choice` (string, required), `confidence` (float64, required), `probabilities` (object of label to float64; defaults to `{}` if absent) |
| `"score"` | `ScoreAnswer` | `score` (float64, required), `confidence` (float64, required), `legend` (object of level index string to description; defaults to `{}` if absent), `probabilities` (object of level index string to float64; defaults to `{}` if absent) |
| anything else | `UnknownAnswer` | Preserved as the raw JSON bytes of the whole answer object (`UnknownAnswer.Raw`); `UnknownAnswer.MarshalJSON` returns those raw bytes unchanged when re-encoded, so an answer type this package does not model round-trips as-is. |

A missing `"type"` on an answer, or a missing required field for a
recognized type, is a `*typesafe.ResponseValidationError` naming the dotted
field path (for example `answers.tone.confidence`). This is a decode-time
failure of the response, distinct from the request-side
`*typesafe.ValidationError`s described above.

## Error envelope

On any failure, `jev ask` (and every other `jev` subcommand) writes one JSON
object to stdout, from `internal/cli/exit.go`'s `errorPayload`:

```go
type errorPayload struct {
    Kind      string `json:"kind"`
    Status    int    `json:"status,omitempty"`
    RequestID string `json:"request_id,omitempty"`
    Message   string `json:"message"`
}
```

```json
{ "error": { "kind": "...", "status": 0, "request_id": "...", "message": "..." } }
```

`status` and `request_id` are omitted (`json:",omitempty"`) when zero or
empty, respectively. `kind` is one of the strings returned by `classify()`:
`interrupted`, `usage`, `validation`, `rate_limit`, `auth`, `server`,
`request`, `connection`, `invalid_response`, or `internal`. The full mapping
from error condition to `kind` and to the process exit code is documented on
the [errors and exit codes reference](errors-and-exit-codes.md).

## Worked example

`testdata/request.json` and `testdata/systemone_ok.json` are this project's
own test fixtures, captured from the live API. The request:

```json
{"model":"jev-latest","state":{"ticket":"I was charged twice this month and nobody answers my emails. Fix it now."},"questions":{"billing":{"type":"noul","instructions":"Is `ticket` about a billing problem?"},"tone":{"type":"choice","instructions":"What is the tone of `ticket`?","criteria":{"calm":"Polite and patient","angry":"Frustrated or hostile","neutral":null}},"urgency":{"type":"score","instructions":"How urgent is `ticket`?","criteria":["Not urgent at all","Somewhat urgent","Very urgent"]}}}
```

produces the recorded response:

```json
{"model":"jev-1.13.0","answers":{"billing":{"type":"noul","noul":0.99},"tone":{"type":"choice","choice":"angry","confidence":1.0,"probabilities":{"neutral":0.0,"angry":1.0,"calm":0.0}},"urgency":{"type":"score","score":1.9,"confidence":0.85,"legend":{"0":"Not urgent at all","1":"Somewhat urgent","2":"Very urgent"},"probabilities":{"0":0.0,"1":0.1,"2":0.9}}},"usage":{"input_tokens":407,"output_tokens":72}}
```

The recorded response carries no `request_id` field, so `SystemOneResponse.RequestID` decodes as the empty string and would itself be omitted if this response were re-marshaled.
