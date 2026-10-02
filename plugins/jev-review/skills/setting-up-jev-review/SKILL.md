---
name: setting-up-jev-review
description: Use when a request mentions jev review, jev-review.json, spec_heading, writing behaviors for a review unit, writing or fixing a Markdown spec for jev review, or reviewing a codebase with Jev. Also use when a run flagged a unit at 0 of N, contradicts_spec, thoroughness, weakest_area, or covers_NN, or when triaging an acceptance did not fire message or jev-review-report.json. Covers the spec rules, file pairing, behavior writing, the config schema, the dry run, and triage after a run.
---

# Setting up jev review

`jev review` judges whether your tests exercise the behaviors you listed and whether your implementation contradicts the spec section you named. It sends those files to Jev and answers with probabilities. It does not find bugs, review style, check security, or verify that the code is correct, and its readings drift between runs on the same input.

Two files drive it. The spec is a Markdown document with one heading per concern and checkable sentences under each. `jev-review.json` names the spec, pairs each heading with implementation and test files, and lists the behaviors to check.

## Procedure

Setting up a review takes the steps below. Triaging a run that already happened starts at "After a real run".

`/jev-review:setup` runs the full workflow. When the user asks in their own words, follow the same steps:

1. Confirm `jev` is on `PATH` and `jev review --help` lists `--dry-run`. Without it, give the install routes from `references/config-schema.md` and stop.
2. Find the spec, or decide to write one. Read `references/writing-the-spec.md`.
3. Map each spec section to the files that implement it and the files that test it. Read `references/pairing-files.md` and `references/language-conventions.md`. For a repository with more than a handful of files, dispatch the `jev-review:repo-mapper` agent and use its proposal.
4. Show the proposed unit map and get approval before writing anything.
5. Write or edit the spec. Add headings and requirement sentences only for approved units. Edit an existing spec in place; never reorder or rewrite it.
6. Write `jev-review.json` in the directory `jev review` will run from, or pass `--units` on every run. Read `references/config-schema.md` and `references/writing-behaviors.md`.
7. Run `jev review --dry-run --units <path>` and fix errors until it exits 0.
8. Hand off: list the files written, the sentences written from code, and the `jev review` command. Do not run the paid review.

## Which reference to open

| Question | Reference |
|---|---|
| How does `jev review` find a section? What makes a sentence checkable? | `references/writing-the-spec.md` |
| Which files go in `implementation` and `tests`? What does 0 of N mean? | `references/pairing-files.md` |
| How do I phrase a behavior? How many? What is an acceptance? | `references/writing-behaviors.md` |
| What are the config fields, path rules, flags, and exit codes? | `references/config-schema.md` |
| Where do tests live in Go, Python, TypeScript, Rust? | `references/language-conventions.md` |

## After a real run

For each flag, choose one response:

- Add a test when the behavior is untested.
- Fix the spec or the code when they disagree.
- Record an acceptance when the flag was reviewed and is noise: add the behavior's exact text, or `contradicts_spec`, `thoroughness`, or `weakest_area`, to the unit's `accepted`, and a note with the readings to `notes`. Never `covers_NN`.

A unit at 0 of N with a real test file present is a mis-pairing. Fix the pairing before adding tests. `acceptance did not fire: <name>` means the acceptance is stale; remove it. Use `--only NAME` while working on one unit.

## Rules

- Never run `jev review` without `--dry-run` unless the user asks. A full run sends the spec section and every listed file to the TypeSafe API and costs money.
- Never print `TYPESAFE_API_KEY`.
- Never overwrite an existing `jev-review.json`. Report the path and get approval first. `jev review init` refuses to overwrite one and exits 1.
- Paths in the config resolve relative to the config file, not the working directory.
- `spec_heading` matches one heading line exactly, `#` characters included.
