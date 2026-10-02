---
title: "Build a jev review Config with Claude Code"
description: "Install the jev-review plugin from this repository's marketplace, run /jev-review:setup, approve the unit map, and check the result with jev review --dry-run before a paid run."
diataxis: how-to
weight: 106
---

`jev review` judges whether your tests exercise the behaviors you listed and whether your implementation contradicts the spec section you named. It sends those files to Jev and answers with probabilities. It does not find bugs, review style, check security, or verify that the code is correct, and its readings drift between runs on the same input.

This page covers getting the spec and `jev-review.json` written by the `jev-review` Claude Code plugin. Writing them by hand is covered by [Review your codebase with jev](./review-your-codebase-with-jev.md) and [How to write a spec jev review can use](./write-a-spec-jev-review-can-use.md).

**Goal**: a spec and a `jev-review.json` for your repository that pass `jev review --dry-run`.

## Prerequisites

- Claude Code.
- `jev` on `PATH`, at a release with `jev review --dry-run`. Install with `go install github.com/therealbill/typesafe-go/cmd/jev@latest` (Go 1.25 or newer), or download a binary from the [releases page](https://github.com/therealbill/typesafe-go/releases). Check with `jev review --help | grep dry-run`.
- A repository with tests.
- `TYPESAFE_API_KEY` is not needed until the last step.

## Steps

### 1. Add the marketplace and install the plugin

Run these two commands in Claude Code:

```
/plugin marketplace add therealbill/typesafe-go
/plugin install jev-review@typesafe-go
```

Run `/plugin` and confirm `jev-review` is listed among your installed plugins.

### 2. Run /jev-review:setup in the repository

Run `/jev-review:setup` in the repository you want to review. It takes two optional arguments: a spec path, then a config path.

```
/jev-review:setup
/jev-review:setup docs/design.md
/jev-review:setup docs/design.md internal/widgets/jev-review.json
```

The config path defaults to `jev-review.json` in the current directory.

When you give no spec path, the command searches the repository for candidate spec files. If it finds none, it switches to new-spec mode and writes the spec at `docs/spec.md` when a `docs/` directory exists, or at `SPEC.md` otherwise.

### 3. Approve the unit map

The command surveys the repository and shows a table like this one:

```
Spec: docs/design.md (existing)

| Unit | Heading | Implementation | Tests | Bytes | Evidence |
|---|---|---|---|---|---|
| retry | ## Retry policy (existing) | retry.go, transport.go | retry_test.go, client_test.go | 11520/34110 | client_test.go asserts on retry-after-ms and the budget |
```

Unit is the unit name. Heading is the spec heading it maps to, marked existing or new. Implementation lists the files that implement the unit. Tests lists the files that test it. Bytes is the implementation bundle's size and the tests bundle's size, in bytes. Evidence states why those test files were paired with that implementation.

A `!` next to a side of the Bytes column marks that side over 50000 bytes. That unit should be split.

You get one question: "Approve this unit map?" Answer "Approve as is" to continue, or "Describe changes" to have the table rerun with your changes applied. This loops until you approve.

### 4. Read the hand-off

The hand-off lists the files written or edited, the final dry-run table, every sentence written from code with its source file and line, and the next command to run.

The command derived each sentence from reading the code. Confirm that each one states intent.

### 5. Rerun the dry run yourself

You can rerun `jev review --dry-run` yourself at any point:

```
$ jev review --dry-run
review: checked 2 units
# jev review --dry-run

budget: 50000 bytes each for implementation and tests

| Unit | Spec bytes | Implementation bytes | Tests bytes | Truncated | Status |
|---|---|---|---|---|---|
| retry | 186 | 269 | 282 | no | ok |
| validation | 132 | 227 | 176 | no | ok |

exit 0
```

Unit is the unit name. Spec bytes is the size of the extracted spec section. Implementation bytes and Tests bytes are each bundle's size before truncation. Truncated is yes when a bundle was cut to fit the budget. Status is ok or error.

### 6. Run jev review for the readings

This step needs `TYPESAFE_API_KEY` set. Running `jev review`, without `--dry-run`, sends the spec section and every listed file to the TypeSafe API.

See [Review your codebase with jev](./review-your-codebase-with-jev.md) for how to triage the flags it returns. The plugin's skill also triages flags when you paste them back into the Claude Code session.

## Verify it works

`jev review --dry-run; echo $?` prints a table with an `ok` row per unit and `0`.

## See also

- [Review your codebase with jev](./review-your-codebase-with-jev.md)
- [How to write a spec jev review can use](./write-a-spec-jev-review-can-use.md)
- [jev review reference](../reference/jev-review.md)
- [jev-review plugin](https://github.com/therealbill/typesafe-go/tree/main/plugins/jev-review)
