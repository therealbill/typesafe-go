---
title: "Your First Judgment in Go"
description: "Send a support ticket to the TypeSafe System One API from a Go program and get back a probability, a choice, and a score."
diataxis: tutorial
weight: 10
---

TypeSafe's System One models turn a piece of application state into typed
judgments: a probability, a label chosen from a set, or a position on a
rubric. In this tutorial you'll write a small Go program that sends a support
ticket to the API and gets back all three kinds of answer at once.

## What you'll build

A command-line Go program that:

1. Sends a support ticket's text to TypeSafe as `state`.
2. Asks three questions about it: is this a billing problem (a probability),
   what's the tone (a label), and how urgent is it (a score).
3. Prints the answers, then the raw JSON the API returned.
4. Decodes the same answers into your own Go struct instead of maps.
5. Handles a failed request the way real code should, using `errors.As`.

Every command below is a command you run. The checkpoints show output
captured from the live API. TypeSafe's judgments come from a model, not a
lookup table, so your probabilities, confidence values, and scores will land
close to the values shown here and will rarely match them to the decimal.

## Prerequisites

- Go 1.25 or newer installed
- A TypeSafe API key

## Step 1: Set up a module and install the library

Create a new directory and initialize a Go module:

```bash
mkdir first-judgment
cd first-judgment
go mod init first-judgment
```

Install the library:

```bash
go get github.com/therealbill/typesafe-go@latest
```

Now export your API key so the library can find it. `typesafe.NewClient()`
reads it from the `TYPESAFE_API_KEY` environment variable automatically. You
never pass it in code:

```bash
export TYPESAFE_API_KEY=<your API key>
```

### Checkpoint

Open `go.mod`. You should see a `require` line for
`github.com/therealbill/typesafe-go`, and a `go.sum` file should now exist
alongside it. The library has no third-party dependencies of its own.

## Step 2: Ask three questions about a ticket

Create `main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	typesafe "github.com/therealbill/typesafe-go"
)

func main() {
	client, err := typesafe.NewClient()
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	state := map[string]any{
		"ticket": "I was charged twice this month and nobody answers my emails. Fix it now.",
	}

	questions := typesafe.Questions{
		"billing": typesafe.Noul{
			Instructions: "Is `ticket` about a billing problem?",
		},
		"tone": typesafe.Choice{
			Instructions: "What is the tone of `ticket`?",
			Criteria: map[string]typesafe.JSONContent{
				"calm":    "Polite and patient",
				"angry":   "Frustrated or hostile",
				"neutral": nil,
			},
		},
		"urgency": typesafe.Score{
			Instructions: "How urgent is `ticket`?",
			Criteria: []typesafe.JSONContent{
				"Not urgent at all",
				"Somewhat urgent",
				"Very urgent",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := client.SystemOne(ctx, state, questions)
	if err != nil {
		log.Fatalf("SystemOne call failed: %v", err)
	}

	fmt.Printf("billing: %.2f\n", res.Nouls()["billing"].Noul)
	fmt.Printf("tone: %s\n", res.Choices()["tone"].Choice)
	fmt.Printf("urgency: %.2f\n", res.Scores()["urgency"].Score)
}
```

Before you run it:

- `state` is the piece of the world you're asking about. It can be a string,
  a map, a slice, or a struct. Anything that encodes to JSON works.
- `typesafe.Questions` is a map from an identifier you choose (`"billing"`,
  `"tone"`, `"urgency"`) to a question. That identifier is how you find the
  matching answer later; the model never sees it.
- A `Noul` asks a yes/no question and answers with a probability. A `Choice`
  asks for one label out of a fixed set. A `Score` asks for a position on an
  ordered rubric, where `Criteria[i]` describes level `i`.

The full set of question fields is on the
[question types reference](../reference/question-types.md); client
construction and configuration are on the
[client options and environment reference](../reference/client-options-and-environment.md).

Run it:

```bash
go run main.go
```

### Checkpoint

You should see output like this (an actual run produced exactly this):

```
billing: 0.98
tone: angry
urgency: 1.89
```

`billing` is a probability from 0 to 1. A value of 0.98 means the model is
almost certain this is a billing issue. `tone` is the chosen label. `urgency`
is a probability-weighted position on the 0–2 rubric you defined. Here, 1.89
sits between "somewhat urgent" (1) and "very urgent" (2), leaning toward the
top.

## Step 3: See the raw JSON

