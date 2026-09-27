---
title: "Span Attributes"
description: "Everything exported from the typesafe/otel package: New, Option, the WithRecordAnswers/WithRecordContent/WithTracerProvider options, TracerName, ProviderName, and every span attribute key constant and its value."
type: reference
---

# Span Attributes

Package: `github.com/therealbill/typesafe-go/otel` (import path `otel`).

Package otel instruments a `typesafe.Client` with OpenTelemetry traces.
Attach it with `typesafe.WithInstrumentation(otel.New())`. Each `SystemOne`
or `ListModels` call becomes a client span named `typesafe.system_one` or
`typesafe.list_models`, with one child HTTP span per attempt. State and
question content are never recorded unless `WithRecordContent` is set.

This package never installs a global tracer provider. Set one in your
application before building the client, or pass `WithTracerProvider`.

## TracerName

```go
const TracerName = "github.com/therealbill/typesafe-go"
```

TracerName is the instrumentation scope name.

## ProviderName

```go
const ProviderName = "typesafe"
```

ProviderName is the value of the `gen_ai.provider.name` attribute.

## New

```go
func New(opts ...Option) typesafe.Instrumentation
```

New returns an `Instrumentation` (documented on the client options and
environment reference) for `typesafe.WithInstrumentation`.

## Option

```go
type Option func(*instrumentation)
```

Option configures the instrumentation.

| Function | Signature | Description |
|---|---|---|
| `WithTracerProvider` | `func WithTracerProvider(tp trace.TracerProvider) Option` | Uses `tp` instead of the global provider. |
| `WithRecordContent` | `func WithRecordContent(maxBytes int) Option` | Records the request state and questions as JSON span attributes, each truncated to `maxBytes`. Off by default because state is often customer data. |
| `WithRecordAnswers` | `func WithRecordAnswers() Option` | Records each answer's value and confidence as `typesafe.answer.<key>.<field>` attributes. Off by default. |

## Span names

Each call to `SystemOne` or `ListModels` opens one client span, named
`"typesafe." + typesafe.RequestInfo.Operation`:

| Span name | Operation |
|---|---|
| `typesafe.system_one` | `SystemOne` calls (`RequestInfo.Operation == "system_one"`) |
| `typesafe.list_models` | `ListModels` calls (`RequestInfo.Operation == "list_models"`) |

Each HTTP attempt made while serving that call becomes a child span, produced
by wrapping the client's HTTP transport (`otelhttp`); those child spans carry
`otelhttp`'s own attributes, not the `Attr*` constants below.

## Attribute key constants

```go
const (
    AttrProviderName  = "gen_ai.provider.name"
    AttrSystem        = "gen_ai.system"
    AttrOperationName = "gen_ai.operation.name"
    AttrRequestModel  = "gen_ai.request.model"
    AttrResponseModel = "gen_ai.response.model"
    AttrInputTokens   = "gen_ai.usage.input_tokens"
    AttrOutputTokens  = "gen_ai.usage.output_tokens"
    AttrRequestID     = "typesafe.request_id"
    AttrQuestionCount = "typesafe.questions.count"
    AttrNoulCount     = "typesafe.questions.noul"
    AttrChoiceCount   = "typesafe.questions.choice"
    AttrScoreCount    = "typesafe.questions.score"
    AttrRetryAttempts = "typesafe.retry.attempts"
    AttrHTTPStatus    = "http.response.status_code"
    AttrErrorType     = "error.type"
    AttrState         = "typesafe.state"
    AttrQuestions     = "typesafe.questions"
)
```

Span attribute keys. The `gen_ai.*` keys follow the OpenTelemetry GenAI
semantic conventions; `gen_ai.system` is the older spelling and is set too.

The table below states, for each constant: its exact string value, which
span(s) carry it, when it is set, and the `typesafe.RequestInfo` /
`typesafe.RequestResult` field (or option) that populates it. `RequestStart`
in `otel/otel.go` sets the start-of-span attributes from the
`typesafe.RequestInfo` passed to it, and the end-of-span attributes from the
`typesafe.RequestResult` passed to the function it returns.

