# TypeSafe Go SDK and `jev` CLI — Design

Date: 2026-09-23
Module: `github.com/therealbill/typesafe-go`
Package: `typesafe`

## Goal

A Go library that covers the public surface of the official TypeSafe Python SDK
(`typesafe-sdk`) for the System One API, plus a small `jev` CLI built on it.
The library is for use inside Go applications. The CLI exists so that the
TypeSafe Claude Code plugin can call Jev without generating and executing
Python.

The plugin itself, Windows builds, OTel metrics and logs, and request caching
are out of scope.

## API being wrapped

Source of truth: https://docs.typesafe.ai (`api.md`, `models.md`, and the
`sdk/python/*` pages). Summary as of this date:

- `POST https://api.typesafe.ai/v1/systemone` with `Authorization: Bearer <key>`.
  Body: `{"state": string|object|array, "model": string, "questions": {id: Question}}`.
- `GET /v1/models` returns `{"models": [{"name", "description", "release_date"}]}`;
  `release_date` is an RFC 3339 timestamp, kept as a string.
- Question types: `noul` (instructions, optional `criteria.true/false`),
  `choice` (instructions, `criteria` map of 1–255 labels to descriptions or
  null), `score` (instructions, `criteria` ordered array of 2–10 levels).
  Instructions and descriptions are "JSON content": string, object, or array.
- Answers: `noul` → `{"type":"noul","noul":p}`; `choice` →
  `{"type":"choice","choice","probabilities","confidence"}`; `score` →
  `{"type":"score","score","legend","probabilities","confidence"}`.
  Response also carries `model`, `usage.input_tokens`, `usage.output_tokens`,
  and header `x-typesafe-request-id`.
- Errors: 400, 401, 403, 404, 422, 429 (with `Retry-After` / `retry-after-ms`),
  5xx including 529 overloaded. Observed error bodies all use a top-level
  `detail` field whose value is a string (`{"detail":"Not Found"}`), an object
  (`{"detail":{"error_type":"authentication_error","message":"..."}}`), or a
  pydantic-style array of `{type, loc, msg, input}` items on 422.
- Models: `jev-1.13.0`; aliases `jev-latest`, `jev-preview`. Context 64k tokens.
- Python SDK defaults: base URL `https://api.typesafe.ai`, model `jev-latest`,
  per-operation timeout 10s, retries 2, backoff 0.5s→5s with 0.25 jitter,
  30s total budget per call, retry on 408/429/5xx/connection/timeout.
- Python env vars: `TYPESAFE_API_KEY`, `TYPESAFE_BASE_URL`,
  `TYPESAFE_DEFAULT_MODEL`, `TYPESAFE_LOG_LEVEL`.

## Repository layout

```
typesafe-go/
  go.mod                   module github.com/therealbill/typesafe-go (go 1.25)
  doc.go                   package documentation
  client.go                Client, NewClient, Option, SystemOne, ListModels
  question.go              Question, Noul, Choice, Score, RawQuestion, JSONContent, validation
  response.go              SystemOneResponse, Answer types, Usage, RawResponse
  models.go                ListModelsResponse, ModelMetadata
  errors.go                APIError, RateLimitError, ConnectionError, TimeoutError,
                           ResponseValidationError, ValidationError, predicates
  retry.go                 RetryPolicy, backoff, Retry-After parsing
  transport.go             request build, send, decode, logging (unexported)
  typed.go                 SystemOneAs[T]
  instrument.go            Instrumentation hook interface
  otel/                    OTel implementation of the hook (separate deps)
  internal/version/        Version string
  cmd/jev/                 main.go only
  internal/cli/            Cobra commands, exit codes, telemetry wiring (testable in-process)
  tools/selfreview/        Jev-driven review of spec vs. implementation vs. tests
  docs/                    Diátaxis documentation
  docs/superpowers/specs/  this document
  .github/workflows/       ci.yml, release.yml
  .goreleaser.yaml, .golangci.yml, Makefile, README.md
```

