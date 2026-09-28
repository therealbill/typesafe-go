---
title: "The Instrumentation Hook"
description: "How the Instrumentation interface observes a call's full lifecycle, including validation failures and panics, without the core package knowing what a span is."
diataxis: explanation
weight: 60
---

# The Instrumentation Hook

`SystemOne` and `ListModels` both call `c.startInstrument` before they do
anything else. This page describes what that hook receives, when it fires
relative to the rest of the call, and how the `otel` subpackage turns it
into OpenTelemetry spans.

## The Instrumentation interface

The root `typesafe` package contains no reference to OpenTelemetry.
`instrument.go` defines the interface the client calls instead:

```go
type Instrumentation interface {
    RequestStart(ctx context.Context, info RequestInfo) (context.Context, func(RequestResult))
    Transport(rt http.RoundTripper) http.RoundTripper
}
```

The client calls this interface without knowing what implements it.
[Why the core is stdlib-only](why-the-core-is-stdlib-only.md) covers that
boundary: a caller who never wires up `typesafe/otel` never compiles in
OpenTelemetry's dependency graph, and the core package stays testable
against plain fakes. The rest of this page covers the hook's own shape:
what it is told, when it is called, and what a non-tracing implementation
of it looks like.

## What `RequestStart` receives: `RequestInfo`

```go
type RequestInfo struct {
    Operation string
    Model     string
    QuestionCount int
    NoulCount     int
    ChoiceCount   int
    ScoreCount    int
    State     any
    Questions Questions
}
```

`Operation` is `"system_one"` or `"list_models"`. The four count fields come
from `countQuestions`, which walks the `Questions` map once and tallies how
many are `Noul`, `Choice`, or `Score`, including the pointer forms and
`RawQuestion` values whose `"type"` field matches one of those three
strings. Those counts are cheap, non-sensitive shape information an
implementation can record unconditionally. `State` and `Questions` carry the
actual request payload, and the field comments in `instrument.go` state the
rule: "Instrumentation must not record them unless the caller opted in."
`RequestInfo.State` is typed `any`, so the type system does not enforce that
rule. The interface documents it, and `otel.WithRecordContent` is the one
implementation that honors it, off by default. See
[Why content is not traced by default](why-content-is-not-traced-by-default.md)
for the reasoning behind that default.

## What `finish` receives: `RequestResult`

The client calls the function `RequestStart` returns exactly once, when the
call ends:

```go
type RequestResult struct {
    Attempts  int
    Status    int
    RequestID string
    Model     string
    Usage     Usage
    Response  *SystemOneResponse
    Err       error
}
```

`Response` is non-nil only on a successful `system_one` call; a `ListModels`
call and any failed call leave it nil. `Attempts` reports how many HTTP
attempts the retry policy made, a number that does not exist yet when
`RequestStart` runs.

## The hook opens before validation

`SystemOne` in `client.go` calls `startInstrument` before it validates
anything:

```go
info := RequestInfo{Operation: "system_one", Model: rc.model, State: state, Questions: questions}
info.QuestionCount, info.NoulCount, info.ChoiceCount, info.ScoreCount = countQuestions(questions)
ctx, finish := c.startInstrument(ctx, info)
defer func() {
    if r := recover(); r != nil {
        finish(RequestResult{Err: fmt.Errorf("panic: %v", r)})
        panic(r)
    }
}()

if err := validateContent("state", state, false); err != nil {
    finish(RequestResult{Err: err})
    return nil, err
}
if err := validateQuestions(questions); err != nil {
    finish(RequestResult{Err: err})
    return nil, err
}
```

`validateContent` and `validateQuestions` run after the hook has already
opened. A request that never reaches the network, such as a `Choice`
question with zero labels or a `state` that fails a size check, still
produces one `finish(RequestResult{Err: ...})` call, and an implementation
like `otel`'s records it as a span with an error status on the same timeline
as every other call. Instrumentation that opened only once bytes went over
HTTP would miss every client-side validation failure, which is the failure a
caller hits most often while first integrating against the API.

## The hook still fires on panics

