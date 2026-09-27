package typesafe

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryPolicy controls how failed requests are retried. Start from
// DefaultRetryPolicy and change fields; a zero RetryPolicy disables retries.
type RetryPolicy struct {
	// MaxRetries is the number of retries after the first attempt. 0 disables retries.
	MaxRetries int
	// InitialDelay is the delay before the first retry. Default 500ms.
	InitialDelay time.Duration
	// MaxDelay caps the exponential backoff. Default 5s.
	MaxDelay time.Duration
	// Jitter is the fraction of each delay randomly subtracted, from 0 to 1. Default 0.25.
	Jitter float64
	// Budget is the total time allowed per call including delays. 0 means unlimited. Default 30s.
	Budget time.Duration
	// RetryStatuses lists HTTP statuses that are retried. Default 408, 429, 500–599.
	RetryStatuses []int
	// RetryOnConnErr retries connection failures. Default true.
	RetryOnConnErr bool
	// RetryOnTimeout retries per-request timeouts. Default true.
	RetryOnTimeout bool
	// HonorRetryAfter uses Retry-After and retry-after-ms headers as the delay. Default true.
	HonorRetryAfter bool
	// MaxRetryAfter caps a server-requested wait from Retry-After or retry-after-ms. Default 5m.
	MaxRetryAfter time.Duration
	// ShouldRetry, when set, replaces every other decision. resp may be nil.
	ShouldRetry func(resp *http.Response, err error) bool
}

// DefaultRetryPolicy returns the policy used when none is configured. It
// matches the official Python SDK's defaults.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:      2,
		InitialDelay:    500 * time.Millisecond,
		MaxDelay:        5 * time.Second,
		Jitter:          0.25,
		Budget:          30 * time.Second,
		RetryStatuses:   defaultRetryStatuses(),
		RetryOnConnErr:  true,
		RetryOnTimeout:  true,
		HonorRetryAfter: true,
		MaxRetryAfter:   5 * time.Minute,
	}
}

func defaultRetryStatuses() []int {
	s := []int{http.StatusRequestTimeout, http.StatusTooManyRequests}
	for code := 500; code <= 599; code++ {
		s = append(s, code)
	}
	return s
}

// normalized fills zero delay and status fields with defaults so a policy
// built by hand still behaves sensibly.
func (p RetryPolicy) normalized() RetryPolicy {
	d := DefaultRetryPolicy()
	if p.InitialDelay <= 0 {
		p.InitialDelay = d.InitialDelay
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = d.MaxDelay
	}
	if p.MaxDelay < p.InitialDelay {
		p.MaxDelay = p.InitialDelay
	}
	if p.Jitter < 0 {
		p.Jitter = 0
	}
	if p.Jitter > 1 {
		p.Jitter = 1
	}
	if p.MaxRetryAfter <= 0 {
		p.MaxRetryAfter = d.MaxRetryAfter
	}
	if p.RetryStatuses == nil {
		p.RetryStatuses = d.RetryStatuses
	}
	return p
}

func (p RetryPolicy) retryableStatus(code int) bool {
	for _, s := range p.RetryStatuses {
		if s == code {
			return true
		}
	}
	return false
}

// retryable decides whether a failed attempt should be retried.
func (p RetryPolicy) retryable(resp *http.Response, err error) bool {
	if p.ShouldRetry != nil {
		return p.ShouldRetry(resp, err)
	}
	var api *APIError
	if errors.As(err, &api) {
		return p.retryableStatus(api.Status)
	}
	var te *TimeoutError
	if errors.As(err, &te) {
		return p.RetryOnTimeout
	}
	var ce *ConnectionError
	if errors.As(err, &ce) {
		return p.RetryOnConnErr
	}
	return false
}

// delay computes the wait before retry number retry (0-based). hdr may be
// nil. random is a draw in [0, 1) used for jitter.
func (p RetryPolicy) delay(retry int, hdr http.Header, now time.Time, random float64) time.Duration {
	if p.HonorRetryAfter && hdr != nil {
		if d, ok := parseRetryAfter(hdr, now); ok {
			if p.MaxRetryAfter > 0 && d > p.MaxRetryAfter {
				d = p.MaxRetryAfter
			}
			return d
		}
	}
	if retry > 30 {
		retry = 30
	}
	d := p.InitialDelay << uint(retry)
	if d <= 0 || d > p.MaxDelay {
		d = p.MaxDelay
	}
	if p.Jitter > 0 {
		d -= time.Duration(float64(d) * p.Jitter * random)
	}
	return d
}

// maxRetryAfterMillis is the largest retry-after-ms value that fits in a
// time.Duration without overflowing.
const maxRetryAfterMillis = int64(math.MaxInt64) / int64(time.Millisecond)

// parseRetryAfter reads retry-after-ms (milliseconds) or Retry-After
// (seconds or an HTTP date). It returns false when neither is usable.
//
// A value that cannot be represented as a time.Duration is never converted
// directly: saturating or wrapping it would produce a nonsense delay that
// overflows the caller's budget arithmetic. A wait too large to represent is
// reported as the maximum Duration so the caller clamps it to MaxRetryAfter,
// while a wait that is not a finite, non-negative number of seconds is
// reported as unusable so the caller falls back to its own backoff.
func parseRetryAfter(h http.Header, now time.Time) (time.Duration, bool) {
	if v := strings.TrimSpace(h.Get("retry-after-ms")); v != "" {
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil && ms >= 0 {
			if ms > maxRetryAfterMillis {
				return time.Duration(math.MaxInt64), true
			}
			return time.Duration(ms) * time.Millisecond, true
		}
	}
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.ParseFloat(v, 64); err == nil {
		ns := secs * float64(time.Second)
		if !math.IsNaN(ns) && !math.IsInf(ns, 0) && ns >= 0 && ns <= float64(math.MaxInt64) {
			return time.Duration(ns), true
		}
	}
	if t, err := http.ParseTime(v); err == nil {
		d := t.Sub(now)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}
