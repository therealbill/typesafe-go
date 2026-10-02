# Pairing spec sections with files

How to choose a unit's `implementation` and `tests`. Commands per language are in `language-conventions.md`.

## Procedure

1. Read the section. List every function, type, method, option, constant, and field it names.
2. Find the non-test files that define those symbols. Those are `implementation`.
3. Find the test files that reference those symbols and assert on their behavior. Those are `tests`.
   When no test file asserts on the section's symbols, leave `tests` empty. An empty list reads as untested, which is the accurate reading. A guessed file name produces a mis-pairing that hides as a coverage result.
4. When a section's API spans several files, list them all.
5. Record the evidence: which test file references which symbol.

## Rules

- A similar file name is not evidence. `tracker.go` paired with `exporter_test.go` reads as untested even when the tracker is tested elsewhere.
- The exporter for a feature is not the feature. Pair the feature with its own tests.
- A test that mentions a symbol without asserting on it does not count.
- Tests that exercise a symbol from another file belong in `tests` too. In the typesafe-go repository the `retry` unit lists `client_test.go` because the retry budget and cancellation are exercised there.
- Rust unit tests live in the implementation file under `#[cfg(test)]`. List that file under both `implementation` and `tests`.

## Budget

- `--max-state-bytes` defaults to 100000. Implementation gets half, tests get half.
- Sizes count bytes before truncation. Past 50000 on either side, split the section and the unit.
- `jev review --dry-run` prints each unit's sizes and whether it was truncated.

## Reading a bad pairing after a full run

- A unit at 0 of N behaviors with a real test file present is a mis-pairing. The tests exist and the unit is not reading them. Fix the pairing first.
- A unit whose `thoroughness` jumps when a test file is added was missing that file.
- Use `--only NAME` while fixing one unit.

Sources:

- https://therealbill.github.io/typesafe-go/docs/how-to/review-your-codebase-with-jev/
- https://therealbill.github.io/typesafe-go/docs/explanation/what-jev-review-measures/
