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

func TestNoulCriteriaOmittedWhenNil(t *testing.T) {
	out, err := json.Marshal(Noul{Instructions: "x"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["criteria"]; ok {
		t.Fatalf("a nil Criteria must be omitted from the wire JSON, got %s", out)
	}

	out, err = json.Marshal(Noul{Instructions: "x", Criteria: &NoulCriteria{True: "yes"}})
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	crit, ok := got["criteria"].(map[string]any)
	if !ok {
		t.Fatalf("a non-nil Criteria must appear in the wire JSON, got %s", out)
	}
	if crit["true"] != "yes" {
		t.Fatalf("criteria.true should carry the description, got %s", out)
	}
	if _, ok := crit["false"]; ok {
		t.Fatalf("an unset Criteria.False must be omitted, got %s", out)
	}
}
