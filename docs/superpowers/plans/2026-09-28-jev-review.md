# `jev review` Implementation Plan

> **For agentic workers:** Execute this plan with an **Agent Team** of named agents (Agent tool with `name`, coordinated by the lead through SendMessage). Do NOT use git worktrees, do NOT use unnamed parallel subagents as the execution method, and do NOT use the `superpowers:executing-plans` skill. Roster and dependencies are in "Execution Model". Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Promote the self-review tool to a `jev review` subcommand usable on any codebase, and document it for that audience with an explicit statement of what it judges.

**Architecture:** The review logic moves from `tools/selfreview/main.go` into package `internal/review` with an `Asker` interface the `*typesafe.Client` satisfies, so it calls the library directly and is testable with a fake. `internal/cli/review.go` adds `jev review` and `jev review init`. The repository's own config moves to `jev-review.json` at the root. Three documentation pages are rewritten for other codebases.

**Tech Stack:** Go 1.25, Cobra, the `typesafe` package, existing test patterns (`httptest`, in-process `Main`).

**Spec:** `docs/superpowers/specs/2026-09-28-jev-review-design.md`. Read it first.

---

## Execution Model

### Team roster

| Agent name | Type / model | Owns | Skills and subagents to use |
|---|---|---|---|
| `cli` | `go-architect`, model **opus** | Tasks 1–3 (`internal/review`, `internal/cli/review.go`, config move, Makefile, tool deletion) | `superpowers:test-driven-development`, `superpowers:verification-before-completion` |
| `docs` | `general-purpose`, model **sonnet** | Tasks 4–5 (pages, aliases, cross-page updates, validation) | `diataxis-docs:doc-howto-writer`, `diataxis-docs:doc-reference-gen`, `diataxis-docs:doc-explanation-writer`, `diataxis-docs:doc-crosslink-validator` |
| lead (this session) | — | Task 6 (gate, push, deploy watch) | — |

### Dependency graph

```
Task 1 (internal/review) → Task 2 (subcommand) → Task 3 (config move, Makefile, delete tool)
Task 2 ──────────────────────────────────────────→ Task 4 (three pages) → Task 5 (cross-page updates, validation)
Tasks 3 and 5 → Task 6
```

`docs` starts Task 4 when Task 2 is committed (the subcommand and its help text exist); `cli` continues to Task 3 concurrently. They touch disjoint files.

### Rules for every agent

1. Work in `/Users/bill/Projects/gojev` on `main`. No worktrees, no other branches.
2. Touch only the files listed under your task.
3. Stage by explicit path only. Never `git add -A` or `git add .`. Retry once after two seconds on `index.lock`.
4. Commit with `git -c user.email=ucntcme@gmail.com -c user.name="Bill Anderson" commit -m "..."`. No "Authored by" lines. Do not push.
5. Prose rule for every page and every doc comment: plain declaratives and imperatives, no em dashes, no contrast-reveal sentences, no narrated reasoning, no duration estimates. The user rejected the earlier style; see the rubric in Task 4.
6. `TYPESAFE_API_KEY` is in the environment for real runs. Never print, log, or write it.
7. Scratch files go under `/private/tmp/claude-501/-Users-bill-Projects-gojev/b96adec7-3da2-45f5-b6b1-95fdebc40eb3/scratchpad/`.
8. Report after each task: task number, commit hash, verification tail, deviations.

---

## File map

| Path | Responsibility | Task |
|---|---|---|
| `internal/review/review.go`, `internal/review/template.go`, `internal/review/review_test.go` | review logic, template, tests | 1 |
| `internal/cli/review.go`, `internal/cli/review_test.go`, `internal/cli/exit.go` | subcommand, `ExitFlagged` | 2 |
| `jev-review.json`, `Makefile`, `.gitignore`, `tools/selfreview/` (deleted) | repository config and targets | 3 |
| `docs/how-to/review-your-codebase-with-jev.md`, `docs/reference/jev-review.md`, `docs/explanation/what-jev-review-measures.md` (old three deleted), the three section `_index.md` | pages | 4 |
| `docs/reference/jev-cli.md`, `docs/reference/errors-and-exit-codes.md`, `docs/reference/makefile-and-repository-layout.md`, `docs/how-to/cut-a-release.md`, `docs/explanation/the-agent-facing-cli-contract.md`, `README.md`, `docs/superpowers/specs/2026-09-23-typesafe-go-sdk-design.md` | cross-page updates | 5 |

---

## Task 1: Package `internal/review`

**Agent:** `cli`

**Files:**
- Create: `internal/review/review.go`, `internal/review/template.go`, `internal/review/review_test.go`

The logic is `tools/selfreview/main.go` restructured: exported types, `Load` with validation and path resolution, `Run` over an `Asker`, typed answers instead of `map[string]any`, and no `os.Exit`, `flag`, `exec`, or environment reads. Do not delete `tools/selfreview` in this task; Task 3 does.

- [x] **Step 1: Write the failing tests `internal/review/review_test.go`**

```go
package review

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/therealbill/typesafe-go"
)

const sampleSpec = "# Title\n\n## Public surface\n\n### Client\n\nclient text\nmore\n\n### Questions\n\nq text\n\n## Next\n\nother\n"

func TestExtractSection(t *testing.T) {
	got, err := extractSection(sampleSpec, "### Client")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got) != "### Client\n\nclient text\nmore" {
		t.Fatalf("got %q", got)
	}
	got, err = extractSection(sampleSpec, "## Public surface")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "### Questions") || strings.Contains(got, "other") {
		t.Fatalf("level-2 section should include subsections and stop at the next level-2: %q", got)
	}
	if _, err := extractSection(sampleSpec, "### Missing"); err == nil {
		t.Fatal("missing heading should error")
	}
}

func TestExtractSectionIgnoresFencedHeadings(t *testing.T) {
	spec := "## One\n\ntext\n\n```sh\n# comment that is not a heading\n## also not a heading\n```\n\nmore text\n\n## Two\n\nother\n"
	got, err := extractSection(spec, "## One")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "# comment that is not a heading") || !strings.Contains(got, "more text") || strings.Contains(got, "other") {
		t.Fatalf("got %q", got)
	}
}

