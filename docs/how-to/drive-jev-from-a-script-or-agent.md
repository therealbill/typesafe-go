---
title: "How to Drive jev from a Script or Agent"
description: "The stdin/stdout/exit-code contract a script or autonomous agent relies on when running jev as a subprocess."
type: how-to
---

# How to Drive jev from a Script or Agent

**Goal**: Treat `jev` as a subprocess API — feed it JSON on stdin, parse its
JSON on stdout, and branch reliably on its exit code — the way a script or
an autonomous agent should.

## Prerequisites

- A built `./bin/jev` and a `TYPESAFE_API_KEY` — see
  [jev from the Command Line](../tutorials/jev-from-the-command-line.md)
- `jq` installed, for the examples below
- Full flag and JSON-shape reference:
  [jev CLI](../reference/jev-cli.md) and
  [errors and exit codes](../reference/errors-and-exit-codes.md)

## Steps

### 1. Send the request as JSON on stdin

With no `-f`, `jev ask` reads the request body shape from stdin:

```json
{
  "state": "...",
  "questions": { "<id>": { "type": "noul" | "choice" | "score" | "...", "...": "..." } },
  "model": "..."
}
```

`model` is optional. `questions.<id>.type` of `noul`, `choice`, or `score`
decodes into the matching typed question; anything else is passed through
as a `RawQuestion`. Any top-level field besides `state`, `questions`, and
`model` is forwarded as extra request-body fields
(`typesafe.WithExtraBody`) — useful for API fields `jev` doesn't model yet
without needing a new release to send them.

### 2. Parse the success shape on stdout

On success, `jev ask` writes one JSON object to stdout:

```json
{"model": "...", "answers": {"...": {}}, "usage": {"input_tokens": 0, "output_tokens": 0}, "request_id": "..."}
```

`request_id` is omitted when empty. This is the only thing written to
stdout on success — safe to pipe straight into `jq` or a JSON parser with no
extra filtering.

### 3. Parse the error shape on failure

On failure, `jev` writes JSON to **stdout**, not just an error to stderr:

```json
{"error": {"kind": "...", "status": 0, "request_id": "...", "message": "..."}}
```

`status` and `request_id` are omitted when zero/empty (they're only present
for API errors). A one-line human summary always goes to stderr as well,
regardless of `--pretty`:

```
jev: <error>
```

A script that only checks the exit code and ignores stdout on failure will
miss `kind`, `status`, and `request_id` — parse stdout on both success and
failure paths.

### 4. Branch on the exit code

| Code | Meaning |
|---|---|
| 0 | OK |
| 1 | Usage — bad flags, unreadable/invalid JSON, missing API key |
| 2 | Validation — request failed client-side validation |
| 3 | Auth — 401 or 403 |
| 4 | Request — other 4xx (400, 404, 422) |
| 5 | Rate limit — 429, after retries |
| 6 | Server — 5xx after retries, or an unreadable 2xx body |
| 7 | Connection — failure or timeout |

Full classification order and edge cases:
[errors and exit codes](../reference/errors-and-exit-codes.md).

### 5. Choose `--raw` or the default encoding

Without `--raw`, `jev` writes its own re-encoding of the parsed response
(Step 2), honoring `--pretty`. With `--raw`, it writes the server's response
bytes unchanged, followed by a trailing newline — reach for this when a
script or agent wants to pass the API's response straight through
untouched, byte for byte, rather than through `jev`'s own JSON encoder.

### 6. Skip `--pretty` when piping to `jq`

`--pretty` indents JSON for a human reading it directly. A script parsing
output with `jq` or any JSON library doesn't need it and should generally
omit it — compact JSON is simpler to pipe and marginally cheaper to
produce and parse.

## Verify it works

### A `case` over the exit code

```bash
#!/usr/bin/env bash
request='{"state":"I was charged twice this month and nobody answers my emails.","questions":{"tone":{"type":"choice","instructions":"What is the tone?","criteria":{"calm":null,"angry":null}}}}'

echo "$request" | ./bin/jev ask
code=$?

case $code in
  0) echo "ok" ;;
  3) echo "auth failure - check API key" ;;
  5) echo "rate limited" ;;
  7) echo "connection failure" ;;
  *) echo "other error (exit $code)" ;;
esac
```

Real run, success path (valid `TYPESAFE_API_KEY` in the environment):

```
$ echo "$request" | ./bin/jev ask
{"model":"jev-1.13.0","answers":{"tone":{"choice":"angry","confidence":1,"probabilities":{"angry":1,"calm":0},"type":"choice"}},"usage":{"input_tokens":301,"output_tokens":34},"request_id":"req_01a0e12f47cc7946a4bc868a5809fa4d"}
exit code: 0
ok
```

Real run, failure path (`--api-key bad` forces a real 401 from the API):

```
$ echo "$request" | ./bin/jev ask --api-key bad
{"error":{"kind":"auth","status":401,"request_id":"req_01a0e12f5cf574ccb3df8402494598d9","message":"authentication_error: Cannot authenticate with the server. Please check your API key and try again."}}
jev: typesafe: POST /v1/systemone returned 401: authentication_error: Cannot authenticate with the server. Please check your API key and try again. (request id req_01a0e12f5cf574ccb3df8402494598d9)
exit code: 3
auth failure - check API key
```

Both branches fired exactly as the table in Step 4 says: exit 0 on success,
exit 3 for the auth error, with the error JSON on stdout and the `jev:`
summary on stderr.

### A `jq` one-liner

Extract one answer's chosen label straight out of a live response:

```bash
$ ./bin/jev ask --state "I was charged twice this month and nobody answers my emails." \
    --choice tone="What is the tone?:calm,angry" | jq -r '.answers.tone.choice'
angry
```

✅ Success! The subprocess contract — JSON in, JSON out, a meaningful exit
code — holds for both the happy path and a real failure.

## Troubleshooting

### Problem: a script treats any non-zero exit as the same failure
**Symptom**: retry logic fires on a 401 (which will never succeed) the same
way it fires on a 429 or a connection error (which might).
**Cause**: exit codes are collapsed to a boolean check instead of switched
on.
**Solution**: `case`/`switch` over the full table in Step 4 — retry on 5 and
7, never on 1, 2, or 3.

### Problem: error details are missing even though the script checked stdout
**Symptom**: a script logs only the stderr `jev: <error>` line and loses
`kind`/`status`/`request_id`.
**Cause**: on failure, the structured error object is on stdout, not
stderr — the reverse of many CLIs' convention.
**Solution**: always parse stdout as JSON, on both success and failure;
branch on `.error` being present.

### Problem: `jq` fails to parse `jev`'s output
**Symptom**: `jq: error: Invalid numeric literal` or similar, only with
`--raw`.
**Cause**: `--raw` passes the server's bytes through unchanged — if the
upstream response isn't the shape your filter expects, `jq` will choke on
it same as any other unexpected JSON.
**Solution**: drop `--raw` unless you specifically want the untouched
server body; the default encoding is guaranteed to match the documented
shape.

### Problem: exit code 1 with no `{"error":...}` on stdout
**Symptom**: only `jev: <error>\nRun 'jev --help' for usage.` on stderr,
nothing on stdout.
**Cause**: the failure happened before `jev` built a request at all (a
Cobra-level flag-parsing error), so it never reached the code path that
writes structured JSON.
**Solution**: treat this as a pure usage bug in how the script invokes
`jev` — fix the flags, don't retry.

## See also

- [jev CLI](../reference/jev-cli.md)
- [Errors and exit codes](../reference/errors-and-exit-codes.md)
- [jev from the Command Line](../tutorials/jev-from-the-command-line.md)
