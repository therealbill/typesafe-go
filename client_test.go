package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type step struct {
	status  int
	body    string
	headers map[string]string
	delay   time.Duration
}

type fakeServer struct {
	*httptest.Server
	mu       sync.Mutex
	steps    []step
	calls    int
	requests []*http.Request
	bodies   [][]byte
}

func newFakeServer(t *testing.T, steps ...step) *fakeServer {
	t.Helper()
	fs := &fakeServer{steps: steps}
	fs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fs.mu.Lock()
		i := fs.calls
		fs.calls++
		fs.requests = append(fs.requests, r.Clone(context.Background()))
		fs.bodies = append(fs.bodies, body)
		fs.mu.Unlock()
		if i >= len(fs.steps) {
			i = len(fs.steps) - 1
		}
		s := fs.steps[i]
		if s.delay > 0 {
			time.Sleep(s.delay)
		}
		for k, v := range s.headers {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(s.status)
		_, _ = w.Write([]byte(s.body))
	}))
	t.Cleanup(fs.Close)
	return fs
}

func (fs *fakeServer) callCount() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.calls
}

func fastPolicy() RetryPolicy {
	p := DefaultRetryPolicy()
	p.InitialDelay = time.Millisecond
	p.MaxDelay = 2 * time.Millisecond
	p.Jitter = 0
	p.Budget = 2 * time.Second
	return p
}