| Constant | Key | Span(s) | Set when | Populated from |
|---|---|---|---|---|
| `AttrProviderName` | `gen_ai.provider.name` | `typesafe.system_one`, `typesafe.list_models` | Always, at span start. | Literal `ProviderName` (`"typesafe"`). |
| `AttrSystem` | `gen_ai.system` | `typesafe.system_one`, `typesafe.list_models` | Always, at span start. | Literal `ProviderName` (`"typesafe"`). |
| `AttrOperationName` | `gen_ai.operation.name` | `typesafe.system_one`, `typesafe.list_models` | Always, at span start. | `RequestInfo.Operation` — the same string (`"system_one"` or `"list_models"`) used to build the span name `"typesafe." + info.Operation`. |
| `AttrRequestModel` | `gen_ai.request.model` | `typesafe.system_one`, `typesafe.list_models` | Always, at span start. | `RequestInfo.Model`. |
| `AttrQuestionCount` | `typesafe.questions.count` | `typesafe.system_one` only | At span start, only when `RequestInfo.Operation == "system_one"`. | `RequestInfo.QuestionCount`. |
| `AttrNoulCount` | `typesafe.questions.noul` | `typesafe.system_one` only | At span start, only when `RequestInfo.Operation == "system_one"`. | `RequestInfo.NoulCount`. |
| `AttrChoiceCount` | `typesafe.questions.choice` | `typesafe.system_one` only | At span start, only when `RequestInfo.Operation == "system_one"`. | `RequestInfo.ChoiceCount`. |
| `AttrScoreCount` | `typesafe.questions.score` | `typesafe.system_one` only | At span start, only when `RequestInfo.Operation == "system_one"`. | `RequestInfo.ScoreCount`. |
| `AttrState` | `typesafe.state` | `typesafe.system_one` only | At span start, only when `RequestInfo.Operation == "system_one"`, `WithRecordContent(maxBytes)` was configured with `maxBytes > 0`, and `span.IsRecording()` is true. | JSON encoding of `RequestInfo.State`, truncated to `maxBytes` bytes. |
| `AttrQuestions` | `typesafe.questions` | `typesafe.system_one` only | At span start, only when `RequestInfo.Operation == "system_one"`, `WithRecordContent(maxBytes)` was configured with `maxBytes > 0`, and `span.IsRecording()` is true. | JSON encoding of `RequestInfo.Questions`, truncated to `maxBytes` bytes. |
| `AttrRetryAttempts` | `typesafe.retry.attempts` | `typesafe.system_one`, `typesafe.list_models` | Always, at span end. | `RequestResult.Attempts`. |
| `AttrHTTPStatus` | `http.response.status_code` | `typesafe.system_one`, `typesafe.list_models` | At span end, only when `RequestResult.Status != 0`. | `RequestResult.Status`. |
| `AttrRequestID` | `typesafe.request_id` | `typesafe.system_one`, `typesafe.list_models` | At span end, only when `RequestResult.RequestID != ""`. | `RequestResult.RequestID`. |
| `AttrResponseModel` | `gen_ai.response.model` | `typesafe.system_one`, `typesafe.list_models` | At span end, only when `RequestResult.Model != ""`. | `RequestResult.Model`. |
| `AttrInputTokens` | `gen_ai.usage.input_tokens` | `typesafe.system_one`, `typesafe.list_models` | At span end, only when `RequestResult.Usage.InputTokens != nil`. | `*RequestResult.Usage.InputTokens`. |
| `AttrOutputTokens` | `gen_ai.usage.output_tokens` | `typesafe.system_one`, `typesafe.list_models` | At span end, only when `RequestResult.Usage.OutputTokens != nil`. | `*RequestResult.Usage.OutputTokens`. |
| `AttrErrorType` | `error.type` | `typesafe.system_one`, `typesafe.list_models` | At span end, only when `RequestResult.Err != nil`. | The Go type name of `RequestResult.Err`, with a leading `*` trimmed (for example `typesafe.APIError`). The span also records the error and sets its status to `codes.Error` with `RequestResult.Err.Error()`. |

`RequestStart` only marshals `RequestInfo.State`/`RequestInfo.Questions` to
JSON when the span will actually use the result: the check is
`if info.Operation == "system_one" && i.contentBytes > 0 && span.IsRecording()`.
When `span.IsRecording()` is false (for example, a sampler decided not to
record this trace), the marshaling is skipped entirely and neither attribute
is set, even though `WithRecordContent` is configured.

Truncation (`AttrState`, `AttrQuestions`): the value is JSON-marshaled; if
the marshaled string is longer than `maxBytes`, it is cut with
`truncateRuneSafe` and the suffix `...(truncated)` appended. `truncateRuneSafe`
walks the cut point back to the nearest preceding UTF-8 rune boundary rather
than cutting the byte slice directly, so truncated content is always valid
UTF-8 rather than potentially ending mid-character. If marshaling fails, the
value is `<unencodable <Go type>>`.

## Answer attributes (WithRecordAnswers)

When `WithRecordAnswers()` is set and `RequestResult.Response` is non-nil (a
successful `system_one` call), one additional, dynamically-keyed attribute
set per question identifier `<key>` in `RequestResult.Response.Answers` is
added at span end, in ascending key order:

| Answer type | Attributes set |
|---|---|
| `typesafe.NoulAnswer` | `typesafe.answer.<key>.noul` (float64) |
| `typesafe.ChoiceAnswer` | `typesafe.answer.<key>.choice` (string, capped — see below), `typesafe.answer.<key>.confidence` (float64) |
| `typesafe.ScoreAnswer` | `typesafe.answer.<key>.score` (float64), `typesafe.answer.<key>.confidence` (float64) |

`typesafe.UnknownAnswer` values do not produce answer attributes. These keys
are not `Attr*` constants; they are built as the literal prefix
`"typesafe.answer."` plus the question identifier plus a field suffix.

A `ChoiceAnswer`'s `Choice` value is truncated with `truncateRuneSafe` (the
same rune-safe helper used for `AttrState`/`AttrQuestions`) to at most
`maxAnswerChoiceBytes` (`= 256`) bytes before being set as the
`typesafe.answer.<key>.choice` attribute, bounding how much a pathologically
long label can bloat a span. `Noul` and `Score` values are numeric and are
not truncated; only the `.choice` string attribute is capped.
