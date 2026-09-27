package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/therealbill/typesafe-go"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
		kind string
	}{
		{"validation", &typesafe.ValidationError{Path: "questions", Err: errors.New("x")}, ExitValidation, "validation"},
		{"401", &typesafe.APIError{Status: 401}, ExitAuth, "auth"},
		{"403", &typesafe.APIError{Status: 403}, ExitAuth, "auth"},
		{"400", &typesafe.APIError{Status: 400}, ExitRequest, "request"},
		{"404", &typesafe.APIError{Status: 404}, ExitRequest, "request"},
		{"422", &typesafe.APIError{Status: 422}, ExitRequest, "request"},
		{"418", &typesafe.APIError{Status: 418}, ExitRequest, "request"},
		{"429", &typesafe.RateLimitError{APIError: typesafe.APIError{Status: 429}}, ExitRateLimit, "rate_limit"},
		{"503", &typesafe.APIError{Status: 503}, ExitServer, "server"},
		{"529", &typesafe.APIError{Status: 529}, ExitServer, "server"},
		{"connection", &typesafe.ConnectionError{Err: errors.New("refused")}, ExitConnection, "connection"},
		{"timeout", &typesafe.TimeoutError{ConnectionError: typesafe.ConnectionError{Err: context.DeadlineExceeded}}, ExitConnection, "connection"},
		{"invalid response", &typesafe.ResponseValidationError{FieldPath: "answers.a", Err: errors.New("x")}, ExitServer, "invalid_response"},
		{"missing key", typesafe.ErrMissingAPIKey, ExitUsage, "usage"},
		{"usage", &usageError{errors.New("bad flag")}, ExitUsage, "usage"},
		{"unknown", errors.New("?"), ExitUsage, "usage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, kind := classify(tt.err)
			if code != tt.code || kind != tt.kind {
				t.Fatalf("got (%d, %s) want (%d, %s)", code, kind, tt.code, tt.kind)
			}
		})
	}
}

func TestFailWritesJSONAndReturnsExitError(t *testing.T) {
	var out, errOut bytes.Buffer
	io := IO{Out: &out, Err: &errOut}
	err := fail(io, false, &typesafe.APIError{Status: 401, Endpoint: "POST /v1/systemone", RequestID: "req_9", Body: []byte(`{"detail":{"error_type":"authentication_error","message":"Cannot authenticate"}}`)})
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != ExitAuth {
		t.Fatalf("err %v", err)
	}
	var payload struct {
		Error struct {
			Kind      string `json:"kind"`
			Status    int    `json:"status"`
			RequestID string `json:"request_id"`
			Message   string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("stdout not JSON: %s", out.String())
	}
	if payload.Error.Kind != "auth" || payload.Error.Status != 401 || payload.Error.RequestID != "req_9" || payload.Error.Message != "authentication_error: Cannot authenticate" {
		t.Fatalf("payload %+v", payload)
	}
	if !bytes.Contains(errOut.Bytes(), []byte("jev: ")) {
		t.Fatalf("stderr %q", errOut.String())
	}
}
