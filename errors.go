package typesafe

import "fmt"

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
