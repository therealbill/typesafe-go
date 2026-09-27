---
title: "Client Options and Environment"
description: "Client construction, functional options, per-call request options, configuration resolution order, defaults, environment variables, retry policy, and instrumentation hooks in the typesafe package."
type: reference
---

# Client Options and Environment

Package: `github.com/therealbill/typesafe-go` (import path `typesafe`).

## Client

```go
type Client struct {
    // Has unexported fields.
}
```

Client calls the TypeSafe API. It is safe for concurrent use.

### NewClient

```go
func NewClient(opts ...Option) (*Client, error)
```

NewClient builds a Client. It fails when no API key is configured, returning
`ErrMissingAPIKey` (documented on the errors and exit codes reference).

### (\*Client) Close

```go
func (c *Client) Close() error
```

Close releases idle connections held by a client-owned `http.Client`.

### (\*Client) SystemOne

```go
func (c *Client) SystemOne(ctx context.Context, state any, questions Questions, opts ...RequestOption) (*SystemOneResponse, error)
```

SystemOne asks the named questions about `state` and returns the typed
answers. `state` is a string, map, slice, or struct that encodes to JSON.
`Questions` and `SystemOneResponse` are documented on the question types and
response types references.

### (\*Client) ListModels

```go
func (c *Client) ListModels(ctx context.Context, opts ...RequestOption) (*ListModelsResponse, error)
```

ListModels returns the models available to the account. `ListModelsResponse`
is documented on the response types reference.

## Configuration resolution order

Defaults and environment variable names. Explicit options win over the
environment, which wins over these defaults.

| Setting | 1. Option | 2. Environment variable | 3. Default |
|---|---|---|---|
| API key | `WithAPIKey` | `TYPESAFE_API_KEY` | none — `NewClient` returns `ErrMissingAPIKey` if neither is set |
| Base URL | `WithBaseURL` | `TYPESAFE_BASE_URL` | `DefaultBaseURL` (`https://api.typesafe.ai`) |
| Model | `WithModel` | `TYPESAFE_DEFAULT_MODEL` | `DefaultModel` (`jev-latest`) |
| Logger | `WithLogger` (sets a `*slog.Logger` directly) | `TYPESAFE_LOG_LEVEL` (selects a text logger on stderr: `debug`, `info`, `warning`/`warn`, `error`) | no logging (any other value, including `off` and unset) |

## Defaults and environment variable constants

```go
const (
    DefaultBaseURL = "https://api.typesafe.ai"
    DefaultModel   = "jev-latest"
    DefaultTimeout = 10 * time.Second

    EnvAPIKey       = "TYPESAFE_API_KEY"
    EnvBaseURL      = "TYPESAFE_BASE_URL"
    EnvDefaultModel = "TYPESAFE_DEFAULT_MODEL"
    EnvLogLevel     = "TYPESAFE_LOG_LEVEL"
)
```

| Name | Value |
|---|---|
| `DefaultBaseURL` | `https://api.typesafe.ai` |
| `DefaultModel` | `jev-latest` |
| `DefaultTimeout` | `10 * time.Second` |
| `EnvAPIKey` | `TYPESAFE_API_KEY` |
| `EnvBaseURL` | `TYPESAFE_BASE_URL` |
| `EnvDefaultModel` | `TYPESAFE_DEFAULT_MODEL` |
| `EnvLogLevel` | `TYPESAFE_LOG_LEVEL` |

## Option

```go
type Option func(*Client) error
```

Option configures a Client.

### Client-level option functions

| Function | Signature | Description |
|---|---|---|
| `WithAPIKey` | `func WithAPIKey(key string) Option` | Sets the API key. Otherwise `TYPESAFE_API_KEY` is used. |
| `WithBaseURL` | `func WithBaseURL(u string) Option` | Sets the API root, for example for a gateway. Otherwise `TYPESAFE_BASE_URL` or `https://api.typesafe.ai` is used. |
| `WithModel` | `func WithModel(m string) Option` | Sets the default model. Otherwise `TYPESAFE_DEFAULT_MODEL` or `jev-latest` is used. |
| `WithRetryPolicy` | `func WithRetryPolicy(p RetryPolicy) Option` | Replaces the default retry policy. |
| `WithTimeout` | `func WithTimeout(d time.Duration) Option` | Sets the timeout for each HTTP attempt. Default 10s. Zero disables the per-attempt timeout. |
| `WithHeaders` | `func WithHeaders(h http.Header) Option` | Adds headers to every request. |
| `WithHTTPClient` | `func WithHTTPClient(hc *http.Client) Option` | Uses a caller-supplied `http.Client`. The client is copied so the caller's value is not modified when instrumentation wraps its transport. |
| `WithInstrumentation` | `func WithInstrumentation(i Instrumentation) Option` | Attaches an observer, such as the otel subpackage's. |
| `WithLogger` | `func WithLogger(l *slog.Logger) Option` | Sets the logger. Otherwise `TYPESAFE_LOG_LEVEL` selects a text logger on stderr, and unset means no logging. |

## RequestOption

```go
type RequestOption func(*requestConfig)
```

RequestOption configures a single call.

### Per-call option functions

