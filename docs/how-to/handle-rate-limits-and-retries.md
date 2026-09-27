---
title: "How to Handle Rate Limits and Retries"
description: "Customize the SDK's retry policy client-wide or per call, detect rate limits with RateLimitError, and decide when your own code should retry a whole batch."
type: how-to
---

# How to Handle Rate Limits and Retries

**Goal**: Configure how `typesafe-go` retries failed requests, detect a rate
limit explicitly, and use `IsRetryable` to drive your own higher-level retry
logic.

## Prerequisites

- A working `SystemOne` call — see
  [Your First Judgment in Go](../tutorials/first-judgment-in-go.md)
- Familiarity with `errors.As` error handling
- For why the policy is shaped this way, see
  [Retries and budgets](../explanation/retries-and-budgets.md)

## Steps

### 1. Start from `DefaultRetryPolicy` and override only what you need

A bare `RetryPolicy{}` only sets `MaxRetries` to 0 — every other field
(`Budget`, `RetryOnConnErr`, `RetryOnTimeout`, `HonorRetryAfter`) is left at
its Go zero value, not the library's default. Always start from
`DefaultRetryPolicy()` and change specific fields:

```go
policy := typesafe.DefaultRetryPolicy()
policy.MaxRetries = 8            // default: 2
policy.Budget = 10 * time.Second // default: 30s

client, err := typesafe.NewClient(
	typesafe.WithAPIKey("your-api-key"),
	typesafe.WithRetryPolicy(policy),
)
```

Every call made with `client` now retries up to 8 times, bounded to a 10s
wall-clock budget per call.

### 2. Override the policy for a single call

`WithRequestRetry` is a `RequestOption`, so it applies only to the call it's
passed to — it never changes the client's configured policy for any other
call:

```go
perCall := typesafe.DefaultRetryPolicy()
perCall.MaxRetries = 5
perCall.Budget = 3 * time.Second

res, err := client.SystemOne(ctx, state, questions,
	typesafe.WithRequestRetry(perCall),
)
```

Use this for a call with different latency tolerance than the rest of your
traffic — a background batch job, say, versus a request on the critical path
of a user-facing request.

Per-call options like `WithRequestRetry` are validated eagerly: `RequestOption`
is `func(*requestConfig) error`, so a malformed value — a negative duration
passed to `WithRequestTimeout`, for example — returns a
`*typesafe.ValidationError` immediately, before any network call. It surfaces
through the same `err` returned from `SystemOne`/`SystemOneAs` above, so no
special error-handling pattern is needed — just don't assume only server
responses can produce an error here.

### 3. Detect a rate limit and read `RetryAfter`

When the API returns 429, the SDK surfaces a `*typesafe.RateLimitError`,
which embeds `APIError` and adds `RetryAfter time.Duration` parsed from the
response's `Retry-After` header — the server's raw requested wait,
unclamped, for your own code to inspect. Recognize it with `errors.As`:

```go
var rateLimitErr *typesafe.RateLimitError
if errors.As(err, &rateLimitErr) {
	fmt.Println("rate limited, retry after:", rateLimitErr.RetryAfter)
}
```

Separately, `RetryPolicy.MaxRetryAfter` (default 5 minutes, backfilled to
that default automatically even in a hand-built policy) caps how long the
SDK's *own* automatic retry loop actually waits when `HonorRetryAfter` is
true and the server asks for longer. `HonorRetryAfter` no longer means "wait
however long the server says" — it means "wait what the server says, up to
`MaxRetryAfter`." This clamp never touches `RateLimitError.RetryAfter`
itself, only the SDK's internal sleep.

To see this without a live rate limit, disable the SDK's own retries
(`MaxRetries: 0`) so the 429 surfaces immediately instead of being retried
away, and point the client at a fake server:

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Retry-After", "2")
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte(`{"detail":"rate limited"}`))
}))
defer server.Close()

