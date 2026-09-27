package otel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/therealbill/typesafe-go"
)

func newRecorder(t *testing.T) (*tracetest.InMemoryExporter, *sdktrace.TracerProvider) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return exp, tp
}

func attrMap(kvs []attribute.KeyValue) map[string]any {
	m := map[string]any{}
	for _, kv := range kvs {
		m[string(kv.Key)] = kv.Value.AsInterface()
	}
	return m
}

func intp(i int) *int { return &i }

func sampleInfo() typesafe.RequestInfo {
	return typesafe.RequestInfo{
		Operation: "system_one", Model: "jev-latest",
		QuestionCount: 3, NoulCount: 1, ChoiceCount: 1, ScoreCount: 1,
		State:     map[string]any{"ticket": "secret customer text"},
		Questions: typesafe.Questions{"billing": typesafe.Noul{Instructions: "Is it billing?"}},
	}
}

func TestSpanOnSuccess(t *testing.T) {
	exp, tp := newRecorder(t)
	inst := New(WithTracerProvider(tp))
	ctx, finish := inst.RequestStart(context.Background(), sampleInfo())
	if !trace.SpanContextFromContext(ctx).IsValid() {
		t.Fatal("returned context must carry the span")
	}
	res := &typesafe.SystemOneResponse{Answers: map[string]typesafe.Answer{"billing": typesafe.NoulAnswer{Noul: 0.9}}}
	finish(typesafe.RequestResult{Attempts: 2, Status: 200, RequestID: "req_1", Model: "jev-1.13.0",
		Usage: typesafe.Usage{InputTokens: intp(407), OutputTokens: intp(72)}, Response: res})

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("spans %d", len(spans))
	}
	s := spans[0]
	if s.Name != "typesafe.system_one" || s.SpanKind != trace.SpanKindClient {
		t.Fatalf("span %s kind %v", s.Name, s.SpanKind)
	}
	a := attrMap(s.Attributes)
	want := map[string]any{
		AttrProviderName: "typesafe", AttrSystem: "typesafe",
		AttrRequestModel: "jev-latest", AttrResponseModel: "jev-1.13.0",
		AttrInputTokens: int64(407), AttrOutputTokens: int64(72),
		AttrRequestID: "req_1", AttrQuestionCount: int64(3), AttrNoulCount: int64(1),
		AttrChoiceCount: int64(1), AttrScoreCount: int64(1), AttrRetryAttempts: int64(2), AttrHTTPStatus: int64(200),
	}
	for k, v := range want {
		if a[k] != v {
			t.Errorf("%s = %v want %v", k, a[k], v)
		}
	}
	for _, k := range []string{AttrState, AttrQuestions, "typesafe.answer.billing.noul", AttrErrorType} {
		if _, ok := a[k]; ok {
			t.Errorf("%s must not be recorded by default", k)
		}
	}
	if s.Status.Code != codes.Unset {
		t.Fatalf("status %v", s.Status)
	}
}

func TestSpanOnError(t *testing.T) {
	exp, tp := newRecorder(t)
	inst := New(WithTracerProvider(tp))
	_, finish := inst.RequestStart(context.Background(), sampleInfo())
	err := &typesafe.APIError{Status: 503, Endpoint: "POST /v1/systemone"}
	finish(typesafe.RequestResult{Attempts: 3, Status: 503, Err: err})
	s := exp.GetSpans()[0]
	if s.Status.Code != codes.Error {
		t.Fatalf("status %v", s.Status)
	}
	a := attrMap(s.Attributes)
	if a[AttrErrorType] != "typesafe.APIError" || a[AttrHTTPStatus] != int64(503) || a[AttrRetryAttempts] != int64(3) {
		t.Fatalf("attrs %v", a)
	}
	if len(s.Events) == 0 || s.Events[0].Name != "exception" {
		t.Fatal("error should be recorded as an exception event")
	}
}

