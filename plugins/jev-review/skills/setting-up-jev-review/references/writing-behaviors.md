# Writing behaviors

How to fill a unit's `behaviors` list.

## Rules

- One behavior per requirement sentence in the section. Five to fifteen per unit.
- Each names one checkable outcome a single test could assert: a count, an error, a value, an order, an absence.
- Write it as a declarative about the code, in the spec's own terms.
- Under five: the section is thin. Add requirement sentences to the spec first, or merge the unit with a sibling.
- Over fifteen: split the section into sub-headings and give each its own unit.
- Keep the text stable. `accepted` entries name a behavior by its exact text; editing the text drops the acceptance.
- Behaviors map to `covers_NN` by position. Inserting or reordering an entry shifts every id after it, and notes or report ids that name a `covers_NN` stop pointing at the same claim. Append a new behavior at the end.

## Good

- "A failed publish is retried up to three times": a test can assert the call count.
- "An empty widget name is rejected before any publish attempt is made": a test can assert an error and zero calls.
- "retry-after-ms takes precedence over Retry-After": a test can set both headers and assert the wait.

## Bad

- "Retries work correctly": nothing to assert.
- "The code handles errors well": no outcome named.
- "Validation": a topic, not a claim.

## Deriving behaviors from a section

Section:

```markdown
### Retry policy

Retries are attempted up to MaxRetries (default 2) on 408, 429, and 5xx. A 422 is never retried. Retry-After is honored up to MaxRetryAfter (default 5m).
```

Behaviors:

```json
[
  "A 408, 429, or 5xx response is retried",
  "Retries stop after MaxRetries and the last error is returned",
  "MaxRetries defaults to 2 when unset",
  "A 422 response is not retried",
  "A Retry-After header sets the delay before the next attempt",
  "A Retry-After longer than MaxRetryAfter is capped at MaxRetryAfter"
]
```

## Acceptances

- After a full run, a flag reviewed and found to be noise goes into `accepted`, with a note in `notes` recording the readings and the review.
- An acceptance is the behavior's exact text, or `contradicts_spec`, `thoroughness`, or `weakest_area`. Never `covers_NN`; the loader rejects it.
- `acceptance did not fire: <name>` in a report means the acceptance is stale. Remove it.

Source: https://therealbill.github.io/typesafe-go/docs/how-to/review-your-codebase-with-jev/
