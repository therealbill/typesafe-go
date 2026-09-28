---
title: "Retries and Budgets"
description: "Why typesafe-go's retry policy is shaped the way it is: bounded backoff, a total time budget, and which failures are worth retrying at all."
diataxis: explanation
weight: 20
---

# Retries and Budgets

A call to `SystemOne` or `ListModels` can fail for reasons unrelated to the request itself: a dropped connection, a momentarily overloaded System One, a rate limit. `RetryPolicy` handles that class of failure so a caller does not write a retry loop at every call site. Retrying on its own turns one slow response into a much slower one, and turns a fleet of clients into a thundering herd against an already-struggling server. `RetryPolicy` therefore combines bounded exponential backoff with jitter, ceilings on both the server's stated wait and the retry schedule, and a fixed line between failures worth retrying and failures that fail again regardless.

## What gets retried

`RetryStatuses` defaults to 408, 429, and the whole 500–599 range. `RetryOnConnErr` and `RetryOnTimeout` both default to true. Those defaults cover the failures where sending the same request again can succeed: a dropped connection, a timeout, a 429, and a 529 overloaded response all leave the request itself valid, and the server or network is what could not handle it this time.

A 422 is different in kind. The server parsed the request and rejected its content, so the identical bytes produce the identical rejection. 422 falls outside `RetryStatuses` and is never retried. `ShouldRetry` overrides the whole default set for a caller that needs different behavior.

`SystemOne` is an HTTP `POST`, and the TypeSafe API provides no idempotency key that would make retrying it safe. A timeout or connection reset, both retried by default, does not tell the client whether the server already processed the request; the request may have gone through and failed to return a response in time. A retry then repeats the same logical request, and if the first attempt landed, the caller can be billed twice for one logical call. The absence of an idempotency key is a limitation of the external API. A caller that cannot tolerate double-billing sets `RetryOnTimeout: false` and `RetryOnConnErr: false`, which surfaces those failures immediately as errors instead of retrying them.

## Backoff and jitter

The delay before a retry grows exponentially. For attempt `n`, the raw delay is `min(InitialDelay * 2^n, MaxDelay)`, doubling from `InitialDelay` (500ms) toward the `MaxDelay` ceiling (5s), both by default. Growth alone spreads retries over time but leaves clients that hit the same failure at the same moment retrying in lockstep. `Jitter` (default 0.25) shaves the computed delay down by a random fraction between 0 and `Jitter`, spreading those retries across a small window instead of one spike. `MaxRetries` (default 2) bounds how many times this repeats, and 0 disables retries outright.

`HonorRetryAfter` (default true) replaces that computed delay with the wait the server states in `retry-after-ms` or `Retry-After`. `MaxRetryAfter` (default 5 minutes) clamps down any parsed duration that exceeds it. The server knows how long its own rate limit or overload will last and the client does not, so the server's figure wins over the client's guess up to that ceiling. Without the ceiling, a misconfigured or malicious server could ask the client to wait an hour, a day, or longer, and be obeyed.

## What the budget bounds

`Budget` (default 30s; 0 means unlimited) bounds when a new retry attempt is allowed to start. It does not limit how many times the client retries, and it does not cap the call's total wall-clock time. The check runs once per retry, right before the client would sleep for the next backoff or server-supplied delay: if that sleep would push elapsed time past the budget, the client skips the retry and returns the last error immediately. An attempt already in flight sits outside this check, so `Budget` never interrupts a request actively waiting on the network, and one slow attempt can still take arbitrarily long. `ctx` and the per-attempt `WithTimeout` and `WithRequestTimeout` bound a single attempt's duration; `Budget` governs retry scheduling.

`ctx` cancellation sits above all of this. A cancelled context aborts the call immediately, mid-backoff included, rather than finishing that sleep. The caller has stated that it no longer wants the result, and waiting out the backoff first would silently delay a cooperative shutdown.

## Marshaling the body once

The client marshals the request body to JSON once, before the first attempt, and replays those bytes unchanged on every retry rather than re-encoding `state` and `questions` each time. Every retried request is therefore byte-identical to the original, a second marshal cannot produce different output, and no attempt re-validates the body.

## Related documentation

- For the client and per-request options that configure this policy (`WithRetryPolicy`, `WithRequestRetry`, and the environment variables behind them), see [Client options and environment](../reference/client-options-and-environment.md).
- For the error types a retried call can still surface, including `RateLimitError` and the `IsRetryable` predicate, see [Errors and exit codes](../reference/errors-and-exit-codes.md).
- For a task-oriented walkthrough of tuning retry behavior to your own rate limits and latency budgets, see [Handle rate limits and retries](../how-to/handle-rate-limits-and-retries.md).
