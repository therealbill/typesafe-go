# Releases and Versioning — Design

Date: 2026-09-29

## Goal

Make `go install github.com/therealbill/typesafe-go/cmd/jev@latest` and the
release binaries real: give `jev version` a meaningful answer for every build
path, define how a release is cut, cut `v0.1.0`, and correct the install
documentation.

Out of scope: Windows builds, package-manager formulas, signing, changelog
tooling beyond goreleaser's git changelog.

## Version reporting

`internal/version` keeps the two ldflags variables and adds a resolver:

```go
type Info struct {
    Version  string // "v0.1.0", a pseudo-version, "(devel)", or "dev"
    Commit   string // short revision, or "none"
    Modified bool   // true when built from a dirty tree
    Source   string // "ldflags", "module", "vcs", or "unknown"
}
func Get() Info
```

Resolution order:

1. If `Version` was set by ldflags (not `"dev"`), use it and `Commit` as
   given; `Source` is `ldflags`. goreleaser builds and `make build` land
   here.
2. Else, from `runtime/debug.ReadBuildInfo()`: if `Main.Version` is set and
   not `(devel)`, use it (`go install ...@v0.1.0` gives `v0.1.0`,
   `@latest` gives the resolved tag or pseudo-version); `Source` is
   `module`. `Commit` comes from the `vcs.revision` setting when present.
3. Else, if the `vcs.revision` setting is present (a plain `go build` in a
   clone), `Version` is `(devel)`, `Commit` is the short revision,
   `Modified` reflects `vcs.modified`; `Source` is `vcs`.
4. Else `dev`/`none`, `Source` `unknown`.

`jev version` prints `{"version","commit","modified","source","go"}`. The
Makefile keeps injecting version and commit through ldflags for `make build`.

## Cutting a release

Semantic versioning. `v0.x.y` until the API settles; the SDK spec's public
surface is the thing a major bump protects.

`make release VERSION=v0.1.0`:

1. Validates `VERSION` against `^v[0-9]+\.[0-9]+\.[0-9]+$`.
2. Refuses unless the working tree is clean, the current branch is `main`,
   and `main` is up to date with `origin/main`.
3. Refuses if the tag already exists locally or remotely.
4. Runs `make lint test`.
5. Creates an annotated tag `git tag -a VERSION -m "typesafe-go VERSION"`
   and pushes it with `git push origin VERSION`.

The existing `.github/workflows/release.yml` runs goreleaser on the tag and
attaches `jev` archives for darwin and linux, amd64 and arm64, plus
`checksums.txt`. No new workflow. `v0.1.0` is cut by the lead at the end of
this work, after the documentation is pushed.

## Documentation

README install section becomes:

```markdown
## Install

Library:

    go get github.com/therealbill/typesafe-go@latest

Command-line tool (needs Go 1.25 or newer):

    go install github.com/therealbill/typesafe-go/cmd/jev@latest

Prebuilt `jev` binaries for macOS and Linux are attached to each
[release](https://github.com/therealbill/typesafe-go/releases). To build from
a clone, run `make build`; the binary lands in `bin/jev`.
```

Pages updated from source:

- `docs/how-to/cut-a-release.md`: the `make release` flow, the checks it
  performs, what the workflow produces, and how to verify (`gh release
  view`, `go install ...@vX.Y.Z`, `jev version`).
- `docs/reference/jev-cli.md`: the `version` subcommand's fields and the
  four resolution sources.
- `docs/reference/makefile-and-repository-layout.md`: the `release` target.
- `docs/tutorials/jev-from-the-command-line.md`: install via `go install`
  as the primary path, `make build` as the alternative; re-run the
  `jev version` example after the change.
- `docs/tutorials/first-judgment-in-go.md`,
  `docs/tutorials/build-a-ticket-router.md`,
  `docs/reference/client-options-and-environment.md`: `go get ...@latest`
  where they show the install line.
- `docs/explanation/the-agent-facing-cli-contract.md` if it describes
  `version` output.

## Verification

- Unit tests for `version.Get()` covering the four sources by injecting the
  build info reader.
- `make build && ./bin/jev version` reports the ldflags source.
- `go build -o /tmp/jev-plain ./cmd/jev && /tmp/jev-plain version` reports
  `(devel)` with the current commit and `source: vcs`.
- After the tag: `gh run watch` on the release workflow succeeds,
  `gh release view v0.1.0` lists four archives and the checksum file, and
  `GOFLAGS= go install github.com/therealbill/typesafe-go/cmd/jev@v0.1.0`
  into a temp `GOBIN` reports `v0.1.0` with `source: module`.
- `make docs` passes.

## Execution

`cli` (go-architect, opus): version package, `jev version`, Makefile target,
tests. `docs` (general-purpose, sonnet, prose rubric applies): README and
pages. Lead: gate, push docs, `make release VERSION=v0.1.0`, verify the
release and the `go install` path.
