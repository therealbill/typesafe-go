---
title: "Response Types"
description: "The Answer interface and its implementations (NoulAnswer, ChoiceAnswer, ScoreAnswer, UnknownAnswer), Usage, RawResponse, SystemOneResponse, ListModelsResponse, ModelMetadata, and SystemOneAs."
diataxis: reference
weight: 30
---

# Response Types

Package: `github.com/therealbill/typesafe-go` (import path `typesafe`).

## Answer

```go
type Answer interface {
    // AnswerType returns the wire "type" of the answer.
    AnswerType() string
}
```

Answer is one decoded answer: `NoulAnswer`, `ChoiceAnswer`, `ScoreAnswer`, or
`UnknownAnswer` for types this package does not model.

### Decode error determinism

Answers are decoded in sorted (alphabetical) key order, not Go's randomized
map iteration order. When a response has more than one malformed answer,
decoding stops at the first one encountered in that sorted order, so the
resulting `*ResponseValidationError`'s `FieldPath` is always the alphabetically
first bad key, consistently across repeated runs.

## NoulAnswer

```go
type NoulAnswer struct {
    Noul float64
}
```

NoulAnswer is the probability, from 0 to 1, that the condition holds.

| Method | Signature | Description |
|---|---|---|
| `AnswerType` | `func (NoulAnswer) AnswerType() string` | Returns `"noul"`. |
| `MarshalJSON` | `func (a NoulAnswer) MarshalJSON() ([]byte, error)` | Encodes the answer with its wire `"type"`. |
| `UnmarshalJSON` | `func (a *NoulAnswer) UnmarshalJSON(b []byte) error` | Decodes a noul answer and verifies its type. |

Wire shape:

```json
{ "type": "noul", "noul": <float64> }
```

## ChoiceAnswer

```go
type ChoiceAnswer struct {
    Choice        string
    Confidence    float64
    Probabilities map[string]float64
}
```

ChoiceAnswer is the selected label with the probability of each label and
the confidence, which summarizes how concentrated the distribution is.

| Method | Signature | Description |
|---|---|---|
| `AnswerType` | `func (ChoiceAnswer) AnswerType() string` | Returns `"choice"`. |
| `MarshalJSON` | `func (a ChoiceAnswer) MarshalJSON() ([]byte, error)` | Encodes the answer with its wire `"type"`. |
| `UnmarshalJSON` | `func (a *ChoiceAnswer) UnmarshalJSON(b []byte) error` | Decodes a choice answer and verifies its type. |

Wire shape:

```json
{
  "type": "choice",
  "choice": <string>,
  "confidence": <float64>,
  "probabilities": { "<label>": <float64>, ... }
}
```

When decoding, a missing `"probabilities"` field is populated as an empty
map rather than nil.

## ScoreAnswer

```go
type ScoreAnswer struct {
    Score         float64
    Confidence    float64
    Legend        map[string]JSONContent
    Probabilities map[string]float64
}
```

ScoreAnswer is the probability-weighted score with the rubric legend, the
probability of each level keyed by its index as a string, and confidence.

| Method | Signature | Description |
|---|---|---|
| `AnswerType` | `func (ScoreAnswer) AnswerType() string` | Returns `"score"`. |
| `MarshalJSON` | `func (a ScoreAnswer) MarshalJSON() ([]byte, error)` | Encodes the answer with its wire `"type"`. |
| `UnmarshalJSON` | `func (a *ScoreAnswer) UnmarshalJSON(b []byte) error` | Decodes a score answer and verifies its type. |

Wire shape:

```json
{
  "type": "score",
  "score": <float64>,
  "confidence": <float64>,
  "legend": { "<index>": <JSONContent>, ... },
  "probabilities": { "<index>": <float64>, ... }
}
```

When decoding, a missing `"legend"` or `"probabilities"` field is populated
as an empty map rather than nil.

## UnknownAnswer

```go
type UnknownAnswer struct {
    Type string
    Raw  json.RawMessage
}
```

UnknownAnswer preserves an answer whose type this package does not know.

