package cli

import (
	"errors"
	"fmt"

	"github.com/therealbill/typesafe-go"
)

// Exit codes. A caller can branch on these without parsing output.
const (
	ExitOK         = 0
	ExitUsage      = 1 // bad flags, unreadable or invalid request JSON, missing API key
	ExitValidation = 2 // request failed client-side validation
	ExitAuth       = 3 // 401 or 403
	ExitRequest    = 4 // other 4xx: 400, 404, 422
	ExitRateLimit  = 5 // 429 after retries
	ExitServer     = 6 // 5xx after retries, or an unreadable 2xx body
	ExitConnection = 7 // connection failure or timeout
)

// ExitError carries the process exit code for an error.
type ExitError struct {
	Code int
	Kind string
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

// usageError marks errors caused by how the command was invoked.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

func classify(err error) (int, string) {
	var ve *typesafe.ValidationError
	if errors.As(err, &ve) {
		return ExitValidation, "validation"
	}
	var rl *typesafe.RateLimitError
	if errors.As(err, &rl) {
		return ExitRateLimit, "rate_limit"
	}
	var api *typesafe.APIError
	if errors.As(err, &api) {
		switch {
		case api.Status == 401 || api.Status == 403:
			return ExitAuth, "auth"
		case api.Status >= 500:
			return ExitServer, "server"
		default:
			return ExitRequest, "request"
		}
	}
	var ce *typesafe.ConnectionError
	if errors.As(err, &ce) {
		return ExitConnection, "connection"
	}
	var rve *typesafe.ResponseValidationError
	if errors.As(err, &rve) {
		return ExitServer, "invalid_response"
	}
	return ExitUsage, "usage"
}

type errorPayload struct {
	Kind      string `json:"kind"`
	Status    int    `json:"status,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Message   string `json:"message"`
}

// fail reports err on both streams and returns the ExitError to propagate.
func fail(io IO, pretty bool, err error) error {
	code, kind := classify(err)
	p := errorPayload{Kind: kind, Message: err.Error()}
	var api *typesafe.APIError
	if errors.As(err, &api) {
		p.Status = api.Status
		p.RequestID = api.RequestID
		if msg := api.Message(); msg != "" {
			p.Message = msg
		}
	}
	_ = writeJSON(io.Out, map[string]any{"error": p}, pretty)
	fmt.Fprintln(io.Err, "jev:", err)
	return &ExitError{Code: code, Kind: kind, Err: err}
}
