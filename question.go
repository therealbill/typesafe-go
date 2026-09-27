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