| Method | Signature | Description |
|---|---|---|
| `AnswerType` | `func (a UnknownAnswer) AnswerType() string` | Returns the wire type of the unknown answer (`Type`). |
| `MarshalJSON` | `func (a UnknownAnswer) MarshalJSON() ([]byte, error)` | Returns the raw answer unchanged. |
| `UnmarshalJSON` | `func (a *UnknownAnswer) UnmarshalJSON(b []byte) error` | Records the answer's `type` into `Type` and keeps the input bytes verbatim in `Raw`. |

MarshalJSON behavior: if `Raw` is empty, it returns `{"type": <Type>}`;
otherwise it returns `Raw` unchanged. Because `UnmarshalJSON` stores the input
bytes unchanged in `Raw`, decoding an `UnknownAnswer` and then re-marshaling
it round-trips byte-for-byte.

## Usage

```go
type Usage struct {
    InputTokens  *int `json:"input_tokens"`
    OutputTokens *int `json:"output_tokens"`
}
```

Usage is the token usage reported by the API. Fields are nil when absent.

## RawResponse

```go
type RawResponse struct {
    Status int
    Header http.Header
    Body   []byte
}
```

RawResponse is the HTTP response behind a decoded result.

## SystemOneResponse

```go
type SystemOneResponse struct {
    Model     string            `json:"model"`
    Answers   map[string]Answer `json:"answers"`
    Usage     Usage             `json:"usage"`
    RequestID string            `json:"request_id,omitempty"`
    Raw       *RawResponse      `json:"-"`
}
```

SystemOneResponse is the decoded result of a `SystemOne` call.

| Method | Signature | Description |
|---|---|---|
| `Nouls` | `func (r *SystemOneResponse) Nouls() map[string]NoulAnswer` | Returns the noul answers keyed by question identifier. |
| `Choices` | `func (r *SystemOneResponse) Choices() map[string]ChoiceAnswer` | Returns the choice answers keyed by question identifier. |
| `Scores` | `func (r *SystemOneResponse) Scores() map[string]ScoreAnswer` | Returns the score answers keyed by question identifier. |
| `UnmarshalJSON` | `func (r *SystemOneResponse) UnmarshalJSON(b []byte) error` | Decodes a response body, or a body previously produced by `json.Marshal` on a `SystemOneResponse`. |

## ListModelsResponse

```go
type ListModelsResponse struct {
    Models    []ModelMetadata `json:"models"`
    RequestID string          `json:"request_id,omitempty"`
    Raw       *RawResponse    `json:"-"`
}
```

ListModelsResponse is the decoded result of a `ListModels` call.

## ModelMetadata

```go
type ModelMetadata struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    ReleaseDate string `json:"release_date"`
}
```

ModelMetadata describes one model available to the account.

| Field | Type | Description |
|---|---|---|
| `Name` | `string` | Model name. |
| `Description` | `string` | Model description. |
| `ReleaseDate` | `string` | The timestamp string reported by the API. |

## Response size limit

A response body over 16 MiB is not decoded. `SystemOne` and `ListModels`
return `&typesafe.ResponseValidationError{FieldPath: "", Err: errors.New("response exceeds 16 MiB")}`
instead. The check (in `transport.go`) reads one byte past the 16 MiB cap to
detect an oversized body rather than silently truncating it into malformed
JSON. This error carries no HTTP status: the underlying `*http.Response` is
not returned to the caller, so it is never seen as an `*APIError`.

## SystemOneAs

```go
func SystemOneAs[T any](ctx context.Context, c *Client, state any, questions Questions, opts ...RequestOption) (T, *SystemOneResponse, error)
```

SystemOneAs calls `SystemOne` and decodes the answers into `T`, a struct
whose fields are `NoulAnswer`, `ChoiceAnswer`, or `ScoreAnswer` (or pointers
to them) tagged with the question identifiers:

```go
type Triage struct {
    Billing typesafe.NoulAnswer   `json:"billing"`
    Tone    typesafe.ChoiceAnswer `json:"tone"`
}
```

The full response is returned alongside so usage and request ID are not
lost. On a request error the response is nil. On a decode error the response
is returned with the error.
