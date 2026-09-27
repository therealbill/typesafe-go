package typesafe

import (
	"errors"
	"net/http"
)

// RetryPolicy is defined fully in Task 5.
type RetryPolicy struct{ MaxRetries int }

func DefaultRetryPolicy() RetryPolicy { return RetryPolicy{MaxRetries: 2} }

func (p RetryPolicy) retryable(_ *http.Response, err error) bool {
	var api *APIError
	if errors.As(err, &api) {
		return api.Status == 408 || api.Status == 429 || api.Status >= 500
	}
	var ce *ConnectionError
	return errors.As(err, &ce)
}
