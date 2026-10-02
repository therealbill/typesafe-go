---
name: setup
description: Write the spec and jev-review.json that jev review needs, from this repository, with one stop to approve the unit map
argument-hint: "[spec-path] [config-path]"
---

# /jev-review:setup

Write the two files `jev review` needs for this repository: a Markdown spec with one heading per concern, and `jev-review.json`. Stop once, to get the unit map approved. Never run the paid review.

Arguments: `$1` is the spec path, optional. `$2` is the config path, optional; the default is `jev-review.json` in the current directory, which is where `jev review` looks.

Read `${CLAUDE_PLUGIN_ROOT}/skills/setting-up-jev-review/SKILL.md` first. The references it names live in `${CLAUDE_PLUGIN_ROOT}/skills/setting-up-jev-review/references/`. Then follow these steps in order.

## 1. Preconditions

Run:

```bash
command -v jev || echo "jev: missing"
jev review --help 2>/dev/null | grep -c -- '--dry-run'
test -n "${TYPESAFE_API_KEY:-}" && echo "key: set" || echo "key: unset"
```

- `jev: missing`: print the install routes from `references/config-schema.md` and stop.
- The count is 0: say the installed `jev` predates `--dry-run`, name the release that ships it (`references/config-schema.md`), and stop.
- Record whether the key is set. It changes only the hand-off text. Never print its value.
- If `$2` is empty and `jev-review.json` exists in the current directory, stop and say: "jev-review.json already exists; pass its path as the second argument to overwrite it, or move it aside." A path given as `$2` may be overwritten.

## 2. Inventory

Resolve the spec:

- `$1` given: that file. Stop if it does not exist.
- Otherwise collect candidates matching `docs/**/*design*.md`, `docs/spec.md`, `SPEC.md`, and `docs/**/*spec*.md`, excluding anything under `docs/superpowers/plans/`.
- One candidate: the spec. Several: pass them all to the mapper. None: new-spec mode, and the spec will be written at `docs/spec.md` when `docs/` exists, otherwise `SPEC.md`.

Set `mode` to `existing` or `new`.

## 3. Map

Dispatch the `jev-review:repo-mapper` agent with this prompt, filled in:

```
root: <absolute repository root>
spec: <none | one path | comma-separated candidate paths>
mode: <new | existing>
Return the JSON block described in your instructions.
```

Wait for it and parse the JSON block from its reply.

## 4. Checkpoint

Render one table from the JSON, then ask with `AskUserQuestion`. This is the only stop.

```
Spec: docs/design.md (existing)

| Unit | Heading | Implementation | Tests | Bytes | Evidence |
|---|---|---|---|---|---|
| retry | ## Retry policy (existing) | retry.go, transport.go | retry_test.go, client_test.go | 11520/34110 | client_test.go asserts on retry-after-ms and the budget |
```

`Bytes` is implementation/tests. Mark a side over 50000 with `!` and say the unit should be split. In new mode the spec line reads `Spec: docs/spec.md (new)` and every heading is marked `(new)`. List `other_candidates`, `unmatched_headings`, and `vague_sentences` under the table when they are not empty.

Question: "Approve this unit map?" Options: "Approve as is" and "Describe changes". On changes, apply them to the map (rename, drop, merge, add or remove files, change a heading, pick another spec candidate), re-render, and ask again. Loop until approved.

## 5. Spec

Read `references/writing-the-spec.md`.

- `new` mode: write the spec at the path from step 2, using the skeleton. One `## <Concern>` per approved unit, in map order. Under each, declarative requirement sentences derived from the implementation and its tests: defaults, limits, error types, field names, orderings, failure behavior. Five to fifteen sentences per section. Record each sentence with the file and line it came from, for the hand-off.
- `existing` mode: for each approved unit whose heading does not exist, add the heading at the level of its siblings, at the end of the nearest parent section, with requirement sentences as above. For each `vague_sentences` entry inside an approved unit's section, reword in place to state the fact the code shows, and record the before and after for the hand-off. Do not move, reorder, or delete anything else.

## 6. Config

Read `references/config-schema.md` and `references/writing-behaviors.md`. Write `$2`, or `jev-review.json`:

- `description`: "Units for jev review. Paths are relative to this file."
- `spec`: the spec path relative to the config file's directory.
- One unit per approved row, in map order: `name`, `spec_heading` (the exact heading line), `implementation`, `tests`, `behaviors` derived one per requirement sentence, `accepted: []`, `notes: []`. Set a unit's own `spec` only when it reads a different file.
- Every path relative to the config file's directory, with forward slashes.

## 7. Dry run

```bash
jev review --dry-run --units <config-path>
```

Exit 0: continue. Exit 8: read each `error:` line, fix the heading text in the config or the spec, or the file path, and rerun. Exit 1: fix the config. Loop until exit 0. Keep the final table for the hand-off.

## 8. Hand-off

Print:

1. The files written or edited, with paths.
2. The final dry-run table.
3. "Sentences written from code; confirm each states intent:" followed by each sentence with its source file and line. In `existing` mode, also each rewording with before and after.
4. The next command: `jev review --units <config-path>`. Say that it sends the spec section and every listed file to the TypeSafe API and needs `TYPESAFE_API_KEY`. Add "TYPESAFE_API_KEY is not set in this shell" when it is unset.
5. One line: "Run it and paste the output, and I will triage the flags: add a test, fix the spec or the code, or record an acceptance with a note."

Do not run the paid review.
