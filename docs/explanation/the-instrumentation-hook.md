---
title: "The Instrumentation Hook"
description: "How the Instrumentation interface observes a call's full lifecycle, including validation failures and panics, without the core package knowing what a span is."
diataxis: explanation
weight: 60
---

# The Instrumentation Hook

Every `SystemOne` or `ListModels` call passes through one seam before it
does anything else: `c.startInstrument`. This page looks at what that hook
sees, when it fires relative to the rest of the call, and how the `otel`
subpackage turns it into OpenTelemetry spans — as one example of what the
interface makes possible, not the only thing it could do.

## Why a hook instead of a dependency

The root `typesafe` package has no reference to OpenTelemetry anywhere in
it. Instead, `instrument.go` defines a small interface:

```go
type Instrumentation interface {
    RequestStart(ctx context.Context, info RequestInfo) (context.Context, func(RequestResult))
    Transport(rt http.RoundTripper) http.RoundTripper
}
```

and the client calls it without knowing what's behind it. This is the same
boundary discussed in
[Why the core is stdlib-only](why-the-core-is-stdlib-only.md): a caller who
never wires up `typesafe/otel` never compiles in OpenTelemetry's dependency
graph, and the core package stays testable against plain fakes. This page
picks up from there to look at the hook's own shape — what it's told, when
it's called, and what a second, non-tracing implementation of it would look
like.

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
many are `Noul`, `Choice`, or `Score` (including the pointer forms, and
`RawQuestion` values whose `"type"` field matches one of those three
strings) — cheap, non-sensitive shape information an implementation can
record unconditionally. `State` and `Questions` are the actual request
payload, and the field comments in `instrument.go` say plainly:
"Instrumentation must not record them unless the caller opted in." That's
not enforced by the type system — `RequestInfo.State` is just `any` — it's a
contract the interface documents and `otel.WithRecordContent` is the one
implementation that honors it, off by default. See
[Why content is not traced by default](why-content-is-not-traced-by-default.md)
for the reasoning behind that default.

## What `finish` receives: `RequestResult`

The function `RequestStart` returns is called exactly once, when the call
ends:

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

`Response` is nil in every case except a successful `system_one` call — a
`ListModels` call, or any failed call, never populates it. `Attempts` is
something `RequestInfo` couldn't have told you at the start: how many HTTP
attempts the retry policy actually made is only known once the call is
over.

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

`validateContent` and `validateQuestions` run after the span is already
open, not before. The practical effect: a request that never reaches the
network — a `Choice` question with zero labels, a `state` that fails a size
check — still produces one `finish(RequestResult{Err: ...})` call, and an
implementation like `otel`'s sees it as a span whose status is an error, on
the same timeline as every other call. A tracing setup that only saw calls
that got as far as sending bytes over HTTP would systematically undercount
client-side validation failures — exactly the failures a caller most wants
visibility into while integrating against the API for the first time.

## The hook still fires on panics

The `defer`/`recover` block quoted above exists because `finish` is
supposed to run exactly once no matter how the call ends, panics included.
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

That `sync.Once` is what makes the panic-recovery pattern safe: if a panic
happens after the normal error-handling `finish(...)` calls further down in
`SystemOne`, the deferred recovery's own `finish(...)` call is a no-op,
because `done` already ran. If a panic happens before any of those calls,
the deferred recovery is the only one that runs. Either way exactly one
`RequestResult` reaches the instrumentation, and the panic itself is
re-raised (`panic(r)`) after `finish` returns, so recovering here never
swallows the panic — it only guarantees the span or metric that was already
open gets closed out before the panic continues to unwind.

## How `otel` implements it

`otel.New()` returns a `typesafe.Instrumentation`. Its `RequestStart` opens
a span named `"typesafe." + info.Operation` — `typesafe.system_one` or
`typesafe.list_models` — with `trace.WithSpanKind(trace.SpanKindClient)` and
a set of attributes drawn straight from `RequestInfo`: `AttrProviderName`
and `AttrSystem` (both the literal `"typesafe"`), `AttrOperationName`,
`AttrRequestModel`, and, for `system_one` calls,
`AttrQuestionCount`/`AttrNoulCount`/`AttrChoiceCount`/`AttrScoreCount`. If
`WithRecordContent` was configured with a positive byte limit and the span
is actually recording, `AttrState` and `AttrQuestions` are added too — the
one place this implementation reaches into the content fields
`RequestInfo` otherwise carries past unused.

The returned closure fills in the rest from `RequestResult` at span end:
`AttrRetryAttempts` always, `AttrHTTPStatus`/`AttrRequestID`/
`AttrResponseModel`/token-usage attributes when present, and, on error,
`span.RecordError`, an error span status, and `AttrErrorType` (the error's Go
type name). `WithRecordAnswers` adds one more layer, walking
`RequestResult.Response.Answers` to set a `typesafe.answer.<key>.<field>`
attribute per answer — also off by default, for the same reason `state` and
`questions` are.

`Transport` is the other half: it wraps whatever `http.RoundTripper` the
client would otherwise use with `otelhttp.NewTransport`, so every HTTP
attempt the retry policy makes becomes its own child span, nested under the
operation span `RequestStart` opened. A call that retries twice produces one
`typesafe.system_one` span with three HTTP child spans underneath it — the
operation-level view and the per-attempt view both exist, at their natural
granularity.

## A different implementation: metrics or logs

Nothing about `Instrumentation` is span-shaped. A metrics-only
implementation could ignore spans entirely and just increment a counter and
record a histogram observation inside the closure `RequestStart` returns —
`typesafe_requests_total{operation,status}` on every call,
`typesafe_request_duration_seconds` from a `time.Since` captured at the top
of `RequestStart`, using exactly the same `RequestInfo`/`RequestResult`
fields the `otel` package reads, just written to a different backend. A
logging-only implementation could be simpler still: one structured log line
per call, written from the closure, with `Operation`, `Model`, `Attempts`,
`Status`, and `Err` as fields — no span, no context propagation, nothing
tracing-specific at all. The interface only asks for "something happened,
here's what" at the start and "here's how it ended" at the end; what an
implementation does with that is entirely its own concern.

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
