package typesafe

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestDefaultRetryPolicy(t *testing.T) {
	p := DefaultRetryPolicy()
	if p.MaxRetries != 2 || p.InitialDelay != 500*time.Millisecond || p.MaxDelay != 5*time.Second || p.Jitter != 0.25 || p.Budget != 30*time.Second {
		t.Fatalf("unexpected defaults %+v", p)
	}
	if !p.RetryOnConnErr || !p.RetryOnTimeout || !p.HonorRetryAfter {
		t.Fatal("boolean defaults should be true")
	}
	if p.MaxRetryAfter != 5*time.Minute {
		t.Fatalf("MaxRetryAfter default should be 5m, got %s", p.MaxRetryAfter)
	}
	for _, s := range []int{408, 429, 500, 529, 599} {
		if !p.retryableStatus(s) {
			t.Errorf("%d should be retryable", s)
		}
	}
	for _, s := range []int{200, 400, 401, 404, 422, 499} {
		if p.retryableStatus(s) {
			t.Errorf("%d should not be retryable", s)
		}
	}
}

func TestRetryPolicyNormalized(t *testing.T) {
	p := RetryPolicy{MaxRetries: 3}.normalized()
	if p.InitialDelay != 500*time.Millisecond || p.MaxDelay != 5*time.Second || len(p.RetryStatuses) == 0 {
		t.Fatalf("normalized should fill zero values: %+v", p)
	}
	if p.MaxRetries != 3 {
		t.Fatal("normalized must keep explicit values")
	}
	q := RetryPolicy{MaxRetries: 1, InitialDelay: time.Second, MaxDelay: time.Millisecond}.normalized()
	if q.MaxDelay != time.Second {
		t.Fatalf("MaxDelay below InitialDelay should be raised, got %s", q.MaxDelay)
	}
}

func TestRetryDelayBackoff(t *testing.T) {
	p := DefaultRetryPolicy()
	p.Jitter = 0
	want := []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	for i, w := range want {
		if got := p.delay(i, nil, time.Now(), 0); got != w {
			t.Errorf("retry %d: got %s want %s", i, got, w)
		}
	}
	if got := p.delay(60, nil, time.Now(), 0); got != 5*time.Second {
		t.Errorf("large retry count must cap, got %s", got)
	}
}

func TestRetryDelayJitter(t *testing.T) {
	p := DefaultRetryPolicy()
	if got := p.delay(0, nil, time.Now(), 1); got != 375*time.Millisecond {
		t.Fatalf("full jitter should subtract 25%%, got %s", got)
	}
	if got := p.delay(0, nil, time.Now(), 0); got != 500*time.Millisecond {
		t.Fatalf("zero jitter draw should keep base, got %s", got)
	}
}

func TestRetryDelayHonorsRetryAfter(t *testing.T) {
	p := DefaultRetryPolicy()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	h := http.Header{}
	h.Set("retry-after-ms", "1500")
	if got := p.delay(0, h, now, 0); got != 1500*time.Millisecond {
		t.Fatalf("retry-after-ms: got %s", got)
	}
	h = http.Header{}
	h.Set("Retry-After", "3")
	if got := p.delay(0, h, now, 0); got != 3*time.Second {
		t.Fatalf("Retry-After seconds: got %s", got)
	}
	h = http.Header{}
	h.Set("Retry-After", now.Add(10*time.Second).Format(http.TimeFormat))
	if got := p.delay(0, h, now, 0); got != 10*time.Second {
		t.Fatalf("Retry-After date: got %s", got)
	}
	h = http.Header{}
	h.Set("Retry-After", now.Add(-10*time.Second).Format(http.TimeFormat))
	if got := p.delay(0, h, now, 0); got != 0 {
		t.Fatalf("past date should be zero, got %s", got)
	}
	h = http.Header{}
	h.Set("Retry-After", "garbage")
	if got := p.delay(0, h, now, 0); got != 500*time.Millisecond {
		t.Fatalf("unparseable header falls back to backoff, got %s", got)
	}
	h = http.Header{}
	h.Set("retry-after-ms", "1500")
	h.Set("Retry-After", "3")
	if got := p.delay(0, h, now, 0); got != 1500*time.Millisecond {
		t.Fatalf("retry-after-ms must win over Retry-After, got %s", got)
	}
	h = http.Header{}
	h.Set("retry-after-ms", "garbage")
	h.Set("Retry-After", "3")
	if got := p.delay(0, h, now, 0); got != 3*time.Second {
		t.Fatalf("unparseable retry-after-ms should fall back to Retry-After, got %s", got)
	}
	p.HonorRetryAfter = false
	h = http.Header{}
	h.Set("Retry-After", "3")
	if got := p.delay(0, h, now, 0); got != 500*time.Millisecond {
		t.Fatalf("disabled honor should ignore header, got %s", got)
	}
}