Dependency policy: the root package `typesafe` imports only the standard
library. `typesafe/otel` imports the OpenTelemetry API and `otelhttp`; GenAI
attribute names are defined as constants in the package because the Go
`semconv` packages do not ship them. `cmd/jev` imports Cobra, `typesafe/otel`, and
`go.opentelemetry.io/contrib/otelconf/x` for exporter setup. All three are
one Go module.

## Public surface: package `typesafe`

### Client

```go
func NewClient(opts ...Option) (*Client, error)

func WithAPIKey(string) Option
func WithBaseURL(string) Option
func WithModel(string) Option
func WithRetryPolicy(RetryPolicy) Option
func WithTimeout(time.Duration) Option          // per HTTP operation, default 10s
func WithHeaders(http.Header) Option
func WithHTTPClient(*http.Client) Option
func WithInstrumentation(Instrumentation) Option
func WithLogger(*slog.Logger) Option

func (c *Client) SystemOne(ctx context.Context, state any, questions Questions,
    opts ...RequestOption) (*SystemOneResponse, error)
func (c *Client) ListModels(ctx context.Context, opts ...RequestOption) (*ListModelsResponse, error)
func (c *Client) Close() error                  // closes idle connections on an owned http.Client

func WithRequestModel(string) RequestOption
func WithRequestRetry(RetryPolicy) RequestOption
func WithRequestTimeout(time.Duration) RequestOption
func WithExtraHeaders(http.Header) RequestOption
func WithExtraBody(map[string]any) RequestOption // shallow-merged into the top-level body

func SystemOneAs[T any](ctx context.Context, c *Client, state any, questions Questions,
    opts ...RequestOption) (T, *SystemOneResponse, error)
```

Resolution order for each setting: option, then env var, then default.
`NewClient` returns an error if no API key is available, matching the Python
SDK's fail-fast behavior. A `Client` is safe for concurrent use. There is no
async client; callers use goroutines and `ctx`.

### Questions

```go
type JSONContent = any            // string, map, slice, json.RawMessage, or a struct that encodes to an object; validated at send time

type Question interface{ question() } // closed set
type Questions map[string]Question

type NoulCriteria struct{ True, False JSONContent }
type Noul   struct{ Instructions JSONContent; Criteria *NoulCriteria }
type Choice struct{ Instructions JSONContent; Criteria map[string]JSONContent }
type Score  struct{ Instructions JSONContent; Criteria []JSONContent }
type RawQuestion map[string]any   // forward-compatibility escape hatch; sent as-is
```

Wire encoding matches the HTTP API exactly. `Noul.Criteria` is omitted when
nil; a nil `Choice` label value encodes as JSON `null`. Client-side validation
runs before any network call and returns `*ValidationError{Path, Err}` with
paths like `questions.tone.criteria`. Rules: choice 1–255 labels; score 2–10
levels; `JSONContent` must be a string, map, slice, array, `json.RawMessage`, or a
struct or pointer to struct that encodes to a JSON object (nil only where optional);
`questions` must be non-empty; `state` must be non-nil JSON content.
`RawQuestion` is only checked for a string `type` field.

### Responses

```go
type SystemOneResponse struct {
    Model     string
    Answers   map[string]Answer
    Usage     Usage
    RequestID string
    Raw       *RawResponse   // Status int, Header http.Header, Body []byte
}
func (r *SystemOneResponse) Nouls()   map[string]NoulAnswer
func (r *SystemOneResponse) Choices() map[string]ChoiceAnswer
func (r *SystemOneResponse) Scores()  map[string]ScoreAnswer

type Answer interface{ AnswerType() string }
type NoulAnswer    struct{ Noul float64 }
type ChoiceAnswer  struct{ Choice string; Confidence float64; Probabilities map[string]float64 }
type ScoreAnswer   struct{ Score float64; Confidence float64; Legend map[string]JSONContent;
                           Probabilities map[string]float64 }
type UnknownAnswer struct{ Type string; Raw json.RawMessage }
type Usage         struct{ InputTokens, OutputTokens *int }

type ListModelsResponse struct{ Models []ModelMetadata; RequestID string; Raw *RawResponse }
type ModelMetadata     struct{ Name, Description, ReleaseDate string }
```

