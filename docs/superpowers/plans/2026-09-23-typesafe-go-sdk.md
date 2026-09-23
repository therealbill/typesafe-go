# TypeSafe Go SDK and `jev` CLI Implementation Plan

> **For agentic workers:** Execute this plan with an **Agent Team** of up to four concurrent named agents (Agent tool with `name`, coordinated by the lead through SendMessage). Do NOT use git worktrees, do NOT use unnamed parallel subagents as the execution method, and do NOT use the `superpowers:executing-plans` skill. The roster, models, and dependency graph are in "Execution Model". Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A stdlib-only Go library covering the TypeSafe Python SDK's public surface for the System One API, an OpenTelemetry subpackage, a Cobra `jev` CLI, a Jev-driven self-review tool, and Diátaxis docs.

**Architecture:** Package `typesafe` owns validation, transport, retries, and decoding with no third-party imports and exposes an `Instrumentation` hook. Package `typesafe/otel` implements the hook with the OpenTelemetry API. `internal/cli` holds the Cobra commands and Honeycomb wiring through `otelconf`; `cmd/jev/main.go` is a one-liner. `tools/selfreview` drives `jev ask` over spec/implementation/test triples.

**Tech Stack:** Go 1.25, `encoding/json`, `net/http`, `log/slog`, `math/rand/v2`; `github.com/spf13/cobra v1.10.2`; `go.opentelemetry.io/otel v1.46.0`, `otel/sdk v1.46.0`, `contrib/instrumentation/net/http/otelhttp v0.71.0`, `contrib/otelconf v0.26.0`; golangci-lint v2; goreleaser v2.

**Spec:** `docs/superpowers/specs/2026-09-23-typesafe-go-sdk-design.md`. Read it before starting any task.

---

## Execution Model

### Team roster

| Agent name | Type / model | Owns | Skills and subagents to use |
|---|---|---|---|
| `core` | `go-architect`, model **opus** | Tasks 1–9 (scaffold, library, typed decode, integration test), Task 20 (self-review tool) | `superpowers:test-driven-development`, `superpowers:verification-before-completion` |
| `otel` | `go-architect`, model **sonnet** | Task 10 (otel package, hook-level tests), Task 11 (end-to-end otel test) | `superpowers:test-driven-development` |
| `cli` | `go-architect`, model **opus** | Task 12 (tooling: Makefile, lint, gitignore, CI, goreleaser), Tasks 13–19 (CLI) | `superpowers:test-driven-development`, `gnu-make:makefile-fundamentals` |
| `docs` | `general-purpose`, model **sonnet** | Tasks 21–25 (Diátaxis docs, README) | `diataxis-docs:doc-explanation-writer`, `diataxis-docs:doc-reference-gen`, `diataxis-docs:doc-tutorial-writer`, `diataxis-docs:doc-howto-writer`, `diataxis-docs:doc-crosslink-validator` |
| lead (this session) | — | Task 26 (self-review run and fix loop), Task 27 (final review) | `superpowers:requesting-code-review` with `superpowers:code-reviewer` |

### Dependency graph

```
Task 1 (scaffold) ──┬─→ Tasks 2..9 (core, sequential)
                    ├─→ Task 10 (otel hook)  ──→ Task 11 (needs Task 7)
                    ├─→ Task 12 (tooling)    ──→ Tasks 13..19 (cli, need Task 7)
                    └─→ Task 21 (explanation docs, from spec)
Task 7 (client) ─────→ Tasks 11, 13
Task 19 (cli done) ──→ Task 20 (selfreview tool) ──→ Task 26 (run + fix loop)
Tasks 9,11,19 ───────→ Tasks 22..25 (reference, tutorials, how-to, README)
Everything ──────────→ Task 27 (final review)
```

Start order for the lead: spawn `core` on Task 1. When `core` reports Task 1 committed, spawn `otel` (Task 10), `cli` (Task 12), and `docs` (Task 21) concurrently. Gate later tasks on the commits named above.

### Rules for every agent

1. Work in `/Users/bill/Projects/gojev` directly. No worktrees, no branches other than `main`.
2. Touch only the files listed under your task. If you need a change in another agent's file, message the lead.
3. TDD per task: write the test, run it and see it fail, implement, run it and see it pass, commit.
4. Stage by explicit path only: `git add path/one.go path/one_test.go`. Never `git add -A` or `git add .`.
5. Before every commit run `gofmt -l . && go vet ./... && go build ./... && go test ./...` and paste the last lines of output in your report. If `git commit` fails with `index.lock`, wait two seconds and retry once.
6. Report to the lead after each task: task number, commit hash, test output tail, anything skipped.
7. No "Authored by" or similar lines in files or commit messages.
8. `TYPESAFE_API_KEY` is in the environment. Never print it, log it, or write it to a file.

---

## File map

