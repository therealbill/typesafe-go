---
title: "Errors and Exit Codes"
description: "The typesafe package's error types and predicate functions (APIError, RateLimitError, ConnectionError, TimeoutError, ResponseValidationError, ValidationError, ErrMissingAPIKey, IsAuthError, IsRateLimited, IsRetryable), and the jev CLI's exit code table and error JSON shape."
diataxis: reference
weight: 40
---

Package: `github.com/therealbill/typesafe-go` (import path `typesafe`), source `errors.go`. The `jev` CLI's exit-code and error-JSON behavior is implemented in `internal/cli/exit.go`.

## Error types

Every error type below satisfies `errors.As` and `errors.Is` through an `Unwrap() error` method: `RateLimitError.Unwrap` and `TimeoutError.Unwrap` return a pointer to their embedded struct (`&e.APIError`, `&e.ConnectionError`); `ValidationError.Unwrap`, `ConnectionError.Unwrap`, and `ResponseValidationError.Unwrap` return the wrapped `Err` field.

### ValidationError

```go
type ValidationError struct {
    Path string
    Err  error
}
```

ValidationError reports a request that failed client-side validation before any network call. `Path` names the offending field, for example `"questions.tone.criteria"`.

| Method | Signature | Description |
|---|---|---|
| `Error` | `func (e *ValidationError) Error() string` | Returns `"typesafe: invalid request: <Path>: <Err>"`. |
| `Unwrap` | `func (e *ValidationError) Unwrap() error` | Returns `Err`. |

### APIError

```go
type APIError struct {
    Status    int
    Body      []byte
    Headers   http.Header
    Endpoint  string
    RequestID string
}
```

APIError is returned when the API responds with a 4xx or 5xx status.

| Field | Type | Description |
|---|---|---|
| `Status` | `int` | The HTTP status code. |
| `Body` | `[]byte` | The raw response body. |
| `Headers` | `http.Header` | The response headers. |
| `Endpoint` | `string` | The method and path, for example `"POST /v1/systemone"`. |
| `RequestID` | `string` | The `x-typesafe-request-id` response header, if present. |

#### (\*APIError) Error

```go
func (e *APIError) Error() string
```

Returns `"typesafe: <Endpoint> returned <Status>"`, with `": <Message()>"` appended when `Message()` is non-empty, and `" (request id <RequestID>)"` appended when `RequestID` is non-empty.

#### (\*APIError) Message

```go
func (e *APIError) Message() string
```

Message extracts a human-readable message from the body. It reads the API's `"detail"` envelope in its string, object, and array forms, falls back to the `"message"` or `"error"` fields, and otherwise returns the trimmed body. Every one of those paths returns through the same final step, which coerces the message to valid UTF-8 (`strings.ToValidUTF8(msg, "�")`, replacing invalid byte sequences with `�`) and then truncates it to 200 bytes (`maxMessageLen`) without splitting a multi-byte UTF-8 rune. `truncate` walks back to the nearest rune boundary before cutting, appending `"..."` when a cut was made.

### RateLimitError

```go
type RateLimitError struct {
    APIError
    RetryAfter time.Duration
}
```

RateLimitError is an APIError with status 429. `RetryAfter` is the wait the server requested through `Retry-After` or `retry-after-ms`, or zero.

| Method | Signature | Description |
|---|---|---|
| `Unwrap` | `func (e *RateLimitError) Unwrap() error` | Returns `&e.APIError`. |

RateLimitError embeds `APIError`, so `Status`, `Body`, `Headers`, `Endpoint`, `RequestID`, `Error()`, and `Message()` are promoted from the embedded value.

### ConnectionError

```go
type ConnectionError struct {
    Err error
}
```

ConnectionError wraps a transport failure: DNS, dial, TLS, a reset connection, or a cancelled context.

| Method | Signature | Description |
|---|---|---|
| `Error` | `func (e *ConnectionError) Error() string` | Returns `"typesafe: connection error: <Err>"`. |
| `Unwrap` | `func (e *ConnectionError) Unwrap() error` | Returns `Err`. |

### TimeoutError

```go
type TimeoutError struct {
    ConnectionError
    Timeout time.Duration
}
```

TimeoutError is a ConnectionError caused by the per-request timeout.

| Method | Signature | Description |
|---|---|---|
| `Error` | `func (e *TimeoutError) Error() string` | Returns `"typesafe: request timed out after <Timeout>: <Err>"`, where `<Err>` is the embedded `ConnectionError`'s `Err` field. |
| `Unwrap` | `func (e *TimeoutError) Unwrap() error` | Returns `&e.ConnectionError`. |

TimeoutError embeds `ConnectionError`, so its `Err` field is promoted from the embedded value.

### ResponseValidationError

```go
type ResponseValidationError struct {
    FieldPath string
    Err       error
}
```

ResponseValidationError reports a 2xx response whose body did not match the expected shape. `FieldPath` is dotted, for example `"answers.tone.confidence"`.

