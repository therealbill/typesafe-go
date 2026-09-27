---
title: "Retries and Budgets"
description: "Why gojev's retry policy is shaped the way it is: bounded backoff, a total time budget, and which failures are worth retrying at all."
type: explanation
---

# Retries and Budgets

Every call to `SystemOne` or `ListModels` can fail for reasons unrelated to the request itself: the network hiccups, System One is momentarily overloaded, or a rate limit kicks in. The retry policy exists to smooth over that class of failure without a hand-rolled retry loop at every call site. But a policy that just "tries harder" is dangerous on its own — it can turn one slow response into a much slower one, or a fleet of clients into a thundering herd hitting an already-struggling server. `RetryPolicy` rests on three ideas that keep retries useful instead of harmful: bounded exponential backoff with jitter, a hard ceiling on call duration, and a sharp line between failures worth retrying and failures that will just fail again.

## What gets retried, and why

`RetryStatuses` defaults to 408, 429, and the whole 500–599 range, and `RetryOnConnErr` / `RetryOnTimeout` both default to true. The idea behind that set isn't "any error" — it's "any failure where the same request might succeed if sent again." A dropped connection, a timeout, a 429, and a 529 overloaded response are all transient: nothing about the request was wrong, the server or network just couldn't handle it this time.

A 422 is different in kind: it means the server parsed the request and rejected its content. Sending the identical bytes again produces the identical rejection, so 422 falls outside `RetryStatuses` and is never retried — that would only add latency to a call already known to fail. The line is status-based, not severity-based: 529 sits inside 500–599 and is retried despite sounding dire, while 422 sits in the 4xx range because it signals a malformed request, not a server or network hiccup. `ShouldRetry` lets a caller override this entirely, but the default logic rests on one question — could a later attempt of the *same* request plausibly succeed?

## Backoff: growing delays, damped by jitter

When a retry is warranted, the delay before it grows exponentially: for attempt `n`, the raw delay is `min(InitialDelay * 2^n, MaxDelay)`, doubling from `InitialDelay` (500ms) toward the `MaxDelay` ceiling (5s), both by default. Growth alone spreads retries over time but doesn't prevent synchronization: if many clients hit the same failure at once — a shared rate limit, a brief outage — they'd all back off on the same schedule and retry at the same instant again, in lockstep. `Jitter` (default 0.25) breaks that lockstep by shaving the computed delay down by a uniform random fraction between 0 and `Jitter`, so clients nominally waiting "the same" delay actually spread across a small window instead of arriving as one synchronized spike. `MaxRetries` (default 2) bounds how many times this repeats — at most two delays by default — and 0 disables retries outright.

`HonorRetryAfter` (default true) overrides that computed delay whenever the server is explicit about it: if the response carries `retry-after-ms`, or `Retry-After` as seconds or an HTTP date, that value replaces whatever backoff math would otherwise apply. The server knows better than the client how long its own rate limit or overload will last, so its stated wait wins over the client's guess.

## The budget bounds worst-case latency, deliberately

`Budget` (default 30s; 0 means unlimited) doesn't limit how many times the client retries — it limits the wall-clock time one call may spend doing so. Before sleeping for a computed or server-supplied delay, the client checks whether that sleep would push the call past its remaining budget. If it would, the retry is skipped and the call returns the last error immediately rather than sleeping further. That trade is deliberate: without a budget, a caller could wait through the full sequence of growing delays with no ceiling on the total, turning one call into an unpredictable, possibly multi-minute hang. The budget turns "retry until `MaxRetries` is exhausted" into "retry until the declared time runs out" — a bound the caller can reason about, at the cost of occasionally surfacing a failure one more retry might have cleared.

`ctx` cancellation sits above all of this: if the caller's context is cancelled, even mid-backoff, the call aborts immediately rather than finishing that sleep first. Retry timing is the SDK's internal bookkeeping, but cancellation is the caller's explicit statement that it no longer wants the result; waiting out the current backoff first would silently delay a cooperative shutdown by up to `MaxDelay`, defeating the point of passing a `ctx` at all.

## Why the body is fixed before the first attempt

The request body is marshaled to JSON once, before the first attempt, and those bytes are replayed unchanged on every retry rather than re-encoding `state` and `questions` each time. This guarantees a retried request is byte-identical to the original — no risk of a second marshal producing subtly different output, and no need to re-validate on each attempt, since validation already ran once against the very bytes every attempt sends.

## Related documentation

- For the client and per-request options that configure this policy — `WithRetryPolicy`, `WithRequestRetry`, and the environment variables behind them — see [Client options and environment](../reference/client-options-and-environment.md).
- For the error types a retried call can still surface, including `RateLimitError` and the `IsRetryable` predicate, see [Errors and exit codes](../reference/errors-and-exit-codes.md).
- For a task-oriented walkthrough of tuning retry behavior to your own rate limits and latency budgets, see [Handle rate limits and retries](../how-to/handle-rate-limits-and-retries.md).
