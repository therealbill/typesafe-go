---
title: "Why Content Is Not Traced by Default"
description: "Why the typesafe/otel instrumentation records call metadata but never state, questions, or answers unless the caller opts in."
diataxis: explanation
weight: 30
---

# Why Content Is Not Traced by Default

When you wrap a `typesafe.Client` with `typesafe/otel.New()`, every `SystemOne`
call produces a span. That span is full of useful metadata: which model
answered, how many tokens it used, how many retries it took, whether it
failed. What it does not contain, unless you ask for it, is the actual
content you sent (the `state` you passed in, the `questions` you asked, or
the answers that came back). This page explains why that boundary exists and
why it is drawn exactly where it is.

## The problem: `state` is often someone else's data

`state` is the free-form payload you hand to System One for judgment. Nothing
in the API constrains what it can hold, and in practice it is very often
customer- or user-submitted content. The design spec's own running example is
a support ticket: `{"ticket": "I was charged twice this month and nobody
answers my emails. Fix it now."}`, sent so Jev can judge whether it's a
billing issue, what tone it carries, and how urgent it is. That single string
is a customer complaint about a specific billing error: exactly the kind of
thing an application already handles carefully in its own logs and
databases, and exactly the kind of thing an organization does not want
silently copied into a third-party observability backend because someone
attached a tracer.

`questions` is less inherently sensitive, but not inert either. Instructions
and criteria can encode business logic or internal category names a team
might not want sitting in a trace store any more than in a public log.

## A span attribute is not a log line

Debug logging seems like a comparable risk, harmless because someone would
have to go looking. A tracing backend is a different kind of exposure than
a log line that scrolls off a terminal or ages out of a short retention
window. Spans in a system like Honeycomb are typically retained for a long
time, the point of tracing being the ability to go back and find the one
slow or failed request from last week, and they are searchable and
exportable by anyone with dashboard access: a query across attributes, a
saved board, an export to a spreadsheet. A support ticket that lands in
`typesafe.state` doesn't sit in one place waiting to be noticed; it becomes
part of a searchable corpus that persists on someone else's infrastructure
for as long as the retention policy says.

That difference is why opting a whole class of data into that store can't be
a neutral default. Whoever turns on tracing for their service is not
necessarily who decided the tracing backend's retention and access policy.
Making content-in-traces the default would mean every team that wires up
`typesafe/otel` for ordinary operational visibility (latency, error rates,
retry counts) also, without choosing to, starts shipping customer content to
that backend.

## What you have to ask for

The `typesafe/otel` package keeps content recording behind two explicit
options, both off unless the caller sets them:

- `WithRecordContent(maxBytes int)` records `state` and `questions` as the
  `typesafe.state` and `typesafe.questions` span attributes, JSON-encoded and
  truncated to `maxBytes`.
- `WithRecordAnswers()` records the response side: a
  `typesafe.answer.<key>.<field>` attribute per answer field: `choice`,
  `score`, `noul`, `confidence`.

Because these are constructor options on `otel.New`, turning them on is a
decision made by whoever configures that instrumentation instance, the same
party who presumably owns, or has evaluated, the tracing backend's retention
and access policy. The opt-in leaves that decision to whoever configures the
instrumentation.

## What's recorded either way

Turning content recording off doesn't mean the span is empty. A fixed set of
attributes is always present, since each describes the call's shape and
outcome, not what it was about: `gen_ai.provider.name`, `gen_ai.system`,
`gen_ai.request.model`, `gen_ai.response.model`, token usage
(`gen_ai.usage.input_tokens` / `output_tokens`), `typesafe.request_id`,
question counts by type (`typesafe.questions.count`, `.noul`, `.choice`,
`.score`), `typesafe.retry.attempts`, `http.response.status_code`, and, on
failure, `error.type`. From these alone you can tell a call used
`jev-1.13.0`, asked one noul and one choice question, took two retries, and
eventually failed with a rate-limit error (enough for dashboards, alerts,
and latency/error analysis) without any of it revealing that the question
was about an angry customer's double charge. Metadata answers "what happened
to this call"; content answers "what was this call about," and only the
second is gated.

## Consistent with the rest of the client

The core `typesafe` client applies
the same instinct to its own `log/slog` logging: the `Authorization` header
is never logged, and request and response bodies are never logged, regardless
of log level. In both places the stance is the same: operational metadata is
safe to record by default, and anything that might carry sensitive payload
content requires the caller to say so explicitly. `typesafe/otel`'s opt-ins
are that same default, applied to spans instead of log lines.

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