| Method | Signature | Description |
|---|---|---|
| `Error` | `func (e *ResponseValidationError) Error() string` | Returns `"typesafe: invalid response field \"<FieldPath>\": <Err>"`. |
| `Unwrap` | `func (e *ResponseValidationError) Unwrap() error` | Returns `Err`. |

### ErrMissingAPIKey

```go
var ErrMissingAPIKey = errors.New("typesafe: API key is required: pass WithAPIKey or set TYPESAFE_API_KEY")
```

ErrMissingAPIKey is returned by `NewClient` when no API key is configured.

## Error predicate functions

| Function | Signature | Description |
|---|---|---|
| `IsAuthError` | `func IsAuthError(err error) bool` | Reports whether `err` is an `APIError` with status 401 or 403. Implemented as `errors.As(err, &api) && (api.Status == 401 \|\| api.Status == 403)`. |
| `IsRateLimited` | `func IsRateLimited(err error) bool` | Reports whether `err` is a `RateLimitError`. Implemented as `errors.As(err, &rl)`. |
| `IsRetryable` | `func IsRetryable(err error) bool` | Reports whether the default `RetryPolicy` would retry `err`. Implemented as `DefaultRetryPolicy().retryable(nil, err)`. |

## jev CLI: exit codes

Source: `internal/cli/exit.go`.

```go
const (
    ExitOK          = 0
    ExitUsage       = 1   // bad flags, unreadable or invalid request JSON, missing API key, or an unrecognized error
    ExitValidation  = 2   // request failed client-side validation
    ExitAuth        = 3   // 401 or 403
    ExitRequest     = 4   // other 4xx: 400, 404, 422
    ExitRateLimit   = 5   // 429 after retries
    ExitServer      = 6   // 5xx after retries, or an unreadable 2xx body
    ExitConnection  = 7   // connection failure or timeout
    ExitInterrupted = 130 // the context was cancelled, conventionally by SIGINT
)
```

The `"kind"` field of the error JSON (below) names the case more precisely
than the exit code: code 1 covers two distinct `kind` values, `"usage"` and
`"internal"`.

| Constant | Code | Condition |
|---|---|---|
| `ExitOK` | 0 | Success. |
| `ExitUsage` | 1 | Bad flags, unreadable or invalid request JSON, missing API key, or an unrecognized error. Covers `kind` `"usage"` and `kind` `"internal"`; see the classification table below. |
| `ExitValidation` | 2 | Request failed client-side validation. |
| `ExitAuth` | 3 | 401 or 403. |
| `ExitRequest` | 4 | Other 4xx: 400, 404, 422. |
| `ExitRateLimit` | 5 | 429 after retries. |
| `ExitServer` | 6 | 5xx after retries, or an unreadable 2xx body. |
| `ExitConnection` | 7 | Connection failure or timeout. |
| `ExitInterrupted` | 130 | The context was cancelled, conventionally by SIGINT. |

## jev CLI: error classification

`classify(err error) (int, string)` in `internal/cli/exit.go` maps a returned error to an exit code and to the `"kind"` string carried in the error JSON (below). The checks run in this order; the first match wins, each using `errors.As`/`errors.Is` so a wrapped error matches whenever it satisfies the target type or value via `Unwrap()`.

| Order | Go error type/value checked | Condition | Exit code | `kind` |
|---|---|---|---|---|
| 1 | `errors.Is(err, context.Canceled)` | any | `ExitInterrupted` (130) | `"interrupted"` |
| 2 | `*usageError`, or `errors.Is(err, typesafe.ErrMissingAPIKey)` | any | `ExitUsage` (1) | `"usage"` |
| 3 | `*typesafe.ValidationError` | any | `ExitValidation` (2) | `"validation"` |
| 4 | `*typesafe.RateLimitError` | any | `ExitRateLimit` (5) | `"rate_limit"` |
| 5 | `*typesafe.APIError` | `Status == 401 \|\| Status == 403` | `ExitAuth` (3) | `"auth"` |
| 6 | `*typesafe.APIError` | `Status >= 500` | `ExitServer` (6) | `"server"` |
| 7 | `*typesafe.APIError` | otherwise (e.g. 400, 404, 422) | `ExitRequest` (4) | `"request"` |
| 8 | `*typesafe.ConnectionError` | any (includes `*typesafe.TimeoutError`, which embeds it) | `ExitConnection` (7) | `"connection"` |
| 9 | `*typesafe.ResponseValidationError` | any | `ExitServer` (6) | `"invalid_response"` |
| Default | none of the above | n/a | `ExitUsage` (1) | `"internal"` |

