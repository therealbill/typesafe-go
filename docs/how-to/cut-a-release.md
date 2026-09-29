---
title: "How to Cut a Release"
description: "Run make release VERSION=vX.Y.Z to validate, tag, and push a release, and know what the resulting workflow builds and publishes."
diataxis: how-to
weight: 110
---

**Goal**: Cut a release with `make release VERSION=vX.Y.Z`, and know what
that command checks, what the tag push triggers, and what ends up attached
to the GitHub release.

## Prerequisites

- Push access to the repository, with permission to push tags
- A clean checkout of `main`, merged and pushed to `origin/main`
- `make lint` and `make test` already pass locally, since `make release`
  runs both and stops if either fails
- No release has been cut from this repository yet, so every example below
  uses a placeholder `vX.Y.Z`; substitute a real version when you run it

## Steps

### 1. Pick a version number

This repository follows semantic versioning. While the library's public
surface is still settling, versions stay `v0.x.y`; a `v1.0.0` marks that
surface as stable. The `jev` CLI's own behavior is not what a major version
bump protects, the Go library's public API is.

A version must match `^v[0-9]+\.[0-9]+\.[0-9]+$`, for example `v0.2.0` or
`v1.4.3`. `make release` rejects anything else before doing anything.

### 2. Run `make release` with that version

```
make release VERSION=vX.Y.Z
```

Leaving `VERSION` off entirely also fails the format check, since the
Makefile's default version string (derived from `git describe`) never
matches `vX.Y.Z`. Running `make release` with no arguments is a safe way to
confirm the target exists without doing anything.

### 3. Know what each check does, in order

`make release` runs these checks and actions in sequence, stopping at the
first failure:

1. Confirms `VERSION` matches `vX.Y.Z`.
2. Confirms the working tree has no uncommitted changes
   (`git status --porcelain` is empty).
3. Confirms the current branch is `main`.
4. Fetches `origin main` and confirms local `HEAD` matches `origin/main`
   exactly, catching both a branch that's behind and one with unpushed
   commits.
5. Confirms the tag doesn't already exist locally.
6. Confirms the tag doesn't already exist on `origin`.
7. Runs `make lint test`.
8. Creates an annotated tag: `git tag -a "vX.Y.Z" -m "typesafe-go vX.Y.Z"`.
9. Pushes the tag: `git push origin "vX.Y.Z"`.
10. Prints a confirmation line naming the version that was pushed.

Steps 8 and 9 are the only ones that change repository state. Everything
before them only reads it.

### 4. Know what the pushed tag triggers

Any tag matching `v*` triggers the `release` workflow
(`.github/workflows/release.yml`). It checks out full git history
(`fetch-depth: 0`, which GoReleaser needs for its changelog), sets up Go
from `go.mod`, and runs `goreleaser/goreleaser-action@v6` with
`args: release --clean`, authenticated with the repository's own
`GITHUB_TOKEN`.

### 5. Know what GoReleaser builds and publishes

`.goreleaser.yaml` builds `./cmd/jev` for `darwin` and `linux`, each for
`amd64` and `arm64`: four binaries, all with `CGO_ENABLED=0`. Each build's
ldflags set `internal/version.Version` to `{{ .Tag }}` (the full tag, for
example `vX.Y.Z`) and `internal/version.Commit` to the short hash of the
tagged commit.

Each binary is archived as a `.tar.gz` named
`jev_{{ .Version }}_{{ .Os }}_{{ .Arch }}`. GoReleaser's `.Version`
variable strips the leading `v` from the tag, so tag `vX.Y.Z` produces
`jev_X.Y.Z_darwin_amd64.tar.gz`, `jev_X.Y.Z_darwin_arm64.tar.gz`,
`jev_X.Y.Z_linux_amd64.tar.gz`, and `jev_X.Y.Z_linux_arm64.tar.gz`. A
`checksums.txt` covers all four. The GitHub release's changelog is
generated from git log, and the release is published under
github.com/therealbill/typesafe-go.

## Verify it works

Watch the workflow run that the tag push triggered:

```bash
gh run watch $(gh run list --workflow release --limit 1 --json databaseId -q '.[0].databaseId') --exit-status
```

Once it finishes, list the assets attached to the release:

```bash
gh release view vX.Y.Z --json assets -q '.assets[].name'
```

Expect four `.tar.gz` archives plus `checksums.txt`.

Confirm the published binary installs and reports itself correctly:

```bash
go install github.com/therealbill/typesafe-go/cmd/jev@vX.Y.Z
jev version
```

This reports `"version":"vX.Y.Z"` and `"source":"module"`. A binary
downloaded from the release itself reports the same version string with
`"source":"ldflags"` instead. The two differ because `go install` resolves
the version from the module system's record of the tag, while a release
binary has the version baked in at build time through ldflags.

✅ The workflow run finished successfully, the release has four archives
and a checksums file attached, and an installed binary reports the version
you tagged.

## Troubleshooting

### Problem: `make release` fails immediately with a VERSION message
**Symptom**: `release: VERSION must look like v1.2.3, got '...'`.
**Cause**: `VERSION` was omitted, or doesn't match `vX.Y.Z` (no `v` prefix,
extra characters, missing a segment).
**Solution**: rerun with `VERSION=vX.Y.Z` in that exact three-segment form.

### Problem: `make release` fails on the working tree or branch check
**Symptom**: `release: working tree is not clean` or `release: not on main`.
**Cause**: there are uncommitted changes, or the current branch isn't
`main`.
**Solution**: commit or stash the changes, or switch to `main`, then rerun.

### Problem: `make release` fails on the up-to-date check
**Symptom**: `release: main is not up to date with origin/main`.
**Cause**: local `main` is behind `origin/main`, or has commits
`origin/main` doesn't have yet.
**Solution**: pull or push as needed until local `HEAD` matches
`origin/main`, then rerun.

### Problem: `make release` fails because the tag already exists
**Symptom**: `release: tag vX.Y.Z already exists locally` or `... already
exists on origin`.
**Cause**: that version was already tagged, locally or on the remote.
**Solution**: pick a different, unused version, or delete the existing tag
first if it was created in error (see below).

### Problem: `make release` fails during `lint` or `test`
**Symptom**: the command stops with lint or test output instead of
reaching the tag step.
**Cause**: `make lint test` failed, so no tag was created or pushed; the
working tree still isn't in a state you'd want to release.
**Solution**: fix the failure, confirm `make lint test` passes on its own,
then rerun `make release VERSION=vX.Y.Z`.

### Problem: the workflow fails after the tag was already pushed
**Symptom**: `gh run watch` exits non-zero, or the release run shows
failed in GitHub Actions.
**Cause**: something in the GoReleaser build or publish step failed after
`make release` already pushed the tag.
**Solution**: delete the tag both locally and on the remote, fix the
underlying problem, and retag:

```bash
git tag -d vX.Y.Z
git push origin :refs/tags/vX.Y.Z
```

Then run `make release VERSION=vX.Y.Z` again.

## Next steps

- [Review your codebase with Jev](./review-your-codebase-with-jev.md) before
  releasing, so a release isn't cut on top of a known, un-accepted
  regression.

## See also

- [Makefile and Repository Layout](../reference/makefile-and-repository-layout.md)
- [jev CLI](../reference/jev-cli.md)
