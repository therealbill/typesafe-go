---
title: "typesafe-go"
layout: hextra-home
---

{{< hextra/hero-badge link="https://github.com/therealbill/typesafe-go/releases" >}}
  Releases
{{< /hextra/hero-badge >}}

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  Typed judgments from Jev, in Go
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  A stdlib-only Go client for the TypeSafe System One API, plus the `jev` command-line tool for scripts and agents.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="Get started" link="docs/tutorials/first-judgment-in-go" >}}
</div>

```bash
go get github.com/therealbill/typesafe-go
```

```go
client, err := typesafe.NewClient() // reads TYPESAFE_API_KEY
res, err := client.SystemOne(ctx, "I was charged twice.", typesafe.Questions{
    "billing": typesafe.Noul{Instructions: "Is this about billing?"},
    "tone":    typesafe.Choice{Instructions: "What is the tone?", Criteria: map[string]typesafe.JSONContent{"calm": nil, "angry": nil}},
})
fmt.Println(res.Nouls()["billing"].Noul, res.Choices()["tone"].Choice)
```

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="Tutorials"
    subtitle="Build something that works, step by step, with a checkpoint after each one."
    link="docs/tutorials"
  >}}
  {{< hextra/feature-card
    title="How-to guides"
    subtitle="Recipes for retries, typed decoding, gateways, tracing, and driving jev from scripts."
    link="docs/how-to"
  >}}
  {{< hextra/feature-card
    title="Reference"
    subtitle="Every option, type, flag, span attribute, and exit code, generated from the source."
    link="docs/reference"
  >}}
  {{< hextra/feature-card
    title="Explanation"
    subtitle="Why the core is stdlib-only, how retries and budgets interact, what is and is not traced."
    link="docs/explanation"
  >}}
{{< /hextra/feature-grid >}}