`finish` runs exactly once however the call ends, panics included. The
`defer`/`recover` block quoted above covers the panic path, and
`startInstrument` wraps the instrumentation's own closure in a `sync.Once`:

```go
func (c *Client) startInstrument(ctx context.Context, info RequestInfo) (context.Context, func(RequestResult)) {
    if c.instr == nil {
        return ctx, func(RequestResult) {}
    }
    ctx, done := c.instr.RequestStart(ctx, info)
    var once sync.Once
    return ctx, func(r RequestResult) { once.Do(func() { done(r) }) }
}
```

The `sync.Once` covers both orderings. A panic after one of the normal
error-handling `finish(...)` calls further down in `SystemOne` makes the
deferred recovery's own `finish(...)` a no-op, because `done` already ran. A
panic before any of them leaves the deferred recovery as the only caller.
Exactly one `RequestResult` reaches the instrumentation either way. The
recovery re-raises the panic with `panic(r)` after `finish` returns, so the
span or metric already open is closed out and the panic continues to unwind.

## How `otel` implements it

`otel.New()` returns a `typesafe.Instrumentation`. Its `RequestStart` opens a
span named `"typesafe." + info.Operation`, so `typesafe.system_one` or
`typesafe.list_models`, with `trace.WithSpanKind(trace.SpanKindClient)`. The
attributes come from `RequestInfo`: `AttrProviderName` and `AttrSystem`
(both the literal `"typesafe"`), `AttrOperationName`, `AttrRequestModel`,
and, on `system_one` calls,
`AttrQuestionCount`/`AttrNoulCount`/`AttrChoiceCount`/`AttrScoreCount`.
`AttrState` and `AttrQuestions` are added when `WithRecordContent` was
configured with a positive byte limit and the span is recording. That is the
only place the implementation reads `RequestInfo`'s content fields.

The returned closure sets the rest from `RequestResult` at span end:
`AttrRetryAttempts` always; `AttrHTTPStatus`, `AttrRequestID`,
`AttrResponseModel`, and the token-usage attributes when present; and, on
error, `span.RecordError`, an error span status, and `AttrErrorType` holding
the error's Go type name. `WithRecordAnswers` walks
`RequestResult.Response.Answers` and sets a `typesafe.answer.<key>.<field>`
attribute per answer. It is off by default, for the same reason `state` and
`questions` are.

`Transport` wraps whatever `http.RoundTripper` the client would otherwise
use with `otelhttp.NewTransport`. Every HTTP attempt the retry policy makes
becomes its own child span under the operation span `RequestStart` opened. A
call that retries twice produces one `typesafe.system_one` span with three
HTTP child spans beneath it, giving both the operation-level view and the
per-attempt view.

## Metrics and logging implementations

`Instrumentation` carries nothing span-specific. A metrics-only
implementation could ignore spans entirely and increment a counter and
record a histogram observation inside the closure `RequestStart` returns:
`typesafe_requests_total{operation,status}` on every call, and
`typesafe_request_duration_seconds` from a `time.Since` captured at the top
of `RequestStart`. It reads the same `RequestInfo` and `RequestResult`
fields the `otel` package reads and writes them to a different backend. A
logging-only implementation could write one structured log line per call
from the closure, with `Operation`, `Model`, `Attempts`, `Status`, and `Err`
as fields, and need no span and no context propagation. The interface
supplies the request's shape at the start and its outcome at the end, and an
implementation decides what to do with both.

## Related documentation

- For why the core package can define this interface without depending on
  OpenTelemetry, see
  [Why the core is stdlib-only](why-the-core-is-stdlib-only.md).
- For why `State`, `Questions`, and answer values are gated behind explicit
  options rather than recorded by default, see
  [Why content is not traced by default](why-content-is-not-traced-by-default.md).
- For the exact attribute keys, when each is set, and what populates it, see
  the [span attributes reference](../reference/span-attributes.md).
- For a worked path to a trace in Honeycomb, see
  [Trace calls and send to Honeycomb](../how-to/trace-calls-and-send-to-honeycomb.md).
