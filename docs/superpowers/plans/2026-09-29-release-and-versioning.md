# Releases and Versioning Implementation Plan

> **For agentic workers:** Execute this plan with an **Agent Team** of named agents (Agent tool with `name`, coordinated by the lead through SendMessage). Do NOT use git worktrees, do NOT use unnamed parallel subagents as the execution method, and do NOT use the `superpowers:executing-plans` skill. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `jev version` meaningful for every build path, add a `make release` target that tags and pushes, cut `v0.1.0`, and correct the install documentation.

**Architecture:** `internal/version.Get()` resolves version, commit, and dirty state from ldflags, then module build info, then VCS build info. `make release VERSION=vX.Y.Z` validates the tree and tag, runs the gate, and pushes an annotated tag; the existing goreleaser workflow builds the release. Docs describe the real install paths.

**Tech Stack:** Go 1.25 (`runtime/debug`), GNU Make, goreleaser v2, `gh`.

**Spec:** `docs/superpowers/specs/2026-09-29-release-and-versioning-design.md`.

---

## Execution Model

| Agent name | Type / model | Owns |
|---|---|---|
| `cli` | `go-architect`, model **opus** | Tasks 1–2 (version package, `jev version`, Makefile, goreleaser tweak) |
| `docs` | `general-purpose`, model **sonnet** | Task 3 (README and pages), after Task 1 so `jev version` output is real |
| lead | — | Task 4 (gate, push, `make release VERSION=v0.1.0`, verification) |

Rules are the same as the previous plans: main branch, explicit-path staging, the given commit identity, no attribution lines, no push by agents, plain prose (no em dashes, no narrated reasoning, no duration estimates), never print `TYPESAFE_API_KEY`.

---

## Task 1: Version resolution

**Agent:** `cli`

**Files:**
- Modify: `internal/version/version.go`, `internal/cli/version.go`, `internal/cli/root.go`, `internal/cli/cli_test.go`
- Create: `internal/version/version_test.go`

- [x] **Step 1: Write the failing tests `internal/version/version_test.go`**

```go
package version

import (
	"runtime/debug"
	"testing"
)

func withBuildInfo(t *testing.T, bi *debug.BuildInfo, ok bool) {
	t.Helper()
	prev := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) { return bi, ok }
	t.Cleanup(func() { readBuildInfo = prev })
}

func withLdflags(t *testing.T, v, c string) {
	t.Helper()
	pv, pc := Version, Commit
	Version, Commit = v, c
	t.Cleanup(func() { Version, Commit = pv, pc })
}

func TestGetPrefersLdflags(t *testing.T) {
	withLdflags(t, "v9.9.9", "abc1234")
	withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "v0.0.1"}}, true)
	got := Get()
	if got.Version != "v9.9.9" || got.Commit != "abc1234" || got.Source != "ldflags" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetUsesModuleVersion(t *testing.T) {
	withLdflags(t, "dev", "none")
	withBuildInfo(t, &debug.BuildInfo{
		Main:     debug.Module{Version: "v0.1.0"},
		Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "0123456789abcdef0123"}, {Key: "vcs.modified", Value: "false"}},
	}, true)
	got := Get()
	if got.Version != "v0.1.0" || got.Commit != "0123456" || got.Modified || got.Source != "module" {
		t.Fatalf("got %+v", got)
	}
	withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20260929120000-0123456789ab"}}, true)
	got = Get()
	if got.Version != "v0.0.0-20260929120000-0123456789ab" || got.Commit != "none" || got.Source != "module" {
		t.Fatalf("pseudo-version: got %+v", got)
	}
}

func TestGetUsesVCSForDevelBuilds(t *testing.T) {
	withLdflags(t, "dev", "none")
	withBuildInfo(t, &debug.BuildInfo{
		Main:     debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "fedcba9876543210"}, {Key: "vcs.modified", Value: "true"}},
	}, true)
	got := Get()
	if got.Version != "(devel)" || got.Commit != "fedcba9" || !got.Modified || got.Source != "vcs" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetUnknown(t *testing.T) {
	withLdflags(t, "dev", "none")
	withBuildInfo(t, nil, false)
	if got := Get(); got.Version != "dev" || got.Commit != "none" || got.Source != "unknown" {
		t.Fatalf("no build info: got %+v", got)
	}
	withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true)
	if got := Get(); got.Version != "dev" || got.Source != "unknown" {
		t.Fatalf("devel without vcs: got %+v", got)
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/version/ 2>&1 | head -3
```
Expected: undefined `Get`, `readBuildInfo`.

- [x] **Step 3: Replace `internal/version/version.go`**

