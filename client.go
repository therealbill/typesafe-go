package typesafe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Defaults and environment variable names. Explicit options win over the
// environment, which wins over these defaults.
const (
	DefaultBaseURL = "https://api.typesafe.ai"
	DefaultModel   = "jev-latest"
	DefaultTimeout = 10 * time.Second

	EnvAPIKey       = "TYPESAFE_API_KEY"
	EnvBaseURL      = "TYPESAFE_BASE_URL"
	EnvDefaultModel = "TYPESAFE_DEFAULT_MODEL"
	EnvLogLevel     = "TYPESAFE_LOG_LEVEL"
)

// Client calls the TypeSafe API. It is safe for concurrent use. The zero
// value is not usable; build one with NewClient.
type Client struct {
	apiKey     string
	baseURL    *url.URL
	model      string
	retry      RetryPolicy
	timeout    time.Duration
	headers    http.Header
	httpClient *http.Client
	ownsHTTP   bool
	instr      Instrumentation
	logger     *slog.Logger
	random     func() float64
}

// Option configures a Client.
type Option func(*Client) error

// WithAPIKey sets the API key, ignoring surrounding whitespace. Otherwise
// TYPESAFE_API_KEY is used. An empty or whitespace-only key is treated as
// unset and the environment is consulted.
func WithAPIKey(key string) Option {
	return func(c *Client) error { c.apiKey = strings.TrimSpace(key); return nil }
}

// WithBaseURL sets the API root, for example for a gateway. Otherwise
// TYPESAFE_BASE_URL or https://api.typesafe.ai is used.
func WithBaseURL(u string) Option {
	return func(c *Client) error {
		v, err := normalizeBaseURL(u)
		if err != nil {
			return err
		}
		c.baseURL = v
		return nil
	}
}

// WithModel sets the default model. Otherwise TYPESAFE_DEFAULT_MODEL or
// jev-latest is used.
func WithModel(m string) Option {
	return func(c *Client) error { c.model = m; return nil }
}

// WithRetryPolicy replaces the default retry policy.
func WithRetryPolicy(p RetryPolicy) Option {
	return func(c *Client) error { c.retry = p; return nil }
}

// WithTimeout sets the timeout for each HTTP attempt. Default 10s. Zero
// disables the per-attempt timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) error {
		if d < 0 {
			return fmt.Errorf("typesafe: timeout must not be negative")
		}
		c.timeout = d
		return nil
	}
}

// WithHeaders adds headers to every request. A header given here replaces the
// one the client would send, so supplying Authorization replaces the API key
// header, which some gateways require.
func WithHeaders(h http.Header) Option {
	// Clone now, not when the option is applied, so a caller that keeps
	// writing to its header after building the option cannot change requests.
	cp := h.Clone()
	return func(c *Client) error { c.headers = cp; return nil }
}

// WithHTTPClient uses a caller-supplied http.Client. The client is copied so
// the caller's value is not modified when instrumentation wraps its transport.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) error {
		if hc == nil {
			return fmt.Errorf("typesafe: http client must not be nil")
		}
		cp := *hc
		c.httpClient = &cp
		c.ownsHTTP = false
		return nil
	}
}

// WithInstrumentation attaches an observer, such as the otel subpackage's.
func WithInstrumentation(i Instrumentation) Option {
	return func(c *Client) error { c.instr = i; return nil }
}

// WithLogger sets the logger. Otherwise TYPESAFE_LOG_LEVEL selects a text
// logger on stderr, and unset means no logging.
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) error { c.logger = l; return nil }
}

// NewClient builds a Client. It fails when no API key is configured.
func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		retry:   DefaultRetryPolicy(),
		timeout: DefaultTimeout,
		headers: http.Header{},
		random:  rand.Float64,
	}
	for _, o := range opts {
		if err := o(c); err != nil {
			return nil, err
		}
	}
	if c.apiKey == "" {
		c.apiKey = strings.TrimSpace(os.Getenv(EnvAPIKey))
	}
	if c.apiKey == "" {
		return nil, ErrMissingAPIKey
	}
	if c.baseURL == nil {
		raw := os.Getenv(EnvBaseURL)
		if raw == "" {
			raw = DefaultBaseURL
		}
		v, err := normalizeBaseURL(raw)
		if err != nil {
			return nil, err
		}
		c.baseURL = v
	}
	if c.model == "" {
		c.model = os.Getenv(EnvDefaultModel)
	}
	if c.model == "" {
		c.model = DefaultModel
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{}
		c.ownsHTTP = true
	}
	if c.instr != nil {
		rt := c.httpClient.Transport
		if rt == nil {
			rt = http.DefaultTransport
		}
		c.httpClient.Transport = c.instr.Transport(rt)
	}
	if c.logger == nil {
		c.logger = NewLogger(os.Stderr, os.Getenv(EnvLogLevel))
	}
	return c, nil
}

