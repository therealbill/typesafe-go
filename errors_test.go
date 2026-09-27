package typesafe

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAPIErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"detail object", `{"detail":{"error_type":"authentication_error","message":"Cannot authenticate with the server. Please check your API key and try again."}}`,
			"authentication_error: Cannot authenticate with the server. Please check your API key and try again."},
		{"detail array", `{"detail":[{"type":"too_short","loc":["body","questions"],"msg":"Dictionary should have at least 1 item after validation, not 0","input":{}}]}`,
			"body.questions: Dictionary should have at least 1 item after validation, not 0"},
		{"detail string", `{"detail":"Not Found"}`, "Not Found"},
		{"message field", `{"message":"boom"}`, "boom"},
		{"error field", `{"error":"bad"}`, "bad"},
		{"plain text", `  service unavailable  `, "service unavailable"},
		{"empty", ``, ""},
		{"long text", strings.Repeat("x", 300), strings.Repeat("x", 200) + "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &APIError{Status: 400, Body: []byte(tt.body)}
			if got := e.Message(); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestAPIErrorError(t *testing.T) {
	e := &APIError{Status: 404, Endpoint: "GET /v1/nope", Body: []byte(`{"detail":"Not Found"}`), RequestID: "req_1"}
	want := "typesafe: GET /v1/nope returned 404: Not Found (request id req_1)"
	if e.Error() != want {
		t.Fatalf("got %q want %q", e.Error(), want)
	}
	e2 := &APIError{Status: 500, Endpoint: "POST /v1/systemone"}
	if e2.Error() != "typesafe: POST /v1/systemone returned 500" {
		t.Fatalf("got %q", e2.Error())
	}
}

func TestRateLimitErrorUnwrapsToAPIError(t *testing.T) {
	var err error = &RateLimitError{APIError: APIError{Status: 429}, RetryAfter: 2 * time.Second}
	var api *APIError
	if !errors.As(err, &api) || api.Status != 429 {
		t.Fatal("RateLimitError should unwrap to *APIError")
	}
	if !IsRateLimited(err) {
		t.Fatal("IsRateLimited should be true")
	}
	if IsAuthError(err) {
		t.Fatal("IsAuthError should be false")
	}
}

func TestIsAuthError(t *testing.T) {
	for _, s := range []int{401, 403} {
		if !IsAuthError(&APIError{Status: s}) {
			t.Fatalf("status %d should be auth error", s)
		}
	}
	if IsAuthError(&APIError{Status: 404}) {
		t.Fatal("404 is not an auth error")
	}
	if IsAuthError(errors.New("x")) {
		t.Fatal("plain error is not an auth error")
	}
}

func TestTimeoutErrorUnwrapsToConnectionError(t *testing.T) {
	var err error = &TimeoutError{ConnectionError: ConnectionError{Err: context.DeadlineExceeded}, Timeout: time.Second}
	var ce *ConnectionError
	if !errors.As(err, &ce) {
		t.Fatal("TimeoutError should unwrap to *ConnectionError")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("should unwrap to the cause")
	}
	if !strings.Contains(err.Error(), "timed out after 1s") {
		t.Fatalf("message: %q", err.Error())
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{&APIError{Status: 429}, true},
		{&RateLimitError{APIError: APIError{Status: 429}}, true},
		{&APIError{Status: 503}, true},
		{&APIError{Status: 529}, true},
		{&APIError{Status: 408}, true},
		{&APIError{Status: 422}, false},
		{&APIError{Status: 401}, false},
		{&ConnectionError{Err: errors.New("refused")}, true},
		{&TimeoutError{ConnectionError: ConnectionError{Err: errors.New("t")}}, true},
		{&ValidationError{Path: "x", Err: errors.New("bad")}, false},
		{&ResponseValidationError{FieldPath: "answers.a", Err: errors.New("bad")}, false},
		{errors.New("other"), false},
	}
	for _, tt := range tests {
		if got := IsRetryable(tt.err); got != tt.want {
			t.Errorf("IsRetryable(%v) = %v want %v", tt.err, got, tt.want)
		}
	}
}

func TestResponseValidationErrorMessage(t *testing.T) {
	e := &ResponseValidationError{FieldPath: "answers.tone.confidence", Err: errors.New("missing")}
	if e.Error() != `typesafe: invalid response field "answers.tone.confidence": missing` {
		t.Fatalf("got %q", e.Error())
	}
}

func TestAPIErrorHeadersNil(t *testing.T) {
	e := &APIError{Status: 500}
	if e.Headers.Get("x") != "" {
		t.Fatal("nil headers must be safe")
	}
	_ = http.Header{}
}
