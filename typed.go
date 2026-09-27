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
