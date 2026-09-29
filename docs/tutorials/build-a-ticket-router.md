---
title: "Build a Ticket Router in Go"
description: "Ask four questions about one support ticket in a single call and turn a probability, a choice, a score, and a second probability into a routing decision."
diataxis: tutorial
weight: 30
---

A single `SystemOne` call can carry more than one question. In this tutorial
you'll send one support ticket to TypeSafe and ask four independent questions
about it at once, then combine the four answers into a routing decision a
real support queue could act on.

## What you'll build

A command-line Go program that:

1. Sends a support ticket's text to TypeSafe as `state`.
2. Asks four questions about it in a single call: is this about billing (a
   probability), which department should handle it (a choice with a label
   that means "none of the above"), how urgent is it (a three-level score),
   and would a human need to review it before replying (a second,
   speculative probability).
3. Combines the four answers into one routing decision: a specific
   department, or a human queue when the ticket is too urgent, too sensitive,
   or too ambiguous to route automatically.
4. Runs that routing logic against more than one ticket, so you can see it
   take different paths.

Every command below is a command you run. The checkpoints show output
captured from the live API. TypeSafe's judgments come from a model, not a
lookup table, so your probabilities, confidence values, and scores will land
close to the values shown here and will rarely match them to the decimal.

## Prerequisites

- Go 1.25 or newer installed
- A TypeSafe API key

This tutorial assumes you're comfortable with the basics: sending `state`,
defining questions, and reading typed answers back with `Nouls()`,
`Choices()`, and `Scores()`. If any of that is unfamiliar, the
[question types](../reference/question-types.md) and
[response types](../reference/response-types.md) references cover it.
Nothing here requires having followed another tutorial first.

## Step 1: Set up a module and install the library

Create a new directory and initialize a Go module:

```bash
mkdir ticket-router
cd ticket-router
go mod init ticket-router
```

Install the library:

```bash
go get github.com/therealbill/typesafe-go@latest
```

Export your API key so `typesafe.NewClient()` can find it:

```bash
export TYPESAFE_API_KEY=<your API key>
```

### Checkpoint

Open `go.mod`. You should see a `require` line for
`github.com/therealbill/typesafe-go`, and a `go.sum` file should now exist
alongside it.

## Step 2: Ask four questions about a ticket

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
		"ticket": "My card was charged twice for the same subscription this month. Can you refund the duplicate charge?",
	}

	questions := typesafe.Questions{
		"billing": typesafe.Noul{
			Instructions: "Is `ticket` about a billing problem?",
		},
		"department": typesafe.Choice{
			Instructions: "Which department should handle `ticket`?",
			Criteria: map[string]typesafe.JSONContent{
				"billing":   "Charges, invoices, refunds, or payment methods",
				"technical": "Something in the product is broken or not working as expected",
				"account":   "Login, account access, or account settings",
				"none":      nil,
			},
		},
		"urgency": typesafe.Score{
			Instructions: "How urgent is `ticket`?",
			Criteria: []typesafe.JSONContent{
				"Not urgent",
				"Somewhat urgent",
				"Very urgent",
			},
		},
		"needs_human": typesafe.Noul{
			Instructions: "Would a human need to review `ticket` before replying, rather than an automated response?",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := client.SystemOne(ctx, state, questions)
	if err != nil {
		log.Fatalf("SystemOne call failed: %v", err)
	}

	nouls := res.Nouls()
	choices := res.Choices()
	scores := res.Scores()

	fmt.Printf("billing:     %.2f\n", nouls["billing"].Noul)
	fmt.Printf("department:  %s (confidence %.2f)\n", choices["department"].Choice, choices["department"].Confidence)
	fmt.Printf("urgency:     %.2f\n", scores["urgency"].Score)
	fmt.Printf("needs_human: %.2f\n", nouls["needs_human"].Noul)
}
```

Before you run it:

- All four questions go out in one `Questions` map, so this is one HTTP call
  to `/v1/systemone`, not four. The identifiers (`"billing"`, `"department"`,
  `"urgency"`, `"needs_human"`) are yours to choose. They're how you find
  each answer afterward, and the model never sees them.
- `department` is a `Choice` with four labels, and one of them, `"none"`, maps
  to `nil` instead of a description. A `nil` value in `Choice.Criteria` leaves
  a label undescribed; it does not remove the label. The model can still pick
  `none`, but is not told what `none` means beyond its name. `none` gives the
  model a label for a ticket that fits no real department, instead of forcing
  the ticket onto `billing`, `technical`, or `account`.
- `needs_human` is a second `Noul`, independent of `billing`. A single call
  can ask more than one probability question; you're not limited to one
  judgment per question type.

Run it:

```bash
go run main.go
```

### Checkpoint

You should see output like this (an actual run produced exactly this):

```
billing:     0.99
department:  billing (confidence 1.00)
urgency:     0.97
needs_human: 0.58
```

One call returned four independent answers. `billing` is 0.99, so the model
reads the ticket as almost certainly a billing problem. It picked the
`billing` department with full confidence. `urgency` sits low on the 0–2
rubric. `needs_human` landed at 0.58, near the middle of the range, which is
neither a clear yes nor a clear no.

## Step 3: Turn the answers into a routing decision

The four answers combine into a single routing decision. Replace the body of
`main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	typesafe "github.com/therealbill/typesafe-go"
)