```go
// Package version reports how the binary was built.
package version

import "runtime/debug"

// Version is set by -ldflags at release time and by make build. "dev" means
// it was not set.
var Version = "dev"

// Commit is the short git commit set by -ldflags. "none" means it was not set.
var Commit = "none"

// Info is the resolved build information.
type Info struct {
	// Version is a tag such as v0.1.0, a pseudo-version, "(devel)" for a
	// build from a clone, or "dev" when nothing is known.
	Version string `json:"version"`
	// Commit is the short revision, or "none".
	Commit string `json:"commit"`
	// Modified is true when the build came from a tree with uncommitted changes.
	Modified bool `json:"modified"`
	// Source says where Version came from: ldflags, module, vcs, or unknown.
	Source string `json:"source"`
}

var readBuildInfo = debug.ReadBuildInfo

// Get resolves build information. ldflags win; otherwise the module version
// recorded by go install or go build in module mode is used; otherwise the
// VCS revision embedded in a build from a clone; otherwise "dev".
func Get() Info {
	if Version != "dev" {
		return Info{Version: Version, Commit: Commit, Source: "ldflags"}
	}
	bi, ok := readBuildInfo()
	if !ok || bi == nil {
		return Info{Version: Version, Commit: Commit, Source: "unknown"}
	}
	rev, modified := "", false
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	short := rev
	if len(short) > 7 {
		short = short[:7]
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		if short == "" {
			short = "none"
		}
		return Info{Version: v, Commit: short, Modified: modified, Source: "module"}
	}
	if rev != "" {
		return Info{Version: "(devel)", Commit: short, Modified: modified, Source: "vcs"}
	}
	return Info{Version: Version, Commit: Commit, Source: "unknown"}
}
```

- [x] **Step 4: Use it in the CLI**

`internal/cli/version.go`:

```go
package cli

import (
	"runtime"

	"github.com/spf13/cobra"

	"github.com/therealbill/typesafe-go/internal/version"
)

func newVersionCmd(streams IO) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pretty, _ := cmd.Flags().GetBool("pretty")
			info := version.Get()
			return writeJSON(streams.Out, map[string]any{
				"version":  info.Version,
				"commit":   info.Commit,
				"modified": info.Modified,
				"source":   info.Source,
				"go":       runtime.Version(),
			}, pretty)
		},
	}
}
```

In `internal/cli/root.go`, set `root.Version = version.Get().Version` (replacing `version.Version`). In `internal/cli/cli_test.go`, the `--version` test compares against `version.Get().Version`, and `TestVersion` additionally asserts `"source"` and `"modified"` appear.

- [x] **Step 5: Verify and commit**

```bash
go test -race ./internal/version/ ./internal/cli/ 2>&1 | tail -3
golangci-lint run ./internal/...
make build && ./bin/jev version
go build -o /tmp/jev-plain ./cmd/jev && /tmp/jev-plain version
git add internal/version/version.go internal/version/version_test.go internal/cli/version.go internal/cli/root.go internal/cli/cli_test.go
git commit -m "Resolve version from ldflags, module build info, or VCS"
```
Expected: `make build` output shows `"source":"ldflags"`; the plain build shows `"version":"(devel)"`, the current short commit, and `"source":"vcs"`. Paste both lines in the report.

---

## Task 2: `make release` and goreleaser tag format

**Agent:** `cli`

**Files:**
- Modify: `Makefile`, `.goreleaser.yaml`

- [x] **Step 1: Add the target**

Add `release` to `.PHONY` and this target after `build`:

```make
release: ## Tag and push a release: make release VERSION=vX.Y.Z (runs lint and test first)
	@echo "$(VERSION)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$' || (echo "release: VERSION must look like v1.2.3, got '$(VERSION)'" && exit 1)
	@test -z "$$(git status --porcelain)" || (echo "release: working tree is not clean" && exit 1)
	@test "$$(git rev-parse --abbrev-ref HEAD)" = "main" || (echo "release: not on main" && exit 1)
	@git fetch -q origin main
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse origin/main)" || (echo "release: main is not up to date with origin/main" && exit 1)
	@! git rev-parse -q --verify "refs/tags/$(VERSION)" >/dev/null || (echo "release: tag $(VERSION) already exists locally" && exit 1)
	@! git ls-remote --exit-code --tags origin "$(VERSION)" >/dev/null 2>&1 || (echo "release: tag $(VERSION) already exists on origin" && exit 1)
	$(MAKE) lint test
	git tag -a "$(VERSION)" -m "typesafe-go $(VERSION)"
	git push origin "$(VERSION)"
	@echo "release: pushed $(VERSION); the release workflow builds and publishes the binaries"
```

`VERSION` is already a Makefile variable (defaulting to `git describe`), so `make release` without `VERSION=` fails the format check, which is the intended behavior.

- [x] **Step 2: goreleaser uses the tag as the version string**

