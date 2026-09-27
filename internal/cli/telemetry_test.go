package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestTelemetryEnabled(t *testing.T) {
	tests := []struct {
		name string
		g    globals
		env  map[string]string
		want bool
	}{
		{"nothing set", globals{}, nil, false},
		{"honeycomb key", globals{}, map[string]string{"HONEYCOMB_API_KEY": "hc"}, true},
		{"otlp endpoint", globals{}, map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://x"}, true},
		{"config file", globals{}, map[string]string{"OTEL_CONFIG_FILE": "/x.yaml"}, true},
		{"--trace forces on", globals{trace: true}, nil, true},
		{"--no-trace wins", globals{noTrace: true}, map[string]string{"HONEYCOMB_API_KEY": "hc"}, false},
		{"--no-trace beats --trace", globals{trace: true, noTrace: true}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := telemetryEnabled(&tt.g, env(tt.env)); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestBuildTelemetryConfig(t *testing.T) {
	cfg := buildTelemetryConfig("hc-key", "", "")
	exp := cfg.TracerProvider.Processors[0].Batch.Exporter.OTLPHttp
	if *exp.Endpoint != "https://api.honeycomb.io/v1/traces" {
		t.Fatalf("endpoint %s", *exp.Endpoint)
	}
	if len(exp.Headers) != 1 || exp.Headers[0].Name != "x-honeycomb-team" || *exp.Headers[0].Value != "hc-key" {
		t.Fatalf("headers %+v", exp.Headers)
	}
	if cfg.Resource.Attributes[0].Name != "service.name" || cfg.Resource.Attributes[0].Value != "jev" {
		t.Fatalf("resource %+v", cfg.Resource.Attributes)
	}

	cfg = buildTelemetryConfig("", "http://collector:4318", "myapp")
	exp = cfg.TracerProvider.Processors[0].Batch.Exporter.OTLPHttp
	if *exp.Endpoint != "http://collector:4318/v1/traces" || len(exp.Headers) != 0 {
		t.Fatalf("custom endpoint %s headers %v", *exp.Endpoint, exp.Headers)
	}
	if cfg.Resource.Attributes[0].Value != "myapp" {
		t.Fatalf("service name %v", cfg.Resource.Attributes[0].Value)
	}

	cfg = buildTelemetryConfig("k", "https://api.eu1.honeycomb.io/v1/traces/", "")
	if *cfg.TracerProvider.Processors[0].Batch.Exporter.OTLPHttp.Endpoint != "https://api.eu1.honeycomb.io/v1/traces" {
		t.Fatal("trailing slash and existing path must be preserved without duplication")
	}
}

func TestTelemetryConfigFromFileExpandsEnv(t *testing.T) {
	path := t.TempDir() + "/otel.yaml"
	yaml := "file_format: \"1.0\"\ntracer_provider:\n  processors:\n    - batch:\n        exporter:\n          otlp_http:\n            endpoint: https://api.honeycomb.io/v1/traces\n            headers:\n              - name: x-honeycomb-team\n                value: ${HONEYCOMB_API_KEY}\n"
	if err := writeFile(path, yaml); err != nil {
		t.Fatal(err)
	}
	cfg, err := telemetryConfig(env(map[string]string{"OTEL_CONFIG_FILE": path, "HONEYCOMB_API_KEY": "from-env"}))
	if err != nil {
		t.Fatal(err)
	}
	h := cfg.TracerProvider.Processors[0].Batch.Exporter.OTLPHttp.Headers
	if len(h) != 1 || *h[0].Value != "from-env" {
		t.Fatalf("headers %+v", h)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func TestSetupTelemetryDisablesOnBadConfigFile(t *testing.T) {
	var errOut bytes.Buffer
	getenv := env(map[string]string{"OTEL_CONFIG_FILE": "/nonexistent/otel.yaml"})
	shutdown, inst := setupTelemetry(context.Background(), &globals{}, IO{Err: &errOut}, getenv)
	if shutdown == nil {
		t.Fatal("shutdown must never be nil")
	}
	shutdown()
	if inst != nil {
		t.Fatalf("instrumentation must be nil when tracing fails, got %T", inst)
	}
	if !strings.HasPrefix(errOut.String(), "jev: tracing disabled:") {
		t.Fatalf("stderr %q", errOut.String())
	}
}

func TestSetupTelemetryDisabledReturnsNoopAndNil(t *testing.T) {
	var errOut bytes.Buffer
	shutdown, inst := setupTelemetry(context.Background(), &globals{}, IO{Err: &errOut}, env(nil))
	if shutdown == nil {
		t.Fatal("shutdown must never be nil")
	}
	shutdown()
	if inst != nil {
		t.Fatalf("instrumentation must be nil when tracing is off, got %T", inst)
	}
	if errOut.Len() != 0 {
		t.Fatalf("tracing off must be silent, got %q", errOut.String())
	}
}
