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

func TestDecodeDefaultsMissingMaps(t *testing.T) {
	res, err := decodeSystemOne([]byte(`{"model":"m","answers":{"tone":{"type":"choice","choice":"angry","confidence":1},"urgency":{"type":"score","score":1.5,"confidence":0.5}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := res.Choices()["tone"]
	if c.Probabilities == nil {
		t.Fatal("a choice answer without probabilities must decode to an empty non-nil map")
	}
	if len(c.Probabilities) != 0 {
		t.Fatalf("probabilities should be empty, got %v", c.Probabilities)
	}
	s := res.Scores()["urgency"]
	if s.Legend == nil || s.Probabilities == nil {
		t.Fatalf("a score answer without legend or probabilities must decode to empty non-nil maps, got legend=%v probabilities=%v", s.Legend, s.Probabilities)
	}
	if len(s.Legend) != 0 || len(s.Probabilities) != 0 {
		t.Fatalf("legend and probabilities should be empty, got legend=%v probabilities=%v", s.Legend, s.Probabilities)
	}
}
