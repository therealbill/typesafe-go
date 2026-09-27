---
title: "Errors and Exit Codes"
description: "The typesafe package's error types and predicate functions (APIError, RateLimitError, ConnectionError, TimeoutError, ResponseValidationError, ValidationError, ErrMissingAPIKey, IsAuthError, IsRateLimited, IsRetryable), and the jev CLI's exit code table and error JSON shape."
type: reference
---

# Errors and Exit Codes

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

Message extracts a human-readable message from the body. It understands the API's `"detail"` envelope in its string, object, and array forms, falls back to `"message"` or `"error"` fields, and otherwise returns the trimmed body, truncated to 200 bytes.

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
    ExitOK         = 0
    ExitUsage      = 1 // bad flags, unreadable or invalid request JSON, missing API key
    ExitValidation = 2 // request failed client-side validation
    ExitAuth       = 3 // 401 or 403
    ExitRequest    = 4 // other 4xx: 400, 404, 422
    ExitRateLimit  = 5 // 429 after retries
    ExitServer     = 6 // 5xx after retries, or an unreadable 2xx body
    ExitConnection = 7 // connection failure or timeout
)
```

| Constant | Code | Condition |
|---|---|---|
| `ExitOK` | 0 | Success. |
| `ExitUsage` | 1 | Bad flags, unreadable or invalid request JSON, missing API key. |
| `ExitValidation` | 2 | Request failed client-side validation. |
| `ExitAuth` | 3 | 401 or 403. |
| `ExitRequest` | 4 | Other 4xx: 400, 404, 422. |
| `ExitRateLimit` | 5 | 429 after retries. |
| `ExitServer` | 6 | 5xx after retries, or an unreadable 2xx body. |
| `ExitConnection` | 7 | Connection failure or timeout. |

## jev CLI: error classification

`classify(err error) (int, string)` in `internal/cli/exit.go` maps a returned error to an exit code and to the `"kind"` string carried in the error JSON (below). The checks run in this order; the first match wins, each using `errors.As` so a wrapped error matches whenever it satisfies the target type via `Unwrap()`.

| Order | Go error type checked | Condition | Exit code | `kind` |
|---|---|---|---|---|
| 1 | `*typesafe.ValidationError` | any | `ExitValidation` (2) | `"validation"` |
| 2 | `*typesafe.RateLimitError` | any | `ExitRateLimit` (5) | `"rate_limit"` |
| 3 | `*typesafe.APIError` | `Status == 401 \|\| Status == 403` | `ExitAuth` (3) | `"auth"` |
| 4 | `*typesafe.APIError` | `Status >= 500` | `ExitServer` (6) | `"server"` |
| 5 | `*typesafe.APIError` | otherwise (e.g. 400, 404, 422) | `ExitRequest` (4) | `"request"` |
| 6 | `*typesafe.ConnectionError` | any (includes `*typesafe.TimeoutError`, which embeds it) | `ExitConnection` (7) | `"connection"` |
| 7 | `*typesafe.ResponseValidationError` | any | `ExitServer` (6) | `"invalid_response"` |
| — | none of the above | — | `ExitUsage` (1) | `"usage"` |

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

Verified against the built binary (no `*typesafe.APIError` involved, so `status` and `request_id` are both absent):

```
$ env -u TYPESAFE_API_KEY ./bin/jev models
{"error":{"kind":"usage","message":"typesafe: API key is required: pass WithAPIKey or set TYPESAFE_API_KEY"}}
jev: typesafe: API key is required: pass WithAPIKey or set TYPESAFE_API_KEY
$ echo $?
1
```

## jev CLI: non-classified failures

`Main` (`internal/cli/root.go`) calls `root.ExecuteContext(ctx)` and, when the returned error is not a `*ExitError` (for example a Cobra-level argument-parsing error that never reached `fail`/`classify`), writes to stderr:

```
jev: <error>
Run 'jev --help' for usage.
```

and returns exit code 1 (`ExitUsage`) without writing anything to stdout.
