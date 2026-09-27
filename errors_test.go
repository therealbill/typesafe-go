package typesafe

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
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
		{"long detail message", `{"detail":{"error_type":"authentication_error","message":"` + strings.Repeat("x", 500) + `"}}`,
			("authentication_error: " + strings.Repeat("x", 500))[:200] + "..."},
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

func TestMessageTruncationIsRuneSafe(t *testing.T) {
	// A snowman is three bytes, so a 200-byte cut lands mid-rune.
	e := &APIError{Status: 500, Body: []byte(strings.Repeat("\u2603", 100))}
	got := e.Message()
	if !utf8.ValidString(got) {
		t.Fatalf("truncation split a rune: %q", got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("a truncated message should be marked: %q", got)
	}
	if len(got) > maxMessageLen+3 {
		t.Fatalf("truncated message is %d bytes", len(got))
	}
	if n := utf8.RuneCountInString(strings.TrimSuffix(got, "...")); n != 66 {
		t.Fatalf("expected 66 whole runes in 200 bytes, got %d", n)
	}
}

func TestMessageIsAlwaysValidUTF8(t *testing.T) {
	// A hostile or broken server can send bytes that are not UTF-8 at all.
	// Message() is printed and logged, so it must never carry them through.
	for _, body := range []string{
		"\xff\xfe broken",
		`{"detail":"` + "\xff\xfe" + `"}`,
		"\xed\xa0\x80",
		strings.Repeat("\xff", 300),
	} {
		got := (&APIError{Status: 500, Body: []byte(body)}).Message()
		if !utf8.ValidString(got) {
			t.Fatalf("Message() returned invalid UTF-8 for %q: %q", body, got)
		}
		if len(got) > maxMessageLen+3 {
			t.Fatalf("message is %d bytes", len(got))
		}
	}
}

func FuzzAPIErrorMessage(f *testing.F) {
	for _, seed := range []string{
		`{"detail":{"error_type":"authentication_error","message":"Cannot authenticate with the server. Please check your API key and try again."}}`,
		`{"detail":[{"type":"too_short","loc":["body","questions"],"msg":"Dictionary should have at least 1 item after validation, not 0","input":{},"ctx":{"field_type":"Dictionary","min_length":1,"actual_length":0}}]}`,
		`{"detail":{"error_type":"api_usage_error","message":"Invalid request."}}`,
		`{"detail":"Not Found"}`,
		"",
		"{",
		strings.Repeat("x", 1024),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body string) {
		e := &APIError{Status: 500, Endpoint: "POST /v1/systemone", Body: []byte(body)}
		got := e.Message()
		if len(got) > maxMessageLen+3 {
			t.Fatalf("message is %d bytes for input of %d: %q", len(got), len(body), got)
		}
		if !utf8.ValidString(got) {
			t.Fatalf("message is not valid UTF-8 for %q: %q", body, got)
		}
		// Error() embeds Message(), so it must stay printable too.
		if !utf8.ValidString(e.Error()) {
			t.Fatalf("Error() is not valid UTF-8 for %q", body)
		}
	})
}
