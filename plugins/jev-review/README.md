# jev-review

A Claude Code plugin that writes the two files `jev review` needs: a Markdown spec with one heading per concern, and `jev-review.json`, which pairs each spec section with the files that implement and test it and lists the behaviors to check.

`jev review` judges whether your tests exercise the behaviors you listed and whether your implementation contradicts the spec section you named. It sends those files to Jev and answers with probabilities. It does not find bugs, review style, check security, or verify that the code is correct, and its readings drift between runs on the same input.

## Requirements

- Claude Code.
- `jev` on your `PATH`, at a release that has `jev review --dry-run`. Install with `go install github.com/therealbill/typesafe-go/cmd/jev@latest` (Go 1.25 or newer) or download a binary from the [releases page](https://github.com/therealbill/typesafe-go/releases). Check with `jev review --help`.
- `TYPESAFE_API_KEY` is not needed to run the plugin. It is needed when you run `jev review` afterwards.

## Install

```
/plugin marketplace add therealbill/typesafe-go
/plugin install jev-review@typesafe-go
```

## Use

Run `/jev-review:setup` in the repository. Both arguments are optional: a spec path, then a config path.

```
/jev-review:setup
/jev-review:setup docs/design.md
/jev-review:setup docs/design.md internal/widgets/jev-review.json
```

The command surveys the repository and shows a table of proposed units: spec section, implementation files, test files, bundle sizes, and the evidence for each pairing. It waits for your approval. Then it writes or edits the spec, writes the config, and runs `jev review --dry-run` until it passes. It never runs the paid review. The hand-off lists every sentence it wrote from code so you can confirm each one states intent.

The skill `setting-up-jev-review` also activates on its own when you ask about `jev-review.json`, spec headings, behaviors, or reviewing a codebase with Jev.

## What it writes

- The spec, at the path you gave, or `docs/spec.md` when `docs/` exists, or `SPEC.md`. An existing spec is edited in place, never rewritten.
- `jev-review.json` in the current directory, or at the path you gave, with one unit per approved row and empty `accepted` and `notes`.

## Components

| Path | Purpose |
|---|---|
| `commands/setup.md` | `/jev-review:setup`, the workflow with one approval stop. |
| `agents/repo-mapper.md` | Read-only survey that proposes units with evidence. |
| `skills/setting-up-jev-review/` | The spec rules, file pairing, behavior writing, config schema, and per-language conventions. |

## Documentation

- [Build a jev review config with Claude Code](https://therealbill.github.io/typesafe-go/docs/how-to/build-a-jev-review-config-with-claude-code/)
- [Review your codebase with jev](https://therealbill.github.io/typesafe-go/docs/how-to/review-your-codebase-with-jev/)
- [jev review reference](https://therealbill.github.io/typesafe-go/docs/reference/jev-review/)
