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
	r = evaluate(u, answers([]float64{0.9, 0.2}, 0.7, 2.4, 0.55, "retry"), defaults())
	if !r.Failing || len(r.Flags) != 2 {
		t.Fatalf("contradiction should fail: %+v", r)
	}
	if r.ThoroughConf != 0.55 || r.WeakestConf != 0.6 {
		t.Fatalf("confidences must travel into the report, got %v and %v", r.ThoroughConf, r.WeakestConf)
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
	if !strings.Contains(cfg.Description, `A unit may set its own "spec" to override the top-level one.`) {
		t.Fatalf("the template description must mention the per-unit override: %q", cfg.Description)
	}
}

func TestLoadAcceptsPerUnitSpecWithoutTopLevelSpec(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, `{"units":[{"name":"a","spec":"a.md","spec_heading":"## A","implementation":["a.go"],"behaviors":["b"]},{"name":"b","spec":"b.md","spec_heading":"## B","implementation":["b.go"],"behaviors":["b"]}]}`)
	cfg, base, err := Load(p)
	if err != nil {
		t.Fatalf("a config whose units all set a spec must load: %v", err)
	}
	if base != dir || cfg.Spec != "" {
		t.Fatalf("base %q cfg %+v", base, cfg)
	}
	if cfg.Units[0].Spec != "a.md" || cfg.Units[1].Spec != "b.md" {
		t.Fatalf("per-unit specs not decoded: %+v", cfg.Units)
	}
}

func TestLoadRejectsAUnitWithNoSpecAnywhere(t *testing.T) {
	p := writeConfig(t, t.TempDir(), `{"units":[{"name":"a","spec":"a.md","spec_heading":"## A","implementation":["a.go"],"behaviors":["b"]},{"name":"b","spec_heading":"## B","implementation":["b.go"],"behaviors":["b"]}]}`)
	_, _, err := Load(p)
	const want = `"spec" is required at the top level or on every unit`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("got %v want substring %q", err, want)
	}
	if !strings.Contains(err.Error(), `unit "b"`) {
		t.Fatalf("the message must name the unit that lacks a spec: %v", err)
	}
}

func TestSpecCacheReadsEachPathOnce(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(p, []byte("## A\n\ntext\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := newSpecCache(dir)
	first, err := c.read("spec.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	second, err := c.read("spec.md")
	if err != nil || second != first {
		t.Fatalf("the second read must come from the cache, got %q %v", second, err)
	}
	if _, err := c.read("missing.md"); err == nil {
		t.Fatal("an unreadable spec must return its error")
	}
}

// setupSplitSpecRepo writes one spec document per unit and a config whose
// second unit overrides the top-level spec.
func setupSplitSpecRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n\n## Retry\n\nretry text\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "other.md"), []byte("# Other\n\n## Other\n\nother text\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "retry.go"), []byte("package x\n// impl\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "retry_test.go"), []byte("package x\n// tests\n"), 0o600)
	p := writeConfig(t, dir, `{"spec":"spec.md","units":[{"name":"retry","spec_heading":"## Retry","implementation":["retry.go"],"tests":["retry_test.go"],"behaviors":["b0"]},{"name":"other","spec":"other.md","spec_heading":"## Other","implementation":["retry.go"],"tests":["retry_test.go"],"behaviors":["b0"]}]}`)
	return dir, p
}

func TestRunSendsEachUnitItsOwnSpec(t *testing.T) {
	_, p := setupSplitSpecRepo(t)
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
	if len(fa.states) != 2 {
		t.Fatalf("want 2 calls, got %d", len(fa.states))
	}
	if s := fa.states[0]["spec"].(string); !strings.Contains(s, "retry text") || strings.Contains(s, "other text") {
		t.Fatalf("the first unit must get the top-level spec: %q", s)
	}
	if s := fa.states[1]["spec"].(string); !strings.Contains(s, "other text") || strings.Contains(s, "retry text") {
		t.Fatalf("the second unit must get its own spec: %q", s)
	}
}

func TestRunMissingSpecIsUnitError(t *testing.T) {
	dir, p := setupSplitSpecRepo(t)
	if err := os.Remove(filepath.Join(dir, "other.md")); err != nil {
		t.Fatal(err)
	}
	cfg, base, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	fa := &fakeAsker{answers: answers([]float64{0.9}, 0.1, 2.5, 0.8, "none")}
	rep, err := Run(context.Background(), cfg, base, fa, defaults())
	if err != nil {
		t.Fatalf("an unreadable unit spec must not stop the run: %v", err)
	}
	if len(rep.Units) != 2 {
		t.Fatalf("both units belong in the report: %+v", rep)
	}
	if rep.Units[0].Failing || rep.Units[0].Error != "" {
		t.Fatalf("the unit with a readable spec still runs: %+v", rep.Units[0])
	}
	if !rep.Units[1].Failing || !strings.Contains(rep.Units[1].Error, "other.md") {
		t.Fatalf("the unit with the missing spec must carry the error: %+v", rep.Units[1])
	}
	if len(fa.states) != 1 {
		t.Fatalf("only the unit with a readable spec should call: %d", len(fa.states))
	}

	if err := os.Remove(filepath.Join(dir, "spec.md")); err != nil {
		t.Fatal(err)
	}
	fa = &fakeAsker{answers: answers([]float64{0.9}, 0.1, 2.5, 0.8, "none")}
	rep, err = Run(context.Background(), cfg, base, fa, defaults())
	if err != nil {
		t.Fatalf("an unreadable top-level spec must not stop the run: %v", err)
	}
	if len(rep.Units) != 2 || !rep.Units[0].Failing || !strings.Contains(rep.Units[0].Error, "spec.md") {
		t.Fatalf("every unit must carry its own spec error: %+v", rep.Units)
	}
	if len(fa.states) != 0 {
		t.Fatalf("no unit should call: %d", len(fa.states))
	}
}

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
