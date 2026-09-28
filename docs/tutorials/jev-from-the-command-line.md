---
title: "jev from the Command Line"
description: "Build the jev binary and use it to check its version, list models, and send System One requests in both flag and JSON mode, straight from the shell."
diataxis: tutorial
weight: 20
---

TypeSafe's System One models turn a piece of application state into typed
judgments: a probability, a label chosen from a set, or a position on a
rubric. The `jev` CLI sends those requests without writing any code. You
describe the state and the questions as flags or as a JSON file, and `jev`
prints the answers as JSON.

In this tutorial you'll build the `jev` binary, inspect it, and send it a
real support ticket in both of its input modes, using the live TypeSafe
API.

## What you'll build

By the end, you will have:

- Built `./bin/jev` from source.
- Checked its version and listed the models available to your account.
- Sent the same judgment request two ways: as command-line flags, and as a
  JSON file.
- Read a successful exit code and a failed one, and seen the JSON shape
  `jev` prints for each.

Every command below is a command you run against the live API. TypeSafe's
judgments come from a model, not a lookup table, so the probabilities,
confidence values, and scores you see will differ slightly from the ones
shown here, and will differ again between two runs of the same command.

## Prerequisites

- Go 1.25 or newer installed
- A clone of this repository, with a working directory at its root
- A TypeSafe API key exported as `TYPESAFE_API_KEY` in your shell

`jev` reads `TYPESAFE_API_KEY` from the environment automatically. You never
type it on the command line.

## Step 1: Build the binary

From the repository root, run:

```bash
make build
```

This compiles `./cmd/jev` and writes the binary to `./bin/jev`. `make`
echoes the build command it runs; the `go build` underneath produces no
output when it succeeds.

### Checkpoint

Confirm the binary exists and is executable:

```bash
ls -la bin/jev
```

```
-rwxr-xr-x  1 bill  staff  25870450 Sep 27 11:58 bin/jev
```

You now have a working `jev` binary at `./bin/jev`. Every command in the
rest of this tutorial invokes it as `./bin/jev`.

## Step 2: Check the version

Run:

```bash
./bin/jev version
```

```json
{"commit":"a48152b","go":"go1.27.1","version":"a48152b"}
```

