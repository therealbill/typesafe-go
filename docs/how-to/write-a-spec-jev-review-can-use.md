---
title: "How to Write a Spec jev review Can Use"
description: "Structure and word a Markdown spec section so jev review can extract it and judge it precisely, with a worked before/after and a real comparison run."
diataxis: how-to
weight: 105
---

`jev review` judges whether your tests exercise the behaviors you listed and whether your implementation contradicts the spec section you named. It sends those files to Jev and answers with probabilities. It does not find bugs, review style, check security, or verify that the code is correct, and its readings drift between runs on the same input.

This page is about the spec document itself: how to structure and word it so `jev review` can extract the right section and judge it precisely. The `jev-review.json` config and the tool's flags, thresholds, and output are covered in [Review your codebase with jev](./review-your-codebase-with-jev.md).

**Goal**: write a spec section that `jev review` extracts cleanly and can judge for coverage and contradiction.

## Steps

### 1. Give every unit-sized concern its own heading

`jev review` extracts a section by finding the line that matches a unit's `spec_heading` exactly, then reading from there through the line before the next heading of the same or higher level, ignoring any heading that appears inside a fenced code block. `spec_heading` must match the heading line in the spec file character for character, including the `#` characters. A heading a level too deep or too shallow, or a stray trailing space, misses the match and the unit fails to load.

A good tree for a small library gives each independent concern its own heading, at a level that groups related sub-concerns underneath it:

```markdown
## Client

### Retry policy

Retries are attempted up to MaxRetries...

### Timeouts

The default per-attempt timeout is...

## Cache

### Eviction

The least recently used entry is evicted first...
```

`spec_heading: "### Retry policy"` extracts only the retry section: from that line through the line before `### Timeouts`, the next heading at the same level. `spec_heading: "## Client"` would instead extract retry policy and timeouts together, since both are below it.

A bad tree puts everything under one heading, or splits one requirement across headings at different levels:

```markdown
## Client

Retries are attempted up to MaxRetries. The default per-attempt timeout is
30s. The cache evicts the least recently used entry first.

### Retry policy

MaxRetries defaults to 2.
```

Here `## Client` bundles retry, timeout, and cache text together with nothing to separate them, and `### Retry policy` repeats part of the same requirement one level down. Neither heading names a section a unit can read on its own.

### 2. State checkable facts

The `contradicts_spec` question counts only actual conflicts: a different default value, a different limit, a different error type, a different field name, a different ordering, or opposite failure behavior. It ignores omissions and anything the spec leaves unspecified. A sentence with nothing checkable in it cannot be contradicted, so it cannot be flagged, and it also gives the `covers_NN` questions nothing specific to test against.

Vague, before:

```markdown
The client retries transient failures sensibly.
```

Concrete, after:

```markdown
Retries are attempted up to MaxRetries (default 2) on 408, 429, and 5xx. A
422 is never retried. Retry-After is honored up to MaxRetryAfter (default
5m).
```

The second version names a default, a status-code list, an explicit exclusion, and a second default with its own cap. Each clause is a fact the implementation either matches or contradicts. Write plain declaratives. Drop "should," "sensibly," "appropriately," and other words that describe an impression rather than a fact. Skip marketing language; it has no truth value to check.

### 3. Keep requirements in prose, not only in code blocks

`extractSection` copies the section's text as-is, fenced code blocks included, but a sentence living only inside a fenced block reads as an illustration, not a requirement, to anyone judging the section, human or not. Code samples still belong in a spec section to show shape or usage. State the requirement itself in a sentence outside the fence, and use the fenced block only to illustrate it:

```markdown
A `Config` with a zero `MaxRetries` falls back to the default of 2.

```go
cfg := Config{} // MaxRetries defaults to 2
```
```

The first line is what gets judged for coverage and contradiction. The code block supports it.

### 4. Size sections for the state budget

`jev review` bundles a unit's `implementation` and `tests` files together, and by default that bundle is capped at 100 KB total, half for implementation and half for tests, with the report's `truncated` field set to `true` when a bundle is cut. A section's behaviors are judged against exactly the files a unit lists, so a section should match what one unit's files actually implement. When a section covers more code than fits comfortably in that budget, split it with sub-headings, and give each sub-heading its own unit with its own narrower file list.