Answer decoding is two-pass: read `type`, then decode into the concrete struct.
An unrecognized `type` yields `UnknownAnswer` without failing the response. A
known type with a missing or mistyped required field yields
`*ResponseValidationError{FieldPath: "answers.tone.confidence", Err}`.

`SystemOneAs[T]` decodes the `answers` JSON object into `T` with
`encoding/json`, so `T` is a plain struct whose fields are `NoulAnswer`,
`ChoiceAnswer`, or `ScoreAnswer` with `json` tags naming the question keys.
The answer structs implement `json.Unmarshaler` so they also verify the `type`
discriminator during typed decoding.

### Errors

```go
type APIError struct {
    Status    int
    Body      []byte
    Headers   http.Header
    Endpoint  string      // "POST /v1/systemone"
    RequestID string
}
func (e *APIError) Error() string
func (e *APIError) Message() string   // text from `detail` (string, object, or array), else "message"/"error", else body prefix

type RateLimitError struct{ APIError; RetryAfter time.Duration }  // status 429
type ConnectionError struct{ Err error }                          // wraps; Unwrap()
type TimeoutError struct{ ConnectionError; Timeout time.Duration }
type ResponseValidationError struct{ FieldPath string; Err error }
type ValidationError struct{ Path string; Err error }

func IsAuthError(error) bool   // 401 or 403
func IsRateLimited(error) bool // *RateLimitError
func IsRetryable(error) bool   // would the default policy retry it
```

All error types support `errors.As` and `errors.Is` via `Unwrap`. Status-code
distinctions beyond 429 are made by reading `Status`, not by type.

### Retry policy

```go
type RetryPolicy struct {
    MaxRetries      int            // 2
    InitialDelay    time.Duration  // 500ms
    MaxDelay        time.Duration  // 5s
    Jitter          float64        // 0.25
    Budget          time.Duration  // 30s; 0 = unlimited
    RetryStatuses   []int          // 408, 429, 500–599
    RetryOnConnErr  bool           // true
    RetryOnTimeout  bool           // true
    HonorRetryAfter bool           // true
    ShouldRetry     func(resp *http.Response, err error) bool // optional override
}
func DefaultRetryPolicy() RetryPolicy
```

Algorithm: delay for attempt n is `min(InitialDelay * 2^n, MaxDelay)`, then
reduced by a uniform random fraction in `[0, Jitter]`. If `HonorRetryAfter`
and the response has `retry-after-ms` or `Retry-After` (seconds or HTTP date),
that value replaces the computed delay. A retry is skipped when its delay would
exceed the remaining budget, and the last error is returned. `ctx` cancellation
aborts immediately, including during a sleep. The request body is marshaled
once and reused as bytes for each attempt. `MaxRetries: 0` disables retries.

### Transport and logging

`net/http` only. The client owns a default `http.Client` unless
`WithHTTPClient` supplies one. Headers on every request: `Authorization`,
`Content-Type: application/json`, `Accept: application/json`,
`User-Agent: typesafe-go/<version>`, plus client and per-request extra headers.

Logging uses `log/slog`. Level comes from `WithLogger` or
`TYPESAFE_LOG_LEVEL` (`debug|info|warning|error|off`, default off). Logged
fields: method, path, status, duration, attempt, request ID. The
`Authorization` header is never logged. Bodies are never logged.

### Instrumentation hook

