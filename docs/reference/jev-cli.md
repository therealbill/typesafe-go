---
title: "jev CLI"
description: "The jev binary: global flags, the ask/models/version subcommands, their input and output JSON shapes, telemetry-related environment variables, and exit codes."
diataxis: reference
weight: 50
---

# jev CLI

Binary: `jev` (built at `./bin/jev`). Source: `internal/cli`.

## Synopsis

```
$ ./bin/jev --help
jev sends a state and a set of typed questions (noul, choice, score) to the TypeSafe System One API and prints the answers as JSON.

Usage:
  jev [command]

Available Commands:
  ask         Send a state and typed questions, print the answers as JSON
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  models      List the models available to the account as JSON
  version     Print version information as JSON

Flags:
      --api-key string     TypeSafe API key (env TYPESAFE_API_KEY)
      --base-url string    API base URL (env TYPESAFE_BASE_URL)
  -h, --help               help for jev
      --log-level string   debug|info|warning|error|off (env TYPESAFE_LOG_LEVEL)
      --max-retries int    retries after the first attempt (default 2)
      --model string       model name (env TYPESAFE_DEFAULT_MODEL; default jev-latest)
      --no-trace           disable tracing even when HONEYCOMB_API_KEY or OTEL_* is set
      --pretty             indent JSON output
      --timeout duration   per-attempt HTTP timeout (default 10s)
      --trace              force OpenTelemetry tracing on
  -v, --version            version for jev

Use "jev [command] --help" for more information about a command.
```

`completion` and `help` are provided by the underlying Cobra command framework and are not detailed further on this page.

### `-v`, `--version` (global flag) versus `jev version` (subcommand)

The root command declares `Version: version.Version`, which makes Cobra
auto-provide a `-v`/`--version` flag on `jev` itself. This flag and the `jev
version` subcommand are different things that happen to share the word
"version":

| | `jev --version` / `jev -v` | `jev version` |
|---|---|---|
| Kind | Global flag on the root command | Subcommand |
| Output | One line: `jev version <string>` | A JSON object: `{"version":...,"commit":...,"go":...}` |
| Exit code | 0 | 0 |

The `<string>` is `internal/version.Version` — the same build-time value
reported by the `version` field of the `jev version` subcommand (see below).
Verified against the built binary:

```
$ ./bin/jev --version
jev version 6d0a13c
```

## Global flags

These are persistent flags (Cobra `PersistentFlags`, defined in `internal/cli/root.go`, `NewRootCmd`). They apply to every subcommand and appear under "Global Flags" in each subcommand's own `--help` output. (`-v`/`--version`, above, is a separate, Cobra-provided flag on the root command only; it is not a persistent flag and does not appear on subcommands.)

| Flag | Type | Flag default | Env fallback | `--help` description |
|---|---|---|---|---|
| `--api-key` | string | `""` | `TYPESAFE_API_KEY` | TypeSafe API key (env TYPESAFE_API_KEY) |
| `--base-url` | string | `""` | `TYPESAFE_BASE_URL` | API base URL (env TYPESAFE_BASE_URL) |
| `--model` | string | `""` | `TYPESAFE_DEFAULT_MODEL` | model name (env TYPESAFE_DEFAULT_MODEL; default jev-latest) |
| `--timeout` | duration | `0` | — | per-attempt HTTP timeout (default 10s) |
| `--max-retries` | int | `2` | — | retries after the first attempt (default 2) |
| `--log-level` | string | `""` | `TYPESAFE_LOG_LEVEL` | debug\|info\|warning\|error\|off (env TYPESAFE_LOG_LEVEL) |
| `--trace` | bool | `false` | — | force OpenTelemetry tracing on |
| `--no-trace` | bool | `false` | — | disable tracing even when HONEYCOMB_API_KEY or OTEL_* is set |
| `--pretty` | bool | `false` | — | indent JSON output |
| `-h`, `--help` | bool | `false` | — | help for jev |

