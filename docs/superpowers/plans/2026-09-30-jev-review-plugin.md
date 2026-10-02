# jev-review Plugin and `jev review --dry-run` Implementation Plan

> **For agentic workers:** Execute this plan with an **Agent Team** of named agents (Agent tool with `name`, coordinated by the lead through SendMessage). Do NOT use git worktrees, do NOT use unnamed parallel subagents as the execution method, and do NOT use the `superpowers:executing-plans` skill. Roster and dependencies are in "Execution Model". Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a Claude Code plugin, installable from this repository as a marketplace, that writes the Markdown spec and `jev-review.json` for a codebase, and add `jev review --dry-run` so a config can be checked without an API key.

**Architecture:** `internal/review` gains a `DryRun` option that stops each unit after the spec section and file bundles are built, plus three byte-size fields on `UnitReport`; `jev review --dry-run` builds no client. The plugin lives in `plugins/jev-review/` with one skill (knowledge and procedure), one command (the workflow with a single approval stop), and one read-only agent (the codebase survey). `.claude-plugin/marketplace.json` at the repository root publishes it. `tools/checkplugin.sh` keeps the plugin's schema reference in step with the Go structs.

**Tech Stack:** Go 1.25, Cobra, the Claude Code plugin format (`marketplace.json`, `plugin.json`, `SKILL.md`, command and agent Markdown with YAML frontmatter), zsh, GNU Make, Hugo docs.

**Spec:** `docs/superpowers/specs/2026-09-30-jev-review-plugin-design.md`. Read it first.

---

## Execution Model

### Team roster

| Agent name | Type / model | Owns | Skills and subagents to use |
|---|---|---|---|
| `go` | `go-architect`, model **opus** | Tasks 1–3 (`internal/review` dry run, `jev review --dry-run`, the reference and how-to edits for it) | `superpowers:test-driven-development`, `superpowers:verification-before-completion` |
| `plugin` | `general-purpose`, model **opus** | Tasks 4–8 (marketplace, manifest, plugin README, skill and references, agent, command, `tools/checkplugin.sh`, Makefile, CI step) | `plugin-dev:plugin-structure`, `plugin-dev:skill-development`, `plugin-dev:command-development`, `plugin-dev:agent-development`; subagents `plugin-dev:skill-reviewer` and `plugin-dev:plugin-validator` for review; `shell-scripting-pro` for Task 8 |
| `docs` | `general-purpose`, model **sonnet** | Task 9 (the new how-to, the how-to index, README, the layout reference) | `diataxis-docs:doc-howto-writer`, `diataxis-docs:doc-crosslink-validator` |
| lead (this session) | none | Task 10 (gate, interactive hand-off, fixes, push) | none |

### Dependency graph

```
Task 1 → Task 2 → Task 3                               (go; starts at once)
Task 4 → Task 5 → Task 6 → Task 7 → Task 8             (plugin; starts at once)
Task 2 and Task 8 → Task 9                             (docs)
Tasks 3, 8, 9 → Task 10                                (lead)
```

`go` and `plugin` start together and touch disjoint files. `docs` starts when Task 2 (the built `--dry-run` binary) and Task 8 (`make plugin`, the CI step, the finished command) are both committed. At most three agents run at once, plus the lead.

### Rules for every agent

1. Work in `/Users/bill/Projects/gojev` on `main`. No worktrees, no other branches.
2. Touch only the files listed under your task.
3. Stage by explicit path only. Never `git add -A` or `git add .`. Retry once after two seconds on `index.lock`.
4. Commit with `git -c user.email=ucntcme@gmail.com -c user.name="Bill Anderson" commit -m "..."`. No "Authored by" lines. Do not push.
5. Prose rule for every page, README, SKILL.md, reference file, command, agent prompt, and doc comment: plain declaratives and imperatives; active voice; one topic per paragraph; no em dashes; no contrast-reveal sentences ("not X, but Y"); no narrated reasoning ("it's worth", "this matters because", "deliberately"); no humor or asides; no summary punchlines; no duration estimates. Before each commit run `grep -nE "—|it's worth|this matters|deliberately|In other words" <files>` and fix every hit.
6. `TYPESAFE_API_KEY` may be in the environment. Never print, log, or write it. No task in this plan runs `jev review` without `--dry-run`.
7. Scratch files go under `/private/tmp/claude-501/-Users-bill-Projects-gojev/a1bded06-2317-436a-b0a2-f4742aea0203/scratchpad/`. Call it `$SCRATCH` below.
8. Report after each task: task number, commit hash, verification tail, deviations.

---

## File map

| Path | Responsibility | Task |
|---|---|---|
| `internal/review/review.go`, `internal/review/review_test.go` | `DryRun` option, byte fields, nil-asker rule, dry-run Markdown | 1 |
| `internal/cli/review.go`, `internal/cli/review_test.go` | `--dry-run` flag, `runDryRun`, `flagged` helper | 2 |
| `docs/reference/jev-review.md`, `docs/reference/jev-cli.md`, `docs/how-to/review-your-codebase-with-jev.md` | dry-run documentation | 3 |
| `.claude-plugin/marketplace.json`, `plugins/jev-review/.claude-plugin/plugin.json`, `plugins/jev-review/README.md` | marketplace and manifest | 4 |
| `plugins/jev-review/skills/setting-up-jev-review/SKILL.md` and `references/*.md` | the skill | 5 |
| `plugins/jev-review/agents/repo-mapper.md` | the agent | 6 |
| `plugins/jev-review/commands/setup.md` | the command | 7 |
| `tools/checkplugin.sh`, `Makefile`, `.github/workflows/ci.yml` | plugin checks | 8 |
| `docs/how-to/build-a-jev-review-config-with-claude-code.md`, `docs/how-to/_index.md`, `README.md`, `docs/reference/makefile-and-repository-layout.md` | plugin documentation | 9 |

---

## Task 1: Dry run in `internal/review`

**Agent:** `go`

**Files:**
- Modify: `internal/review/review.go`, `internal/review/review_test.go`

Read `internal/review/review.go` in full first. The change: `Options.DryRun`, three size fields on `UnitReport`, `Report.DryRun`, `bundle` returning the pre-truncation size, `runUnit` returning before the call in dry-run mode, `Run` rejecting a nil asker outside dry-run mode, and a dry-run variant of `Markdown`.

- [x] **Step 1: Update `TestBundleTruncates` for the new `bundle` signature**

Replace the existing function in `internal/review/review_test.go`:

```go
func TestBundleTruncates(t *testing.T) {
	files := map[string]string{"a.go": strings.Repeat("x", 100), "b.go": strings.Repeat("y", 100)}
	text, size, truncated := bundle([]string{"a.go", "b.go"}, files, 150)
	// Each file is "// file: a.go\n" (14 bytes) + 100 + "\n\n".
	if !truncated || size != 232 || len(text) > 150+len(truncatedMarker)*2+40 {
		t.Fatalf("truncated=%v size=%d len=%d", truncated, size, len(text))
	}
	text, size, truncated = bundle([]string{"a.go"}, files, 1000)
	if truncated || size != len(text) || !strings.Contains(text, "// file: a.go") {
		t.Fatalf("truncated=%v size=%d text=%q", truncated, size, text)
	}
}
```

- [x] **Step 2: Append the dry-run tests to `internal/review/review_test.go`**