### 5. Build one from nothing, three routes

**From existing package READMEs or design docs.** Read what's already written and rewrite each section's claims as declaratives, following step 2. A README that says a function "handles errors gracefully" becomes a sentence naming which errors and what happens to them.

**From the code and tests.** Read what the code does now, write it down as declaratives, and mark anything that looks wrong as you go. Run `jev review` afterward and see whether it agrees; a `contradicts_spec` flag on a section you wrote directly from the code usually means you wrote down what the code does today, not what it's supposed to do, and the two differ somewhere you didn't notice while reading.

**From a design spec written before the code.** This is the route where the contradiction question earns its keep: the spec was never derived from the implementation, so a conflict it finds is a real candidate for either a bug or a stale requirement, not a restatement of what you already knew.

### 6. Keep spec and config in step

Each requirement sentence in a section should map to one behavior line in the unit that reviews it. When a spec sentence changes, for example the default in step 2 changing from 2 to 3, change the matching behavior text and re-run that unit. A spec and a behaviors list that drift apart stop measuring the same thing: the spec says one thing, the behaviors ask about another, and the readings for both grow harder to interpret.

## Verify it works

The vague and concrete sections above were tested against a small retry helper, `retrydemo`, and its one test, which exercises only the success path (`Do` returning on the first call):

```go
// Package retry provides a minimal retry helper for transient failures.
package retry

// DefaultMaxRetries is used when Config.MaxRetries is zero or negative.
const DefaultMaxRetries = 2
```

The concrete spec deliberately states a default of 3 to plant a mismatch against the code's actual default of 2. One `jev-review.json` config named both units, identical except for `spec`, the per-unit override described in [Review your codebase with jev](./review-your-codebase-with-jev.md#3-pair-each-section-with-the-files-that-implement-it):

```json
{
  "units": [
    {
      "name": "vague",
      "spec": "spec-vague.md",
      "spec_heading": "## Retry",
      "implementation": ["retry.go"],
      "tests": ["retry_test.go"],
      "behaviors": [
        "A failed call is retried before Do gives up",
        "Do waits Delay between retry attempts",
        "Do returns an error when every attempt fails",
        "A canceled context stops the retry loop"
      ]
    },
    {
      "name": "concrete",
      "spec": "spec-concrete.md",
      "spec_heading": "## Retry",
      "implementation": ["retry.go"],
      "tests": ["retry_test.go"],
      "behaviors": ["... same four behaviors ..."]
    }
  ]
}
```

A real run of `jev review` against that config produced:

| Unit | Behaviors covered | Contradicts | Thoroughness | Weakest | Flags |
|---|---|---|---|---|---|
| vague | 0/4 | 0.07 | 1.00 (0.99) | retry | 5 FAIL |
| concrete | 0/4 | 0.97 | 1.00 (1.00) | retry | 6 FAIL |

The concrete section's `contradicts_spec` reads 0.97, pointing straight at the planted mismatch between the spec's stated default of 3 and the code's actual default of 2. The vague section's `contradicts_spec` reads 0.07: there was nothing concrete in that section for the implementation to conflict with, so the question has almost nothing to flag. Both sections' `covers_NN` readings come back low across all four behaviors, correctly: the test exercises only the success path, so none of the retry, delay, failure, or cancellation behaviors are actually tested, in either unit. The coverage readings are doing real work here, separating a genuinely untested behavior from a tested one; the contradiction reading is doing the opposite job, and it only has something to find when the spec states something specific enough to be wrong.

## See also

- [Review your codebase with jev](./review-your-codebase-with-jev.md) for the `jev-review.json` config, flags, and how to respond to a flag.
- [jev review reference](../reference/jev-review.md) for the full config schema and the section-extraction rule.
- [What jev review measures](../explanation/what-jev-review-measures.md) for what each question type can and cannot show.
- [Build a jev review config with Claude Code](./build-a-jev-review-config-with-claude-code.md) for having the `jev-review` plugin write the spec and config.
