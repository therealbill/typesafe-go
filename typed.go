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
	res, err := c.SystemOne(ctx, state, questions, opts...)
	if err != nil {
		var out T
		return out, nil, err
	}
	out, err := answersInto[T](res)
	return out, res, err
}

// answersInto decodes the answers object of a response into T.
func answersInto[T any](res *SystemOneResponse) (T, error) {
	var out T
	if res == nil || res.Raw == nil {
		return out, &ResponseValidationError{FieldPath: "answers", Err: errors.New("raw response body unavailable")}
	}
	var w struct {
		Answers json.RawMessage `json:"answers"`
	}
	if err := json.Unmarshal(res.Raw.Body, &w); err != nil {
		return out, fieldErr("answers", err)
	}
	if err := json.Unmarshal(w.Answers, &out); err != nil {
		var rve *ResponseValidationError
		if errors.As(err, &rve) {
			return out, err
		}
		return out, fieldErr("answers", err)
	}
	return out, nil
}