const (
	needsHumanThreshold  = 0.6
	urgencyThreshold     = 1.5
	departmentConfidence = 0.6
)

func main() {
	client, err := typesafe.NewClient()
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	state := map[string]any{
		"ticket": "My card was charged twice for the same subscription this month. Can you refund the duplicate charge?",
	}

	questions := typesafe.Questions{
		"billing": typesafe.Noul{
			Instructions: "Is `ticket` about a billing problem?",
		},
		"department": typesafe.Choice{
			Instructions: "Which department should handle `ticket`?",
			Criteria: map[string]typesafe.JSONContent{
				"billing":   "Charges, invoices, refunds, or payment methods",
				"technical": "Something in the product is broken or not working as expected",
				"account":   "Login, account access, or account settings",
				"none":      nil,
			},
		},
		"urgency": typesafe.Score{
			Instructions: "How urgent is `ticket`?",
			Criteria: []typesafe.JSONContent{
				"Not urgent",
				"Somewhat urgent",
				"Very urgent",
			},
		},
		"needs_human": typesafe.Noul{
			Instructions: "Would a human need to review `ticket` before replying, rather than an automated response?",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := client.SystemOne(ctx, state, questions)
	if err != nil {
		log.Fatalf("SystemOne call failed: %v", err)
	}

	nouls := res.Nouls()
	choices := res.Choices()
	scores := res.Scores()

	needsHuman := nouls["needs_human"].Noul
	urgency := scores["urgency"].Score
	department := choices["department"]

	fmt.Printf("billing:     %.2f\n", nouls["billing"].Noul)
	fmt.Printf("department:  %s (confidence %.2f)\n", department.Choice, department.Confidence)
	fmt.Printf("urgency:     %.2f\n", urgency)
	fmt.Printf("needs_human: %.2f\n", needsHuman)
	fmt.Printf("route:       %s\n", route(needsHuman, urgency, department))
}

// route decides where a ticket goes next, given its needs_human probability,
// its urgency score, and its department choice.
func route(needsHuman, urgency float64, department typesafe.ChoiceAnswer) string {
	if needsHuman > needsHumanThreshold || urgency > urgencyThreshold {
		return "human queue (needs a human, or too urgent to risk)"
	}
	if department.Confidence < departmentConfidence {
		return "human queue (department pick too uncertain)"
	}
	return department.Choice
}
```

`route` checks three things, in order:

1. If `needs_human` is above `0.6`, or `urgency` is above `1.5` (past
   "somewhat urgent" and toward "very urgent" on the 0–2 rubric), the ticket
   goes to a human queue. The first condition covers a ticket the model
   flagged for review; the second covers one urgent enough that an automated
   reply carries risk.
2. Otherwise, if the `department` choice's `Confidence` is below `0.6`, the
   ticket still goes to the human queue. A department pick the model is not
   confident about does not route straight to that team.
3. If neither condition holds, the ticket goes to `department.Choice`
   directly.

Run it:

```bash
go run main.go
```

### Checkpoint

An actual run produced:

```
billing:     0.99
department:  billing (confidence 1.00)
urgency:     0.96
needs_human: 0.58
route:       billing
```

This was a fresh call, which is why `urgency` reads `0.96` here instead of
Step 2's `0.97`. Each call is an independent judgment, as noted above. All
three of `route`'s checks pass this ticket through: `needs_human` (`0.58`) is
under `0.6`, `urgency` (`0.96`) is under `1.5`, and `department`'s confidence
(`1.00`) is comfortably over `0.6`, so it lands on `billing`.

## Step 4: Route more than one ticket

Three tickets take three different paths through `route`. Replace `main.go`
once more to loop over a clear billing complaint, an angry and urgent one,
and a calm but ambiguous one.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	typesafe "github.com/therealbill/typesafe-go"
)

const (
	needsHumanThreshold  = 0.6
	urgencyThreshold     = 1.5
	departmentConfidence = 0.6
)

var tickets = []string{
	"My card was charged twice for the same subscription this month. Can you refund the duplicate charge?",
	"This is absolutely unacceptable. Your app deleted three days of my work and nobody from support has responded in 48 hours. Fix this immediately or I'm canceling my account and telling everyone how bad this is.",
	"I want to switch my card on file and update my profile settings, whenever that's convenient.",
}

func main() {
	client, err := typesafe.NewClient()
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	for _, ticket := range tickets {
		if err := handle(client, ticket); err != nil {
			log.Fatalf("handle ticket: %v", err)
		}
		fmt.Println()
	}
}

func handle(client *typesafe.Client, ticket string) error {
	state := map[string]any{"ticket": ticket}

	questions := typesafe.Questions{
		"billing": typesafe.Noul{
			Instructions: "Is `ticket` about a billing problem?",
		},
		"department": typesafe.Choice{
			Instructions: "Which department should handle `ticket`?",
			Criteria: map[string]typesafe.JSONContent{
				"billing":   "Charges, invoices, refunds, or payment methods",
				"technical": "Something in the product is broken or not working as expected",
				"account":   "Login, account access, or account settings",
				"none":      nil,
			},
		},
		"urgency": typesafe.Score{
			Instructions: "How urgent is `ticket`?",
			Criteria: []typesafe.JSONContent{
				"Not urgent",
				"Somewhat urgent",
				"Very urgent",
			},
		},
		"needs_human": typesafe.Noul{
			Instructions: "Would a human need to review `ticket` before replying, rather than an automated response?",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := client.SystemOne(ctx, state, questions)
	if err != nil {
		return err
	}

	nouls := res.Nouls()
	choices := res.Choices()
	scores := res.Scores()

	needsHuman := nouls["needs_human"].Noul
	urgency := scores["urgency"].Score
	department := choices["department"]

	fmt.Printf("ticket:      %s\n", ticket)
	fmt.Printf("billing:     %.2f\n", nouls["billing"].Noul)
	fmt.Printf("department:  %s (confidence %.2f)\n", department.Choice, department.Confidence)
	fmt.Printf("urgency:     %.2f\n", urgency)
	fmt.Printf("needs_human: %.2f\n", needsHuman)
	fmt.Printf("route:       %s\n", route(needsHuman, urgency, department))
	return nil
}

// route decides where a ticket goes next, given its needs_human probability,
// its urgency score, and its department choice.
func route(needsHuman, urgency float64, department typesafe.ChoiceAnswer) string {
	if needsHuman > needsHumanThreshold || urgency > urgencyThreshold {
		return "human queue (needs a human, or too urgent to risk)"
	}
	if department.Confidence < departmentConfidence {
		return "human queue (department pick too uncertain)"
	}
	return department.Choice
}
```

`handle` is Step 3's logic pulled into its own function so `main` can call it
once per ticket. Nothing about `route` changed.

Run it:

```bash
go run main.go
```

### Checkpoint

An actual run produced:

```
ticket:      My card was charged twice for the same subscription this month. Can you refund the duplicate charge?
billing:     0.99
department:  billing (confidence 1.00)
urgency:     0.97
needs_human: 0.56
route:       billing

ticket:      This is absolutely unacceptable. Your app deleted three days of my work and nobody from support has responded in 48 hours. Fix this immediately or I'm canceling my account and telling everyone how bad this is.
billing:     0.06
department:  technical (confidence 0.99)
urgency:     2.00
needs_human: 0.88
route:       human queue (needs a human, or too urgent to risk)

ticket:      I want to switch my card on file and update my profile settings, whenever that's convenient.
billing:     0.31
department:  account (confidence 0.58)
urgency:     0.00
needs_human: 0.38
route:       human queue (department pick too uncertain)
```

The three tickets take different paths through `route`:

- The first ticket repeats Step 3's result: low `needs_human`, low `urgency`,
  high department confidence, so it routes straight to `billing`.
- The second ticket trips the first check on both of its conditions.
  `needs_human` (`0.88`) is above `0.6`, and `urgency` (`2.00`) is pinned at
  the top of the rubric. Either condition alone sends a ticket to the human
  queue.
- The third ticket is calm and not urgent: `urgency` is `0.00` and
  `needs_human` is `0.38`, both comfortably under threshold. It mentions a
  payment method and a profile setting, so the model split its confidence
  across `account` and `billing`. `department`'s confidence (`0.58`) falls
  under `0.6`, so the third check sends the ticket to the human queue on a
  low-confidence department pick alone.

In the third case, `department.Choice` still holds a value (`account`) even
though `route` didn't use it. A `Choice` answer always names a label.
Checking `Confidence` alongside it tells you whether that label is worth
acting on.

## What you built

You now have a Go program that asks four independent questions about a
support ticket in a single call (a probability, a choice with an
explicit "none of the above" label, an urgency rubric, and a second,
speculative probability) and combines all four into a routing decision,
including the case where the right department can't be picked with enough
confidence to trust automatically.

## Next steps

- **Act on the numbers behind the labels**: see
  [Act on probabilities and confidence](../how-to/act-on-probabilities-and-confidence.md)
  for a task-focused look at thresholds, confidence, and probability
  distributions, including TypeSafe's own guidance on interpreting
  confidence, at [docs.typesafe.ai/confidence](https://docs.typesafe.ai/confidence).
- **Decode answers into your own struct**: see
  [Decode answers into your own struct](../how-to/decode-answers-into-your-own-struct.md).
- **See every question field and wire shape**: the
  [question types reference](../reference/question-types.md) covers `Noul`,
  `Choice`, `Score`, and the escape-hatch `RawQuestion`.
- **See every response field**: the
  [response types reference](../reference/response-types.md) covers
  `SystemOneResponse`, `ChoiceAnswer.Probabilities`, and `ScoreAnswer.Legend`.
