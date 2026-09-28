---
title: "The Agent-Facing CLI Contract"
description: "Why jev ask mirrors the HTTP wire shape, separates its JSON and human-readable output, and is built around exit codes and input limits that a non-human caller needs."
diataxis: explanation
weight: 70
---

# The Agent-Facing CLI Contract

`jev` is a normal-looking command-line tool — flags, stdin, stdout, an exit
code — but several of its design choices only make sense once you notice
who its typical caller is. A human running `jev ask` interactively is a
secondary use case; the primary one is a script or an autonomous agent
spawning `jev` as a subprocess, feeding it a request, and needing to know
programmatically, and quickly, what happened. This page discusses the design
choices that follow from that: why the input format needs no translation
layer, why failure is reported twice in two different ways, why the exit
codes are shaped the way they are, and why the tool refuses to do the one
thing a human might expect of it — wait for you to type something.

## Why the input mirrors the wire format

`jev ask` reads a JSON document — from `--file`, or from stdin when no file
or question flags are given — and expects it to look like this:

```json
{"state": ..., "questions": {"id": {"type": "noul", ...}}, "model": "..."}
```

`parseRequestJSON` in `internal/cli/request.go` requires exactly `state` and
`questions`, treats `model` as optional, and forwards anything else as extra
body fields. That's not a CLI-specific schema invented for `jev ask` — it's
the same shape `SystemOne` builds and sends over HTTP:

```go
body := map[string]any{"state": state, "model": rc.model, "questions": questions}
```

The CLI's input format and the library's wire format are the same document
on purpose. A caller — human or automated — who already knows the API's
request body, from reading the HTTP reference or from having built one for
a direct API call, needs no CLI-specific mental model layered on top. There
is nothing to learn about how `jev` "wraps" a request, because it doesn't;
it decodes the document, calls `SystemOne`, and re-serializes the result. An
agent generating this JSON programmatically can generate exactly what it
would send to the HTTP endpoint directly, and pipe that same document to
`jev ask` interchangeably.

## Why every failure writes to two streams, differently

When a call fails, `fail` in `internal/cli/exit.go` writes to both stdout
and stderr, but not the same thing to each:

```go
_ = writeJSON(io.Out, map[string]any{"error": p}, pretty)
_, _ = fmt.Fprintln(io.Err, "jev:", sanitize(err.Error()))
```

stdout always gets a JSON object — `{"error": {"kind", "status",
"request_id", "message"}}` — whether the call succeeded or failed. That's
deliberate: a caller parsing `jev`'s output doesn't need to branch its
parsing logic on whether the call worked. It reads stdout as JSON either
way and looks for an `error` key, rather than needing separate success and
failure code paths just to know which stream to read.

stderr gets a different, human-oriented rendering of the same error, and
that line is passed through `sanitize` first:

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

Every control character below `0x20` except tab, plus DEL, is rewritten as
a `\xNN` escape. The reason this matters specifically for `jev` is that the
text being sanitized isn't text `jev` wrote — `err.Error()` can contain a
message that came back from the remote API, verbatim, inside `APIError`'s
`Message()`. A malicious or merely buggy upstream response could embed
ANSI/terminal escape sequences (which begin with the `0x1b` ESC byte, one of
the characters `sanitize` escapes) designed to move the cursor, hide text,
or otherwise manipulate whatever terminal happens to render that stderr
line. Escaping those bytes before they reach a terminal closes that off.
Note this sanitization is stderr-only: the stdout JSON doesn't need it,
because `encoding/json` already escapes control characters as part of
producing valid JSON.

## The exit-code design: coarse for branching, fine-grained in the JSON

`classify` in `internal/cli/exit.go` maps every error to one of nine exit
codes — `ExitOK` (0) through `ExitConnection` (7), plus `ExitInterrupted`
(130) — and the comment above the constants states the intent directly:
"A caller can branch on these without parsing output." A script that only
cares about "should I retry" or "should I give up" can switch on the raw
exit code and never touch stdout at all: retry on 5 (rate limited) and 7
(connection failure), never on 3 (auth) or 2 (validation), because those
will fail identically on a retry.

That coarseness is deliberate but not the whole story — the JSON `kind`
field on stdout is finer-grained than the exit code, and the code comment
says so explicitly: "code 1 covers both kind \"usage\" and kind
\"internal\"." A caller that wants to distinguish "you passed bad flags"
from "an error type this CLI doesn't specifically classify occurred" has to
read `kind`, because both currently exit 1. The two-tier design — a small
number of exit codes for the common "keep going or stop" branch, a richer
`kind` string in the body for anyone who wants more — mirrors the two
streams above: cheap, structural information one caller needs is available
without parsing; everything else is one JSON decode away for callers that
want it.

