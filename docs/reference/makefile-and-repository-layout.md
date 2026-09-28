---
title: "Makefile and Repository Layout"
description: "The make targets from make help, one line per top-level directory and package, and every GitHub Actions workflow with its trigger."
diataxis: reference
weight: 90
---

## `make help`

Captured by running `make help` from the repository root:

```
$ make help
  help         Show this help
  test         Run unit tests with the race detector
  lint         Run gofmt check, go vet, and golangci-lint
  vuln         Run govulncheck
  build        Build bin/jev
  integration  Run the live API test (needs TYPESAFE_API_KEY)
  selfreview   Run the Jev-driven self-review (needs TYPESAFE_API_KEY)
  docs         Build the site and check that every docs/ page is reachable and links resolve
  site         Build the documentation site into site/public
  site-serve   Serve the documentation site locally with live reload
  clean        Remove build outputs
```

## Repository layout

One line per top-level directory and top-level package, as they exist in the
repository root at the time this page was written:

| Path | Contents |
|---|---|
| `*.go` (repository root) | The `typesafe` client library package (module `github.com/therealbill/typesafe-go`): `client.go`, `errors.go`, `instrument.go`, `models.go`, `question.go`, `response.go`, `retry.go`, `transport.go`, `typed.go`, `doc.go`, plus each file's `_test.go` counterpart and `integration_test.go`. |
| `cmd/jev` | The `jev` binary's `main` package (`main.go`); calls `internal/cli.Main`. |
| `internal/cli` | The `jev` command implementation: `root.go`, `ask.go`, `models.go`, `version.go`, `request.go`, `exit.go`, `output.go`, `telemetry.go`, and their `_test.go` counterparts. |
| `otel` | The `typesafe/otel` package: an `Instrumentation` implementation (`otel.go`) that reports OpenTelemetry traces for client calls, plus `otel_test.go`. |
| `tools/selfreview` | The self-review tool: `main.go`, `main_test.go`, and `units.json`. Documented on the [self-review tool reference](self-review-tool.md). |
| `testdata` | Fixture JSON files: `models_ok.json`, `request.json`, `systemone_ok.json`. |
| `docs` | This documentation set: `_index.md` plus `explanation`, `how-to`, `reference`, `superpowers`, and `tutorials` subdirectories. |
| `site` | The Hugo site that builds `docs/` (present in the repository at the time of writing): `hugo.toml`, `go.mod`, `go.sum`, `content`, `layouts`, `resources`, `public`. |
| `.github/workflows` | CI/CD workflow definitions: `ci.yml`, `hugo-deploy.yml`, `hugo-pr.yml`, `release.yml`. |

## GitHub Actions workflows

Read from `.github/workflows/` at the time of writing.

### `ci.yml` (`name: ci`)

Triggers on `push` to branch `main`, and on every `pull_request`. Two jobs:

| Job | Steps |
|---|---|
| `test` | Checks out the repository, sets up Go from `go.mod`, runs `go test -race -cover ./...`, then `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`. |
| `lint` | Checks out the repository, sets up Go from `go.mod`, runs `golangci/golangci-lint-action@v8` at `version: v2.5`. |

### `hugo-deploy.yml` (`name: Deploy documentation site`)

Triggers on `push` to branch `main` when the change touches `docs/**`,
`site/**`, or `.github/workflows/hugo-deploy.yml`, and on
`workflow_dispatch`. Permissions: `contents: read`, `pages: write`,
`id-token: write`. Concurrency group `pages` with `cancel-in-progress:
false`. Two jobs:

| Job | Steps |
|---|---|
| `build` | Installs Hugo `0.166.0` (extended, via a downloaded `.deb`), checks out with `fetch-depth: 0`, sets up Go from `go.mod`, configures GitHub Pages (`actions/configure-pages@v5`), caches `~/.cache/hugo_cache` keyed on `site/go.sum`, runs `hugo --gc --minify -s site --baseURL "<pages base url>/"`, and uploads `site/public` as the Pages artifact. |
| `deploy` (needs `build`) | Runs `actions/deploy-pages@v4` against the `github-pages` environment. |

### `hugo-pr.yml` (`name: Documentation site build check`)

Triggers on `pull_request` when the change touches `docs/**`, `site/**`, or
`.github/workflows/hugo-pr.yml`. One job:

| Job | Steps |
|---|---|
| `build` | Installs Hugo `0.166.0` (extended, via a downloaded `.deb`), checks out with `fetch-depth: 0`, sets up Go from `go.mod`, runs `hugo --gc --minify -s site --baseURL "https://therealbill.github.io/typesafe-go/"`, then appends a page count (`find site/public -name index.html \| wc -l`) and a size (`du -sh site/public`) to `$GITHUB_STEP_SUMMARY`. |

### `release.yml` (`name: release`)

Triggers on `push` of a tag matching `v*`. Permissions: `contents: write`.
One job:

| Job | Steps |
|---|---|
| `goreleaser` | Checks out with `fetch-depth: 0`, sets up Go from `go.mod`, runs `goreleaser/goreleaser-action@v6` (distribution `goreleaser`, version `~> v2`) with `args: release --clean`, using `GITHUB_TOKEN` from `secrets.GITHUB_TOKEN`. |