The typed accessors from Step 2 (`Nouls()`, `Choices()`, `Scores()`) are
built from the response's underlying JSON. Every `*SystemOneResponse`
keeps the original bytes on `Raw.Body`, so you can look at exactly what the
API sent back.

Replace the body of `main.go` with a version that pretty-prints that raw body
instead of the three-line summary:

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	typesafe "github.com/therealbill/typesafe-go"
)

func main() {
	client, err := typesafe.NewClient()
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	state := map[string]any{
		"ticket": "I was charged twice this month and nobody answers my emails. Fix it now.",
	}

	questions := typesafe.Questions{
		"billing": typesafe.Noul{
			Instructions: "Is `ticket` about a billing problem?",
		},
		"tone": typesafe.Choice{
			Instructions: "What is the tone of `ticket`?",
			Criteria: map[string]typesafe.JSONContent{
				"calm":    "Polite and patient",
				"angry":   "Frustrated or hostile",
				"neutral": nil,
			},
		},
		"urgency": typesafe.Score{
			Instructions: "How urgent is `ticket`?",
			Criteria: []typesafe.JSONContent{
				"Not urgent at all",
				"Somewhat urgent",
				"Very urgent",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := client.SystemOne(ctx, state, questions)
	if err != nil {
		log.Fatalf("SystemOne call failed: %v", err)
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, res.Raw.Body, "", "  "); err != nil {
		log.Fatalf("failed to indent raw response body: %v", err)
	}
	fmt.Println(pretty.String())
}
```

`res.Raw.Body` is already JSON text (a `[]byte`), so this uses
`json.Indent` to format it, not `json.Marshal`.

Run it:

```bash
go run main.go
```

### Checkpoint

An actual run produced this (yours will carry the same shape, with your own
probabilities and scores):

```json
{
  "model": "jev-1.13.0",
  "answers": {
    "billing": {
      "type": "noul",
      "noul": 0.98
    },
    "tone": {
      "type": "choice",
      "choice": "angry",
      "confidence": 1.0,
      "probabilities": {
        "neutral": 0.0,
        "calm": 0.0,
        "angry": 1.0
      }
    },
    "urgency": {
      "type": "score",
      "score": 1.92,
      "confidence": 0.88,
      "legend": {
        "0": "Not urgent at all",
        "1": "Somewhat urgent",
        "2": "Very urgent"
      },
      "probabilities": {
        "0": 0.0,
        "1": 0.08,
        "2": 0.92
      }
    }
  },
  "usage": {
    "input_tokens": 407,
    "output_tokens": 72
  }
}
```

> **Reading this response**
>
> This was a separate call from Step 2's, so `urgency.score` reads 1.92 here
> instead of 1.89. Each call is an independent model judgment.
>
> The response names a concrete, versioned model in `"model": "jev-1.13.0"`,
> even though nothing in the program asked for one. The library's default is
> `jev-latest`, which the server resolves to whichever version currently
> backs it.
>
> Every answer carries its own `"type"` field (`noul`, `choice`, `score`),
> which is how the library knows which Go type to decode that answer into.
> The full shape of this response (every field, and how each answer type
> decodes) is documented on the
> [response types reference](../reference/response-types.md).

## Step 4: Decode into your own struct

Reading `res.Nouls()["billing"]` works, but it means threading string keys
through your code. `typesafe.SystemOneAs[T]` decodes straight into a
struct of your own instead, tagged with the same identifiers you used in
`Questions`.

Replace `main.go` again:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	typesafe "github.com/therealbill/typesafe-go"
)

type TicketAnswers struct {
	Billing typesafe.NoulAnswer   `json:"billing"`
	Tone    typesafe.ChoiceAnswer `json:"tone"`
	Urgency typesafe.ScoreAnswer  `json:"urgency"`
}

func main() {
	client, err := typesafe.NewClient()
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	state := map[string]any{
		"ticket": "I was charged twice this month and nobody answers my emails. Fix it now.",
	}

	questions := typesafe.Questions{
		"billing": typesafe.Noul{
			Instructions: "Is `ticket` about a billing problem?",
		},
		"tone": typesafe.Choice{
			Instructions: "What is the tone of `ticket`?",
			Criteria: map[string]typesafe.JSONContent{
				"calm":    "Polite and patient",
				"angry":   "Frustrated or hostile",
				"neutral": nil,
			},
		},
		"urgency": typesafe.Score{
			Instructions: "How urgent is `ticket`?",
			Criteria: []typesafe.JSONContent{
				"Not urgent at all",
				"Somewhat urgent",
				"Very urgent",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	answers, _, err := typesafe.SystemOneAs[TicketAnswers](ctx, client, state, questions)
	if err != nil {
		log.Fatalf("SystemOneAs call failed: %v", err)
	}

	fmt.Printf("billing: %.2f\n", answers.Billing.Noul)
	fmt.Printf("tone: %s\n", answers.Tone.Choice)
	fmt.Printf("urgency: %.2f\n", answers.Urgency.Score)
}
```

The field tags (`json:"billing"`, `json:"tone"`, `json:"urgency"`) tie each
struct field back to the identifier you chose in `Questions`, the same three
names throughout. `SystemOneAs` also returns the full
`*SystemOneResponse` as its second value (ignored here with `_`), so you
still have access to `Usage`, `RequestID`, and `Raw` when you need them.

Run it:

```bash
go run main.go
```

### Checkpoint

An actual run produced:

```
billing: 0.98
tone: angry
urgency: 1.91
```

Same three answers as Step 2, now reached through struct fields instead of
map lookups. This was again a fresh call, so `urgency` moved slightly. For a
task-focused walkthrough of designing answer structs (including optional
questions and multiple question sets), see
[Decode answers into your own struct](../how-to/decode-answers-into-your-own-struct.md).

## Step 5: Handle a failed request

`SystemOne` and `SystemOneAs` return a `*typesafe.APIError` when a request
fails: an expired key, a bad request, a rate limit. `errors.As` recognizes
that type, and the error carries a status code and a human-readable message.

Build a client with a key the server will reject to see one:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	typesafe "github.com/therealbill/typesafe-go"
)

func main() {
	client, err := typesafe.NewClient(typesafe.WithAPIKey("wrong-key"))
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	state := map[string]any{
		"ticket": "I was charged twice this month and nobody answers my emails. Fix it now.",
	}

	questions := typesafe.Questions{
		"billing": typesafe.Noul{
			Instructions: "Is `ticket` about a billing problem?",
		},
		"tone": typesafe.Choice{
			Instructions: "What is the tone of `ticket`?",
			Criteria: map[string]typesafe.JSONContent{
				"calm":    "Polite and patient",
				"angry":   "Frustrated or hostile",
				"neutral": nil,
			},
		},
		"urgency": typesafe.Score{
			Instructions: "How urgent is `ticket`?",
			Criteria: []typesafe.JSONContent{
				"Not urgent at all",
				"Somewhat urgent",
				"Very urgent",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, err = client.SystemOne(ctx, state, questions)
	if err == nil {
		log.Fatal("expected an error but got none")
	}

	var apiErr *typesafe.APIError
	if errors.As(err, &apiErr) {
		fmt.Printf("status: %d\n", apiErr.Status)
		fmt.Printf("message: %s\n", apiErr.Message())
	} else {
		fmt.Printf("unexpected error: %v\n", err)
	}
}
```

`typesafe.WithAPIKey("wrong-key")` overrides the environment variable for
this client only, so your real `TYPESAFE_API_KEY` is untouched and unused
here, and this client cannot authenticate. `errors.As` finds the
`*typesafe.APIError` even when it is wrapped inside another error, so use it
in your own code any time a `SystemOne` call fails.

Run it:

```bash
go run main.go
```

### Checkpoint

An actual run against the live API produced:

```
status: 401
message: authentication_error: Cannot authenticate with the server. Please check your API key and try again.
```

`Status` is the HTTP status code the server responded with. `Message()`
extracts a human-readable string from the response body, so you don't have
to parse `apiErr.Body` yourself in the common case.

Once you're done experimenting, unset the fake key by not passing
`WithAPIKey`. Your program goes back to reading `TYPESAFE_API_KEY` from the
environment, as it did in Steps 2 through 4.

## What you built

You now have a Go program that sends application state to TypeSafe, asks
three different kinds of question about it in a single call, reads the
answers both as maps and as your own struct, and handles a failed request
the way production code should.

## Next steps

- **Decode into your own struct in more detail**: see
  [Decode answers into your own struct](../how-to/decode-answers-into-your-own-struct.md).
- **See every client and request option**: the
  [client options and environment reference](../reference/client-options-and-environment.md)
  covers timeouts, retries, custom HTTP clients, and instrumentation.
- **See every question field and wire shape**: the
  [question types reference](../reference/question-types.md) covers `Noul`,
  `Choice`, `Score`, and the escape-hatch `RawQuestion`.
- **See every response field**: the
  [response types reference](../reference/response-types.md) covers
  `SystemOneResponse`, `Usage`, and `ListModels`.