`clientOptions` (`internal/cli/root.go`) turns `globals` fields into `typesafe.Option` values. Whether a flag was explicitly passed is tracked with `cmd.Flags().Changed("<name>")`, not by any sentinel value. A flag not explicitly passed is omitted from the options, letting the `typesafe` package's own default (env var, then package default — see the [client options and environment reference](./client-options-and-environment.md)) apply unmodified.

### `--max-retries`

The flag's Cobra default is `2` (`pf.IntVar(&g.maxRetries, "max-retries", 2, "retries after the first attempt")`) — the same value the library's own `DefaultRetryPolicy()` uses, and there is no sentinel value. Whether `--max-retries` was explicitly passed is tracked via `cmd.Flags().Changed("max-retries")`:

- Not passed: `clientOptions` does not call `typesafe.WithRetryPolicy` at all. The client's own configured policy (or `typesafe.DefaultRetryPolicy()`) applies as-is.
- Passed (including `--max-retries 2`, matching the default): `clientOptions` calls `p := typesafe.DefaultRetryPolicy(); p.MaxRetries = g.maxRetries; typesafe.WithRetryPolicy(p)` — every other `RetryPolicy` field still comes from `DefaultRetryPolicy()`.

`--max-retries 2` and omitting the flag are practically indistinguishable in
effect (both end up at `MaxRetries: 2` with default everything else), but
mechanically the first explicitly calls `WithRetryPolicy` and the second does
not call it at all. `--max-retries 0` disables retries.

### `--timeout`

The flag's Go zero value is still `0`, but `0` is now a meaningful, explicit
value the flag can carry. Whether `--timeout` was explicitly passed is
tracked via `cmd.Flags().Changed("timeout")`:

- Not passed: `clientOptions` does not call `typesafe.WithTimeout` at all. `typesafe.DefaultTimeout` (`10 * time.Second`) applies at the library level, untouched.
- Passed, at any value including `0`: `clientOptions` calls `typesafe.WithTimeout(g.timeout)` regardless of the value. `--timeout 0` explicitly disables the per-attempt timeout at the library level.

### Negative `--timeout` or `--max-retries`

`validateFlags` (`internal/cli/root.go`) rejects an explicitly-passed
negative value for either flag, before any client is built:

| Condition | Error | Exit code | `kind` |
|---|---|---|---|
| `changed("timeout") && g.timeout < 0` | `--timeout must not be negative, got <value>` | 1 | `"usage"` |
| `changed("max-retries") && g.maxRetries < 0` | `--max-retries must not be negative, got <value>` | 1 | `"usage"` |

Verified against the built binary:

```
$ ./bin/jev ask --timeout -1s --state x --noul a=b
{"error":{"kind":"usage","message":"--timeout must not be negative, got -1s"}}
```

## `jev ask`

```
$ ./bin/jev ask --help
Send one System One request and print the response as JSON.

Two input modes:

  JSON   jev ask -f request.json        (or pipe the JSON to stdin)
         The document is the HTTP body shape:
         {"state": ..., "questions": {"id": {"type": "noul", ...}}, "model": "..."}

  Flags  jev ask --state "text" --noul billing="Is this about billing?" \
             --choice tone="What is the tone?:calm,angry" \
             --score urgency="How urgent?:low|medium|high"

Output: {"model": ..., "answers": {...}, "usage": {...}, "request_id": ...}

Usage:
  jev ask [flags]

Flags:
      --choice stringArray   key=instructions:label1,label2,... (repeatable; labels must not contain ':' or ',')
  -f, --file string          request JSON file; '-' or omitted reads stdin
  -h, --help                 help for ask
      --noul stringArray     key=instructions (repeatable)
      --raw                  print the server response body unchanged
      --score stringArray    key=instructions:level0|level1|... (repeatable; levels must not contain ':' or '|')
      --state string         state text, @path to read a file, or '-' for stdin

Global Flags:
      --api-key string     TypeSafe API key (env TYPESAFE_API_KEY)
      --base-url string    API base URL (env TYPESAFE_BASE_URL)
      --log-level string   debug|info|warning|error|off (env TYPESAFE_LOG_LEVEL)
      --max-retries int    retries after the first attempt (default 2)
      --model string       model name (env TYPESAFE_DEFAULT_MODEL; default jev-latest)
      --no-trace           disable tracing even when HONEYCOMB_API_KEY or OTEL_* is set
      --pretty             indent JSON output
      --timeout duration   per-attempt HTTP timeout (default 10s)
      --trace              force OpenTelemetry tracing on
```

