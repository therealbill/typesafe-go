---
title: "Retries and Budgets"
description: "Why gojev's retry policy is shaped the way it is: bounded backoff, a total time budget, and which failures are worth retrying at all."
type: explanation
---

# Retries and Budgets

Every call to `SystemOne` or `ListModels` can fail for reasons unrelated to the request itself: the network hiccups, System One is momentarily overloaded, or a rate limit kicks in. The retry policy exists to smooth over that class of failure without a hand-rolled retry loop at every call site. But a policy that just "tries harder" is dangerous on its own — it can turn one slow response into a much slower one, or a fleet of clients into a thundering herd hitting an already-struggling server. `RetryPolicy` rests on bounded exponential backoff with jitter, ceilings on both the server's stated wait and the retry schedule, and a sharp line between failures worth retrying and failures that will just fail again.

## What gets retried, and why

`RetryStatuses` defaults to 408, 429, and the whole 500–599 range, and `RetryOnConnErr` / `RetryOnTimeout` both default to true. The idea isn't "any error" — it's "any failure where the same request might succeed if sent again." A dropped connection, a timeout, a 429, and a 529 overloaded response are all transient: nothing about the request was wrong, the server or network just couldn't handle it this time.

A 422 is different in kind: the server parsed the request and rejected its content, so sending the identical bytes again produces the identical rejection — 422 falls outside `RetryStatuses` and is never retried. `ShouldRetry` lets a caller override this entirely, but the default logic rests on one question — could a later attempt of the *same* request plausibly succeed?

That question has a sharp edge for `SystemOne`, an HTTP `POST` the TypeSafe API gives no idempotency key to make retrying safe. A timeout or connection reset — exactly what `RetryOnTimeout` and `RetryOnConnErr` retry by default — doesn't tell the client whether the server already processed the request; it may have gone through and just failed to return a response in time. Retrying then repeats the same logical request, and if the first attempt landed, the caller can be billed twice for one logical call. This is a limitation of the external API, not the SDK: callers who cannot tolerate double-billing should set `RetryOnTimeout: false` and `RetryOnConnErr: false`, accepting that it then surfaces immediately as an error instead of being retried.

## Backoff: growing delays, damped by jitter

When a retry is warranted, the delay before it grows exponentially: for attempt `n`, the raw delay is `min(InitialDelay * 2^n, MaxDelay)`, doubling from `InitialDelay` (500ms) toward the `MaxDelay` ceiling (5s), both by default. Growth alone spreads retries over time but doesn't prevent synchronization: clients hitting the same failure at once would otherwise retry on the same schedule, in lockstep. `Jitter` (default 0.25) breaks that by shaving the computed delay down by a random fraction between 0 and `Jitter`, spreading retries across a small window instead of one spike. `MaxRetries` (default 2) bounds how many times this repeats, and 0 disables retries outright.

`HonorRetryAfter` (default true) overrides that computed delay whenever the server states one via `retry-after-ms` or `Retry-After` — but only up to `MaxRetryAfter` (default 5 minutes), which clamps down any parsed duration that exceeds it. The server usually knows better than the client how long its own rate limit or overload will last, so its stated wait still wins over the client's guess — up to that ceiling. Without one, a misconfigured or malicious server could ask the client to wait an hour, a day, or longer, and be obeyed.

## What the budget bounds

`Budget` (default 30s; 0 means unlimited) doesn't limit how many times the client retries, nor does it cap the call's total wall-clock time — it bounds when a new retry attempt is allowed to *start*. The check runs once per retry, right before the client would sleep for the next backoff or server-supplied delay: if that sleep would push elapsed time past the budget, the retry is skipped and the call returns the last error immediately. An attempt already in flight sits outside this check — `Budget` never interrupts a request actively waiting on the network, so one slow attempt can still take arbitrarily long. To bound a single attempt's duration, use `ctx` or the per-attempt `WithTimeout` / `WithRequestTimeout`; `Budget` governs retry scheduling, not attempt duration.

`ctx` cancellation sits above all of this: if the caller's context is cancelled, even mid-backoff, the call aborts immediately rather than finishing that sleep — cancellation is the caller's explicit statement it no longer wants the result, and waiting out the backoff first would silently delay a cooperative shutdown.

## Why the body is fixed before the first attempt

The request body is marshaled to JSON once, before the first attempt, and those bytes are replayed unchanged on every retry rather than re-encoding `state` and `questions` each time. This guarantees a retried request is byte-identical to the original, with no risk of a second marshal producing different output and no need to re-validate on each attempt.

## Related documentation

- For the client and per-request options that configure this policy — `WithRetryPolicy`, `WithRequestRetry`, and the environment variables behind them — see [Client options and environment](../reference/client-options-and-environment.md).
- For the error types a retried call can still surface, including `RateLimitError` and the `IsRetryable` predicate, see [Errors and exit codes](../reference/errors-and-exit-codes.md).
- For a task-oriented walkthrough of tuning retry behavior to your own rate limits and latency budgets, see [Handle rate limits and retries](../how-to/handle-rate-limits-and-retries.md).