`*usageError` (order 2) marks errors caused by how the CLI itself was
invoked. See [Usage errors versus validation errors](#usage-errors-versus-validation-errors)
below. The final, unlabeled row is the fallback for an error that reached
`fail` but matched none of the typed cases; it is reported with `kind`
`"internal"` rather than `"usage"`, even though both share exit code 1.

The `"interrupted"` versus `"connection"` split is made by the CLI, not by
the library. At the library level both cases are the same Go type: when the
parent context passed to `SystemOne`/`ListModels` is done
(`wrapTransportError` in `transport.go` checks `parent.Err() != nil` first),
the request fails with `&typesafe.ConnectionError{Err: parent.Err()}`, the
same type whatever the cause, carrying whichever error that context holds.
`classify` is what separates them:

- A SIGINT-cancelled context (`Main` installs a `signal.NotifyContext` on `os.Interrupt` and `syscall.SIGTERM`) yields `context.Canceled`, which the order-1 `errors.Is(err, context.Canceled)` check catches → `"interrupted"` (130).
- A parent context that instead hit its own deadline yields `context.DeadlineExceeded`, which is not `context.Canceled`, so it falls through to the order-8 `*typesafe.ConnectionError` check → `"connection"` (7).

At the library level the same distinction comes from
`errors.Is(err, context.Canceled)`. `errors.As` against
`*typesafe.ConnectionError` alone does not separate the two cases. A
`*typesafe.TimeoutError` is a third, unrelated case: it is produced only by
the per-attempt timeout from `WithTimeout`/`WithRequestTimeout`, never by the
caller's own context.

### Usage errors versus validation errors

The CLI rejects four kinds of input before calling the library: malformed
top-level request JSON (missing `"questions"`, missing `"state"`), a blank or
whitespace-only flag-mode question key, an unknown field inside a JSON
question object, and `--state` given without any `--noul`/`--choice`/`--score`.
Each surfaces as `*usageError` (exit code 1, kind `"usage"`).
`*typesafe.ValidationError` (exit code 2, kind `"validation"`) is a separate
case: the library's own semantic validation of an already-well-formed
`Questions` map (for example a `Choice` with zero labels), performed after
the CLI has parsed the request.

Verified against the built binary:

```
$ ./bin/jev ask --state "hi" --noul billing="is this about billing" \
    --api-key dummy --base-url http://127.0.0.1:1 --timeout 1s --max-retries 0
{"error":{"kind":"connection","message":"typesafe: connection error: Post \"http://127.0.0.1:1/v1/systemone\": dial tcp 127.0.0.1:1: connect: connection refused"}}
jev: typesafe: connection error: Post "http://127.0.0.1:1/v1/systemone": dial tcp 127.0.0.1:1: connect: connection refused
$ echo $?
7
```

## jev CLI: error JSON shape

On failure, `fail` (`internal/cli/exit.go`) writes one JSON object to stdout:

```json
{
  "error": {
    "kind": "<string>",
    "status": "<int, omitted if 0>",
    "request_id": "<string, omitted if empty>",
    "message": "<string>"
  }
}
```

| Field | Type | Presence | Value |
|---|---|---|---|
| `kind` | string | Always. | The `kind` returned by `classify(err)`. |
| `status` | int | Omitted when 0 (`json:",omitempty"`). | `(*typesafe.APIError).Status`, when `err` is (or wraps) an `*typesafe.APIError`. |
| `request_id` | string | Omitted when empty (`json:",omitempty"`). | `(*typesafe.APIError).RequestID`, when `err` is (or wraps) an `*typesafe.APIError`. |
| `message` | string | Always. | `err.Error()` by default; replaced with `(*typesafe.APIError).Message()` when `err` is (or wraps) an `*typesafe.APIError` and `Message()` is non-empty. |

`fail` also writes one line to stderr, regardless of `--pretty`:

```
jev: <err>
```

`<err>` is `sanitize(err.Error())`: `sanitize` (`internal/cli/exit.go`)
replaces every control character except tab with a `\xNN` escape, so a
response body containing terminal escape sequences cannot manipulate the
user's terminal. Newline (`\n`) is one of the escaped characters, so a
multi-line error message is collapsed onto this single stderr line (each
newline appears as `\x0a`) rather than spanning multiple lines. This
sanitization applies only to this stderr line, not to the stdout JSON, which
is already safely JSON-encoded (`encoding/json` escapes control characters on
its own).

Verified against the built binary (no `*typesafe.APIError` involved, so `status` and `request_id` are both absent):

```
$ env -u TYPESAFE_API_KEY ./bin/jev models
{"error":{"kind":"usage","message":"typesafe: API key is required: pass WithAPIKey or set TYPESAFE_API_KEY"}}
jev: typesafe: API key is required: pass WithAPIKey or set TYPESAFE_API_KEY
$ echo $?
1
```

## jev CLI: non-classified failures

`runMain` (`internal/cli/root.go`) calls `root.ExecuteContext(ctx)` and, when the returned error is not a `*ExitError` (for example a Cobra-level flag-parsing or unknown-command error that never reached `fail`/`classify`), it writes both streams, mirroring a classified failure:

- stdout: the same error JSON envelope shape as `fail` produces, with `kind` hardcoded to `"usage"` (this path does not call `classify`):

  ```json
  {"error": {"kind": "usage", "message": "<error>"}}
  ```

- stderr:

  ```
  jev: <sanitized error>
  Run 'jev --help' for usage.
  ```

The stdout JSON envelope is therefore written on every non-zero exit,
including a Cobra-level flag or command error rejected before any subcommand
runs. This path returns exit code 1 (`ExitUsage`).