```go
// refuseAsker fails the test if the review calls it.
type refuseAsker struct{ t *testing.T }

func (r refuseAsker) SystemOne(context.Context, any, typesafe.Questions, ...typesafe.RequestOption) (*typesafe.SystemOneResponse, error) {
	r.t.Fatal("the asker must not be called in a dry run")
	return nil, nil
}

func dryRunOptions() Options {
	o := DefaultOptions()
	o.DryRun = true
	return o
}

func TestRunDryRunStopsBeforeTheCall(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("spec.md", "# S\n\n## Retry\n\nretry text\n\n## Other\n\nx\n")
	write("retry.go", strings.Repeat("a", 30))
	write("retry_test.go", strings.Repeat("b", 20))
	cfg := &Config{Spec: "spec.md", Units: []Unit{{Name: "retry", SpecHeading: "## Retry", Implementation: []string{"retry.go"}, Tests: []string{"retry_test.go"}, Behaviors: []string{"b0"}, Notes: []string{"n"}}}}
	rep, err := Run(context.Background(), cfg, dir, refuseAsker{t}, dryRunOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !rep.DryRun || len(rep.Units) != 1 {
		t.Fatalf("report %+v", rep)
	}
	u := rep.Units[0]
	if u.Failing || u.Error != "" || u.RequestID != "" || u.Behaviors != nil {
		t.Fatalf("a clean dry run carries no failure and no answers: %+v", u)
	}
	// The section is "## Retry\n\nretry text\n" (21 bytes). Each bundle is
	// "// file: <name>\n" + body + "\n\n": 18+30+2 and 23+20+2.
	if u.SpecBytes != 21 || u.ImplementationBytes != 50 || u.TestsBytes != 45 || u.Truncated {
		t.Fatalf("sizes: spec %d impl %d tests %d truncated %v", u.SpecBytes, u.ImplementationBytes, u.TestsBytes, u.Truncated)
	}
	if len(u.Notes) != 1 {
		t.Fatalf("notes must travel into the dry-run report: %+v", u)
	}
}

func TestRunDryRunAcceptsNilAskerAndRejectsItOtherwise(t *testing.T) {
	_, p := setupRepo(t)
	cfg, base, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), cfg, base, nil, dryRunOptions()); err != nil {
		t.Fatalf("a nil asker must be accepted in a dry run: %v", err)
	}
	if _, err := Run(context.Background(), cfg, base, nil, defaults()); err == nil {
		t.Fatal("a nil asker must be rejected outside a dry run")
	}
}

func TestRunDryRunRecordsUnitErrors(t *testing.T) {
	dir, _ := setupRepo(t)
	cfg := &Config{Spec: "spec.md", Units: []Unit{
		{Name: "heading", SpecHeading: "## Missing", Implementation: []string{"retry.go"}, Behaviors: []string{"b0"}},
		{Name: "file", SpecHeading: "## Retry", Implementation: []string{"nope.go"}, Behaviors: []string{"b0"}},
		{Name: "doc", Spec: "nope.md", SpecHeading: "## Retry", Implementation: []string{"retry.go"}, Behaviors: []string{"b0"}},
	}}
	rep, err := Run(context.Background(), cfg, dir, nil, dryRunOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Failing() {
		t.Fatal("unit errors must fail the dry run")
	}
	for i, want := range []string{`heading "## Missing" not found`, "nope.go", "nope.md"} {
		u := rep.Units[i]
		if !u.Failing || !strings.Contains(u.Error, want) {
			t.Fatalf("unit %s: %+v", u.Name, u)
		}
	}
}

func TestRunDryRunReportsSizesBeforeTruncation(t *testing.T) {
	dir, _ := setupRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "big.go"), []byte(strings.Repeat("x", 300)), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Spec: "spec.md", Units: []Unit{{Name: "big", SpecHeading: "## Retry", Implementation: []string{"big.go"}, Behaviors: []string{"b0"}}}}
	o := dryRunOptions()
	o.MaxStateBytes = 100
	rep, err := Run(context.Background(), cfg, dir, nil, o)
	if err != nil {
		t.Fatal(err)
	}
	u := rep.Units[0]
	// "// file: big.go\n" (16 bytes) + 300 + "\n\n".
	if !u.Truncated || u.ImplementationBytes != 318 || u.TestsBytes != 0 || u.Failing {
		t.Fatalf("%+v", u)
	}
}

func TestRunCarriesSizesIntoRealReports(t *testing.T) {
	_, p := setupRepo(t)
	cfg, base, _ := Load(p)
	fa := &fakeAsker{answers: answers([]float64{0.9}, 0.1, 2.5, 0.8, "none")}
	rep, err := Run(context.Background(), cfg, base, fa, defaults())
	if err != nil {
		t.Fatal(err)
	}
	u := rep.Units[0]
	if rep.DryRun || u.SpecBytes == 0 || u.ImplementationBytes == 0 || u.TestsBytes == 0 {
		t.Fatalf("sizes must be set on a real run: %+v", u)
	}
	fa.err = errors.New("boom")
	rep, _ = Run(context.Background(), cfg, base, fa, defaults())
	if rep.Units[0].ImplementationBytes == 0 {
		t.Fatalf("sizes must survive a call error: %+v", rep.Units[0])
	}
}

func TestDryRunMarkdown(t *testing.T) {
	rep := Report{DryRun: true, Units: []UnitReport{
		{Name: "retry", SpecBytes: 21, ImplementationBytes: 50, TestsBytes: 45},
		{Name: "big", SpecBytes: 21, ImplementationBytes: 318, Truncated: true},
		{Name: "cache", Error: `heading "## Eviction" not found in spec`, Failing: true},
	}}
	out := rep.Markdown(defaults())
	for _, want := range []string{
		"# jev review --dry-run\n",
		"budget: 50000 bytes each for implementation and tests\n",
		"| Unit | Spec bytes | Implementation bytes | Tests bytes | Truncated | Status |\n",
		"| retry | 21 | 50 | 45 | no | ok |\n",
		"| big | 21 | 318 | 0 | yes | ok |\n",
		"| cache | 0 | 0 | 0 | no | error |\n",
		"## big\n\nnote: sources were truncated to fit the state budget\n",
		"## cache\n\nerror: heading \"## Eviction\" not found in spec\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "## retry") {
		t.Fatalf("a clean unit gets no section:\n%s", out)
	}
}
```

- [x] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/review/ 2>&1 | head -20`
Expected: compile errors naming `o.DryRun`, `rep.DryRun`, `u.SpecBytes`, and `assignment mismatch: 3 variables but bundle returns 2 values`.

- [x] **Step 4: Implement in `internal/review/review.go`**

Replace `Options`:

```go
// Options controls thresholds, limits, and selection.
type Options struct {
	Only          string
	MaxStateBytes int
	MinCover      float64
	MaxContradict float64
	MinThorough   float64
	UnitTimeout   time.Duration
	// DryRun stops each unit after its spec section and file bundles are
	// built, before the API call. Run accepts a nil Asker when it is set.
	DryRun bool
}
```

Add three fields to `UnitReport`, after `Truncated`:

```go
	Truncated        bool             `json:"truncated"`
	// SpecBytes is the length of the extracted spec section.
	// ImplementationBytes and TestsBytes are the bundle sizes before
	// truncation, so a report shows how close a unit sits to the budget.
	SpecBytes           int              `json:"spec_bytes,omitempty"`
	ImplementationBytes int              `json:"implementation_bytes,omitempty"`
	TestsBytes          int              `json:"tests_bytes,omitempty"`
	Behaviors        []BehaviorResult `json:"behaviors"`
```

Replace `Report`:

```go
// Report is the outcome of a run. DryRun records that the units stopped
// before the API call; it is not part of the JSON document.
type Report struct {
	Units  []UnitReport
	DryRun bool
}
```

Replace `Run`:

```go
// Run evaluates every unit (or the one named by opts.Only) and returns the
// report. Errors reading a spec document, reading files, or finding a spec
// section, API failures, and timeouts are recorded on the unit; Run itself
// fails only when no unit matched or when asker is nil outside a dry run.
func Run(ctx context.Context, cfg *Config, base string, asker Asker, opts Options) (Report, error) {
	rep := Report{DryRun: opts.DryRun}
	if asker == nil && !opts.DryRun {
		return rep, errors.New("review: an Asker is required unless Options.DryRun is set")
	}
	specs := newSpecCache(base)
	for _, u := range cfg.Units {
		if opts.Only != "" && u.Name != opts.Only {
			continue
		}
		rep.Units = append(rep.Units, runUnit(ctx, u, specs, cfg.Spec, base, asker, opts))
	}
	if len(rep.Units) == 0 {
		return rep, ErrNoUnits
	}
	return rep, nil
}
```

In `runUnit`, replace the block from `half := opts.MaxStateBytes / 2` through the end of the function:

```go
	half := opts.MaxStateBytes / 2
	impl, implSize, t1 := bundle(u.Implementation, files, half)
	tests, testsSize, t2 := bundle(u.Tests, files, half)
	r.SpecBytes, r.ImplementationBytes, r.TestsBytes = len(section), implSize, testsSize
	r.Truncated = t1 || t2
	if opts.DryRun {
		return r
	}

	state := map[string]any{"spec": section, "implementation": impl, "tests": tests}
	questions := typesafe.Questions{}
	for k, q := range buildQuestions(u) {
		questions[k] = typesafe.RawQuestion(q)
	}
	uctx, cancel := context.WithTimeout(ctx, opts.UnitTimeout)
	defer cancel()
	res, err := asker.SystemOne(uctx, state, questions)
	if err != nil {
		if errors.Is(uctx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
			r.Error = fmt.Sprintf("the review call did not finish within %s", opts.UnitTimeout)
		} else {
			r.Error = err.Error()
			r.err = err
		}
		r.Failing = true
		return r
	}
	out := evaluate(u, res.Answers, opts)
	out.RequestID = res.RequestID
	out.Model = res.Model
	if res.Usage.InputTokens != nil {
		out.InputTokens = *res.Usage.InputTokens
	}
	out.Truncated = r.Truncated
	out.SpecBytes, out.ImplementationBytes, out.TestsBytes = r.SpecBytes, r.ImplementationBytes, r.TestsBytes
	return out
}
```

Replace `bundle`:

```go
// bundle concatenates files with a header line each, cutting the total to
// maxBytes and marking the cut. size is the length before any cut.
func bundle(paths []string, files map[string]string, maxBytes int) (text string, size int, truncated bool) {
	var b strings.Builder
	for _, p := range paths {
		b.WriteString("// file: " + filepath.ToSlash(p) + "\n")
		b.WriteString(files[p])
		b.WriteString("\n\n")
	}
	s := b.String()
	if len(s) <= maxBytes {
		return s, len(s), false
	}
	return s[:maxBytes] + truncatedMarker, len(s), true
}
```

At the top of `Markdown`, before `var b strings.Builder`, add:

```go
	if r.DryRun {
		return r.dryRunMarkdown(opts)
	}