In `.goreleaser.yaml`, change the ldflags line for `Version` to use `{{ .Tag }}` instead of `{{ .Version }}`, so binaries report `v0.1.0` exactly as `go install` builds do.

- [x] **Step 3: Verify the guard rails without tagging**

```bash
make release VERSION=bad 2>&1 | tail -1; echo "exit $?"
touch scratch.tmp && make release VERSION=v9.9.9 2>&1 | tail -1; rm scratch.tmp
git stash list | wc -l
go run github.com/goreleaser/goreleaser/v2@latest check 2>&1 | tail -1
make help | grep release
```
Expected: "VERSION must look like v1.2.3" then "working tree is not clean"; goreleaser `check` passes; help lists `release`. Do not run `make release` with a valid version; the lead cuts the tag in Task 4.

- [x] **Step 4: Commit**

```bash
git add Makefile .goreleaser.yaml
git commit -m "Add make release target and report tags as versions"
```

---

## Task 3: Install and release documentation

**Agent:** `docs` (after Task 1 is committed)

**Files:**
- Modify: `README.md`, `docs/how-to/cut-a-release.md`, `docs/reference/jev-cli.md`, `docs/reference/makefile-and-repository-layout.md`, `docs/tutorials/jev-from-the-command-line.md`, `docs/tutorials/first-judgment-in-go.md`, `docs/tutorials/build-a-ticket-router.md`, `docs/reference/client-options-and-environment.md`, `docs/explanation/the-agent-facing-cli-contract.md` (only if it describes `version`)

- [x] **Step 1: README install section**

Replace the `## Install` section with the text in the spec's Documentation section. Keep the rest of the README.

- [x] **Step 2: Pages**

Run `make build` and `go build -o /tmp/jev-plain ./cmd/jev` first so both `version` outputs are real. Dispatch the how-to and reference writers with the plan's prose rubric and these facts, all verified against `Makefile`, `.goreleaser.yaml`, `.github/workflows/release.yml`, and `internal/version/version.go`:

- `cut-a-release.md`: semantic versioning, `v0.x` until the API settles; `make release VERSION=vX.Y.Z` and each check it performs in order; what the tag triggers (goreleaser, four archives, `checksums.txt`, git changelog); how to watch (`gh run watch`) and verify (`gh release view vX.Y.Z`, `go install github.com/therealbill/typesafe-go/cmd/jev@vX.Y.Z`, `jev version` showing `source: module`); what to do if the workflow fails (delete the tag locally and remotely, fix, retag). Do not create a tag while writing.
- `jev-cli.md`: the `version` subcommand's five fields and the four `source` values with what produces each; real output from both builds.
- `makefile-and-repository-layout.md`: the `release` target and its checks.
- `jev-from-the-command-line.md`: install via `go install github.com/therealbill/typesafe-go/cmd/jev@latest` as the primary path (state that it needs Go 1.25 or newer and that the binary lands in `$(go env GOPATH)/bin`), `make build` as the alternative; re-capture the `jev version` step's output from the `make build` binary.
- The two Go tutorials and the client-options reference: `go get github.com/therealbill/typesafe-go@latest` wherever the install line appears.

- [x] **Step 3: Validate and commit**

```bash
make docs && echo "docs ok"
git add README.md docs
git commit -m "Document install paths, version reporting, and the release flow"
```

---

## Task 4: Gate, push, cut v0.1.0, verify

**Agent:** lead (after Tasks 2 and 3)

- [ ] **Step 1: Gate and push**

```bash
make lint && make test && make docs && git status --short && echo "(clean)"
git push origin main
```

- [ ] **Step 2: Cut the release**

```bash
make release VERSION=v0.1.0
gh run watch $(gh run list --workflow release --limit 1 --json databaseId -q '.[0].databaseId') --exit-status
gh release view v0.1.0 --json assets -q '.assets[].name'
```
Expected: four `jev_v0.1.0_*.tar.gz` archives (or `jev_0.1.0_*` depending on goreleaser's archive template; report which) and `checksums.txt`.

- [ ] **Step 3: Verify the go install path**

```bash
tmp=$(mktemp -d) && GOBIN=$tmp GOFLAGS= go install github.com/therealbill/typesafe-go/cmd/jev@v0.1.0 && $tmp/jev version
GOBIN=$tmp GOFLAGS= go install github.com/therealbill/typesafe-go/cmd/jev@latest && $tmp/jev version
```
Expected: `"version":"v0.1.0"`, `"source":"module"` for both (until a newer tag exists). If the module proxy has not indexed the tag yet, wait a minute and retry with `GOPROXY=direct`.

- [ ] **Step 4: Docs deploy and plan**

The docs push in Step 1 triggers the site deploy; confirm it succeeded. Tick the plan, commit, push, report.
