---
title: "The Agent-Facing CLI Contract"
description: "Why jev ask mirrors the HTTP wire shape, separates its JSON and human-readable output, and is built around exit codes and input limits that a non-human caller needs."
diataxis: explanation
weight: 70
---

`jev` takes flags, reads stdin, writes stdout, and returns an exit code. Its
primary caller is a script or an autonomous agent that spawns `jev` as a
subprocess, feeds it a request, and needs to know programmatically, and
quickly, what happened; a human running `jev ask` interactively is the
secondary case. This page covers what follows from that: an input format
identical to the wire format, failure reported on two streams in two forms,
exit codes a caller can branch on without parsing, and a refusal to read
from an interactive terminal.

## The input format and the wire format

`jev ask` reads a JSON document, from `--file` or from stdin when no file or
question flags are given, in this shape:

```json
{"state": ..., "questions": {"id": {"type": "noul", ...}}, "model": "..."}
```

`parseRequestJSON` in `internal/cli/request.go` requires exactly `state` and
`questions`, treats `model` as optional, and forwards anything else as extra
body fields. `SystemOne` builds and sends the same shape over HTTP:

```go
body := map[string]any{"state": state, "model": rc.model, "questions": questions}
```

The CLI's input format and the library's wire format are the same document.
`jev ask` decodes the document, calls `SystemOne`, and re-serializes the
result; it adds no wrapper of its own. A caller, human or automated, that
already knows the API's request body from the HTTP reference or from having
built one for a direct API call needs no second format. An agent generating
this JSON programmatically produces exactly what it would send to the HTTP
endpoint, and pipes that same document to `jev ask`.

## Failure on two streams

When a call fails, `fail` in `internal/cli/exit.go` writes a different
rendering to each of stdout and stderr:

```go
_ = writeJSON(io.Out, map[string]any{"error": p}, pretty)
_, _ = fmt.Fprintln(io.Err, "jev:", sanitize(err.Error()))
```

stdout carries a JSON object whether the call succeeded or failed, the
failure shape being `{"error": {"kind", "status", "request_id",
"message"}}`. A caller parses stdout as JSON either way and looks for an
`error` key, with no separate success and failure code paths deciding which
stream to read.

stderr carries a human-oriented rendering of the same error, passed through
`sanitize` first:

```go
func sanitize(s string) string {
    ...
    for _, r := range s {
        if r == '\t' || (r >= 0x20 && r != 0x7f) {
            b.WriteRune(r)
            continue
        }
        fmt.Fprintf(&b, "\\x%02x", r)
    }
    return b.String()
}
```

`sanitize` rewrites every control character below `0x20` except tab, plus
DEL, as a `\xNN` escape. `jev` did not write that text: `err.Error()` can
carry a message that came back from the remote API, verbatim, inside
`APIError`'s `Message()`. A malicious or merely buggy upstream response can
embed ANSI terminal escape sequences, which begin with the `0x1b` ESC byte
that `sanitize` escapes, to move the cursor, hide text, or otherwise
manipulate whatever terminal renders that stderr line. `sanitize` escapes
those bytes before they reach a terminal. The stdout JSON needs no such
pass, because `encoding/json` already escapes control characters while
producing valid JSON.

## Exit codes and the `kind` field

`classify` in `internal/cli/exit.go` maps every error to one of nine exit
codes, `ExitOK` (0) through `ExitConnection` (7), plus `ExitInterrupted`
(130). The comment above the constants states the intent directly: "A caller
can branch on these without parsing output." A script deciding whether to
retry or give up switches on the raw exit code and never touches stdout:
retry on 5 (rate limited) and 7 (connection failure), never on 3 (auth) or 2
(validation), because those fail identically on a retry.

The JSON `kind` field on stdout is finer-grained than the exit code, and the
code comment says so explicitly: "code 1 covers both kind \"usage\" and kind
\"internal\"." Bad flags and an error type this CLI doesn't specifically
classify both exit 1, so a caller that needs to tell them apart reads
`kind`. The two tiers match the two streams: the keep-going-or-stop branch
reads an integer with no parsing, and everything finer is one JSON decode
away for callers that want it.

### Exit code 130

130 is 128 plus `SIGINT`'s signal number (2), the conventional Unix exit
code for a process terminated by Ctrl-C. `classify` checks for it before any
other case:

