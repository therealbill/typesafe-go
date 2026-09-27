package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/therealbill/typesafe-go"
)

// Exit codes. A caller can branch on these without parsing output. The
// "kind" field of the error JSON names the case more precisely than the code:
// code 1 covers both kind "usage" and kind "internal".
const (
	ExitOK          = 0
	ExitUsage       = 1   // bad flags, unreadable or invalid request JSON, missing API key, or an unrecognized error
	ExitValidation  = 2   // request failed client-side validation
	ExitAuth        = 3   // 401 or 403
	ExitRequest     = 4   // other 4xx: 400, 404, 422
	ExitRateLimit   = 5   // 429 after retries
	ExitServer      = 6   // 5xx after retries, or an unreadable 2xx body
	ExitConnection  = 7   // connection failure or timeout
	ExitInterrupted = 130 // the context was cancelled, conventionally by SIGINT
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
	// An interrupt is checked first: a cancelled request surfaces as a
	// ConnectionError wrapping context.Canceled, which would otherwise be
	// reported as a transport failure.
	if errors.Is(err, context.Canceled) {
		return ExitInterrupted, "interrupted"
	}
	var ue *usageError
	if errors.As(err, &ue) || errors.Is(err, typesafe.ErrMissingAPIKey) {
		return ExitUsage, "usage"
	}
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
	return ExitUsage, "internal"
}

// sanitize replaces control characters with a printable escape so an error
// message echoed from a remote endpoint cannot drive the user's terminal.
// Tab is kept; every other character below 0x20, plus DEL, is escaped.
func sanitize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\t' || (r >= 0x20 && r != 0x7f) {
			b.WriteRune(r)
			continue
		}
		fmt.Fprintf(&b, "\\x%02x", r)
	}
	return b.String()
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
	_, _ = fmt.Fprintln(io.Err, "jev:", sanitize(err.Error()))
	return &ExitError{Code: code, Kind: kind, Err: err}
}