### Why 130, and why it's checked first

130 is exit code 128 + `SIGINT`'s signal number (2) — the conventional Unix
exit code for a process terminated by Ctrl-C. `classify` checks for it
before any other case:

```go
// An interrupt is checked first: a cancelled request surfaces as a
// ConnectionError wrapping context.Canceled, which would otherwise be
// reported as a transport failure.
if errors.Is(err, context.Canceled) {
    return ExitInterrupted, "interrupted"
}
```

The ordering matters because of what a cancelled context looks like by the
time it reaches `classify`: at the library level, a context cancellation
surfaces as an ordinary `*typesafe.ConnectionError` wrapping
`context.Canceled` — the same Go type a DNS failure or a reset TCP
connection would produce. If the `context.Canceled` check ran after the
`*typesafe.ConnectionError` check instead of before it, every user-initiated
Ctrl-C would be misreported as exit code 7, "connection failure" — telling a
calling script or agent that the network was the problem, when the truth is
that a human, or an orchestrating process, asked for the call to stop. An
agent that retries on connection failures but not on interruption needs that
distinction to behave correctly; checking `context.Canceled` first is what
makes it available.

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
given, and only when `--file` itself was omitted entirely (an explicit `-f
-` always reads stdin, terminal or not). If stdin is a character device — a
real terminal, not a pipe or redirected file — it returns `errNoRequest`
immediately instead of blocking.

The reasoning is about what "no input yet" looks like from the two possible
callers' perspectives. A human running `jev ask` with no arguments, by
mistake, sees the process hang and can just Ctrl-C it — mildly annoying, but
recoverable in a few seconds once they notice. A script or agent that
invoked `jev ask` expecting it to either produce output or exit has no such
recovery: from its perspective, a `jev` process attached to a terminal that
never gets typed into looks identical to a genuinely stuck process, and
whatever is waiting on it either blocks forever or eventually times out
having wasted that whole window. Failing fast with a clear "no request
given" usage error converts a silent hang into an immediate, diagnosable
error — the right trade for a tool whose primary caller can't interactively
notice and correct a hang the way a human would.

## The input cap

`readCapped` refuses to read more than `maxRequestBytes` — `16 << 20`, 16
MiB — from stdin or a file, returning `errTooLarge` instead of reading
without bound:

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

This applies everywhere `jev` reads request bytes: JSON on stdin, `--file`,
and `--state @path`/`--state -`. Reading one byte past the cap, rather than
truncating at it, is what lets the check detect an oversized input instead
of silently handing a truncated (and likely invalid) document further down
the pipeline.

An interactively-run tool could reasonably skip this — a human isn't going
to paste gigabytes into a terminal. The risk this guards against is
specific to the agent-facing case: a `jev` process whose stdin is connected
to the output of another program, potentially another agent, that
misbehaves or is itself compromised. Without a cap, a pipe that never closes
and keeps producing bytes would grow `jev`'s memory without bound, turning
one malfunctioning upstream process into a resource-exhaustion problem for
whatever is running `jev`. `tools/selfreview`, described in
[What the self-review measures](what-the-self-review-measures.md), is
itself exactly this shape of caller — a program that constructs JSON and
pipes it into `jev ask` — which is part of why this limit exists at the CLI
layer and not just as documentation advice.

## How a script or agent is expected to consume `jev`

Put together, the contract this page has walked through describes one
consumption pattern: build a request document in the same shape you'd send
to the API directly; pipe or pass it to `jev ask` via stdin or `--file`;
always parse stdout as JSON, regardless of what the exit code was, since the
success shape and the `{"error": ...}` envelope are both there waiting;
branch control flow — retry, fail, escalate — on the exit code alone,
without needing to parse anything to make that decision; and treat stderr as
optional, human-readable context rather than a source of structured data.
Nothing in that pattern requires knowing `jev`'s internals — only the
documented shapes on each stream and the exit code table.

## Related documentation

- For the exact flag reference, JSON shapes, and telemetry environment
  variables, see the [jev CLI reference](../reference/jev-cli.md).
- For the full error-classification table and error JSON shape, see
  [Errors and exit codes](../reference/errors-and-exit-codes.md).
- For a step-by-step walkthrough of wiring a script or agent to `jev`, see
  [How to drive jev from a script or agent](../how-to/drive-jev-from-a-script-or-agent.md).
