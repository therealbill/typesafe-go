---
title: "How to Call Through a Gateway"
description: "Point the client at a gateway or proxy instead of the default TypeSafe endpoint, add gateway authentication headers, route through a custom HTTP transport, and pass gateway-specific request fields."
diataxis: how-to
weight: 30
---

**Goal**: Route `typesafe-go` traffic through an API gateway or corporate
proxy instead of `https://api.typesafe.ai` directly, while keeping the
client's own authentication and adding what the gateway requires.

## Prerequisites

- A working `SystemOne` call, see
  [Your First Judgment in Go](../tutorials/first-judgment-in-go.md)
- Familiarity with client options (`Option`) and per-call options
  (`RequestOption`), see the
  [client options and environment reference](../reference/client-options-and-environment.md)

## Steps

### 1. Point the client at the gateway

`WithBaseURL` replaces `https://api.typesafe.ai` with any URL serving the
same API surface:

```go
client, err := typesafe.NewClient(
	typesafe.WithAPIKey("your-api-key"),
	typesafe.WithBaseURL("https://gateway.internal.example.com"),
)
```

Outside of code, set `TYPESAFE_BASE_URL` in the environment instead.
`WithBaseURL` only needs to be called when you want to override that
per-client, for example to send one client through a gateway and another
straight to the API.

### 2. Add gateway authentication headers

The client already sends `Authorization: Bearer <key>` on every request.
`WithHeaders` adds further headers (such as a gateway's own API key header)
alongside it, client-wide:

```go
client, err := typesafe.NewClient(
	typesafe.WithAPIKey("your-api-key"),
	typesafe.WithBaseURL("https://gateway.internal.example.com"),
	typesafe.WithHeaders(http.Header{"X-Gateway-Key": {"gateway-secret"}}),
)
```

Both headers reach the gateway on every call. Avoid naming a header
`Authorization` in `WithHeaders` (or the per-call `WithExtraHeaders`) unless
you intend to replace the client's own. Headers are applied after the
client sets `Authorization`, so a same-named header wins.

### 3. Route through a custom transport

`WithHTTPClient` accepts any `*http.Client`, including one configured with a
proxy or custom TLS settings:

```go
proxyURL, _ := url.Parse("https://proxy.internal.example.com:8443")
httpClient := &http.Client{
	Transport: &http.Transport{
		Proxy:           http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	},
}

client, err := typesafe.NewClient(
	typesafe.WithAPIKey("your-api-key"),
	typesafe.WithHTTPClient(httpClient),
)
```

The SDK copies the `http.Client` struct you pass in. It does not hold a
reference to it, so anything it does internally (such as wrapping the
transport for instrumentation) never mutates `httpClient` itself. You can
safely reuse the same `*http.Client` value elsewhere in your program.

### 4. Add gateway-specific request fields

Some gateways expect extra top-level fields on every request body, such as a
tenant ID or a routing key. `WithExtraBody` merges a map into the top level of
the JSON body for one call:

```go
res, err := client.SystemOne(ctx, state, questions,
	typesafe.WithExtraBody(map[string]any{"gateway_tenant": "acme-corp"}),
)
```

The merge is shallow and sits alongside the SDK's own `state`, `model`, and
`questions` fields. Don't reuse those names unless you mean to overwrite
them.

## Verify it works

Point the client at a local `httptest.Server` standing in for the gateway,
and confirm the request lands there:

```go
gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("gateway received: %s %s\n", r.Method, r.URL.Path)
	w.Write([]byte(`{"model":"jev-latest","answers":{"billing":{"type":"noul","noul":0.87}}}`))
}))
defer gateway.Close()

client, _ := typesafe.NewClient(
	typesafe.WithAPIKey("dummy-api-key"),
	typesafe.WithBaseURL(gateway.URL),
)
res, _ := client.SystemOne(ctx, state, questions)
fmt.Printf("billing: %.2f\n", res.Nouls()["billing"].Noul)
```

An actual run prints:

```
gateway received: POST /v1/systemone
billing: 0.87
```

Adding `WithHeaders(http.Header{"X-Gateway-Key": {"some-value"}})` from Step
2 and printing the headers the same server sees confirms both arrive
together:

```
Authorization header seen by gateway: "Bearer dummy-api-key"
X-Gateway-Key header seen by gateway:  "some-value"
```

And adding `WithExtraBody(map[string]any{"gateway_tenant": "acme-corp"})`
from Step 4 while printing the raw request body confirms the shallow merge:

```
gateway received body: {"gateway_tenant":"acme-corp","model":"jev-latest","questions":{"billing":{"instructions":"Is this about billing?","type":"noul"}},"state":"I was charged twice."}
```

✅ Requests land on the gateway, carry both authentication headers, and
include gateway-specific fields.

## Troubleshooting

### Problem: requests still go to `https://api.typesafe.ai`
**Symptom**: no traffic reaches the gateway despite calling `WithBaseURL`.
**Cause**: `TYPESAFE_BASE_URL` is unset and `WithBaseURL` wasn't
passed to `NewClient` for that client instance, or a different client
(without the option) is being used.
**Solution**: confirm the option is on the exact `Option` slice passed to
`NewClient`; there's no global default to fall back to once you're
constructing clients explicitly.

### Problem: the gateway never sees your custom header
**Symptom**: `WithHeaders` values don't appear on the request.
**Cause**: a per-call `WithExtraHeaders` with the same header name overrode
it for that call, or the header was set on the wrong client instance.
**Solution**: use `WithHeaders` for anything that should apply to every call
from a client; reserve `WithExtraHeaders` for per-call overrides.

### Problem: `gateway_tenant` (or another extra field) overwrites `state`, `model`, or `questions`
**Symptom**: the request body is missing your actual question data.
**Cause**: `WithExtraBody`'s map used one of the SDK's own top-level field
names.
**Solution**: pick field names that don't collide with `state`, `model`, or
`questions`.

## See also

- [Client options and environment](../reference/client-options-and-environment.md)
- [Your First Judgment in Go](../tutorials/first-judgment-in-go.md)
- [Handle rate limits and retries](./handle-rate-limits-and-retries.md)