// normalizeBaseURL parses the API root. Only http and https are accepted, and
// a query, fragment, or embedded credentials are rejected: they would be
// silently dropped when the request path is appended.
func normalizeBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("typesafe: invalid base URL %q: %w", raw, err)
	}
	switch {
	case u.Scheme != "http" && u.Scheme != "https":
		return nil, fmt.Errorf("typesafe: invalid base URL %q: scheme must be http or https", raw)
	case u.Host == "":
		return nil, fmt.Errorf("typesafe: invalid base URL %q: missing host", raw)
	case u.RawQuery != "" || u.ForceQuery:
		return nil, fmt.Errorf("typesafe: invalid base URL %q: must not carry a query", raw)
	case u.Fragment != "":
		return nil, fmt.Errorf("typesafe: invalid base URL %q: must not carry a fragment", raw)
	case u.User != nil:
		return nil, fmt.Errorf("typesafe: invalid base URL %q: must not carry credentials", raw)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

// NewLogger returns a text logger on w at the named level: debug, info,
// warning (or warn), error. Any other value, including "off" and "",
// returns a logger that discards everything.
func NewLogger(w io.Writer, level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warning", "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		return slog.New(slog.DiscardHandler)
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: lvl}))
}

// String describes the client without its API key, so the key cannot reach a
// log through fmt.
func (c *Client) String() string {
	return fmt.Sprintf("typesafe.Client{base_url: %s, model: %s}", c.baseURL, c.model)
}

// LogValue describes the client without its API key, so the key cannot reach a
// log through slog.
func (c *Client) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("base_url", c.baseURL.String()),
		slog.String("model", c.model),
	)
}

// Close releases idle connections held by a client-owned http.Client.
func (c *Client) Close() error {
	if c.ownsHTTP {
		c.httpClient.CloseIdleConnections()
	}
	return nil
}

type requestConfig struct {
	model     string
	retry     RetryPolicy
	timeout   time.Duration
	headers   http.Header
	extraBody map[string]any
}

// RequestOption configures a single call. An option that cannot be applied
// returns an error, which the call returns before contacting the API.
type RequestOption func(*requestConfig) error

// WithRequestModel overrides the client's model for this call.
func WithRequestModel(m string) RequestOption {
	return func(rc *requestConfig) error { rc.model = m; return nil }
}

// WithRequestRetry overrides the retry policy for this call.
func WithRequestRetry(p RetryPolicy) RequestOption {
	return func(rc *requestConfig) error { rc.retry = p; return nil }
}

// WithRequestTimeout overrides the per-attempt timeout for this call. Zero
// disables the per-attempt timeout; a negative value is an error.
func WithRequestTimeout(d time.Duration) RequestOption {
	return func(rc *requestConfig) error {
		if d < 0 {
			return &ValidationError{Path: "timeout", Err: errors.New("timeout must not be negative")}
		}
		rc.timeout = d
		return nil
	}
}

// WithExtraHeaders adds headers to this call, replacing client headers with
// the same name. As with WithHeaders, an Authorization header given here
// replaces the API key header.
func WithExtraHeaders(h http.Header) RequestOption {
	// Cloned at construction, as in WithHeaders.
	cp := h.Clone()
	return func(rc *requestConfig) error { rc.headers = cp; return nil }
}

// WithExtraBody merges fields into the top level of the request body. Use it
// for API fields this package does not model yet. The map is copied, and the
// fields the client sets itself, state, model, and questions, are rejected.
func WithExtraBody(fields map[string]any) RequestOption {
	// Copy now, not when the option is applied, so a caller that keeps
	// writing to its map after building the option cannot change the request.
	cp := make(map[string]any, len(fields))
	for k, v := range fields {
		cp[k] = v
	}
	return func(rc *requestConfig) error { rc.extraBody = cp; return nil }
}

// reservedBodyKeys are set by the client and may not be overridden through
// WithExtraBody. Kept sorted so the reported error is deterministic.
var reservedBodyKeys = []string{"model", "questions", "state"}

func (c *Client) requestConfig(opts []RequestOption) (requestConfig, error) {
	rc := requestConfig{model: c.model, retry: c.retry, timeout: c.timeout}
	for _, o := range opts {
		if err := o(&rc); err != nil {
			return rc, err
		}
	}
	return rc, nil
}

// startInstrument opens the instrumentation span for one call. The returned
// finish function runs at most once, so the normal path and the panic guard
// cannot both report a result.
func (c *Client) startInstrument(ctx context.Context, info RequestInfo) (context.Context, func(RequestResult)) {
	if c.instr == nil {
		return ctx, func(RequestResult) {}
	}
	ctx, done := c.instr.RequestStart(ctx, info)
	var once sync.Once
	return ctx, func(r RequestResult) { once.Do(func() { done(r) }) }
}