```go
type RequestInfo struct {
    Operation     string   // "system_one" | "list_models"
    Model         string
    QuestionCount int
    NoulCount, ChoiceCount, ScoreCount int
    State         any      // for opt-in content recording only
    Questions     Questions
}
type RequestResult struct {
    Attempts  int
    Status    int
    RequestID string
    Model     string       // resolved model from the response
    Usage     Usage
    Response  *SystemOneResponse // nil for list_models or on error
    Err       error
}
type Instrumentation interface {
    RequestStart(ctx context.Context, info RequestInfo) (context.Context, func(RequestResult))
    // Transport wraps the HTTP round tripper used for this client. May return rt unchanged.
    Transport(rt http.RoundTripper) http.RoundTripper
}
```

The core calls `RequestStart` once per `SystemOne`/`ListModels` call, uses the
returned `ctx` for all attempts, and calls the returned function exactly once
when the call finishes. `Transport` is applied once at client construction. When `WithHTTPClient` supplied the client, a shallow copy is wrapped so the caller's client is not mutated.

## Subpackage `typesafe/otel`

```go
func New(opts ...Option) typesafe.Instrumentation
func WithTracerProvider(trace.TracerProvider) Option
func WithRecordContent(maxBytes int) Option   // records state and questions, truncated
func WithRecordAnswers() Option               // records per-answer choice/score/noul/confidence
```

Behavior:

- `RequestStart` opens a span named `typesafe.system_one` or
  `typesafe.list_models` (kind Client) as a child of any span in `ctx`.
- `Transport` returns `otelhttp.NewTransport(rt)` so each HTTP attempt is a
  child span with standard HTTP attributes.
- Attributes set on the operation span:

  | Attribute | Source |
  |---|---|
  | `gen_ai.system` | constant `typesafe` |
  | `gen_ai.request.model` | requested model |
  | `gen_ai.response.model` | `model` from response |
  | `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens` | `usage` |
  | `typesafe.request_id` | response header |
  | `typesafe.questions.count`, `.noul`, `.choice`, `.score` | request |
  | `typesafe.retry.attempts` | attempts made |
  | `http.response.status_code` | final status |
  | `error.type` | Go type name of the error, on failure |

- On error the span status is Error and the error is recorded as an event.
- State and question content are never recorded unless `WithRecordContent` is
  set; then `typesafe.state` and `typesafe.questions` carry JSON truncated to
  `maxBytes`. `WithRecordAnswers` adds `typesafe.answer.<key>.<field>`
  attributes for each answer.
- No global provider or exporter is installed by this package.

## CLI: `cmd/jev`

Cobra. Root persistent flags, each falling back to the matching env var:
`--api-key`, `--base-url`, `--model`, `--timeout`, `--max-retries`,
`--log-level`, `--trace`, `--no-trace`, `--pretty`.

Commands:

- `jev ask` — one System One call.
  - Input: `-f FILE` or stdin JSON in the exact HTTP body shape
    (`state`, `questions`, optional `model`); or flags `--state STR|@file|-`,
    repeated `--noul key=instructions`, `--choice key=instructions:l1,l2,...`,
    `--score key=instructions:lvl0|lvl1|...`. `-f` and the question flags are
    mutually exclusive; in flag mode stdin is read only when `--state -` is
    given. Instructions may contain `:`; labels and levels may not contain
    `:`, and labels may not contain `,` nor levels `|`. With no `-f`, no
    question flags, and a terminal on stdin, `ask` exits 1 immediately.
  - Output: response JSON on stdout: `model`, `answers`, `usage`,
    `request_id`. `--raw` prints the server body unchanged.
- `jev models` — prints `{"models":[...]}`.
- `jev version` — prints version, commit, Go version.