func TestBuildQuestions(t *testing.T) {
	u := Unit{Name: "x", Behaviors: []string{"b0", "b1"}}
	qs := buildQuestions(u)
	for _, k := range []string{"covers_00", "covers_01", "contradicts_spec", "thoroughness", "weakest_area"} {
		if _, ok := qs[k]; !ok {
			t.Fatalf("missing question %s", k)
		}
	}
	if qs["thoroughness"]["type"] != "score" || qs["weakest_area"]["type"] != "choice" {
		t.Fatal("wrong question types")
	}
	labels := qs["weakest_area"]["criteria"].(map[string]any)
	if len(labels) != 7 {
		t.Fatalf("weakest_area has %d labels, want 7", len(labels))
	}
	b, err := json.Marshal(qs)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"type":"noul"`) {
		t.Fatalf("questions must marshal as raw questions: %s", b)
	}
}

func TestBundleTruncates(t *testing.T) {
	files := map[string]string{"a.go": strings.Repeat("x", 100), "b.go": strings.Repeat("y", 100)}
	text, truncated := bundle([]string{"a.go", "b.go"}, files, 150)
	if !truncated || len(text) > 150+len(truncatedMarker)*2+40 {
		t.Fatalf("truncated=%v len=%d", truncated, len(text))
	}
	text, truncated = bundle([]string{"a.go"}, files, 1000)
	if truncated || !strings.Contains(text, "// file: a.go") {
		t.Fatalf("truncated=%v text=%q", truncated, text)
	}
}

func answers(cover []float64, contradicts, score, conf float64, choice string) map[string]typesafe.Answer {
	a := map[string]typesafe.Answer{}
	for i, p := range cover {
		a[coversKey(i)] = typesafe.NoulAnswer{Noul: p}
	}
	a["contradicts_spec"] = typesafe.NoulAnswer{Noul: contradicts}
	a["thoroughness"] = typesafe.ScoreAnswer{Score: score, Confidence: conf}
	a["weakest_area"] = typesafe.ChoiceAnswer{Choice: choice, Confidence: 0.6}
	return a
}

func defaults() Options { return DefaultOptions() }

func TestEvaluate(t *testing.T) {
	u := Unit{Name: "x", Behaviors: []string{"b0", "b1"}, Accepted: []string{"b1"}, Notes: []string{"b1: known noise"}}
	r := evaluate(u, answers([]float64{0.9, 0.2}, 0.1, 2.4, 0.8, "retry"), defaults())
	if len(r.Flags) != 1 || !strings.Contains(r.Flags[0], "covers_01") || !strings.Contains(r.Flags[0], "accepted") {
		t.Fatalf("flags %v", r.Flags)
	}
	if r.Failing {
		t.Fatal("an accepted flag must not fail the unit")
	}
	if len(r.Notes) != 1 || r.Notes[0] != "b1: known noise" {
		t.Fatalf("notes must travel into the report, got %v", r.Notes)
	}
	r = evaluate(u, answers([]float64{0.9, 0.2}, 0.7, 2.4, 0.8, "retry"), defaults())
	if !r.Failing || len(r.Flags) != 2 {
		t.Fatalf("contradiction should fail: %+v", r)
	}
}

func TestEvaluateReportsStaleAcceptance(t *testing.T) {
	u := Unit{Name: "x", Behaviors: []string{"b0"}, Accepted: []string{"b0", "thoroughness"}}
	r := evaluate(u, answers([]float64{0.9}, 0.1, 2.5, 0.8, "retry"), defaults())
	if r.Failing {
		t.Fatalf("nothing should fail: %+v", r)
	}
	if len(r.StaleAcceptances) != 2 {
		t.Fatalf("both unused acceptances should be reported, got %v", r.StaleAcceptances)
	}
	out := Report{Units: []UnitReport{r}}.Markdown(defaults())
	if !strings.Contains(out, "acceptance did not fire: b0") {
		t.Fatalf("stale acceptances must appear in the summary:\n%s", out)
	}
}

func TestEvaluateMissingOrWrongTypeAnswerIsError(t *testing.T) {
	u := Unit{Name: "x", Behaviors: []string{"b0"}}
	full := answers([]float64{0.9}, 0.1, 2.5, 0.8, "retry")
	if r := evaluate(u, full, defaults()); r.Error != "" {
		t.Fatalf("a complete answer set must not error: %q", r.Error)
	}
	for _, drop := range []string{"covers_00", "contradicts_spec", "thoroughness", "weakest_area"} {
		a := answers([]float64{0.9}, 0.1, 2.5, 0.8, "retry")
		delete(a, drop)
		if r := evaluate(u, a, defaults()); r.Error == "" || !r.Failing {
			t.Errorf("a missing %s must be a unit error, got %+v", drop, r)
		}
	}
	a := answers([]float64{0.9}, 0.1, 2.5, 0.8, "retry")
	a["thoroughness"] = typesafe.NoulAnswer{Noul: 0.5}
	if r := evaluate(u, a, defaults()); r.Error == "" || !r.Failing {
		t.Errorf("a wrongly typed answer must be a unit error, got %+v", r)
	}
}

func TestMarkdownUsesMinCoverThreshold(t *testing.T) {
	r := UnitReport{Name: "x", Behaviors: []BehaviorResult{{ID: "covers_00", Covered: 0.65}, {ID: "covers_01", Covered: 0.80}}}
	o := defaults()
	if out := (Report{Units: []UnitReport{r}}).Markdown(o); !strings.Contains(out, "2/2") {
		t.Fatalf("both behaviors clear 0.6:\n%s", out)
	}
	o.MinCover = 0.7
	if out := (Report{Units: []UnitReport{r}}).Markdown(o); !strings.Contains(out, "1/2") {
		t.Fatalf("only one behavior clears 0.7:\n%s", out)
	}
}

func writeConfig(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "jev-review.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadResolvesRelativeToConfigAndValidates(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "cfg")
	if err := os.MkdirAll(filepath.Join(sub, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(sub, "spec.md"), []byte("## A\n\ntext\n"), 0o600)
	_ = os.WriteFile(filepath.Join(sub, "src", "a.go"), []byte("package a\n"), 0o600)
	_ = os.WriteFile(filepath.Join(sub, "src", "a_test.go"), []byte("package a\n"), 0o600)
	p := writeConfig(t, sub, `{"spec":"spec.md","units":[{"name":"u","spec_heading":"## A","implementation":["src/a.go"],"tests":["src/a_test.go"],"behaviors":["does x"],"accepted":["does x","thoroughness"]}]}`)
	cfg, base, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if base != sub || cfg.Units[0].Name != "u" {
		t.Fatalf("base %q cfg %+v", base, cfg)
	}
	bad := []struct{ name, body, want string }{
		{"no units", `{"spec":"spec.md","units":[]}`, "at least one unit"},
		{"dup names", `{"spec":"s","units":[{"name":"u","spec_heading":"## A","implementation":["a"],"behaviors":["b"]},{"name":"u","spec_heading":"## A","implementation":["a"],"behaviors":["b"]}]}`, "duplicate"},
		{"no heading", `{"spec":"s","units":[{"name":"u","implementation":["a"],"behaviors":["b"]}]}`, "spec_heading"},
		{"no impl", `{"spec":"s","units":[{"name":"u","spec_heading":"## A","behaviors":["b"]}]}`, "implementation"},
		{"no behaviors", `{"spec":"s","units":[{"name":"u","spec_heading":"## A","implementation":["a"]}]}`, "behavior"},
		{"positional accept", `{"spec":"s","units":[{"name":"u","spec_heading":"## A","implementation":["a"],"behaviors":["b"],"accepted":["covers_00"]}]}`, "covers_00"},
		{"unknown accept", `{"spec":"s","units":[{"name":"u","spec_heading":"## A","implementation":["a"],"behaviors":["b"],"accepted":["nope"]}]}`, "nope"},
		{"no spec", `{"units":[{"name":"u","spec_heading":"## A","implementation":["a"],"behaviors":["b"]}]}`, "spec"},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			p := writeConfig(t, t.TempDir(), tt.body)
			_, _, err := Load(p)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v want substring %q", err, tt.want)
			}
		})
	}
	if _, _, err := Load(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("missing file must error")
	}
}

type fakeAsker struct {
	states    []map[string]any
	questions []typesafe.Questions
	answers   map[string]typesafe.Answer
	err       error
	delay     time.Duration
}

func (f *fakeAsker) SystemOne(ctx context.Context, state any, qs typesafe.Questions, _ ...typesafe.RequestOption) (*typesafe.SystemOneResponse, error) {
	f.states = append(f.states, state.(map[string]any))
	f.questions = append(f.questions, qs)
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, &typesafe.ConnectionError{Err: ctx.Err()}
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	return &typesafe.SystemOneResponse{Model: "jev-test", Answers: f.answers, RequestID: "req_fake", Usage: typesafe.Usage{InputTokens: intp(42)}}, nil
}

func intp(i int) *int { return &i }

func setupRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n\n## Retry\n\nretry text\n\n## Other\n\nno\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "retry.go"), []byte("package x\n// impl\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "retry_test.go"), []byte("package x\n// tests\n"), 0o600)
	p := writeConfig(t, dir, `{"spec":"spec.md","units":[{"name":"retry","spec_heading":"## Retry","implementation":["retry.go"],"tests":["retry_test.go"],"behaviors":["b0"]},{"name":"other","spec_heading":"## Other","implementation":["retry.go"],"tests":["retry_test.go"],"behaviors":["b0"]}]}`)
	return dir, p
}

func TestRunBuildsStateAndEvaluates(t *testing.T) {
	_, p := setupRepo(t)
	cfg, base, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	fa := &fakeAsker{answers: answers([]float64{0.9}, 0.1, 2.5, 0.8, "none")}
	rep, err := Run(context.Background(), cfg, base, fa, defaults())
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Units) != 2 || rep.Failing() {
		t.Fatalf("report %+v", rep)
	}
	st := fa.states[0]
	if !strings.Contains(st["spec"].(string), "retry text") || strings.Contains(st["spec"].(string), "no\n") {
		t.Fatalf("spec section wrong: %q", st["spec"])
	}
	if !strings.Contains(st["implementation"].(string), "// file: retry.go") || !strings.Contains(st["tests"].(string), "// tests") {
		t.Fatalf("bundles wrong: %+v", st)
	}
	if _, ok := fa.questions[0]["covers_00"].(typesafe.RawQuestion); !ok {
		t.Fatalf("questions must be RawQuestion values: %T", fa.questions[0]["covers_00"])
	}
	if rep.Units[0].RequestID != "req_fake" || rep.Units[0].InputTokens != 42 || rep.Units[0].Model != "jev-test" {
		t.Fatalf("metadata not copied: %+v", rep.Units[0])
	}
}

func TestRunOnlyAndNoMatch(t *testing.T) {
	_, p := setupRepo(t)
	cfg, base, _ := Load(p)
	fa := &fakeAsker{answers: answers([]float64{0.9}, 0.1, 2.5, 0.8, "none")}
	o := defaults()
	o.Only = "other"
	rep, err := Run(context.Background(), cfg, base, fa, o)
	if err != nil || len(rep.Units) != 1 || rep.Units[0].Name != "other" {
		t.Fatalf("only: %+v %v", rep, err)
	}
	o.Only = "missing"
	if _, err := Run(context.Background(), cfg, base, fa, o); !errors.Is(err, ErrNoUnits) {
		t.Fatalf("want ErrNoUnits, got %v", err)
	}
}

func TestRunRecordsCallErrorsAndTimeouts(t *testing.T) {
	_, p := setupRepo(t)
	cfg, base, _ := Load(p)
	apiErr := &typesafe.APIError{Status: 401, Endpoint: "POST /v1/systemone"}
	fa := &fakeAsker{err: apiErr}
	rep, err := Run(context.Background(), cfg, base, fa, defaults())
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Failing() || rep.Units[0].Error == "" || !errors.Is(rep.FirstCallError(), apiErr) {
		t.Fatalf("call error must be recorded and exposed: %+v", rep)
	}
	slow := &fakeAsker{delay: time.Second, answers: answers([]float64{0.9}, 0.1, 2.5, 0.8, "none")}
	o := defaults()
	o.UnitTimeout = 20 * time.Millisecond
	rep, err = Run(context.Background(), cfg, base, slow, o)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Failing() || !strings.Contains(rep.Units[0].Error, "did not finish within") {
		t.Fatalf("timeout must be a unit error: %+v", rep.Units[0])
	}
}

func TestRunMissingFileIsUnitError(t *testing.T) {
	dir, p := setupRepo(t)
	_ = os.Remove(filepath.Join(dir, "retry_test.go"))
	cfg, base, _ := Load(p)
	fa := &fakeAsker{answers: answers([]float64{0.9}, 0.1, 2.5, 0.8, "none")}
	rep, err := Run(context.Background(), cfg, base, fa, defaults())
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Failing() || !strings.Contains(rep.Units[0].Error, "retry_test.go") || len(fa.states) != 0 {
		t.Fatalf("missing file must be a unit error before any call: %+v", rep.Units[0])
	}
}

func TestTemplateLoads(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "jev-review.json")
	if err := os.WriteFile(p, Template(), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(p)
	if err != nil {
		t.Fatalf("template must load: %v", err)
	}
	if cfg.Description == "" || len(cfg.Units) != 1 || len(cfg.Units[0].Behaviors) < 2 {
		t.Fatalf("template shape: %+v", cfg)
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/review/ 2>&1 | head -3
```
Expected: package does not exist or undefined symbols.

- [x] **Step 3: Write `internal/review/review.go`**

```go
// Package review asks Jev whether a codebase's tests exercise the behaviors
// its specification requires and whether the implementation contradicts the
// specification. It judges only the behaviors listed in the configuration
// against the named spec section and files. It does not find bugs, review
// style, check security, or verify correctness.
package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/therealbill/typesafe-go"
)

// Config is the contents of jev-review.json. File paths, including Spec,
// resolve relative to the directory that contains the config file.
type Config struct {
	Description string `json:"description,omitempty"`
	Spec        string `json:"spec"`
	Units       []Unit `json:"units"`
}

// Unit pairs one spec section with the files that implement and test it and
// the behaviors the section requires.
type Unit struct {
	Name           string   `json:"name"`
	SpecHeading    string   `json:"spec_heading"`
	Implementation []string `json:"implementation"`
	Tests          []string `json:"tests"`
	Behaviors      []string `json:"behaviors"`
	// Accepted lists flags that are acknowledged and do not fail the run:
	// a behavior string verbatim, or one of the ids contradicts_spec,
	// thoroughness, weakest_area.
	Accepted []string `json:"accepted,omitempty"`
	// Notes records why each acceptance was granted. They are copied into
	// the report.
	Notes []string `json:"notes,omitempty"`
}

// Options controls thresholds, limits, and selection.
type Options struct {
	Only          string
	MaxStateBytes int
	MinCover      float64
	MaxContradict float64
	MinThorough   float64
	UnitTimeout   time.Duration
}

// DefaultOptions returns the thresholds and limits used when none are given.
func DefaultOptions() Options {
	return Options{MaxStateBytes: 100000, MinCover: 0.6, MaxContradict: 0.4, MinThorough: 2.0, UnitTimeout: 2 * time.Minute}
}

// Asker is the part of *typesafe.Client the review uses.
type Asker interface {
	SystemOne(ctx context.Context, state any, questions typesafe.Questions, opts ...typesafe.RequestOption) (*typesafe.SystemOneResponse, error)
}

// BehaviorResult is one behavior's coverage probability.
type BehaviorResult struct {
	ID       string  `json:"id"`
	Behavior string  `json:"behavior"`
	Covered  float64 `json:"covered"`
}

// UnitReport is the outcome for one unit.
type UnitReport struct {
	Name             string           `json:"name"`
	RequestID        string           `json:"request_id,omitempty"`
	Model            string           `json:"model,omitempty"`
	InputTokens      int              `json:"input_tokens,omitempty"`
	Truncated        bool             `json:"truncated"`
	Behaviors        []BehaviorResult `json:"behaviors"`
	Contradicts      float64          `json:"contradicts_spec"`
	Thoroughness     float64          `json:"thoroughness"`
	ThoroughConf     float64          `json:"thoroughness_confidence"`
	Weakest          string           `json:"weakest_area"`
	WeakestConf      float64          `json:"weakest_confidence"`
	Flags            []string         `json:"flags"`
	Notes            []string         `json:"notes,omitempty"`
	StaleAcceptances []string         `json:"stale_acceptances,omitempty"`
	Failing          bool             `json:"failing"`
	Error            string           `json:"error,omitempty"`
	err              error
}

// Report is the outcome of a run.
type Report struct {
	Units []UnitReport
}

// Failing reports whether any unit has an unaccepted flag or an error.
func (r Report) Failing() bool {
	for _, u := range r.Units {
		if u.Failing {
			return true
		}
	}
	return false
}

// FirstCallError returns the first API or transport error that stopped a
// unit, or nil. Callers use it to choose a process exit code.
func (r Report) FirstCallError() error {
	for _, u := range r.Units {
		if u.err != nil {
			return u.err
		}
	}
	return nil
}

// JSON encodes the units as the report document.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r.Units, "", "  ")
}

// ErrNoUnits is returned by Run when Options.Only matches no unit.
var ErrNoUnits = errors.New("review: no units matched")

const truncatedMarker = "\n// ...(truncated)\n"

var coversID = regexp.MustCompile(`^covers_\d+$`)

func coversKey(i int) string { return fmt.Sprintf("covers_%02d", i) }

// Load reads and validates a config file and returns it with the directory
// that relative paths resolve against.
func Load(path string) (*Config, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, "", fmt.Errorf("%s: %w", path, err)
	}
	if err := validate(&cfg); err != nil {
		return nil, "", fmt.Errorf("%s: %w", path, err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	return &cfg, filepath.Dir(abs), nil
}

func validate(cfg *Config) error {
	if strings.TrimSpace(cfg.Spec) == "" {
		return errors.New(`"spec" is required`)
	}
	if len(cfg.Units) == 0 {
		return errors.New("at least one unit is required")
	}
	seen := map[string]bool{}
	for i, u := range cfg.Units {
		where := fmt.Sprintf("units[%d]", i)
		if strings.TrimSpace(u.Name) == "" {
			return fmt.Errorf("%s: name is required", where)
		}
		if seen[u.Name] {
			return fmt.Errorf("duplicate unit name %q", u.Name)
		}
		seen[u.Name] = true
		if strings.TrimSpace(u.SpecHeading) == "" {
			return fmt.Errorf("unit %q: spec_heading is required", u.Name)
		}
		if len(u.Implementation) == 0 {
			return fmt.Errorf("unit %q: at least one implementation file is required", u.Name)
		}
		if len(u.Behaviors) == 0 {
			return fmt.Errorf("unit %q: at least one behavior is required", u.Name)
		}
		behaviors := map[string]bool{}
		for _, b := range u.Behaviors {
			behaviors[b] = true
		}
		for _, a := range u.Accepted {
			if coversID.MatchString(a) {
				return fmt.Errorf("unit %q accepts %q by position; name the behavior text instead", u.Name, a)
			}
			if !behaviors[a] && a != "contradicts_spec" && a != "thoroughness" && a != "weakest_area" {
				return fmt.Errorf("unit %q accepts %q, which is neither one of its behaviors nor a question id", u.Name, a)
			}
		}
	}
	return nil
}

func resolve(base, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}

// Run evaluates every unit (or the one named by opts.Only) and returns the
// report. Errors reading files or a spec section, API failures, and
// timeouts are recorded on the unit; Run itself fails only when no unit
// matched or the spec file cannot be read.
func Run(ctx context.Context, cfg *Config, base string, asker Asker, opts Options) (Report, error) {
	specBytes, err := os.ReadFile(resolve(base, cfg.Spec))
	if err != nil {
		return Report{}, err
	}
	var rep Report
	for _, u := range cfg.Units {
		if opts.Only != "" && u.Name != opts.Only {
			continue
		}
		rep.Units = append(rep.Units, runUnit(ctx, u, string(specBytes), base, asker, opts))
	}
	if len(rep.Units) == 0 {
		return rep, ErrNoUnits
	}
	return rep, nil
}

func runUnit(ctx context.Context, u Unit, spec, base string, asker Asker, opts Options) UnitReport {
	r := UnitReport{Name: u.Name, Notes: u.Notes}
	section, err := extractSection(spec, u.SpecHeading)
	if err != nil {
		r.Error, r.Failing = err.Error(), true
		return r
	}
	files := map[string]string{}
	for _, p := range append(append([]string{}, u.Implementation...), u.Tests...) {
		b, err := os.ReadFile(resolve(base, p))
		if err != nil {
			r.Error, r.Failing = err.Error(), true
			return r
		}
		files[p] = string(b)
	}
	half := opts.MaxStateBytes / 2
	impl, t1 := bundle(u.Implementation, files, half)
	tests, t2 := bundle(u.Tests, files, half)
	truncated := t1 || t2

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
	out.Truncated = truncated
	return out
}

// extractSection returns the lines from the heading through the line before
// the next heading of the same or higher level, ignoring headings inside
// fenced code blocks.
func extractSection(spec, heading string) (string, error) {
	level := len(heading) - len(strings.TrimLeft(heading, "#"))
	lines := strings.Split(spec, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == heading {
			start = i
			break
		}
	}
	if start < 0 {
		return "", fmt.Errorf("heading %q not found in spec", heading)
	}
	end := len(lines)
	fenced := false
	for i := start + 1; i < len(lines); i++ {
		l := lines[i]
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fenced = !fenced
			continue
		}
		if fenced || !strings.HasPrefix(l, "#") {
			continue
		}
		hl := len(l) - len(strings.TrimLeft(l, "#"))
		if hl <= level && strings.HasPrefix(l[hl:], " ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n"), nil
}

// bundle concatenates files with a header line each, cutting the total to
// maxBytes and marking the cut.
func bundle(paths []string, files map[string]string, maxBytes int) (string, bool) {
	var b strings.Builder
	for _, p := range paths {
		b.WriteString("// file: " + filepath.ToSlash(p) + "\n")
		b.WriteString(files[p])
		b.WriteString("\n\n")
	}
	s := b.String()
	if len(s) <= maxBytes {
		return s, false
	}
	return s[:maxBytes] + truncatedMarker, true
}

func buildQuestions(u Unit) map[string]map[string]any {
	qs := map[string]map[string]any{}
	for i, behavior := range u.Behaviors {
		qs[coversKey(i)] = map[string]any{
			"type": "noul",
			"instructions": map[string]any{
				"question": "Do the tests in `tests` exercise the behavior below, which `spec` requires of the code in `implementation`?",
				"behavior": behavior,
				"guidance": "Answer yes only if at least one test would fail if this behavior were removed or broken. A test that merely mentions the feature without asserting on it does not count.",
			},
		}
	}
	qs["contradicts_spec"] = map[string]any{
		"type":         "noul",
		"instructions": "Does the code in `implementation` contradict a requirement stated in `spec`? Count only actual conflicts such as a different default value, a different error type, a different wire field name, or opposite behavior. Ignore omissions and ignore anything the spec leaves unspecified.",
		"criteria": map[string]any{
			"true":  "At least one statement in `spec` is contradicted by `implementation`.",
			"false": "`implementation` is consistent with every statement in `spec` that it addresses.",
		},
	}
	qs["thoroughness"] = map[string]any{
		"type":         "score",
		"instructions": "How thoroughly do the tests in `tests` cover the behaviors that `spec` requires of `implementation`?",
		"criteria": []any{
			"No test exercises this section's behavior.",
			"Only the success path is tested; error cases named in `spec` are not.",
			"The success path and the error cases named in `spec` are tested.",
			"Success, error cases, and the boundary values, cancellation, or concurrency conditions named in `spec` are all tested.",
		},
	}
	qs["weakest_area"] = map[string]any{
		"type":         "choice",
		"instructions": "Which area of `implementation` is least covered by `tests` relative to what `spec` requires? Choose none if coverage is even.",
		"criteria": map[string]any{
			"validation":    "Input checks and validation error paths.",
			"error_mapping": "Turning failures from dependencies into the error types the code exposes.",
			"retry":         "Backoff, retry limits, budgets, and cancellation.",
			"decoding":      "Turning input or response data into typed values.",
			"encoding":      "Turning typed values into output or wire formats.",
			"logging":       "What is and is not logged.",
			"none":          "No area stands out.",
		},
	}
	return qs
}

func evaluate(u Unit, answers map[string]typesafe.Answer, opts Options) UnitReport {
	r := UnitReport{Name: u.Name, Notes: u.Notes}
	fail := func(msg string) UnitReport {
		return UnitReport{Name: u.Name, Notes: u.Notes, Error: msg, Failing: true}
	}
	accepted := map[string]bool{}
	for _, a := range u.Accepted {
		accepted[a] = false
	}
	flag := func(key, msg string) {
		if _, ok := accepted[key]; ok {
			accepted[key] = true
			r.Flags = append(r.Flags, msg+" (accepted)")
			return
		}
		r.Flags = append(r.Flags, msg)
		r.Failing = true
	}
	for i, behavior := range u.Behaviors {
		id := coversKey(i)
		n, ok := answers[id].(typesafe.NoulAnswer)
		if !ok {
			return fail(missing(answers, id, "noul"))
		}
		r.Behaviors = append(r.Behaviors, BehaviorResult{ID: id, Behavior: behavior, Covered: n.Noul})
		if n.Noul < opts.MinCover {
			flag(behavior, fmt.Sprintf("%s: coverage %.2f < %.2f for %q", id, n.Noul, opts.MinCover, behavior))
		}
	}
	c, ok := answers["contradicts_spec"].(typesafe.NoulAnswer)
	if !ok {
		return fail(missing(answers, "contradicts_spec", "noul"))
	}
	r.Contradicts = c.Noul
	if r.Contradicts > opts.MaxContradict {
		flag("contradicts_spec", fmt.Sprintf("contradicts_spec: %.2f > %.2f", r.Contradicts, opts.MaxContradict))
	}
	s, ok := answers["thoroughness"].(typesafe.ScoreAnswer)
	if !ok {
		return fail(missing(answers, "thoroughness", "score"))
	}
	r.Thoroughness, r.ThoroughConf = s.Score, s.Confidence
	if r.Thoroughness < opts.MinThorough {
		flag("thoroughness", fmt.Sprintf("thoroughness: %.2f < %.2f", r.Thoroughness, opts.MinThorough))
	}
	w, ok := answers["weakest_area"].(typesafe.ChoiceAnswer)
	if !ok {
		return fail(missing(answers, "weakest_area", "choice"))
	}
	r.Weakest, r.WeakestConf = w.Choice, w.Confidence
	for _, a := range u.Accepted {
		if !accepted[a] {
			r.StaleAcceptances = append(r.StaleAcceptances, a)
		}
	}
	sort.Strings(r.StaleAcceptances)
	return r
}

func missing(answers map[string]typesafe.Answer, id, want string) string {
	a, ok := answers[id]
	if !ok {
		return fmt.Sprintf("answer %q is missing from the response", id)
	}
	return fmt.Sprintf("answer %q has type %q, want %q", id, a.AnswerType(), want)
}

// Markdown renders the summary table and the flagged units.
func (r Report) Markdown(opts Options) string {
	var b strings.Builder
	b.WriteString("# jev review\n\n| Unit | Behaviors covered | Contradicts | Thoroughness | Weakest | Flags |\n|---|---|---|---|---|---|\n")
	for _, u := range r.Units {
		covered := 0
		for _, br := range u.Behaviors {
			if br.Covered >= opts.MinCover {
				covered++
			}
		}
		status := fmt.Sprintf("%d", len(u.Flags))
		if u.Failing {
			status += " FAIL"
		}
		if u.Error != "" {
			status = "error"
		}
		fmt.Fprintf(&b, "| %s | %d/%d | %.2f | %.2f (%.2f) | %s | %s |\n", u.Name, covered, len(u.Behaviors), u.Contradicts, u.Thoroughness, u.ThoroughConf, u.Weakest, status)
	}
	b.WriteString("\n")
	for _, u := range r.Units {
		if u.Error != "" {
			fmt.Fprintf(&b, "## %s\n\nerror: %s\n\n", u.Name, u.Error)
			continue
		}
		if len(u.Flags) == 0 && len(u.StaleAcceptances) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", u.Name)
		if u.Truncated {
			b.WriteString("note: sources were truncated to fit the state budget\n\n")
		}
		for _, n := range u.Notes {
			b.WriteString("note: " + n + "\n")
		}
		if len(u.Notes) > 0 {
			b.WriteString("\n")
		}
		flags := append([]string(nil), u.Flags...)
		sort.Strings(flags)
		for _, f := range flags {
			b.WriteString("- " + f + "\n")
		}
		for _, a := range u.StaleAcceptances {
			b.WriteString("- acceptance did not fire: " + a + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}
```

Note the `weakest_area` label descriptions are reworded to be codebase-neutral (the old ones named this SDK's own types). The labels themselves are unchanged, so existing acceptances still match.

- [x] **Step 4: Write `internal/review/template.go`**

```go
package review

// Template returns a starter jev-review.json. Paths in it are examples.
func Template() []byte {
	return []byte(`{
  "description": "Units for jev review. Paths are relative to this file. Each unit names one spec section, the files that implement it, the files that test it, and the behaviors the section requires, one concrete claim per line.",
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
`)
}
```

- [x] **Step 5: Run tests, lint, commit**

```bash
go test -race -v ./internal/review/ 2>&1 | tail -20
golangci-lint run ./internal/review/
git add internal/review/review.go internal/review/template.go internal/review/review_test.go
git commit -m "Add internal/review package for spec-driven test coverage review"
```
Expected: all PASS, 0 issues. `tools/selfreview` still compiles and passes on its own; leave it for Task 3.

---
## Task 2: `jev review` and `jev review init`

**Agent:** `cli`

**Files:**
- Create: `internal/cli/review.go`, `internal/cli/review_test.go`
- Modify: `internal/cli/exit.go` (add `ExitFlagged`), `internal/cli/root.go` (register the command)

- [x] **Step 1: Write the failing tests `internal/cli/review_test.go`**

```go
package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// reviewServer answers any review request: covers_* nouls get cover, the
// contradiction noul 0.1, thoroughness 2.5, weakest_area "none". A non-200
// status returns an error body instead.
func reviewServer(t *testing.T, cover float64, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-typesafe-request-id", "req_review")
		if status != 200 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"detail":"no"}`))
			return
		}
		var req struct {
			Questions map[string]map[string]any `json:"questions"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		answers := map[string]any{}
		for id, q := range req.Questions {
			switch q["type"] {
			case "noul":
				p := 0.1
				if strings.HasPrefix(id, "covers_") {
					p = cover
				}
				answers[id] = map[string]any{"type": "noul", "noul": p}
			case "score":
				answers[id] = map[string]any{"type": "score", "score": 2.5, "confidence": 0.8, "legend": map[string]any{}, "probabilities": map[string]any{}}
			case "choice":
				answers[id] = map[string]any{"type": "choice", "choice": "none", "confidence": 0.6, "probabilities": map[string]any{}}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "jev-test", "answers": answers, "usage": map[string]int{"input_tokens": 10, "output_tokens": 1}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// reviewRepo writes a spec, an implementation file, a test file, and a config
// under a temp dir and returns the config and report paths.
func reviewRepo(t *testing.T) (cfg, report string) {
	t.Helper()
	dir := t.TempDir()
	must := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	must("spec.md", "# Spec\n\n## Retry\n\nretry text\n")
	must("retry.go", "package x\n")
	must("retry_test.go", "package x\n")
	must("jev-review.json", `{"spec":"spec.md","units":[{"name":"retry","spec_heading":"## Retry","implementation":["retry.go"],"tests":["retry_test.go"],"behaviors":["b0","b1"]}]}`)
	return filepath.Join(dir, "jev-review.json"), filepath.Join(dir, "report.json")
}

func reviewArgs(cfg, report, url string, extra ...string) []string {
	args := []string{"review", "--units", cfg, "--report", report, "--api-key", "k", "--base-url", url, "--max-retries", "0"}
	return append(args, extra...)
}

func TestReviewInitWritesAndRefusesOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jev-review.json")
	code, out, _ := run(t, "", "review", "init", "--units", path)
	if code != ExitOK || !strings.Contains(out, `"wrote"`) {
		t.Fatalf("exit %d out %s", code, out)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("template not written")
	}
	code, out, _ = run(t, "", "review", "init", "--units", path)
	if code != ExitUsage || !strings.Contains(out, `"kind":"usage"`) || !strings.Contains(out, "already exists") {
		t.Fatalf("second init: exit %d out %s", code, out)
	}
}

func TestReviewCleanRun(t *testing.T) {
	cfg, report := reviewRepo(t)
	srv := reviewServer(t, 0.9, 200)
	code, out, errOut := run(t, "", reviewArgs(cfg, report, srv.URL)...)
	if code != ExitOK {
		t.Fatalf("exit %d stdout %s stderr %s", code, out, errOut)
	}
	if !strings.Contains(out, "# jev review") || !strings.Contains(out, "| retry | 2/2 |") {
		t.Fatalf("markdown missing: %s", out)
	}
	if !strings.Contains(errOut, "review: ran 1 units") {
		t.Fatalf("stderr %q", errOut)
	}
	b, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	var units []map[string]any
	if err := json.Unmarshal(b, &units); err != nil || len(units) != 1 || units[0]["request_id"] != "req_review" {
		t.Fatalf("report %s err %v", b, err)
	}
}

func TestReviewFlaggedExits8(t *testing.T) {
	cfg, report := reviewRepo(t)
	srv := reviewServer(t, 0.2, 200)
	code, out, errOut := run(t, "", reviewArgs(cfg, report, srv.URL)...)
	if code != ExitFlagged {
		t.Fatalf("exit %d stdout %s stderr %s", code, out, errOut)
	}
	if !strings.Contains(out, "FAIL") || !strings.Contains(out, "covers_00") {
		t.Fatalf("flags missing from stdout: %s", out)
	}
	if !strings.Contains(errOut, "review: 1 of 1 units failing") {
		t.Fatalf("stderr %q", errOut)
	}
}

func TestReviewJSONOutput(t *testing.T) {
	cfg, report := reviewRepo(t)
	srv := reviewServer(t, 0.9, 200)
	code, out, _ := run(t, "", reviewArgs(cfg, report, srv.URL, "--json")...)
	var units []map[string]any
	if code != ExitOK || json.Unmarshal([]byte(out), &units) != nil || len(units) != 1 {
		t.Fatalf("exit %d out %s", code, out)
	}
}

func TestReviewUsageErrors(t *testing.T) {
	cfg, report := reviewRepo(t)
	srv := reviewServer(t, 0.9, 200)
	code, out, _ := run(t, "", reviewArgs(filepath.Join(t.TempDir(), "nope.json"), report, srv.URL)...)
	if code != ExitUsage || !strings.Contains(out, `"kind":"usage"`) {
		t.Fatalf("missing config: exit %d out %s", code, out)
	}
	code, out, _ = run(t, "", reviewArgs(cfg, report, srv.URL, "--only", "nope")...)
	if code != ExitUsage || !strings.Contains(out, "no units matched") {
		t.Fatalf("no match: exit %d out %s", code, out)
	}
}

func TestReviewAPIErrorClassified(t *testing.T) {
	cfg, report := reviewRepo(t)
	srv := reviewServer(t, 0.9, 401)
	code, out, _ := run(t, "", reviewArgs(cfg, report, srv.URL)...)
	if code != ExitAuth || !strings.Contains(out, `"kind":"auth"`) {
		t.Fatalf("exit %d out %s", code, out)
	}
	b, err := os.ReadFile(report)
	if err != nil || !strings.Contains(string(b), `"error"`) {
		t.Fatalf("report must still be written with the unit error: %v %s", err, b)
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

```bash
go test -run TestReview ./internal/cli/ 2>&1 | head -3
```
Expected: `ExitFlagged` undefined, and `unknown command "review"` once it compiles.

- [x] **Step 3: Add `ExitFlagged` to `internal/cli/exit.go`**

In the const block, after `ExitConnection`:

```go
	ExitFlagged     = 8   // jev review: at least one unit is failing; see the report
```
And extend the exit-code comment table in that file accordingly. `classify` is unchanged.

- [x] **Step 4: Write `internal/cli/review.go`**

```go
package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/therealbill/typesafe-go"
	"github.com/therealbill/typesafe-go/internal/review"
)