| Function | Signature | Description |
|---|---|---|
| `WithRequestModel` | `func WithRequestModel(m string) RequestOption` | Overrides the client's model for this call. |
| `WithRequestRetry` | `func WithRequestRetry(p RetryPolicy) RequestOption` | Overrides the retry policy for this call. |
| `WithRequestTimeout` | `func WithRequestTimeout(d time.Duration) RequestOption` | Overrides the per-attempt timeout for this call. |
| `WithExtraHeaders` | `func WithExtraHeaders(h http.Header) RequestOption` | Adds headers to this call, replacing client headers with the same name. |
| `WithExtraBody` | `func WithExtraBody(fields map[string]any) RequestOption` | Merges fields into the top level of the request body. Use it for API fields this package does not model yet. |

## NewLogger

```go
func NewLogger(w io.Writer, level string) *slog.Logger
```

NewLogger returns a text logger on `w` at the named level: `debug`, `info`,
`warning` (or `warn`), `error`. Any other value, including `"off"` and `""`,
returns a logger that discards everything.

## RetryPolicy

```go
type RetryPolicy struct {
    MaxRetries      int
    InitialDelay    time.Duration
    MaxDelay        time.Duration
    Jitter          float64
    Budget          time.Duration
    RetryStatuses   []int
    RetryOnConnErr  bool
    RetryOnTimeout  bool
    HonorRetryAfter bool
    ShouldRetry     func(resp *http.Response, err error) bool
}
```

RetryPolicy controls how failed requests are retried. Start from
`DefaultRetryPolicy` and change fields; a zero RetryPolicy disables retries.

| Field | Type | Description | Default (`DefaultRetryPolicy`) |
|---|---|---|---|
| `MaxRetries` | `int` | Number of retries after the first attempt. 0 disables retries. | `2` |
| `InitialDelay` | `time.Duration` | Delay before the first retry. | `500ms` |
| `MaxDelay` | `time.Duration` | Caps the exponential backoff. | `5s` |
| `Jitter` | `float64` | Fraction of each delay randomly subtracted, from 0 to 1. | `0.25` |
| `Budget` | `time.Duration` | Total time allowed per call including delays. 0 means unlimited. | `30s` |
| `RetryStatuses` | `[]int` | HTTP statuses that are retried. | `408, 429, 500–599` |
| `RetryOnConnErr` | `bool` | Retries connection failures. | `true` |
| `RetryOnTimeout` | `bool` | Retries per-request timeouts. | `true` |
| `HonorRetryAfter` | `bool` | Uses `Retry-After` and `retry-after-ms` headers as the delay. | `true` |
| `ShouldRetry` | `func(resp *http.Response, err error) bool` | When set, replaces every other decision. `resp` may be nil. | `nil` (unset) |

### DefaultRetryPolicy

```go
func DefaultRetryPolicy() RetryPolicy
```

DefaultRetryPolicy returns the policy used when none is configured. It
matches the official Python SDK's defaults.

## Instrumentation

```go
type Instrumentation interface {
    // RequestStart is called once per SystemOne or ListModels call. The
    // returned context is used for every HTTP attempt. The returned function
    // is called exactly once when the call finishes.
    RequestStart(ctx context.Context, info RequestInfo) (context.Context, func(RequestResult))
    // Transport wraps the HTTP round tripper used by the client. It is called
    // once at client construction and may return rt unchanged.
    Transport(rt http.RoundTripper) http.RoundTripper
}
```

Instrumentation observes client calls. The otel subpackage provides an
OpenTelemetry implementation; its span attribute keys are documented on the
span attributes reference.

## RequestInfo

```go
type RequestInfo struct {
    Operation     string
    Model         string
    QuestionCount int
    NoulCount     int
    ChoiceCount   int
    ScoreCount    int
    State         any
    Questions     Questions
}
```

RequestInfo describes a call about to be made. It is passed to
`Instrumentation.RequestStart` before the first HTTP attempt.

| Field | Type | Description |
|---|---|---|
| `Operation` | `string` | `"system_one"` or `"list_models"`. |
| `Model` | `string` | The requested model name or alias. |
| `QuestionCount` | `int` | Total number of questions in the map. |
| `NoulCount` | `int` | Number of `Noul` questions. |
| `ChoiceCount` | `int` | Number of `Choice` questions. |
| `ScoreCount` | `int` | Number of `Score` questions. |
| `State` | `any` | The request input state. Instrumentation must not record it unless the caller opted in. |
| `Questions` | `Questions` | The request input questions. Instrumentation must not record them unless the caller opted in. |

## RequestResult

```go
type RequestResult struct {
    Attempts  int
    Status    int
    RequestID string
    Model     string
    Usage     Usage
    Response  *SystemOneResponse
    Err       error
}
```

RequestResult describes how a call ended. It is passed to the function
returned by `Instrumentation.RequestStart` exactly once.

| Field | Type | Description |
|---|---|---|
| `Attempts` | `int` | Number of HTTP attempts made, including the first. |
| `Status` | `int` | Final HTTP status, or 0 if no response was received. |
| `RequestID` | `string` | The `x-typesafe-request-id` header of the final response. |
| `Model` | `string` | Model reported by the response, if any. |
| `Usage` | `Usage` | Token usage reported by the response, if any. See the response types reference. |
| `Response` | `*SystemOneResponse` | Decoded response for `system_one`, nil otherwise or on error. See the response types reference. |
| `Err` | `error` | Error returned to the caller, nil on success. |