Exit codes:

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | usage error, bad flags, unreadable or invalid request JSON, missing API key (`kind` `usage`); or an unclassified internal error (`kind` `internal`) |
| 2 | client-side validation failure (`*ValidationError`) |
| 3 | 401 or 403 |
| 4 | 400, 404, 422 |
| 5 | 429 after retries exhausted |
| 6 | 5xx (including 529) after retries exhausted, or a 2xx body that failed decoding (`kind` `invalid_response`) |
| 7 | connection error or per-attempt timeout, including a caller context deadline |
| 130 | interrupted by Ctrl-C or SIGTERM while a request was in flight (`kind` `interrupted`) |

On error, including flag and command errors from the argument parser: one
line on stderr with control characters escaped, and on stdout a JSON object
`{"error": {"status", "request_id", "message", "kind"}}`. Inbound JSON and
state files are capped at 16 MiB.

Telemetry: the CLI initializes the OTel SDK via `otelconf.NewSDK()` when
`--trace` is set or when `HONEYCOMB_API_KEY`, `OTEL_EXPORTER_OTLP_ENDPOINT`,
or `OTEL_CONFIG_FILE` is present in the environment, unless `--no-trace`,
which always wins. An `OTEL_CONFIG_FILE` is read with `${VAR}` expansion from
the environment before parsing. Exporter failures are reported on stderr and
never change the exit code. `OTEL_SERVICE_NAME` defaults to `jev`. The SDK is flushed and
shut down before exit. The client is built with `typesafe/otel.New()`.

## Testing

Standard `testing` package only.

- Unit: marshaling of each question type against golden JSON from the API
  docs; validation rules and paths; retry delay math with a seeded RNG;
  `Retry-After` parsing; error mapping by status; answer decoding including
  unknown types and validation errors; `SystemOneAs` decoding.
- Fake server (`httptest.Server`) with scripted response sequences: success;
  429 with `retry-after-ms` then 200; 529 then 200; retries exhausted; budget
  exhausted; `ctx` cancelled during backoff; body replayed across attempts;
  request ID propagated; extra headers and body merged.
- CLI: Cobra commands executed in-process against the fake server with
  captured stdout, stderr, and exit code for each exit-code row above and for
  both input modes.
- OTel: `tracetest` in-memory exporter asserting span names, parentage,
  attributes, error status, and absence of content unless opted in.
- Live integration: skipped unless `TYPESAFE_API_KEY` is set; one `SystemOne`
  with one question of each type and one `ListModels`.

## Self-review with Jev

Once `jev` works, the repository uses it to judge its own implementation and
tests against this spec. Jev returns typed judgments, not explanations, so the
tool asks many narrow questions and escalates the doubtful ones.

Layout: `tools/selfreview/` holds a Go driver (`main.go`, run via
`make selfreview`) with the question templates built in code, and
`units.json` describing the units. The driver requires
`TYPESAFE_API_KEY` and invokes `jev ask` in JSON mode over stdin, so it is
also the end-to-end exercise of the input mode the plugin will use.

Units: `units.json` lists triples of spec section heading, implementation
file(s), and test file(s). One request per unit with state
`{"spec": "...", "implementation": "...", "tests": "..."}` and questions:

- Noul per behavior named in the spec section, generated from a per-section
  list in the template: "Do the tests exercise `<behavior>`?"
- Noul: "Does the implementation contradict the spec section?"
- Score on test thoroughness with concrete levels (0: no tests for this
  section; 1: happy path only; 2: happy path plus the error cases the spec
  names; 3: also concurrency, cancellation, and boundary values the spec
  names).
- Choice of the weakest area from a fixed set: `validation`, `error_mapping`,
  `retry`, `decoding`, `encoding`, `logging`, `none`.

Output: `selfreview-report.json` and a Markdown summary on stdout. Thresholds
(defaults, overridable by flag): a behavior Noul below 0.6, a contradiction
Noul above 0.4, or a thoroughness Score below 2.0 flags the unit. Flagged
units are listed with the exact failing question so a reviewer, human or
agent, gets a specific claim to check rather than a general request.

