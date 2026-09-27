package typesafe

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/therealbill/typesafe-go/internal/version"
)

const (
	headerRequestID  = "X-Typesafe-Request-Id"
	maxResponseBytes = 16 << 20
)

var userAgent = "typesafe-go/" + version.Version

// send performs one logical request with retries. It returns the final
// response, its body, the number of attempts made, and an error when the
// final attempt failed. resp is nil when no response was received.
func (c *Client) send(ctx context.Context, method, path string, body []byte, extra http.Header, policy RetryPolicy, timeout time.Duration) (*http.Response, []byte, int, error) {
	policy = policy.normalized()
	endpoint := method + " " + path
	start := time.Now()
	for retry := 0; ; retry++ {
		resp, respBody, err := c.attempt(ctx, method, path, body, extra, timeout)
		if err == nil && resp.StatusCode < 400 {
			c.logger.Debug("typesafe request ok", "endpoint", endpoint, "status", resp.StatusCode, "attempt", retry+1, "request_id", resp.Header.Get(headerRequestID), "elapsed", time.Since(start))
			return resp, respBody, retry + 1, nil
		}
		if err == nil {
			err = newAPIError(resp, respBody, endpoint)
		}
		if ctx.Err() != nil || retry >= policy.MaxRetries || !policy.retryable(resp, err) {
			// A 4xx is the caller's mistake and not an incident; reserve the
			// error level for failures the caller cannot fix.
			var api *APIError
			if errors.As(err, &api) && api.Status < 500 {
				c.logger.Warn("typesafe request failed", "endpoint", endpoint, "attempt", retry+1, "error", err)
			} else {
				c.logger.Error("typesafe request failed", "endpoint", endpoint, "attempt", retry+1, "error", err)
			}
			return resp, respBody, retry + 1, err
		}
		var hdr http.Header
		if resp != nil {
			hdr = resp.Header
		}
		delay := policy.delay(retry, hdr, time.Now(), c.random())
		if policy.Budget > 0 && delay > policy.Budget-time.Since(start) {
			c.logger.Warn("typesafe retry budget exhausted", "endpoint", endpoint, "attempt", retry+1, "error", err)
			return resp, respBody, retry + 1, err
		}
		c.logger.Warn("typesafe retrying", "endpoint", endpoint, "attempt", retry+1, "delay", delay, "error", err)
		if serr := sleep(ctx, delay); serr != nil {
			// Keep the last response so instrumentation still sees the
			// status and request id that prompted the abandoned retry.
			return resp, respBody, retry + 1, &ConnectionError{Err: serr}
		}
	}
}

// attempt performs a single HTTP request and reads the body. A non-2xx
// status is not an error here; send maps it.
func (c *Client) attempt(ctx context.Context, method, path string, body []byte, extra http.Header, timeout time.Duration) (*http.Response, []byte, error) {
	actx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		actx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(actx, method, c.baseURL.JoinPath(path).String(), rdr)
	if err != nil {
		return nil, nil, &ConnectionError{Err: err}
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	applyHeaders(req.Header, c.headers)
	applyHeaders(req.Header, extra)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, wrapTransportError(err, ctx, actx, timeout)
	}
	defer func() { _ = resp.Body.Close() }()
	// Read one byte past the cap so an oversized body is detected rather than
	// silently truncated into malformed JSON.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, nil, wrapTransportError(err, ctx, actx, timeout)
	}
	if len(data) > maxResponseBytes {
		return nil, nil, &ResponseValidationError{FieldPath: "", Err: errors.New("response exceeds 16 MiB")}
	}
	return resp, data, nil
}

// applyHeaders replaces each key in dst with the values from src.
func applyHeaders(dst, src http.Header) {
	for k, vs := range src {
		dst.Del(k)
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

func wrapTransportError(err error, parent, attemptCtx context.Context, timeout time.Duration) error {
	if parent.Err() != nil {
		return &ConnectionError{Err: parent.Err()}
	}
	if errors.Is(attemptCtx.Err(), context.DeadlineExceeded) || isTimeout(err) {
		return &TimeoutError{ConnectionError: ConnectionError{Err: err}, Timeout: timeout}
	}
	return &ConnectionError{Err: err}
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func newAPIError(resp *http.Response, body []byte, endpoint string) error {
	base := APIError{
		Status:    resp.StatusCode,
		Body:      body,
		Headers:   resp.Header.Clone(),
		Endpoint:  endpoint,
		RequestID: resp.Header.Get(headerRequestID),
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		d, _ := parseRetryAfter(resp.Header, time.Now())
		return &RateLimitError{APIError: base, RetryAfter: d}
	}
	return &base
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