Three fields, always in alphabetical order because they come from a Go map:
`commit`, `go`, and `version`. `go` is the Go toolchain version this binary
was built with. For why `version` and `commit` are identical here, see the
[jev CLI reference](../reference/jev-cli.md#jev-version).

### Checkpoint

You should see a JSON object with exactly these three keys and no error.
If you instead see a `command not found` error, you're not running the
binary from the repository root. Use `./bin/jev`, not `jev`.

## Step 3: List available models

Run:

```bash
./bin/jev models --pretty
```

```json
{
  "models": [
    {
      "name": "jev-latest",
      "description": "The latest iteration of TypeSafe's System One Model: Jev",
      "release_date": "2026-09-10T18:38:01.391457+00:00"
    },
    {
      "name": "jev-preview",
      "description": "A preview version of `jev-latest`: should be better in most ways",
      "release_date": "2026-09-10T18:39:06.057655+00:00"
    }
  ],
  "request_id": "req_01a0e128c0f37d98bee89da82fd6d57a"
}
```

`--pretty` indents the JSON; without it, `jev` prints the same data on one
line. `jev models` is the first command in this tutorial that calls the API.
`request_id` is the server's identifier for that call, which you can
reference when troubleshooting a specific request.

### Checkpoint

You should see at least one entry under `"models"`, each with a `name`,
`description`, and `release_date`. Your `request_id` will differ from the
one shown above. Every request gets a unique one.

## Step 4: Ask a question in flag mode

`jev ask` sends one state and a set of typed questions to the API. In flag
mode, you describe everything on the command line: `--state` is the text
being judged, and each question is one of three flags:

- `--noul key=instructions` asks a yes/no question and answers with a
  probability.
- `--choice key=instructions:label1,label2,...` asks for one label out of a
  set.
- `--score key=instructions:level0|level1|...` asks for a position on an
  ordered rubric.

Run:

```bash
./bin/jev ask --state "I was charged twice this month and nobody answers my emails. Fix it now." \
    --noul billing="Is this about billing?" \
    --choice tone="What is the tone?:calm,angry" \
    --score urgency="How urgent is this?:not urgent|somewhat urgent|very urgent"
```

```json
{"model":"jev-1.13.0","answers":{"billing":{"noul":0.98,"type":"noul"},"tone":{"choice":"angry","confidence":1,"probabilities":{"angry":1,"calm":0},"type":"choice"},"urgency":{"confidence":0.76,"legend":{"0":"not urgent","1":"somewhat urgent","2":"very urgent"},"probabilities":{"0":0,"1":0.16,"2":0.84},"score":1.84,"type":"score"}},"usage":{"input_tokens":358,"output_tokens":65},"request_id":"req_01a0e12902d2747b96449fe6197f00f7"}
```

Three answers came back, one per question. `billing` is a `noul` answer: a
0.98 probability that this is about billing. `tone` is a `choice` answer:
the model chose `"angry"`. `urgency` is a `score` answer: a
probability-weighted position between the three levels you defined. The
full flag grammar, including how repeated flags and errors are handled, is
on the [jev CLI reference](../reference/jev-cli.md#flag-mode).

### Checkpoint

You should see a JSON object with a `"billing"`, `"tone"`, and `"urgency"`
key under `"answers"`, and a top-level `"model"` field naming the model
that answered. The exact numbers will vary from what's shown above, and
will vary again if you re-run the same command. See the note on live
judgments at the top of this tutorial.

## Step 5: Ask the same request in JSON mode from a file

The second input mode sends the same information as one JSON document,
shaped like the API's own HTTP request body. This is the mode to use when
code generates your questions instead of a person typing them.

Create `request.json` in the repository root:

```json
{
  "state": "I was charged twice this month and nobody answers my emails. Fix it now.",
  "questions": {
    "billing": {
      "type": "noul",
      "instructions": "Is this about billing?"
    },
    "tone": {
      "type": "choice",
      "instructions": "What is the tone?",
      "criteria": {
        "calm": null,
        "angry": null
      }
    },
    "urgency": {
      "type": "score",
      "instructions": "How urgent is this?",
      "criteria": ["not urgent", "somewhat urgent", "very urgent"]
    }
  }
}
```

This is the same state and the same three questions as Step 4, written in
the JSON shape instead of flags. Run:

```bash
./bin/jev ask -f request.json
```

```json
{"model":"jev-1.13.0","answers":{"billing":{"noul":0.98,"type":"noul"},"tone":{"choice":"angry","confidence":1,"probabilities":{"angry":1,"calm":0},"type":"choice"},"urgency":{"confidence":0.76,"legend":{"0":"not urgent","1":"somewhat urgent","2":"very urgent"},"probabilities":{"0":0,"1":0.16,"2":0.84},"score":1.84,"type":"score"}},"usage":{"input_tokens":358,"output_tokens":65},"request_id":"req_01a0e12933347ae180185523b52e723b"}
```

The `request_id` differs from Step 4's. This was an independent live call
to the model, not a replay, so small numeric differences from Step 4's
answers (or from a second run of this same command) are expected.

You can delete `request.json` once you're done with this tutorial; it isn't
needed by anything else in the repository.

### Checkpoint

You should see the same three answer keys as Step 4, with a different
`request_id`. If you get `invalid request: "state" is required` or a
similar error instead, check that `request.json` is valid JSON and that
you ran the command from the same directory where you created it.

## Step 6: Read the exit code

Every `jev` command sets its process exit code, which is what scripts and
CI pipelines check instead of parsing output. Re-run Step 4's command and
check the exit code immediately after:

```bash
./bin/jev ask --state "I was charged twice this month and nobody answers my emails. Fix it now." \
    --noul billing="Is this about billing?" \
    --choice tone="What is the tone?:calm,angry" \
    --score urgency="How urgent is this?:not urgent|somewhat urgent|very urgent"
echo $?
```

```
0
```

`0` means success. Every other exit code `jev` can return, and exactly what
condition produces it, is documented on the
[errors and exit codes reference](../reference/errors-and-exit-codes.md).

### Checkpoint

`echo $?` must be run as the very next command after `jev ask`. Anything
in between, even another command that succeeds, overwrites `$?` with its
own exit code.

## Step 7: Provoke an auth error

Run the same command as Step 4 with `--api-key bad` added, which overrides
your real key with an invalid one for this one call:

```bash
./bin/jev ask --state "I was charged twice this month and nobody answers my emails. Fix it now." \
    --noul billing="Is this about billing?" \
    --choice tone="What is the tone?:calm,angry" \
    --score urgency="How urgent is this?:not urgent|somewhat urgent|very urgent" \
    --api-key bad
echo $?
```

stdout:

```json
{"error":{"kind":"auth","status":401,"request_id":"req_01a0e1295a8b7650acefb4abd5cee8a2","message":"authentication_error: Cannot authenticate with the server. Please check your API key and try again."}}
```

stderr:

```
jev: typesafe: POST /v1/systemone returned 401: authentication_error: Cannot authenticate with the server. Please check your API key and try again. (request id req_01a0e1295a8b7650acefb4abd5cee8a2)
```

exit code:

```
3
```

`jev` always writes the error as JSON on stdout and a one-line `jev: ...`
summary on stderr, and always sets a non-zero exit code. Here `3` means an
auth failure (an HTTP 401 or 403). The full error JSON shape and the
complete table mapping failure conditions to exit codes are on the
[errors and exit codes reference](../reference/errors-and-exit-codes.md).

`--api-key bad` only overrides the key for this one invocation; your real
`TYPESAFE_API_KEY` environment variable is untouched, so the next command
you run without `--api-key` will authenticate normally.

### Checkpoint

You should see `"kind":"auth"` and `"status":401` in the stdout JSON, a
`jev: ...` line on stderr, and `3` printed by `echo $?`.

## What you built

You built `jev` from source and used it to:

- Print version information with no network call.
- List the models available to your account.
- Send the same judgment request in both of `jev ask`'s input modes.
- Read a successful exit code and a failed one, and recognize `jev`'s
  error JSON shape.

## Next steps

- **See the full flag and command reference**: [jev CLI reference](../reference/jev-cli.md)
- **Look up any exit code or error field**: [errors and exit codes reference](../reference/errors-and-exit-codes.md)
- **Call System One from Go instead of the shell**: [client options and environment](../reference/client-options-and-environment.md) covers building a `*Client` and calling `SystemOne` from your own program
- **Script or automate `jev` calls**: [Drive jev from a script or agent](../how-to/drive-jev-from-a-script-or-agent.md)

## Troubleshooting

### Problem: `jev: typesafe: API key is required`

**Symptom**: Every command fails immediately with exit code `1` and this
message.
**Solution**: Export your API key before running any command:
`export TYPESAFE_API_KEY=<your key>`. `jev` reads it from the environment;
it does not prompt for it.

### Problem: `--file cannot be combined with --state, --noul, --choice, or --score`

**Symptom**: Running `jev ask` with both `-f` and a question flag fails
with exit code `1`.
**Solution**: Pick one input mode per call: flags (Step 4) or a JSON file
(Step 5), never both.

### Need more help?

- [jev CLI reference](../reference/jev-cli.md)
- [Errors and exit codes reference](../reference/errors-and-exit-codes.md)