### Flags

| Flag | Type | Default | Repeatable | Description |
|---|---|---|---|---|
| `-f`, `--file` | string | `""` | No | Request JSON file; `-` or omitted reads stdin. |
| `--state` | string | `""` | No | State text, `@path` to read a file, or `-` for stdin. |
| `--noul` | stringArray | none | Yes | `key=instructions` |
| `--choice` | stringArray | none | Yes | `key=instructions:label1,label2,...` (repeatable; labels must not contain `:` or `,`) |
| `--score` | stringArray | none | Yes | `key=instructions:level0\|level1\|...` (repeatable; levels must not contain `:` or `\|`) |
| `--raw` | bool | `false` | No | Print the server response body unchanged. |

### Input size limit

`maxRequestBytes` (`internal/cli/request.go`) is `16 << 20` (16 MiB). Reading
stops one byte past this cap, so an oversized input is detected rather than
silently truncated. It applies everywhere `jev ask` reads bytes from stdin or
a file: stdin in JSON mode, `-f <path>`, `--state -`, and `--state @path`.
Exceeding it produces the error `request exceeds 16 MiB` (a `*usageError`:
exit code 1, kind `"usage"`).

### Input modes

`jev ask` accepts exactly one of two mutually exclusive input modes, decided by `buildRequest` (`internal/cli/ask.go`): question flags (`--state`, `--noul`, `--choice`, `--score`; any one of them present triggers flag mode) versus JSON (`-f`/stdin). Combining `-f` with any question flag is an error:

```
--file cannot be combined with --state, --noul, --choice, or --score
```

#### JSON mode

```
jev ask -f request.json
jev ask < request.json
```

Used when `-f` is given, or when `-f` is empty/omitted and no question flags are set (in which case JSON is read from stdin). The document is the HTTP request body shape:

```json
{
  "state": "...",
  "questions": { "<id>": { "type": "noul", "...": "..." } },
  "model": "..."
}
```

Parsing rules (`parseRequestJSON`, `internal/cli/request.go`):

