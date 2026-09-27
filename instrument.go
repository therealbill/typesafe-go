package typesafe

import (
	"context"
	"net/http"
)

// RequestInfo describes a call about to be made. It is passed to
// Instrumentation.RequestStart before the first HTTP attempt.
type RequestInfo struct {
	// Operation is "system_one" or "list_models".
	Operation string
	// Model is the requested model name or alias.
	Model string
	// QuestionCount and the per-type counts describe the questions map.
	QuestionCount int
	NoulCount     int
	ChoiceCount   int
	ScoreCount    int
	// State and Questions are the request inputs. Instrumentation must not
	// record them unless the caller opted in.
	State     any
	Questions Questions
}

// RequestResult describes how a call ended. It is passed to the function
// returned by Instrumentation.RequestStart exactly once.
type RequestResult struct {
	// Attempts is the number of HTTP attempts made, including the first.
	Attempts int
	// Status is the final HTTP status, or 0 if no response was received.
	Status int
	// RequestID is the x-typesafe-request-id header of the final response.
	RequestID string
	// Model is the model reported by the response, if any.
	Model string
	// Usage is the token usage reported by the response, if any.
	Usage Usage
	// Response is the decoded response for system_one, nil otherwise or on error.
	Response *SystemOneResponse
	// Err is the error returned to the caller, nil on success.
	Err error
}

// Instrumentation observes client calls. The otel subpackage provides an
// OpenTelemetry implementation.
type Instrumentation interface {
	// RequestStart is called once per SystemOne or ListModels call. The
	// returned context is used for every HTTP attempt. The returned function
	// is called exactly once when the call finishes.
	RequestStart(ctx context.Context, info RequestInfo) (context.Context, func(RequestResult))
	// Transport wraps the HTTP round tripper used by the client. It is called
	// once at client construction and may return rt unchanged.
	Transport(rt http.RoundTripper) http.RoundTripper
}
