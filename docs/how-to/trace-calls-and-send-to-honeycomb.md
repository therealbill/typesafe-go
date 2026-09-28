---
title: "How to Trace Calls and Send Them to Honeycomb"
description: "Attach OpenTelemetry instrumentation to a typesafe.Client or the jev CLI, opt into content recording, and route spans to Honeycomb."
diataxis: how-to
weight: 40
---

# How to Trace Calls and Send Them to Honeycomb

**Goal**: Emit OpenTelemetry spans for `SystemOne`/`ListModels` calls — from
your own Go program and from the `jev` CLI — and route them to Honeycomb.

## Prerequisites

- A working `SystemOne` call — see
  [Your First Judgment in Go](../tutorials/first-judgment-in-go.md)
- A built `./bin/jev` — see
  [jev from the Command Line](../tutorials/jev-from-the-command-line.md)
- Familiarity with OpenTelemetry's tracer provider / exporter model
- Why content is opt-in: see
  [Why Content Is Not Traced by Default](../explanation/why-content-is-not-traced-by-default.md)
- Why this is a separate package: see
  [Why the Core Is Stdlib-Only](../explanation/why-the-core-is-stdlib-only.md)

## Steps

### 1. Set a global tracer provider

`typesafe/otel` never installs a tracer provider itself — it uses whatever
is registered globally, or one you pass explicitly. For a quick local check,
export finished spans to stdout:

```go
exporter, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
otel.SetTracerProvider(tp)
defer tp.Shutdown(ctx)
```

(`go.opentelemetry.io/otel/sdk/trace` as `sdktrace`,
`go.opentelemetry.io/otel/exporters/stdout/stdouttrace`.) For a real OTLP
pipeline in production, build the provider from a YAML file with
`go.opentelemetry.io/contrib/otelconf` instead — this is exactly what `jev`
itself does (Step 4).

### 2. Attach instrumentation to the client

```go
client, err := typesafe.NewClient(
    typesafe.WithAPIKey(apiKey),
    typesafe.WithInstrumentation(tsotel.New(
        tsotel.WithRecordContent(200), // opt in: state + questions, truncated to 200 bytes
        tsotel.WithRecordAnswers(),    // opt in: each answer's value + confidence
    )),
)
```

Import `tsotel "github.com/therealbill/typesafe-go/otel"`. Both options are
off by default; see
[Why Content Is Not Traced by Default](../explanation/why-content-is-not-traced-by-default.md)
for why. Without them you still get every non-content attribute — model,
token usage, retry count, status.

### 3. Make a call and see the exported span

Calling `client.SystemOne(ctx, state, questions)` against a real API (or, to
keep this deterministic, an `httptest.Server` fake) produces exactly one
`typesafe.system_one` span plus one child `HTTP POST` span from `otelhttp`.
A real run of Steps 1–3 against an `httptest` fake exported this span (JSON
trimmed to the attributes that matter here):

```json
{
  "Name": "typesafe.system_one",
  "Attributes": [
    {"Key": "gen_ai.provider.name", "Value": {"Value": "typesafe"}},
    {"Key": "gen_ai.operation.name", "Value": {"Value": "system_one"}},
    {"Key": "gen_ai.request.model", "Value": {"Value": "jev-latest"}},
    {"Key": "typesafe.questions.count", "Value": {"Value": 1}},
    {"Key": "typesafe.state", "Value": {"Value": "{\"ticket\":\"I was charged twice this month.\"}"}},
    {"Key": "typesafe.questions", "Value": {"Value": "{\"tone\":{\"criteria\":{\"angry\":null,\"calm\":null},\"instructions\":\"What is the tone?\",\"type\":\"choice\"}}"}},
    {"Key": "gen_ai.response.model", "Value": {"Value": "jev-1.13.0"}},
    {"Key": "gen_ai.usage.input_tokens", "Value": {"Value": 41}},
    {"Key": "typesafe.request_id", "Value": {"Value": "req_demo123"}},
    {"Key": "typesafe.answer.tone.choice", "Value": {"Value": "angry"}},
    {"Key": "typesafe.answer.tone.confidence", "Value": {"Value": 0.87}}
  ]
}
```

`gen_ai.operation.name` is always set to the operation string (`system_one`
here, `list_models` for `ListModels`) — the same value used to build the
span name `typesafe.<operation>`.
`typesafe.state` and `typesafe.questions` (from `WithRecordContent`) and
`typesafe.answer.tone.*` (from `WithRecordAnswers`) are exactly the
attributes that disappear if you drop those two options. Full key list and
when each is set: [span attributes reference](../reference/span-attributes.md).

### 4. Turn tracing on for `jev`

`jev` decides whether to start the OTel SDK per invocation
(`internal/cli/telemetry.go`), checked in this order:

| Condition | Result |
|---|---|
| `--no-trace` passed | Tracing disabled. Checked first; wins over everything else. |
| `--trace` passed | Tracing enabled. |
| `HONEYCOMB_API_KEY`, `OTEL_EXPORTER_OTLP_ENDPOINT`, or `OTEL_CONFIG_FILE` set | Tracing enabled. |
| None of the above | Tracing disabled. |

With no `OTEL_CONFIG_FILE`, `OTEL_SERVICE_NAME` sets the `service.name`
resource attribute (default `jev`). Full table:
[client options and environment](../reference/client-options-and-environment.md).

