---
title: "How to Cut a Release"
description: "Tag a version to trigger the GoReleaser-based release workflow, and know what it builds, names, and attaches to the GitHub release."
diataxis: how-to
weight: 110
---

# How to Cut a Release

**Goal**: Trigger a `jev` release by pushing a version tag, and know exactly
what the automated pipeline builds and publishes from it.

## Prerequisites

- Push access to the repository, with permission to push tags
- A clean `main` (or the commit you intend to release) already merged
- `jev version` already builds locally with `make build`, see
  [How to Configure Logging](./configure-logging.md)'s `bin/jev` prerequisite
  for confirming a local build works

## Steps

### 1. Choose a version and tag it

Tags matching `v*` trigger the release workflow (`.github/workflows/release.yml`,
`on: push: tags: ["v*"]`). Use a semantic version:

```
git tag v1.2.0
git push origin v1.2.0
```

**Do not run these two commands as a rehearsal.** Pushing a real `v*` tag
starts the actual release pipeline and publishes a GitHub release. Everything
past this point describes what that push does; treat it as reference, not a
next step to try right now.

### 2. Know what the tag push kicks off

The `release` workflow checks out full history (`fetch-depth: 0`, which
GoReleaser needs to generate a changelog from git log), sets up Go from
`go.mod`, and runs `goreleaser/goreleaser-action@v6` with
`args: release --clean`, authenticated as `GITHUB_TOKEN`.

### 3. Know what GoReleaser builds

`.goreleaser.yaml` builds the `./cmd/jev` binary for `goos: [darwin,
linux]` × `goarch: [amd64, arm64]` (four binaries) with `CGO_ENABLED=0`.
Each build's `ldflags` inject the same two version symbols `make build` does
locally:

```
-X github.com/therealbill/typesafe-go/internal/version.Version={{ .Version }}
-X github.com/therealbill/typesafe-go/internal/version.Commit={{ .ShortCommit }}
```

`{{ .Version }}` comes from the tag (`v1.2.0` → `1.2.0`); `{{ .ShortCommit }}`
is the short commit hash being tagged.

### 4. Know what gets attached to the GitHub release

Each of the four binaries is archived as a `.tar.gz`
(`name_template: "jev_{{ .Version }}_{{ .Os }}_{{ .Arch }}"`), and a
`checksums.txt` is generated over all of them (`checksum.name_template`).
The GoReleaser action attaches every archive plus `checksums.txt` to the
GitHub release it creates under `release.github.owner: therealbill`,
`release.github.name: typesafe-go`, with the changelog generated from git
log (`changelog.use: git`).

### 5. Confirm the version a build reports, before and after a release

`jev version` prints `{"version": ..., "commit": ..., "go": ...}`.
Locally, without a real tag in the repository's history, `Version` and
`Commit` both fall back to the same short commit hash. `Makefile`'s
`VERSION` is `git describe --tags --always --dirty`, which falls back to
`git rev-parse --short HEAD` when there's no tag to describe from. Once a
real `v*` tag exists and is checked out, `git describe` reports that tag
instead, and GoReleaser's build reports its own `{{ .Version }}` the same
way.

## Verify it works

A real local build, run right now against this repository's current state
(no release tag exists yet), prints:

```
$ ./bin/jev version --pretty
{
  "commit": "359389b",
  "go": "go1.27.1",
  "version": "359389b"
}
```

Both `version` and `commit` are the same short commit hash. This is a
**local development build**, not a release. A binary built by the release
workflow from a `v1.2.0` tag would instead report `"version": "1.2.0"` with
`"commit"` still the short hash of the tagged commit.

✅ You know what a tag push does before you do it, and what the resulting
binary's `jev version` output should look like once it's a real release,
not a local build.

## Troubleshooting

### Problem: the workflow doesn't start after pushing a tag
**Symptom**: no `release` run appears in GitHub Actions.
**Cause**: the tag doesn't match the `v*` glob (`.github/workflows/release.yml`'s
`on.push.tags`), or it was pushed to a fork, not this repository.
**Solution**: confirm the tag name starts with `v` (`v1.2.0`, not `1.2.0`),
and that `git push origin <tag>` targeted the real repository's remote.

### Problem: GoReleaser fails with a changelog or version error
**Symptom**: the `goreleaser-action` step fails early, before any build.
**Cause**: the checkout didn't use `fetch-depth: 0`, or the tag wasn't an
annotated/lightweight tag reachable from the checked-out history. GoReleaser
needs both to compute `{{ .Version }}` and the git-log
changelog.
**Solution**: this repository's workflow already sets `fetch-depth: 0`; if
you're reproducing the build locally with `goreleaser release --clean`,
make sure your local clone isn't shallow.

### Problem: `jev version` on a downloaded release binary still shows a raw commit hash instead of a version number
**Symptom**: `"version"` doesn't look like `"1.2.0"`.
**Cause**: you're running a binary built from a commit that wasn't
tagged at build time, or ldflags weren't applied to that build.
**Solution**: rebuild from a real `v*` tag through the release workflow, or
locally with `git describe --tags` returning a real tag name. A local
`make build` on an untagged commit always falls back to the short hash, by
design.

## Next steps

- [Run the self-review](./run-the-self-review.md) before tagging, so a
  release isn't cut on top of a known, un-accepted regression.

## See also

- [Makefile and Repository Layout](../reference/makefile-and-repository-layout.md)
- [jev CLI](../reference/jev-cli.md)
