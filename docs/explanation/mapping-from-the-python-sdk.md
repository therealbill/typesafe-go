---
title: "Mapping from the Python SDK"
description: "How each piece of the Python typesafe-sdk surface maps onto the Go SDK, and why the translations are not always one-to-one."
diataxis: explanation
weight: 40
---

# Mapping from the Python SDK

Both SDKs wrap the same System One API and preserve the same wire semantics. Go's type system, concurrency model, and error-handling idioms turn several Python conveniences into different shapes. This page maps each piece of the Python `typesafe-sdk` surface onto its Go counterpart and describes what changed in the translation.

## One client, not two

Python exposes `TypeSafeClient` and `AsyncTypeSafeClient` with parallel methods, because an `async def` function cannot be called from ordinary code without an event loop. `NewClient` produces a single `*Client`, and both `SystemOne` and `ListModels` take a `context.Context` as their first argument. `ctx` and goroutines together supply what the async client exists to provide in Python: cancellation, deadlines, and many concurrent calls without blocking a program on any one of them. An async client type in Go would be the same struct wrapped in goroutines the caller was always free to start, so the Go SDK omits it.

## Typed decoding without a validation runtime

Python's `response_model=` parameter hands `system_one` a pydantic model and returns a validated instance, because pydantic inspects arbitrary Python objects against a schema at runtime. Go's `encoding/json` has no equivalent introspection step; it decodes into whatever concrete type it is given. `SystemOneAs[T any](ctx, c, state, questions, opts...) (T, *SystemOneResponse, error)` gives the same convenience, decoding answers straight into a caller's own struct, through a different mechanism. `T` is a plain struct whose fields are `NoulAnswer`, `ChoiceAnswer`, or `ScoreAnswer` with `json` tags naming the question keys, and the `answers` object unmarshals directly into it.

The validation pydantic would perform centrally lives on the answer types instead. Each implements `json.Unmarshaler`, so decoding an answer also checks that its `type` discriminator matches what the field expects. A caller gets an error when the shape does not match in either SDK. In Go that check is distributed across small `UnmarshalJSON` methods rather than concentrated in one schema validator.

## One error struct instead of an exception hierarchy

Python's exception hierarchy allows a `try`/`except` block to catch by class. The Go SDK collapses that hierarchy into one `APIError` struct carrying `Status int`. Callers branch on the status directly, or call `IsAuthError`, `IsRateLimited`, or `IsRetryable` when the check is common enough to name. A status-specific Go type for each of a dozen HTTP status codes would give a caller a dozen types to track for a branch that reads an integer.

`RateLimitError` is the one status with its own type. A 429 carries a `RetryAfter` value nothing else does, so `RateLimitError{APIError; RetryAfter time.Duration}` holds it and still unwraps to `*APIError` through `errors.As`, which keeps generic status-based handling working for rate limits. `TypeSafeAPIConnectionError` and `TimeoutError` become `ConnectionError` and `TimeoutError` for the same reason: neither failure reaches an HTTP status at all. `TypeSafeAPIResponseValidationError.field_path` becomes `ResponseValidationError.FieldPath`, and both name the answer field that failed to decode. [Errors and exit codes](../reference/errors-and-exit-codes.md) lists the fields and predicates; [Response types](../reference/response-types.md) covers the shapes being decoded.

## Configuration and logging

The keyword arguments to `TypeSafeClient(api_key=, base_url=, model=, retry=, timeout=, headers=, http_client=)` become functional options passed to `NewClient`: `WithAPIKey`, `WithBaseURL`, `WithModel`, `WithRetryPolicy`, `WithTimeout`, `WithHeaders`, and `WithHTTPClient`. `RetryPolicy` keeps the semantics of Python's `RetryPolicy(max_retries, backoff_max, timeout, ...)` under Go-idiomatic field names and adds `MaxRetryAfter`, which caps how long a server-requested `Retry-After` wait is honored and has no Python counterpart. The environment variables match by name and by semantics: `TYPESAFE_API_KEY`, `TYPESAFE_BASE_URL`, `TYPESAFE_DEFAULT_MODEL`, and `TYPESAFE_LOG_LEVEL` are spelled identically in both SDKs, so a deployment already exporting them needs no changes to run the Go SDK too. See [Client options and environment](../reference/client-options-and-environment.md) for the full resolution order.

Logging differs only in mechanism. Python logs through the module-level `typesafe_sdk` logger, configured like any Python logger. Go has no ambient global logger to hook into, so the SDK uses the standard `log/slog` package, configured through `WithLogger` or by setting `TYPESAFE_LOG_LEVEL` to `debug`, `info`, `warning`, or `error`. Any other value, `off` included, disables logging, and that is the default. Both SDKs log the same fields: method, path, status, duration, attempt, and request ID, with the `Authorization` header and all bodies excluded.

## What has no Python counterpart

Three parts of the Go SDK map to nothing on the Python side: the `Instrumentation` hook, the `typesafe/otel` subpackage, and the `jev` CLI. A Python caller can generate and execute Python to drive Jev directly and needs none of them. The Go SDK serves callers that cannot or should not do that: the future TypeSafe Claude Code plugin and other non-Go tooling need to ask Jev a question and get a typed judgment back without shelling out to a Python interpreter. `jev` is that entry point as a standalone binary, and `Instrumentation` and `typesafe/otel` make calls placed that way traceable. All three extend the surface rather than translating something Python already had.
