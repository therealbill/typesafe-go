package typesafe

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
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

// AnswerType returns "noul".
func (NoulAnswer) AnswerType() string { return "noul" }

// AnswerType returns "choice".
func (ChoiceAnswer) AnswerType() string { return "choice" }

// AnswerType returns "score".
func (ScoreAnswer) AnswerType() string { return "score" }

// AnswerType returns the wire type of the unknown answer.
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
	// Decode in key order so a response with more than one bad answer always
	// reports the same field path.
	keys := make([]string, 0, len(w.Answers))
	for key := range w.Answers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		a, err := decodeAnswer(joinPath("answers", key), w.Answers[key])
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

// UnmarshalJSON records the answer's type and keeps the raw bytes so the
// answer survives a marshal unchanged.
func (a *UnknownAnswer) UnmarshalJSON(b []byte) error {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return fieldErr("", err)
	}
	a.Type = head.Type
	a.Raw = append(json.RawMessage(nil), b...)
	return nil
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