```

Add after `Markdown`:

```go
// dryRunMarkdown renders the sizes and status of every unit in a dry run.
func (r Report) dryRunMarkdown(opts Options) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# jev review --dry-run\n\nbudget: %d bytes each for implementation and tests\n\n", opts.MaxStateBytes/2)
	b.WriteString("| Unit | Spec bytes | Implementation bytes | Tests bytes | Truncated | Status |\n|---|---|---|---|---|---|\n")
	yesNo := map[bool]string{true: "yes", false: "no"}
	for _, u := range r.Units {
		status := "ok"
		if u.Error != "" {
			status = "error"
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %s | %s |\n", u.Name, u.SpecBytes, u.ImplementationBytes, u.TestsBytes, yesNo[u.Truncated], status)
	}
	b.WriteString("\n")
	for _, u := range r.Units {
		switch {
		case u.Error != "":
			fmt.Fprintf(&b, "## %s\n\nerror: %s\n\n", u.Name, u.Error)
		case u.Truncated:
			fmt.Fprintf(&b, "## %s\n\nnote: sources were truncated to fit the state budget\n\n", u.Name)
		}
	}
	return b.String()
}
```

- [x] **Step 5: Run the package tests**

Run: `go test -race ./internal/review/`
Expected: `ok`.

- [x] **Step 6: Lint and commit**

```bash
gofmt -l internal/review; go vet ./internal/review/ && golangci-lint run ./internal/review/
git add internal/review/review.go internal/review/review_test.go
git -c user.email=ucntcme@gmail.com -c user.name="Bill Anderson" commit -m "Add a dry-run option and bundle sizes to the review package"
```
Expected: `gofmt -l` prints nothing; lint passes.

---

## Task 2: `jev review --dry-run`

**Agent:** `go` (after Task 1)

**Files:**
- Modify: `internal/cli/review.go`, `internal/cli/review_test.go`

Read `internal/cli/review.go` and `internal/cli/review_test.go` first. The `run` helper in `internal/cli/cli_test.go` clears `TYPESAFE_API_KEY`, so a test that passes no `--api-key` proves the dry run needs no key.

- [x] **Step 1: Append the tests to `internal/cli/review_test.go`**

Add `"errors"` to the import block, then append:

```go
func TestReviewDryRunNeedsNoKeyAndWritesNoReport(t *testing.T) {
	cfg, report := reviewRepo(t)
	code, out, errOut := run(t, "", "review", "--dry-run", "--units", cfg, "--report", report)
	if code != ExitOK {
		t.Fatalf("exit %d stdout %s stderr %s", code, out, errOut)
	}
	// spec section "## Retry\n\nretry text\n" is 21 bytes; retry.go is
	// "package x\n" (10 bytes) so its bundle is 18+10+2; the test bundle is 23+10+2.
	for _, want := range []string{"# jev review --dry-run", "budget: 50000 bytes", "| retry | 21 | 30 | 35 | no | ok |"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in stdout:\n%s", want, out)
		}
	}
	if !strings.Contains(errOut, "review: checked 1 units") {
		t.Fatalf("stderr %q", errOut)
	}
	if _, err := os.Stat(report); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a dry run must not write the report file: %v", err)
	}
}

