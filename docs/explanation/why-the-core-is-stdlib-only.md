---
title: "Why the Core Is Stdlib-Only"
description: "Why the typesafe package imports nothing but the Go standard library, and how the Instrumentation interface keeps tracing outside it."
diataxis: explanation
weight: 10
---

The root `typesafe` package imports only the standard library: `net/http`, `encoding/json`, `context`, `time`, and `log/slog`. The module's `go.mod` lists other dependencies, but none is reachable from the root package. This page describes why that boundary exists and what it costs.

An SDK that emits telemetry usually imports an observability framework directly. Every program that uses the SDK then compiles and ships that framework and its dependency graph, whether or not it traces anything. For a library meant to be embedded in other people's binaries, that is a cost paid by everyone for a feature used by some.

## The dependency policy

The dependency policy splits the module into three import graphs. The root package imports only the standard library. `typesafe/otel` imports the OpenTelemetry API and `otelhttp`. `cmd/jev` imports Cobra, `typesafe/otel`, and `otelconf`. A program pays only for the graph it imports.

## The Instrumentation interface

The split works through the `Instrumentation` interface in `instrument.go`, which separates making HTTP calls from observing them:

```go
type Instrumentation interface {
    RequestStart(ctx context.Context, info RequestInfo) (context.Context, func(RequestResult))
    Transport(rt http.RoundTripper) http.RoundTripper
}
```

The core calls `RequestStart` once per `SystemOne` or `ListModels` call. It threads the returned `context.Context` through every retry attempt and calls the returned closure exactly once when the call finishes, successful or not. `Transport` is applied once, at client construction, wrapping whatever `http.RoundTripper` the client would otherwise use. The core package calls this interface and never implements tracing, span creation, or exporting. `RequestInfo` and `RequestResult` are plain structs built from data the core already holds, and neither requires OpenTelemetry to compile.

## The typesafe/otel package

`typesafe/otel` is a separate package that implements `Instrumentation`. Its `otel.New(...)` constructor returns a `typesafe.Instrumentation` value, which `WithInstrumentation` wires into a client. It is the only package in the module that knows what a span is, and the only one that imports the OpenTelemetry SDK and `otelhttp`. A caller who never references `typesafe/otel` never compiles it in and never links its transitive dependencies, the OTel API and its semantic-convention plumbing included.

## Consequences

The boundary has three consequences. A binary that embeds this SDK carries fewer modules in `go.sum`, which means less to vet and a smaller attack surface. Fewer transitive dependencies also means fewer places a CVE or a compromised release can reach a binary that only wanted to ask Jev a question. The core is also testable without any telemetry setup: tests exercise `SystemOne` and `ListModels` against `httptest` servers and fake `Instrumentation` implementations, with no exporter, no collector, and no OTel SDK initialization anywhere in the test binary.

The `jev` binary measures the cost the core package avoids. `go.mod` carries about 90 requirements marked `// indirect`, nearly all of them entering the graph through `go.opentelemetry.io/contrib/otelconf` in the CLI's exporter-configuration code, which drags in the AWS SDK, Kubernetes' `client-go`, Prometheus's client libraries, and several OTLP exporters. None of that is reachable from the root `typesafe` package, which still has zero non-stdlib dependencies. The built `jev` binary is about 25MB, most of it that exporter and cloud-SDK code, which a caller of the bare library never links in. Both dependency graphs sit in the same `go.mod`.

## What you give up by default

A `Client` with no `Instrumentation` attached produces no traces. The core package does not pick up a global tracer provider, and no environment variable turns tracing on inside it. A caller that wants spans constructs an implementation, typically `typesafe/otel.New()`, and attaches it with `WithInstrumentation`. SDKs built around a global observability singleton default the other way. This SDK's Python counterpart differs again: it logs through a module-level logger and exposes no hook for an external tracer at all. See [Mapping from the Python SDK](./mapping-from-the-python-sdk.md) for that comparison. For the mechanics of attaching instrumentation, see [Client options and environment](../reference/client-options-and-environment.md); for a worked path to a working trace in Honeycomb, see [Trace calls and send to Honeycomb](../how-to/trace-calls-and-send-to-honeycomb.md).