type reviewOptions struct {
	units  string
	report string
	json   bool
	opts   review.Options
}

func newReviewCmd(g *globals, streams IO, getenv func(string) string) *cobra.Command {
	o := &reviewOptions{opts: review.DefaultOptions()}
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Ask Jev whether your tests cover the behaviors your spec requires",
		Long: `Review a codebase against its specification.

For each unit in the config file, jev review sends the named spec section and
the implementation and test files to Jev and asks whether the tests exercise
each listed behavior, whether the implementation contradicts the spec, how
thorough the tests are, and which area is weakest. It reports probabilities
and flags readings past the thresholds.

It does not find bugs, review style, check security, or verify that the code
is correct. Readings drift between runs on identical input.

Start with: jev review init`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runReview(cmd, g, o, streams, getenv)
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.units, "units", "jev-review.json", "config file; paths inside it resolve relative to the file")
	f.StringVar(&o.report, "report", "jev-review-report.json", "where to write the JSON report")
	f.BoolVar(&o.json, "json", false, "print the report JSON on stdout instead of the Markdown summary")
	f.StringVar(&o.opts.Only, "only", "", "run a single unit by name")
	f.Float64Var(&o.opts.MinCover, "min-cover", o.opts.MinCover, "flag a behavior whose coverage probability is below this")
	f.Float64Var(&o.opts.MaxContradict, "max-contradict", o.opts.MaxContradict, "flag a unit whose contradiction probability is above this")
	f.Float64Var(&o.opts.MinThorough, "min-thorough", o.opts.MinThorough, "flag a unit whose thoroughness score is below this")
	f.IntVar(&o.opts.MaxStateBytes, "max-state-bytes", o.opts.MaxStateBytes, "byte cap for implementation and tests together")
	f.DurationVar(&o.opts.UnitTimeout, "unit-timeout", o.opts.UnitTimeout, "time allowed for one unit's call")
	cmd.AddCommand(newReviewInitCmd(streams))
	return cmd
}