func newTestClient(t *testing.T, url string, opts ...Option) *Client {
	t.Helper()
	base := []Option{WithAPIKey("test-key"), WithBaseURL(url), WithRetryPolicy(fastPolicy())}
	c, err := NewClient(append(base, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var fixtureQuestions = Questions{
	"billing": Noul{Instructions: "Is `ticket` about a billing problem?"},
	"tone":    Choice{Instructions: "What is the tone of `ticket`?", Criteria: map[string]JSONContent{"calm": "Polite and patient", "angry": "Frustrated or hostile", "neutral": nil}},
	"urgency": Score{Instructions: "How urgent is `ticket`?", Criteria: []JSONContent{"Not urgent at all", "Somewhat urgent", "Very urgent"}},
}

var fixtureState = map[string]any{"ticket": "I was charged twice this month and nobody answers my emails. Fix it now."}

func okStep(t *testing.T) step {
	return step{status: 200, body: string(mustRead(t, "testdata/systemone_ok.json")), headers: map[string]string{"x-typesafe-request-id": "req_01a0d07208027e0bbeb09c9f9b2b205a"}}
}

func TestSystemOneSuccess(t *testing.T) {
	fs := newFakeServer(t, okStep(t))
	c := newTestClient(t, fs.URL)
	res, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	if err != nil {
		t.Fatal(err)
	}
	req := fs.requests[0]
	if req.Method != http.MethodPost || req.URL.Path != "/v1/systemone" {
		t.Fatalf("bad request line %s %s", req.Method, req.URL.Path)
	}
	if req.Header.Get("Authorization") != "Bearer test-key" || req.Header.Get("Content-Type") != "application/json" || !strings.HasPrefix(req.Header.Get("User-Agent"), "typesafe-go/") {
		t.Fatalf("headers %v", req.Header)
	}
	var got, want map[string]any
	if err := json.Unmarshal(fs.bodies[0], &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mustRead(t, "testdata/request.json"), &want); err != nil {
		t.Fatal(err)
	}
	gotB, _ := json.Marshal(got)
	wantB, _ := json.Marshal(want)
	if !bytes.Equal(gotB, wantB) {
		t.Fatalf("body\n got %s\nwant %s", gotB, wantB)
	}
	if res.Model != "jev-1.13.0" || res.RequestID != "req_01a0d07208027e0bbeb09c9f9b2b205a" || res.Raw.Status != 200 {
		t.Fatalf("response meta %+v", res)
	}
	if res.Nouls()["billing"].Noul != 0.99 || res.Choices()["tone"].Choice != "angry" || res.Scores()["urgency"].Score != 1.9 {
		t.Fatalf("answers %+v", res.Answers)
	}
	if *res.Usage.InputTokens != 407 {
		t.Fatalf("usage %+v", res.Usage)
	}
}

func TestSystemOneRetriesOn429ThenSucceeds(t *testing.T) {
	fs := newFakeServer(t,
		step{status: 429, body: `{"detail":"slow down"}`, headers: map[string]string{"retry-after-ms": "5"}},
		okStep(t))
	c := newTestClient(t, fs.URL)
	start := time.Now()
	res, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	if err != nil {
		t.Fatal(err)
	}
	if fs.callCount() != 2 {
		t.Fatalf("calls %d", fs.callCount())
	}
	if time.Since(start) < 5*time.Millisecond {
		t.Fatal("retry-after-ms was not honored")
	}
	if !bytes.Equal(fs.bodies[0], fs.bodies[1]) {
		t.Fatal("request body must be replayed identically")
	}
	if res.Model != "jev-1.13.0" {
		t.Fatal("wrong response")
	}
}

func TestSystemOneRetriesOn529(t *testing.T) {
	fs := newFakeServer(t, step{status: 529, body: `overloaded`}, okStep(t))
	c := newTestClient(t, fs.URL)
	if _, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions); err != nil {
		t.Fatal(err)
	}
	if fs.callCount() != 2 {
		t.Fatalf("calls %d", fs.callCount())
	}
}

func TestSystemOneRetriesExhausted(t *testing.T) {
	fs := newFakeServer(t, step{status: 503, body: `{"detail":"down"}`, headers: map[string]string{"x-typesafe-request-id": "req_x"}})
	c := newTestClient(t, fs.URL)
	_, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	var api *APIError
	if !errors.As(err, &api) || api.Status != 503 || api.RequestID != "req_x" || api.Endpoint != "POST /v1/systemone" {
		t.Fatalf("err %v", err)
	}
	if fs.callCount() != 3 {
		t.Fatalf("calls %d want 3", fs.callCount())
	}
	if !IsRetryable(err) {
		t.Fatal("503 is retryable")
	}
}

func TestSystemOneNoRetryOn422(t *testing.T) {
	fs := newFakeServer(t, step{status: 422, body: `{"detail":[{"type":"too_short","loc":["body","questions"],"msg":"Dictionary should have at least 1 item after validation, not 0","input":{}}]}`})
	c := newTestClient(t, fs.URL)
	_, err := c.SystemOne(context.Background(), fixtureState, Questions{"q": RawQuestion{"type": "noul"}})
	var api *APIError
	if !errors.As(err, &api) || api.Status != 422 {
		t.Fatalf("err %v", err)
	}
	if !strings.Contains(api.Message(), "body.questions") {
		t.Fatalf("message %q", api.Message())
	}
	if fs.callCount() != 1 {
		t.Fatalf("calls %d", fs.callCount())
	}
}

func TestSystemOneRateLimitError(t *testing.T) {
	fs := newFakeServer(t, step{status: 429, body: `{"detail":"slow"}`, headers: map[string]string{"Retry-After": "0"}})
	c := newTestClient(t, fs.URL)
	_, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	var rl *RateLimitError
	if !errors.As(err, &rl) || rl.Status != 429 || rl.RetryAfter != 0 {
		t.Fatalf("err %v", err)
	}
	if !IsRateLimited(err) {
		t.Fatal("IsRateLimited")
	}
	if fs.callCount() != 3 {
		t.Fatalf("calls %d", fs.callCount())
	}
}

func TestSystemOneBudgetExhausted(t *testing.T) {
	fs := newFakeServer(t, step{status: 503, body: `down`})
	p := fastPolicy()
	p.InitialDelay = 50 * time.Millisecond
	p.MaxDelay = 50 * time.Millisecond
	p.Budget = 10 * time.Millisecond
	c := newTestClient(t, fs.URL, WithRetryPolicy(p))
	_, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	var api *APIError
	if !errors.As(err, &api) || api.Status != 503 {
		t.Fatalf("err %v", err)
	}
	if fs.callCount() != 1 {
		t.Fatalf("budget should stop before the first retry, calls %d", fs.callCount())
	}
}

func TestSystemOneContextCancelledDuringBackoff(t *testing.T) {
	fs := newFakeServer(t, step{status: 503, body: `down`})
	p := fastPolicy()
	p.InitialDelay = time.Second
	p.MaxDelay = time.Second
	c := newTestClient(t, fs.URL, WithRetryPolicy(p))
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err := c.SystemOne(ctx, fixtureState, fixtureQuestions)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v", err)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("cancellation did not interrupt the backoff sleep")
	}
	if fs.callCount() != 1 {
		t.Fatalf("calls %d", fs.callCount())
	}
}

func TestSystemOneTimeout(t *testing.T) {
	fs := newFakeServer(t, step{status: 200, body: `{}`, delay: 200 * time.Millisecond})
	p := fastPolicy()
	p.MaxRetries = 0
	c := newTestClient(t, fs.URL, WithRetryPolicy(p), WithTimeout(20*time.Millisecond))
	_, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	var te *TimeoutError
	if !errors.As(err, &te) || te.Timeout != 20*time.Millisecond {
		t.Fatalf("err %v", err)
	}
	if !IsRetryable(err) {
		t.Fatal("timeouts are retryable by default")
	}
}

func TestSystemOneConnectionError(t *testing.T) {
	fs := newFakeServer(t, okStep(t))
	url := fs.URL
	fs.Close()
	p := fastPolicy()
	p.MaxRetries = 1
	c := newTestClient(t, url, WithRetryPolicy(p))
	_, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	var ce *ConnectionError
	if !errors.As(err, &ce) {
		t.Fatalf("err %v", err)
	}
	var te *TimeoutError
	if errors.As(err, &te) {
		t.Fatal("a refused connection is not a timeout")
	}
}

func TestSystemOneValidatesBeforeNetwork(t *testing.T) {
	fs := newFakeServer(t, okStep(t))
	c := newTestClient(t, fs.URL)
	_, err := c.SystemOne(context.Background(), fixtureState, Questions{})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err %v", err)
	}
	_, err = c.SystemOne(context.Background(), nil, fixtureQuestions)
	if !errors.As(err, &ve) || ve.Path != "state" {
		t.Fatalf("err %v", err)
	}
	if fs.callCount() != 0 {
		t.Fatal("no request should be sent")
	}
}