func TestRetryableDecisions(t *testing.T) {
	p := DefaultRetryPolicy()
	if !p.retryable(nil, &APIError{Status: 503}) || p.retryable(nil, &APIError{Status: 422}) {
		t.Fatal("status decisions wrong")
	}
	if !p.retryable(nil, &ConnectionError{Err: errors.New("x")}) {
		t.Fatal("connection errors retry by default")
	}
	p.RetryOnConnErr = false
	if p.retryable(nil, &ConnectionError{Err: errors.New("x")}) {
		t.Fatal("RetryOnConnErr=false must be respected")
	}
	if !p.retryable(nil, &TimeoutError{ConnectionError: ConnectionError{Err: errors.New("x")}}) {
		t.Fatal("timeouts still retry when RetryOnTimeout is true")
	}
	p.RetryOnTimeout = false
	if p.retryable(nil, &TimeoutError{ConnectionError: ConnectionError{Err: errors.New("x")}}) {
		t.Fatal("RetryOnTimeout=false must be respected")
	}
	calls := 0
	p.ShouldRetry = func(resp *http.Response, err error) bool { calls++; return true }
	if !p.retryable(nil, &APIError{Status: 422}) || calls != 1 {
		t.Fatal("ShouldRetry must override everything")
	}
}

func TestRetryDelayRejectsUnusableRetryAfter(t *testing.T) {
	p := DefaultRetryPolicy()
	p.Jitter = 0
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	// A value that is not a finite, non-negative number of seconds must be
	// ignored in favour of the normal backoff. Saturating it into a Duration
	// would overflow the retry budget check and block the call.
	for _, v := range []string{"inf", "Inf", "NaN", "1e300", "-5"} {
		t.Run(v, func(t *testing.T) {
			h := http.Header{}
			h.Set("Retry-After", v)
			if got := p.delay(0, h, now, 0); got != 500*time.Millisecond {
				t.Fatalf("Retry-After %q should fall back to backoff, got %s", v, got)
			}
		})
	}
}

func TestRetryDelayCapsServerRequestedWait(t *testing.T) {
	p := DefaultRetryPolicy()
	p.Jitter = 0
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

	h := http.Header{}
	h.Set("retry-after-ms", "999999999999999")
	if got := p.delay(0, h, now, 0); got != p.MaxRetryAfter {
		t.Fatalf("a retry-after-ms that overflows a Duration should clamp to MaxRetryAfter, got %s", got)
	}

	h = http.Header{}
	h.Set("Retry-After", "600")
	if got := p.delay(0, h, now, 0); got != 5*time.Minute {
		t.Fatalf("600s should clamp to the 5m default cap, got %s", got)
	}

	h = http.Header{}
	h.Set("Retry-After", "60")
	if got := p.delay(0, h, now, 0); got != time.Minute {
		t.Fatalf("a wait under the cap must be honored unchanged, got %s", got)
	}
}