func newReviewInitCmd(streams IO) *cobra.Command {
	var units string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a starter jev-review.json",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pretty, _ := cmd.Flags().GetBool("pretty")
			if _, err := os.Stat(units); err == nil {
				return fail(streams, pretty, &usageError{fmt.Errorf("%s already exists; remove it or pass --units", units)})
			}
			if err := os.WriteFile(units, review.Template(), 0o644); err != nil {
				return fail(streams, pretty, &usageError{err})
			}
			return writeJSON(streams.Out, map[string]string{"wrote": units}, pretty)
		},
	}
	cmd.Flags().StringVar(&units, "units", "jev-review.json", "path to write")
	return cmd
}

func runReview(cmd *cobra.Command, g *globals, o *reviewOptions, streams IO, getenv func(string) string) error {
	cfg, base, err := review.Load(o.units)
	if err != nil {
		return fail(streams, g.pretty, &usageError{err})
	}
	ctx := cmd.Context()
	shutdown, inst := setupTelemetry(ctx, g, streams, getenv)
	defer shutdown()
	client, err := typesafe.NewClient(g.clientOptions(streams, inst, cmd.Flags().Changed, getenv)...)
	if err != nil {
		return fail(streams, g.pretty, err)
	}
	defer func() { _ = client.Close() }()

	rep, err := review.Run(ctx, cfg, base, client, o.opts)
	if err != nil {
		return fail(streams, g.pretty, &usageError{err})
	}
	_, _ = fmt.Fprintf(streams.Err, "review: ran %d units\n", len(rep.Units))
	out, err := rep.JSON()
	if err != nil {
		return fail(streams, g.pretty, err)
	}
	if err := os.WriteFile(o.report, out, 0o644); err != nil {
		return fail(streams, g.pretty, &usageError{fmt.Errorf("write report: %w", err)})
	}
	if callErr := rep.FirstCallError(); callErr != nil {
		return fail(streams, g.pretty, callErr)
	}
	if o.json {
		_, _ = streams.Out.Write(out)
		_, _ = fmt.Fprintln(streams.Out)
	} else {
		_, _ = fmt.Fprint(streams.Out, rep.Markdown(o.opts))
	}
	if rep.Failing() {
		failing := 0
		for _, u := range rep.Units {
			if u.Failing {
				failing++
			}
		}
		_, _ = fmt.Fprintf(streams.Err, "review: %d of %d units failing\n", failing, len(rep.Units))
		return &ExitError{Code: ExitFlagged, Kind: "flagged", Err: errors.New("review flagged at least one unit")}
	}
	return nil
}
```

Check the exact signature of `g.clientOptions` in `root.go` (it takes `changed func(string) bool` since the review fixes) and how `ask.go` obtains `changed`; match it.

- [x] **Step 5: Register the command in `root.go`**

In `NewRootCmd`, add `newReviewCmd(g, streams, getenv)` to the `AddCommand` call.

- [x] **Step 6: Run tests, lint, commit**

```bash
go test -race ./internal/cli/ 2>&1 | tail -3
golangci-lint run ./internal/... ./cmd/...
go run ./cmd/jev review --help | head -12
git add internal/cli/review.go internal/cli/review_test.go internal/cli/exit.go internal/cli/root.go
git commit -m "Add jev review and jev review init subcommands"
```
Expected: PASS, 0 issues, help text shows the scope paragraph. Report "Task 2 done" to the lead; this unblocks the docs agent.

---

## Task 3: Repository config, Makefile, and tool removal

**Agent:** `cli`

**Files:**
- Move: `tools/selfreview/units.json` → `jev-review.json`
- Delete: `tools/selfreview/main.go`, `tools/selfreview/main_test.go`
- Modify: `Makefile`, `.gitignore`

- [ ] **Step 1: Move the config and add its description**

```bash
git mv tools/selfreview/units.json jev-review.json
python3 - <<'EOF'
import json
p='jev-review.json'; d=json.load(open(p))
d={"description":"Units for jev review of this repository. Paths are relative to this file.", **d}
json.dump(d, open(p,'w'), indent=2); open(p,'a').write("\n")
EOF
git rm -q tools/selfreview/main.go tools/selfreview/main_test.go
ls tools/
```
Expected: `tools/` now contains only `checkdocs.sh`.

- [ ] **Step 2: Makefile and .gitignore**

Replace the `selfreview` target with:

```make
review: build ## Run jev review against this repository (needs TYPESAFE_API_KEY)
	./bin/jev review
