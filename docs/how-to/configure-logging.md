---
title: "How to Configure Logging"
description: "Set the SDK's log level with WithLogger or TYPESAFE_LOG_LEVEL, know what debug/warning/error each log, and control jev's own stderr logging with --log-level."
diataxis: how-to
weight: 90
---

# How to Configure Logging

**Goal**: Turn on `typesafe-go`'s request logging at the level you need, know
exactly what each level logs (and that it never logs your API key or request
bodies), and control it the same way from `jev`.

## Prerequisites

- A working `SystemOne` call — see
  [Your First Judgment in Go](../tutorials/first-judgment-in-go.md)
- Familiarity with `log/slog`, which `typesafe-go` logs through directly

## Steps

### 1. Let `TYPESAFE_LOG_LEVEL` pick a default logger, or set one explicitly

If you don't call `WithLogger`, `NewClient` builds one from the
`TYPESAFE_LOG_LEVEL` environment variable via `NewLogger`. The accepted
values, case-insensitive and trimmed, are `debug`, `info`, `warning` (or
`warn`), and `error`. Anything else — including `off`, empty, or unset —
returns a logger that discards everything:

```go
client, err := typesafe.NewClient(
	typesafe.WithAPIKey("your-api-key"),
	// no WithLogger: TYPESAFE_LOG_LEVEL decides, or logging is off
)
```

To set the level explicitly in code instead of through the environment, use
`NewLogger` directly and pass the result to `WithLogger`:

```go
client, err := typesafe.NewClient(
	typesafe.WithAPIKey("your-api-key"),
	typesafe.WithLogger(typesafe.NewLogger(os.Stderr, "debug")),
)
```

### 2. Or supply your own `*slog.Logger` entirely

`WithLogger` accepts any `*slog.Logger` — route it into your application's
existing logging setup (JSON handler, a different sink, extra attributes)
instead of the SDK's own text-on-stderr default:

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
client, err := typesafe.NewClient(
	typesafe.WithAPIKey("your-api-key"),
	typesafe.WithLogger(logger),
)
```

### 3. Know what each level actually logs

From `transport.go`'s request loop:

- **debug** logs every successful attempt: `"typesafe request ok"` with
  `endpoint`, `status`, `attempt`, `request_id`, and `elapsed`.
- **warn** logs each retry attempt (`"typesafe retrying"`), retry-budget
  exhaustion (`"typesafe retry budget exhausted"`), and a *final* failure
  when the resulting `APIError.Status` is below 500 — a 4xx is treated as
  the caller's mistake, not an incident.
- **error** logs a final failure otherwise: a 5xx `APIError`, or any
  non-API error such as a connection failure or timeout.

Debug implies everything at warn and error too, since `slog` levels are a
threshold, not a filter on individual messages.

### 4. Confirm what's never logged

The `Authorization` header and request/response bodies never appear in any
log call in `transport.go` — every log line names only `endpoint`, `status`,
`attempt`, `request_id`, `elapsed`, and `error`. `(*Client).String()` and
`(*Client).LogValue()` are both written the same way, so passing a `*Client`
itself to `fmt.Println` or `slog` never leaks the API key either.

### 5. Set the level from `jev`'s `--log-level` flag or `TYPESAFE_LOG_LEVEL`

`jev` always installs a logger — writing to `jev`'s own stderr — so
environment-driven logging lands on the stream you actually asked for
instead of the process's real stderr when you've redirected `jev`'s output
elsewhere. `--log-level` takes `debug|info|warning|error|off` and wins over
`TYPESAFE_LOG_LEVEL` when both are set:

```
jev --log-level debug ask -f request.json
```

## Verify it works

Running a normal request with `--log-level debug` prints exactly one line to
stderr for the successful attempt:

```
$ ./bin/jev --log-level debug ask -f request.json --pretty 1>/dev/null
time=2026-09-28T13:19:35.293-05:00 level=DEBUG msg="typesafe request ok" endpoint="POST /v1/systemone" status=200 attempt=1 request_id=req_01a0e93e63c97339b7e3cfc3274ea2da elapsed=183.532792ms
```

Deliberately triggering a 401 with a bad `--api-key` shows the warn path —
a 4xx logs at warn, not error:

```
$ ./bin/jev --log-level debug --api-key invalid-test-key-0000 ask -f request.json --pretty 1>/dev/null
time=2026-09-28T13:19:41.038-05:00 level=WARN msg="typesafe request failed" endpoint="POST /v1/systemone" attempt=1 error="typesafe: POST /v1/systemone returned 401: authentication_error: Cannot authenticate with the server. Please check your API key and try again. (request id req_01a0e93e7a767212b4082cb5fa7c9004)"
```

And a connection failure (pointed at an address nothing is listening on)
shows the error path:

```
$ ./bin/jev --log-level error --base-url http://127.0.0.1:1 --max-retries 0 ask -f request.json --pretty 1>/dev/null
time=2026-09-28T13:19:44.487-05:00 level=ERROR msg="typesafe request failed" endpoint="POST /v1/systemone" attempt=1 error="typesafe: connection error: Post \"http://127.0.0.1:1/v1/systemone\": dial tcp 127.0.0.1:1: connect: connection refused"
```

None of the three lines above contain an `Authorization` value, an API key,
or the request/response body.

✅ The level you pick controls exactly what's logged, and nothing logs the
request body or the key regardless of level.

## Troubleshooting

### Problem: nothing is logged at all
**Symptom**: no output on stderr no matter what happens.
**Cause**: `TYPESAFE_LOG_LEVEL` is unset (or set to something unrecognized,
including `off`), and no `--log-level`/`WithLogger` was given — the default
logger discards everything.
**Solution**: set `TYPESAFE_LOG_LEVEL=debug` (or `--log-level debug` for
`jev`), or pass `WithLogger` explicitly in code.

### Problem: a 4xx failure only shows up at warn, not error
**Symptom**: `--log-level error` shows nothing for a 400/401/422 failure.
**Cause**: this is intentional — the code comment in `transport.go` treats a
4xx as the caller's mistake, not an incident worth error-level severity.
**Solution**: use `--log-level warning` (or lower) to see 4xx failures, and
reserve `error` for what actually needs paging: 5xx responses and
connection failures.

### Problem: retries aren't visible even though the call eventually succeeds
**Symptom**: only the final "ok" line appears, no retry detail.
**Cause**: retry attempts log at warn (`"typesafe retrying"`); at a level
above warn (i.e. `error`), you only see the outcome, not the attempts.
**Solution**: use `debug` or `warning` to see the retry sequence, not just
the final result.

## Next steps

- [Trace calls and send them to Honeycomb](./trace-calls-and-send-to-honeycomb.md)
  for span-based observability alongside (or instead of) these logs.
- See every log call's attributes in
  [the instrumentation hook](../explanation/the-instrumentation-hook.md).

## See also

- [Client options and environment](../reference/client-options-and-environment.md)
- [jev CLI](../reference/jev-cli.md)
- [Handle rate limits and retries](./handle-rate-limits-and-retries.md)