func TestSystemOneRequestOptions(t *testing.T) {
	fs := newFakeServer(t, okStep(t))
	c := newTestClient(t, fs.URL, WithHeaders(http.Header{"X-Client": {"a"}}))
	_, err := c.SystemOne(context.Background(), "state", fixtureQuestions,
		WithRequestModel("jev-preview"),
		WithExtraHeaders(http.Header{"X-Req": {"b"}, "X-Client": {"override"}}),
		WithExtraBody(map[string]any{"weight": 2}))
	if err != nil {
		t.Fatal(err)
	}
	req := fs.requests[0]
	if req.Header.Get("X-Req") != "b" || req.Header.Get("X-Client") != "override" {
		t.Fatalf("headers %v", req.Header)
	}
	var body map[string]any
	_ = json.Unmarshal(fs.bodies[0], &body)
	if body["model"] != "jev-preview" || body["weight"] != float64(2) || body["state"] != "state" {
		t.Fatalf("body %v", body)
	}
}

func TestSystemOneUnknownAnswerType(t *testing.T) {
	fs := newFakeServer(t, step{status: 200, body: `{"model":"m","answers":{"v":{"type":"vibe","x":1}},"usage":{"input_tokens":1,"output_tokens":1}}`})
	c := newTestClient(t, fs.URL)
	res, err := c.SystemOne(context.Background(), "s", fixtureQuestions)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := res.Answers["v"].(UnknownAnswer); !ok {
		t.Fatalf("got %#v", res.Answers["v"])
	}
}

func TestSystemOneInvalidResponse(t *testing.T) {
	fs := newFakeServer(t, step{status: 200, body: `{"model":"m","answers":{"tone":{"type":"choice","choice":"x","confidence":"high","probabilities":{}}}}`})
	c := newTestClient(t, fs.URL)
	_, err := c.SystemOne(context.Background(), "s", fixtureQuestions)
	var rve *ResponseValidationError
	if !errors.As(err, &rve) || rve.FieldPath != "answers.tone.confidence" {
		t.Fatalf("err %v", err)
	}
}

func TestListModels(t *testing.T) {
	fs := newFakeServer(t, step{status: 200, body: string(mustRead(t, "testdata/models_ok.json")), headers: map[string]string{"x-typesafe-request-id": "req_m"}})
	c := newTestClient(t, fs.URL)
	res, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	req := fs.requests[0]
	if req.Method != http.MethodGet || req.URL.Path != "/v1/models" || req.Header.Get("Content-Type") != "" {
		t.Fatalf("request %s %s %v", req.Method, req.URL.Path, req.Header)
	}
	if len(res.Models) != 2 || res.RequestID != "req_m" || res.Raw.Status != 200 {
		t.Fatalf("res %+v", res)
	}
}

func TestNewClientMissingKey(t *testing.T) {
	t.Setenv(EnvAPIKey, "")
	_, err := NewClient()
	if !errors.Is(err, ErrMissingAPIKey) {
		t.Fatalf("err %v", err)
	}
}

func TestNewClientEnvAndDefaults(t *testing.T) {
	t.Setenv(EnvAPIKey, "env-key")
	t.Setenv(EnvBaseURL, "https://example.test/")
	t.Setenv(EnvDefaultModel, "jev-preview")
	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	if c.apiKey != "env-key" || c.baseURL != "https://example.test" || c.model != "jev-preview" || c.timeout != DefaultTimeout {
		t.Fatalf("client %+v", c)
	}
	t.Setenv(EnvBaseURL, "")
	t.Setenv(EnvDefaultModel, "")
	c, err = NewClient(WithAPIKey("k"))
	if err != nil {
		t.Fatal(err)
	}
	if c.baseURL != DefaultBaseURL || c.model != DefaultModel {
		t.Fatalf("defaults %+v", c)
	}
	if _, err := NewClient(WithAPIKey("k"), WithBaseURL("://bad")); err == nil {
		t.Fatal("bad base URL should fail")
	}
}