### 5. Point `jev` at Honeycomb

The simplest path is `HONEYCOMB_API_KEY`:

```bash
export HONEYCOMB_API_KEY=your-honeycomb-key
./bin/jev ask --state "..." --noul billing="Is this about billing?"
```

This is enough by itself to enable tracing (Step 4) and send an
`x-honeycomb-team` header on the OTLP/HTTP export.

For anything beyond that default — a different region, more processors,
sampling — set `OTEL_CONFIG_FILE` to a YAML file, read and `${VAR}`-expanded
against your environment before parsing, so it can reference
`${HONEYCOMB_API_KEY}` without the key living in the file:

```yaml
file_format: "1.0"
tracer_provider:
  processors:
    - batch:
        exporter:
          otlp_http:
            endpoint: https://api.honeycomb.io/v1/traces
            headers:
              - name: x-honeycomb-team
                value: ${HONEYCOMB_API_KEY}
```

If this file fails to parse, every `${VAR}`-expanded secret is redacted from
the resulting stderr message — a YAML error near `${HONEYCOMB_API_KEY}`
can't leak the key's value.

## Verify it works

Send one request with tracing forced on and check Honeycomb (or your
collector) for a `typesafe.system_one` span a few seconds later:

```bash
HONEYCOMB_API_KEY=your-key ./bin/jev ask --trace \
  --state "..." --noul billing="Is this about billing?"
```

✅ Success looks like the normal answer JSON on stdout, and, shortly after,
a trace in Honeycomb showing `typesafe.system_one` with a child HTTP span.

## Two things to know before you rely on this

**A misconfigured exporter never fails the command.** Run `jev ask --trace`
with no `HONEYCOMB_API_KEY` and no collector listening — the command still
succeeds; exit code and stdout are independent of whether the best-effort
trace export succeeds. Captured on this machine, right now:

```
$ ./bin/jev ask --trace --state "..." --noul billing="Is this about billing?"
{"model":"jev-1.13.0","answers":{"billing":{"noul":0.97,"type":"noul"}},"usage":{"input_tokens":277,"output_tokens":20},"request_id":"req_01a0e12e9fee760e9e02e286872ab672"}
2026/09/26 23:45:24 traces export: failed to send to https://api.honeycomb.io/v1/traces: 401 Unauthorized (body: !missing 'x-honeycomb-team' header)
```

Exit code 0. That stderr line comes from the OTel SDK's own default error
handler, not a `jev:`-prefixed message — a tracing misconfiguration must
never turn a working `ask`/`models` call into a failure.

**`jev` is a much heavier binary than the core library.** See
[Why the Core Is Stdlib-Only](../explanation/why-the-core-is-stdlib-only.md)
for why the core has zero dependencies while `jev` carries the OpenTelemetry
stack, and how to get tracing without paying for it in your own binary.

## Troubleshooting

### Problem: no spans arrive in Honeycomb, but the command succeeds
**Symptom**: exit code 0, normal JSON output, nothing shows up in Honeycomb.
**Cause**: export failed silently to your terminal's normal flow — check
stderr for the `traces export: failed to send to ...` line described above.
**Solution**: fix the underlying cause (usually a missing/wrong
`HONEYCOMB_API_KEY`), then re-run.

### Problem: tracing never turns on
**Symptom**: no error, but also no spans, and no stderr line at all.
**Cause**: none of `--trace`, `HONEYCOMB_API_KEY`, `OTEL_EXPORTER_OTLP_ENDPOINT`,
or `OTEL_CONFIG_FILE` was set, or `--no-trace` was also passed.
**Solution**: pass `--trace` explicitly to confirm the rest of the pipeline
works, independent of environment detection.

### Problem: `jev: tracing disabled: <error>`
**Symptom**: this line on stderr instead of the export-time error above.
**Cause**: `OTEL_CONFIG_FILE` failed to read or parse, or SDK construction
itself failed — this happens before any request, not during export.
**Solution**: validate the YAML with `otelconf.ParseYAML` semantics in mind
(the schema shown in Step 5), and confirm the path is readable. Safe to
paste into a bug report — any `${VAR}`-expanded secret is already redacted.

### Problem: state/questions never appear on the span
**Symptom**: everything else on the span is populated, but no
`typesafe.state` or `typesafe.questions`.
**Cause**: `WithRecordContent` was not set, or set with `maxBytes <= 0`.
**Solution**: pass `tsotel.WithRecordContent(n)` with `n > 0` (Step 2), and
confirm that's an acceptable exposure — see
[Why Content Is Not Traced by Default](../explanation/why-content-is-not-traced-by-default.md).

## Next steps

- Full attribute reference, including answer attributes:
  [span attributes](../reference/span-attributes.md)
- All telemetry environment variables and CLI flags:
  [jev CLI](../reference/jev-cli.md) and
  [client options and environment](../reference/client-options-and-environment.md)

## See also

- [Why Content Is Not Traced by Default](../explanation/why-content-is-not-traced-by-default.md)
- [Why the Core Is Stdlib-Only](../explanation/why-the-core-is-stdlib-only.md)
- [Span attributes](../reference/span-attributes.md)
- [Client options and environment](../reference/client-options-and-environment.md)
- [jev CLI](../reference/jev-cli.md)