client, _ := typesafe.NewClient(
	typesafe.WithAPIKey("dummy-api-key"),
	typesafe.WithBaseURL(server.URL),
	typesafe.WithRetryPolicy(typesafe.RetryPolicy{MaxRetries: 0}),
)

_, err := client.SystemOne(ctx, state, questions)
var rateLimitErr *typesafe.RateLimitError
if errors.As(err, &rateLimitErr) {
	fmt.Println("rate limited, retry after:", rateLimitErr.RetryAfter)
}
```

### 4. Let a caller-owned retry loop use `IsRetryable`

`IsRetryable(err)` reports whether `DefaultRetryPolicy` would retry that
error — independent of whatever policy the client is actually configured
with. Reach for it when you're retrying something bigger than one call, such
as a whole batch, and want to give up immediately on errors that will never
succeed no matter how many times you try:

```go
func runBatch(ctx context.Context, client *typesafe.Client) error {
	_, err := client.SystemOne(ctx, state, questions)
	return err
}

var lastErr error
for attempt := 0; attempt < 3; attempt++ {
	lastErr = runBatch(ctx, client)
	if lastErr == nil {
		break
	}
	if !typesafe.IsRetryable(lastErr) {
		return lastErr // not worth retrying the batch
	}
	time.Sleep(50 * time.Millisecond)
}
```

### 5. Disable retries entirely

Pass a `RetryPolicy` whose `MaxRetries` is 0 — either the zero value or
explicitly:

```go
typesafe.WithRetryPolicy(typesafe.RetryPolicy{})
// equivalent, more explicit:
typesafe.WithRetryPolicy(typesafe.RetryPolicy{MaxRetries: 0})
```

Both disable retries: the SDK's retry loop checks `retry >=
policy.MaxRetries`, which is already true before the first retry when
`MaxRetries` is 0.

## Verify it works

Running the Step 3 program against the fake 429 server produces:

```
rate limited, retry after: 2s
```

`RetryAfter` is exactly `2 * time.Second`, matching the `Retry-After: 2`
header.

✅ The client surfaces rate limits as a typed error you can branch on,
instead of a bare status code.

## Troubleshooting

### Problem: a hand-built `RetryPolicy{MaxRetries: N}` doesn't retry connection errors or honor `Retry-After`
**Symptom**: retries happen for 5xx/429 but not for dropped connections, and
a `Retry-After` header seems ignored.
**Cause**: `RetryPolicy{MaxRetries: N}` sets every other field to its Go
zero value — `RetryOnConnErr`, `RetryOnTimeout`, and `HonorRetryAfter` are
all `false` unless you started from `DefaultRetryPolicy()`.
**Solution**: always build from `DefaultRetryPolicy()` and override
individual fields, as in Steps 1 and 2.

### Problem: `IsRetryable` says an error is retryable, but the client already gave up
**Symptom**: `IsRetryable(err)` returns `true` even though the client's own
policy already exhausted its retries or budget.
**Cause**: `IsRetryable` checks the error against `DefaultRetryPolicy()`'s
classification, not the client's configured policy or how many attempts
already happened.
**Solution**: use it to decide whether *your* retry loop should try again,
not to infer what the client already did.

### Problem: calls take longer than expected under load
**Symptom**: a single `SystemOne` call blocks for many seconds during an
outage.
**Cause**: `Budget` (default 30s) caps total retry time per call, but a
large `MaxRetries` combined with a large `Budget` can still add up real
wall-clock time.
**Solution**: lower `Budget` for latency-sensitive calls (Step 1 or 2), or
set `MaxRetries: 0` where a fast failure beats a slow retry.

## Next steps

- Read [Retries and budgets](../explanation/retries-and-budgets.md) for why
  the policy is shaped this way.
- See every `RetryPolicy` field and its default on the
  [client options and environment reference](../reference/client-options-and-environment.md).

## See also

- [Your First Judgment in Go](../tutorials/first-judgment-in-go.md)
- [Client options and environment](../reference/client-options-and-environment.md)
- [Retries and budgets](../explanation/retries-and-budgets.md)
