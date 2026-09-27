package cli

import (
	"encoding/json"
	"io"
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

func TestBuildRequestTerminalStdin(t *testing.T) {
	saved := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = saved })
	stdinIsTerminal = func(io.Reader) bool { return true }

	if _, err := buildRequest(&askOptions{}, strings.NewReader("")); err == nil ||
		!strings.Contains(err.Error(), "no request given") {
		t.Fatalf("a terminal stdin must fail immediately, got %v", err)
	}

	doc := `{"state":"x","questions":{"a":{"type":"noul","instructions":"?"}}}`
	req, err := buildRequest(&askOptions{file: "-"}, strings.NewReader(doc))
	if err != nil {
		t.Fatalf("-f - must read stdin even on a terminal: %v", err)
	}
	if req.State != "x" {
		t.Fatalf("state %v", req.State)
	}
}

func TestBuildRequestPipedStdinStillReads(t *testing.T) {
	saved := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = saved })
	stdinIsTerminal = func(io.Reader) bool { return false }

	doc := `{"state":"y","questions":{"a":{"type":"noul","instructions":"?"}}}`
	req, err := buildRequest(&askOptions{}, strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	if req.State != "y" {
		t.Fatalf("state %v", req.State)
	}
}

func TestStdinIsTerminalOnNonFile(t *testing.T) {
	if stdinIsTerminal(strings.NewReader("")) {
		t.Fatal("a non-*os.File reader is never a terminal")
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close(); _ = w.Close() })
	if stdinIsTerminal(r) {
		t.Fatal("a pipe is not a terminal")
	}
}

func TestSplitInstrLabelsUsesLastColon(t *testing.T) {
	instr, labels, err := splitInstrLabels("Is the tone calm: yes or no?:calm,angry", ",", "--choice")
	if err != nil {
		t.Fatal(err)
	}
	if instr != "Is the tone calm: yes or no?" {
		t.Fatalf("instructions %q", instr)
	}
	if len(labels) != 2 || labels[0] != "calm" || labels[1] != "angry" {
		t.Fatalf("labels %v", labels)
	}

	instr, levels, err := splitInstrLabels("Urgency, on a scale: how bad?:low|mid|high", "|", "--score")
	if err != nil {
		t.Fatal(err)
	}
	if instr != "Urgency, on a scale: how bad?" {
		t.Fatalf("instructions %q", instr)
	}
	if len(levels) != 3 || levels[2] != "high" {
		t.Fatalf("levels %v", levels)
	}

	if _, _, err := splitInstrLabels("no colon here", ",", "--choice"); err == nil {
		t.Fatal("a value with no colon must still error")
	}
}

func TestRequestFromFlagsColonInInstructions(t *testing.T) {
	o := &askOptions{
		state:   "s",
		choices: []string{"tone=Is the tone calm: yes or no?:calm,angry"},
		scores:  []string{"urgency=Urgency: how bad?:low|high"},
	}
	req, err := requestFromFlags(o, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	c := req.Questions["tone"].(typesafe.Choice)
	if c.Instructions != "Is the tone calm: yes or no?" || len(c.Criteria) != 2 {
		t.Fatalf("choice %+v", c)
	}
	if _, ok := c.Criteria["angry"]; !ok {
		t.Fatalf("choice labels %v", c.Criteria)
	}
	s := req.Questions["urgency"].(typesafe.Score)
	if s.Instructions != "Urgency: how bad?" || len(s.Criteria) != 2 || s.Criteria[1] != "high" {
		t.Fatalf("score %+v", s)
	}
}

func TestReadCapRejectsOversizedInput(t *testing.T) {
	saved := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = saved })
	stdinIsTerminal = func(io.Reader) bool { return false }

	big := strings.NewReader(strings.Repeat("x", maxRequestBytes+1))
	if _, err := buildRequest(&askOptions{}, big); err == nil ||
		!strings.Contains(err.Error(), "request exceeds 16 MiB") {
		t.Fatalf("oversized stdin: got %v", err)
	}

	path := filepath.Join(t.TempDir(), "big.json")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maxRequestBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := buildRequest(&askOptions{file: path}, strings.NewReader("")); err == nil ||
		!strings.Contains(err.Error(), "request exceeds 16 MiB") {
		t.Fatalf("oversized file: got %v", err)
	}

	if _, err := requestFromFlags(&askOptions{state: "@" + path, nouls: []string{"a=b"}}, strings.NewReader("")); err == nil ||
		!strings.Contains(err.Error(), "request exceeds 16 MiB") {
		t.Fatalf("oversized @file state: got %v", err)
	}

	stateBig := strings.NewReader(strings.Repeat("y", maxRequestBytes+1))
	if _, err := requestFromFlags(&askOptions{state: "-", nouls: []string{"a=b"}}, stateBig); err == nil ||
		!strings.Contains(err.Error(), "request exceeds 16 MiB") {
		t.Fatalf("oversized stdin state: got %v", err)
	}
}

