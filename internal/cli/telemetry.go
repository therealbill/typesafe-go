package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	otelconf "go.opentelemetry.io/contrib/otelconf/x"
	otelapi "go.opentelemetry.io/otel"

	"github.com/therealbill/typesafe-go"
	tsotel "github.com/therealbill/typesafe-go/otel"
)

// telemetryEnabled decides whether to start the OpenTelemetry SDK.
// --no-trace always wins. Otherwise --trace, HONEYCOMB_API_KEY,
// OTEL_EXPORTER_OTLP_ENDPOINT, or OTEL_CONFIG_FILE turns tracing on.
func telemetryEnabled(g *globals, getenv func(string) string) bool {
	if g.noTrace {
		return false
	}
	return g.trace || getenv("HONEYCOMB_API_KEY") != "" || getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" || getenv("OTEL_CONFIG_FILE") != ""
}

// setupTelemetry starts the SDK when enabled and returns a shutdown function
// plus the client instrumentation to attach. When disabled or on failure it
// returns a no-op shutdown and nil instrumentation; failures are reported on
// stderr and never stop the command.
func setupTelemetry(ctx context.Context, g *globals, streams IO, getenv func(string) string) (func(), typesafe.Instrumentation) {
	noop := func() {}
	if !telemetryEnabled(g, getenv) {
		return noop, nil
	}
	cfg, err := telemetryConfig(getenv)
	if err != nil {
		_, _ = fmt.Fprintln(streams.Err, "jev: tracing disabled:", err)
		return noop, nil
	}
	sdk, err := otelconf.NewSDK(otelconf.WithContext(ctx), otelconf.WithOpenTelemetryConfiguration(cfg))
	if err != nil {
		_, _ = fmt.Fprintln(streams.Err, "jev: tracing disabled:", err)
		return noop, nil
	}
	otelapi.SetTracerProvider(sdk.TracerProvider())
	if p := sdk.Propagator(); p != nil {
		otelapi.SetTextMapPropagator(p)
	}
	shutdown := func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := sdk.Shutdown(c); err != nil {
			_, _ = fmt.Fprintln(streams.Err, "jev: tracing shutdown:", err)
		}
	}
	return shutdown, tsotel.New()
}

// telemetryConfig loads OTEL_CONFIG_FILE (with ${VAR} expansion from the
// environment) or builds a configuration from HONEYCOMB_API_KEY,
// OTEL_EXPORTER_OTLP_ENDPOINT, and OTEL_SERVICE_NAME.
func telemetryConfig(getenv func(string) string) (otelconf.OpenTelemetryConfiguration, error) {
	if file := getenv("OTEL_CONFIG_FILE"); file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return otelconf.OpenTelemetryConfiguration{}, fmt.Errorf("read OTEL_CONFIG_FILE: %w", err)
		}
		expanded := os.Expand(string(b), getenv)
		cfg, err := otelconf.ParseYAML([]byte(expanded))
		if err != nil {
			return otelconf.OpenTelemetryConfiguration{}, fmt.Errorf("parse OTEL_CONFIG_FILE: %w", err)
		}
		return *cfg, nil
	}
	return buildTelemetryConfig(getenv("HONEYCOMB_API_KEY"), getenv("OTEL_EXPORTER_OTLP_ENDPOINT"), getenv("OTEL_SERVICE_NAME")), nil
}

// buildTelemetryConfig returns a tracer-only configuration exporting over
// OTLP/HTTP. With no endpoint it targets Honeycomb's US region.
func buildTelemetryConfig(honeycombKey, endpoint, serviceName string) otelconf.OpenTelemetryConfiguration {
	if serviceName == "" {
		serviceName = "jev"
	}
	if endpoint == "" {
		endpoint = "https://api.honeycomb.io"
	}
	endpoint = strings.TrimRight(endpoint, "/")
	if !strings.HasSuffix(endpoint, "/v1/traces") {
		endpoint += "/v1/traces"
	}
	exp := otelconf.OTLPHttpExporter{Endpoint: &endpoint}
	if honeycombKey != "" {
		key := honeycombKey
		exp.Headers = []otelconf.NameStringValuePair{{Name: "x-honeycomb-team", Value: &key}}
	}
	return otelconf.OpenTelemetryConfiguration{
		FileFormat: "1.0",
		Resource: &otelconf.Resource{
			Attributes: []otelconf.AttributeNameValue{{Name: "service.name", Value: serviceName}},
		},
		TracerProvider: &otelconf.TracerProvider{
			Processors: []otelconf.SpanProcessor{{
				Batch: &otelconf.BatchSpanProcessor{Exporter: otelconf.SpanExporter{OTLPHttp: &exp}},
			}},
		},
	}
}