- `state` is required (error: `invalid request: "state" is required`).
- `questions` is required (error: `invalid request: "questions" is required`); each entry must be a JSON object with a string `type` field (error otherwise: `invalid request: questions.<id>: must be an object with a "type"`, or `invalid request: questions.<id>.type: is required`). `type` of `"noul"`, `"choice"`, or `"score"` is decoded, with a strict decoder (`json.NewDecoder(...).DisallowUnknownFields()`), into `typesafe.Noul`, `typesafe.Choice`, or `typesafe.Score` respectively — an unrecognized field inside that question object is rejected, naming the field: `invalid request: questions.<id>: json: unknown field "<field>"`. Strictness is nested: an unrecognized field inside a nested object (for example a `noul` question's `criteria`, which accepts only `true` and `false`) is rejected the same way. Any other `type` value is decoded into a `typesafe.RawQuestion` (passed through unmodeled with a plain `json.Unmarshal`, so it is not subject to the strict decoder — a forward-compatible question type accepts any fields).
- `model` is optional and, if present, becomes the request's model (see Model selection, below).
- Any other top-level field is collected and sent via `typesafe.WithExtraBody`.
- An empty document (after trimming whitespace) is an error: `no request given: pass --file, pipe JSON to stdin, or use --state with --noul/--choice/--score`.
- Malformed JSON is reported as `invalid request JSON: <json error>` (top-level) or `invalid request: <field>: <json error>` (per-field).
- Reading stdin, `-f <path>`, or `--state @path` refuses input over 16 MiB — see [Input size limit](#input-size-limit) above.

Verified against the built binary:

```
$ echo '{"state":"x","questions":{"a":{"type":"noul","instructions":"y","bogus":1}}}' | ./bin/jev ask
{"error":{"kind":"usage","message":"invalid request: questions.a: json: unknown field \"bogus\""}}
```

If `jev ask` is invoked with no `-f`/`--file` and no question flags, and
stdin is an interactive terminal (`stdinIsTerminal`, checked via
`os.File.Stat()`'s `os.ModeCharDevice`), it returns the `no request given`
usage error immediately rather than blocking on input that will never
arrive. This check only applies when `--file` is omitted entirely: an
explicit `-f -` always reads stdin unconditionally, terminal or not. Piped
or redirected stdin (with `--file` omitted) is unaffected by the check. The
same `no request given` error is also returned when the input, once read, is
present but blank after trimming whitespace.

#### Flag mode

```
jev ask --state "text" --noul billing="Is this about billing?" \
    --choice tone="What is the tone?:calm,angry" \
    --score urgency="How urgent?:low|medium|high"
```

Used when any of `--state`, `--noul`, `--choice`, `--score` is set (`requestFromFlags`, `internal/cli/request.go`, called from `internal/cli/ask.go`). Parsing rules:

- `--state` is required whenever `--noul`, `--choice`, or `--score` is given (error: `--state is required when using --noul, --choice, or --score`).
- `--state` is also required to come with at least one question flag: `--state` given with zero `--noul`/`--choice`/`--score` flags is an error: `--state needs at least one --noul, --choice, or --score question`.
- `--state` value resolution: `-` reads state from stdin; a value starting with `@` reads the remainder as a file path (both subject to the [16 MiB input limit](#input-size-limit)); any other value is used literally as text.
- Every `--noul`/`--choice`/`--score` value is first split on the first `=` into a key and a remainder (`splitKV`). A missing or leading `=` is an error: `<flag> "<value>": expected key=instructions`. Otherwise, the key must be non-blank after trimming whitespace, or it is an error: `<flag> "<value>": question key must not be blank`.
- `--noul key=instructions`: the remainder after `key=` is used directly as `Instructions`. Produces `typesafe.Noul{Instructions: instructions}` with no criteria.
- `--choice key=instructions:label1,label2,...` and `--score key=instructions:level0|level1|...`: the remainder after `key=` is split on its **last** `:` (`strings.LastIndex`, in `splitInstrLabels`) into instructions and a label/level list, which is then split on `,` (`--choice`) or `|` (`--score`). Splitting on the last `:` — not the first — lets `instructions` itself contain colons, for example `--choice tone="What time is it: morning or evening?:calm,angry"` parses instructions as `"What time is it: morning or evening?"` and labels as `calm,angry`. A missing `:` is an error: `<flag> "<value>": expected key=instructions:labels`; an empty label/level (after trimming) is an error: `<flag> "<value>": empty label`.
- There is no character-forbidding validation on a label or level: since the split point is always the *last* `:` in the whole value, a label or level occurring after that point must not itself contain a `:`, or that colon is mistaken for the split point (shifting where instructions ends). For example, `--choice 'tone=What tone?:calm,an:gry'` splits on the last `:` (the one inside `an:gry`), producing `Instructions: "What tone?:calm,an"` and a single label `"gry"` — `calm` silently disappears into the instructions text rather than being rejected. Likewise a `--choice` label containing `,`, or a `--score` level containing `|`, is not rejected — it is mechanically split into multiple labels/levels at that separator, the same as any other occurrence of it. None of these cases produce a validation error message; each is a parsing consequence of where the splits land, matching the `--help` text above (`labels must not contain ':' or ','`, `levels must not contain ':' or '|'`).
- `--score` additionally requires at least two levels (error: `--score "<value>": needs at least two levels separated by |`). Produces `typesafe.Choice{Instructions: instructions, Criteria: {label: nil, ...}}` for `--choice`, `typesafe.Score{Instructions: instructions, Criteria: [level0, level1, ...]}` for `--score`.
- A question key repeated across `--noul`/`--choice`/`--score` is an error: `duplicate question key "<key>"`.

Verified against the built binary:

```
$ ./bin/jev ask --state x --noul "  =instructions"
{"error":{"kind":"usage","message":"--noul \"  =instructions\": question key must not be blank"}}
$ ./bin/jev ask --state "just text"
{"error":{"kind":"usage","message":"--state needs at least one --noul, --choice, or --score question"}}
```

### `--raw`

Without `--raw`, `jev ask` writes its own JSON encoding of the decoded `*typesafe.SystemOneResponse` via the shared `writeJSON` helper (honoring `--pretty`). With `--raw`, it writes `res.Raw.Body` — the server's response bytes, unchanged — followed by a trailing newline; `--pretty` has no effect on `--raw` output.

### Output shape (non-`--raw`)

```json
{
  "model": "...",
  "answers": { "<id>": { "type": "noul|choice|score", "...": "..." } },
  "usage": { "input_tokens": 0, "output_tokens": 0 },
  "request_id": "..."
}
```

This is the JSON encoding of `typesafe.SystemOneResponse`. Per-answer wire shapes (`NoulAnswer`, `ChoiceAnswer`, `ScoreAnswer`) are documented on the [response types reference](./response-types.md). `request_id` is omitted when empty.

### Model selection

If the parsed request has a non-empty `Model` (from JSON mode's `"model"` field; flag mode never sets it) and `--model` was **not** explicitly passed on the command line (`cmd.Flags().Changed("model")` is `false`), the request's model is sent via `typesafe.WithRequestModel`, overriding the client's configured model for that call only. If `--model` was passed explicitly, it wins (the client-level model set through `clientOptions`/`typesafe.WithModel` applies, and the request's `Model` field is not separately forwarded).

## `jev models`

```
$ ./bin/jev models --help
List the models available to the account as JSON

Usage:
  jev models [flags]

Flags:
  -h, --help   help for models

Global Flags:
      --api-key string     TypeSafe API key (env TYPESAFE_API_KEY)
      --base-url string    API base URL (env TYPESAFE_BASE_URL)
      --log-level string   debug|info|warning|error|off (env TYPESAFE_LOG_LEVEL)
      --max-retries int    retries after the first attempt (default 2)
      --model string       model name (env TYPESAFE_DEFAULT_MODEL; default jev-latest)
      --no-trace           disable tracing even when HONEYCOMB_API_KEY or OTEL_* is set
      --pretty             indent JSON output
      --timeout duration   per-attempt HTTP timeout (default 10s)
      --trace              force OpenTelemetry tracing on
```

No command-specific flags. Calls `typesafe.Client.ListModels` and prints the JSON encoding of the result:

```json
{ "models": [ ... ], "request_id": "..." }
```

`ListModelsResponse` and `ModelMetadata` are documented on the [response types reference](./response-types.md).

## `jev version`

```
$ ./bin/jev version --help
Print version information as JSON

Usage:
  jev version [flags]

Flags:
  -h, --help   help for version

Global Flags:
      --api-key string     TypeSafe API key (env TYPESAFE_API_KEY)
      --base-url string    API base URL (env TYPESAFE_BASE_URL)
      --log-level string   debug|info|warning|error|off (env TYPESAFE_LOG_LEVEL)
      --max-retries int    retries after the first attempt (default 2)
      --model string       model name (env TYPESAFE_DEFAULT_MODEL; default jev-latest)
      --no-trace           disable tracing even when HONEYCOMB_API_KEY or OTEL_* is set
      --pretty             indent JSON output
      --timeout duration   per-attempt HTTP timeout (default 10s)
      --trace              force OpenTelemetry tracing on
```

No command-specific flags. Makes no network call. Prints:

```json
{ "version": "...", "commit": "...", "go": "..." }
```

`version` and `commit` are both the `internal/version` package's `Version`/`Commit` values, set by the build: the Makefile's `build` target injects them via `-ldflags` from `git describe --tags --always --dirty` and `git rev-parse --short HEAD`; a plain `go build` leaves the package defaults, `"dev"` and `"none"`. `go` is the value of `runtime.Version()`. Because this is encoded from a Go `map[string]string`, `encoding/json` emits the keys in alphabetical order (`commit`, `go`, `version`), independent of `--pretty`. Verified against the built binary:

```
$ ./bin/jev version
{"commit":"6d0a13c","go":"go1.27.1","version":"6d0a13c"}
$ ./bin/jev --pretty version
{
  "commit": "6d0a13c",
  "go": "go1.27.1",
  "version": "6d0a13c"
}
```

## Telemetry

Source: `internal/cli/telemetry.go`.

### Enabling tracing

`telemetryEnabled` decides whether the OpenTelemetry SDK is started for the command:

| Condition | Result |
|---|---|
| `--no-trace` passed | Tracing is disabled. This check runs first and short-circuits every other condition. |
| Otherwise, `--trace` passed | Tracing is enabled. |
| Otherwise, `HONEYCOMB_API_KEY` set (non-empty) | Tracing is enabled. |
| Otherwise, `OTEL_EXPORTER_OTLP_ENDPOINT` set (non-empty) | Tracing is enabled. |
| Otherwise, `OTEL_CONFIG_FILE` set (non-empty) | Tracing is enabled. |
| None of the above | Tracing is disabled. |

### Configuration source

`telemetryConfig` chooses how the enabled configuration is built:

| `OTEL_CONFIG_FILE` | Behavior |
|---|---|
| Set (non-empty) | The named file's bytes are read, then `${VAR}`-expanded against the environment via `os.Expand`, then parsed as OpenTelemetry YAML configuration via `otelconf.ParseYAML`. The parsed configuration is used directly — `HONEYCOMB_API_KEY`, `OTEL_EXPORTER_OTLP_ENDPOINT`, and `OTEL_SERVICE_NAME` are not consulted. |
| Unset | A configuration is built from `HONEYCOMB_API_KEY`, `OTEL_EXPORTER_OTLP_ENDPOINT`, and `OTEL_SERVICE_NAME` (below), via `buildTelemetryConfig`. |

### Built configuration (no `OTEL_CONFIG_FILE`)

| Environment variable | Default when unset | Effect |
|---|---|---|
| `HONEYCOMB_API_KEY` | — | When non-empty, added as an `x-honeycomb-team` HTTP header on the OTLP exporter. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `https://api.honeycomb.io` | A trailing `/` is trimmed; `/v1/traces` is appended if the (trimmed) value does not already end with it. |
| `OTEL_SERVICE_NAME` | `jev` | Set as the `service.name` resource attribute. |

The resulting configuration (`buildTelemetryConfig`) is `FileFormat: "1.0"` with a single tracer provider processor: a batch span processor exporting via OTLP/HTTP to the resolved endpoint.

### Failure handling

A telemetry setup failure — reading or parsing `OTEL_CONFIG_FILE`, or SDK construction — is reported on stderr as:

```
jev: tracing disabled: <error>
```

and the command proceeds with tracing disabled (`typesafe.Instrumentation` is `nil` for that run); it never stops the command from otherwise succeeding. When `OTEL_CONFIG_FILE` fails to parse after `${VAR}` expansion, every expanded environment value of 4 or more bytes is redacted out of `<error>` as `[redacted]`, so a secret substituted into the file cannot leak into this message. A shutdown failure (during the deferred SDK shutdown) is separately reported as:

```
jev: tracing shutdown: <error>
```

## Exit codes

| Code | Constant | Condition |
|---|---|---|
| 0 | `ExitOK` | Success. |
| 1 | `ExitUsage` | Bad flags, unreadable or invalid request JSON, missing API key, or an unrecognized error. |
| 2 | `ExitValidation` | Request failed client-side validation. |
| 3 | `ExitAuth` | 401 or 403. |
| 4 | `ExitRequest` | Other 4xx: 400, 404, 422. |
| 5 | `ExitRateLimit` | 429 after retries. |
| 6 | `ExitServer` | 5xx after retries, or an unreadable 2xx body. |
| 7 | `ExitConnection` | Connection failure or timeout. |
| 130 | `ExitInterrupted` | The context was cancelled, conventionally by SIGINT. |

The full error-type-to-exit-code classification and the error JSON shape written to stdout on failure — including the `"usage"` versus `"internal"` split at code 1 and the `"interrupted"` case at code 130 — are documented on the [errors and exit codes reference](./errors-and-exit-codes.md).
