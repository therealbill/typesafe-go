# typesafe-go

Go client for the [TypeSafe](https://typesafe.ai) System One API, plus the `jev` command-line tool. Ask Jev typed questions about a piece of state and get back probabilities, choices, and scores your code can act on.

## Install

```bash
go get github.com/therealbill/typesafe-go
```

Prebuilt `jev` binaries for macOS and Linux are attached to each [release](https://github.com/therealbill/typesafe-go/releases). Or `make build` to produce `bin/jev`.

## Library

```go
client, err := typesafe.NewClient() // reads TYPESAFE_API_KEY
res, err := client.SystemOne(ctx, "I was charged twice and nobody replies.", typesafe.Questions{
    "billing": typesafe.Noul{Instructions: "Is this about billing?"},
    "tone":    typesafe.Choice{Instructions: "What is the tone?", Criteria: map[string]typesafe.JSONContent{"calm": nil, "angry": nil}},
})
fmt.Println(res.Nouls()["billing"].Noul, res.Choices()["tone"].Choice)
```

## Command line

```bash
export TYPESAFE_API_KEY=...
jev ask --state "I was charged twice." --noul billing="Is this about billing?"
echo '{"state":"...","questions":{"billing":{"type":"noul","instructions":"Is this about billing?"}}}' | jev ask --pretty
```

## Documentation

- Tutorials: [Your first judgment in Go](docs/tutorials/first-judgment-in-go.md), [jev from the command line](docs/tutorials/jev-from-the-command-line.md)
- How-to: [rate limits and retries](docs/how-to/handle-rate-limits-and-retries.md), [decode into your own struct](docs/how-to/decode-answers-into-your-own-struct.md), [trace calls and send to Honeycomb](docs/how-to/trace-calls-and-send-to-honeycomb.md), [call through a gateway](docs/how-to/call-through-a-gateway.md), [drive jev from a script or agent](docs/how-to/drive-jev-from-a-script-or-agent.md)
- Reference: [client options and environment](docs/reference/client-options-and-environment.md), [question types](docs/reference/question-types.md), [response types](docs/reference/response-types.md), [errors and exit codes](docs/reference/errors-and-exit-codes.md), [jev CLI](docs/reference/jev-cli.md), [span attributes](docs/reference/span-attributes.md)
- Explanation: [why the core is stdlib-only](docs/explanation/why-the-core-is-stdlib-only.md), [retries and budgets](docs/explanation/retries-and-budgets.md), [why content is not traced by default](docs/explanation/why-content-is-not-traced-by-default.md), [mapping from the Python SDK](docs/explanation/mapping-from-the-python-sdk.md)

## Environment

| Variable | Purpose | Default |
|---|---|---|
| `TYPESAFE_API_KEY` | API key (required) | |
| `TYPESAFE_BASE_URL` | API root | `https://api.typesafe.ai` |
| `TYPESAFE_DEFAULT_MODEL` | Model | `jev-latest` |
| `TYPESAFE_LOG_LEVEL` | `debug`, `info`, `warning`, `error`, `off` | off |
| `HONEYCOMB_API_KEY` | Enables tracing in `jev` and sets the Honeycomb header | |
| `OTEL_SERVICE_NAME` | Service name for `jev` traces | `jev` |

## License

BSD 3-Clause. See [LICENSE](LICENSE).