| Path | Responsibility | Task |
|---|---|---|
| `go.mod`, `doc.go`, `internal/version/version.go`, `instrument.go` | Module, package docs, version, hook interface | 1 |
| `question.go`, `question_test.go` | Question types, wire marshaling, client-side validation | 2 |
| `errors.go`, `errors_test.go` | Error types, `Message()`, predicates | 3 |
| `response.go`, `response_test.go`, `models.go`, `testdata/*.json` | Answer decoding, accessors, models list | 4 |
| `retry.go`, `retry_test.go` | Retry policy, backoff, `Retry-After` | 5 |
| `transport.go` | Attempt loop, header handling, error mapping | 6 |
| `client.go`, `client_test.go` | Client, options, `SystemOne`, `ListModels`, fake-server tests | 7 |
| `typed.go`, `typed_test.go` | `SystemOneAs[T]` | 8 |
| `integration_test.go` | Live API test, env-gated | 9 |
| `otel/otel.go`, `otel/otel_test.go` | Instrumentation implementation | 10, 11 |
| `Makefile`, `.golangci.yml`, `.gitignore`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.goreleaser.yaml` | Tooling | 12 |
| `internal/cli/exit.go`, `exit_test.go` | Exit codes, error JSON | 13 |
| `internal/cli/request.go`, `request_test.go` | JSON and flag request parsing | 14 |
| `internal/cli/root.go`, `internal/cli/output.go`, `cmd/jev/main.go` | Root command, globals, `Main` | 15 |
| `internal/cli/ask.go`, `internal/cli/models.go`, `internal/cli/version.go`, `internal/cli/cli_test.go` | Commands and in-process tests | 16, 17 |
| `internal/cli/telemetry.go`, `telemetry_test.go` | `otelconf` wiring | 18 |
| `tools/selfreview/main.go`, `tools/selfreview/units.json` | Self-review driver | 20 |
| `docs/explanation/*.md`, `docs/reference/*.md`, `docs/tutorials/*.md`, `docs/how-to/*.md`, `README.md` | Diátaxis docs | 21–25 |

---

## Fixtures (captured from the live API on 2026-09-23)

Use these byte-for-byte in tests. They live in `testdata/` after Task 4.

**Request sent** (`testdata/request.json`):

```json
{"model":"jev-latest","state":{"ticket":"I was charged twice this month and nobody answers my emails. Fix it now."},"questions":{"billing":{"type":"noul","instructions":"Is `ticket` about a billing problem?"},"tone":{"type":"choice","instructions":"What is the tone of `ticket`?","criteria":{"calm":"Polite and patient","angry":"Frustrated or hostile","neutral":null}},"urgency":{"type":"score","instructions":"How urgent is `ticket`?","criteria":["Not urgent at all","Somewhat urgent","Very urgent"]}}}
```

**Response 200** (`testdata/systemone_ok.json`), header `x-typesafe-request-id: req_01a0d07208027e0bbeb09c9f9b2b205a`:

```json
{"model":"jev-1.13.0","answers":{"billing":{"type":"noul","noul":0.99},"tone":{"type":"choice","choice":"angry","confidence":1.0,"probabilities":{"neutral":0.0,"angry":1.0,"calm":0.0}},"urgency":{"type":"score","score":1.9,"confidence":0.85,"legend":{"0":"Not urgent at all","1":"Somewhat urgent","2":"Very urgent"},"probabilities":{"0":0.0,"1":0.1,"2":0.9}}},"usage":{"input_tokens":407,"output_tokens":72}}
```

**Models 200** (`testdata/models_ok.json`):

```json
{"models":[{"name":"jev-latest","description":"The latest iteration of TypeSafe's System One Model: Jev","release_date":"2026-09-10T18:38:01.391457+00:00"},{"name":"jev-preview","description":"A preview version of `jev-latest`: should be better in most ways","release_date":"2026-09-10T18:39:06.057655+00:00"}]}
```

**Error bodies:**

| Status | Body |
|---|---|
| 401 | `{"detail":{"error_type":"authentication_error","message":"Cannot authenticate with the server. Please check your API key and try again."}}` |
| 422 | `{"detail":[{"type":"too_short","loc":["body","questions"],"msg":"Dictionary should have at least 1 item after validation, not 0","input":{},"ctx":{"field_type":"Dictionary","min_length":1,"actual_length":0}}]}` |
| 400 | `{"detail":{"error_type":"api_usage_error","message":"Invalid request."}}` |
| 404 | `{"detail":"Not Found"}` |

---
## Task 1: Scaffold module, version, hook interface

**Agent:** `core`

**Files:**
- Create: `go.mod`, `doc.go`, `internal/version/version.go`, `instrument.go`

- [ ] **Step 1: Create the module**

```bash
cd /Users/bill/Projects/gojev
go mod init github.com/therealbill/typesafe-go
go mod edit -go=1.25
cat go.mod
```
Expected: `module github.com/therealbill/typesafe-go` and `go 1.25`.

- [ ] **Step 2: Write `doc.go`**

```go
// Package typesafe is a client for the TypeSafe System One API.
//
// System One models such as Jev answer named, typed questions about a piece
// of state: a Noul returns the probability that a condition holds, a Choice
// selects one label from a set, and a Score places the state on an ordered
// rubric. Build a Client with NewClient, then call SystemOne with the state
// and a Questions map.
//
//	client, err := typesafe.NewClient() // reads TYPESAFE_API_KEY
//	res, err := client.SystemOne(ctx, "I was charged twice.", typesafe.Questions{
//	    "billing": typesafe.Noul{Instructions: "Is this about billing?"},
//	})
//	p := res.Nouls()["billing"].Noul
//
// The package depends only on the standard library. Tracing lives in the
// otel subpackage and is attached with WithInstrumentation.
package typesafe
```

- [ ] **Step 3: Write `internal/version/version.go`**

```go
// Package version holds build information set at link time.
package version

// Version is the module version, overridden by -ldflags at release time.
var Version = "dev"

// Commit is the short git commit, overridden by -ldflags at release time.
var Commit = "none"
```

- [ ] **Step 4: Write `instrument.go`**

```go
package typesafe

import (
	"context"
	"net/http"
)

// RequestInfo describes a call about to be made. It is passed to
// Instrumentation.RequestStart before the first HTTP attempt.
type RequestInfo struct {
	// Operation is "system_one" or "list_models".
	Operation string
	// Model is the requested model name or alias.
	Model string
	// QuestionCount and the per-type counts describe the questions map.
	QuestionCount int
	NoulCount     int
	ChoiceCount   int
	ScoreCount    int
	// State and Questions are the request inputs. Instrumentation must not
	// record them unless the caller opted in.
	State     any
	Questions Questions
}

// RequestResult describes how a call ended. It is passed to the function
// returned by Instrumentation.RequestStart exactly once.
type RequestResult struct {
	// Attempts is the number of HTTP attempts made, including the first.
	Attempts int
	// Status is the final HTTP status, or 0 if no response was received.
	Status int
	// RequestID is the x-typesafe-request-id header of the final response.
	RequestID string
	// Model is the model reported by the response, if any.
	Model string
	// Usage is the token usage reported by the response, if any.
	Usage Usage
	// Response is the decoded response for system_one, nil otherwise or on error.
	Response *SystemOneResponse
	// Err is the error returned to the caller, nil on success.
	Err error
}

// Instrumentation observes client calls. The otel subpackage provides an
// OpenTelemetry implementation.
type Instrumentation interface {
	// RequestStart is called once per SystemOne or ListModels call. The
	// returned context is used for every HTTP attempt. The returned function
	// is called exactly once when the call finishes.
	RequestStart(ctx context.Context, info RequestInfo) (context.Context, func(RequestResult))
	// Transport wraps the HTTP round tripper used by the client. It is called
	// once at client construction and may return rt unchanged.
	Transport(rt http.RoundTripper) http.RoundTripper
}
```

Note: `Usage` and `SystemOneResponse` are defined in Task 4. Until then the package will not compile, so add this temporary file and delete it in Task 4:

`response.go` (temporary):
```go
package typesafe

// Usage is defined fully in Task 4.
type Usage struct{ InputTokens, OutputTokens *int }

// SystemOneResponse is defined fully in Task 4.
type SystemOneResponse struct{}
```

- [ ] **Step 5: Verify build and commit**

```bash
gofmt -l . ; go vet ./... && go build ./...
git add go.mod doc.go internal/version/version.go instrument.go response.go
git commit -m "Scaffold typesafe-go module with version and instrumentation hook"
```
Expected: no gofmt output, no vet errors, clean build.

---

## Task 2: Question types, marshaling, validation

**Agent:** `core`

**Files:**
- Create: `question.go`, `question_test.go`
- Create: `errors.go` (only `ValidationError`, the rest comes in Task 3)

- [ ] **Step 1: Write `errors.go` with just `ValidationError`**

```go
package typesafe

import "fmt"

// ValidationError reports a request that failed client-side validation
// before any network call. Path names the offending field, for example
// "questions.tone.criteria".
type ValidationError struct {
	Path string
	Err  error
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("typesafe: invalid request: %s: %v", e.Path, e.Err)
}

func (e *ValidationError) Unwrap() error { return e.Err }
```

- [ ] **Step 2: Write the failing tests `question_test.go`**

```go
package typesafe

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestQuestionMarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		q    Question
		want string
	}{
		{"noul plain", Noul{Instructions: "Is it about billing?"}, `{"instructions":"Is it about billing?","type":"noul"}`},
		{"noul no instructions", Noul{}, `{"type":"noul"}`},
		{"noul criteria", Noul{Instructions: "x", Criteria: &NoulCriteria{True: "yes", False: "no"}},
			`{"criteria":{"false":"no","true":"yes"},"instructions":"x","type":"noul"}`},
		{"noul criteria partial", Noul{Instructions: "x", Criteria: &NoulCriteria{True: "yes"}},
			`{"criteria":{"true":"yes"},"instructions":"x","type":"noul"}`},
		{"choice", Choice{Instructions: "tone", Criteria: map[string]JSONContent{"calm": "Polite", "neutral": nil}},
			`{"criteria":{"calm":"Polite","neutral":null},"instructions":"tone","type":"choice"}`},
		{"choice structured", Choice{Instructions: map[string]any{"q": "tone"}, Criteria: map[string]JSONContent{"a": []any{"x"}}},
			`{"criteria":{"a":["x"]},"instructions":{"q":"tone"},"type":"choice"}`},
		{"score", Score{Instructions: "how urgent", Criteria: []JSONContent{"low", "high"}},
			`{"criteria":["low","high"],"instructions":"how urgent","type":"score"}`},
		{"raw", RawQuestion{"type": "noul", "instructions": "x", "weight": 2},
			`{"instructions":"x","type":"noul","weight":2}`},
		{"pointer noul", &Noul{Instructions: "p"}, `{"instructions":"p","type":"noul"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.q)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("got %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestQuestionsMarshalJSON(t *testing.T) {
	qs := Questions{
		"b": Score{Instructions: "s", Criteria: []JSONContent{"0", "1"}},
		"a": Noul{Instructions: "n"},
	}
	got, err := json.Marshal(qs)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":{"instructions":"n","type":"noul"},"b":{"criteria":["0","1"],"instructions":"s","type":"score"}}`
	if string(got) != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

func TestValidateQuestions(t *testing.T) {
	big := map[string]JSONContent{}
	for i := 0; i < 256; i++ {
		big[strings.Repeat("x", i+1)] = nil
	}
	tests := []struct {
		name     string
		qs       Questions
		wantPath string
		wantMsg  string
	}{
		{"empty map", Questions{}, "questions", "at least one question"},
		{"nil map", nil, "questions", "at least one question"},
		{"empty key", Questions{"": Noul{}}, "questions", "must not be empty"},
		{"nil question", Questions{"q": nil}, "questions.q", "nil"},
		{"nil pointer", Questions{"q": (*Noul)(nil)}, "questions.q", "nil"},
		{"noul bad instructions", Questions{"q": Noul{Instructions: 42}}, "questions.q.instructions", "text, an object, or an array"},
		{"noul bad criteria", Questions{"q": Noul{Instructions: "x", Criteria: &NoulCriteria{True: true}}}, "questions.q.criteria.true", "text, an object, or an array"},
		{"choice no labels", Questions{"q": Choice{Instructions: "x"}}, "questions.q.criteria", "at least one label"},
		{"choice too many", Questions{"q": Choice{Instructions: "x", Criteria: big}}, "questions.q.criteria", "at most 255"},
		{"choice empty label", Questions{"q": Choice{Criteria: map[string]JSONContent{"": nil}}}, "questions.q.criteria", "labels must not be empty"},
		{"choice bad description", Questions{"q": Choice{Criteria: map[string]JSONContent{"a": 1.5}}}, "questions.q.criteria.a", "text, an object, or an array"},
		{"score one level", Questions{"q": Score{Criteria: []JSONContent{"only"}}}, "questions.q.criteria", "between 2 and 10"},
		{"score eleven levels", Questions{"q": Score{Criteria: make([]JSONContent, 11)}}, "questions.q.criteria", "between 2 and 10"},
		{"score nil level", Questions{"q": Score{Criteria: []JSONContent{"a", nil}}}, "questions.q.criteria[1]", "required"},
		{"raw no type", Questions{"q": RawQuestion{"instructions": "x"}}, "questions.q.type", "type"},
		{"raw empty type", Questions{"q": RawQuestion{"type": ""}}, "questions.q.type", "type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateQuestions(tt.qs)
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want *ValidationError, got %v", err)
			}
			if ve.Path != tt.wantPath {
				t.Fatalf("path: got %q want %q", ve.Path, tt.wantPath)
			}
			if !strings.Contains(ve.Err.Error(), tt.wantMsg) {
				t.Fatalf("message: got %q want substring %q", ve.Err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestValidateQuestionsOK(t *testing.T) {
	type structured struct {
		Q string `json:"q"`
	}
	qs := Questions{
		"n":  Noul{Instructions: "x", Criteria: &NoulCriteria{True: "yes"}},
		"np": &Noul{Instructions: "x"},
		"c":  Choice{Instructions: structured{Q: "tone"}, Criteria: map[string]JSONContent{"a": nil, "b": "desc"}},
		"s":  Score{Criteria: []JSONContent{"a", map[string]any{"desc": "b"}}},
		"r":  RawQuestion{"type": "future", "anything": 1},
		"j":  Noul{Instructions: json.RawMessage(`{"q":"raw"}`)},
	}
	if err := validateQuestions(qs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateContentState(t *testing.T) {
	if err := validateContent("state", nil, false); err == nil {
		t.Fatal("nil state should fail")
	}
	if err := validateContent("state", 7, false); err == nil {
		t.Fatal("numeric state should fail")
	}
	for _, v := range []any{"text", map[string]any{"a": 1}, []string{"a"}, struct{ A int }{1}, &struct{ A int }{1}} {
		if err := validateContent("state", v, false); err != nil {
			t.Fatalf("%T should be valid: %v", v, err)
		}
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./... 2>&1 | head -5
```
Expected: compile errors naming `Noul`, `validateQuestions`, and friends as undefined.

- [ ] **Step 4: Write `question.go`**

```go
package typesafe

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
)

// JSONContent is text, a JSON object, or a JSON array. Valid values are a
// string, a map with string keys, a slice or array, json.RawMessage, or a
// struct (or pointer to one) that encodes to a JSON object.
type JSONContent = any

// Question is one of Noul, Choice, Score, or RawQuestion. Pointers to the
// struct types are accepted too.
type Question interface{ question() }

// Questions maps question identifiers to questions. Identifiers name the
// answers in the response; they are not shown to the model.
type Questions map[string]Question

// NoulCriteria optionally describes the yes and no outcomes of a Noul.
type NoulCriteria struct {
	True  JSONContent
	False JSONContent
}

// Noul asks whether a condition holds. The answer is the probability of yes.
type Noul struct {
	Instructions JSONContent
	Criteria     *NoulCriteria
}

// Choice asks for one label out of a defined set. Criteria maps each label to
// a description, or nil to leave it undescribed. Between 1 and 255 labels.
type Choice struct {
	Instructions JSONContent
	Criteria     map[string]JSONContent
}

// Score asks for a position on an ordered rubric. Criteria[i] describes
// score i. Between 2 and 10 levels.
type Score struct {
	Instructions JSONContent
	Criteria     []JSONContent
}

// RawQuestion is sent to the API unchanged. It must carry a string "type"
// field. Use it for question fields this package does not model yet.
type RawQuestion map[string]any

func (Noul) question()        {}
func (Choice) question()      {}
func (Score) question()       {}
func (RawQuestion) question() {}

// MarshalJSON encodes the question in the API's wire format.
func (q Noul) MarshalJSON() ([]byte, error) {
	out := map[string]any{"type": "noul"}
	if q.Instructions != nil {
		out["instructions"] = q.Instructions
	}
	if q.Criteria != nil {
		crit := map[string]any{}
		if q.Criteria.True != nil {
			crit["true"] = q.Criteria.True
		}
		if q.Criteria.False != nil {
			crit["false"] = q.Criteria.False
		}
		out["criteria"] = crit
	}
	return json.Marshal(out)
}

// MarshalJSON encodes the question in the API's wire format.
func (q Choice) MarshalJSON() ([]byte, error) {
	out := map[string]any{"type": "choice", "criteria": q.Criteria}
	if q.Instructions != nil {
		out["instructions"] = q.Instructions
	}
	return json.Marshal(out)
}

// MarshalJSON encodes the question in the API's wire format.
func (q Score) MarshalJSON() ([]byte, error) {
	out := map[string]any{"type": "score", "criteria": q.Criteria}
	if q.Instructions != nil {
		out["instructions"] = q.Instructions
	}
	return json.Marshal(out)
}

const (
	maxChoiceLabels = 255
	minScoreLevels  = 2
	maxScoreLevels  = 10
)

func validateQuestions(qs Questions) error {
	if len(qs) == 0 {
		return &ValidationError{Path: "questions", Err: errors.New("at least one question is required")}
	}
	keys := make([]string, 0, len(qs))
	for k := range qs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k == "" {
			return &ValidationError{Path: "questions", Err: errors.New("question identifiers must not be empty")}
		}
		if err := validateQuestion("questions."+k, qs[k]); err != nil {
			return err
		}
	}
	return nil
}

func validateQuestion(path string, q Question) error {
	switch v := q.(type) {
	case nil:
		return &ValidationError{Path: path, Err: errors.New("question is nil")}
	case *Noul:
		if v == nil {
			return &ValidationError{Path: path, Err: errors.New("question is nil")}
		}
		return validateQuestion(path, *v)
	case *Choice:
		if v == nil {
			return &ValidationError{Path: path, Err: errors.New("question is nil")}
		}
		return validateQuestion(path, *v)
	case *Score:
		if v == nil {
			return &ValidationError{Path: path, Err: errors.New("question is nil")}
		}
		return validateQuestion(path, *v)
	case Noul:
		if err := validateContent(path+".instructions", v.Instructions, true); err != nil {
			return err
		}
		if v.Criteria != nil {
			if err := validateContent(path+".criteria.true", v.Criteria.True, true); err != nil {
				return err
			}
			if err := validateContent(path+".criteria.false", v.Criteria.False, true); err != nil {
				return err
			}
		}
		return nil
	case Choice:
		if err := validateContent(path+".instructions", v.Instructions, true); err != nil {
			return err
		}
		if len(v.Criteria) == 0 {
			return &ValidationError{Path: path + ".criteria", Err: errors.New("choice requires at least one label")}
		}
		if len(v.Criteria) > maxChoiceLabels {
			return &ValidationError{Path: path + ".criteria", Err: fmt.Errorf("choice allows at most %d labels, got %d", maxChoiceLabels, len(v.Criteria))}
		}
		labels := make([]string, 0, len(v.Criteria))
		for label := range v.Criteria {
			labels = append(labels, label)
		}
		sort.Strings(labels)
		for _, label := range labels {
			if label == "" {
				return &ValidationError{Path: path + ".criteria", Err: errors.New("choice labels must not be empty")}
			}
			if err := validateContent(path+".criteria."+label, v.Criteria[label], true); err != nil {
				return err
			}
		}
		return nil
	case Score:
		if err := validateContent(path+".instructions", v.Instructions, true); err != nil {
			return err
		}
		if n := len(v.Criteria); n < minScoreLevels || n > maxScoreLevels {
			return &ValidationError{Path: path + ".criteria", Err: fmt.Errorf("score requires between %d and %d levels, got %d", minScoreLevels, maxScoreLevels, n)}
		}
		for i, level := range v.Criteria {
			if err := validateContent(fmt.Sprintf("%s.criteria[%d]", path, i), level, false); err != nil {
				return err
			}
		}
		return nil
	case RawQuestion:
		t, ok := v["type"].(string)
		if !ok || t == "" {
			return &ValidationError{Path: path + ".type", Err: errors.New(`raw question requires a non-empty string "type"`)}
		}
		return nil
	default:
		return &ValidationError{Path: path, Err: fmt.Errorf("unsupported question type %T", q)}
	}
}

// validateContent checks that v is acceptable JSON content. When optional is
// true a nil value passes.
func validateContent(path string, v any, optional bool) error {
	if v == nil {
		if optional {
			return nil
		}
		return &ValidationError{Path: path, Err: errors.New("value is required")}
	}
	if _, ok := v.(json.RawMessage); ok {
		return nil
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			if optional {
				return nil
			}
			return &ValidationError{Path: path, Err: errors.New("value is required")}
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.String, reflect.Map, reflect.Slice, reflect.Array, reflect.Struct:
		return nil
	default:
		return &ValidationError{Path: path, Err: fmt.Errorf("must be text, an object, or an array, got %T", v)}
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test -run 'TestQuestion|TestValidate' -v . 2>&1 | tail -15
```
Expected: all `--- PASS`, ending `ok  github.com/therealbill/typesafe-go`.

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; go vet ./... && go test ./...
git add question.go question_test.go errors.go
git commit -m "Add question types with wire marshaling and validation"
```

---
## Task 3: Error types and predicates

**Agent:** `core`

**Files:**
- Modify: `errors.go` (replace whole file)
- Create: `errors_test.go`

- [ ] **Step 1: Write the failing tests `errors_test.go`**

```go
package typesafe

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAPIErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"detail object", `{"detail":{"error_type":"authentication_error","message":"Cannot authenticate with the server. Please check your API key and try again."}}`,
			"authentication_error: Cannot authenticate with the server. Please check your API key and try again."},
		{"detail array", `{"detail":[{"type":"too_short","loc":["body","questions"],"msg":"Dictionary should have at least 1 item after validation, not 0","input":{}}]}`,
			"body.questions: Dictionary should have at least 1 item after validation, not 0"},
		{"detail string", `{"detail":"Not Found"}`, "Not Found"},
		{"message field", `{"message":"boom"}`, "boom"},
		{"error field", `{"error":"bad"}`, "bad"},
		{"plain text", `  service unavailable  `, "service unavailable"},
		{"empty", ``, ""},
		{"long text", strings.Repeat("x", 300), strings.Repeat("x", 200) + "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &APIError{Status: 400, Body: []byte(tt.body)}
			if got := e.Message(); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestAPIErrorError(t *testing.T) {
	e := &APIError{Status: 404, Endpoint: "GET /v1/nope", Body: []byte(`{"detail":"Not Found"}`), RequestID: "req_1"}
	want := "typesafe: GET /v1/nope returned 404: Not Found (request id req_1)"
	if e.Error() != want {
		t.Fatalf("got %q want %q", e.Error(), want)
	}
	e2 := &APIError{Status: 500, Endpoint: "POST /v1/systemone"}
	if e2.Error() != "typesafe: POST /v1/systemone returned 500" {
		t.Fatalf("got %q", e2.Error())
	}
}

func TestRateLimitErrorUnwrapsToAPIError(t *testing.T) {
	var err error = &RateLimitError{APIError: APIError{Status: 429}, RetryAfter: 2 * time.Second}
	var api *APIError
	if !errors.As(err, &api) || api.Status != 429 {
		t.Fatal("RateLimitError should unwrap to *APIError")
	}
	if !IsRateLimited(err) {
		t.Fatal("IsRateLimited should be true")
	}
	if IsAuthError(err) {
		t.Fatal("IsAuthError should be false")
	}
}

func TestIsAuthError(t *testing.T) {
	for _, s := range []int{401, 403} {
		if !IsAuthError(&APIError{Status: s}) {
			t.Fatalf("status %d should be auth error", s)
		}
	}
	if IsAuthError(&APIError{Status: 404}) {
		t.Fatal("404 is not an auth error")
	}
	if IsAuthError(errors.New("x")) {
		t.Fatal("plain error is not an auth error")
	}
}

func TestTimeoutErrorUnwrapsToConnectionError(t *testing.T) {
	var err error = &TimeoutError{ConnectionError: ConnectionError{Err: context.DeadlineExceeded}, Timeout: time.Second}
	var ce *ConnectionError
	if !errors.As(err, &ce) {
		t.Fatal("TimeoutError should unwrap to *ConnectionError")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("should unwrap to the cause")
	}
	if !strings.Contains(err.Error(), "timed out after 1s") {
		t.Fatalf("message: %q", err.Error())
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{&APIError{Status: 429}, true},
		{&RateLimitError{APIError: APIError{Status: 429}}, true},
		{&APIError{Status: 503}, true},
		{&APIError{Status: 529}, true},
		{&APIError{Status: 408}, true},
		{&APIError{Status: 422}, false},
		{&APIError{Status: 401}, false},
		{&ConnectionError{Err: errors.New("refused")}, true},
		{&TimeoutError{ConnectionError: ConnectionError{Err: errors.New("t")}}, true},
		{&ValidationError{Path: "x", Err: errors.New("bad")}, false},
		{&ResponseValidationError{FieldPath: "answers.a", Err: errors.New("bad")}, false},
		{errors.New("other"), false},
	}
	for _, tt := range tests {
		if got := IsRetryable(tt.err); got != tt.want {
			t.Errorf("IsRetryable(%v) = %v want %v", tt.err, got, tt.want)
		}
	}
}

func TestResponseValidationErrorMessage(t *testing.T) {
	e := &ResponseValidationError{FieldPath: "answers.tone.confidence", Err: errors.New("missing")}
	if e.Error() != `typesafe: invalid response field "answers.tone.confidence": missing` {
		t.Fatalf("got %q", e.Error())
	}
}

func TestAPIErrorHeadersNil(t *testing.T) {
	e := &APIError{Status: 500}
	if e.Headers.Get("x") != "" {
		t.Fatal("nil headers must be safe")
	}
	_ = http.Header{}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -run 'TestAPIError|TestRateLimit|TestIsAuth|TestTimeoutError|TestIsRetryable|TestResponseValidation' . 2>&1 | head -5
```
Expected: compile errors for `APIError`, `RateLimitError`, and friends.

- [ ] **Step 3: Replace `errors.go`**

```go
package typesafe

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ErrMissingAPIKey is returned by NewClient when no API key is configured.
var ErrMissingAPIKey = errors.New("typesafe: API key is required: pass WithAPIKey or set TYPESAFE_API_KEY")

// ValidationError reports a request that failed client-side validation
// before any network call. Path names the offending field, for example
// "questions.tone.criteria".
type ValidationError struct {
	Path string
	Err  error
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("typesafe: invalid request: %s: %v", e.Path, e.Err)
}

func (e *ValidationError) Unwrap() error { return e.Err }

// APIError is returned when the API responds with a 4xx or 5xx status.
type APIError struct {
	// Status is the HTTP status code.
	Status int
	// Body is the raw response body.
	Body []byte
	// Headers are the response headers.
	Headers http.Header
	// Endpoint is the method and path, for example "POST /v1/systemone".
	Endpoint string
	// RequestID is the x-typesafe-request-id response header, if present.
	RequestID string
}

func (e *APIError) Error() string {
	s := fmt.Sprintf("typesafe: %s returned %d", e.Endpoint, e.Status)
	if msg := e.Message(); msg != "" {
		s += ": " + msg
	}
	if e.RequestID != "" {
		s += " (request id " + e.RequestID + ")"
	}
	return s
}

const maxMessageLen = 200

// Message extracts a human-readable message from the body. It understands the
// API's "detail" envelope in its string, object, and array forms, falls back
// to "message" or "error" fields, and otherwise returns the trimmed body,
// truncated to 200 bytes.
func (e *APIError) Message() string {
	var env struct {
		Detail  json.RawMessage `json:"detail"`
		Message string          `json:"message"`
		Error   string          `json:"error"`
	}
	if json.Unmarshal(e.Body, &env) == nil {
		if len(env.Detail) > 0 {
			if msg := detailMessage(env.Detail); msg != "" {
				return msg
			}
		}
		if env.Message != "" {
			return env.Message
		}
		if env.Error != "" {
			return env.Error
		}
	}
	s := strings.TrimSpace(string(e.Body))
	if len(s) > maxMessageLen {
		s = s[:maxMessageLen] + "..."
	}
	return s
}

func detailMessage(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj struct {
		ErrorType string `json:"error_type"`
		Message   string `json:"message"`
	}
	if json.Unmarshal(raw, &obj) == nil && obj.Message != "" {
		if obj.ErrorType != "" {
			return obj.ErrorType + ": " + obj.Message
		}
		return obj.Message
	}
	var items []struct {
		Loc []any  `json:"loc"`
		Msg string `json:"msg"`
	}
	if json.Unmarshal(raw, &items) == nil && len(items) > 0 {
		parts := make([]string, 0, len(items))
		for _, it := range items {
			loc := make([]string, 0, len(it.Loc))
			for _, l := range it.Loc {
				loc = append(loc, fmt.Sprint(l))
			}
			if len(loc) > 0 {
				parts = append(parts, strings.Join(loc, ".")+": "+it.Msg)
			} else {
				parts = append(parts, it.Msg)
			}
		}
		return strings.Join(parts, "; ")
	}
	return ""
}

// RateLimitError is an APIError with status 429. RetryAfter is the wait the
// server requested through Retry-After or retry-after-ms, or zero.
type RateLimitError struct {
	APIError
	RetryAfter time.Duration
}

func (e *RateLimitError) Unwrap() error { return &e.APIError }

// ConnectionError wraps a transport failure: DNS, dial, TLS, a reset
// connection, or a cancelled context.
type ConnectionError struct {
	Err error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("typesafe: connection error: %v", e.Err)
}

func (e *ConnectionError) Unwrap() error { return e.Err }

// TimeoutError is a ConnectionError caused by the per-request timeout.
type TimeoutError struct {
	ConnectionError
	Timeout time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("typesafe: request timed out after %s: %v", e.Timeout, e.Err)
}

func (e *TimeoutError) Unwrap() error { return &e.ConnectionError }

// ResponseValidationError reports a 2xx response whose body did not match
// the expected shape. FieldPath is dotted, for example
// "answers.tone.confidence".
type ResponseValidationError struct {
	FieldPath string
	Err       error
}

func (e *ResponseValidationError) Error() string {
	return fmt.Sprintf("typesafe: invalid response field %q: %v", e.FieldPath, e.Err)
}

func (e *ResponseValidationError) Unwrap() error { return e.Err }

// IsAuthError reports whether err is an APIError with status 401 or 403.
func IsAuthError(err error) bool {
	var api *APIError
	return errors.As(err, &api) && (api.Status == http.StatusUnauthorized || api.Status == http.StatusForbidden)
}

// IsRateLimited reports whether err is a RateLimitError.
func IsRateLimited(err error) bool {
	var rl *RateLimitError
	return errors.As(err, &rl)
}

// IsRetryable reports whether the default RetryPolicy would retry err.
func IsRetryable(err error) bool {
	return DefaultRetryPolicy().retryable(nil, err)
}
```

`DefaultRetryPolicy` and `retryable` arrive in Task 5. To keep the build green now, add a temporary `retry.go`:

```go
package typesafe

import "net/http"

// RetryPolicy is defined fully in Task 5.
type RetryPolicy struct{ MaxRetries int }

func DefaultRetryPolicy() RetryPolicy { return RetryPolicy{MaxRetries: 2} }

func (p RetryPolicy) retryable(_ *http.Response, err error) bool {
	var api *APIError
	if errorsAs(err, &api) {
		return api.Status == 408 || api.Status == 429 || api.Status >= 500
	}
	var ce *ConnectionError
	return errorsAs(err, &ce)
}
```

and in `errors.go` add `func errorsAs(err error, target any) bool { return errors.As(err, target) }`. Task 5 removes both.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -run 'TestAPIError|TestRateLimit|TestIsAuth|TestTimeoutError|TestIsRetryable|TestResponseValidation' -v . 2>&1 | tail -12
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./... && go test ./...
git add errors.go errors_test.go retry.go
git commit -m "Add API, connection, timeout, and validation error types"
```

---

## Task 4: Responses, answer decoding, models list

**Agent:** `core`

**Files:**
- Replace: `response.go`
- Create: `response_test.go`, `models.go`, `testdata/request.json`, `testdata/systemone_ok.json`, `testdata/models_ok.json`

- [ ] **Step 1: Write the fixtures**

Create `testdata/request.json`, `testdata/systemone_ok.json`, and `testdata/models_ok.json` with the exact contents from the Fixtures section at the top of this plan, one line each, no trailing whitespace changes.

- [ ] **Step 2: Write the failing tests `response_test.go`**

```go
package typesafe

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDecodeSystemOneFixture(t *testing.T) {
	res, err := decodeSystemOne(mustRead(t, "testdata/systemone_ok.json"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Model != "jev-1.13.0" {
		t.Fatalf("model %q", res.Model)
	}
	if *res.Usage.InputTokens != 407 || *res.Usage.OutputTokens != 72 {
		t.Fatalf("usage %+v", res.Usage)
	}
	if len(res.Answers) != 3 {
		t.Fatalf("answers %d", len(res.Answers))
	}
	n := res.Nouls()
	if len(n) != 1 || n["billing"].Noul != 0.99 {
		t.Fatalf("nouls %+v", n)
	}
	c := res.Choices()
	if len(c) != 1 || c["tone"].Choice != "angry" || c["tone"].Confidence != 1.0 || c["tone"].Probabilities["angry"] != 1.0 {
		t.Fatalf("choices %+v", c)
	}
	s := res.Scores()
	if len(s) != 1 || s["urgency"].Score != 1.9 || s["urgency"].Confidence != 0.85 || s["urgency"].Legend["2"] != "Very urgent" || s["urgency"].Probabilities["2"] != 0.9 {
		t.Fatalf("scores %+v", s)
	}
	for _, a := range res.Answers {
		if a.AnswerType() == "" {
			t.Fatal("empty answer type")
		}
	}
}

func TestDecodeUnknownAnswerType(t *testing.T) {
	body := []byte(`{"model":"m","answers":{"v":{"type":"vibe","vibe":"good"}},"usage":{}}`)
	res, err := decodeSystemOne(body)
	if err != nil {
		t.Fatal(err)
	}
	u, ok := res.Answers["v"].(UnknownAnswer)
	if !ok || u.Type != "vibe" || string(u.Raw) != `{"type":"vibe","vibe":"good"}` {
		t.Fatalf("got %#v", res.Answers["v"])
	}
	if res.Usage.InputTokens != nil {
		t.Fatal("missing usage should stay nil")
	}
}

func TestDecodeInvalidAnswers(t *testing.T) {
	tests := []struct {
		name string
		body string
		path string
	}{
		{"not json", `nope`, ""},
		{"missing type", `{"answers":{"a":{"noul":0.5}}}`, "answers.a.type"},
		{"noul missing value", `{"answers":{"a":{"type":"noul"}}}`, "answers.a.noul"},
		{"choice wrong type", `{"answers":{"tone":{"type":"choice","choice":"x","confidence":"high","probabilities":{}}}}`, "answers.tone.confidence"},
		{"choice missing choice", `{"answers":{"tone":{"type":"choice","confidence":1,"probabilities":{}}}}`, "answers.tone.choice"},
		{"score missing score", `{"answers":{"u":{"type":"score","confidence":1}}}`, "answers.u.score"},
		{"answer not object", `{"answers":{"a":5}}`, "answers.a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeSystemOne([]byte(tt.body))
			var rve *ResponseValidationError
			if !errors.As(err, &rve) {
				t.Fatalf("want *ResponseValidationError, got %v", err)
			}
			if rve.FieldPath != tt.path {
				t.Fatalf("path got %q want %q", rve.FieldPath, tt.path)
			}
		})
	}
}

func TestAnswerMarshalRoundTrip(t *testing.T) {
	res, err := decodeSystemOne(mustRead(t, "testdata/systemone_ok.json"))
	if err != nil {
		t.Fatal(err)
	}
	res.RequestID = "req_1"
	out, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var back SystemOneResponse
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if back.RequestID != "req_1" || back.Model != "jev-1.13.0" {
		t.Fatalf("round trip lost fields: %+v", back)
	}
	if back.Choices()["tone"].Choice != "angry" || back.Scores()["urgency"].Score != 1.9 || back.Nouls()["billing"].Noul != 0.99 {
		t.Fatalf("round trip lost answers: %+v", back.Answers)
	}
	var probe map[string]any
	if err := json.Unmarshal(out, &probe); err != nil {
		t.Fatal(err)
	}
	if probe["answers"].(map[string]any)["billing"].(map[string]any)["type"] != "noul" {
		t.Fatalf("marshaled answers must carry type: %s", out)
	}
}

func TestAnswerUnmarshalTypeMismatch(t *testing.T) {
	var a NoulAnswer
	err := json.Unmarshal([]byte(`{"type":"choice","choice":"x","confidence":1,"probabilities":{}}`), &a)
	var rve *ResponseValidationError
	if !errors.As(err, &rve) {
		t.Fatalf("want *ResponseValidationError, got %v", err)
	}
}

func TestDecodeModels(t *testing.T) {
	res, err := decodeModels(mustRead(t, "testdata/models_ok.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Models) != 2 || res.Models[0].Name != "jev-latest" || res.Models[1].ReleaseDate != "2026-09-10T18:39:06.057655+00:00" {
		t.Fatalf("models %+v", res.Models)
	}
	if _, err := decodeModels([]byte(`{"models":"x"}`)); err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test -run 'TestDecode|TestAnswer' . 2>&1 | head -5
```
Expected: compile errors for `decodeSystemOne`, `NoulAnswer`, and friends.

- [ ] **Step 4: Replace `response.go`**

```go
package typesafe

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Answer is one decoded answer: NoulAnswer, ChoiceAnswer, ScoreAnswer, or
// UnknownAnswer for types this package does not model.
type Answer interface {
	// AnswerType returns the wire "type" of the answer.
	AnswerType() string
}

// NoulAnswer is the probability, from 0 to 1, that the condition holds.
type NoulAnswer struct {
	Noul float64
}

// ChoiceAnswer is the selected label with the probability of each label and
// the confidence, which summarizes how concentrated the distribution is.
type ChoiceAnswer struct {
	Choice        string
	Confidence    float64
	Probabilities map[string]float64
}

// ScoreAnswer is the probability-weighted score with the rubric legend, the
// probability of each level keyed by its index as a string, and confidence.
type ScoreAnswer struct {
	Score         float64
	Confidence    float64
	Legend        map[string]JSONContent
	Probabilities map[string]float64
}

// UnknownAnswer preserves an answer whose type this package does not know.
type UnknownAnswer struct {
	Type string
	Raw  json.RawMessage
}

func (NoulAnswer) AnswerType() string     { return "noul" }
func (ChoiceAnswer) AnswerType() string   { return "choice" }
func (ScoreAnswer) AnswerType() string    { return "score" }
func (a UnknownAnswer) AnswerType() string { return a.Type }

// Usage is the token usage reported by the API. Fields are nil when absent.
type Usage struct {
	InputTokens  *int `json:"input_tokens"`
	OutputTokens *int `json:"output_tokens"`
}

// RawResponse is the HTTP response behind a decoded result.
type RawResponse struct {
	Status int
	Header http.Header
	Body   []byte
}

// SystemOneResponse is the decoded result of a SystemOne call.
type SystemOneResponse struct {
	Model     string            `json:"model"`
	Answers   map[string]Answer `json:"answers"`
	Usage     Usage             `json:"usage"`
	RequestID string            `json:"request_id,omitempty"`
	Raw       *RawResponse      `json:"-"`
}

// Nouls returns the noul answers keyed by question identifier.
func (r *SystemOneResponse) Nouls() map[string]NoulAnswer {
	out := map[string]NoulAnswer{}
	for k, a := range r.Answers {
		if v, ok := a.(NoulAnswer); ok {
			out[k] = v
		}
	}
	return out
}

// Choices returns the choice answers keyed by question identifier.
func (r *SystemOneResponse) Choices() map[string]ChoiceAnswer {
	out := map[string]ChoiceAnswer{}
	for k, a := range r.Answers {
		if v, ok := a.(ChoiceAnswer); ok {
			out[k] = v
		}
	}
	return out
}

// Scores returns the score answers keyed by question identifier.
func (r *SystemOneResponse) Scores() map[string]ScoreAnswer {
	out := map[string]ScoreAnswer{}
	for k, a := range r.Answers {
		if v, ok := a.(ScoreAnswer); ok {
			out[k] = v
		}
	}
	return out
}

// UnmarshalJSON decodes a response body, or a body previously produced by
// json.Marshal on a SystemOneResponse.
func (r *SystemOneResponse) UnmarshalJSON(b []byte) error {
	res, err := decodeSystemOne(b)
	if err != nil {
		return err
	}
	*r = *res
	return nil
}

var errMissing = errors.New("missing")

type wireResponse struct {
	Model     string                     `json:"model"`
	Answers   map[string]json.RawMessage `json:"answers"`
	Usage     Usage                      `json:"usage"`
	RequestID string                     `json:"request_id"`
}

func decodeSystemOne(body []byte) (*SystemOneResponse, error) {
	var w wireResponse
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fieldErr("", err)
	}
	res := &SystemOneResponse{Model: w.Model, Usage: w.Usage, RequestID: w.RequestID, Answers: make(map[string]Answer, len(w.Answers))}
	for key, raw := range w.Answers {
		a, err := decodeAnswer(joinPath("answers", key), raw)
		if err != nil {
			return nil, err
		}
		res.Answers[key] = a
	}
	return res, nil
}

func joinPath(base, field string) string {
	if base == "" {
		return field
	}
	return base + "." + field
}

// fieldErr wraps a json error as a ResponseValidationError, extending path
// with the field named by an *json.UnmarshalTypeError when available.
func fieldErr(path string, err error) error {
	var ute *json.UnmarshalTypeError
	if errors.As(err, &ute) && ute.Field != "" {
		return &ResponseValidationError{FieldPath: joinPath(path, ute.Field), Err: err}
	}
	return &ResponseValidationError{FieldPath: path, Err: err}
}

func decodeAnswer(path string, raw json.RawMessage) (Answer, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, fieldErr(path, err)
	}
	switch head.Type {
	case "":
		return nil, &ResponseValidationError{FieldPath: joinPath(path, "type"), Err: errMissing}
	case "noul":
		var w struct {
			Noul *float64 `json:"noul"`
		}
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fieldErr(path, err)
		}
		if w.Noul == nil {
			return nil, &ResponseValidationError{FieldPath: joinPath(path, "noul"), Err: errMissing}
		}
		return NoulAnswer{Noul: *w.Noul}, nil
	case "choice":
		var w struct {
			Choice        *string            `json:"choice"`
			Confidence    *float64           `json:"confidence"`
			Probabilities map[string]float64 `json:"probabilities"`
		}
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fieldErr(path, err)
		}
		if w.Choice == nil {
			return nil, &ResponseValidationError{FieldPath: joinPath(path, "choice"), Err: errMissing}
		}
		if w.Confidence == nil {
			return nil, &ResponseValidationError{FieldPath: joinPath(path, "confidence"), Err: errMissing}
		}
		if w.Probabilities == nil {
			w.Probabilities = map[string]float64{}
		}
		return ChoiceAnswer{Choice: *w.Choice, Confidence: *w.Confidence, Probabilities: w.Probabilities}, nil
	case "score":
		var w struct {
			Score         *float64               `json:"score"`
			Confidence    *float64               `json:"confidence"`
			Legend        map[string]JSONContent `json:"legend"`
			Probabilities map[string]float64     `json:"probabilities"`
		}
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fieldErr(path, err)
		}
		if w.Score == nil {
			return nil, &ResponseValidationError{FieldPath: joinPath(path, "score"), Err: errMissing}
		}
		if w.Confidence == nil {
			return nil, &ResponseValidationError{FieldPath: joinPath(path, "confidence"), Err: errMissing}
		}
		if w.Legend == nil {
			w.Legend = map[string]JSONContent{}
		}
		if w.Probabilities == nil {
			w.Probabilities = map[string]float64{}
		}
		return ScoreAnswer{Score: *w.Score, Confidence: *w.Confidence, Legend: w.Legend, Probabilities: w.Probabilities}, nil
	default:
		return UnknownAnswer{Type: head.Type, Raw: append(json.RawMessage(nil), raw...)}, nil
	}
}

// MarshalJSON encodes the answer with its wire "type".
func (a NoulAnswer) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"type": "noul", "noul": a.Noul})
}

// MarshalJSON encodes the answer with its wire "type".
func (a ChoiceAnswer) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"type": "choice", "choice": a.Choice, "confidence": a.Confidence, "probabilities": a.Probabilities})
}

// MarshalJSON encodes the answer with its wire "type".
func (a ScoreAnswer) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"type": "score", "score": a.Score, "confidence": a.Confidence, "legend": a.Legend, "probabilities": a.Probabilities})
}

// MarshalJSON returns the raw answer unchanged.
func (a UnknownAnswer) MarshalJSON() ([]byte, error) {
	if len(a.Raw) == 0 {
		return json.Marshal(map[string]any{"type": a.Type})
	}
	return a.Raw, nil
}

func unmarshalAs[T Answer](b []byte, want string) (T, error) {
	var zero T
	a, err := decodeAnswer("", b)
	if err != nil {
		return zero, err
	}
	v, ok := a.(T)
	if !ok {
		return zero, &ResponseValidationError{FieldPath: "type", Err: errors.New("expected " + want + " answer, got " + a.AnswerType())}
	}
	return v, nil
}

// UnmarshalJSON decodes a noul answer and verifies its type.
func (a *NoulAnswer) UnmarshalJSON(b []byte) error {
	v, err := unmarshalAs[NoulAnswer](b, "noul")
	if err != nil {
		return err
	}
	*a = v
	return nil
}

// UnmarshalJSON decodes a choice answer and verifies its type.
func (a *ChoiceAnswer) UnmarshalJSON(b []byte) error {
	v, err := unmarshalAs[ChoiceAnswer](b, "choice")
	if err != nil {
		return err
	}
	*a = v
	return nil
}

// UnmarshalJSON decodes a score answer and verifies its type.
func (a *ScoreAnswer) UnmarshalJSON(b []byte) error {
	v, err := unmarshalAs[ScoreAnswer](b, "score")
	if err != nil {
		return err
	}
	*a = v
	return nil
}
```

- [ ] **Step 5: Write `models.go`**

```go
package typesafe

import "encoding/json"

// ModelMetadata describes one model available to the account.
type ModelMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// ReleaseDate is the timestamp string reported by the API.
	ReleaseDate string `json:"release_date"`
}

// ListModelsResponse is the decoded result of a ListModels call.
type ListModelsResponse struct {
	Models    []ModelMetadata `json:"models"`
	RequestID string          `json:"request_id,omitempty"`
	Raw       *RawResponse    `json:"-"`
}

func decodeModels(body []byte) (*ListModelsResponse, error) {
	var res ListModelsResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fieldErr("", err)
	}
	return &res, nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test -run 'TestDecode|TestAnswer' -v . 2>&1 | tail -15
```
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
gofmt -l . ; go vet ./... && go test ./...
git add response.go response_test.go models.go testdata/request.json testdata/systemone_ok.json testdata/models_ok.json
git commit -m "Add response decoding, answer types, and models list"
```

---
## Task 5: Retry policy

**Agent:** `core`

**Files:**
- Replace: `retry.go`
- Create: `retry_test.go`
- Modify: `errors.go` (delete the temporary `errorsAs` helper)

- [ ] **Step 1: Write the failing tests `retry_test.go`**

```go
package typesafe

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestDefaultRetryPolicy(t *testing.T) {
	p := DefaultRetryPolicy()
	if p.MaxRetries != 2 || p.InitialDelay != 500*time.Millisecond || p.MaxDelay != 5*time.Second || p.Jitter != 0.25 || p.Budget != 30*time.Second {
		t.Fatalf("unexpected defaults %+v", p)
	}
	if !p.RetryOnConnErr || !p.RetryOnTimeout || !p.HonorRetryAfter {
		t.Fatal("boolean defaults should be true")
	}
	for _, s := range []int{408, 429, 500, 529, 599} {
		if !p.retryableStatus(s) {
			t.Errorf("%d should be retryable", s)
		}
	}
	for _, s := range []int{200, 400, 401, 404, 422, 499} {
		if p.retryableStatus(s) {
			t.Errorf("%d should not be retryable", s)
		}
	}
}

func TestRetryPolicyNormalized(t *testing.T) {
	p := RetryPolicy{MaxRetries: 3}.normalized()
	if p.InitialDelay != 500*time.Millisecond || p.MaxDelay != 5*time.Second || len(p.RetryStatuses) == 0 {
		t.Fatalf("normalized should fill zero values: %+v", p)
	}
	if p.MaxRetries != 3 {
		t.Fatal("normalized must keep explicit values")
	}
	q := RetryPolicy{MaxRetries: 1, InitialDelay: time.Second, MaxDelay: time.Millisecond}.normalized()
	if q.MaxDelay != time.Second {
		t.Fatalf("MaxDelay below InitialDelay should be raised, got %s", q.MaxDelay)
	}
}

func TestRetryDelayBackoff(t *testing.T) {
	p := DefaultRetryPolicy()
	p.Jitter = 0
	want := []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	for i, w := range want {
		if got := p.delay(i, nil, time.Now(), 0); got != w {
			t.Errorf("retry %d: got %s want %s", i, got, w)
		}
	}
	if got := p.delay(60, nil, time.Now(), 0); got != 5*time.Second {
		t.Errorf("large retry count must cap, got %s", got)
	}
}

func TestRetryDelayJitter(t *testing.T) {
	p := DefaultRetryPolicy()
	if got := p.delay(0, nil, time.Now(), 1); got != 375*time.Millisecond {
		t.Fatalf("full jitter should subtract 25%%, got %s", got)
	}
	if got := p.delay(0, nil, time.Now(), 0); got != 500*time.Millisecond {
		t.Fatalf("zero jitter draw should keep base, got %s", got)
	}
}

func TestRetryDelayHonorsRetryAfter(t *testing.T) {
	p := DefaultRetryPolicy()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	h := http.Header{}
	h.Set("retry-after-ms", "1500")
	if got := p.delay(0, h, now, 0); got != 1500*time.Millisecond {
		t.Fatalf("retry-after-ms: got %s", got)
	}
	h = http.Header{}
	h.Set("Retry-After", "3")
	if got := p.delay(0, h, now, 0); got != 3*time.Second {
		t.Fatalf("Retry-After seconds: got %s", got)
	}
	h = http.Header{}
	h.Set("Retry-After", now.Add(10*time.Second).Format(http.TimeFormat))
	if got := p.delay(0, h, now, 0); got != 10*time.Second {
		t.Fatalf("Retry-After date: got %s", got)
	}
	h = http.Header{}
	h.Set("Retry-After", now.Add(-10*time.Second).Format(http.TimeFormat))
	if got := p.delay(0, h, now, 0); got != 0 {
		t.Fatalf("past date should be zero, got %s", got)
	}
	h = http.Header{}
	h.Set("Retry-After", "garbage")
	if got := p.delay(0, h, now, 0); got != 500*time.Millisecond {
		t.Fatalf("unparseable header falls back to backoff, got %s", got)
	}
	p.HonorRetryAfter = false
	h = http.Header{}
	h.Set("Retry-After", "3")
	if got := p.delay(0, h, now, 0); got != 500*time.Millisecond {
		t.Fatalf("disabled honor should ignore header, got %s", got)
	}
}

func TestRetryableDecisions(t *testing.T) {
	p := DefaultRetryPolicy()
	if !p.retryable(nil, &APIError{Status: 503}) || p.retryable(nil, &APIError{Status: 422}) {
		t.Fatal("status decisions wrong")
	}
	if !p.retryable(nil, &ConnectionError{Err: errors.New("x")}) {
		t.Fatal("connection errors retry by default")
	}
	p.RetryOnConnErr = false
	if p.retryable(nil, &ConnectionError{Err: errors.New("x")}) {
		t.Fatal("RetryOnConnErr=false must be respected")
	}
	if !p.retryable(nil, &TimeoutError{ConnectionError: ConnectionError{Err: errors.New("x")}}) {
		t.Fatal("timeouts still retry when RetryOnTimeout is true")
	}
	p.RetryOnTimeout = false
	if p.retryable(nil, &TimeoutError{ConnectionError: ConnectionError{Err: errors.New("x")}}) {
		t.Fatal("RetryOnTimeout=false must be respected")
	}
	calls := 0
	p.ShouldRetry = func(resp *http.Response, err error) bool { calls++; return true }
	if !p.retryable(nil, &APIError{Status: 422}) || calls != 1 {
		t.Fatal("ShouldRetry must override everything")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -run 'TestDefaultRetryPolicy|TestRetry' . 2>&1 | head -5
```
Expected: compile errors for `InitialDelay`, `delay`, `normalized`.

- [ ] **Step 3: Replace `retry.go`**

```go
package typesafe

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryPolicy controls how failed requests are retried. Start from
// DefaultRetryPolicy and change fields; a zero RetryPolicy disables retries.
type RetryPolicy struct {
	// MaxRetries is the number of retries after the first attempt. 0 disables retries.
	MaxRetries int
	// InitialDelay is the delay before the first retry. Default 500ms.
	InitialDelay time.Duration
	// MaxDelay caps the exponential backoff. Default 5s.
	MaxDelay time.Duration
	// Jitter is the fraction of each delay randomly subtracted, from 0 to 1. Default 0.25.
	Jitter float64
	// Budget is the total time allowed per call including delays. 0 means unlimited. Default 30s.
	Budget time.Duration
	// RetryStatuses lists HTTP statuses that are retried. Default 408, 429, 500–599.
	RetryStatuses []int
	// RetryOnConnErr retries connection failures. Default true.
	RetryOnConnErr bool
	// RetryOnTimeout retries per-request timeouts. Default true.
	RetryOnTimeout bool
	// HonorRetryAfter uses Retry-After and retry-after-ms headers as the delay. Default true.
	HonorRetryAfter bool
	// ShouldRetry, when set, replaces every other decision. resp may be nil.
	ShouldRetry func(resp *http.Response, err error) bool
}

// DefaultRetryPolicy returns the policy used when none is configured. It
// matches the official Python SDK's defaults.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:      2,
		InitialDelay:    500 * time.Millisecond,
		MaxDelay:        5 * time.Second,
		Jitter:          0.25,
		Budget:          30 * time.Second,
		RetryStatuses:   defaultRetryStatuses(),
		RetryOnConnErr:  true,
		RetryOnTimeout:  true,
		HonorRetryAfter: true,
	}
}

func defaultRetryStatuses() []int {
	s := []int{http.StatusRequestTimeout, http.StatusTooManyRequests}
	for code := 500; code <= 599; code++ {
		s = append(s, code)
	}
	return s
}

// normalized fills zero delay and status fields with defaults so a policy
// built by hand still behaves sensibly.
func (p RetryPolicy) normalized() RetryPolicy {
	d := DefaultRetryPolicy()
	if p.InitialDelay <= 0 {
		p.InitialDelay = d.InitialDelay
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = d.MaxDelay
	}
	if p.MaxDelay < p.InitialDelay {
		p.MaxDelay = p.InitialDelay
	}
	if p.Jitter < 0 {
		p.Jitter = 0
	}
	if p.Jitter > 1 {
		p.Jitter = 1
	}
	if p.RetryStatuses == nil {
		p.RetryStatuses = d.RetryStatuses
	}
	return p
}

func (p RetryPolicy) retryableStatus(code int) bool {
	for _, s := range p.RetryStatuses {
		if s == code {
			return true
		}
	}
	return false
}

// retryable decides whether a failed attempt should be retried.
func (p RetryPolicy) retryable(resp *http.Response, err error) bool {
	if p.ShouldRetry != nil {
		return p.ShouldRetry(resp, err)
	}
	var api *APIError
	if errors.As(err, &api) {
		return p.retryableStatus(api.Status)
	}
	var te *TimeoutError
	if errors.As(err, &te) {
		return p.RetryOnTimeout
	}
	var ce *ConnectionError
	if errors.As(err, &ce) {
		return p.RetryOnConnErr
	}
	return false
}

// delay computes the wait before retry number retry (0-based). hdr may be
// nil. random is a draw in [0, 1) used for jitter.
func (p RetryPolicy) delay(retry int, hdr http.Header, now time.Time, random float64) time.Duration {
	if p.HonorRetryAfter && hdr != nil {
		if d, ok := parseRetryAfter(hdr, now); ok {
			return d
		}
	}
	if retry > 30 {
		retry = 30
	}
	d := p.InitialDelay << uint(retry)
	if d <= 0 || d > p.MaxDelay {
		d = p.MaxDelay
	}
	if p.Jitter > 0 {
		d -= time.Duration(float64(d) * p.Jitter * random)
	}
	return d
}

// parseRetryAfter reads retry-after-ms (milliseconds) or Retry-After
// (seconds or an HTTP date). It returns false when neither is usable.
func parseRetryAfter(h http.Header, now time.Time) (time.Duration, bool) {
	if v := strings.TrimSpace(h.Get("retry-after-ms")); v != "" {
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil && ms >= 0 {
			return time.Duration(ms) * time.Millisecond, true
		}
	}
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.ParseFloat(v, 64); err == nil {
		if secs < 0 {
			secs = 0
		}
		return time.Duration(secs * float64(time.Second)), true
	}
	if t, err := http.ParseTime(v); err == nil {
		d := t.Sub(now)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}
```

- [ ] **Step 4: Remove the temporary helper from `errors.go`**

Delete the `errorsAs` function added in Task 3. Nothing else in `errors.go` changes.

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test -run 'TestDefaultRetryPolicy|TestRetry|TestIsRetryable' -v . 2>&1 | tail -12
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; go vet ./... && go test ./...
git add retry.go retry_test.go errors.go
git commit -m "Add retry policy with exponential backoff and Retry-After support"
```

---

## Task 6: Transport: attempt loop, headers, error mapping

**Agent:** `core`

**Files:**
- Create: `transport.go`

This task has no tests of its own; Task 7's fake-server tests cover it. It is split out so the file stays focused.

- [ ] **Step 1: Write `transport.go`**

```go
package typesafe

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/therealbill/typesafe-go/internal/version"
)

const (
	headerRequestID  = "X-Typesafe-Request-Id"
	maxResponseBytes = 16 << 20
)

var userAgent = "typesafe-go/" + version.Version

// send performs one logical request with retries. It returns the final
// response, its body, the number of attempts made, and an error when the
// final attempt failed. resp is nil when no response was received.
func (c *Client) send(ctx context.Context, method, path string, body []byte, extra http.Header, policy RetryPolicy, timeout time.Duration) (*http.Response, []byte, int, error) {
	policy = policy.normalized()
	endpoint := method + " " + path
	start := time.Now()
	for retry := 0; ; retry++ {
		resp, respBody, err := c.attempt(ctx, method, path, body, extra, timeout)
		if err == nil && resp.StatusCode < 400 {
			c.logger.Debug("typesafe request ok", "endpoint", endpoint, "status", resp.StatusCode, "attempt", retry+1, "request_id", resp.Header.Get(headerRequestID), "elapsed", time.Since(start))
			return resp, respBody, retry + 1, nil
		}
		if err == nil {
			err = newAPIError(resp, respBody, endpoint)
		}
		if ctx.Err() != nil || retry >= policy.MaxRetries || !policy.retryable(resp, err) {
			c.logger.Error("typesafe request failed", "endpoint", endpoint, "attempt", retry+1, "error", err)
			return resp, respBody, retry + 1, err
		}
		var hdr http.Header
		if resp != nil {
			hdr = resp.Header
		}
		delay := policy.delay(retry, hdr, time.Now(), c.random())
		if policy.Budget > 0 && time.Since(start)+delay > policy.Budget {
			c.logger.Warn("typesafe retry budget exhausted", "endpoint", endpoint, "attempt", retry+1, "error", err)
			return resp, respBody, retry + 1, err
		}
		c.logger.Warn("typesafe retrying", "endpoint", endpoint, "attempt", retry+1, "delay", delay, "error", err)
		if serr := sleep(ctx, delay); serr != nil {
			return nil, nil, retry + 1, &ConnectionError{Err: serr}
		}
	}
}

// attempt performs a single HTTP request and reads the body. A non-2xx
// status is not an error here; send maps it.
func (c *Client) attempt(ctx context.Context, method, path string, body []byte, extra http.Header, timeout time.Duration) (*http.Response, []byte, error) {
	actx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		actx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(actx, method, c.baseURL+path, rdr)
	if err != nil {
		return nil, nil, &ConnectionError{Err: err}
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	applyHeaders(req.Header, c.headers)
	applyHeaders(req.Header, extra)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, wrapTransportError(err, ctx, actx, timeout)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, nil, wrapTransportError(err, ctx, actx, timeout)
	}
	return resp, data, nil
}

// applyHeaders replaces each key in dst with the values from src.
func applyHeaders(dst, src http.Header) {
	for k, vs := range src {
		dst.Del(k)
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

func wrapTransportError(err error, parent, attemptCtx context.Context, timeout time.Duration) error {
	if parent.Err() != nil {
		return &ConnectionError{Err: parent.Err()}
	}
	if errors.Is(attemptCtx.Err(), context.DeadlineExceeded) || isTimeout(err) {
		return &TimeoutError{ConnectionError: ConnectionError{Err: err}, Timeout: timeout}
	}
	return &ConnectionError{Err: err}
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func newAPIError(resp *http.Response, body []byte, endpoint string) error {
	base := APIError{
		Status:    resp.StatusCode,
		Body:      body,
		Headers:   resp.Header.Clone(),
		Endpoint:  endpoint,
		RequestID: resp.Header.Get(headerRequestID),
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		d, _ := parseRetryAfter(resp.Header, time.Now())
		return &RateLimitError{APIError: base, RetryAfter: d}
	}
	return &base
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
```

- [ ] **Step 2: Build**

```bash
go build ./... 2>&1 | head
```
Expected: errors only about `c.logger`, `c.random`, `c.httpClient`, `c.baseURL`, `c.apiKey`, `c.headers` (the `Client` struct comes in Task 7). Do not commit yet; Task 7 commits `transport.go` together with `client.go`.

---
## Task 7: Client, options, SystemOne, ListModels

**Agent:** `core`

**Files:**
- Create: `client.go`, `client_test.go`
- Commit also: `transport.go` from Task 6

- [ ] **Step 1: Write the failing tests `client_test.go`**

```go
package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type step struct {
	status  int
	body    string
	headers map[string]string
	delay   time.Duration
}

type fakeServer struct {
	*httptest.Server
	mu       sync.Mutex
	steps    []step
	calls    int
	requests []*http.Request
	bodies   [][]byte
}

func newFakeServer(t *testing.T, steps ...step) *fakeServer {
	t.Helper()
	fs := &fakeServer{steps: steps}
	fs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fs.mu.Lock()
		i := fs.calls
		fs.calls++
		fs.requests = append(fs.requests, r.Clone(context.Background()))
		fs.bodies = append(fs.bodies, body)
		fs.mu.Unlock()
		if i >= len(fs.steps) {
			i = len(fs.steps) - 1
		}
		s := fs.steps[i]
		if s.delay > 0 {
			time.Sleep(s.delay)
		}
		for k, v := range s.headers {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(s.status)
		_, _ = w.Write([]byte(s.body))
	}))
	t.Cleanup(fs.Close)
	return fs
}

func (fs *fakeServer) callCount() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.calls
}

func fastPolicy() RetryPolicy {
	p := DefaultRetryPolicy()
	p.InitialDelay = time.Millisecond
	p.MaxDelay = 2 * time.Millisecond
	p.Jitter = 0
	p.Budget = 2 * time.Second
	return p
}

func newTestClient(t *testing.T, url string, opts ...Option) *Client {
	t.Helper()
	base := []Option{WithAPIKey("test-key"), WithBaseURL(url), WithRetryPolicy(fastPolicy())}
	c, err := NewClient(append(base, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var fixtureQuestions = Questions{
	"billing": Noul{Instructions: "Is `ticket` about a billing problem?"},
	"tone":    Choice{Instructions: "What is the tone of `ticket`?", Criteria: map[string]JSONContent{"calm": "Polite and patient", "angry": "Frustrated or hostile", "neutral": nil}},
	"urgency": Score{Instructions: "How urgent is `ticket`?", Criteria: []JSONContent{"Not urgent at all", "Somewhat urgent", "Very urgent"}},
}

var fixtureState = map[string]any{"ticket": "I was charged twice this month and nobody answers my emails. Fix it now."}

func okStep(t *testing.T) step {
	return step{status: 200, body: string(mustRead(t, "testdata/systemone_ok.json")), headers: map[string]string{"x-typesafe-request-id": "req_01a0d07208027e0bbeb09c9f9b2b205a"}}
}

func TestSystemOneSuccess(t *testing.T) {
	fs := newFakeServer(t, okStep(t))
	c := newTestClient(t, fs.URL)
	res, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	if err != nil {
		t.Fatal(err)
	}
	req := fs.requests[0]
	if req.Method != http.MethodPost || req.URL.Path != "/v1/systemone" {
		t.Fatalf("bad request line %s %s", req.Method, req.URL.Path)
	}
	if req.Header.Get("Authorization") != "Bearer test-key" || req.Header.Get("Content-Type") != "application/json" || !strings.HasPrefix(req.Header.Get("User-Agent"), "typesafe-go/") {
		t.Fatalf("headers %v", req.Header)
	}
	var got, want map[string]any
	if err := json.Unmarshal(fs.bodies[0], &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mustRead(t, "testdata/request.json"), &want); err != nil {
		t.Fatal(err)
	}
	gotB, _ := json.Marshal(got)
	wantB, _ := json.Marshal(want)
	if !bytes.Equal(gotB, wantB) {
		t.Fatalf("body\n got %s\nwant %s", gotB, wantB)
	}
	if res.Model != "jev-1.13.0" || res.RequestID != "req_01a0d07208027e0bbeb09c9f9b2b205a" || res.Raw.Status != 200 {
		t.Fatalf("response meta %+v", res)
	}
	if res.Nouls()["billing"].Noul != 0.99 || res.Choices()["tone"].Choice != "angry" || res.Scores()["urgency"].Score != 1.9 {
		t.Fatalf("answers %+v", res.Answers)
	}
	if *res.Usage.InputTokens != 407 {
		t.Fatalf("usage %+v", res.Usage)
	}
}

func TestSystemOneRetriesOn429ThenSucceeds(t *testing.T) {
	fs := newFakeServer(t,
		step{status: 429, body: `{"detail":"slow down"}`, headers: map[string]string{"retry-after-ms": "5"}},
		okStep(t))
	c := newTestClient(t, fs.URL)
	start := time.Now()
	res, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	if err != nil {
		t.Fatal(err)
	}
	if fs.callCount() != 2 {
		t.Fatalf("calls %d", fs.callCount())
	}
	if time.Since(start) < 5*time.Millisecond {
		t.Fatal("retry-after-ms was not honored")
	}
	if !bytes.Equal(fs.bodies[0], fs.bodies[1]) {
		t.Fatal("request body must be replayed identically")
	}
	if res.Model != "jev-1.13.0" {
		t.Fatal("wrong response")
	}
}

func TestSystemOneRetriesOn529(t *testing.T) {
	fs := newFakeServer(t, step{status: 529, body: `overloaded`}, okStep(t))
	c := newTestClient(t, fs.URL)
	if _, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions); err != nil {
		t.Fatal(err)
	}
	if fs.callCount() != 2 {
		t.Fatalf("calls %d", fs.callCount())
	}
}

func TestSystemOneRetriesExhausted(t *testing.T) {
	fs := newFakeServer(t, step{status: 503, body: `{"detail":"down"}`, headers: map[string]string{"x-typesafe-request-id": "req_x"}})
	c := newTestClient(t, fs.URL)
	_, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	var api *APIError
	if !errors.As(err, &api) || api.Status != 503 || api.RequestID != "req_x" || api.Endpoint != "POST /v1/systemone" {
		t.Fatalf("err %v", err)
	}
	if fs.callCount() != 3 {
		t.Fatalf("calls %d want 3", fs.callCount())
	}
	if !IsRetryable(err) {
		t.Fatal("503 is retryable")
	}
}

func TestSystemOneNoRetryOn422(t *testing.T) {
	fs := newFakeServer(t, step{status: 422, body: `{"detail":[{"type":"too_short","loc":["body","questions"],"msg":"Dictionary should have at least 1 item after validation, not 0","input":{}}]}`})
	c := newTestClient(t, fs.URL)
	_, err := c.SystemOne(context.Background(), fixtureState, Questions{"q": RawQuestion{"type": "noul"}})
	var api *APIError
	if !errors.As(err, &api) || api.Status != 422 {
		t.Fatalf("err %v", err)
	}
	if !strings.Contains(api.Message(), "body.questions") {
		t.Fatalf("message %q", api.Message())
	}
	if fs.callCount() != 1 {
		t.Fatalf("calls %d", fs.callCount())
	}
}

func TestSystemOneRateLimitError(t *testing.T) {
	fs := newFakeServer(t, step{status: 429, body: `{"detail":"slow"}`, headers: map[string]string{"Retry-After": "0"}})
	c := newTestClient(t, fs.URL)
	_, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	var rl *RateLimitError
	if !errors.As(err, &rl) || rl.Status != 429 || rl.RetryAfter != 0 {
		t.Fatalf("err %v", err)
	}
	if !IsRateLimited(err) {
		t.Fatal("IsRateLimited")
	}
	if fs.callCount() != 3 {
		t.Fatalf("calls %d", fs.callCount())
	}
}

func TestSystemOneBudgetExhausted(t *testing.T) {
	fs := newFakeServer(t, step{status: 503, body: `down`})
	p := fastPolicy()
	p.InitialDelay = 50 * time.Millisecond
	p.MaxDelay = 50 * time.Millisecond
	p.Budget = 10 * time.Millisecond
	c := newTestClient(t, fs.URL, WithRetryPolicy(p))
	_, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	var api *APIError
	if !errors.As(err, &api) || api.Status != 503 {
		t.Fatalf("err %v", err)
	}
	if fs.callCount() != 1 {
		t.Fatalf("budget should stop before the first retry, calls %d", fs.callCount())
	}
}

func TestSystemOneContextCancelledDuringBackoff(t *testing.T) {
	fs := newFakeServer(t, step{status: 503, body: `down`})
	p := fastPolicy()
	p.InitialDelay = time.Second
	p.MaxDelay = time.Second
	c := newTestClient(t, fs.URL, WithRetryPolicy(p))
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err := c.SystemOne(ctx, fixtureState, fixtureQuestions)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v", err)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("cancellation did not interrupt the backoff sleep")
	}
	if fs.callCount() != 1 {
		t.Fatalf("calls %d", fs.callCount())
	}
}

func TestSystemOneTimeout(t *testing.T) {
	fs := newFakeServer(t, step{status: 200, body: `{}`, delay: 200 * time.Millisecond})
	p := fastPolicy()
	p.MaxRetries = 0
	c := newTestClient(t, fs.URL, WithRetryPolicy(p), WithTimeout(20*time.Millisecond))
	_, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	var te *TimeoutError
	if !errors.As(err, &te) || te.Timeout != 20*time.Millisecond {
		t.Fatalf("err %v", err)
	}
	if !IsRetryable(err) {
		t.Fatal("timeouts are retryable by default")
	}
}

func TestSystemOneConnectionError(t *testing.T) {
	fs := newFakeServer(t, okStep(t))
	url := fs.URL
	fs.Close()
	p := fastPolicy()
	p.MaxRetries = 1
	c := newTestClient(t, url, WithRetryPolicy(p))
	_, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	var ce *ConnectionError
	if !errors.As(err, &ce) {
		t.Fatalf("err %v", err)
	}
	var te *TimeoutError
	if errors.As(err, &te) {
		t.Fatal("a refused connection is not a timeout")
	}
}

func TestSystemOneValidatesBeforeNetwork(t *testing.T) {
	fs := newFakeServer(t, okStep(t))
	c := newTestClient(t, fs.URL)
	_, err := c.SystemOne(context.Background(), fixtureState, Questions{})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err %v", err)
	}
	_, err = c.SystemOne(context.Background(), nil, fixtureQuestions)
	if !errors.As(err, &ve) || ve.Path != "state" {
		t.Fatalf("err %v", err)
	}
	if fs.callCount() != 0 {
		t.Fatal("no request should be sent")
	}
}

func TestSystemOneRequestOptions(t *testing.T) {
	fs := newFakeServer(t, okStep(t))
	c := newTestClient(t, fs.URL, WithHeaders(http.Header{"X-Client": {"a"}}))
	_, err := c.SystemOne(context.Background(), "state", fixtureQuestions,
		WithRequestModel("jev-preview"),
		WithExtraHeaders(http.Header{"X-Req": {"b"}, "X-Client": {"override"}}),
		WithExtraBody(map[string]any{"weight": 2}))
	if err != nil {
		t.Fatal(err)
	}
	req := fs.requests[0]
	if req.Header.Get("X-Req") != "b" || req.Header.Get("X-Client") != "override" {
		t.Fatalf("headers %v", req.Header)
	}
	var body map[string]any
	_ = json.Unmarshal(fs.bodies[0], &body)
	if body["model"] != "jev-preview" || body["weight"] != float64(2) || body["state"] != "state" {
		t.Fatalf("body %v", body)
	}
}

func TestSystemOneUnknownAnswerType(t *testing.T) {
	fs := newFakeServer(t, step{status: 200, body: `{"model":"m","answers":{"v":{"type":"vibe","x":1}},"usage":{"input_tokens":1,"output_tokens":1}}`})
	c := newTestClient(t, fs.URL)
	res, err := c.SystemOne(context.Background(), "s", fixtureQuestions)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := res.Answers["v"].(UnknownAnswer); !ok {
		t.Fatalf("got %#v", res.Answers["v"])
	}
}

func TestSystemOneInvalidResponse(t *testing.T) {
	fs := newFakeServer(t, step{status: 200, body: `{"model":"m","answers":{"tone":{"type":"choice","choice":"x","confidence":"high","probabilities":{}}}}`})
	c := newTestClient(t, fs.URL)
	_, err := c.SystemOne(context.Background(), "s", fixtureQuestions)
	var rve *ResponseValidationError
	if !errors.As(err, &rve) || rve.FieldPath != "answers.tone.confidence" {
		t.Fatalf("err %v", err)
	}
}

func TestListModels(t *testing.T) {
	fs := newFakeServer(t, step{status: 200, body: string(mustRead(t, "testdata/models_ok.json")), headers: map[string]string{"x-typesafe-request-id": "req_m"}})
	c := newTestClient(t, fs.URL)
	res, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	req := fs.requests[0]
	if req.Method != http.MethodGet || req.URL.Path != "/v1/models" || req.Header.Get("Content-Type") != "" {
		t.Fatalf("request %s %s %v", req.Method, req.URL.Path, req.Header)
	}
	if len(res.Models) != 2 || res.RequestID != "req_m" || res.Raw.Status != 200 {
		t.Fatalf("res %+v", res)
	}
}

func TestNewClientMissingKey(t *testing.T) {
	t.Setenv(EnvAPIKey, "")
	_, err := NewClient()
	if !errors.Is(err, ErrMissingAPIKey) {
		t.Fatalf("err %v", err)
	}
}

func TestNewClientEnvAndDefaults(t *testing.T) {
	t.Setenv(EnvAPIKey, "env-key")
	t.Setenv(EnvBaseURL, "https://example.test/")
	t.Setenv(EnvDefaultModel, "jev-preview")
	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	if c.apiKey != "env-key" || c.baseURL != "https://example.test" || c.model != "jev-preview" || c.timeout != DefaultTimeout {
		t.Fatalf("client %+v", c)
	}
	t.Setenv(EnvBaseURL, "")
	t.Setenv(EnvDefaultModel, "")
	c, err = NewClient(WithAPIKey("k"))
	if err != nil {
		t.Fatal(err)
	}
	if c.baseURL != DefaultBaseURL || c.model != DefaultModel {
		t.Fatalf("defaults %+v", c)
	}
	if _, err := NewClient(WithAPIKey("k"), WithBaseURL("://bad")); err == nil {
		t.Fatal("bad base URL should fail")
	}
}

type fakeInstrumentation struct {
	transportCalls int
	infos          []RequestInfo
	results        []RequestResult
}

func (f *fakeInstrumentation) RequestStart(ctx context.Context, info RequestInfo) (context.Context, func(RequestResult)) {
	f.infos = append(f.infos, info)
	return context.WithValue(ctx, ctxKey{}, "set"), func(r RequestResult) { f.results = append(f.results, r) }
}

func (f *fakeInstrumentation) Transport(rt http.RoundTripper) http.RoundTripper {
	f.transportCalls++
	return rt
}

type ctxKey struct{}

func TestInstrumentationHook(t *testing.T) {
	fs := newFakeServer(t, step{status: 503, body: "x"}, okStep(t))
	fi := &fakeInstrumentation{}
	c := newTestClient(t, fs.URL, WithInstrumentation(fi))
	if fi.transportCalls != 1 {
		t.Fatalf("Transport called %d times", fi.transportCalls)
	}
	res, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	if err != nil {
		t.Fatal(err)
	}
	info := fi.infos[0]
	if info.Operation != "system_one" || info.Model != DefaultModel || info.QuestionCount != 3 || info.NoulCount != 1 || info.ChoiceCount != 1 || info.ScoreCount != 1 {
		t.Fatalf("info %+v", info)
	}
	r := fi.results[0]
	if r.Attempts != 2 || r.Status != 200 || r.RequestID != res.RequestID || r.Model != "jev-1.13.0" || r.Response != res || r.Err != nil {
		t.Fatalf("result %+v", r)
	}
	fs2 := newFakeServer(t, step{status: 401, body: `{"detail":"no"}`})
	c2 := newTestClient(t, fs2.URL, WithInstrumentation(fi))
	_, err = c2.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	r = fi.results[1]
	if fi.infos[1].Operation != "list_models" || r.Status != 401 || r.Err == nil || r.Attempts != 1 {
		t.Fatalf("result %+v", r)
	}
}

func TestWithHTTPClientDoesNotMutateCaller(t *testing.T) {
	fs := newFakeServer(t, okStep(t))
	hc := &http.Client{}
	fi := &fakeInstrumentation{}
	c := newTestClient(t, fs.URL, WithHTTPClient(hc), WithInstrumentation(fi))
	if hc.Transport != nil {
		t.Fatal("caller's client was mutated")
	}
	if c.httpClient == hc {
		t.Fatal("client should hold a copy")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLoggingRedactsAuthorization(t *testing.T) {
	fs := newFakeServer(t, step{status: 503, body: "x"}, okStep(t))
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	c := newTestClient(t, fs.URL, WithLogger(logger))
	if _, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "retrying") || !strings.Contains(out, "request ok") {
		t.Fatalf("expected retry and success logs, got %s", out)
	}
	if strings.Contains(out, "test-key") || strings.Contains(out, "Bearer") || strings.Contains(out, "ticket") {
		t.Fatalf("log leaked secrets or body: %s", out)
	}
}

func TestNewLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf, "warning")
	l.Info("hidden")
	l.Warn("shown")
	if strings.Contains(buf.String(), "hidden") || !strings.Contains(buf.String(), "shown") {
		t.Fatalf("level filtering wrong: %s", buf.String())
	}
	buf.Reset()
	NewLogger(&buf, "off").Error("nothing")
	if buf.Len() != 0 {
		t.Fatal("off must discard")
	}
}

func TestClientConcurrentUse(t *testing.T) {
	fs := newFakeServer(t, okStep(t))
	c := newTestClient(t, fs.URL)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -run 'TestSystemOne|TestListModels|TestNewClient|TestInstrumentation|TestWithHTTPClient|TestLogging|TestNewLogger|TestClientConcurrent' . 2>&1 | head -5
```
Expected: compile errors for `NewClient`, `WithAPIKey`, and friends.

- [ ] **Step 3: Write `client.go`**

```go
package typesafe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Defaults and environment variable names. Explicit options win over the
// environment, which wins over these defaults.
const (
	DefaultBaseURL = "https://api.typesafe.ai"
	DefaultModel   = "jev-latest"
	DefaultTimeout = 10 * time.Second

	EnvAPIKey       = "TYPESAFE_API_KEY"
	EnvBaseURL      = "TYPESAFE_BASE_URL"
	EnvDefaultModel = "TYPESAFE_DEFAULT_MODEL"
	EnvLogLevel     = "TYPESAFE_LOG_LEVEL"
)

// Client calls the TypeSafe API. It is safe for concurrent use.
type Client struct {
	apiKey     string
	baseURL    string
	model      string
	retry      RetryPolicy
	timeout    time.Duration
	headers    http.Header
	httpClient *http.Client
	ownsHTTP   bool
	instr      Instrumentation
	logger     *slog.Logger
	random     func() float64
}

// Option configures a Client.
type Option func(*Client) error

// WithAPIKey sets the API key. Otherwise TYPESAFE_API_KEY is used.
func WithAPIKey(key string) Option {
	return func(c *Client) error { c.apiKey = key; return nil }
}

// WithBaseURL sets the API root, for example for a gateway. Otherwise
// TYPESAFE_BASE_URL or https://api.typesafe.ai is used.
func WithBaseURL(u string) Option {
	return func(c *Client) error {
		v, err := normalizeBaseURL(u)
		if err != nil {
			return err
		}
		c.baseURL = v
		return nil
	}
}

// WithModel sets the default model. Otherwise TYPESAFE_DEFAULT_MODEL or
// jev-latest is used.
func WithModel(m string) Option {
	return func(c *Client) error { c.model = m; return nil }
}

// WithRetryPolicy replaces the default retry policy.
func WithRetryPolicy(p RetryPolicy) Option {
	return func(c *Client) error { c.retry = p; return nil }
}

// WithTimeout sets the timeout for each HTTP attempt. Default 10s. Zero
// disables the per-attempt timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) error {
		if d < 0 {
			return fmt.Errorf("typesafe: timeout must not be negative")
		}
		c.timeout = d
		return nil
	}
}

// WithHeaders adds headers to every request.
func WithHeaders(h http.Header) Option {
	return func(c *Client) error { c.headers = h.Clone(); return nil }
}

// WithHTTPClient uses a caller-supplied http.Client. The client is copied so
// the caller's value is not modified when instrumentation wraps its transport.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) error {
		if hc == nil {
			return fmt.Errorf("typesafe: http client must not be nil")
		}
		cp := *hc
		c.httpClient = &cp
		c.ownsHTTP = false
		return nil
	}
}

// WithInstrumentation attaches an observer, such as the otel subpackage's.
func WithInstrumentation(i Instrumentation) Option {
	return func(c *Client) error { c.instr = i; return nil }
}

// WithLogger sets the logger. Otherwise TYPESAFE_LOG_LEVEL selects a text
// logger on stderr, and unset means no logging.
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) error { c.logger = l; return nil }
}

// NewClient builds a Client. It fails when no API key is configured.
func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		retry:   DefaultRetryPolicy(),
		timeout: DefaultTimeout,
		headers: http.Header{},
		random:  rand.Float64,
	}
	for _, o := range opts {
		if err := o(c); err != nil {
			return nil, err
		}
	}
	if c.apiKey == "" {
		c.apiKey = os.Getenv(EnvAPIKey)
	}
	if c.apiKey == "" {
		return nil, ErrMissingAPIKey
	}
	if c.baseURL == "" {
		raw := os.Getenv(EnvBaseURL)
		if raw == "" {
			raw = DefaultBaseURL
		}
		v, err := normalizeBaseURL(raw)
		if err != nil {
			return nil, err
		}
		c.baseURL = v
	}
	if c.model == "" {
		c.model = os.Getenv(EnvDefaultModel)
	}
	if c.model == "" {
		c.model = DefaultModel
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{}
		c.ownsHTTP = true
	}
	if c.instr != nil {
		rt := c.httpClient.Transport
		if rt == nil {
			rt = http.DefaultTransport
		}
		c.httpClient.Transport = c.instr.Transport(rt)
	}
	if c.logger == nil {
		c.logger = NewLogger(os.Stderr, os.Getenv(EnvLogLevel))
	}
	return c, nil
}

func normalizeBaseURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("typesafe: invalid base URL %q", raw)
	}
	return strings.TrimRight(raw, "/"), nil
}

// NewLogger returns a text logger on w at the named level: debug, info,
// warning (or warn), error. Any other value, including "off" and "",
// returns a logger that discards everything.
func NewLogger(w io.Writer, level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warning", "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		return slog.New(slog.DiscardHandler)
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: lvl}))
}

// Close releases idle connections held by a client-owned http.Client.
func (c *Client) Close() error {
	if c.ownsHTTP {
		c.httpClient.CloseIdleConnections()
	}
	return nil
}

type requestConfig struct {
	model     string
	retry     RetryPolicy
	timeout   time.Duration
	headers   http.Header
	extraBody map[string]any
}

// RequestOption configures a single call.
type RequestOption func(*requestConfig)

// WithRequestModel overrides the client's model for this call.
func WithRequestModel(m string) RequestOption {
	return func(rc *requestConfig) { rc.model = m }
}

// WithRequestRetry overrides the retry policy for this call.
func WithRequestRetry(p RetryPolicy) RequestOption {
	return func(rc *requestConfig) { rc.retry = p }
}

// WithRequestTimeout overrides the per-attempt timeout for this call.
func WithRequestTimeout(d time.Duration) RequestOption {
	return func(rc *requestConfig) { rc.timeout = d }
}

// WithExtraHeaders adds headers to this call, replacing client headers with
// the same name.
func WithExtraHeaders(h http.Header) RequestOption {
	return func(rc *requestConfig) { rc.headers = h.Clone() }
}

// WithExtraBody merges fields into the top level of the request body. Use it
// for API fields this package does not model yet.
func WithExtraBody(fields map[string]any) RequestOption {
	return func(rc *requestConfig) { rc.extraBody = fields }
}

func (c *Client) requestConfig(opts []RequestOption) requestConfig {
	rc := requestConfig{model: c.model, retry: c.retry, timeout: c.timeout}
	for _, o := range opts {
		o(&rc)
	}
	return rc
}

func (c *Client) startInstrument(ctx context.Context, info RequestInfo) (context.Context, func(RequestResult)) {
	if c.instr == nil {
		return ctx, func(RequestResult) {}
	}
	return c.instr.RequestStart(ctx, info)
}

// SystemOne asks the named questions about state and returns the typed
// answers. state is a string, map, slice, or struct that encodes to JSON.
func (c *Client) SystemOne(ctx context.Context, state any, questions Questions, opts ...RequestOption) (*SystemOneResponse, error) {
	rc := c.requestConfig(opts)
	if err := validateContent("state", state, false); err != nil {
		return nil, err
	}
	if err := validateQuestions(questions); err != nil {
		return nil, err
	}
	body := map[string]any{"state": state, "model": rc.model, "questions": questions}
	for k, v := range rc.extraBody {
		body[k] = v
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, &ValidationError{Path: "body", Err: err}
	}
	info := RequestInfo{Operation: "system_one", Model: rc.model, State: state, Questions: questions}
	info.QuestionCount, info.NoulCount, info.ChoiceCount, info.ScoreCount = countQuestions(questions)
	ctx, finish := c.startInstrument(ctx, info)

	resp, raw, attempts, err := c.send(ctx, http.MethodPost, "/v1/systemone", payload, rc.headers, rc.retry, rc.timeout)
	if err != nil {
		finish(RequestResult{Attempts: attempts, Status: statusOf(resp), RequestID: requestIDOf(resp), Err: err})
		return nil, err
	}
	out, err := decodeSystemOne(raw)
	if err != nil {
		finish(RequestResult{Attempts: attempts, Status: resp.StatusCode, RequestID: requestIDOf(resp), Err: err})
		return nil, err
	}
	out.RequestID = requestIDOf(resp)
	out.Raw = &RawResponse{Status: resp.StatusCode, Header: resp.Header.Clone(), Body: raw}
	finish(RequestResult{Attempts: attempts, Status: resp.StatusCode, RequestID: out.RequestID, Model: out.Model, Usage: out.Usage, Response: out})
	return out, nil
}

// ListModels returns the models available to the account.
func (c *Client) ListModels(ctx context.Context, opts ...RequestOption) (*ListModelsResponse, error) {
	rc := c.requestConfig(opts)
	ctx, finish := c.startInstrument(ctx, RequestInfo{Operation: "list_models", Model: rc.model})
	resp, raw, attempts, err := c.send(ctx, http.MethodGet, "/v1/models", nil, rc.headers, rc.retry, rc.timeout)
	if err != nil {
		finish(RequestResult{Attempts: attempts, Status: statusOf(resp), RequestID: requestIDOf(resp), Err: err})
		return nil, err
	}
	out, err := decodeModels(raw)
	if err != nil {
		finish(RequestResult{Attempts: attempts, Status: resp.StatusCode, RequestID: requestIDOf(resp), Err: err})
		return nil, err
	}
	out.RequestID = requestIDOf(resp)
	out.Raw = &RawResponse{Status: resp.StatusCode, Header: resp.Header.Clone(), Body: raw}
	finish(RequestResult{Attempts: attempts, Status: resp.StatusCode, RequestID: out.RequestID})
	return out, nil
}

func statusOf(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

func requestIDOf(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	return resp.Header.Get(headerRequestID)
}

func countQuestions(qs Questions) (total, nouls, choices, scores int) {
	for _, q := range qs {
		total++
		switch v := q.(type) {
		case Noul, *Noul:
			nouls++
		case Choice, *Choice:
			choices++
		case Score, *Score:
			scores++
		case RawQuestion:
			switch v["type"] {
			case "noul":
				nouls++
			case "choice":
				choices++
			case "score":
				scores++
			}
		}
	}
	return total, nouls, choices, scores
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -race ./... 2>&1 | tail -5
```
Expected: `ok  github.com/therealbill/typesafe-go` with no race reports. If `TestSystemOneTimeout` is flaky on a loaded machine, raise the server delay to 500ms; do not loosen the assertion.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./... && go test -race ./...
git add client.go client_test.go transport.go
git commit -m "Add client with options, retries, and fake-server tests"
```

---
## Task 8: Typed decoding with `SystemOneAs[T]`

**Agent:** `core`

**Files:**
- Create: `typed.go`, `typed_test.go`

- [ ] **Step 1: Write the failing tests `typed_test.go`**

```go
package typesafe

import (
	"context"
	"errors"
	"testing"
)

type triage struct {
	Billing NoulAnswer    `json:"billing"`
	Tone    ChoiceAnswer  `json:"tone"`
	Urgency *ScoreAnswer  `json:"urgency"`
}

func TestSystemOneAs(t *testing.T) {
	fs := newFakeServer(t, okStep(t))
	c := newTestClient(t, fs.URL)
	got, full, err := SystemOneAs[triage](context.Background(), c, fixtureState, fixtureQuestions)
	if err != nil {
		t.Fatal(err)
	}
	if got.Billing.Noul != 0.99 || got.Tone.Choice != "angry" || got.Urgency == nil || got.Urgency.Score != 1.9 {
		t.Fatalf("typed %+v", got)
	}
	if full == nil || full.RequestID == "" || *full.Usage.InputTokens != 407 {
		t.Fatalf("full response missing: %+v", full)
	}
}

func TestSystemOneAsTypeMismatch(t *testing.T) {
	type wrong struct {
		Tone NoulAnswer `json:"tone"`
	}
	fs := newFakeServer(t, okStep(t))
	c := newTestClient(t, fs.URL)
	_, full, err := SystemOneAs[wrong](context.Background(), c, fixtureState, fixtureQuestions)
	var rve *ResponseValidationError
	if !errors.As(err, &rve) {
		t.Fatalf("want *ResponseValidationError, got %v", err)
	}
	if full == nil {
		t.Fatal("full response should still be returned on decode failure")
	}
}

func TestSystemOneAsPropagatesRequestErrors(t *testing.T) {
	fs := newFakeServer(t, step{status: 401, body: `{"detail":"no"}`})
	c := newTestClient(t, fs.URL)
	_, full, err := SystemOneAs[triage](context.Background(), c, fixtureState, fixtureQuestions)
	if !IsAuthError(err) || full != nil {
		t.Fatalf("err %v full %v", err, full)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -run TestSystemOneAs . 2>&1 | head -3
```
Expected: `undefined: SystemOneAs`.

- [ ] **Step 3: Write `typed.go`**

```go
package typesafe

import (
	"context"
	"encoding/json"
	"errors"
)

// SystemOneAs calls SystemOne and decodes the answers into T, a struct whose
// fields are NoulAnswer, ChoiceAnswer, or ScoreAnswer (or pointers to them)
// tagged with the question identifiers:
//
//	type Triage struct {
//	    Billing typesafe.NoulAnswer   `json:"billing"`
//	    Tone    typesafe.ChoiceAnswer `json:"tone"`
//	}
//
// The full response is returned alongside so usage and request ID are not
// lost. On a request error the response is nil. On a decode error the
// response is returned with the error.
func SystemOneAs[T any](ctx context.Context, c *Client, state any, questions Questions, opts ...RequestOption) (T, *SystemOneResponse, error) {
	var out T
	res, err := c.SystemOne(ctx, state, questions, opts...)
	if err != nil {
		return out, nil, err
	}
	var w struct {
		Answers json.RawMessage `json:"answers"`
	}
	if err := json.Unmarshal(res.Raw.Body, &w); err != nil {
		return out, res, fieldErr("answers", err)
	}
	if err := json.Unmarshal(w.Answers, &out); err != nil {
		var rve *ResponseValidationError
		if errors.As(err, &rve) {
			return out, res, err
		}
		return out, res, fieldErr("answers", err)
	}
	return out, res, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -run TestSystemOneAs -v . 2>&1 | tail -6
```
Expected: three PASS lines.

- [ ] **Step 5: Commit**

```bash
gofmt -l . ; go vet ./... && go test ./...
git add typed.go typed_test.go
git commit -m "Add SystemOneAs generic typed decoding"
```

---

## Task 9: Live integration test

**Agent:** `core`

**Files:**
- Create: `integration_test.go`

- [ ] **Step 1: Write `integration_test.go`**

```go
package typesafe

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestIntegrationLive talks to the real API. It is skipped unless
// TYPESAFE_API_KEY is set. Run with: go test -run Integration -v .
func TestIntegrationLive(t *testing.T) {
	if os.Getenv(EnvAPIKey) == "" {
		t.Skip("TYPESAFE_API_KEY not set")
	}
	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := c.SystemOne(ctx, fixtureState, fixtureQuestions)
	if err != nil {
		t.Fatalf("SystemOne: %v", err)
	}
	if res.RequestID == "" || res.Model == "" {
		t.Fatalf("missing metadata: %+v", res)
	}
	if len(res.Nouls()) != 1 || len(res.Choices()) != 1 || len(res.Scores()) != 1 {
		t.Fatalf("unexpected answer partition: %+v", res.Answers)
	}
	if p := res.Nouls()["billing"].Noul; p < 0.5 {
		t.Errorf("billing noul %.2f, expected a clear yes", p)
	}
	if ch := res.Choices()["tone"]; ch.Choice != "angry" {
		t.Errorf("tone %q, expected angry", ch.Choice)
	}
	if s := res.Scores()["urgency"]; s.Score < 1.0 || len(s.Legend) != 3 {
		t.Errorf("urgency %+v", s)
	}
	if res.Usage.InputTokens == nil || *res.Usage.InputTokens == 0 {
		t.Errorf("usage %+v", res.Usage)
	}

	models, err := c.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models.Models) == 0 {
		t.Fatal("no models returned")
	}
}
```

- [ ] **Step 2: Run it live**

```bash
go test -run Integration -v . 2>&1 | tail -5
```
Expected: `--- PASS: TestIntegrationLive`. If the tone answer differs from `angry`, report the actual answer to the lead rather than editing the assertion.

- [ ] **Step 3: Commit**

```bash
git add integration_test.go
git commit -m "Add env-gated live integration test"
```

---

## Task 10: OpenTelemetry instrumentation package

**Agent:** `otel` (starts after Task 1 is committed; the package only needs `instrument.go`, `response.go`, and `internal/version`)

**Files:**
- Create: `otel/otel.go`, `otel/otel_test.go`
- Modify: `go.mod`, `go.sum` (via `go get`)

- [ ] **Step 1: Add dependencies**

```bash
cd /Users/bill/Projects/gojev
go get go.opentelemetry.io/otel@v1.46.0 go.opentelemetry.io/otel/trace@v1.46.0 go.opentelemetry.io/otel/sdk@v1.46.0 go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.71.0
go mod tidy
grep -c "opentelemetry" go.mod
```
Expected: `go.mod` lists the four modules; the count is at least 4. Confirm `go 1.25` is still the directive (it will not be lowered).

- [ ] **Step 2: Write the failing tests `otel/otel_test.go`**

```go
package otel

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/therealbill/typesafe-go"
)

func newRecorder(t *testing.T) (*tracetest.InMemoryExporter, *sdktrace.TracerProvider) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return exp, tp
}

func attrMap(kvs []attribute.KeyValue) map[string]any {
	m := map[string]any{}
	for _, kv := range kvs {
		m[string(kv.Key)] = kv.Value.AsInterface()
	}
	return m
}

func intp(i int) *int { return &i }

func sampleInfo() typesafe.RequestInfo {
	return typesafe.RequestInfo{
		Operation: "system_one", Model: "jev-latest",
		QuestionCount: 3, NoulCount: 1, ChoiceCount: 1, ScoreCount: 1,
		State:     map[string]any{"ticket": "secret customer text"},
		Questions: typesafe.Questions{"billing": typesafe.Noul{Instructions: "Is it billing?"}},
	}
}

func TestSpanOnSuccess(t *testing.T) {
	exp, tp := newRecorder(t)
	inst := New(WithTracerProvider(tp))
	ctx, finish := inst.RequestStart(context.Background(), sampleInfo())
	if !trace.SpanContextFromContext(ctx).IsValid() {
		t.Fatal("returned context must carry the span")
	}
	res := &typesafe.SystemOneResponse{Answers: map[string]typesafe.Answer{"billing": typesafe.NoulAnswer{Noul: 0.9}}}
	finish(typesafe.RequestResult{Attempts: 2, Status: 200, RequestID: "req_1", Model: "jev-1.13.0",
		Usage: typesafe.Usage{InputTokens: intp(407), OutputTokens: intp(72)}, Response: res})

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("spans %d", len(spans))
	}
	s := spans[0]
	if s.Name != "typesafe.system_one" || s.SpanKind != trace.SpanKindClient {
		t.Fatalf("span %s kind %v", s.Name, s.SpanKind)
	}
	a := attrMap(s.Attributes)
	want := map[string]any{
		AttrProviderName: "typesafe", AttrSystem: "typesafe",
		AttrRequestModel: "jev-latest", AttrResponseModel: "jev-1.13.0",
		AttrInputTokens: int64(407), AttrOutputTokens: int64(72),
		AttrRequestID: "req_1", AttrQuestionCount: int64(3), AttrNoulCount: int64(1),
		AttrChoiceCount: int64(1), AttrScoreCount: int64(1), AttrRetryAttempts: int64(2), AttrHTTPStatus: int64(200),
	}
	for k, v := range want {
		if a[k] != v {
			t.Errorf("%s = %v want %v", k, a[k], v)
		}
	}
	for _, k := range []string{AttrState, AttrQuestions, "typesafe.answer.billing.noul", AttrErrorType} {
		if _, ok := a[k]; ok {
			t.Errorf("%s must not be recorded by default", k)
		}
	}
	if s.Status.Code != codes.Unset {
		t.Fatalf("status %v", s.Status)
	}
}

func TestSpanOnError(t *testing.T) {
	exp, tp := newRecorder(t)
	inst := New(WithTracerProvider(tp))
	_, finish := inst.RequestStart(context.Background(), sampleInfo())
	err := &typesafe.APIError{Status: 503, Endpoint: "POST /v1/systemone"}
	finish(typesafe.RequestResult{Attempts: 3, Status: 503, Err: err})
	s := exp.GetSpans()[0]
	if s.Status.Code != codes.Error {
		t.Fatalf("status %v", s.Status)
	}
	a := attrMap(s.Attributes)
	if a[AttrErrorType] != "typesafe.APIError" || a[AttrHTTPStatus] != int64(503) || a[AttrRetryAttempts] != int64(3) {
		t.Fatalf("attrs %v", a)
	}
	if len(s.Events) == 0 || s.Events[0].Name != "exception" {
		t.Fatal("error should be recorded as an exception event")
	}
	_ = errors.New
}

func TestRecordContentOptIn(t *testing.T) {
	exp, tp := newRecorder(t)
	inst := New(WithTracerProvider(tp), WithRecordContent(20), WithRecordAnswers())
	_, finish := inst.RequestStart(context.Background(), sampleInfo())
	res := &typesafe.SystemOneResponse{Answers: map[string]typesafe.Answer{
		"billing": typesafe.NoulAnswer{Noul: 0.9},
		"tone":    typesafe.ChoiceAnswer{Choice: "angry", Confidence: 0.8},
		"urgency": typesafe.ScoreAnswer{Score: 1.5, Confidence: 0.7},
	}}
	finish(typesafe.RequestResult{Attempts: 1, Status: 200, Response: res})
	a := attrMap(exp.GetSpans()[0].Attributes)
	state, _ := a[AttrState].(string)
	if !strings.HasPrefix(state, `{"ticket":"secret cu`) || !strings.HasSuffix(state, "...(truncated)") {
		t.Fatalf("state attr %q", state)
	}
	if _, ok := a[AttrQuestions]; !ok {
		t.Fatal("questions attr missing")
	}
	if a["typesafe.answer.billing.noul"] != 0.9 || a["typesafe.answer.tone.choice"] != "angry" || a["typesafe.answer.tone.confidence"] != 0.8 || a["typesafe.answer.urgency.score"] != 1.5 {
		t.Fatalf("answer attrs %v", a)
	}
}

func TestListModelsSpan(t *testing.T) {
	exp, tp := newRecorder(t)
	inst := New(WithTracerProvider(tp))
	_, finish := inst.RequestStart(context.Background(), typesafe.RequestInfo{Operation: "list_models", Model: "jev-latest"})
	finish(typesafe.RequestResult{Attempts: 1, Status: 200})
	s := exp.GetSpans()[0]
	if s.Name != "typesafe.list_models" {
		t.Fatalf("name %s", s.Name)
	}
	if _, ok := attrMap(s.Attributes)[AttrQuestionCount]; ok {
		t.Fatal("question counts do not apply to list_models")
	}
}

func TestTransportWraps(t *testing.T) {
	_, tp := newRecorder(t)
	inst := New(WithTracerProvider(tp))
	rt := inst.Transport(nil)
	if rt == nil {
		t.Fatal("transport must not be nil")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./otel/ 2>&1 | head -3
```
Expected: `undefined: New` and attribute constants.

- [ ] **Step 4: Write `otel/otel.go`**

```go
// Package otel instruments a typesafe.Client with OpenTelemetry traces.
//
// Attach it with typesafe.WithInstrumentation(otel.New()). Each SystemOne or
// ListModels call becomes a client span named typesafe.system_one or
// typesafe.list_models, with one child HTTP span per attempt. State and
// question content are never recorded unless WithRecordContent is set.
//
// This package never installs a global tracer provider. Set one in your
// application before building the client, or pass WithTracerProvider.
package otel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/therealbill/typesafe-go"
	"github.com/therealbill/typesafe-go/internal/version"
)

// TracerName is the instrumentation scope name.
const TracerName = "github.com/therealbill/typesafe-go"

// ProviderName is the value of the gen_ai.provider.name attribute.
const ProviderName = "typesafe"

// Span attribute keys. The gen_ai.* keys follow the OpenTelemetry GenAI
// semantic conventions; gen_ai.system is the older spelling and is set too.
const (
	AttrProviderName  = "gen_ai.provider.name"
	AttrSystem        = "gen_ai.system"
	AttrRequestModel  = "gen_ai.request.model"
	AttrResponseModel = "gen_ai.response.model"
	AttrInputTokens   = "gen_ai.usage.input_tokens"
	AttrOutputTokens  = "gen_ai.usage.output_tokens"
	AttrRequestID     = "typesafe.request_id"
	AttrQuestionCount = "typesafe.questions.count"
	AttrNoulCount     = "typesafe.questions.noul"
	AttrChoiceCount   = "typesafe.questions.choice"
	AttrScoreCount    = "typesafe.questions.score"
	AttrRetryAttempts = "typesafe.retry.attempts"
	AttrHTTPStatus    = "http.response.status_code"
	AttrErrorType     = "error.type"
	AttrState         = "typesafe.state"
	AttrQuestions     = "typesafe.questions"
)

const truncatedMarker = "...(truncated)"

// Option configures the instrumentation.
type Option func(*instrumentation)

// WithTracerProvider uses tp instead of the global provider.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(i *instrumentation) { i.tp = tp }
}

// WithRecordContent records the request state and questions as JSON span
// attributes, each truncated to maxBytes. Off by default because state is
// often customer data.
func WithRecordContent(maxBytes int) Option {
	return func(i *instrumentation) { i.contentBytes = maxBytes }
}

// WithRecordAnswers records each answer's value and confidence as
// typesafe.answer.<key>.<field> attributes. Off by default.
func WithRecordAnswers() Option {
	return func(i *instrumentation) { i.answers = true }
}

type instrumentation struct {
	tp           trace.TracerProvider
	contentBytes int
	answers      bool
}

// New returns an Instrumentation for typesafe.WithInstrumentation.
func New(opts ...Option) typesafe.Instrumentation {
	i := &instrumentation{}
	for _, o := range opts {
		o(i)
	}
	return i
}

func (i *instrumentation) provider() trace.TracerProvider {
	if i.tp != nil {
		return i.tp
	}
	return otelapi.GetTracerProvider()
}

func (i *instrumentation) tracer() trace.Tracer {
	return i.provider().Tracer(TracerName, trace.WithInstrumentationVersion(version.Version))
}

// RequestStart opens the operation span.
func (i *instrumentation) RequestStart(ctx context.Context, info typesafe.RequestInfo) (context.Context, func(typesafe.RequestResult)) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrProviderName, ProviderName),
		attribute.String(AttrSystem, ProviderName),
		attribute.String(AttrRequestModel, info.Model),
	}
	if info.Operation == "system_one" {
		attrs = append(attrs,
			attribute.Int(AttrQuestionCount, info.QuestionCount),
			attribute.Int(AttrNoulCount, info.NoulCount),
			attribute.Int(AttrChoiceCount, info.ChoiceCount),
			attribute.Int(AttrScoreCount, info.ScoreCount),
		)
		if i.contentBytes > 0 {
			attrs = append(attrs,
				attribute.String(AttrState, truncatedJSON(info.State, i.contentBytes)),
				attribute.String(AttrQuestions, truncatedJSON(info.Questions, i.contentBytes)),
			)
		}
	}
	ctx, span := i.tracer().Start(ctx, "typesafe."+info.Operation,
		trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(attrs...))
	return ctx, func(r typesafe.RequestResult) {
		defer span.End()
		out := []attribute.KeyValue{attribute.Int(AttrRetryAttempts, r.Attempts)}
		if r.Status != 0 {
			out = append(out, attribute.Int(AttrHTTPStatus, r.Status))
		}
		if r.RequestID != "" {
			out = append(out, attribute.String(AttrRequestID, r.RequestID))
		}
		if r.Model != "" {
			out = append(out, attribute.String(AttrResponseModel, r.Model))
		}
		if r.Usage.InputTokens != nil {
			out = append(out, attribute.Int(AttrInputTokens, *r.Usage.InputTokens))
		}
		if r.Usage.OutputTokens != nil {
			out = append(out, attribute.Int(AttrOutputTokens, *r.Usage.OutputTokens))
		}
		if i.answers && r.Response != nil {
			out = append(out, answerAttributes(r.Response)...)
		}
		span.SetAttributes(out...)
		if r.Err != nil {
			span.RecordError(r.Err)
			span.SetStatus(codes.Error, r.Err.Error())
			span.SetAttributes(attribute.String(AttrErrorType, errorType(r.Err)))
		}
	}
}

// Transport wraps rt with otelhttp so each HTTP attempt is a child span.
func (i *instrumentation) Transport(rt http.RoundTripper) http.RoundTripper {
	if rt == nil {
		rt = http.DefaultTransport
	}
	opts := []otelhttp.Option{}
	if i.tp != nil {
		opts = append(opts, otelhttp.WithTracerProvider(i.tp))
	}
	return otelhttp.NewTransport(rt, opts...)
}

func answerAttributes(res *typesafe.SystemOneResponse) []attribute.KeyValue {
	keys := make([]string, 0, len(res.Answers))
	for k := range res.Answers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []attribute.KeyValue
	for _, k := range keys {
		p := "typesafe.answer." + k
		switch v := res.Answers[k].(type) {
		case typesafe.NoulAnswer:
			out = append(out, attribute.Float64(p+".noul", v.Noul))
		case typesafe.ChoiceAnswer:
			out = append(out, attribute.String(p+".choice", v.Choice), attribute.Float64(p+".confidence", v.Confidence))
		case typesafe.ScoreAnswer:
			out = append(out, attribute.Float64(p+".score", v.Score), attribute.Float64(p+".confidence", v.Confidence))
		}
	}
	return out
}

func truncatedJSON(v any, maxBytes int) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<unencodable %T>", v)
	}
	s := string(b)
	if len(s) > maxBytes {
		return s[:maxBytes] + truncatedMarker
	}
	return s
}

func errorType(err error) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", err), "*")
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test -v ./otel/ 2>&1 | tail -8
```
Expected: five PASS lines and `ok`.

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; go vet ./... && go build ./... && go test ./otel/
git add go.mod go.sum otel/otel.go otel/otel_test.go
git commit -m "Add OpenTelemetry instrumentation package"
```

---

## Task 11: End-to-end otel test through a real client

**Agent:** `otel` (after Task 7 is committed)

**Files:**
- Modify: `otel/otel_test.go` (append)

- [ ] **Step 1: Append the test**

```go
func TestEndToEndParentage(t *testing.T) {
	exp, tp := newRecorder(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Traceparent") == "" {
			t.Error("traceparent header should be propagated by otelhttp")
		}
		w.Header().Set("x-typesafe-request-id", "req_e2e")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"jev-1.13.0","answers":{"b":{"type":"noul","noul":0.5}},"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	t.Cleanup(srv.Close)

	prev := otelapi.GetTextMapPropagator()
	otelapi.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otelapi.SetTextMapPropagator(prev) })

	client, err := typesafe.NewClient(typesafe.WithAPIKey("k"), typesafe.WithBaseURL(srv.URL), typesafe.WithInstrumentation(New(WithTracerProvider(tp))))
	if err != nil {
		t.Fatal(err)
	}
	ctx, parent := tp.Tracer("test").Start(context.Background(), "caller")
	if _, err := client.SystemOne(ctx, "s", typesafe.Questions{"b": typesafe.Noul{Instructions: "?"}}); err != nil {
		t.Fatal(err)
	}
	parent.End()

	spans := exp.GetSpans()
	byName := map[string]tracetest.SpanStub{}
	for _, s := range spans {
		byName[s.Name] = s
	}
	op, ok := byName["typesafe.system_one"]
	if !ok {
		t.Fatalf("operation span missing, have %v", names(spans))
	}
	if op.Parent.SpanID() != parent.SpanContext().SpanID() {
		t.Fatal("operation span should be a child of the caller span")
	}
	var httpSpan *tracetest.SpanStub
	for i := range spans {
		if spans[i].SpanKind == trace.SpanKindClient && spans[i].Name != "typesafe.system_one" {
			httpSpan = &spans[i]
		}
	}
	if httpSpan == nil || httpSpan.Parent.SpanID() != op.SpanContext.SpanID() {
		t.Fatalf("HTTP span should be a child of the operation span; spans %v", names(spans))
	}
	if attrMap(op.Attributes)[AttrRequestID] != "req_e2e" {
		t.Fatal("request id not recorded")
	}
}

func names(spans tracetest.SpanStubs) []string {
	out := make([]string, 0, len(spans))
	for _, s := range spans {
		out = append(out, s.Name)
	}
	return out
}
```

Add to the imports of `otel/otel_test.go`: `"net/http"`, `"net/http/httptest"`, `otelapi "go.opentelemetry.io/otel"`, `"go.opentelemetry.io/otel/propagation"`.

- [ ] **Step 2: Run and commit**

```bash
go test -race -v ./otel/ 2>&1 | tail -8
git add otel/otel_test.go
git commit -m "Add end-to-end otel span parentage test"
```
Expected: all PASS.

---
## Task 12: Tooling: Makefile, lint, gitignore, CI, goreleaser

**Agent:** `cli` (starts after Task 1 is committed)

**Files:**
- Create: `Makefile`, `.golangci.yml`, `.gitignore`, `.goreleaser.yaml`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`

- [ ] **Step 1: Write `.gitignore`**

```
bin/
dist/
coverage.out
selfreview-report.json
```

- [ ] **Step 2: Write `.golangci.yml`**

```yaml
version: "2"
linters:
  default: standard
  enable:
    - misspell
    - revive
    - unconvert
    - unparam
  settings:
    revive:
      rules:
        - name: exported
          arguments: ["checkPrivateReceivers", "disableStutteringCheck"]
formatters:
  enable:
    - gofmt
    - goimports
```

- [ ] **Step 3: Write `Makefile`**

```make
MODULE   := github.com/therealbill/typesafe-go
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS  := -s -w -X $(MODULE)/internal/version.Version=$(VERSION) -X $(MODULE)/internal/version.Commit=$(COMMIT)

.DEFAULT_GOAL := help
.DELETE_ON_ERROR:
.PHONY: help test lint vuln build integration selfreview docs clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

test: ## Run unit tests with the race detector
	go test -race -cover ./...

lint: ## Run gofmt check, go vet, and golangci-lint
	@test -z "$$(gofmt -l .)" || (gofmt -l . && echo "gofmt: files need formatting" && exit 1)
	go vet ./...
	golangci-lint run ./...

vuln: ## Run govulncheck
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

build: ## Build bin/jev
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o bin/jev ./cmd/jev

integration: ## Run the live API test (needs TYPESAFE_API_KEY)
	go test -run Integration -v .

selfreview: build ## Run the Jev-driven self-review (needs TYPESAFE_API_KEY)
	go run ./tools/selfreview -jev ./bin/jev

docs: ## Check that every docs/ page is reachable from README.md and links resolve
	@./tools/checkdocs.sh

clean: ## Remove build outputs
	rm -rf bin dist coverage.out selfreview-report.json
```

Indentation under each target must be a real tab.

- [ ] **Step 4: Write `tools/checkdocs.sh`**

```bash
#!/usr/bin/env bash
# Verifies that every markdown page under docs/ (excluding superpowers/) is
# linked from README.md or another docs page, and that every relative link in
# README.md and docs/ resolves to a file.
set -euo pipefail
cd "$(dirname "$0")/.."
status=0
pages=$(find docs -name '*.md' -not -path 'docs/superpowers/*' | sort)
for page in $pages; do
  if ! grep -rq --include='*.md' -F "$(basename "$page")" README.md docs; then
    echo "unlinked: $page"; status=1
  fi
done
while IFS= read -r line; do
  file=${line%%:*}; link=${line#*:}
  link=${link%%#*}
  [ -z "$link" ] && continue
  case "$link" in http*|mailto*) continue;; esac
  target="$(dirname "$file")/$link"
  if [ ! -e "$target" ]; then
    echo "broken link in $file: $link"; status=1
  fi
done < <(grep -rhoE --include='*.md' '\]\(([^)]+)\)' README.md docs 2>/dev/null | sed -E 's/\]\((.*)\)/\1/' | while read -r l; do grep -rlF --include='*.md' "]($l)" README.md docs | sed "s|$|:$l|"; done)
exit $status
```

```bash
chmod +x tools/checkdocs.sh
```

- [ ] **Step 5: Write `.goreleaser.yaml`**

```yaml
version: 2
project_name: jev
builds:
  - id: jev
    main: ./cmd/jev
    binary: jev
    env:
      - CGO_ENABLED=0
    goos: [darwin, linux]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w
      - -X github.com/therealbill/typesafe-go/internal/version.Version={{ .Version }}
      - -X github.com/therealbill/typesafe-go/internal/version.Commit={{ .ShortCommit }}
archives:
  - id: jev
    formats: [tar.gz]
    name_template: "jev_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
checksum:
  name_template: checksums.txt
changelog:
  use: git
release:
  github:
    owner: therealbill
    name: typesafe-go
```

- [ ] **Step 6: Write `.github/workflows/ci.yml`**

```yaml
name: ci
on:
  push:
    branches: [main]
  pull_request:
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go test -race -cover ./...
      - run: go run golang.org/x/vuln/cmd/govulncheck@latest ./...
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: golangci/golangci-lint-action@v8
        with:
          version: v2.5
```

- [ ] **Step 7: Write `.github/workflows/release.yml`**

```yaml
name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 8: Verify and commit**

```bash
make help
make lint 2>&1 | tail -5
git add Makefile .golangci.yml .gitignore .goreleaser.yaml .github/workflows/ci.yml .github/workflows/release.yml tools/checkdocs.sh
git commit -m "Add Makefile, lint config, CI, and goreleaser configuration"
```
Expected: `make help` lists the targets; `make lint` passes on whatever code exists at this point (if it reports issues in files owned by `core`, report them to the lead instead of editing). `make build` will fail until Task 15 creates `cmd/jev`; that is expected.

---

## Task 13: CLI exit codes and error JSON

**Agent:** `cli` (after Task 7 is committed)

**Files:**
- Create: `internal/cli/exit.go`, `internal/cli/exit_test.go`, `internal/cli/output.go`
- Modify: `go.mod`, `go.sum` (via `go get`)

- [ ] **Step 1: Add Cobra**

```bash
go get github.com/spf13/cobra@v1.10.2
```

- [ ] **Step 2: Write the failing tests `internal/cli/exit_test.go`**

```go
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/therealbill/typesafe-go"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
		kind string
	}{
		{"validation", &typesafe.ValidationError{Path: "questions", Err: errors.New("x")}, ExitValidation, "validation"},
		{"401", &typesafe.APIError{Status: 401}, ExitAuth, "auth"},
		{"403", &typesafe.APIError{Status: 403}, ExitAuth, "auth"},
		{"400", &typesafe.APIError{Status: 400}, ExitRequest, "request"},
		{"404", &typesafe.APIError{Status: 404}, ExitRequest, "request"},
		{"422", &typesafe.APIError{Status: 422}, ExitRequest, "request"},
		{"418", &typesafe.APIError{Status: 418}, ExitRequest, "request"},
		{"429", &typesafe.RateLimitError{APIError: typesafe.APIError{Status: 429}}, ExitRateLimit, "rate_limit"},
		{"503", &typesafe.APIError{Status: 503}, ExitServer, "server"},
		{"529", &typesafe.APIError{Status: 529}, ExitServer, "server"},
		{"connection", &typesafe.ConnectionError{Err: errors.New("refused")}, ExitConnection, "connection"},
		{"timeout", &typesafe.TimeoutError{ConnectionError: typesafe.ConnectionError{Err: context.DeadlineExceeded}}, ExitConnection, "connection"},
		{"invalid response", &typesafe.ResponseValidationError{FieldPath: "answers.a", Err: errors.New("x")}, ExitServer, "invalid_response"},
		{"missing key", typesafe.ErrMissingAPIKey, ExitUsage, "usage"},
		{"usage", &usageError{errors.New("bad flag")}, ExitUsage, "usage"},
		{"unknown", errors.New("?"), ExitUsage, "usage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, kind := classify(tt.err)
			if code != tt.code || kind != tt.kind {
				t.Fatalf("got (%d, %s) want (%d, %s)", code, kind, tt.code, tt.kind)
			}
		})
	}
}

func TestFailWritesJSONAndReturnsExitError(t *testing.T) {
	var out, errOut bytes.Buffer
	io := IO{Out: &out, Err: &errOut}
	err := fail(io, false, &typesafe.APIError{Status: 401, Endpoint: "POST /v1/systemone", RequestID: "req_9", Body: []byte(`{"detail":{"error_type":"authentication_error","message":"Cannot authenticate"}}`)})
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != ExitAuth {
		t.Fatalf("err %v", err)
	}
	var payload struct {
		Error struct {
			Kind      string `json:"kind"`
			Status    int    `json:"status"`
			RequestID string `json:"request_id"`
			Message   string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("stdout not JSON: %s", out.String())
	}
	if payload.Error.Kind != "auth" || payload.Error.Status != 401 || payload.Error.RequestID != "req_9" || payload.Error.Message != "authentication_error: Cannot authenticate" {
		t.Fatalf("payload %+v", payload)
	}
	if !bytes.Contains(errOut.Bytes(), []byte("jev: ")) {
		t.Fatalf("stderr %q", errOut.String())
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/cli/ 2>&1 | head -3
```
Expected: undefined `classify`, `fail`, `IO`, `ExitError`.

- [ ] **Step 4: Write `internal/cli/output.go`**

```go
package cli

import (
	"encoding/json"
	"io"
)

// IO bundles the streams a command reads and writes.
type IO struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// writeJSON encodes v to w as one JSON document followed by a newline.
func writeJSON(w io.Writer, v any, pretty bool) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}
```

- [ ] **Step 5: Write `internal/cli/exit.go`**

```go
package cli

import (
	"errors"
	"fmt"

	"github.com/therealbill/typesafe-go"
)

// Exit codes. A caller can branch on these without parsing output.
const (
	ExitOK         = 0
	ExitUsage      = 1 // bad flags, unreadable or invalid request JSON, missing API key
	ExitValidation = 2 // request failed client-side validation
	ExitAuth       = 3 // 401 or 403
	ExitRequest    = 4 // other 4xx: 400, 404, 422
	ExitRateLimit  = 5 // 429 after retries
	ExitServer     = 6 // 5xx after retries, or an unreadable 2xx body
	ExitConnection = 7 // connection failure or timeout
)

// ExitError carries the process exit code for an error.
type ExitError struct {
	Code int
	Kind string
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

// usageError marks errors caused by how the command was invoked.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

func classify(err error) (int, string) {
	var ve *typesafe.ValidationError
	if errors.As(err, &ve) {
		return ExitValidation, "validation"
	}
	var rl *typesafe.RateLimitError
	if errors.As(err, &rl) {
		return ExitRateLimit, "rate_limit"
	}
	var api *typesafe.APIError
	if errors.As(err, &api) {
		switch {
		case api.Status == 401 || api.Status == 403:
			return ExitAuth, "auth"
		case api.Status >= 500:
			return ExitServer, "server"
		default:
			return ExitRequest, "request"
		}
	}
	var ce *typesafe.ConnectionError
	if errors.As(err, &ce) {
		return ExitConnection, "connection"
	}
	var rve *typesafe.ResponseValidationError
	if errors.As(err, &rve) {
		return ExitServer, "invalid_response"
	}
	return ExitUsage, "usage"
}

type errorPayload struct {
	Kind      string `json:"kind"`
	Status    int    `json:"status,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Message   string `json:"message"`
}

// fail reports err on both streams and returns the ExitError to propagate.
func fail(io IO, pretty bool, err error) error {
	code, kind := classify(err)
	p := errorPayload{Kind: kind, Message: err.Error()}
	var api *typesafe.APIError
	if errors.As(err, &api) {
		p.Status = api.Status
		p.RequestID = api.RequestID
		if msg := api.Message(); msg != "" {
			p.Message = msg
		}
	}
	_ = writeJSON(io.Out, map[string]any{"error": p}, pretty)
	fmt.Fprintln(io.Err, "jev:", err)
	return &ExitError{Code: code, Kind: kind, Err: err}
}
```

- [ ] **Step 6: Run tests to verify they pass, then commit**

```bash
go test -v ./internal/cli/ 2>&1 | tail -6
git add go.mod go.sum internal/cli/exit.go internal/cli/exit_test.go internal/cli/output.go
git commit -m "Add CLI exit codes and error output"
```

---
## Task 14: CLI request parsing (JSON and flags)

**Agent:** `cli`

**Files:**
- Create: `internal/cli/request.go`, `internal/cli/request_test.go`

- [ ] **Step 1: Write the failing tests `internal/cli/request_test.go`**

```go
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/therealbill/typesafe-go"
)

func TestParseRequestJSON(t *testing.T) {
	doc := `{"model":"jev-preview","state":{"ticket":"hi"},"questions":{
		"b":{"type":"noul","instructions":"Is it billing?","criteria":{"true":"yes","false":"no"}},
		"t":{"type":"choice","instructions":"tone","criteria":{"calm":null,"angry":"mad"}},
		"u":{"type":"score","instructions":"urgency","criteria":["low","high"]},
		"f":{"type":"future","weight":2}},
		"weight":3}`
	req, err := parseRequestJSON([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if req.Model != "jev-preview" {
		t.Fatalf("model %q", req.Model)
	}
	if req.State.(map[string]any)["ticket"] != "hi" {
		t.Fatalf("state %v", req.State)
	}
	n := req.Questions["b"].(typesafe.Noul)
	if n.Instructions != "Is it billing?" || n.Criteria == nil || n.Criteria.True != "yes" || n.Criteria.False != "no" {
		t.Fatalf("noul %+v", n)
	}
	c := req.Questions["t"].(typesafe.Choice)
	if c.Criteria["angry"] != "mad" || c.Criteria["calm"] != nil {
		t.Fatalf("choice %+v", c)
	}
	s := req.Questions["u"].(typesafe.Score)
	if len(s.Criteria) != 2 || s.Criteria[1] != "high" {
		t.Fatalf("score %+v", s)
	}
	r := req.Questions["f"].(typesafe.RawQuestion)
	if r["type"] != "future" || r["weight"] != float64(2) {
		t.Fatalf("raw %+v", r)
	}
	if req.Extra["weight"] != float64(3) {
		t.Fatalf("extra %v", req.Extra)
	}
	out, _ := json.Marshal(req.Questions["b"])
	if string(out) != `{"criteria":{"false":"no","true":"yes"},"instructions":"Is it billing?","type":"noul"}` {
		t.Fatalf("re-marshaled noul %s", out)
	}
}

func TestParseRequestJSONErrors(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{"not json", `{`, "invalid request JSON"},
		{"array", `[]`, "invalid request JSON"},
		{"no state", `{"questions":{"a":{"type":"noul"}}}`, `"state" is required`},
		{"no questions", `{"state":"x"}`, `"questions" is required`},
		{"question not object", `{"state":"x","questions":{"a":5}}`, "questions.a"},
		{"question no type", `{"state":"x","questions":{"a":{"instructions":"?"}}}`, "questions.a.type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseRequestJSON([]byte(tt.doc))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v want substring %q", err, tt.want)
			}
		})
	}
}

func TestRequestFromFlags(t *testing.T) {
	o := &askOptions{
		state:   "I was charged twice",
		nouls:   []string{"billing=Is this about billing?"},
		choices: []string{"tone=What is the tone?:calm, angry,neutral"},
		scores:  []string{"urgency=How urgent?:low|mid|high"},
	}
	req, err := requestFromFlags(o, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if req.State != "I was charged twice" {
		t.Fatalf("state %v", req.State)
	}
	if req.Questions["billing"].(typesafe.Noul).Instructions != "Is this about billing?" {
		t.Fatalf("noul %+v", req.Questions["billing"])
	}
	c := req.Questions["tone"].(typesafe.Choice)
	if c.Instructions != "What is the tone?" || len(c.Criteria) != 3 || c.Criteria["angry"] != nil {
		t.Fatalf("choice %+v", c)
	}
	if _, ok := c.Criteria["angry"]; !ok {
		t.Fatal("labels must be trimmed")
	}
	s := req.Questions["urgency"].(typesafe.Score)
	if s.Instructions != "How urgent?" || len(s.Criteria) != 3 || s.Criteria[2] != "high" {
		t.Fatalf("score %+v", s)
	}
}

func TestRequestFromFlagsStateSources(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.txt")
	if err := os.WriteFile(path, []byte("from file"), 0o600); err != nil {
		t.Fatal(err)
	}
	req, err := requestFromFlags(&askOptions{state: "@" + path, nouls: []string{"a=b"}}, strings.NewReader(""))
	if err != nil || req.State != "from file" {
		t.Fatalf("file state: %v %v", req, err)
	}
	req, err = requestFromFlags(&askOptions{state: "-", nouls: []string{"a=b"}}, strings.NewReader("from stdin"))
	if err != nil || req.State != "from stdin" {
		t.Fatalf("stdin state: %v %v", req, err)
	}
}

func TestRequestFromFlagsErrors(t *testing.T) {
	tests := []struct {
		name string
		o    *askOptions
		want string
	}{
		{"no state", &askOptions{nouls: []string{"a=b"}}, "--state is required"},
		{"noul no equals", &askOptions{state: "s", nouls: []string{"ab"}}, "--noul"},
		{"choice no colon", &askOptions{state: "s", choices: []string{"a=b"}}, "--choice"},
		{"choice empty label", &askOptions{state: "s", choices: []string{"a=b:x,,y"}}, "--choice"},
		{"score no bar", &askOptions{state: "s", scores: []string{"a=b:only"}}, "--score"},
		{"duplicate key", &askOptions{state: "s", nouls: []string{"a=b"}, scores: []string{"a=b:x|y"}}, "duplicate"},
		{"missing file", &askOptions{state: "@/nonexistent/file", nouls: []string{"a=b"}}, "state file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := requestFromFlags(tt.o, strings.NewReader(""))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v want substring %q", err, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/cli/ 2>&1 | head -3
```
Expected: undefined `parseRequestJSON`, `askOptions`, `requestFromFlags`.

- [ ] **Step 3: Write `internal/cli/request.go`**

```go
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/therealbill/typesafe-go"
)

// askOptions are the flags of `jev ask`.
type askOptions struct {
	file    string
	state   string
	nouls   []string
	choices []string
	scores  []string
	raw     bool
}

// askRequest is a parsed request ready for the client.
type askRequest struct {
	State     any
	Questions typesafe.Questions
	Model     string
	Extra     map[string]any
}

func (o *askOptions) hasQuestionFlags() bool {
	return o.state != "" || len(o.nouls)+len(o.choices)+len(o.scores) > 0
}

// buildRequest chooses flag mode or JSON mode. JSON comes from --file, or
// from stdin when --file is empty and no question flags were given.
func buildRequest(o *askOptions, stdin io.Reader) (*askRequest, error) {
	if o.hasQuestionFlags() {
		if o.file != "" {
			return nil, errors.New("--file cannot be combined with --state, --noul, --choice, or --score")
		}
		return requestFromFlags(o, stdin)
	}
	var data []byte
	var err error
	if o.file == "" || o.file == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(o.file)
	}
	if err != nil {
		return nil, fmt.Errorf("read request: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, errors.New("no request given: pass --file, pipe JSON to stdin, or use --state with --noul/--choice/--score")
	}
	return parseRequestJSON(data)
}

// parseRequestJSON decodes the HTTP request body shape:
// {"state": ..., "questions": {...}, "model": "..."} plus any extra fields.
func parseRequestJSON(data []byte) (*askRequest, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("invalid request JSON: %w", err)
	}
	req := &askRequest{Questions: typesafe.Questions{}, Extra: map[string]any{}}
	stateRaw, ok := top["state"]
	if !ok {
		return nil, errors.New(`invalid request: "state" is required`)
	}
	if err := json.Unmarshal(stateRaw, &req.State); err != nil {
		return nil, fmt.Errorf("invalid request: state: %w", err)
	}
	qRaw, ok := top["questions"]
	if !ok {
		return nil, errors.New(`invalid request: "questions" is required`)
	}
	var qs map[string]json.RawMessage
	if err := json.Unmarshal(qRaw, &qs); err != nil {
		return nil, fmt.Errorf("invalid request: questions: %w", err)
	}
	for key, raw := range qs {
		q, err := parseQuestion(key, raw)
		if err != nil {
			return nil, err
		}
		req.Questions[key] = q
	}
	if m, ok := top["model"]; ok {
		if err := json.Unmarshal(m, &req.Model); err != nil {
			return nil, fmt.Errorf("invalid request: model: %w", err)
		}
	}
	for k, v := range top {
		switch k {
		case "state", "questions", "model":
			continue
		}
		var any_ any
		if err := json.Unmarshal(v, &any_); err != nil {
			return nil, fmt.Errorf("invalid request: %s: %w", k, err)
		}
		req.Extra[k] = any_
	}
	return req, nil
}

func parseQuestion(key string, raw json.RawMessage) (typesafe.Question, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, fmt.Errorf("invalid request: questions.%s: must be an object with a \"type\"", key)
	}
	if head.Type == "" {
		return nil, fmt.Errorf("invalid request: questions.%s.type: is required", key)
	}
	switch head.Type {
	case "noul":
		var w struct {
			Instructions any `json:"instructions"`
			Criteria     *struct {
				True  any `json:"true"`
				False any `json:"false"`
			} `json:"criteria"`
		}
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("invalid request: questions.%s: %w", key, err)
		}
		q := typesafe.Noul{Instructions: w.Instructions}
		if w.Criteria != nil {
			q.Criteria = &typesafe.NoulCriteria{True: w.Criteria.True, False: w.Criteria.False}
		}
		return q, nil
	case "choice":
		var w struct {
			Instructions any                         `json:"instructions"`
			Criteria     map[string]typesafe.JSONContent `json:"criteria"`
		}
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("invalid request: questions.%s: %w", key, err)
		}
		return typesafe.Choice{Instructions: w.Instructions, Criteria: w.Criteria}, nil
	case "score":
		var w struct {
			Instructions any                    `json:"instructions"`
			Criteria     []typesafe.JSONContent `json:"criteria"`
		}
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("invalid request: questions.%s: %w", key, err)
		}
		return typesafe.Score{Instructions: w.Instructions, Criteria: w.Criteria}, nil
	default:
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("invalid request: questions.%s: %w", key, err)
		}
		return typesafe.RawQuestion(m), nil
	}
}

// requestFromFlags builds a request from --state and repeated --noul,
// --choice, and --score flags.
func requestFromFlags(o *askOptions, stdin io.Reader) (*askRequest, error) {
	if o.state == "" {
		return nil, errors.New("--state is required when using --noul, --choice, or --score")
	}
	state, err := readStateFlag(o.state, stdin)
	if err != nil {
		return nil, err
	}
	req := &askRequest{State: state, Questions: typesafe.Questions{}, Extra: map[string]any{}}
	add := func(key string, q typesafe.Question) error {
		if _, dup := req.Questions[key]; dup {
			return fmt.Errorf("duplicate question key %q", key)
		}
		req.Questions[key] = q
		return nil
	}
	for _, s := range o.nouls {
		key, instr, err := splitKV(s, "--noul")
		if err != nil {
			return nil, err
		}
		if err := add(key, typesafe.Noul{Instructions: instr}); err != nil {
			return nil, err
		}
	}
	for _, s := range o.choices {
		key, rest, err := splitKV(s, "--choice")
		if err != nil {
			return nil, err
		}
		instr, labels, err := splitInstrLabels(rest, ",", "--choice")
		if err != nil {
			return nil, err
		}
		crit := make(map[string]typesafe.JSONContent, len(labels))
		for _, l := range labels {
			crit[l] = nil
		}
		if err := add(key, typesafe.Choice{Instructions: instr, Criteria: crit}); err != nil {
			return nil, err
		}
	}
	for _, s := range o.scores {
		key, rest, err := splitKV(s, "--score")
		if err != nil {
			return nil, err
		}
		instr, levels, err := splitInstrLabels(rest, "|", "--score")
		if err != nil {
			return nil, err
		}
		if len(levels) < 2 {
			return nil, fmt.Errorf("--score %q: needs at least two levels separated by |", s)
		}
		crit := make([]typesafe.JSONContent, len(levels))
		for i, l := range levels {
			crit[i] = l
		}
		if err := add(key, typesafe.Score{Instructions: instr, Criteria: crit}); err != nil {
			return nil, err
		}
	}
	return req, nil
}

func readStateFlag(v string, stdin io.Reader) (string, error) {
	switch {
	case v == "-":
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read state from stdin: %w", err)
		}
		return string(b), nil
	case strings.HasPrefix(v, "@"):
		b, err := os.ReadFile(v[1:])
		if err != nil {
			return "", fmt.Errorf("read state file: %w", err)
		}
		return string(b), nil
	default:
		return v, nil
	}
}

// splitKV splits "key=rest" on the first '='.
func splitKV(s, flag string) (string, string, error) {
	i := strings.Index(s, "=")
	if i <= 0 {
		return "", "", fmt.Errorf("%s %q: expected key=instructions", flag, s)
	}
	return strings.TrimSpace(s[:i]), s[i+1:], nil
}

// splitInstrLabels splits "instructions:l1<sep>l2" on the first ':' and
// then on sep, trimming each label and rejecting empties.
func splitInstrLabels(rest, sep, flag string) (string, []string, error) {
	i := strings.Index(rest, ":")
	if i < 0 {
		return "", nil, fmt.Errorf("%s %q: expected key=instructions:labels", flag, rest)
	}
	instr := strings.TrimSpace(rest[:i])
	var labels []string
	for _, l := range strings.Split(rest[i+1:], sep) {
		l = strings.TrimSpace(l)
		if l == "" {
			return "", nil, fmt.Errorf("%s %q: empty label", flag, rest)
		}
		labels = append(labels, l)
	}
	return instr, labels, nil
}
```

- [ ] **Step 4: Run tests to verify they pass, then commit**

```bash
go test -v -run 'TestParse|TestRequestFrom' ./internal/cli/ 2>&1 | tail -8
git add internal/cli/request.go internal/cli/request_test.go
git commit -m "Add CLI request parsing for JSON and flag modes"
```

---

## Task 15: Root command, globals, and `main`

**Agent:** `cli`

**Files:**
- Create: `internal/cli/root.go`, `cmd/jev/main.go`

- [ ] **Step 1: Write `internal/cli/root.go`**

```go
// Package cli implements the jev command.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/therealbill/typesafe-go"
)

// globals are the persistent flags shared by every subcommand.
type globals struct {
	apiKey     string
	baseURL    string
	model      string
	timeout    time.Duration
	maxRetries int
	logLevel   string
	trace      bool
	noTrace    bool
	pretty     bool
}

// NewRootCmd builds the command tree. getenv is used for telemetry decisions
// so tests can control the environment.
func NewRootCmd(streams IO, getenv func(string) string) *cobra.Command {
	g := &globals{}
	root := &cobra.Command{
		Use:           "jev",
		Short:         "Ask TypeSafe's Jev model typed questions from the command line",
		Long:          "jev sends a state and a set of typed questions (noul, choice, score) to the TypeSafe System One API and prints the answers as JSON.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	pf := root.PersistentFlags()
	pf.StringVar(&g.apiKey, "api-key", "", "TypeSafe API key (env TYPESAFE_API_KEY)")
	pf.StringVar(&g.baseURL, "base-url", "", "API base URL (env TYPESAFE_BASE_URL)")
	pf.StringVar(&g.model, "model", "", "model name (env TYPESAFE_DEFAULT_MODEL; default jev-latest)")
	pf.DurationVar(&g.timeout, "timeout", 0, "per-attempt HTTP timeout (default 10s)")
	pf.IntVar(&g.maxRetries, "max-retries", -1, "retries after the first attempt (default 2)")
	pf.StringVar(&g.logLevel, "log-level", "", "debug|info|warning|error|off (env TYPESAFE_LOG_LEVEL)")
	pf.BoolVar(&g.trace, "trace", false, "force OpenTelemetry tracing on")
	pf.BoolVar(&g.noTrace, "no-trace", false, "disable tracing even when HONEYCOMB_API_KEY or OTEL_* is set")
	pf.BoolVar(&g.pretty, "pretty", false, "indent JSON output")

	root.SetIn(streams.In)
	root.SetOut(streams.Out)
	root.SetErr(streams.Err)
	root.AddCommand(newAskCmd(g, streams, getenv), newModelsCmd(g, streams, getenv), newVersionCmd(streams))
	return root
}

// clientOptions turns the globals into client options.
func (g *globals) clientOptions(streams IO, inst typesafe.Instrumentation) []typesafe.Option {
	var opts []typesafe.Option
	if g.apiKey != "" {
		opts = append(opts, typesafe.WithAPIKey(g.apiKey))
	}
	if g.baseURL != "" {
		opts = append(opts, typesafe.WithBaseURL(g.baseURL))
	}
	if g.model != "" {
		opts = append(opts, typesafe.WithModel(g.model))
	}
	if g.timeout > 0 {
		opts = append(opts, typesafe.WithTimeout(g.timeout))
	}
	if g.maxRetries >= 0 {
		p := typesafe.DefaultRetryPolicy()
		p.MaxRetries = g.maxRetries
		opts = append(opts, typesafe.WithRetryPolicy(p))
	}
	if g.logLevel != "" {
		opts = append(opts, typesafe.WithLogger(typesafe.NewLogger(streams.Err, g.logLevel)))
	}
	if inst != nil {
		opts = append(opts, typesafe.WithInstrumentation(inst))
	}
	return opts
}

// Main runs the command and returns the process exit code.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	root := NewRootCmd(IO{In: stdin, Out: stdout, Err: stderr}, os.Getenv)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return ExitOK
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	fmt.Fprintf(stderr, "jev: %v\nRun 'jev --help' for usage.\n", err)
	return ExitUsage
}
```

- [ ] **Step 2: Write `cmd/jev/main.go`**

```go
// Command jev asks TypeSafe's Jev model typed questions from the command line.
package main

import (
	"os"

	"github.com/therealbill/typesafe-go/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
```

- [ ] **Step 3: Build check**

```bash
go build ./... 2>&1 | head
```
Expected: errors only for `newAskCmd`, `newModelsCmd`, `newVersionCmd` (Task 16). Do not commit yet.

---
## Task 16: `ask`, `models`, and `version` commands

**Agent:** `cli`

**Files:**
- Create: `internal/cli/ask.go`, `internal/cli/models.go`, `internal/cli/version.go`
- Create: `internal/cli/telemetry.go` (stub now, full in Task 18)

- [ ] **Step 1: Write the telemetry stub `internal/cli/telemetry.go`**

```go
package cli

import (
	"context"

	"github.com/therealbill/typesafe-go"
)

// setupTelemetry is completed in Task 18. This stub keeps tracing off.
func setupTelemetry(ctx context.Context, g *globals, streams IO, getenv func(string) string) (func(), typesafe.Instrumentation) {
	return func() {}, nil
}
```

- [ ] **Step 2: Write `internal/cli/ask.go`**

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/therealbill/typesafe-go"
)

func newAskCmd(g *globals, streams IO, getenv func(string) string) *cobra.Command {
	o := &askOptions{}
	cmd := &cobra.Command{
		Use:   "ask",
		Short: "Send a state and typed questions, print the answers as JSON",
		Long: `Send one System One request and print the response as JSON.

Two input modes:

  JSON   jev ask -f request.json        (or pipe the JSON to stdin)
         The document is the HTTP body shape:
         {"state": ..., "questions": {"id": {"type": "noul", ...}}, "model": "..."}

  Flags  jev ask --state "text" --noul billing="Is this about billing?" \
             --choice tone="What is the tone?:calm,angry" \
             --score urgency="How urgent?:low|medium|high"

Output: {"model": ..., "answers": {...}, "usage": {...}, "request_id": ...}`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAsk(cmd, g, o, streams, getenv)
		},
	}
	f := cmd.Flags()
	f.StringVarP(&o.file, "file", "f", "", "request JSON file; '-' or omitted reads stdin")
	f.StringVar(&o.state, "state", "", "state text, @path to read a file, or '-' for stdin")
	f.StringArrayVar(&o.nouls, "noul", nil, "key=instructions (repeatable)")
	f.StringArrayVar(&o.choices, "choice", nil, "key=instructions:label1,label2,... (repeatable)")
	f.StringArrayVar(&o.scores, "score", nil, "key=instructions:level0|level1|... (repeatable)")
	f.BoolVar(&o.raw, "raw", false, "print the server response body unchanged")
	return cmd
}

func runAsk(cmd *cobra.Command, g *globals, o *askOptions, streams IO, getenv func(string) string) error {
	req, err := buildRequest(o, streams.In)
	if err != nil {
		return fail(streams, g.pretty, &usageError{err})
	}
	ctx := cmd.Context()
	shutdown, inst := setupTelemetry(ctx, g, streams, getenv)
	defer shutdown()

	client, err := typesafe.NewClient(g.clientOptions(streams, inst)...)
	if err != nil {
		return fail(streams, g.pretty, err)
	}
	defer client.Close()

	var ropts []typesafe.RequestOption
	if req.Model != "" && !cmd.Flags().Changed("model") {
		ropts = append(ropts, typesafe.WithRequestModel(req.Model))
	}
	if len(req.Extra) > 0 {
		ropts = append(ropts, typesafe.WithExtraBody(req.Extra))
	}
	res, err := client.SystemOne(ctx, req.State, req.Questions, ropts...)
	if err != nil {
		return fail(streams, g.pretty, err)
	}
	if o.raw {
		if _, err := streams.Out.Write(res.Raw.Body); err != nil {
			return err
		}
		_, err = fmt.Fprintln(streams.Out)
		return err
	}
	return writeJSON(streams.Out, res, g.pretty)
}
```

- [ ] **Step 3: Write `internal/cli/models.go`**

```go
package cli

import (
	"github.com/spf13/cobra"

	"github.com/therealbill/typesafe-go"
)

func newModelsCmd(g *globals, streams IO, getenv func(string) string) *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List the models available to the account as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			shutdown, inst := setupTelemetry(ctx, g, streams, getenv)
			defer shutdown()
			client, err := typesafe.NewClient(g.clientOptions(streams, inst)...)
			if err != nil {
				return fail(streams, g.pretty, err)
			}
			defer client.Close()
			res, err := client.ListModels(ctx)
			if err != nil {
				return fail(streams, g.pretty, err)
			}
			return writeJSON(streams.Out, res, g.pretty)
		},
	}
}
```

- [ ] **Step 4: Write `internal/cli/version.go`**

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
			return writeJSON(streams.Out, map[string]string{
				"version": version.Version,
				"commit":  version.Commit,
				"go":      runtime.Version(),
			}, pretty)
		},
	}
}
```

- [ ] **Step 5: Build, smoke test, commit**

```bash
go build ./... && go run ./cmd/jev version && go run ./cmd/jev ask --help | head -5
git add internal/cli/ask.go internal/cli/models.go internal/cli/version.go internal/cli/telemetry.go internal/cli/root.go cmd/jev/main.go
git commit -m "Add jev root, ask, models, and version commands"
```
Expected: version JSON on one line, then the ask help text.

---

## Task 17: In-process CLI tests

**Agent:** `cli`

**Files:**
- Create: `internal/cli/cli_test.go`

- [ ] **Step 1: Write the tests**

```go
package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const okBody = `{"model":"jev-1.13.0","answers":{"billing":{"type":"noul","noul":0.99},"tone":{"type":"choice","choice":"angry","confidence":1.0,"probabilities":{"neutral":0.0,"angry":1.0,"calm":0.0}},"urgency":{"type":"score","score":1.9,"confidence":0.85,"legend":{"0":"Not urgent at all","1":"Somewhat urgent","2":"Very urgent"},"probabilities":{"0":0.0,"1":0.1,"2":0.9}}},"usage":{"input_tokens":407,"output_tokens":72}}`

const modelsBody = `{"models":[{"name":"jev-latest","description":"The latest iteration of TypeSafe's System One Model: Jev","release_date":"2026-09-10T18:38:01.391457+00:00"}]}`

const requestJSON = `{"model":"jev-latest","state":{"ticket":"I was charged twice this month and nobody answers my emails. Fix it now."},"questions":{"billing":{"type":"noul","instructions":"Is ` + "`ticket`" + ` about a billing problem?"},"tone":{"type":"choice","instructions":"What is the tone of ` + "`ticket`" + `?","criteria":{"calm":"Polite and patient","angry":"Frustrated or hostile","neutral":null}},"urgency":{"type":"score","instructions":"How urgent is ` + "`ticket`" + `?","criteria":["Not urgent at all","Somewhat urgent","Very urgent"]}}}`

type recorded struct {
	mu     sync.Mutex
	calls  int
	bodies [][]byte
	paths  []string
}

func serve(t *testing.T, status int, body string, headers map[string]string) (*httptest.Server, *recorded) {
	t.Helper()
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rec.mu.Lock()
		rec.calls++
		rec.bodies = append(rec.bodies, b)
		rec.paths = append(rec.paths, r.URL.Path)
		rec.mu.Unlock()
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func run(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	t.Setenv("TYPESAFE_API_KEY", "")
	t.Setenv("HONEYCOMB_API_KEY", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_CONFIG_FILE", "")
	var out, errOut bytes.Buffer
	code = Main(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func baseArgs(url string) []string {
	return []string{"--api-key", "k", "--base-url", url, "--max-retries", "0"}
}

func TestAskJSONMode(t *testing.T) {
	srv, rec := serve(t, 200, okBody, map[string]string{"x-typesafe-request-id": "req_cli"})
	code, out, errOut := run(t, requestJSON, append([]string{"ask"}, baseArgs(srv.URL)...)...)
	if code != ExitOK {
		t.Fatalf("exit %d stderr %s", code, errOut)
	}
	var res struct {
		Model     string                            `json:"model"`
		Answers   map[string]map[string]any         `json:"answers"`
		Usage     map[string]int                    `json:"usage"`
		RequestID string                            `json:"request_id"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("stdout not JSON: %s", out)
	}
	if res.Model != "jev-1.13.0" || res.RequestID != "req_cli" || res.Answers["tone"]["choice"] != "angry" || res.Answers["tone"]["type"] != "choice" || res.Usage["input_tokens"] != 407 {
		t.Fatalf("response %+v", res)
	}
	var sent, want map[string]any
	_ = json.Unmarshal(rec.bodies[0], &sent)
	_ = json.Unmarshal([]byte(requestJSON), &want)
	sb, _ := json.Marshal(sent)
	wb, _ := json.Marshal(want)
	if !bytes.Equal(sb, wb) {
		t.Fatalf("sent body\n %s\nwant %s", sb, wb)
	}
}

func TestAskFileAndModelPrecedence(t *testing.T) {
	srv, rec := serve(t, 200, okBody, nil)
	path := filepath.Join(t.TempDir(), "req.json")
	if err := os.WriteFile(path, []byte(requestJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := run(t, "", append([]string{"ask", "-f", path, "--model", "jev-preview"}, baseArgs(srv.URL)...)...)
	if code != ExitOK {
		t.Fatalf("exit %d stderr %s", code, errOut)
	}
	var sent map[string]any
	_ = json.Unmarshal(rec.bodies[0], &sent)
	if sent["model"] != "jev-preview" {
		t.Fatalf("--model flag must override the document model, got %v", sent["model"])
	}
}

func TestAskFlagMode(t *testing.T) {
	srv, rec := serve(t, 200, okBody, nil)
	args := append([]string{"ask",
		"--state", "I was charged twice",
		"--noul", "billing=Is this about billing?",
		"--choice", "tone=What is the tone?:calm,angry",
		"--score", "urgency=How urgent?:low|high",
	}, baseArgs(srv.URL)...)
	code, _, errOut := run(t, "", args...)
	if code != ExitOK {
		t.Fatalf("exit %d stderr %s", code, errOut)
	}
	var sent map[string]any
	_ = json.Unmarshal(rec.bodies[0], &sent)
	qs := sent["questions"].(map[string]any)
	if sent["state"] != "I was charged twice" || qs["billing"].(map[string]any)["type"] != "noul" {
		t.Fatalf("sent %v", sent)
	}
	crit := qs["tone"].(map[string]any)["criteria"].(map[string]any)
	if _, ok := crit["angry"]; !ok || crit["angry"] != nil {
		t.Fatalf("choice criteria %v", crit)
	}
	levels := qs["urgency"].(map[string]any)["criteria"].([]any)
	if len(levels) != 2 || levels[1] != "high" {
		t.Fatalf("score criteria %v", levels)
	}
}

func TestAskRawAndPretty(t *testing.T) {
	srv, _ := serve(t, 200, okBody, nil)
	code, out, _ := run(t, requestJSON, append([]string{"ask", "--raw"}, baseArgs(srv.URL)...)...)
	if code != ExitOK || out != okBody+"\n" {
		t.Fatalf("raw output mismatch:\n%s", out)
	}
	code, out, _ = run(t, requestJSON, append([]string{"ask", "--pretty"}, baseArgs(srv.URL)...)...)
	if code != ExitOK || !strings.Contains(out, "\n  \"answers\"") {
		t.Fatalf("pretty output not indented:\n%s", out)
	}
}

func TestAskExitCodes(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		headers map[string]string
		stdin   string
		extra   []string
		code    int
		kind    string
	}{
		{"auth", 401, `{"detail":{"error_type":"authentication_error","message":"no"}}`, nil, requestJSON, nil, ExitAuth, "auth"},
		{"forbidden", 403, `{"detail":"no"}`, nil, requestJSON, nil, ExitAuth, "auth"},
		{"bad request", 400, `{"detail":{"error_type":"api_usage_error","message":"Invalid request."}}`, nil, requestJSON, nil, ExitRequest, "request"},
		{"unprocessable", 422, `{"detail":[{"loc":["body","questions"],"msg":"too short"}]}`, nil, requestJSON, nil, ExitRequest, "request"},
		{"rate limit", 429, `{"detail":"slow"}`, map[string]string{"Retry-After": "0"}, requestJSON, nil, ExitRateLimit, "rate_limit"},
		{"server", 503, `down`, nil, requestJSON, nil, ExitServer, "server"},
		{"overloaded", 529, `overloaded`, nil, requestJSON, nil, ExitServer, "server"},
		{"invalid response", 200, `{"answers":{"a":{"type":"noul"}}}`, nil, requestJSON, nil, ExitServer, "invalid_response"},
		{"validation", 200, okBody, nil, `{"state":"x","questions":{"q":{"type":"choice","instructions":"?","criteria":{}}}}`, nil, ExitValidation, "validation"},
		{"bad json", 200, okBody, nil, `{not json`, nil, ExitUsage, "usage"},
		{"empty stdin", 200, okBody, nil, ``, nil, ExitUsage, "usage"},
		{"file with flags", 200, okBody, nil, ``, []string{"-f", "x.json", "--noul", "a=b", "--state", "s"}, ExitUsage, "usage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := serve(t, tt.status, tt.body, tt.headers)
			args := append([]string{"ask"}, baseArgs(srv.URL)...)
			args = append(args, tt.extra...)
			code, out, errOut := run(t, tt.stdin, args...)
			if code != tt.code {
				t.Fatalf("exit %d want %d; stdout %s stderr %s", code, tt.code, out, errOut)
			}
			var payload struct {
				Error struct {
					Kind   string `json:"kind"`
					Status int    `json:"status"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(out), &payload); err != nil {
				t.Fatalf("stdout not error JSON: %s", out)
			}
			if payload.Error.Kind != tt.kind {
				t.Fatalf("kind %q want %q", payload.Error.Kind, tt.kind)
			}
			if tt.status >= 400 && payload.Error.Status != tt.status {
				t.Fatalf("status %d want %d", payload.Error.Status, tt.status)
			}
			if !strings.HasPrefix(errOut, "jev: ") {
				t.Fatalf("stderr %q", errOut)
			}
		})
	}
}

func TestAskConnectionError(t *testing.T) {
	srv, _ := serve(t, 200, okBody, nil)
	url := srv.URL
	srv.Close()
	code, out, _ := run(t, requestJSON, append([]string{"ask"}, baseArgs(url)...)...)
	if code != ExitConnection || !strings.Contains(out, `"kind":"connection"`) {
		t.Fatalf("exit %d out %s", code, out)
	}
}

func TestAskMissingAPIKey(t *testing.T) {
	srv, _ := serve(t, 200, okBody, nil)
	code, out, _ := run(t, requestJSON, "ask", "--base-url", srv.URL)
	if code != ExitUsage || !strings.Contains(out, `"kind":"usage"`) {
		t.Fatalf("exit %d out %s", code, out)
	}
}

func TestModels(t *testing.T) {
	srv, rec := serve(t, 200, modelsBody, nil)
	code, out, errOut := run(t, "", append([]string{"models"}, baseArgs(srv.URL)...)...)
	if code != ExitOK {
		t.Fatalf("exit %d stderr %s", code, errOut)
	}
	if rec.paths[0] != "/v1/models" {
		t.Fatalf("path %s", rec.paths[0])
	}
	var res struct {
		Models []map[string]string `json:"models"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil || len(res.Models) != 1 || res.Models[0]["name"] != "jev-latest" {
		t.Fatalf("out %s err %v", out, err)
	}
}

func TestVersion(t *testing.T) {
	code, out, _ := run(t, "", "version")
	if code != ExitOK || !strings.Contains(out, `"version"`) || !strings.Contains(out, `"go"`) {
		t.Fatalf("exit %d out %s", code, out)
	}
}

func TestUnknownCommandIsUsage(t *testing.T) {
	code, _, errOut := run(t, "", "bogus")
	if code != ExitUsage || !strings.Contains(errOut, "jev --help") {
		t.Fatalf("exit %d stderr %s", code, errOut)
	}
}
```

- [ ] **Step 2: Run and commit**

```bash
go test -race -v ./internal/cli/ 2>&1 | tail -20
git add internal/cli/cli_test.go
git commit -m "Add in-process jev CLI tests"
```
Expected: all PASS. If any exit-code row fails, fix `classify` or the command, not the test.

---
## Task 18: Telemetry wiring through `otelconf`

**Agent:** `cli` (after Task 10 is committed)

**Files:**
- Replace: `internal/cli/telemetry.go`
- Create: `internal/cli/telemetry_test.go`
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the dependency**

```bash
go get go.opentelemetry.io/contrib/otelconf@v0.26.0
go mod tidy
```

- [ ] **Step 2: Write the failing tests `internal/cli/telemetry_test.go`**

```go
package cli

import (
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestTelemetryEnabled(t *testing.T) {
	tests := []struct {
		name string
		g    globals
		env  map[string]string
		want bool
	}{
		{"nothing set", globals{}, nil, false},
		{"honeycomb key", globals{}, map[string]string{"HONEYCOMB_API_KEY": "hc"}, true},
		{"otlp endpoint", globals{}, map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://x"}, true},
		{"config file", globals{}, map[string]string{"OTEL_CONFIG_FILE": "/x.yaml"}, true},
		{"--trace forces on", globals{trace: true}, nil, true},
		{"--no-trace wins", globals{noTrace: true}, map[string]string{"HONEYCOMB_API_KEY": "hc"}, false},
		{"--no-trace beats --trace", globals{trace: true, noTrace: true}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := telemetryEnabled(&tt.g, env(tt.env)); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestBuildTelemetryConfig(t *testing.T) {
	cfg := buildTelemetryConfig("hc-key", "", "")
	exp := cfg.TracerProvider.Processors[0].Batch.Exporter.OTLPHttp
	if *exp.Endpoint != "https://api.honeycomb.io/v1/traces" {
		t.Fatalf("endpoint %s", *exp.Endpoint)
	}
	if len(exp.Headers) != 1 || exp.Headers[0].Name != "x-honeycomb-team" || *exp.Headers[0].Value != "hc-key" {
		t.Fatalf("headers %+v", exp.Headers)
	}
	if cfg.Resource.Attributes[0].Name != "service.name" || cfg.Resource.Attributes[0].Value != "jev" {
		t.Fatalf("resource %+v", cfg.Resource.Attributes)
	}

	cfg = buildTelemetryConfig("", "http://collector:4318", "myapp")
	exp = cfg.TracerProvider.Processors[0].Batch.Exporter.OTLPHttp
	if *exp.Endpoint != "http://collector:4318/v1/traces" || len(exp.Headers) != 0 {
		t.Fatalf("custom endpoint %s headers %v", *exp.Endpoint, exp.Headers)
	}
	if cfg.Resource.Attributes[0].Value != "myapp" {
		t.Fatalf("service name %v", cfg.Resource.Attributes[0].Value)
	}

	cfg = buildTelemetryConfig("k", "https://api.eu1.honeycomb.io/v1/traces/", "")
	if *cfg.TracerProvider.Processors[0].Batch.Exporter.OTLPHttp.Endpoint != "https://api.eu1.honeycomb.io/v1/traces" {
		t.Fatal("trailing slash and existing path must be preserved without duplication")
	}
}

func TestTelemetryConfigFromFileExpandsEnv(t *testing.T) {
	path := t.TempDir() + "/otel.yaml"
	yaml := "file_format: \"1.0\"\ntracer_provider:\n  processors:\n    - batch:\n        exporter:\n          otlp_http:\n            endpoint: https://api.honeycomb.io/v1/traces\n            headers:\n              - name: x-honeycomb-team\n                value: ${HONEYCOMB_API_KEY}\n"
	if err := writeFile(path, yaml); err != nil {
		t.Fatal(err)
	}
	cfg, err := telemetryConfig(env(map[string]string{"OTEL_CONFIG_FILE": path, "HONEYCOMB_API_KEY": "from-env"}))
	if err != nil {
		t.Fatal(err)
	}
	h := cfg.TracerProvider.Processors[0].Batch.Exporter.OTLPHttp.Headers
	if len(h) != 1 || *h[0].Value != "from-env" {
		t.Fatalf("headers %+v", h)
	}
}
```

Add this helper at the bottom of the test file:

```go
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
```
and add `"os"` to the imports.

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/cli/ 2>&1 | head -3
```
Expected: undefined `telemetryEnabled`, `buildTelemetryConfig`, `telemetryConfig`.

- [ ] **Step 4: Replace `internal/cli/telemetry.go`**

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	otelconf "go.opentelemetry.io/contrib/otelconf/x"
	otelapi "go.opentelemetry.io/otel"

	"github.com/therealbill/typesafe-go"
	tsotel "github.com/therealbill/typesafe-go/otel"
)

// telemetryEnabled decides whether to start the OpenTelemetry SDK.
// --no-trace always wins. Otherwise --trace, HONEYCOMB_API_KEY,
// OTEL_EXPORTER_OTLP_ENDPOINT, or OTEL_CONFIG_FILE turns tracing on.
func telemetryEnabled(g *globals, getenv func(string) string) bool {
	if g.noTrace {
		return false
	}
	return g.trace || getenv("HONEYCOMB_API_KEY") != "" || getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" || getenv("OTEL_CONFIG_FILE") != ""
}

// setupTelemetry starts the SDK when enabled and returns a shutdown function
// plus the client instrumentation to attach. When disabled or on failure it
// returns a no-op shutdown and nil instrumentation; failures are reported on
// stderr and never stop the command.
func setupTelemetry(ctx context.Context, g *globals, streams IO, getenv func(string) string) (func(), typesafe.Instrumentation) {
	noop := func() {}
	if !telemetryEnabled(g, getenv) {
		return noop, nil
	}
	cfg, err := telemetryConfig(getenv)
	if err != nil {
		fmt.Fprintln(streams.Err, "jev: tracing disabled:", err)
		return noop, nil
	}
	sdk, err := otelconf.NewSDK(otelconf.WithContext(ctx), otelconf.WithOpenTelemetryConfiguration(cfg))
	if err != nil {
		fmt.Fprintln(streams.Err, "jev: tracing disabled:", err)
		return noop, nil
	}
	otelapi.SetTracerProvider(sdk.TracerProvider())
	if p := sdk.Propagator(); p != nil {
		otelapi.SetTextMapPropagator(p)
	}
	shutdown := func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := sdk.Shutdown(c); err != nil {
			fmt.Fprintln(streams.Err, "jev: tracing shutdown:", err)
		}
	}
	return shutdown, tsotel.New()
}

// telemetryConfig loads OTEL_CONFIG_FILE (with ${VAR} expansion from the
// environment) or builds a configuration from HONEYCOMB_API_KEY,
// OTEL_EXPORTER_OTLP_ENDPOINT, and OTEL_SERVICE_NAME.
func telemetryConfig(getenv func(string) string) (otelconf.OpenTelemetryConfiguration, error) {
	if file := getenv("OTEL_CONFIG_FILE"); file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return otelconf.OpenTelemetryConfiguration{}, fmt.Errorf("read OTEL_CONFIG_FILE: %w", err)
		}
		expanded := os.Expand(string(b), getenv)
		cfg, err := otelconf.ParseYAML([]byte(expanded))
		if err != nil {
			return otelconf.OpenTelemetryConfiguration{}, fmt.Errorf("parse OTEL_CONFIG_FILE: %w", err)
		}
		return *cfg, nil
	}
	return buildTelemetryConfig(getenv("HONEYCOMB_API_KEY"), getenv("OTEL_EXPORTER_OTLP_ENDPOINT"), getenv("OTEL_SERVICE_NAME")), nil
}

// buildTelemetryConfig returns a tracer-only configuration exporting over
// OTLP/HTTP. With no endpoint it targets Honeycomb's US region.
func buildTelemetryConfig(honeycombKey, endpoint, serviceName string) otelconf.OpenTelemetryConfiguration {
	if serviceName == "" {
		serviceName = "jev"
	}
	if endpoint == "" {
		endpoint = "https://api.honeycomb.io"
	}
	endpoint = strings.TrimRight(endpoint, "/")
	if !strings.HasSuffix(endpoint, "/v1/traces") {
		endpoint += "/v1/traces"
	}
	exp := otelconf.OTLPHttpExporter{Endpoint: &endpoint}
	if honeycombKey != "" {
		key := honeycombKey
		exp.Headers = []otelconf.NameStringValuePair{{Name: "x-honeycomb-team", Value: &key}}
	}
	return otelconf.OpenTelemetryConfiguration{
		FileFormat: "1.0",
		Resource: &otelconf.Resource{
			Attributes: []otelconf.AttributeNameValue{{Name: "service.name", Value: serviceName}},
		},
		TracerProvider: &otelconf.TracerProvider{
			Processors: []otelconf.SpanProcessor{{
				Batch: &otelconf.BatchSpanProcessor{Exporter: otelconf.SpanExporter{OTLPHttp: &exp}},
			}},
		},
	}
}
```

If the compiler reports that `otelconf.NewSDK` or a struct name differs, check the installed source with `go doc go.opentelemetry.io/contrib/otelconf/x` and adapt the names; the field paths verified on 2026-09-23 are `OpenTelemetryConfiguration.TracerProvider.Processors[].Batch.Exporter.OTLPHttp.{Endpoint *string, Headers []NameStringValuePair{Name string, Value *string}}` and `Resource.Attributes []AttributeNameValue{Name string, Value any}`.

- [ ] **Step 5: Run tests, live smoke, commit**

```bash
go test -race -v -run 'TestTelemetry|TestBuildTelemetry' ./internal/cli/ 2>&1 | tail -8
go build -o bin/jev ./cmd/jev && echo '{"state":"hello","questions":{"g":{"type":"noul","instructions":"Is `state` a greeting?"}}}' | HONEYCOMB_API_KEY="${HONEYCOMB_API_KEY:-}" ./bin/jev ask --trace 2>&1 | tail -3
git add go.mod go.sum internal/cli/telemetry.go internal/cli/telemetry_test.go
git commit -m "Wire jev tracing through otelconf with Honeycomb defaults"
```
Expected: tests PASS. The smoke run prints the answer JSON; if `HONEYCOMB_API_KEY` is unset the exporter fails to send but the command still succeeds, which is the intended behavior (tracing must never break a call). Report whether a span export error appeared on stderr.

---

## Task 19: CLI build and lint gate

**Agent:** `cli`

- [ ] **Step 1: Full verification**

```bash
make lint && make test && make build && ./bin/jev version && ./bin/jev models --pretty | head -8
```
Expected: lint clean, all tests pass, `bin/jev` runs, models list prints (needs `TYPESAFE_API_KEY`, which is set). Fix lint findings in `internal/cli` and `cmd/jev`; report findings in other packages to the lead.

- [ ] **Step 2: Commit any lint fixes**

```bash
git add internal/cli cmd/jev
git commit -m "Address lint findings in CLI"
```
Skip the commit if there were no changes. Report "Task 19 done, CLI ready" to the lead; this unblocks Task 20.

---
## Task 20: Self-review tool driven by `jev`

**Agent:** `core` (after Task 19 reports the CLI ready)

**Files:**
- Create: `tools/selfreview/main.go`, `tools/selfreview/main_test.go`, `tools/selfreview/units.json`

The tool reads `units.json`, extracts each unit's spec section, bundles it with the implementation and test sources as state, asks Jev one request per unit through `jev ask` (JSON on stdin), evaluates thresholds, writes `selfreview-report.json`, prints a Markdown summary, and exits 1 when any unit is flagged and not accepted.

- [ ] **Step 1: Write `tools/selfreview/units.json`**

```json
{
  "spec": "docs/superpowers/specs/2026-09-23-typesafe-go-sdk-design.md",
  "units": [
    {
      "name": "questions",
      "spec_heading": "### Questions",
      "implementation": ["question.go"],
      "tests": ["question_test.go"],
      "behaviors": [
        "Noul.Criteria is omitted from the wire JSON when nil",
        "A nil Choice label value encodes as JSON null",
        "Choice with zero labels fails validation with path questions.<key>.criteria",
        "Choice with more than 255 labels fails validation",
        "Score with fewer than 2 or more than 10 levels fails validation",
        "An empty questions map fails validation with path questions",
        "RawQuestion without a string type field fails validation",
        "Instructions that are a number fail validation naming the instructions path",
        "Pointers to Noul, Choice, and Score are accepted as questions"
      ]
    },
    {
      "name": "responses",
      "spec_heading": "### Responses",
      "implementation": ["response.go", "models.go"],
      "tests": ["response_test.go"],
      "behaviors": [
        "An answer with an unrecognized type decodes to UnknownAnswer instead of failing",
        "A choice answer with a non-numeric confidence yields ResponseValidationError with path answers.<key>.confidence",
        "Nouls, Choices, and Scores partition the answers by type",
        "Marshaling an answer includes its wire type field",
        "A NoulAnswer refuses to unmarshal from a choice payload",
        "The models list decodes name, description, and release_date"
      ]
    },
    {
      "name": "errors",
      "spec_heading": "### Errors",
      "implementation": ["errors.go"],
      "tests": ["errors_test.go"],
      "behaviors": [
        "APIError.Message extracts text from a detail string, a detail object, and a detail array",
        "RateLimitError unwraps to APIError via errors.As",
        "TimeoutError unwraps to ConnectionError via errors.As",
        "IsAuthError is true for 401 and 403 only",
        "IsRetryable is true for 408, 429, 5xx, connection errors, and timeouts, and false for 4xx and validation errors"
      ]
    },
    {
      "name": "retry",
      "spec_heading": "### Retry policy",
      "implementation": ["retry.go"],
      "tests": ["retry_test.go"],
      "behaviors": [
        "Default policy is 2 retries, 500ms initial delay, 5s max delay, 0.25 jitter, 30s budget",
        "Delay doubles each retry and is capped at MaxDelay",
        "Jitter subtracts up to the jitter fraction of the delay",
        "retry-after-ms takes precedence over Retry-After",
        "Retry-After accepts seconds and HTTP dates, and a past date yields zero",
        "HonorRetryAfter=false ignores the headers",
        "ShouldRetry overrides all other decisions",
        "RetryOnConnErr=false and RetryOnTimeout=false are respected"
      ]
    },
    {
      "name": "client",
      "spec_heading": "### Client",
      "implementation": ["client.go", "transport.go"],
      "tests": ["client_test.go"],
      "behaviors": [
        "A 429 followed by a 200 is retried and the retry-after-ms wait is honored",
        "A 529 followed by a 200 is retried",
        "Retries stop after MaxRetries and the final APIError is returned",
        "A 422 is not retried",
        "The request body is identical across retry attempts",
        "The retry budget prevents a retry whose delay would exceed it",
        "Cancelling the context during backoff returns promptly with context.Canceled",
        "A per-attempt timeout produces TimeoutError",
        "Validation errors are returned before any network request",
        "WithExtraHeaders and WithExtraBody reach the wire and WithRequestModel overrides the model",
        "NewClient fails with ErrMissingAPIKey when no key is configured",
        "Environment variables supply base URL and model when options are absent",
        "WithHTTPClient does not mutate the caller's http.Client",
        "The instrumentation hook receives the request info and result including attempt count",
        "The Authorization header and request body are never logged",
        "The client is safe for concurrent use"
      ]
    },
    {
      "name": "typed",
      "spec_heading": "### Responses",
      "implementation": ["typed.go"],
      "tests": ["typed_test.go"],
      "behaviors": [
        "SystemOneAs decodes answers into a caller struct with json tags",
        "SystemOneAs returns the full response alongside the typed value",
        "SystemOneAs returns ResponseValidationError when a field's answer type does not match",
        "SystemOneAs returns a nil response on request errors"
      ]
    },
    {
      "name": "otel",
      "spec_heading": "## Subpackage `typesafe/otel`",
      "implementation": ["otel/otel.go"],
      "tests": ["otel/otel_test.go"],
      "behaviors": [
        "The operation span is named typesafe.system_one or typesafe.list_models with client kind",
        "gen_ai.request.model, gen_ai.response.model, and token usage attributes are set",
        "State and questions are not recorded unless WithRecordContent is set",
        "Recorded content is truncated to the configured byte limit",
        "Per-answer attributes appear only with WithRecordAnswers",
        "On error the span status is Error and error.type is set",
        "The HTTP span is a child of the operation span and the operation span is a child of the caller's span"
      ]
    },
    {
      "name": "cli",
      "spec_heading": "## CLI: `cmd/jev`",
      "implementation": ["internal/cli/root.go", "internal/cli/ask.go", "internal/cli/request.go", "internal/cli/exit.go", "internal/cli/models.go", "internal/cli/telemetry.go"],
      "tests": ["internal/cli/cli_test.go", "internal/cli/request_test.go", "internal/cli/exit_test.go", "internal/cli/telemetry_test.go"],
      "behaviors": [
        "JSON on stdin in the HTTP body shape is sent unchanged apart from client-side normalization",
        "The --model flag overrides a model in the JSON document",
        "Flag mode builds noul, choice, and score questions from --noul, --choice, and --score",
        "--file combined with question flags is a usage error with exit code 1",
        "Exit code 2 for client-side validation failures",
        "Exit code 3 for 401 and 403",
        "Exit code 4 for 400, 404, and 422",
        "Exit code 5 for 429 after retries",
        "Exit code 6 for 5xx after retries",
        "Exit code 7 for connection failures",
        "On error stdout carries a JSON object with kind, status, request_id, and message",
        "--raw prints the server body unchanged",
        "--no-trace disables tracing even when HONEYCOMB_API_KEY is set",
        "Without an endpoint the tracing config targets https://api.honeycomb.io/v1/traces with the x-honeycomb-team header"
      ]
    }
  ]
}
```

- [ ] **Step 2: Write the failing tests `tools/selfreview/main_test.go`**

```go
package main

import (
	"strings"
	"testing"
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

func TestBuildQuestions(t *testing.T) {
	u := unit{Name: "x", Behaviors: []string{"b0", "b1"}}
	qs := buildQuestions(u)
	for _, k := range []string{"covers_00", "covers_01", "contradicts_spec", "thoroughness", "weakest_area"} {
		if _, ok := qs[k]; !ok {
			t.Fatalf("missing question %s", k)
		}
	}
	if qs["thoroughness"].(map[string]any)["type"] != "score" || qs["weakest_area"].(map[string]any)["type"] != "choice" {
		t.Fatal("wrong question types")
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

func TestEvaluate(t *testing.T) {
	u := unit{Name: "x", Behaviors: []string{"b0", "b1"}, Accepted: []string{"covers_01"}}
	ans := map[string]map[string]any{
		"covers_00":        {"type": "noul", "noul": 0.9},
		"covers_01":        {"type": "noul", "noul": 0.2},
		"contradicts_spec": {"type": "noul", "noul": 0.1},
		"thoroughness":     {"type": "score", "score": 2.4, "confidence": 0.8},
		"weakest_area":     {"type": "choice", "choice": "retry", "confidence": 0.6},
	}
	r := evaluate(u, ans, thresholds{minCover: 0.6, maxContradict: 0.4, minThorough: 2.0})
	if len(r.Flags) != 1 || !strings.Contains(r.Flags[0], "covers_01") || !strings.Contains(r.Flags[0], "accepted") {
		t.Fatalf("flags %v", r.Flags)
	}
	if r.Failing {
		t.Fatal("an accepted flag must not fail the unit")
	}
	ans["contradicts_spec"]["noul"] = 0.7
	r = evaluate(u, ans, thresholds{minCover: 0.6, maxContradict: 0.4, minThorough: 2.0})
	if !r.Failing || len(r.Flags) != 2 {
		t.Fatalf("contradiction should fail: %+v", r)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./tools/selfreview/ 2>&1 | head -3
```
Expected: undefined `extractSection`, `unit`, and friends.

- [ ] **Step 4: Write `tools/selfreview/main.go`**

```go
// Command selfreview asks Jev, through the jev CLI, whether each unit's tests
// cover the behaviors its spec section requires and whether the
// implementation contradicts the spec. It writes selfreview-report.json,
// prints a Markdown summary, and exits 1 when any unit is flagged and not
// accepted in units.json.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type unit struct {
	Name           string   `json:"name"`
	SpecHeading    string   `json:"spec_heading"`
	Implementation []string `json:"implementation"`
	Tests          []string `json:"tests"`
	Behaviors      []string `json:"behaviors"`
	// Accepted lists question ids whose flags are acknowledged and do not
	// fail the run, for example "covers_03".
	Accepted []string `json:"accepted,omitempty"`
}

type config struct {
	Spec  string `json:"spec"`
	Units []unit `json:"units"`
}

type thresholds struct {
	minCover      float64
	maxContradict float64
	minThorough   float64
}

type behaviorResult struct {
	ID       string  `json:"id"`
	Behavior string  `json:"behavior"`
	Covered  float64 `json:"covered"`
}

type unitReport struct {
	Name         string           `json:"name"`
	RequestID    string           `json:"request_id,omitempty"`
	Model        string           `json:"model,omitempty"`
	InputTokens  int              `json:"input_tokens,omitempty"`
	Truncated    bool             `json:"truncated"`
	Behaviors    []behaviorResult `json:"behaviors"`
	Contradicts  float64          `json:"contradicts_spec"`
	Thoroughness float64          `json:"thoroughness"`
	ThoroughConf float64          `json:"thoroughness_confidence"`
	Weakest      string           `json:"weakest_area"`
	WeakestConf  float64          `json:"weakest_confidence"`
	Flags        []string         `json:"flags"`
	Failing      bool             `json:"failing"`
	Error        string           `json:"error,omitempty"`
}

const truncatedMarker = "\n// ...(truncated)\n"

func main() {
	unitsPath := flag.String("units", "tools/selfreview/units.json", "units file")
	jevPath := flag.String("jev", "./bin/jev", "path to the jev binary")
	reportPath := flag.String("report", "selfreview-report.json", "where to write the JSON report")
	only := flag.String("only", "", "run a single unit by name")
	maxBytes := flag.Int("max-state-bytes", 100000, "byte cap for implementation and tests together")
	minCover := flag.Float64("min-cover", 0.6, "flag a behavior whose coverage probability is below this")
	maxContradict := flag.Float64("max-contradict", 0.4, "flag a unit whose contradiction probability is above this")
	minThorough := flag.Float64("min-thorough", 2.0, "flag a unit whose thoroughness score is below this")
	flag.Parse()

	if os.Getenv("TYPESAFE_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "selfreview: TYPESAFE_API_KEY is not set")
		os.Exit(2)
	}
	cfgBytes, err := os.ReadFile(*unitsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "selfreview:", err)
		os.Exit(2)
	}
	var cfg config
	if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
		fmt.Fprintln(os.Stderr, "selfreview: units.json:", err)
		os.Exit(2)
	}
	specBytes, err := os.ReadFile(cfg.Spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, "selfreview:", err)
		os.Exit(2)
	}
	th := thresholds{minCover: *minCover, maxContradict: *maxContradict, minThorough: *minThorough}

	var reports []unitReport
	failing := false
	for _, u := range cfg.Units {
		if *only != "" && u.Name != *only {
			continue
		}
		r := runUnit(u, string(specBytes), *jevPath, *maxBytes, th)
		if r.Failing {
			failing = true
		}
		reports = append(reports, r)
	}
	out, _ := json.MarshalIndent(reports, "", "  ")
	if err := os.WriteFile(*reportPath, out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "selfreview: write report:", err)
	}
	fmt.Print(markdown(reports))
	if failing {
		os.Exit(1)
	}
}

func runUnit(u unit, spec, jevPath string, maxBytes int, th thresholds) unitReport {
	r := unitReport{Name: u.Name}
	section, err := extractSection(spec, u.SpecHeading)
	if err != nil {
		r.Error = err.Error()
		r.Failing = true
		return r
	}
	files := map[string]string{}
	for _, p := range append(append([]string{}, u.Implementation...), u.Tests...) {
		b, err := os.ReadFile(p)
		if err != nil {
			r.Error = err.Error()
			r.Failing = true
			return r
		}
		files[p] = string(b)
	}
	half := maxBytes / 2
	impl, t1 := bundle(u.Implementation, files, half)
	tests, t2 := bundle(u.Tests, files, half)
	r.Truncated = t1 || t2

	req := map[string]any{
		"state":     map[string]any{"spec": section, "implementation": impl, "tests": tests},
		"questions": buildQuestions(u),
	}
	payload, _ := json.Marshal(req)
	cmd := exec.Command(jevPath, "ask")
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		code := -1
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		}
		r.Error = fmt.Sprintf("jev ask exited %d: %s %s", code, strings.TrimSpace(stderr.String()), strings.TrimSpace(stdout.String()))
		r.Failing = true
		return r
	}
	var resp struct {
		Model     string                     `json:"model"`
		Answers   map[string]map[string]any  `json:"answers"`
		Usage     struct{ InputTokens int `json:"input_tokens"` } `json:"usage"`
		RequestID string                     `json:"request_id"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		r.Error = "cannot parse jev output: " + err.Error()
		r.Failing = true
		return r
	}
	out := evaluate(u, resp.Answers, th)
	out.RequestID = resp.RequestID
	out.Model = resp.Model
	out.InputTokens = resp.Usage.InputTokens
	out.Truncated = r.Truncated
	return out
}

// extractSection returns the lines from the heading through the line before
// the next heading of the same or higher level.
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
	for i := start + 1; i < len(lines); i++ {
		l := lines[i]
		if !strings.HasPrefix(l, "#") {
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

func buildQuestions(u unit) map[string]any {
	qs := map[string]any{}
	for i, behavior := range u.Behaviors {
		qs[fmt.Sprintf("covers_%02d", i)] = map[string]any{
			"type": "noul",
			"instructions": map[string]any{
				"question": "Do the Go tests in `tests` exercise the behavior below, which `spec` requires of the code in `implementation`?",
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
			"validation":    "Input checks and the ValidationError paths.",
			"error_mapping": "Turning HTTP statuses and transport failures into error types.",
			"retry":         "Backoff, Retry-After, budget, and cancellation.",
			"decoding":      "Turning response JSON into answer types.",
			"logging":       "What is and is not logged.",
			"none":          "No area stands out.",
		},
	}
	return qs
}

func evaluate(u unit, answers map[string]map[string]any, th thresholds) unitReport {
	r := unitReport{Name: u.Name}
	accepted := map[string]bool{}
	for _, a := range u.Accepted {
		accepted[a] = true
	}
	flag := func(id, msg string) {
		if accepted[id] {
			r.Flags = append(r.Flags, msg+" (accepted)")
			return
		}
		r.Flags = append(r.Flags, msg)
		r.Failing = true
	}
	for i, behavior := range u.Behaviors {
		id := fmt.Sprintf("covers_%02d", i)
		p := num(answers[id]["noul"])
		r.Behaviors = append(r.Behaviors, behaviorResult{ID: id, Behavior: behavior, Covered: p})
		if p < th.minCover {
			flag(id, fmt.Sprintf("%s: coverage %.2f < %.2f for %q", id, p, th.minCover, behavior))
		}
	}
	r.Contradicts = num(answers["contradicts_spec"]["noul"])
	if r.Contradicts > th.maxContradict {
		flag("contradicts_spec", fmt.Sprintf("contradicts_spec: %.2f > %.2f", r.Contradicts, th.maxContradict))
	}
	r.Thoroughness = num(answers["thoroughness"]["score"])
	r.ThoroughConf = num(answers["thoroughness"]["confidence"])
	if r.Thoroughness < th.minThorough {
		flag("thoroughness", fmt.Sprintf("thoroughness: %.2f < %.2f", r.Thoroughness, th.minThorough))
	}
	r.Weakest, _ = answers["weakest_area"]["choice"].(string)
	r.WeakestConf = num(answers["weakest_area"]["confidence"])
	return r
}

func num(v any) float64 {
	f, _ := v.(float64)
	return f
}

func markdown(reports []unitReport) string {
	var b strings.Builder
	b.WriteString("# Self-review\n\n| Unit | Behaviors covered | Contradicts | Thoroughness | Weakest | Flags |\n|---|---|---|---|---|---|\n")
	for _, r := range reports {
		covered := 0
		for _, br := range r.Behaviors {
			if br.Covered >= 0.6 {
				covered++
			}
		}
		status := fmt.Sprintf("%d", len(r.Flags))
		if r.Failing {
			status += " FAIL"
		}
		if r.Error != "" {
			status = "error"
		}
		fmt.Fprintf(&b, "| %s | %d/%d | %.2f | %.2f (%.2f) | %s | %s |\n", r.Name, covered, len(r.Behaviors), r.Contradicts, r.Thoroughness, r.ThoroughConf, r.Weakest, status)
	}
	b.WriteString("\n")
	for _, r := range reports {
		if r.Error != "" {
			fmt.Fprintf(&b, "## %s\n\nerror: %s\n\n", r.Name, r.Error)
			continue
		}
		if len(r.Flags) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", r.Name)
		if r.Truncated {
			b.WriteString("note: sources were truncated to fit the state budget\n\n")
		}
		flags := append([]string(nil), r.Flags...)
		sort.Strings(flags)
		for _, f := range flags {
			b.WriteString("- " + f + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}
```

- [ ] **Step 5: Run unit tests, then run it for real**

```bash
go test -v ./tools/selfreview/ 2>&1 | tail -8
make build && go run ./tools/selfreview -only questions 2>&1 | head -20
```
Expected: unit tests PASS. The real run prints a one-row table for `questions` with a coverage fraction and a request ID in `selfreview-report.json`. If `jev ask` exits non-zero, the report row shows the error and the exit code; fix the request shape before proceeding.

- [ ] **Step 6: Commit**

```bash
gofmt -l . ; go vet ./... && go test ./...
git add tools/selfreview/main.go tools/selfreview/main_test.go tools/selfreview/units.json
git commit -m "Add Jev-driven self-review tool"
```

---
## Task 21: Explanation pages

**Agent:** `docs` (starts after Task 1; written from the spec, revised in Task 25 if the code diverged)

**Files:**
- Create: `docs/explanation/why-the-core-is-stdlib-only.md`, `docs/explanation/retries-and-budgets.md`, `docs/explanation/why-content-is-not-traced-by-default.md`, `docs/explanation/mapping-from-the-python-sdk.md`

- [ ] **Step 1: Dispatch `diataxis-docs:doc-explanation-writer`** with this brief, once per page:

> Write `<path>` as a Diátaxis explanation page (understanding-oriented, no step-by-step instructions, no exhaustive reference). Source of truth: `docs/superpowers/specs/2026-09-23-typesafe-go-sdk-design.md`. Front matter: `title`, `description`, `type: explanation`. Length 400–900 words. Link to related reference and how-to pages by relative path even if they do not exist yet (they are listed in the spec's Documentation section). No "Authored by" lines.
>
> - `why-the-core-is-stdlib-only.md`: the `typesafe` package has zero third-party imports; the `Instrumentation` hook interface is the seam; `typesafe/otel` implements it; consequences for dependency footprint, supply chain, and testability; what a user gives up (no automatic tracing without opting in).
> - `retries-and-budgets.md`: how MaxRetries, InitialDelay, MaxDelay, Jitter, Budget, RetryStatuses, and Retry-After interact; why a retry is skipped when its delay would exceed the budget; why context cancellation interrupts a backoff sleep; why 422 is never retried but 529 is; why bodies are marshaled once and replayed.
> - `why-content-is-not-traced-by-default.md`: state is often customer data; span attributes are retained and searchable; the opt-ins WithRecordContent and WithRecordAnswers and their truncation; what is always recorded (model, counts, usage, request ID, status).
> - `mapping-from-the-python-sdk.md`: expand the spec's mapping table into prose: sync vs async, response_model vs SystemOneAs, status-specific exception classes vs APIError.Status plus RateLimitError, identical env var names, logging differences.

- [ ] **Step 2: Commit**

```bash
git add docs/explanation
git commit -m "Add explanation docs"
```

---

## Task 22: Reference pages

**Agent:** `docs` (after Tasks 9, 11, and 19 are committed)

**Files:**
- Create: `docs/reference/client-options-and-environment.md`, `docs/reference/question-types.md`, `docs/reference/response-types.md`, `docs/reference/errors-and-exit-codes.md`, `docs/reference/jev-cli.md`, `docs/reference/span-attributes.md`

- [ ] **Step 1: Dispatch `diataxis-docs:doc-reference-gen`** with this brief:

> Generate the six reference pages listed above from the source code in `/Users/bill/Projects/gojev`, not from the spec. Run `go doc -all github.com/therealbill/typesafe-go`, `go doc -all github.com/therealbill/typesafe-go/otel`, and `go run ./cmd/jev ask --help`, `models --help`, `--help` to collect signatures, defaults, flags, and help text. Reference pages state facts only: no advice, no tutorials. Front matter: `title`, `description`, `type: reference`. Tables for options, env vars, flags, exit codes, and attributes. Every exported identifier in the two packages must appear on exactly one page. The exit-code table must match the constants in `internal/cli/exit.go`. The span attribute table must match the constants in `otel/otel.go`. No "Authored by" lines.

- [ ] **Step 2: Verify completeness and commit**

```bash
for sym in $(go doc -all . | grep -oE '^(func|type) [A-Z][A-Za-z0-9]*' | awk '{print $2}' | sort -u); do grep -rq "$sym" docs/reference || echo "undocumented: $sym"; done
git add docs/reference
git commit -m "Add reference docs generated from source"
```
Expected: no `undocumented:` lines.

---

## Task 23: Tutorials

**Agent:** `docs` (after Task 22)

**Files:**
- Create: `docs/tutorials/first-judgment-in-go.md`, `docs/tutorials/jev-from-the-command-line.md`

- [ ] **Step 1: Dispatch `diataxis-docs:doc-tutorial-writer`** with this brief:

> Write two learning-oriented tutorials. Each must be runnable top to bottom by someone who has never used TypeSafe, with a checkpoint after each step showing expected output. Front matter: `title`, `description`, `type: tutorial`. Run every command yourself before writing its expected output (`TYPESAFE_API_KEY` is set). No "Authored by" lines.
>
> `first-judgment-in-go.md` (about 20 minutes): create a module, `go get github.com/therealbill/typesafe-go`, set the key, write a `main.go` that sends a support ticket as state with one Noul, one Choice, and one Score (use the spec's fixture questions), print the three answers, then extend it with `SystemOneAs` into a struct, then add `errors.As` handling for `*typesafe.APIError`. Show the actual JSON the API returned.
>
> `jev-from-the-command-line.md` (about 10 minutes): build with `make build`, run `jev version`, `jev models --pretty`, one `jev ask` in flag mode, the same request in JSON mode from a file, read the exit code with `echo $?`, and provoke an auth error with a bad key to show the error JSON and exit code 3.

- [ ] **Step 2: Commit**

```bash
git add docs/tutorials
git commit -m "Add tutorials"
```

---

## Task 24: How-to guides

**Agent:** `docs` (after Task 22)

**Files:**
- Create: `docs/how-to/handle-rate-limits-and-retries.md`, `docs/how-to/decode-answers-into-your-own-struct.md`, `docs/how-to/trace-calls-and-send-to-honeycomb.md`, `docs/how-to/call-through-a-gateway.md`, `docs/how-to/drive-jev-from-a-script-or-agent.md`

- [ ] **Step 1: Dispatch `diataxis-docs:doc-howto-writer`** with this brief:

> Write five goal-oriented how-to guides, each under 1200 words, numbered steps, assuming the reader has done the tutorials. Front matter: `title`, `description`, `type: how-to`. Compile every Go snippet (`go vet` on a scratch file) and run every shell command before writing expected output. No "Authored by" lines.
>
> - `handle-rate-limits-and-retries.md`: customize `RetryPolicy` from `DefaultRetryPolicy()`, per-call `WithRequestRetry`, detect `*RateLimitError` and read `RetryAfter`, use `IsRetryable`, disable retries.
> - `decode-answers-into-your-own-struct.md`: `SystemOneAs[T]`, pointer vs value fields, handling `*ResponseValidationError`, keeping the full response for usage.
> - `trace-calls-and-send-to-honeycomb.md`: library side: set a global tracer provider with `otelconf` or `sdktrace`, attach `otel.New()` with `WithInstrumentation`, opt into content and answers; CLI side: `HONEYCOMB_API_KEY`, `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_CONFIG_FILE` with `${HONEYCOMB_API_KEY}` expansion, `--trace`, `--no-trace`; what the resulting spans look like (name, attributes).
> - `call-through-a-gateway.md`: `WithBaseURL` and `TYPESAFE_BASE_URL`, `WithHeaders` for gateway auth, `WithHTTPClient` for proxies and custom TLS, `WithExtraBody` for gateway-specific fields.
> - `drive-jev-from-a-script-or-agent.md`: this is the contract the future plugin relies on: the stdin JSON shape, the stdout response shape, the error JSON shape, the full exit-code table, `--raw`, `--pretty`, and a bash example that branches on `$?`. Include a `jq` one-liner extracting a choice.

- [ ] **Step 2: Commit**

```bash
git add docs/how-to
git commit -m "Add how-to guides"
```

---

## Task 25: README, doc validation, explanation refresh

**Agent:** `docs` (after Tasks 23 and 24)

**Files:**
- Create: `README.md`
- Modify: `docs/explanation/*.md` if the code diverged from the spec

- [ ] **Step 1: Write `README.md`**

```markdown
# typesafe-go

Go client for the [TypeSafe](https://typesafe.ai) System One API, plus the `jev` command-line tool. Ask Jev typed questions about a piece of state and get back probabilities, choices, and scores your code can act on.

## Install

```bash
go get github.com/therealbill/typesafe-go
```

Prebuilt `jev` binaries for macOS and Linux are attached to each [release](https://github.com/therealbill/typesafe-go/releases). Or `make build` to produce `bin/jev`.

## Library

```go
client, err := typesafe.NewClient() // reads TYPESAFE_API_KEY
res, err := client.SystemOne(ctx, "I was charged twice and nobody replies.", typesafe.Questions{
    "billing": typesafe.Noul{Instructions: "Is this about billing?"},
    "tone":    typesafe.Choice{Instructions: "What is the tone?", Criteria: map[string]typesafe.JSONContent{"calm": nil, "angry": nil}},
})
fmt.Println(res.Nouls()["billing"].Noul, res.Choices()["tone"].Choice)
```

## Command line

```bash
export TYPESAFE_API_KEY=...
jev ask --state "I was charged twice." --noul billing="Is this about billing?"
echo '{"state":"...","questions":{"billing":{"type":"noul","instructions":"Is this about billing?"}}}' | jev ask --pretty
```

## Documentation

- Tutorials: [Your first judgment in Go](docs/tutorials/first-judgment-in-go.md), [jev from the command line](docs/tutorials/jev-from-the-command-line.md)
- How-to: [rate limits and retries](docs/how-to/handle-rate-limits-and-retries.md), [decode into your own struct](docs/how-to/decode-answers-into-your-own-struct.md), [trace calls and send to Honeycomb](docs/how-to/trace-calls-and-send-to-honeycomb.md), [call through a gateway](docs/how-to/call-through-a-gateway.md), [drive jev from a script or agent](docs/how-to/drive-jev-from-a-script-or-agent.md)
- Reference: [client options and environment](docs/reference/client-options-and-environment.md), [question types](docs/reference/question-types.md), [response types](docs/reference/response-types.md), [errors and exit codes](docs/reference/errors-and-exit-codes.md), [jev CLI](docs/reference/jev-cli.md), [span attributes](docs/reference/span-attributes.md)
- Explanation: [why the core is stdlib-only](docs/explanation/why-the-core-is-stdlib-only.md), [retries and budgets](docs/explanation/retries-and-budgets.md), [why content is not traced by default](docs/explanation/why-content-is-not-traced-by-default.md), [mapping from the Python SDK](docs/explanation/mapping-from-the-python-sdk.md)

## Environment

| Variable | Purpose | Default |
|---|---|---|
| `TYPESAFE_API_KEY` | API key (required) | |
| `TYPESAFE_BASE_URL` | API root | `https://api.typesafe.ai` |
| `TYPESAFE_DEFAULT_MODEL` | Model | `jev-latest` |
| `TYPESAFE_LOG_LEVEL` | `debug`, `info`, `warning`, `error`, `off` | off |
| `HONEYCOMB_API_KEY` | Enables tracing in `jev` and sets the Honeycomb header | |
| `OTEL_SERVICE_NAME` | Service name for `jev` traces | `jev` |

## License

MIT
```

Also create `LICENSE` with the MIT text and the copyright line `Copyright (c) 2026 Bill Anderson`.

- [ ] **Step 2: Refresh explanation pages against the code**

Dispatch `diataxis-docs:doc-explanation-writer` to re-read the four explanation pages against the actual source and fix any claim that no longer matches (for example if a default or type name changed during implementation).

- [ ] **Step 3: Validate and commit**

```bash
make docs
```
Then dispatch `diataxis-docs:doc-crosslink-validator` over `docs/` and `README.md`; fix everything it reports.

```bash
git add README.md LICENSE docs
git commit -m "Add README, license, and validate documentation"
```

---

## Task 26: Self-review run and fix loop

**Agent:** lead (after Tasks 20, 22 committed; all agents idle)

- [ ] **Step 1: Run the full self-review**

```bash
make selfreview 2>&1 | tee /private/tmp/claude-501/-Users-bill-Projects-gojev/b96adec7-3da2-45f5-b6b1-95fdebc40eb3/scratchpad/selfreview.md
```
Expected: a table with one row per unit. Exit code 0 means nothing is flagged.

- [ ] **Step 2: Triage each flag**

For each flagged line (`covers_NN`, `contradicts_spec`, `thoroughness`), read the named behavior, the test file, and the implementation. Decide:

- **Real gap:** send the owning agent (`core` for library units, `otel` for otel, `cli` for cli) a message naming the unit, the behavior text, and the file to add a test to or the code to fix. The agent adds the test first, watches it fail if it is a code fix, fixes, runs `go test -race ./...`, commits by explicit path, and reports the commit hash.
- **Model miss** (the test exists and asserts the behavior): add the question id to that unit's `accepted` list in `tools/selfreview/units.json` with a sibling `"notes"` entry such as `"covers_03: TestSystemOneBudgetExhausted asserts this"`, so the acceptance is recorded next to the reason.

- [ ] **Step 3: Rerun until clean**

```bash
make selfreview
echo "exit $?"
```
Repeat Steps 2 and 3 until the exit code is 0. Commit `units.json` changes:

```bash
git add tools/selfreview/units.json selfreview-report.json
git commit -m "Record self-review acceptances"
```
Note: `selfreview-report.json` is gitignored; only `units.json` will be staged. Keep the final report in the scratchpad and quote its table in the completion message.

---

## Task 27: Final verification and code review

**Agent:** lead

- [ ] **Step 1: Full gate**

```bash
make lint && make test && make vuln && make build && make integration && make docs
git status --short
```
Expected: every target passes, working tree clean.

- [ ] **Step 2: Code review**

Invoke `superpowers:requesting-code-review` and dispatch `superpowers:code-reviewer` with the spec path, the plan path, and `git log --oneline` as context. Route each finding to the owning agent; agents fix by explicit-path commits. Re-run Step 1 after fixes.

- [ ] **Step 3: Tag**

Do not tag or push unless the user asks. Report: commit count, test summary, self-review table, and anything accepted rather than fixed.
