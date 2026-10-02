# Writing a spec jev review can use

How `jev review` reads a spec and how to write one it can judge.

## How a section is extracted

- A unit's `spec_heading` must match one heading line character for character, `#` characters included: `"## Retry policy"`, not `"Retry policy"`. Whitespace around the heading line in the spec file is ignored. The config value is not trimmed, so `spec_heading` carries no leading or trailing space.
- The section runs from that line through the line before the next heading of the same or higher level. Headings inside fenced code blocks are ignored.
- The first line that matches wins. Two headings with the same text resolve to the first one and the unit reviews the wrong section with no error. Give every heading text that appears once in the file.
- The last section of a file runs to the end of the file. A line closes a section only when its `#` characters start at column 0 and are followed by a space, so an indented heading is swallowed. Only backtick fences are tracked, so a heading line inside a `~~~` block closes the section early.
- A heading one level too deep or too shallow misses the match and the unit errors.

## Structure

- One heading per unit-sized concern. Put sub-concerns under it at the next level.
- Keep sibling headings at the same level. Do not repeat part of a requirement one level down.
- Size a section to what one unit's files implement. Past about 50000 bytes of implementation or of tests, split the section with sub-headings and give each its own unit.

A good tree:

```markdown
## Client

### Retry policy

Retries are attempted up to MaxRetries (default 2) on 408, 429, and 5xx.

### Timeouts

The default per-attempt timeout is 10s.

## Cache

### Eviction

The least recently used entry is evicted first.
```

`spec_heading: "### Retry policy"` extracts the retry section alone. `"## Client"` extracts retry policy and timeouts together.

## Sentences

- Write plain declaratives. Each sentence states a fact the code either matches or contradicts: a default, a limit, a status-code list, an error type, a field name, an ordering, a failure behavior.
- Drop "should", "sensibly", "appropriately", "gracefully", "correctly", and every other word that describes an impression.
- Leave out marketing language and design rationale. They have no truth value to check.
- Keep each requirement in prose. A fenced code block illustrates a requirement; it does not state one.

Vague: "The client retries transient failures sensibly."

Concrete: "Retries are attempted up to MaxRetries (default 2) on 408, 429, and 5xx. A 422 is never retried. Retry-After is honored up to MaxRetryAfter (default 5m)."

## Three routes to a spec

- From READMEs or design docs: rewrite each claim as a declarative. "Handles errors gracefully" becomes which errors and what happens to them.
- From code and tests: write down what the code does as declaratives and mark anything that looks wrong. After a full run, a `contradicts_spec` flag on such a section means the sentence describes what the code does today and that differs from what it should do.
- From a design spec written before the code: a contradiction found here is a real candidate for a bug or a stale requirement.

## New-spec skeleton

```markdown
# <Project> specification

## <Concern>

<One declarative sentence per requirement.>

## <Concern>

<One declarative sentence per requirement.>
```

## Editing an existing spec

- Add a missing heading at the level of its siblings, at the end of the nearest parent section.
- Reword a vague sentence in place. Do not move, reorder, or delete existing text.
- Leave a heading with no code behind it alone, and do not make a unit for it.

## Keep spec and config in step

Each requirement sentence maps to one behavior line in the unit that reviews it. When a sentence changes, change the behavior and rerun that unit with `--only`.

Source: https://therealbill.github.io/typesafe-go/docs/how-to/write-a-spec-jev-review-can-use/