type fakeInstrumentation struct {
	transportCalls int
	infos          []RequestInfo
	results        []RequestResult
}

func (f *fakeInstrumentation) RequestStart(ctx context.Context, info RequestInfo) (context.Context, func(RequestResult)) {
	f.infos = append(f.infos, info)
	return context.WithValue(ctx, ctxKey{}, "set"), func(r RequestResult) { f.results = append(f.results, r) }
}

func (f *fakeInstrumentation) Transport(rt http.RoundTripper) http.RoundTripper {
	f.transportCalls++
	return rt
}

type ctxKey struct{}

func TestInstrumentationHook(t *testing.T) {
	fs := newFakeServer(t, step{status: 503, body: "x"}, okStep(t))
	fi := &fakeInstrumentation{}
	c := newTestClient(t, fs.URL, WithInstrumentation(fi))
	if fi.transportCalls != 1 {
		t.Fatalf("Transport called %d times", fi.transportCalls)
	}
	res, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
	if err != nil {
		t.Fatal(err)
	}
	info := fi.infos[0]
	if info.Operation != "system_one" || info.Model != DefaultModel || info.QuestionCount != 3 || info.NoulCount != 1 || info.ChoiceCount != 1 || info.ScoreCount != 1 {
		t.Fatalf("info %+v", info)
	}
	r := fi.results[0]
	if r.Attempts != 2 || r.Status != 200 || r.RequestID != res.RequestID || r.Model != "jev-1.13.0" || r.Response != res || r.Err != nil {
		t.Fatalf("result %+v", r)
	}
	fs2 := newFakeServer(t, step{status: 401, body: `{"detail":"no"}`})
	c2 := newTestClient(t, fs2.URL, WithInstrumentation(fi))
	_, err = c2.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	r = fi.results[1]
	if fi.infos[1].Operation != "list_models" || r.Status != 401 || r.Err == nil || r.Attempts != 1 {
		t.Fatalf("result %+v", r)
	}
}

func TestWithHTTPClientDoesNotMutateCaller(t *testing.T) {
	fs := newFakeServer(t, okStep(t))
	hc := &http.Client{}
	fi := &fakeInstrumentation{}
	c := newTestClient(t, fs.URL, WithHTTPClient(hc), WithInstrumentation(fi))
	if hc.Transport != nil {
		t.Fatal("caller's client was mutated")
	}
	if c.httpClient == hc {
		t.Fatal("client should hold a copy")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLoggingRedactsAuthorization(t *testing.T) {
	fs := newFakeServer(t, step{status: 503, body: "x"}, okStep(t))
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	c := newTestClient(t, fs.URL, WithLogger(logger))
	if _, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "retrying") || !strings.Contains(out, "request ok") {
		t.Fatalf("expected retry and success logs, got %s", out)
	}
	if strings.Contains(out, "test-key") || strings.Contains(out, "Bearer") || strings.Contains(out, "ticket") {
		t.Fatalf("log leaked secrets or body: %s", out)
	}
}

func TestNewLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf, "warning")
	l.Info("hidden")
	l.Warn("shown")
	if strings.Contains(buf.String(), "hidden") || !strings.Contains(buf.String(), "shown") {
		t.Fatalf("level filtering wrong: %s", buf.String())
	}
	buf.Reset()
	NewLogger(&buf, "off").Error("nothing")
	if buf.Len() != 0 {
		t.Fatal("off must discard")
	}
}

func TestClientConcurrentUse(t *testing.T) {
	fs := newFakeServer(t, okStep(t))
	c := newTestClient(t, fs.URL)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

func TestSystemOneNonFiniteRetryAfterDoesNotHang(t *testing.T) {
	fs := newFakeServer(t, step{status: 503, body: `down`, headers: map[string]string{"Retry-After": "inf"}})
	p := fastPolicy()
	p.Budget = 50 * time.Millisecond
	c := newTestClient(t, fs.URL, WithRetryPolicy(p))
	done := make(chan error, 1)
	go func() {
		_, err := c.SystemOne(context.Background(), fixtureState, fixtureQuestions)
		done <- err
	}()
	select {
	case err := <-done:
		var api *APIError
		if !errors.As(err, &api) || api.Status != 503 {
			t.Fatalf("err %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SystemOne did not return within 2s: a non-finite Retry-After hung the retry loop")
	}
}
