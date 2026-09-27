package typesafe

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrMissingAPIKey is returned by NewClient when no API key is configured.
var ErrMissingAPIKey = errors.New("typesafe: API key is required: pass WithAPIKey or set TYPESAFE_API_KEY")

// ValidationError reports a request that failed client-side validation
// before any network call. Path names the offending field, for example
// "questions.tone.criteria".
type ValidationError struct {
	Path string
	Err  error
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("typesafe: invalid request: %s: %v", e.Path, e.Err)
}

func (e *ValidationError) Unwrap() error { return e.Err }

// APIError is returned when the API responds with a 4xx or 5xx status.
type APIError struct {
	// Status is the HTTP status code.
	Status int
	// Body is the raw response body.
	Body []byte
	// Headers are the response headers.
	Headers http.Header
	// Endpoint is the method and path, for example "POST /v1/systemone".
	Endpoint string
	// RequestID is the x-typesafe-request-id response header, if present.
	RequestID string
}

func (e *APIError) Error() string {
	s := fmt.Sprintf("typesafe: %s returned %d", e.Endpoint, e.Status)
	if msg := e.Message(); msg != "" {
		s += ": " + msg
	}
	if e.RequestID != "" {
		s += " (request id " + e.RequestID + ")"
	}
	return s
}

const maxMessageLen = 200

// Message extracts a human-readable message from the body. It understands the
// API's "detail" envelope in its string, object, and array forms, falls back
// to "message" or "error" fields, and otherwise returns the trimmed body,
// truncated to 200 bytes.
func (e *APIError) Message() string {
	var env struct {
		Detail  json.RawMessage `json:"detail"`
		Message string          `json:"message"`
		Error   string          `json:"error"`
	}
	var msg string
	if json.Unmarshal(e.Body, &env) == nil {
		if len(env.Detail) > 0 {
			msg = detailMessage(env.Detail)
		}
		if msg == "" {
			msg = env.Message
		}
		if msg == "" {
			msg = env.Error
		}
	}
	if msg == "" {
		msg = strings.TrimSpace(string(e.Body))
	}
	return truncate(msg, maxMessageLen)
}

// truncate shortens s to at most n bytes without splitting a rune, marking the
// cut when one was made.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}

func detailMessage(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj struct {
		ErrorType string `json:"error_type"`
		Message   string `json:"message"`
	}
	if json.Unmarshal(raw, &obj) == nil && obj.Message != "" {
		if obj.ErrorType != "" {
			return obj.ErrorType + ": " + obj.Message
		}
		return obj.Message
	}
	var items []struct {
		Loc []any  `json:"loc"`
		Msg string `json:"msg"`
	}
	if json.Unmarshal(raw, &items) == nil && len(items) > 0 {
		parts := make([]string, 0, len(items))
		for _, it := range items {
			loc := make([]string, 0, len(it.Loc))
			for _, l := range it.Loc {
				loc = append(loc, fmt.Sprint(l))
			}
			if len(loc) > 0 {
				parts = append(parts, strings.Join(loc, ".")+": "+it.Msg)
			} else {
				parts = append(parts, it.Msg)
			}
		}
		return strings.Join(parts, "; ")
	}
	return ""
}

// RateLimitError is an APIError with status 429. RetryAfter is the wait the
// server requested through Retry-After or retry-after-ms, or zero.
type RateLimitError struct {
	APIError
	RetryAfter time.Duration
}

func (e *RateLimitError) Unwrap() error { return &e.APIError }

// ConnectionError wraps a transport failure: DNS, dial, TLS, a reset
// connection, or a cancelled context.
type ConnectionError struct {
	Err error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("typesafe: connection error: %v", e.Err)
}

func (e *ConnectionError) Unwrap() error { return e.Err }

// TimeoutError is a ConnectionError caused by the per-request timeout.
type TimeoutError struct {
	ConnectionError
	Timeout time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("typesafe: request timed out after %s: %v", e.Timeout, e.Err)
}

func (e *TimeoutError) Unwrap() error { return &e.ConnectionError }

// ResponseValidationError reports a 2xx response whose body did not match
// the expected shape. FieldPath is dotted, for example
// "answers.tone.confidence".
type ResponseValidationError struct {
	FieldPath string
	Err       error
}

func (e *ResponseValidationError) Error() string {
	return fmt.Sprintf("typesafe: invalid response field %q: %v", e.FieldPath, e.Err)
}

func (e *ResponseValidationError) Unwrap() error { return e.Err }

// IsAuthError reports whether err is an APIError with status 401 or 403.
func IsAuthError(err error) bool {
	var api *APIError
	return errors.As(err, &api) && (api.Status == http.StatusUnauthorized || api.Status == http.StatusForbidden)
}

// IsRateLimited reports whether err is a RateLimitError.
func IsRateLimited(err error) bool {
	var rl *RateLimitError
	return errors.As(err, &rl)
}

// IsRetryable reports whether the default RetryPolicy would retry err.
func IsRetryable(err error) bool {
	return DefaultRetryPolicy().retryable(nil, err)
}
