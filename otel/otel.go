// Package otel instruments a typesafe.Client with OpenTelemetry traces.
//
// Attach it with typesafe.WithInstrumentation(otel.New()). Each SystemOne or
// ListModels call becomes a client span named typesafe.system_one or
// typesafe.list_models, with one child HTTP span per attempt. State and
// question content are never recorded unless WithRecordContent is set.
//
// This package never installs a global tracer provider. Set one in your
// application before building the client, or pass WithTracerProvider.
package otel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/therealbill/typesafe-go"
	"github.com/therealbill/typesafe-go/internal/version"
)

// TracerName is the instrumentation scope name.
const TracerName = "github.com/therealbill/typesafe-go"

// ProviderName is the value of the gen_ai.provider.name attribute.
const ProviderName = "typesafe"

// Span attribute keys. The gen_ai.* keys follow the OpenTelemetry GenAI
// semantic conventions; gen_ai.system is the older spelling and is set too.
const (
	AttrProviderName  = "gen_ai.provider.name"
	AttrSystem        = "gen_ai.system"
	AttrRequestModel  = "gen_ai.request.model"
	AttrResponseModel = "gen_ai.response.model"
	AttrInputTokens   = "gen_ai.usage.input_tokens"
	AttrOutputTokens  = "gen_ai.usage.output_tokens"
	AttrRequestID     = "typesafe.request_id"
	AttrQuestionCount = "typesafe.questions.count"
	AttrNoulCount     = "typesafe.questions.noul"
	AttrChoiceCount   = "typesafe.questions.choice"
	AttrScoreCount    = "typesafe.questions.score"
	AttrRetryAttempts = "typesafe.retry.attempts"
	AttrHTTPStatus    = "http.response.status_code"
	AttrErrorType     = "error.type"
	AttrState         = "typesafe.state"
	AttrQuestions     = "typesafe.questions"
)

const truncatedMarker = "...(truncated)"

// Option configures the instrumentation.
type Option func(*instrumentation)

// WithTracerProvider uses tp instead of the global provider.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(i *instrumentation) { i.tp = tp }
}

// WithRecordContent records the request state and questions as JSON span
// attributes, each truncated to maxBytes. Off by default because state is
// often customer data.
func WithRecordContent(maxBytes int) Option {
	return func(i *instrumentation) { i.contentBytes = maxBytes }
}

// WithRecordAnswers records each answer's value and confidence as
// typesafe.answer.<key>.<field> attributes. Off by default.
func WithRecordAnswers() Option {
	return func(i *instrumentation) { i.answers = true }
}

type instrumentation struct {
	tp           trace.TracerProvider
	contentBytes int
	answers      bool
}

// New returns an Instrumentation for typesafe.WithInstrumentation.
func New(opts ...Option) typesafe.Instrumentation {
	i := &instrumentation{}
	for _, o := range opts {
		o(i)
	}
	return i
}

func (i *instrumentation) provider() trace.TracerProvider {
	if i.tp != nil {
		return i.tp
	}
	return otelapi.GetTracerProvider()
}

func (i *instrumentation) tracer() trace.Tracer {
	return i.provider().Tracer(TracerName, trace.WithInstrumentationVersion(version.Version))
}

// RequestStart opens the operation span.
func (i *instrumentation) RequestStart(ctx context.Context, info typesafe.RequestInfo) (context.Context, func(typesafe.RequestResult)) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrProviderName, ProviderName),
		attribute.String(AttrSystem, ProviderName),
		attribute.String(AttrRequestModel, info.Model),
	}
	if info.Operation == "system_one" {
		attrs = append(attrs,
			attribute.Int(AttrQuestionCount, info.QuestionCount),
			attribute.Int(AttrNoulCount, info.NoulCount),
			attribute.Int(AttrChoiceCount, info.ChoiceCount),
			attribute.Int(AttrScoreCount, info.ScoreCount),
		)
		if i.contentBytes > 0 {
			attrs = append(attrs,
				attribute.String(AttrState, truncatedJSON(info.State, i.contentBytes)),
				attribute.String(AttrQuestions, truncatedJSON(info.Questions, i.contentBytes)),
			)
		}
	}
	ctx, span := i.tracer().Start(ctx, "typesafe."+info.Operation,
		trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(attrs...))
	return ctx, func(r typesafe.RequestResult) {
		defer span.End()
		out := []attribute.KeyValue{attribute.Int(AttrRetryAttempts, r.Attempts)}
		if r.Status != 0 {
			out = append(out, attribute.Int(AttrHTTPStatus, r.Status))
		}
		if r.RequestID != "" {
			out = append(out, attribute.String(AttrRequestID, r.RequestID))
		}
		if r.Model != "" {
			out = append(out, attribute.String(AttrResponseModel, r.Model))
		}
		if r.Usage.InputTokens != nil {
			out = append(out, attribute.Int(AttrInputTokens, *r.Usage.InputTokens))
		}
		if r.Usage.OutputTokens != nil {
			out = append(out, attribute.Int(AttrOutputTokens, *r.Usage.OutputTokens))
		}
		if i.answers && r.Response != nil {
			out = append(out, answerAttributes(r.Response)...)
		}
		span.SetAttributes(out...)
		if r.Err != nil {
			span.RecordError(r.Err)
			span.SetStatus(codes.Error, r.Err.Error())
			span.SetAttributes(attribute.String(AttrErrorType, errorType(r.Err)))
		}
	}
}

// Transport wraps rt with otelhttp so each HTTP attempt is a child span.
func (i *instrumentation) Transport(rt http.RoundTripper) http.RoundTripper {
	if rt == nil {
		rt = http.DefaultTransport
	}
	opts := []otelhttp.Option{}
	if i.tp != nil {
		opts = append(opts, otelhttp.WithTracerProvider(i.tp))
	}
	return otelhttp.NewTransport(rt, opts...)
}

func answerAttributes(res *typesafe.SystemOneResponse) []attribute.KeyValue {
	keys := make([]string, 0, len(res.Answers))
	for k := range res.Answers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []attribute.KeyValue
	for _, k := range keys {
		p := "typesafe.answer." + k
		switch v := res.Answers[k].(type) {
		case typesafe.NoulAnswer:
			out = append(out, attribute.Float64(p+".noul", v.Noul))
		case typesafe.ChoiceAnswer:
			out = append(out, attribute.String(p+".choice", v.Choice), attribute.Float64(p+".confidence", v.Confidence))
		case typesafe.ScoreAnswer:
			out = append(out, attribute.Float64(p+".score", v.Score), attribute.Float64(p+".confidence", v.Confidence))
		}
	}
	return out
}

func truncatedJSON(v any, maxBytes int) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<unencodable %T>", v)
	}
	s := string(b)
	if len(s) > maxBytes {
		return s[:maxBytes] + truncatedMarker
	}
	return s
}

func errorType(err error) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", err), "*")
}