```
Update `.PHONY` (replace `selfreview` with `review`) and the `clean` target (replace `selfreview-report.json` with `jev-review-report.json`). In `.gitignore`, replace `selfreview-report.json` with `jev-review-report.json`.

- [ ] **Step 3: Verify, including a live run**

```bash
go build ./... && go test ./... 2>&1 | tail -6
make lint 2>&1 | tail -1
make review 2>&1 | tail -15; echo "exit ${PIPESTATUS[0]}"
./bin/jev review --only questions --json | python3 -c "import sys,json; d=json.load(sys.stdin); print('units', len(d), 'flags', d[0]['flags'])"
```
Expected: build and tests pass; `make review` prints the table with the same accepted flags as before (exit 0), or shows drift, which you report rather than accept; the `--json` run prints one unit. Note the report file name in `make review`'s output is `jev-review-report.json`, gitignored.

- [ ] **Step 4: Commit**

```bash
git add jev-review.json tools/selfreview Makefile .gitignore
git commit -m "Move the self-review config to jev-review.json and remove tools/selfreview"
```
(`git add tools/selfreview` stages the deletions.) Report hashes and the `make review` table.

---

## Task 4: The three pages

**Agent:** `docs` (after Task 2 is committed)

**Files:**
- Create: `docs/how-to/review-your-codebase-with-jev.md`, `docs/reference/jev-review.md`, `docs/explanation/what-jev-review-measures.md`
- Delete: `docs/how-to/run-the-self-review.md`, `docs/reference/self-review-tool.md`, `docs/explanation/what-the-self-review-measures.md`
- Modify: `docs/how-to/_index.md`, `docs/reference/_index.md`, `docs/explanation/_index.md`

Build the binary first (`make build`) so `./bin/jev review --help` and `./bin/jev review init --help` reflect Task 2. Task 3 may still be landing; the pages describe the subcommand, not the Makefile.

**Scope statement**, to appear as the first paragraph of each page, in these words or a close paraphrase that keeps every clause:

> `jev review` judges whether your tests exercise the behaviors you listed and whether your implementation contradicts the spec section you named. It sends those files to Jev and answers with probabilities. It does not find bugs, review style, check security, or verify that the code is correct, and its readings drift between runs on the same input.

**Prose rubric** for every writer: plain declaratives and imperatives; active voice; one topic per paragraph; no em dashes; no contrast-reveal sentences ("not X, but Y"); no narrated reasoning ("it's worth", "this matters because", "deliberately"); no humor or asides; no summary punchlines; descriptive headings; explanations describe in the third person and never defend the design. Facts, commands, outputs, and links are verified before writing.

Each new page's front matter carries `aliases` with the old site path so old links keep working:

| New page | `aliases` |
|---|---|
| `docs/how-to/review-your-codebase-with-jev.md` (weight 100) | `["/docs/how-to/run-the-self-review/"]` |
| `docs/reference/jev-review.md` (weight 80) | `["/docs/reference/self-review-tool/"]` |
| `docs/explanation/what-jev-review-measures.md` (weight 80) | `["/docs/explanation/what-the-self-review-measures/"]` |

- [ ] **Step 1: How-to** (`diataxis-docs:doc-howto-writer`)

Goal: review your own codebase with `jev review`. Prerequisites: a `jev` binary (release download or `make build`), `TYPESAFE_API_KEY`, a Markdown spec with headings. Steps: run `jev review init`; edit the file (one unit per spec section; paths relative to the file; `spec_heading` must match a heading line exactly); write behaviors as one concrete testable claim each, five to fifteen per unit, with two good and two bad examples; run `jev review`; read the table and the flagged lines; for each flag decide between adding a test (when the behavior is untested), fixing the spec or the code (when they disagree), or recording an acceptance with a note (when the reading is noise), and show the `accepted` and `notes` fields; rerun; tune thresholds with `--min-cover` and friends on your own data; use `--only` while iterating. Every command output comes from a real run against a scratch repository under the scratchpad with two or three units, using the live API. Use this repository's `jev-review.json` as the worked example of a mature config. Link to the reference and the explanation.

- [ ] **Step 2: Reference** (`diataxis-docs:doc-reference-gen`)

From source and `--help`: `jev review` and `jev review init`, every flag with its default, the config schema (each field, type, required or optional, path resolution rule, the acceptance rule and why `covers_NN` is rejected), the four question texts verbatim from `internal/review/review.go`, the report document schema (every `UnitReport` field), the Markdown table columns, thresholds and defaults, exit codes (0, 1, 3 to 7, 8) with the rule for which applies, and the `weakest_area` labels with their descriptions. Facts only.

- [ ] **Step 3: Explanation** (`diataxis-docs:doc-explanation-writer`)

What each question type measures and what its number means; why thresholds sit inside the model's drift band, with this repository's observed ranges as the example (contradiction readings of 0.35 to 0.50 on identical input, thoroughness 1.94 to 2.06); what acceptances and stale-acceptance lines record; how unit boundaries change readings (the retry unit moved from 1.68 to 2.99 when `transport.go` and `client_test.go` joined it); why it is a development aid and not a CI gate; what it cannot tell you.

- [ ] **Step 4: Landing pages and commit**

Replace the three old entries in the section `_index.md` lists with the new titles and paths. Then:

```bash
git rm -q docs/how-to/run-the-self-review.md docs/reference/self-review-tool.md docs/explanation/what-the-self-review-measures.md
./tools/checkdocs.sh
git add docs/how-to docs/reference docs/explanation
git commit -m "Rewrite the review documentation for any codebase"
```
`checkdocs.sh` may report links from other pages to the deleted files; Task 5 fixes those, so list them in the report rather than editing pages outside this task.

---

## Task 5: Cross-page updates and validation

**Agent:** `docs`

**Files:**
- Modify: `docs/reference/jev-cli.md`, `docs/reference/errors-and-exit-codes.md`, `docs/reference/makefile-and-repository-layout.md`, `docs/how-to/cut-a-release.md`, `docs/explanation/the-agent-facing-cli-contract.md`, `README.md`, `docs/superpowers/specs/2026-09-23-typesafe-go-sdk-design.md`, and any page `checkdocs.sh` flagged in Task 4

- [ ] **Step 1: Update each page from source**

- `jev-cli.md`: add the `review` and `review init` subcommands (flag tables from `--help`), exit code 8 in the table.
- `errors-and-exit-codes.md`: exit 8, kind `flagged`, and the rule that a unit's API failure uses the classifier's code while a failing unit without one exits 8.
- `makefile-and-repository-layout.md`: `review` target replaces `selfreview`; `tools/` holds only `checkdocs.sh`; `internal/review` listed; `jev-review.json` at the root.
- `cut-a-release.md`: replace any mention of `tools/selfreview` or `make selfreview`.
- `the-agent-facing-cli-contract.md`: the reviewer now calls the library directly; remove the sentence that it consumes the CLI as a subprocess and say the CLI tests cover that contract.
- `README.md`: the how-to line names the new page; add one sentence under the CLI section: "`jev review` asks Jev whether your tests cover the behaviors your spec requires; see the how-to."
- `docs/superpowers/specs/2026-09-23-typesafe-go-sdk-design.md`: one line under the Self-review section pointing to `2026-09-28-jev-review-design.md`.
- Fix every link `checkdocs.sh` flagged.

- [ ] **Step 2: Validate and commit**

Run `diataxis-docs:doc-crosslink-validator` over `docs/` and `README.md` with the same front-matter note as before (this project uses `title`, `description`, `diataxis`, `weight`, and now `aliases`). Fix what it reports. Then:

```bash
make docs && echo "docs ok"
git add README.md docs
git commit -m "Update pages for jev review and validate documentation"
```
Expected: `docs ok` with only the excludeFiles deprecation warning. Report the validator's counts.

---

## Task 6: Gate, push, deploy

**Agent:** lead (after Tasks 3 and 5)

- [ ] **Step 1: Gate**

```bash
make lint && make test && make docs && git status --short && echo "(clean)"
./bin/jev review init --units /tmp/jr/jev-review.json; ./bin/jev review init --units /tmp/jr/jev-review.json; echo "second init exit $?"
```
Expected: all pass; second init exits 1.

- [ ] **Step 2: Push and verify**

```bash
git push origin main
gh run watch $(gh run list --workflow "Deploy documentation site" --limit 1 --json databaseId -q '.[0].databaseId') --exit-status
curl -fsS https://therealbill.github.io/typesafe-go/docs/how-to/review-your-codebase-with-jev/ | grep -o '<title>[^<]*</title>'
curl -sS -o /dev/null -w '%{http_code}\n' https://therealbill.github.io/typesafe-go/docs/how-to/run-the-self-review/
curl -fsS https://therealbill.github.io/typesafe-go/docs/how-to/run-the-self-review/ | grep -o 'refresh[^>]*' | head -1
```
Expected: deploy success; new page title; the old URL returns 200 with a meta refresh to the new page.

- [ ] **Step 3: Tick the plan, commit, push, report.**