func TestReadCapAcceptsInputAtTheLimit(t *testing.T) {
	saved := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = saved })
	stdinIsTerminal = func(io.Reader) bool { return false }

	doc := `{"state":"` + strings.Repeat("z", 1024) + `","questions":{"a":{"type":"noul","instructions":"?"}}}`
	if _, err := buildRequest(&askOptions{}, strings.NewReader(doc)); err != nil {
		t.Fatalf("input well under the limit must pass: %v", err)
	}
}

// typoCriteria is a deliberate misspelling of "criteria". It lives in a
// constant so the misspell linter is suppressed in exactly one place.
const typoCriteria = "critera" //nolint:misspell // deliberate typo fixture

func TestParseQuestionRejectsUnknownFields(t *testing.T) {
	tests := []struct {
		name  string
		doc   string
		wants []string
	}{
		{"choice typo", `{"state":"x","questions":{"q":{"type":"choice","instructions":"x","` + typoCriteria + `":{"a":null}}}}`,
			[]string{"questions.q", typoCriteria}},
		{"noul typo", `{"state":"x","questions":{"q":{"type":"noul","instruction":"x"}}}`,
			[]string{"questions.q", "instruction"}},
		{"score typo", `{"state":"x","questions":{"q":{"type":"score","levels":["a","b"]}}}`,
			[]string{"questions.q", "levels"}},
		{"noul nested criteria typo", `{"state":"x","questions":{"q":{"type":"noul","criteria":{"yes":"a"}}}}`,
			[]string{"questions.q", "yes"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseRequestJSON([]byte(tt.doc))
			if err == nil {
				t.Fatal("expected an error")
			}
			for _, w := range tt.wants {
				if !strings.Contains(err.Error(), w) {
					t.Fatalf("error %q must contain %q", err, w)
				}
			}
		})
	}
}

func TestParseRequestJSONStillAllowsUnknownTopLevelAndQuestionTypes(t *testing.T) {
	doc := `{"state":"x","weight":3,"questions":{"f":{"type":"future","anything":1}}}`
	req, err := parseRequestJSON([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if req.Extra["weight"] != float64(3) {
		t.Fatalf("extra %v", req.Extra)
	}
	r := req.Questions["f"].(typesafe.RawQuestion)
	if r["type"] != "future" || r["anything"] != float64(1) {
		t.Fatalf("raw question %+v", r)
	}
}

func TestRequestFromFlagsStateWithoutQuestions(t *testing.T) {
	_, err := requestFromFlags(&askOptions{state: "s"}, strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "--noul") {
		t.Fatalf("got %v, want an error naming the question flags", err)
	}
}

func TestSplitKVRejectsBlankKeys(t *testing.T) {
	for _, s := range []string{" =b", "\t=b", "   =instructions"} {
		if _, _, err := splitKV(s, "--noul"); err == nil {
			t.Fatalf("splitKV(%q) must reject a blank key", s)
		}
	}
	key, rest, err := splitKV("  a  =b", "--noul")
	if err != nil || key != "a" || rest != "b" {
		t.Fatalf("got %q %q %v", key, rest, err)
	}
}