// SystemOne asks the named questions about state and returns the typed
// answers. state is a string, map, slice, or struct that encodes to JSON.
func (c *Client) SystemOne(ctx context.Context, state any, questions Questions, opts ...RequestOption) (*SystemOneResponse, error) {
	rc, err := c.requestConfig(opts)
	if err != nil {
		return nil, err
	}
	info := RequestInfo{Operation: "system_one", Model: rc.model, State: state, Questions: questions}
	info.QuestionCount, info.NoulCount, info.ChoiceCount, info.ScoreCount = countQuestions(questions)
	ctx, finish := c.startInstrument(ctx, info)
	defer func() {
		if r := recover(); r != nil {
			finish(RequestResult{Err: fmt.Errorf("panic: %v", r)})
			panic(r)
		}
	}()

	if err := validateContent("state", state, false); err != nil {
		finish(RequestResult{Err: err})
		return nil, err
	}
	if err := validateQuestions(questions); err != nil {
		finish(RequestResult{Err: err})
		return nil, err
	}
	for _, k := range reservedBodyKeys {
		if _, ok := rc.extraBody[k]; ok {
			verr := &ValidationError{Path: "extra_body." + k, Err: errors.New("field is set by the client and must not be overridden")}
			finish(RequestResult{Err: verr})
			return nil, verr
		}
	}
	body := map[string]any{"state": state, "model": rc.model, "questions": questions}
	for k, v := range rc.extraBody {
		body[k] = v
	}
	payload, err := json.Marshal(body)
	if err != nil {
		verr := &ValidationError{Path: "body", Err: err}
		finish(RequestResult{Err: verr})
		return nil, verr
	}

	resp, raw, attempts, err := c.send(ctx, http.MethodPost, "/v1/systemone", payload, rc.headers, rc.retry, rc.timeout)
	if err != nil {
		finish(RequestResult{Attempts: attempts, Status: statusOf(resp), RequestID: requestIDOf(resp), Err: err})
		return nil, err
	}
	out, err := decodeSystemOne(raw)
	if err != nil {
		finish(RequestResult{Attempts: attempts, Status: resp.StatusCode, RequestID: requestIDOf(resp), Err: err})
		return nil, err
	}
	out.RequestID = requestIDOf(resp)
	out.Raw = &RawResponse{Status: resp.StatusCode, Header: resp.Header.Clone(), Body: raw}
	finish(RequestResult{Attempts: attempts, Status: resp.StatusCode, RequestID: out.RequestID, Model: out.Model, Usage: out.Usage, Response: out})
	return out, nil
}

// ListModels returns the models available to the account.
func (c *Client) ListModels(ctx context.Context, opts ...RequestOption) (*ListModelsResponse, error) {
	rc, err := c.requestConfig(opts)
	if err != nil {
		return nil, err
	}
	ctx, finish := c.startInstrument(ctx, RequestInfo{Operation: "list_models", Model: rc.model})
	defer func() {
		if r := recover(); r != nil {
			finish(RequestResult{Err: fmt.Errorf("panic: %v", r)})
			panic(r)
		}
	}()
	resp, raw, attempts, err := c.send(ctx, http.MethodGet, "/v1/models", nil, rc.headers, rc.retry, rc.timeout)
	if err != nil {
		finish(RequestResult{Attempts: attempts, Status: statusOf(resp), RequestID: requestIDOf(resp), Err: err})
		return nil, err
	}
	out, err := decodeModels(raw)
	if err != nil {
		finish(RequestResult{Attempts: attempts, Status: resp.StatusCode, RequestID: requestIDOf(resp), Err: err})
		return nil, err
	}
	out.RequestID = requestIDOf(resp)
	out.Raw = &RawResponse{Status: resp.StatusCode, Header: resp.Header.Clone(), Body: raw}
	finish(RequestResult{Attempts: attempts, Status: resp.StatusCode, RequestID: out.RequestID})
	return out, nil
}

func statusOf(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

func requestIDOf(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	return resp.Header.Get(headerRequestID)
}

func countQuestions(qs Questions) (total, nouls, choices, scores int) {
	for _, q := range qs {
		total++
		switch v := q.(type) {
		case Noul, *Noul:
			nouls++
		case Choice, *Choice:
			choices++
		case Score, *Score:
			scores++
		case RawQuestion:
			switch v["type"] {
			case "noul":
				nouls++
			case "choice":
				choices++
			case "score":
				scores++
			}
		}
	}
	return total, nouls, choices, scores
}