func TestReviewDryRunMissingHeadingExits8(t *testing.T) {
	cfg, report := reviewRepo(t)
	bad := `{"spec":"spec.md","units":[{"name":"retry","spec_heading":"## Nope","implementation":["retry.go"],"tests":["retry_test.go"],"behaviors":["b0"]}]}`
	if err := os.WriteFile(cfg, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := run(t, "", "review", "--dry-run", "--units", cfg, "--report", report)
	if code != ExitFlagged {
		t.Fatalf("exit %d stdout %s stderr %s", code, out, errOut)
	}
	if !strings.Contains(out, "| retry | 0 | 0 | 0 | no | error |") || !strings.Contains(out, `error: heading "## Nope" not found in spec`) {
		t.Fatalf("stdout:\n%s", out)
	}
	if !strings.Contains(errOut, "review: 1 of 1 units failing") {
		t.Fatalf("stderr %q", errOut)
	}
}

func TestReviewDryRunJSON(t *testing.T) {
	cfg, report := reviewRepo(t)
	code, out, _ := run(t, "", "review", "--dry-run", "--json", "--units", cfg, "--report", report)
	var units []map[string]any
	if code != ExitOK || json.Unmarshal([]byte(out), &units) != nil || len(units) != 1 {
		t.Fatalf("exit %d out %s", code, out)
	}
	if units[0]["spec_bytes"] != 21.0 || units[0]["implementation_bytes"] != 30.0 || units[0]["tests_bytes"] != 35.0 {
		t.Fatalf("sizes missing: %v", units[0])
	}
	if _, ok := units[0]["request_id"]; ok {
		t.Fatalf("no request happens in a dry run: %v", units[0])
	}
}

func TestReviewDryRunOnlyNoMatchIsUsage(t *testing.T) {
	cfg, report := reviewRepo(t)
	code, out, _ := run(t, "", "review", "--dry-run", "--only", "nope", "--units", cfg, "--report", report)
	if code != ExitUsage || !strings.Contains(out, "no units matched") {
		t.Fatalf("exit %d out %s", code, out)
	}
}
```

- [x] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli/ -run DryRun 2>&1 | head`
Expected: each test fails with `unknown flag: --dry-run` in the output (exit 1, not the expected code).

- [x] **Step 3: Implement in `internal/cli/review.go`**

Add `"context"` to the import block. After the `--unit-timeout` flag line in `newReviewCmd`, add:

```go
	f.BoolVar(&o.opts.DryRun, "dry-run", false, "check the config, spec headings, and files without calling the API")
```

In `runReview`, right after the `review.Load` error check, add:

```go
	if o.opts.DryRun {
		return runDryRun(cmd.Context(), cfg, base, o, streams, g.pretty)
	}
```

Replace the tail of `runReview`, from `if rep.Failing() {` to the end of the function, with:

```go
	if rep.Failing() {
		return flagged(streams, rep)
	}
	return nil
}

// runDryRun checks every unit up to the API call and prints the result. It
// builds no client, so no API key is needed, and it writes no report file.
func runDryRun(ctx context.Context, cfg *review.Config, base string, o *reviewOptions, streams IO, pretty bool) error {
	rep, err := review.Run(ctx, cfg, base, nil, o.opts)
	if err != nil {
		return fail(streams, pretty, &usageError{err})
	}
	_, _ = fmt.Fprintf(streams.Err, "review: checked %d units\n", len(rep.Units))
	if o.json {
		out, err := rep.JSON()
		if err != nil {
			return fail(streams, pretty, err)
		}
		_, _ = streams.Out.Write(out)
		_, _ = fmt.Fprintln(streams.Out)
	} else {
		_, _ = fmt.Fprint(streams.Out, rep.Markdown(o.opts))
	}
	if rep.Failing() {
		return flagged(streams, rep)
	}
	return nil
}

// flagged prints the failing count and returns the exit-8 error.
func flagged(streams IO, rep review.Report) error {
	failing := 0
	for _, u := range rep.Units {
		if u.Failing {
			failing++
		}
	}
	_, _ = fmt.Fprintf(streams.Err, "review: %d of %d units failing\n", failing, len(rep.Units))
	return &ExitError{Code: ExitFlagged, Kind: "flagged", Err: errors.New("review flagged at least one unit")}
}
```

- [x] **Step 4: Run the CLI tests**

Run: `go test -race ./internal/cli/`
Expected: `ok`.

- [x] **Step 5: Build and check the binary by hand**

```bash
make build
./bin/jev review --dry-run; echo "exit $?"
env -u TYPESAFE_API_KEY ./bin/jev review --dry-run --json | jq 'map({name, spec_bytes, implementation_bytes, tests_bytes, truncated})'
ls jev-review-report.json 2>/dev/null || echo "no report file"
```
Expected: a table with eight `ok` rows and exit 0; the JSON parses with non-zero sizes; no report file was written by the dry run (delete a stale one from an earlier real run first: `rm -f jev-review-report.json`).

- [x] **Step 6: Lint and commit**

```bash
make lint
git add internal/cli/review.go internal/cli/review_test.go
git -c user.email=ucntcme@gmail.com -c user.name="Bill Anderson" commit -m "Add jev review --dry-run"
```

---

## Task 3: Document the dry run

**Agent:** `go` (after Task 2)

**Files:**
- Modify: `docs/reference/jev-review.md`, `docs/reference/jev-cli.md`, `docs/how-to/review-your-codebase-with-jev.md`

Every transcript on these pages is real output from `./bin/jev` built from Task 2. Follow the prose rule in "Rules for every agent".

- [ ] **Step 1: Build a scratch project for the transcripts**

Create `$SCRATCH/dryrun-demo/` with these files.

`spec.md`:

```markdown
# Widgets

## Retry policy

A failed publish is retried up to three times. The delay between retries is a fixed 100ms and does not grow. The last error is returned after all retries are exhausted.

## Validation

An empty widget name is rejected before any publish attempt is made. A widget name longer than 64 bytes is rejected.
```

`retry.go`:

```go
package widgets

import "time"

const maxRetries = 3

func publish(send func() error) error {
	var err error
	for i := 0; i <= maxRetries; i++ {
		if err = send(); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}
```

`retry_test.go`:

```go
package widgets

import (
	"errors"
	"testing"
)

func TestPublishRetriesThreeTimes(t *testing.T) {
	calls := 0
	err := publish(func() error { calls++; return errors.New("no") })
	if err == nil || calls != 4 {
		t.Fatalf("err %v calls %d", err, calls)
	}
}
```

`validation.go`:

```go
package widgets

import "errors"

func validateName(name string) error {
	if name == "" {
		return errors.New("empty name")
	}
	if len(name) > 64 {
		return errors.New("name too long")
	}
	return nil
}
```

`validation_test.go`:

```go
package widgets

import "testing"

func TestEmptyNameRejected(t *testing.T) {
	if validateName("") == nil {
		t.Fatal("empty name accepted")
	}
}
```

`jev-review.json`:

```json
{
  "spec": "spec.md",
  "units": [
    {
      "name": "retry",
      "spec_heading": "## Retry policy",
      "implementation": ["retry.go"],
      "tests": ["retry_test.go"],
      "behaviors": [
        "A failed publish is retried up to three times",
        "The delay between retries is a fixed 100ms and does not grow",
        "The last error is returned after all retries are exhausted"
      ]
    },
    {
      "name": "validation",
      "spec_heading": "## Validation",
      "implementation": ["validation.go"],
      "tests": ["validation_test.go"],
      "behaviors": [
        "An empty widget name is rejected before any publish attempt is made",
        "A widget name longer than 64 bytes is rejected"
      ]
    }
  ]
}
```

Capture, from the repository root:

```bash
./bin/jev review --help > $SCRATCH/review-help.txt
(cd $SCRATCH/dryrun-demo && /Users/bill/Projects/gojev/bin/jev review --dry-run; echo "exit $?") > $SCRATCH/dryrun-ok.txt 2>&1
sed -i '' 's/## Validation"/## Validating"/' $SCRATCH/dryrun-demo/jev-review.json
(cd $SCRATCH/dryrun-demo && /Users/bill/Projects/gojev/bin/jev review --dry-run; echo "exit $?") > $SCRATCH/dryrun-bad.txt 2>&1
sed -i '' 's/## Validating"/## Validation"/' $SCRATCH/dryrun-demo/jev-review.json
cat $SCRATCH/dryrun-ok.txt $SCRATCH/dryrun-bad.txt
```
Expected: the first run prints two `ok` rows and `exit 0`; the second prints an `error` row for `validation`, a `## validation` section with `error: heading "## Validating" not found in spec`, `review: 1 of 2 units failing` on stderr, and `exit 8`.

- [ ] **Step 2: Edit `docs/reference/jev-review.md`**

Make these changes, in this order:

1. Synopsis: change the first line to `jev review [--units jev-review.json] [--only NAME] [--report jev-review-report.json]` followed on the next lines by `[--json] [--dry-run] [--min-cover 0.6] ...` (add `[--dry-run]` after `[--json]`).
2. Replace the fenced `./bin/jev review --help` block under `## jev review` with the contents of `$SCRATCH/review-help.txt`.
3. In the `### Flags` table under `## jev review`, add as the first row: `| \`--dry-run\` | bool | \`false\` | check the config, spec headings, and files without calling the API |`.
4. After the `## jev review init` section and before `## Config schema`, add this section. Substitute the real transcripts.

```markdown
## `jev review --dry-run`

Source: `internal/cli/review.go`, `runDryRun`, and `internal/review/review.go`, `runUnit`.

With `--dry-run`, `jev review` loads the config, then for each unit reads its spec document, extracts the section named by `spec_heading`, reads every `implementation` and `tests` file, and builds both bundles. It stops there. No client is built, no request is sent, and no API key is needed. `--report` is ignored and no report file is written. `--only` and `--json` work as they do without `--dry-run`. The global flags are accepted and unused.

The Markdown summary is a different table from the one a full run prints:

| Column | Content |
|---|---|
| Unit | The unit's `name`. |
| Spec bytes | Length of the extracted section. |
| Implementation bytes | Size of the implementation bundle before truncation. |
| Tests bytes | Size of the tests bundle before truncation. |
| Truncated | `yes` when either bundle was cut to fit half of `--max-state-bytes`. |
| Status | `ok`, or `error` when the spec document, heading, or a file could not be read. |

A `budget:` line above the table states the per-bundle limit. Below the table, each errored unit gets a `## <unit>` section with `error: <message>`, and each truncated unit gets one with `note: sources were truncated to fit the state budget`.

Verified against the built binary, on a two-unit scratch project:

<paste $SCRATCH/dryrun-ok.txt as a fenced block, with a `$ jev review --dry-run` first line>

The same config with the second unit's heading changed to `## Validating`, which does not appear in the spec:

<paste $SCRATCH/dryrun-bad.txt as a fenced block>

Exit codes with `--dry-run`: 0 when every unit resolved, 8 when any unit errored (`kind` `flagged`, with `review: k of n units failing` on stderr), 1 when the config is missing or invalid or `--only` matched nothing. Codes 3 to 7 cannot occur.
```

5. In `## Report JSON schema`, add the three fields to the Go struct after `Truncated`, with the same comment as in the source, and add three rows to the field table after the `truncated` row:

```markdown
| `spec_bytes` | int | omitempty | Length in bytes of the extracted spec section. |
| `implementation_bytes` | int | omitempty | Size in bytes of the implementation bundle before truncation. |
| `tests_bytes` | int | omitempty | Size in bytes of the tests bundle before truncation; absent when `tests` is empty. |
```

Then add, after the table: "In a dry run, `request_id`, `model`, `input_tokens`, `behaviors`, and the answer fields are absent or zero and `flags` is `null`; the three byte fields, `truncated`, `notes`, `failing`, and `error` are set as in a full run."

6. In `## Exit codes`, after the table, add: "With `--dry-run`, only 0, 1, and 8 occur; see [`jev review --dry-run`](#jev-review---dry-run)."

- [ ] **Step 3: Edit `docs/reference/jev-cli.md`**

Replace the fenced `./bin/jev review --help` block under `## jev review` with the contents of `$SCRATCH/review-help.txt`. In the paragraph that follows it, after the sentence about `jev review init`, add: "`jev review --dry-run` checks the config, spec headings, and files without building a client, so it needs no API key; its table and exit codes are on the [jev review reference](./jev-review.md#jev-review---dry-run)."

- [ ] **Step 4: Edit `docs/how-to/review-your-codebase-with-jev.md`**

Insert a new step after `### 4. Write behaviors as concrete, testable claims` and renumber the later steps 5 through 9 to 6 through 10, including the sentence in "Verify it works" that says "on the fixed scratch example from step 7" (it becomes step 8). Then run `grep -rn 'review-your-codebase-with-jev.md#' docs README.md` and fix any link whose anchor number changed; at the time of writing the only anchor link is to step 3, which does not move.

The new step:

```markdown
### 5. Check the config with `--dry-run`

`jev review --dry-run` loads the config, extracts every unit's spec section, and reads and bundles every listed file, then stops before the API call. It needs no API key and sends nothing. A heading that does not match a line in the spec, or a file that does not exist, shows up here instead of in a paid run.

<paste $SCRATCH/dryrun-bad.txt as a fenced block, with a `$ jev review --dry-run` first line>

Fix the heading text in the config or the spec and rerun until the exit code is 0:

<paste $SCRATCH/dryrun-ok.txt as a fenced block>

Each row shows the extracted section's size and each bundle's size before truncation, against the budget line above the table. A `yes` under Truncated means the bundle was cut; split the unit or raise `--max-state-bytes`.
```

- [ ] **Step 5: Check and commit**

```bash
./tools/checkdocs.sh && grep -nE "—|it's worth|this matters|deliberately" docs/reference/jev-review.md docs/reference/jev-cli.md docs/how-to/review-your-codebase-with-jev.md; echo "grep exit $?"
git add docs/reference/jev-review.md docs/reference/jev-cli.md docs/how-to/review-your-codebase-with-jev.md
git -c user.email=ucntcme@gmail.com -c user.name="Bill Anderson" commit -m "Document jev review --dry-run"
```
Expected: `checkdocs.sh` prints nothing; the grep exits 1 (no matches).

---

## Task 4: Marketplace, plugin manifest, plugin README

**Agent:** `plugin` (starts at once)

**Files:**
- Create: `.claude-plugin/marketplace.json`, `plugins/jev-review/.claude-plugin/plugin.json`, `plugins/jev-review/README.md`

Load `plugin-dev:plugin-structure` before starting. `claude plugin validate` is on this machine; `claude plugin validate .` from the repository root validates the marketplace manifest and `claude plugin validate --strict plugins/jev-review` validates the plugin. `--strict` turns warnings into errors, and the manifests below carry the fields that avoid every warning (marketplace `description`, plugin `author`).

- [x] **Step 1: Write `.claude-plugin/marketplace.json`**

```json
{
  "name": "typesafe-go",
  "description": "Claude Code plugins for the typesafe-go SDK and the jev command-line tool.",
  "owner": {
    "name": "Bill"
  },
  "plugins": [
    {
      "name": "jev-review",
      "source": "./plugins/jev-review",
      "description": "Write the spec and jev-review.json that jev review needs, from your codebase.",
      "version": "0.1.0",
      "keywords": ["jev", "typesafe", "review", "spec", "tests"]
    }
  ]
}
```

- [x] **Step 2: Write `plugins/jev-review/.claude-plugin/plugin.json`**

```json
{
  "name": "jev-review",
  "version": "0.1.0",
  "description": "Write the spec and jev-review.json that jev review needs, from your codebase.",
  "author": {
    "name": "Bill"
  },
  "homepage": "https://therealbill.github.io/typesafe-go/docs/how-to/build-a-jev-review-config-with-claude-code/",
  "repository": "https://github.com/therealbill/typesafe-go",
  "license": "BSD-3-Clause",
  "keywords": ["jev", "typesafe", "review", "spec", "tests"]
}
```

- [x] **Step 3: Write `plugins/jev-review/README.md`**

```markdown
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
```

- [x] **Step 4: Validate and commit**

```bash
claude plugin validate --strict . && claude plugin validate --strict plugins/jev-review
git add .claude-plugin/marketplace.json plugins/jev-review/.claude-plugin/plugin.json plugins/jev-review/README.md
git -c user.email=ucntcme@gmail.com -c user.name="Bill Anderson" commit -m "Add the typesafe-go marketplace and the jev-review plugin manifest"
```
Expected: both validations print `Validation passed`.

---

## Task 5: The skill and its references

**Agent:** `plugin` (after Task 4)

**Files:**
- Create: `plugins/jev-review/skills/setting-up-jev-review/SKILL.md`, and under `plugins/jev-review/skills/setting-up-jev-review/references/`: `writing-the-spec.md`, `pairing-files.md`, `writing-behaviors.md`, `config-schema.md`, `language-conventions.md`

Load `plugin-dev:skill-development` first. Each file below is complete; write it as given, then have `plugin-dev:skill-reviewer` review the skill and apply its findings only where they keep the prose rule and do not remove content. The reference files are written for an agent: short, imperative, no narrative. `config-schema.md` has a constraint the checker in Task 8 relies on: the only table rows that start with a backticked lowercase name are the rows of the two field tables.

- [ ] **Step 1: Write `SKILL.md`**

```markdown
---
name: setting-up-jev-review
description: Use when a request mentions jev review, jev-review.json, spec_heading, writing behaviors for a review unit, writing or fixing a Markdown spec for jev review, or reviewing a codebase with Jev. Covers the spec rules, file pairing, behavior writing, the config schema, the dry run, and triage after a run.
---

# Setting up jev review

`jev review` judges whether your tests exercise the behaviors you listed and whether your implementation contradicts the spec section you named. It sends those files to Jev and answers with probabilities. It does not find bugs, review style, check security, or verify that the code is correct, and its readings drift between runs on the same input.

Two files drive it. The spec is a Markdown document with one heading per concern and checkable sentences under each. `jev-review.json` names the spec, pairs each heading with implementation and test files, and lists the behaviors to check.

## Procedure

`/jev-review:setup` runs the full workflow. When the user asks in their own words, follow the same steps:

1. Confirm `jev` is on `PATH` and `jev review --help` lists `--dry-run`. Without it, give the install routes from `references/config-schema.md` and stop.
2. Find the spec, or decide to write one. Read `references/writing-the-spec.md`.
3. Map each spec section to the files that implement it and the files that test it. Read `references/pairing-files.md` and `references/language-conventions.md`. For a repository with more than a handful of files, dispatch the `jev-review:repo-mapper` agent and use its proposal.
4. Show the proposed unit map and get approval before writing anything.
5. Write or edit the spec. Add headings and requirement sentences only for approved units. Edit an existing spec in place; never reorder or rewrite it.
6. Write `jev-review.json`. Read `references/config-schema.md` and `references/writing-behaviors.md`.
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
- Paths in the config resolve relative to the config file, not the working directory.
- `spec_heading` matches one heading line exactly, `#` characters included.
```

- [ ] **Step 2: Write `references/writing-the-spec.md`**

```markdown
# Writing a spec jev review can use

How `jev review` reads a spec and how to write one it can judge.

## How a section is extracted

- A unit's `spec_heading` must match one heading line character for character, `#` characters included: `"## Retry policy"`, not `"Retry policy"`. No trailing space.
- The section runs from that line through the line before the next heading of the same or higher level. Headings inside fenced code blocks are ignored.
- A heading one level too deep or too shallow misses the match and the unit errors.

## Structure

- One heading per unit-sized concern. Put sub-concerns under it at the next level.
- Keep sibling headings at the same level. Do not repeat part of a requirement one level down.
- Size a section to what one unit's files implement. Past about 50000 bytes of implementation or of tests, split the section with sub-headings and give each its own unit.

A good tree:

```markdown
## Client

### Retry policy

Retries are attempted up to MaxRetries (default 2) on 408, 429, and 5xx.

### Timeouts

The default per-attempt timeout is 10s.

## Cache

### Eviction

The least recently used entry is evicted first.
```

`spec_heading: "### Retry policy"` extracts the retry section alone. `"## Client"` extracts retry policy and timeouts together.

## Sentences

- Write plain declaratives. Each sentence states a fact the code either matches or contradicts: a default, a limit, a status-code list, an error type, a field name, an ordering, a failure behavior.
- Drop "should", "sensibly", "appropriately", "gracefully", "correctly", and every other word that describes an impression.
- Leave out marketing language and design rationale. They have no truth value to check.
- Keep each requirement in prose. A fenced code block illustrates a requirement; it does not state one.

Vague: "The client retries transient failures sensibly."

Concrete: "Retries are attempted up to MaxRetries (default 2) on 408, 429, and 5xx. A 422 is never retried. Retry-After is honored up to MaxRetryAfter (default 5m)."

## Three routes to a spec

- From READMEs or design docs: rewrite each claim as a declarative. "Handles errors gracefully" becomes which errors and what happens to them.
- From code and tests: write down what the code does as declaratives and mark anything that looks wrong. After a full run, a `contradicts_spec` flag on such a section means the sentence describes what the code does today and that differs from what it should do.
- From a design spec written before the code: a contradiction found here is a real candidate for a bug or a stale requirement.

## New-spec skeleton

```markdown
# <Project> specification

## <Concern>

<One declarative sentence per requirement.>

## <Concern>

<One declarative sentence per requirement.>
```

## Editing an existing spec

- Add a missing heading at the level of its siblings, at the end of the nearest parent section.
- Reword a vague sentence in place. Do not move, reorder, or delete existing text.
- Leave a heading with no code behind it alone, and do not make a unit for it.

## Keep spec and config in step

Each requirement sentence maps to one behavior line in the unit that reviews it. When a sentence changes, change the behavior and rerun that unit with `--only`.

Source: https://therealbill.github.io/typesafe-go/docs/how-to/write-a-spec-jev-review-can-use/
```

- [ ] **Step 3: Write `references/pairing-files.md`**

```markdown
# Pairing spec sections with files

How to choose a unit's `implementation` and `tests`. Commands per language are in `language-conventions.md`.

## Procedure

1. Read the section. List every function, type, method, option, constant, and field it names.
2. Find the non-test files that define those symbols. Those are `implementation`.
3. Find the test files that reference those symbols and assert on their behavior. Those are `tests`.
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

- A unit at 0 of N behaviors with a real test file present is a mis-pairing, not missing tests. Fix the pairing first.
- A unit whose `thoroughness` jumps when a test file is added was missing that file.
- Use `--only NAME` while fixing one unit.

Sources:

- https://therealbill.github.io/typesafe-go/docs/how-to/review-your-codebase-with-jev/
- https://therealbill.github.io/typesafe-go/docs/explanation/what-jev-review-measures/
```

- [ ] **Step 4: Write `references/writing-behaviors.md`**

```markdown
# Writing behaviors

How to fill a unit's `behaviors` list.

## Rules

- One behavior per requirement sentence in the section. Five to fifteen per unit.
- Each names one checkable outcome a single test could assert: a count, an error, a value, an order, an absence.
- Write it as a declarative about the code, in the spec's own terms.
- Under five: the section is thin. Add requirement sentences to the spec first, or merge the unit with a sibling.
- Over fifteen: split the section into sub-headings and give each its own unit.
- Keep the text stable. `accepted` entries name a behavior by its exact text; editing the text drops the acceptance.

## Good

- "A failed publish is retried up to three times": a test can assert the call count.
- "An empty widget name is rejected before any publish attempt is made": a test can assert an error and zero calls.
- "retry-after-ms takes precedence over Retry-After": a test can set both headers and assert the wait.

## Bad

- "Retries work correctly": nothing to assert.
- "The code handles errors well": no outcome named.
- "Validation": a topic, not a claim.

## Deriving behaviors from a section

Section:

```markdown
### Retry policy

Retries are attempted up to MaxRetries (default 2) on 408, 429, and 5xx. A 422 is never retried. Retry-After is honored up to MaxRetryAfter (default 5m).
```

Behaviors:

```json
[
  "A 408, 429, or 5xx response is retried",
  "Retries stop after MaxRetries and the last error is returned",
  "MaxRetries defaults to 2 when unset",
  "A 422 response is not retried",
  "A Retry-After header sets the delay before the next attempt",
  "A Retry-After longer than MaxRetryAfter is capped at MaxRetryAfter"
]
```

## Acceptances

- After a full run, a flag reviewed and found to be noise goes into `accepted`, with a note in `notes` recording the readings and the review.
- An acceptance is the behavior's exact text, or `contradicts_spec`, `thoroughness`, or `weakest_area`. Never `covers_NN`; the loader rejects it.
- `acceptance did not fire: <name>` in a report means the acceptance is stale. Remove it.

Source: https://therealbill.github.io/typesafe-go/docs/how-to/review-your-codebase-with-jev/
```

- [ ] **Step 5: Write `references/config-schema.md`**

```markdown
# jev-review.json

The config `jev review` reads. The two field tables are checked against the Go structs by `tools/checkplugin.sh` in the typesafe-go repository: keep every field on a row that starts with its backticked name, and start no other table row that way.

## Top level

| Field | Type | Required | Meaning |
|---|---|---|---|
| `description` | string | no | Free text. |
| `spec` | string | unless every unit sets its own | Path to the spec file, relative to this config file. |
| `units` | array | yes, at least one | The units below. |

## Unit

| Field | Type | Required | Meaning |
|---|---|---|---|
| `name` | string | yes, unique | Names the unit in the report and in `--only`. |
| `spec` | string | no | Spec file for this unit only; overrides the top-level one. Relative to the config file. |
| `spec_heading` | string | yes | A heading line matched exactly, `#` characters included: `"## Retry policy"`. |
| `implementation` | array of string | yes, at least one | Files that implement the section. |
| `tests` | array of string | no | Files that test it. Empty is allowed and reads as untested. |
| `behaviors` | array of string | yes, at least one | One checkable claim per entry. They become `covers_00`, `covers_01`, and so on, in order. |
| `accepted` | array of string | no | Flags reviewed and allowed: a behavior's exact text, or `contradicts_spec`, `thoroughness`, `weakest_area`. A `covers_NN` entry is rejected at load. |
| `notes` | array of string | no | Why each acceptance was granted. Copied into the report. |

## Paths

Every path, including `spec`, resolves relative to the directory containing the config file, from wherever `jev review` runs. An absolute path is used as is. Use forward slashes.

## Starter

`jev review init` writes this file and refuses to overwrite an existing one:

```json
{
  "description": "Units for jev review. Paths are relative to this file. Each unit names one spec section, the files that implement it, the files that test it, and the behaviors the section requires, one concrete claim per line. A unit may set its own \"spec\" to override the top-level one.",
  "spec": "docs/design.md",
  "units": [
    {
      "name": "example",
      "spec_heading": "## Retry policy",
      "implementation": ["retry.go"],
      "tests": ["retry_test.go"],
      "behaviors": [
        "A 429 response is retried after the Retry-After delay",
        "Retries stop after MaxRetries and the last error is returned"
      ],
      "accepted": [],
      "notes": []
    }
  ]
}
```

## Checking a config

`jev review --dry-run --units <path>` loads the config, extracts every unit's section, reads and bundles every file, and stops before the API call. No key, no network, no report file. Exit 0 when every unit resolved; 8 when a heading or file was missing, with the unit's error under the table; 1 when the config is invalid. Add `--json` for the report as JSON.

## Running the review

`jev review --units <path>` needs `TYPESAFE_API_KEY` and sends the spec section and every listed file to the TypeSafe API. Exit 0 clean, 8 flagged, 1 bad config, 3 to 7 API or transport failure. It writes `jev-review-report.json` next to where it runs; add that file to `.gitignore`.

Flags: `--only NAME`, `--json`, `--report PATH`, `--min-cover 0.6`, `--max-contradict 0.4`, `--min-thorough 2`, `--max-state-bytes 100000`, `--unit-timeout 2m`.

## Installing jev

- `go install github.com/therealbill/typesafe-go/cmd/jev@latest`, with Go 1.25 or newer.
- A binary from https://github.com/therealbill/typesafe-go/releases.
- `--dry-run` arrived in the first release after v0.1.1. Check with `jev review --help`.

Source: https://therealbill.github.io/typesafe-go/docs/reference/jev-review/
```

- [ ] **Step 6: Write `references/language-conventions.md`**

```markdown
# Language conventions for pairing

Where tests live and how to find definitions and references, per language. Use with `pairing-files.md`. Every command is read-only. Replace `Symbol` with the name from the spec.

## Any language

- Inventory: `git ls-files`
- Tests referencing a symbol: `grep -rln --exclude-dir=.git 'Symbol' <test locations>`
- Sizes: `wc -c <files>`
- Read one existing test before searching, to learn the naming the repository uses.

## Go

- Layout: one package per directory. Tests are `_test.go` siblings in the same directory. A file declaring `package foo_test` is an external test package for `foo`.
- Files per package: `go list -f '{{.Dir}}: {{.GoFiles}} | {{.TestGoFiles}} {{.XTestGoFiles}}' ./...`
- Definition of a symbol: `grep -rn --include='*.go' --exclude='*_test.go' -E '^func (\([^)]*\) )?Symbol\(|^type Symbol\b|^\s*Symbol\s+=' .`
- Tests referencing it: `grep -rln --include='*_test.go' '\bSymbol\b' .`
- A method's receiver type names the file to look in first. Behaviors exercised through the client (retries, cancellation, budgets) are often asserted in the client's tests, not the helper's.
- Sizes for a package: `wc -c $(go list -f '{{range .GoFiles}}{{$.Dir}}/{{.}} {{end}}' ./pkg)`

## Python

- Tests: `tests/`, `test_*.py`, `*_test.py`; pytest.
- Definition: `grep -rn --include='*.py' -E '^\s*(def|class) Symbol\b' .`
- Tests referencing it: `grep -rln --include='test_*.py' --include='*_test.py' '\bSymbol\b' .`
- The import lines at the top of a test file name the modules it exercises.

## TypeScript and JavaScript

- Tests: `*.test.ts`, `*.spec.ts`, `__tests__/`, `test/`; Jest, Vitest, Mocha.
- Definition: `grep -rn --include='*.ts' --include='*.tsx' --include='*.js' -E '(export )?(default )?(async )?(function|class|const|let|interface|type|enum) Symbol\b' .`
- Tests referencing it: `grep -rln --include='*.test.ts' --include='*.spec.ts' --include='*.test.js' '\bSymbol\b' .`
- The `import` lines in a test name the modules under test.

## Rust

- Unit tests live in the implementation file under `#[cfg(test)] mod tests`. List that file under both `implementation` and `tests`.
- Integration tests: `tests/*.rs`, which use the crate's public API.
- Definition: `grep -rn --include='*.rs' -E '^\s*(pub(\([a-z]+\))? )?(fn|struct|enum|trait|type|const) Symbol\b' src`
- Test files referencing it: `grep -rl --include='*.rs' -E '#\[(test|tokio::test)\]' . | xargs grep -l '\bSymbol\b'`

## Other languages

Apply the "Any language" commands with the test naming the repository uses.
```

- [ ] **Step 7: Review, validate, commit**

Dispatch `plugin-dev:skill-reviewer` on `plugins/jev-review/skills/setting-up-jev-review/` and apply findings that keep the prose rule and the content. Then:

```bash
wc -l plugins/jev-review/skills/setting-up-jev-review/SKILL.md
claude plugin validate --strict plugins/jev-review
grep -rnE "—|it's worth|this matters|deliberately" plugins/jev-review/skills; echo "grep exit $?"
git add plugins/jev-review/skills
git -c user.email=ucntcme@gmail.com -c user.name="Bill Anderson" commit -m "Add the setting-up-jev-review skill"
```
Expected: SKILL.md is under 120 lines; validation passes; the grep exits 1.

---

## Task 6: The `repo-mapper` agent

**Agent:** `plugin` (after Task 5)

**Files:**
- Create: `plugins/jev-review/agents/repo-mapper.md`

Load `plugin-dev:agent-development` first. The agent reads only. Its final message ends with one fenced JSON block whose shape the command in Task 7 parses.

- [ ] **Step 1: Write `plugins/jev-review/agents/repo-mapper.md`**

```markdown
---
name: repo-mapper
description: |
  Use this agent to propose jev review units for a repository: which spec sections exist or are needed, which files implement each one, which files test it, and how big each bundle is. Trigger from /jev-review:setup, or when the user asks to "map spec sections to files", "pair tests with the spec for jev review", or "propose units for jev-review.json". It reads only and writes nothing.

  <example>
  Context: The setup command needs a unit map before writing anything.
  user: "/jev-review:setup docs/design.md"
  assistant: "I'll dispatch the repo-mapper agent to survey the repository and propose units."
  <commentary>
  The command maps before it writes, and the map comes from this agent.
  </commentary>
  </example>

  <example>
  Context: The user has a config and doubts one pairing.
  user: "Which test files actually exercise the tracker? jev review says 0 of 6."
  assistant: "I'll use the repo-mapper agent to find the tests that reference the tracker's symbols."
  <commentary>
  A 0-of-N reading with real tests present is a pairing question, which this agent answers with evidence.
  </commentary>
  </example>
model: inherit
color: cyan
tools: ["Read", "Grep", "Glob", "Bash"]
---

You survey a repository and propose the units for `jev review`. You read files and run read-only commands. You never create, edit, or delete anything, and you never run `jev review` without `--dry-run`.

## Input

The dispatch prompt gives you:

- `root`: the repository root. Stay inside it.
- `spec`: `none`, one path, or several candidate paths.
- `mode`: `new` (no spec exists) or `existing`.

## Before you start

Read these two files:

- `${CLAUDE_PLUGIN_ROOT}/skills/setting-up-jev-review/references/pairing-files.md`
- `${CLAUDE_PLUGIN_ROOT}/skills/setting-up-jev-review/references/language-conventions.md`

## Procedure

1. Inventory with `git ls-files` under `root`. Detect the main language from file extensions and the test convention from where test files live. Read one test file to confirm the convention.
2. Spec. With several candidates, read each one's headings and pick the one whose headings name things the code defines; list the others in `other_candidates`. With one, use it. With `none`, skip to step 4. For the chosen spec, list every heading with its line number and level. Put headings with no code behind them in `unmatched_headings`. Put sentences with nothing checkable in them (no default, limit, error, field, order, or failure named) in `vague_sentences`, at most twenty.
3. For each heading at the unit level (the level at which each heading names one concern with code behind it), build a unit: list the symbols the section names, find the non-test files that define them, find the test files that reference them, and record the evidence.
4. With no spec, build units from the code: one per package, module, or file group that has its own tests, named after the concern it implements. Propose a heading for each as `## <Concern>` with `heading_exists` false.
5. Measure with `wc -c`. Report each bundle's size as the sum of its files' bytes. `jev review --dry-run` reports the exact bundle size later.
6. Keep unit names short, lowercase, and unique: `retry`, `cache-eviction`.

## Output

End your reply with exactly one fenced `json` block in this shape, then a summary of at most ten lines naming weak pairings and anything the user must decide:

```json
{
  "language": "go",
  "test_convention": "_test.go siblings; client_test.go is an external test package",
  "spec": {
    "path": "docs/design.md",
    "other_candidates": [],
    "headings": [{"line": 12, "level": 2, "text": "## Retry policy"}],
    "unmatched_headings": ["## Roadmap"],
    "vague_sentences": [{"line": 40, "text": "The client retries sensibly."}]
  },
  "units": [
    {
      "name": "retry",
      "heading": "## Retry policy",
      "heading_exists": true,
      "concern": "Backoff, jitter, Retry-After, and the retry budget",
      "implementation": ["retry.go", "transport.go"],
      "tests": ["retry_test.go", "client_test.go"],
      "symbols": ["RetryPolicy", "backoff", "parseRetryAfter"],
      "implementation_bytes": 11520,
      "tests_bytes": 34110,
      "evidence": "client_test.go asserts on retry-after-ms and the budget; retry_test.go covers backoff and jitter"
    }
  ]
}
```

`spec` is `null` in `new` mode. Paths are relative to `root` with forward slashes. When evidence is weak, say so in `evidence` in plain words: `no test file references Tracker; exporter_test.go tests the exporter only`.

## Rules

- Bash is for `git ls-files`, `go list`, `wc`, `find`, `grep`, and `head` only. No writes, no network, no `jev review` without `--dry-run`.
- Do not read files outside `root`.
- Do not paste file contents into your reply. Report paths, symbols, sizes, and evidence.
- Prefer fewer, well-evidenced units over many weak ones. Leave out a section with no code behind it and list it in `unmatched_headings`.
```

- [ ] **Step 2: Validate and commit**

```bash
claude plugin validate --strict plugins/jev-review
grep -nE "—|it's worth|this matters|deliberately" plugins/jev-review/agents/repo-mapper.md; echo "grep exit $?"
git add plugins/jev-review/agents/repo-mapper.md
git -c user.email=ucntcme@gmail.com -c user.name="Bill Anderson" commit -m "Add the repo-mapper agent to the jev-review plugin"
```

---

## Task 7: The `setup` command

**Agent:** `plugin` (after Task 6)

**Files:**
- Create: `plugins/jev-review/commands/setup.md`

Load `plugin-dev:command-development` first. `$1` and `$2` are the positional arguments.

- [ ] **Step 1: Write `plugins/jev-review/commands/setup.md`**

```markdown
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
```

- [ ] **Step 2: Validate and commit**

```bash
claude plugin validate --strict plugins/jev-review
grep -nE "—|it's worth|this matters|deliberately" plugins/jev-review/commands/setup.md; echo "grep exit $?"
git add plugins/jev-review/commands/setup.md
git -c user.email=ucntcme@gmail.com -c user.name="Bill Anderson" commit -m "Add the /jev-review:setup command"
```

---

## Task 8: `tools/checkplugin.sh`, `make plugin`, CI step

**Agent:** `plugin` (after Task 7)

**Files:**
- Create: `tools/checkplugin.sh`
- Modify: `Makefile`, `.github/workflows/ci.yml`

The script is zsh. Consult `shell-scripting-pro` if a zsh construct is in doubt. It parses JSON with `python3`, which is on macOS and on the Ubuntu runner image. It runs `claude plugin validate --strict` only when `claude` is on `PATH`, so CI without Claude Code still passes.

- [ ] **Step 1: Write `tools/checkplugin.sh`**

```zsh
#!/bin/zsh
# Checks the Claude Code marketplace and plugin files: both manifests parse,
# every plugin source exists with its own manifest and matching name, every
# skill, command, and agent carries the frontmatter Claude Code needs, and the
# config-schema reference names exactly the fields of the review package's
# Config and Unit structs. Runs `claude plugin validate --strict` when the
# claude CLI is on PATH.
set -euo pipefail
cd "${0:A:h}/.."
status=0
fail() { print -u2 -- "checkplugin: $*"; status=1 }

json_ok() { python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$1" 2>/dev/null }

json_field() { python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))[sys.argv[2]])' "$1" "$2" }

