---
title: "Why the Core Is Stdlib-Only"
description: "Why the typesafe package imports nothing but the Go standard library, and how the Instrumentation interface lets tracing live outside it."
diataxis: explanation
weight: 10
---

# Why the Core Is Stdlib-Only

Open `go.mod` for this module and you'll find dependencies, but none of them
are reachable from the root `typesafe` package. Every import in `client.go`,
`question.go`, `response.go`, `errors.go`, `retry.go`, and `transport.go`
comes from the standard library. That's not an accident of an early
prototype that hasn't grown dependencies yet — it's a boundary the design
holds deliberately, and it's worth understanding why, because it shapes how
tracing is added to this SDK rather than left out of it.

## The problem

An SDK that talks to an HTTP API needs surprisingly little: `net/http`,
`encoding/json`, `context`, `time`, `log/slog`. But the moment an SDK also
wants to *emit telemetry* — spans, exporters, propagation — it tends to pull
in an observability framework as a hard dependency. Every consumer of the
SDK then inherits that framework whether or not they use it, and inherits
whatever the framework's own dependency graph looks like this month.

For a library meant to be embedded inside other people's Go binaries — CLI
tools, servers, long-running agents — that's a cost imposed on everyone to
benefit only those who trace. The design spec settles this with an explicit
dependency policy: the root `typesafe` package imports only the standard
library; `typesafe/otel` imports the OpenTelemetry API and `otelhttp`; and
`cmd/jev` imports Cobra, `typesafe/otel`, and `otelconf` for exporter setup.
Three import graphs, one module, and a caller only pays for the graph they
actually use.

## The seam: `Instrumentation`

None of this works without a seam between "the client makes HTTP calls" and
"something observes those calls." That seam is the `Instrumentation`
interface, defined in `instrument.go` alongside the core client types:

```go
type Instrumentation interface {
    RequestStart(ctx context.Context, info RequestInfo) (context.Context, func(RequestResult))
    Transport(rt http.RoundTripper) http.RoundTripper
}
```

The core calls `RequestStart` once per `SystemOne` or `ListModels` call,
threads the returned `context.Context` through every retry attempt, and
calls the returned closure exactly once when the call finishes, successful
or not. `Transport` is applied once, at client construction, to wrap
whatever `http.RoundTripper` the client would otherwise use. Crucially, the
core package only ever *calls* this interface — it never implements
tracing, span creation, or exporting itself. `RequestInfo` and
`RequestResult` are plain structs built from data the core already has;
nothing about them requires OpenTelemetry to exist.

## The implementation lives elsewhere

`typesafe/otel` is a separate package that implements `Instrumentation`.
Its `otel.New(...)` constructor returns a `typesafe.Instrumentation` value,
wired into a client with `WithInstrumentation`. It's the one place in the
module that knows what a span is, and the one place that imports the
OpenTelemetry SDK and `otelhttp`. A caller who never references
`typesafe/otel` never compiles it in, and never pulls in its transitive
dependencies — the OTel API, its semantic-convention plumbing, none of it.

## Consequences

The payoff shows up in three places. First, a smaller dependency footprint
for anyone embedding this SDK in a larger binary: fewer modules in
`go.sum`, less to vet, a leaner attack surface. Second, less supply-chain
exposure — fewer transitive dependencies means fewer places a CVE or a
compromised release can reach a binary that only wanted to ask Jev a
question. Third, easier unit testing of the core itself: tests exercise
`SystemOne` and `ListModels` against `httptest` servers and fake
`Instrumentation` implementations, with no exporter, no collector, and no
OTel SDK initialization anywhere in the test binary.

The `jev` binary itself now demonstrates the cost the core package avoids.
`go.mod` carries about 90 requirements marked `// indirect` — essentially
all of them pulled in once `go.opentelemetry.io/contrib/otelconf` enters the
graph through the CLI's exporter-configuration code, dragging in the AWS
SDK, Kubernetes' `client-go`, Prometheus's client libraries, and several
OTLP exporters along with it. None of that is reachable from the root
`typesafe` package, which still has zero non-stdlib dependencies. The built
`jev` binary is about 25MB, most of it that exporter and cloud-SDK code —
weight a caller of the bare library never links in. A reader can compare
the two dependency graphs directly in the same `go.mod`.

## What you give up by default

The trade is explicit: with no `Instrumentation` attached, a `Client`
produces no traces at all. There is no ambient pickup of a global tracer
provider, no environment variable that silently turns tracing on inside the
core package. If you want spans, you construct one — typically via
`typesafe/otel.New()` — and attach it with `WithInstrumentation`. That's a
different default than SDKs built around a global observability
singleton, and it's a different shape than this SDK's own Python
counterpart, which logs through a module-level logger rather than exposing
a hook for an external tracer at all; see
[Mapping from the Python SDK](./mapping-from-the-python-sdk.md) for that
comparison. For the mechanics of attaching instrumentation, see
[Client options and environment](../reference/client-options-and-environment.md);
for a worked path to a working trace in Honeycomb, see
[Trace calls and send to Honeycomb](../how-to/trace-calls-and-send-to-honeycomb.md).
