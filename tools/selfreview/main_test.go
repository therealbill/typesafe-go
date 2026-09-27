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
	labels := qs["weakest_area"].(map[string]any)["criteria"].(map[string]any)
	for _, want := range []string{"validation", "error_mapping", "retry", "decoding", "encoding", "logging", "none"} {
		if _, ok := labels[want]; !ok {
			t.Fatalf("weakest_area is missing the %q label", want)
		}
	}
	if len(labels) != 7 {
		t.Fatalf("weakest_area has %d labels, want 7", len(labels))
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
	u := unit{Name: "x", Behaviors: []string{"b0", "b1"}, Accepted: []string{"b1"}, Notes: []string{"b1: known noise"}}
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
	if len(r.Notes) != 1 || r.Notes[0] != "b1: known noise" {
		t.Fatalf("notes must travel into the report so an acceptance carries its rationale, got %v", r.Notes)
	}
	ans["contradicts_spec"]["noul"] = 0.7
	r = evaluate(u, ans, thresholds{minCover: 0.6, maxContradict: 0.4, minThorough: 2.0})
	if !r.Failing || len(r.Flags) != 2 {
		t.Fatalf("contradiction should fail: %+v", r)
	}
}

func TestExtractSectionIgnoresFencedHeadings(t *testing.T) {
	spec := "## One\n\ntext\n\n```sh\n# comment that is not a heading\n## also not a heading\n```\n\nmore text\n\n## Two\n\nother\n"
	got, err := extractSection(spec, "## One")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "# comment that is not a heading") {
		t.Fatalf("a fenced comment must stay inside the section: %q", got)
	}
	if !strings.Contains(got, "more text") {
		t.Fatalf("the section must continue past the fence: %q", got)
	}
	if strings.Contains(got, "other") {
		t.Fatalf("the section must still stop at the next real heading: %q", got)
	}
}

func TestValidateUnitsRejectsPositionalAcceptance(t *testing.T) {
	cfg := config{Units: []unit{{Name: "x", Behaviors: []string{"b0"}, Accepted: []string{"covers_00"}}}}
	err := validateUnits(cfg)
	if err == nil {
		t.Fatal("a positional covers_NN acceptance must be rejected")
	}
	if !strings.Contains(err.Error(), "covers_00") || !strings.Contains(err.Error(), "x") {
		t.Fatalf("the error should name the unit and the entry: %v", err)
	}
	ok := config{Units: []unit{{Name: "x", Behaviors: []string{"b0"}, Accepted: []string{"b0", "thoroughness"}}}}
	if err := validateUnits(ok); err != nil {
		t.Fatalf("behavior text and question ids are both valid: %v", err)
	}
}

func TestEvaluateReportsStaleAcceptance(t *testing.T) {
	u := unit{Name: "x", Behaviors: []string{"b0"}, Accepted: []string{"b0", "thoroughness"}}
	ans := map[string]map[string]any{
		"covers_00":        {"noul": 0.9},
		"contradicts_spec": {"noul": 0.1},
		"thoroughness":     {"score": 2.5, "confidence": 0.8},
		"weakest_area":     {"choice": "retry", "confidence": 0.6},
	}
	r := evaluate(u, ans, thresholds{minCover: 0.6, maxContradict: 0.4, minThorough: 2.0})
	if r.Failing {
		t.Fatalf("nothing should fail: %+v", r)
	}
	if len(r.StaleAcceptances) != 2 {
		t.Fatalf("both unused acceptances should be reported, got %v", r.StaleAcceptances)
	}
	out := markdown([]unitReport{r}, thresholds{minCover: 0.6})
	if !strings.Contains(out, "acceptance did not fire: b0") {
		t.Fatalf("stale acceptances must appear in the summary:\n%s", out)
	}
}

func TestEvaluateMissingOrNonNumericAnswerIsError(t *testing.T) {
	u := unit{Name: "x", Behaviors: []string{"b0"}}
	th := thresholds{minCover: 0.6, maxContradict: 0.4, minThorough: 2.0}
	full := func() map[string]map[string]any {
		return map[string]map[string]any{
			"covers_00":        {"noul": 0.9},
			"contradicts_spec": {"noul": 0.1},
			"thoroughness":     {"score": 2.5, "confidence": 0.8},
			"weakest_area":     {"choice": "retry", "confidence": 0.6},
		}
	}
	if r := evaluate(u, full(), th); r.Error != "" {
		t.Fatalf("a complete answer set must not error: %q", r.Error)
	}
	for _, drop := range []string{"covers_00", "contradicts_spec", "thoroughness", "weakest_area"} {
		ans := full()
		delete(ans, drop)
		r := evaluate(u, ans, th)
		if r.Error == "" || !r.Failing {
			t.Errorf("a missing %s must be a unit error, got %+v", drop, r)
		}
	}
	ans := full()
	ans["thoroughness"]["score"] = "high"
	r := evaluate(u, ans, th)
	if r.Error == "" || !r.Failing {
		t.Errorf("a non-numeric score must be a unit error, got %+v", r)
	}
}

func TestMarkdownUsesMinCoverThreshold(t *testing.T) {
	r := unitReport{Name: "x", Behaviors: []behaviorResult{
		{ID: "covers_00", Covered: 0.65},
		{ID: "covers_01", Covered: 0.80},
	}}
	if out := markdown([]unitReport{r}, thresholds{minCover: 0.6}); !strings.Contains(out, "2/2") {
		t.Fatalf("both behaviors clear 0.6:\n%s", out)
	}
	if out := markdown([]unitReport{r}, thresholds{minCover: 0.7}); !strings.Contains(out, "1/2") {
		t.Fatalf("only one behavior clears 0.7:\n%s", out)
	}
}