func TestRecordContentOptIn(t *testing.T) {
	exp, tp := newRecorder(t)
	inst := New(WithTracerProvider(tp), WithRecordContent(20), WithRecordAnswers())
	_, finish := inst.RequestStart(context.Background(), sampleInfo())
	res := &typesafe.SystemOneResponse{Answers: map[string]typesafe.Answer{
		"billing": typesafe.NoulAnswer{Noul: 0.9},
		"tone":    typesafe.ChoiceAnswer{Choice: "angry", Confidence: 0.8},
		"urgency": typesafe.ScoreAnswer{Score: 1.5, Confidence: 0.7},
	}}
	finish(typesafe.RequestResult{Attempts: 1, Status: 200, Response: res})
	a := attrMap(exp.GetSpans()[0].Attributes)
	state, _ := a[AttrState].(string)
	if !strings.HasPrefix(state, `{"ticket":"secret cu`) || !strings.HasSuffix(state, "...(truncated)") {
		t.Fatalf("state attr %q", state)
	}
	if _, ok := a[AttrQuestions]; !ok {
		t.Fatal("questions attr missing")
	}
	if a["typesafe.answer.billing.noul"] != 0.9 || a["typesafe.answer.tone.choice"] != "angry" || a["typesafe.answer.tone.confidence"] != 0.8 || a["typesafe.answer.urgency.score"] != 1.5 {
		t.Fatalf("answer attrs %v", a)
	}
}

func TestListModelsSpan(t *testing.T) {
	exp, tp := newRecorder(t)
	inst := New(WithTracerProvider(tp))
	_, finish := inst.RequestStart(context.Background(), typesafe.RequestInfo{Operation: "list_models", Model: "jev-latest"})
	finish(typesafe.RequestResult{Attempts: 1, Status: 200})
	s := exp.GetSpans()[0]
	if s.Name != "typesafe.list_models" {
		t.Fatalf("name %s", s.Name)
	}
	if _, ok := attrMap(s.Attributes)[AttrQuestionCount]; ok {
		t.Fatal("question counts do not apply to list_models")
	}
}

func TestTransportWraps(t *testing.T) {
	_, tp := newRecorder(t)
	inst := New(WithTracerProvider(tp))
	rt := inst.Transport(nil)
	if rt == nil {
		t.Fatal("transport must not be nil")
	}
}

func TestEndToEndParentage(t *testing.T) {
	exp, tp := newRecorder(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Traceparent") == "" {
			t.Error("traceparent header should be propagated by otelhttp")
		}
		w.Header().Set("x-typesafe-request-id", "req_e2e")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"jev-1.13.0","answers":{"b":{"type":"noul","noul":0.5}},"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	t.Cleanup(srv.Close)

	prev := otelapi.GetTextMapPropagator()
	otelapi.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otelapi.SetTextMapPropagator(prev) })

	client, err := typesafe.NewClient(typesafe.WithAPIKey("k"), typesafe.WithBaseURL(srv.URL), typesafe.WithInstrumentation(New(WithTracerProvider(tp))))
	if err != nil {
		t.Fatal(err)
	}
	ctx, parent := tp.Tracer("test").Start(context.Background(), "caller")
	if _, err := client.SystemOne(ctx, "s", typesafe.Questions{"b": typesafe.Noul{Instructions: "?"}}); err != nil {
		t.Fatal(err)
	}
	parent.End()

	spans := exp.GetSpans()
	byName := map[string]tracetest.SpanStub{}
	for _, s := range spans {
		byName[s.Name] = s
	}
	op, ok := byName["typesafe.system_one"]
	if !ok {
		t.Fatalf("operation span missing, have %v", names(spans))
	}
	if op.Parent.SpanID() != parent.SpanContext().SpanID() {
		t.Fatal("operation span should be a child of the caller span")
	}
	var httpSpan *tracetest.SpanStub
	for i := range spans {
		if spans[i].SpanKind == trace.SpanKindClient && spans[i].Name != "typesafe.system_one" {
			httpSpan = &spans[i]
		}
	}
	if httpSpan == nil || httpSpan.Parent.SpanID() != op.SpanContext.SpanID() {
		t.Fatalf("HTTP span should be a child of the operation span; spans %v", names(spans))
	}
	if attrMap(op.Attributes)[AttrRequestID] != "req_e2e" {
		t.Fatal("request id not recorded")
	}
}

func names(spans tracetest.SpanStubs) []string {
	out := make([]string, 0, len(spans))
	for _, s := range spans {
		out = append(out, s.Name)
	}
	return out
}