# frontmatter_has FILE KEY: true when the file starts with a YAML frontmatter
# block that has KEY at the top level.
frontmatter_has() {
  local file=$1 key=$2
  [[ "$(head -n1 "$file")" == "---" ]] || return 1
  sed -n '2,/^---$/p' "$file" | grep -qE "^${key}:"
}

market=.claude-plugin/marketplace.json
[[ -f $market ]] || { fail "$market is missing"; exit 1 }
json_ok $market || { fail "$market is not valid JSON"; exit 1 }

typeset -a sources
sources=(${(f)"$(python3 -c 'import json; [print(p["source"], p["name"]) for p in json.load(open(".claude-plugin/marketplace.json"))["plugins"]]')"})
for entry in $sources; do
  src=${entry%% *}
  name=${entry#* }
  [[ -d $src ]] || { fail "plugin source $src does not exist"; continue }
  manifest=$src/.claude-plugin/plugin.json
  [[ -f $manifest ]] || { fail "$manifest is missing"; continue }
  json_ok $manifest || { fail "$manifest is not valid JSON"; continue }
  [[ "$(json_field $manifest name)" == "$name" ]] || fail "$manifest name does not match marketplace entry $name"
  for d in $src/skills/*(N/); do
    [[ -f $d/SKILL.md ]] || { fail "$d has no SKILL.md"; continue }
    frontmatter_has $d/SKILL.md name || fail "$d/SKILL.md frontmatter lacks name"
    frontmatter_has $d/SKILL.md description || fail "$d/SKILL.md frontmatter lacks description"
  done
  for f in $src/commands/*.md(N) $src/agents/*.md(N); do
    frontmatter_has $f description || fail "$f frontmatter lacks description"
  done
done

# The config-schema reference must name exactly the json fields of Config and
# Unit. Only its field-table rows start with a backticked lowercase name.
schema=plugins/jev-review/skills/setting-up-jev-review/references/config-schema.md
[[ -f $schema ]] || { fail "$schema is missing"; exit 1 }
typeset -a tags documented
tags=(${(f)"$(awk '/^type (Config|Unit) struct/,/^}/' internal/review/review.go | grep -o 'json:"[a-z_]*' | sed 's/json:"//' | sort -u)"})
documented=(${(f)"$(grep -oE '^\| `[a-z_]+`' $schema | sed -E 's/^\| `([a-z_]+)`/\1/' | sort -u)"})
for t in $tags; do
  (( ${documented[(Ie)$t]} )) || fail "config-schema.md does not document field $t"
done
for d in $documented; do
  (( ${tags[(Ie)$d]} )) || fail "config-schema.md documents $d, which is not a field of Config or Unit"
done

if command -v claude >/dev/null 2>&1; then
  claude plugin validate --strict . >/dev/null || fail "claude plugin validate --strict . failed"
  for entry in $sources; do
    src=${entry%% *}
    claude plugin validate --strict $src >/dev/null || fail "claude plugin validate --strict $src failed"
  done
fi

exit $status
```

Make it executable: `chmod +x tools/checkplugin.sh`.

- [ ] **Step 2: Add the `plugin` target to `Makefile`**

Change the `.PHONY` line to:

```make
.PHONY: help test lint vuln build release integration review plugin docs site site-serve clean
```

Insert after the `review` target and before `docs`:

```make
plugin: ## Check the Claude Code marketplace and plugin files
	@./tools/checkplugin.sh
```

- [ ] **Step 3: Add the CI step**

In `.github/workflows/ci.yml`, append to the `lint` job's steps, after the golangci-lint step:

```yaml
      - run: command -v zsh >/dev/null || sudo apt-get install -y zsh
      - run: make plugin
```

- [ ] **Step 4: Run the check, prove it catches drift, restore**

```bash
make plugin; echo "exit $?"
sed -i '' 's/^| `notes` |/| `remarks` |/' plugins/jev-review/skills/setting-up-jev-review/references/config-schema.md
make plugin; echo "exit $?"
git checkout plugins/jev-review/skills/setting-up-jev-review/references/config-schema.md
make plugin; echo "exit $?"
make help | grep plugin
```
Expected: exit 0; then two `checkplugin:` lines (`does not document field notes`, `documents remarks, which is not a field`) and exit 1; then exit 0; `make help` lists `plugin`.

- [ ] **Step 5: Commit**

```bash
git add tools/checkplugin.sh Makefile .github/workflows/ci.yml
git -c user.email=ucntcme@gmail.com -c user.name="Bill Anderson" commit -m "Add make plugin and the plugin checker"
```

---

## Task 9: The how-to, index, README, and layout reference

**Agent:** `docs` (after Tasks 2 and 8 are committed)

**Files:**
- Create: `docs/how-to/build-a-jev-review-config-with-claude-code.md`
- Modify: `docs/how-to/_index.md`, `README.md`, `docs/reference/makefile-and-repository-layout.md`

Run `make build` first so `./bin/jev` has `--dry-run`. Read `plugins/jev-review/commands/setup.md` and `plugins/jev-review/README.md`; the how-to describes what they do, in the same words for the same things. Follow the prose rule in "Rules for every agent". Every command output on the page is real.

- [ ] **Step 1: Make the transcripts**

Create `$SCRATCH/howto-demo/` with the same five source files and `jev-review.json` as Task 3 Step 1 (`spec.md`, `retry.go`, `retry_test.go`, `validation.go`, `validation_test.go`). Run from inside it:

```bash
/Users/bill/Projects/gojev/bin/jev review --dry-run; echo "exit $?"
```

Keep the output for the page.

- [ ] **Step 2: Write the how-to** (`diataxis-docs:doc-howto-writer`)

Front matter:

```yaml
---
title: "Build a jev review Config with Claude Code"
description: "Install the jev-review plugin from this repository's marketplace, run /jev-review:setup, approve the unit map, and check the result with jev review --dry-run before a paid run."
diataxis: how-to
weight: 106
---
```

First paragraph, the scope statement in these words:

> `jev review` judges whether your tests exercise the behaviors you listed and whether your implementation contradicts the spec section you named. It sends those files to Jev and answers with probabilities. It does not find bugs, review style, check security, or verify that the code is correct, and its readings drift between runs on the same input.

Second paragraph: this page is about getting the spec and `jev-review.json` written by the `jev-review` Claude Code plugin; writing them by hand is covered by [Review your codebase with jev](./review-your-codebase-with-jev.md) and [How to write a spec jev review can use](./write-a-spec-jev-review-can-use.md).

**Goal**: a spec and a `jev-review.json` for your repository that pass `jev review --dry-run`.

Prerequisites: Claude Code; `jev` on `PATH` at a release with `--dry-run` (the two install routes from the README, and `jev review --help | grep dry-run` to check); a repository with tests. `TYPESAFE_API_KEY` is not needed until the last step.

Steps:

1. Add the marketplace and install the plugin: the two slash commands from the plugin README, then `/plugin` to confirm `jev-review` is listed.
2. Run `/jev-review:setup` in the repository, with the optional spec and config path arguments and what each defaults to.
3. Approve the unit map: show the checkpoint table format from `commands/setup.md` step 4 as an example, explain each column, say that `!` marks a bundle over the half-budget, and say that "Describe changes" reruns the table.
4. Read the hand-off: the files written, the dry-run table, the list of sentences written from code with file and line, and why to confirm each one states intent.
5. Rerun the dry run yourself: the transcript from Step 1 of this task, and what the columns mean.
6. Run `jev review` for the readings: needs `TYPESAFE_API_KEY`; sends the spec section and every listed file to the TypeSafe API; then triage per [Review your codebase with jev](./review-your-codebase-with-jev.md), and the plugin's skill will triage flags pasted back into the session.

Verify it works: `jev review --dry-run; echo $?` prints a table with an `ok` row per unit and `0`.

See also: the two how-tos above, the [jev review reference](../reference/jev-review.md), and the plugin directory on GitHub, linked as `https://github.com/therealbill/typesafe-go/tree/main/plugins/jev-review` (not a relative path; the plugin is outside the docs mount).

- [ ] **Step 3: Link the page**

In `docs/how-to/_index.md`, add after the "How to Write a Spec jev review Can Use" line:

```markdown
- [Build a jev review config with Claude Code](build-a-jev-review-config-with-claude-code.md)
```

In `README.md`, after the paragraph that starts with "`jev review` asks Jev whether your tests cover", add:

```markdown
A Claude Code plugin writes the spec and `jev-review.json` for you. Add this repository as a marketplace with `/plugin marketplace add therealbill/typesafe-go`, install with `/plugin install jev-review@typesafe-go`, and run `/jev-review:setup` in your repository; see [the how-to](docs/how-to/build-a-jev-review-config-with-claude-code.md).
```

In the README's "How-to" documentation line, add `[build a jev review config with Claude Code](docs/how-to/build-a-jev-review-config-with-claude-code.md)` after the "review your codebase with jev" entry.

- [ ] **Step 4: Update `docs/reference/makefile-and-repository-layout.md`**

1. Replace the `make help` block with the output of `make help` run now (it gains the `plugin` line after `review`).
2. In the repository layout table, replace the `tools` row and add two rows after the `.github/workflows` row:

```markdown
| `tools` | `checkdocs.sh`, the documentation link checker `make docs` runs, and `checkplugin.sh`, the plugin checker `make plugin` runs. |
| `.claude-plugin` | `marketplace.json`, the Claude Code marketplace manifest that lists `plugins/jev-review`. |
| `plugins/jev-review` | The jev-review Claude Code plugin: `.claude-plugin/plugin.json`, `README.md`, `commands/setup.md`, `agents/repo-mapper.md`, and `skills/setting-up-jev-review/`. Documented on [Build a jev review config with Claude Code](../how-to/build-a-jev-review-config-with-claude-code.md). |
```

3. In the `ci.yml` table, change the `lint` row to end with: "runs `golangci/golangci-lint-action@v8` at `version: v2.5`, installs `zsh` when it is absent, then runs `make plugin`."

- [ ] **Step 5: Validate and commit**

Run `diataxis-docs:doc-crosslink-validator` on the new page, then:

```bash
./tools/checkdocs.sh && make docs
grep -nE "—|it's worth|this matters|deliberately" docs/how-to/build-a-jev-review-config-with-claude-code.md README.md docs/reference/makefile-and-repository-layout.md; echo "grep exit $?"
git add docs/how-to/build-a-jev-review-config-with-claude-code.md docs/how-to/_index.md README.md docs/reference/makefile-and-repository-layout.md
git -c user.email=ucntcme@gmail.com -c user.name="Bill Anderson" commit -m "Document the jev-review plugin"
```
Expected: `checkdocs.sh` prints nothing, `make docs` passes, the grep exits 1.

---

## Task 10: Gate, interactive hand-off, fixes, push

**Agent:** lead (after Tasks 3, 8, and 9)

- [ ] **Step 1: Gate**

```bash
make lint && make test && make build && make plugin && make docs && git status --short && echo "(clean)"
./bin/jev review --dry-run; echo "exit $?"
env -u TYPESAFE_API_KEY ./bin/jev review --dry-run --json | jq -e 'length == 8 and all(.spec_bytes > 0)'
claude plugin validate --strict . && claude plugin validate --strict plugins/jev-review
```
Expected: every target passes; the tree is clean; eight `ok` rows and exit 0; `jq` prints `true`; both validations pass.

- [ ] **Step 2: Prepare the two interactive runs**

The checkpoint in `/jev-review:setup` needs a person, so the end-to-end runs happen in the user's own Claude Code session. Prepare both targets:

```bash
mkdir -p $SCRATCH/retrydemo && cd $SCRATCH/retrydemo && git init -q
printf 'module example.com/retrydemo\n\ngo 1.25\n' > go.mod
cat > retry.go <<'EOF_GO'
// Package retry provides a minimal retry helper for transient failures.
package retry

import (
	"context"
	"time"
)

// DefaultMaxRetries is used when Config.MaxRetries is zero or negative.
const DefaultMaxRetries = 2

// Config controls Do.
type Config struct {
	MaxRetries int
	Delay      time.Duration
}

// Do calls fn until it returns nil or the retries are exhausted.
func Do(ctx context.Context, cfg Config, fn func() error) error {
	max := cfg.MaxRetries
	if max <= 0 {
		max = DefaultMaxRetries
	}
	var err error
	for attempt := 0; attempt <= max; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if attempt == max {
			break
		}
		select {
		case <-time.After(cfg.Delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}
EOF_GO
cat > retry_test.go <<'EOF_GO'
package retry

import (
	"context"
	"testing"
)

func TestDoReturnsOnFirstSuccess(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Config{}, func() error { calls++; return nil })
	if err != nil || calls != 1 {
		t.Fatalf("err %v calls %d", err, calls)
	}
}
EOF_GO
go test ./... && git add go.mod retry.go retry_test.go && git -c user.email=ucntcme@gmail.com -c user.name="Bill Anderson" commit -q -m "retry demo"
git clone -q /Users/bill/Projects/gojev $SCRATCH/gojev-copy && rm $SCRATCH/gojev-copy/jev-review.json
```

Then hand these steps to the user in the final report, verbatim:

```
In any Claude Code session:
  /plugin marketplace add /Users/bill/Projects/gojev
  /plugin install jev-review@typesafe-go

Run 1, new-spec mode (a repo with tests and no spec):
  cd $SCRATCH/retrydemo && claude
  /jev-review:setup
  Expect: one unit named retry, heading "## Retry" marked new, retry.go and retry_test.go paired,
  a written SPEC.md, a jev-review.json, a passing dry run, and a hand-off listing the sentences
  written from code. Then, with TYPESAFE_API_KEY set: jev review

Run 2, existing-spec mode (this repository without its config):
  cd $SCRATCH/gojev-copy && claude
  /jev-review:setup docs/superpowers/specs/2026-09-23-typesafe-go-sdk-design.md
  Expect: units close to the checked-in jev-review.json (questions, responses, errors, retry,
  client, typed, otel, cli). Compare with: diff <(jq -S . jev-review.json) <(jq -S . /Users/bill/Projects/gojev/jev-review.json)
  Differences are reviewed, not necessarily fixed.
```

- [ ] **Step 3: Apply fixes from the interactive runs**

For each problem the user reports, edit the plugin file at fault (`commands/setup.md`, `agents/repo-mapper.md`, or a reference), rerun `make plugin` and `claude plugin validate --strict plugins/jev-review`, and commit with an explicit path. Reinstalling picks up the change: `/plugin uninstall jev-review@typesafe-go` then `/plugin install jev-review@typesafe-go`.

- [ ] **Step 4: Tick the plan, commit, push, watch the site deploy**

```bash
git add docs/superpowers/plans/2026-09-30-jev-review-plugin.md
git -c user.email=ucntcme@gmail.com -c user.name="Bill Anderson" commit -m "Check off the jev-review plugin plan"
git push origin main
gh run watch $(gh run list --workflow ci --limit 1 --json databaseId -q '.[0].databaseId') --exit-status
gh run watch $(gh run list --workflow "Deploy documentation site" --limit 1 --json databaseId -q '.[0].databaseId') --exit-status
curl -fsS https://therealbill.github.io/typesafe-go/docs/how-to/build-a-jev-review-config-with-claude-code/ | grep -o '<title>[^<]*</title>'
```
Expected: `ci` passes with the `make plugin` step; the site deploys; the new page's title prints.

- [ ] **Step 5: Report**

Task numbers, commit hashes, the gate tail, the interactive-run results, and any deviation from the spec. Then ask the user whether to cut a release, since `go install ...@latest` and the release binaries carry `--dry-run` only after one: `make release VERSION=v0.2.0`, following [Cut a release](../../how-to/cut-a-release.md). Cutting it is the user's call.