The implementation plan ends with a phase that runs the self-review over
every unit, sends flagged items to a code-review pass, fixes what that pass
confirms, and reruns until nothing is flagged or remaining flags are
explicitly accepted with a note in the report.

This tool was later promoted from `tools/selfreview` to the `jev review`
subcommand, usable on any codebase. See
`2026-09-28-jev-review-design.md`.

## Tooling and release

Go 1.25 minimum in `go.mod`: the stable OpenTelemetry line (otel v1.46.0,
contrib v0.71.0, otelconf v0.26.0) requires it. Local development on an older
toolchain works through `GOTOOLCHAIN=auto`. `golangci-lint` with a
small config, `govulncheck`, Makefile targets `test`, `lint`, `build`,
`integration`, `selfreview`, `docs`. GitHub Actions: `ci.yml` runs test and lint on push and
PR; `release.yml` on a `v*` tag runs goreleaser to build `jev` for
darwin/linux × amd64/arm64 and attach binaries to the GitHub release.

## Documentation (Diátaxis)

```
docs/
  tutorials/
    first-judgment-in-go.md        build and run a small Go program end to end
    jev-from-the-command-line.md   install jev, ask one question, read the answer
  how-to/
    handle-rate-limits-and-retries.md
    decode-answers-into-your-own-struct.md
    trace-calls-and-send-to-honeycomb.md
    call-through-a-gateway.md
    drive-jev-from-a-script-or-agent.md
  reference/
    client-options-and-environment.md
    question-types.md
    response-types.md
    errors-and-exit-codes.md
    jev-cli.md
    span-attributes.md
  explanation/
    why-the-core-is-stdlib-only.md
    retries-and-budgets.md
    why-content-is-not-traced-by-default.md
    mapping-from-the-python-sdk.md
```

`README.md` is short: install, a five-line library example, a `jev` example,
and links into each Diátaxis section. `doc.go` carries package docs for
`pkg.go.dev`. The diataxis-docs plugin's writer agents are used for the
tutorial, how-to, reference, and explanation pages, and its validator checks
cross-links before release. `docs/how-to/drive-jev-from-a-script-or-agent.md`
plus `reference/jev-cli.md` together define the contract the future plugin
will be written against.

## Mapping from the Python SDK

| Python | Go |
|---|---|
| `TypeSafeClient(api_key=, base_url=, model=, retry=, timeout=, headers=, http_client=)` | `NewClient(WithAPIKey, WithBaseURL, WithModel, WithRetryPolicy, WithTimeout, WithHeaders, WithHTTPClient)` |
| `AsyncTypeSafeClient` | none; `ctx` + goroutines |
| `client.system_one(state, questions, model=, retry=, timeout=, extra_headers=, extra_body=)` | `c.SystemOne(ctx, state, questions, WithRequest...)` |
| `response_model=` | `SystemOneAs[T]` |
| `client.models.list()` | `c.ListModels(ctx)` |
| `Noul/Choice/Score` pydantic models, raw dicts | `Noul/Choice/Score` structs, `RawQuestion` |
| `result.nouls/choices/scores` | `Nouls()/Choices()/Scores()` |
| `result.request_id`, `raw_http_response` | `RequestID`, `Raw` |
| `TypeSafeAPIError.status/body/headers/endpoint/request_id` | `APIError` same fields |
| status-specific exception classes | `APIError.Status` plus `RateLimitError` |
| `TypeSafeAPIConnectionError/TimeoutError` | `ConnectionError`/`TimeoutError` |
| `TypeSafeAPIResponseValidationError.field_path` | `ResponseValidationError.FieldPath` |
| `RetryPolicy(max_retries, backoff_max, timeout, ...)` | `RetryPolicy` same semantics, Go names |
| `TYPESAFE_*` env vars | identical names |
| logging via `typesafe_sdk` logger | `log/slog` via `WithLogger` / `TYPESAFE_LOG_LEVEL` |
| none | `Instrumentation` hook, `typesafe/otel`, `jev` CLI |