```go
// An interrupt is checked first: a cancelled request surfaces as a
// ConnectionError wrapping context.Canceled, which would otherwise be
// reported as a transport failure.
if errors.Is(err, context.Canceled) {
    return ExitInterrupted, "interrupted"
}
```

A cancelled context reaches `classify` as an ordinary
`*typesafe.ConnectionError` wrapping `context.Canceled`, the same Go type a
DNS failure or a reset TCP connection produces. Running the
`context.Canceled` check after the `*typesafe.ConnectionError` check would
report every user-initiated Ctrl-C as exit code 7, a connection failure,
telling a calling script or agent that the network was the problem when a
human or an orchestrating process asked for the call to stop. An agent that
retries connection failures but not interruptions depends on that
distinction.

## The terminal-stdin rule

`jev ask` refuses to read from an interactive terminal:

```go
var stdinIsTerminal = func(r io.Reader) bool {
    f, ok := r.(*os.File)
    if !ok {
        return false
    }
    fi, err := f.Stat()
    return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
```

`buildRequest` calls this only when no `--file` and no question flags were
given, and only when `--file` itself was omitted entirely; an explicit `-f
-` always reads stdin, terminal or not. When stdin is a character device, a
real terminal rather than a pipe or redirected file, `buildRequest` returns
`errNoRequest` immediately instead of blocking.

The two callers recover from a blocked read differently. A human who runs
`jev ask` with no arguments by mistake sees the process hang and presses
Ctrl-C. A script or agent that invoked `jev ask` expecting output or an exit
has no such recovery: a `jev` process attached to a terminal that never gets
typed into is indistinguishable from a stuck process, and whatever is
waiting on it either blocks forever or eventually times out having wasted
that whole window. Returning `errNoRequest` immediately turns a silent hang into a
diagnosable usage error.

## The input cap

`readCapped` reads at most `maxRequestBytes` (`16 << 20`, 16 MiB) from stdin
or a file, and returns `errTooLarge` rather than reading without bound:

```go
const maxRequestBytes = 16 << 20

func readCapped(r io.Reader) ([]byte, error) {
    b, err := io.ReadAll(io.LimitReader(r, maxRequestBytes+1))
    if err != nil {
        return nil, err
    }
    if len(b) > maxRequestBytes {
        return nil, errTooLarge
    }
    return b, nil
}
```

The cap applies everywhere `jev` reads request bytes: JSON on stdin,
`--file`, and `--state @path` or `--state -`. The `LimitReader` allows one
byte past the cap rather than truncating at it, so an oversized input fails
the length check instead of passing a truncated, likely invalid, document
further down the pipeline.

A human is not going to paste gigabytes into a terminal, and an
interactively-run tool could reasonably skip this. The cap covers the
agent-facing case: a `jev` process whose stdin is connected to the output of
another program, potentially another agent, that misbehaves or is itself
compromised. Without the cap, a pipe that never closes and keeps producing
bytes grows `jev`'s memory without bound, and one malfunctioning upstream
process becomes a resource-exhaustion problem for whatever is running `jev`.
`tools/selfreview`, described in
[What the self-review measures](what-the-self-review-measures.md), is a
caller of exactly this shape: a program that constructs JSON and pipes it
into `jev ask`. The limit therefore lives at the CLI layer and not only in
documentation advice.

## The consumption pattern

The pieces above combine into one consumption pattern. Build a request
document in the same shape the API takes directly. Pass it to `jev ask` on
stdin or through `--file`. Parse stdout as JSON whatever the exit code was,
since both the success shape and the `{"error": ...}` envelope arrive there.
Branch control flow, meaning retry, fail, or escalate, on the exit code
alone. Treat stderr as optional, human-readable context rather than a source
of structured data. The pattern needs the documented shapes on each stream
and the exit code table, and nothing about `jev`'s internals.

## Related documentation

- For the exact flag reference, JSON shapes, and telemetry environment
  variables, see the [jev CLI reference](../reference/jev-cli.md).
- For the full error-classification table and error JSON shape, see
  [Errors and exit codes](../reference/errors-and-exit-codes.md).
- For a step-by-step walkthrough of wiring a script or agent to `jev`, see
  [How to drive jev from a script or agent](../how-to/drive-jev-from-a-script-or-agent.md).
