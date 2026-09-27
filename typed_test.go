package typesafe

import (
	"context"
	"errors"
	"testing"
)

type triage struct {
	Billing NoulAnswer   `json:"billing"`
	Tone    ChoiceAnswer `json:"tone"`
	Urgency *ScoreAnswer `json:"urgency"`
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
