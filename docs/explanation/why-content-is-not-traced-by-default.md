---
title: "Why Content Is Not Traced by Default"
description: "Why the typesafe/otel instrumentation records call metadata but never state, questions, or answers unless the caller opts in."
diataxis: explanation
weight: 30
---

# Why Content Is Not Traced by Default

A `typesafe.Client` wrapped with `typesafe/otel.New()` produces a span for
every `SystemOne` call. The span carries the call's metadata: which model
answered, how many tokens it used, how many retries it took, whether it
failed. It carries none of the call's content, meaning the `state` that was
sent, the `questions` that were asked, and the answers that came back,
unless the caller turns that on. This page describes where that boundary
sits and what it separates.

## What `state` carries

`state` is the free-form payload a caller hands to System One for judgment.
Nothing in the API constrains what it can hold, and in practice it is very
often customer- or user-submitted content. The design spec's own running
example is a support ticket: `{"ticket": "I was charged twice this month and
nobody answers my emails. Fix it now."}`, sent so Jev can judge whether it's
a billing issue, what tone it carries, and how urgent it is. That single
string is a customer complaint about a specific billing error: the kind of
content an application already handles carefully in its own logs and
databases, and the kind an organization does not want silently copied into a
third-party observability backend because someone attached a tracer.

`questions` carries less inherently sensitive material, but it is not inert
either. Instructions and criteria can encode business logic or internal
category names a team may not want sitting in a trace store any more than in
a public log.

## How a trace store differs from a log

A log line scrolls off a terminal or ages out of a short retention window.
Spans in a system like Honeycomb are typically retained far longer, since
the point of tracing is the ability to go back and find the one slow or
failed request from last week, and anyone with dashboard access can search
and export them: a query across attributes, a saved board, an export to a
spreadsheet. A support ticket that lands in `typesafe.state` becomes part of
a searchable corpus that persists on someone else's infrastructure for as
long as the retention policy says.

Whoever turns on tracing for a service is not necessarily whoever decided
the tracing backend's retention and access policy. Recording content by
default would mean every team that wires up `typesafe/otel` for ordinary
operational visibility, meaning latency, error rates, and retry counts, also
starts shipping customer content to that backend without choosing to.

## The two opt-in options

The `typesafe/otel` package keeps content recording behind two explicit
options, both off unless the caller sets them:

- `WithRecordContent(maxBytes int)` records `state` and `questions` as the
  `typesafe.state` and `typesafe.questions` span attributes, JSON-encoded and
  truncated to `maxBytes`.
- `WithRecordAnswers()` records the response side: a
  `typesafe.answer.<key>.<field>` attribute per answer field: `choice`,
  `score`, `noul`, `confidence`.

Both are constructor options on `otel.New`, so turning them on is a decision
made where that instrumentation instance is configured, by the party that
owns or has evaluated the tracing backend's retention and access policy.

## What's recorded either way

A span with content recording off is not empty. A fixed set of attributes is
always present, each describing the call's shape and outcome rather than
what it was about: `gen_ai.provider.name`, `gen_ai.system`,
`gen_ai.request.model`, `gen_ai.response.model`, token usage
(`gen_ai.usage.input_tokens` / `output_tokens`), `typesafe.request_id`,
question counts by type (`typesafe.questions.count`, `.noul`, `.choice`,
`.score`), `typesafe.retry.attempts`, `http.response.status_code`, and, on
failure, `error.type`. Those attributes show that a call used `jev-1.13.0`,
asked one noul and one choice question, took two retries, and eventually
failed with a rate-limit error. That is enough for dashboards, alerts, and
latency and error analysis, and none of it reveals that the question was
about an angry customer's double charge.

## The same rule in the client's logging

The core `typesafe` client applies the same rule to its own `log/slog`
output. It never logs the `Authorization` header, and it never logs request
or response bodies, regardless of log level. Both places record operational
metadata by default and require the caller to say so explicitly before
anything that might carry sensitive payload content is recorded.

## Related documentation

- For the full list of span attributes and exactly which recorded value
  populates each one, see the
  [span attributes reference](../reference/span-attributes.md).
- To turn `WithRecordContent` and `WithRecordAnswers` on and wire spans to a
  backend, see
  [Trace calls and send to Honeycomb](../how-to/trace-calls-and-send-to-honeycomb.md).
- For why instrumentation is a separately-imported piece rather than baked
  into the core client, see
  [Why the core is stdlib-only](./why-the-core-is-stdlib-only.md).
