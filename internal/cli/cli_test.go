package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const okBody = `{"model":"jev-1.13.0","answers":{"billing":{"type":"noul","noul":0.99},"tone":{"type":"choice","choice":"angry","confidence":1.0,"probabilities":{"neutral":0.0,"angry":1.0,"calm":0.0}},"urgency":{"type":"score","score":1.9,"confidence":0.85,"legend":{"0":"Not urgent at all","1":"Somewhat urgent","2":"Very urgent"},"probabilities":{"0":0.0,"1":0.1,"2":0.9}}},"usage":{"input_tokens":407,"output_tokens":72}}`

const modelsBody = `{"models":[{"name":"jev-latest","description":"The latest iteration of TypeSafe's System One Model: Jev","release_date":"2026-09-10T18:38:01.391457+00:00"}]}`

const requestJSON = `{"model":"jev-latest","state":{"ticket":"I was charged twice this month and nobody answers my emails. Fix it now."},"questions":{"billing":{"type":"noul","instructions":"Is ` + "`ticket`" + ` about a billing problem?"},"tone":{"type":"choice","instructions":"What is the tone of ` + "`ticket`" + `?","criteria":{"calm":"Polite and patient","angry":"Frustrated or hostile","neutral":null}},"urgency":{"type":"score","instructions":"How urgent is ` + "`ticket`" + `?","criteria":["Not urgent at all","Somewhat urgent","Very urgent"]}}}`

type recorded struct {
	mu     sync.Mutex
	calls  int
	bodies [][]byte
	paths  []string
}

func serve(t *testing.T, status int, body string, headers map[string]string) (*httptest.Server, *recorded) {
	t.Helper()
	rec := &recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rec.mu.Lock()
		rec.calls++
		rec.bodies = append(rec.bodies, b)
		rec.paths = append(rec.paths, r.URL.Path)
		rec.mu.Unlock()
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func run(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	t.Setenv("TYPESAFE_API_KEY", "")
	t.Setenv("HONEYCOMB_API_KEY", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_CONFIG_FILE", "")
	var out, errOut bytes.Buffer
	code = Main(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func baseArgs(url string) []string {
	return []string{"--api-key", "k", "--base-url", url, "--max-retries", "0"}
}

func TestAskJSONMode(t *testing.T) {
	srv, rec := serve(t, 200, okBody, map[string]string{"x-typesafe-request-id": "req_cli"})
	code, out, errOut := run(t, requestJSON, append([]string{"ask"}, baseArgs(srv.URL)...)...)
	if code != ExitOK {
		t.Fatalf("exit %d stderr %s", code, errOut)
	}
	var res struct {
		Model     string                    `json:"model"`
		Answers   map[string]map[string]any `json:"answers"`
		Usage     map[string]int            `json:"usage"`
		RequestID string                    `json:"request_id"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("stdout not JSON: %s", out)
	}
	if res.Model != "jev-1.13.0" || res.RequestID != "req_cli" || res.Answers["tone"]["choice"] != "angry" || res.Answers["tone"]["type"] != "choice" || res.Usage["input_tokens"] != 407 {
		t.Fatalf("response %+v", res)
	}
	var sent, want map[string]any
	_ = json.Unmarshal(rec.bodies[0], &sent)
	_ = json.Unmarshal([]byte(requestJSON), &want)
	sb, _ := json.Marshal(sent)
	wb, _ := json.Marshal(want)
	if !bytes.Equal(sb, wb) {
		t.Fatalf("sent body\n %s\nwant %s", sb, wb)
	}
}

func TestAskFileAndModelPrecedence(t *testing.T) {
	srv, rec := serve(t, 200, okBody, nil)
	path := filepath.Join(t.TempDir(), "req.json")
	if err := os.WriteFile(path, []byte(requestJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := run(t, "", append([]string{"ask", "-f", path, "--model", "jev-preview"}, baseArgs(srv.URL)...)...)
	if code != ExitOK {
		t.Fatalf("exit %d stderr %s", code, errOut)
	}
	var sent map[string]any
	_ = json.Unmarshal(rec.bodies[0], &sent)
	if sent["model"] != "jev-preview" {
		t.Fatalf("--model flag must override the document model, got %v", sent["model"])
	}
}

func TestAskFlagMode(t *testing.T) {
	srv, rec := serve(t, 200, okBody, nil)
	args := append([]string{"ask",
		"--state", "I was charged twice",
		"--noul", "billing=Is this about billing?",
		"--choice", "tone=What is the tone?:calm,angry",
		"--score", "urgency=How urgent?:low|high",
	}, baseArgs(srv.URL)...)
	code, _, errOut := run(t, "", args...)
	if code != ExitOK {
		t.Fatalf("exit %d stderr %s", code, errOut)
	}
	var sent map[string]any
	_ = json.Unmarshal(rec.bodies[0], &sent)
	qs := sent["questions"].(map[string]any)
	if sent["state"] != "I was charged twice" || qs["billing"].(map[string]any)["type"] != "noul" {
		t.Fatalf("sent %v", sent)
	}
	crit := qs["tone"].(map[string]any)["criteria"].(map[string]any)
	if _, ok := crit["angry"]; !ok || crit["angry"] != nil {
		t.Fatalf("choice criteria %v", crit)
	}
	levels := qs["urgency"].(map[string]any)["criteria"].([]any)
	if len(levels) != 2 || levels[1] != "high" {
		t.Fatalf("score criteria %v", levels)
	}
}

func TestAskRawAndPretty(t *testing.T) {
	srv, _ := serve(t, 200, okBody, nil)
	code, out, _ := run(t, requestJSON, append([]string{"ask", "--raw"}, baseArgs(srv.URL)...)...)
	if code != ExitOK || out != okBody+"\n" {
		t.Fatalf("raw output mismatch:\n%s", out)
	}
	code, out, _ = run(t, requestJSON, append([]string{"ask", "--pretty"}, baseArgs(srv.URL)...)...)
	if code != ExitOK || !strings.Contains(out, "\n  \"answers\"") {
		t.Fatalf("pretty output not indented:\n%s", out)
	}
}

func TestAskExitCodes(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		headers map[string]string
		stdin   string
		extra   []string
		code    int
		kind    string
	}{
		{"auth", 401, `{"detail":{"error_type":"authentication_error","message":"no"}}`, nil, requestJSON, nil, ExitAuth, "auth"},
		{"forbidden", 403, `{"detail":"no"}`, nil, requestJSON, nil, ExitAuth, "auth"},
		{"bad request", 400, `{"detail":{"error_type":"api_usage_error","message":"Invalid request."}}`, nil, requestJSON, nil, ExitRequest, "request"},
		{"unprocessable", 422, `{"detail":[{"loc":["body","questions"],"msg":"too short"}]}`, nil, requestJSON, nil, ExitRequest, "request"},
		{"rate limit", 429, `{"detail":"slow"}`, map[string]string{"Retry-After": "0"}, requestJSON, nil, ExitRateLimit, "rate_limit"},
		{"server", 503, `down`, nil, requestJSON, nil, ExitServer, "server"},
		{"overloaded", 529, `overloaded`, nil, requestJSON, nil, ExitServer, "server"},
		{"invalid response", 200, `{"answers":{"a":{"type":"noul"}}}`, nil, requestJSON, nil, ExitServer, "invalid_response"},
		{"validation", 200, okBody, nil, `{"state":"x","questions":{"q":{"type":"choice","instructions":"?","criteria":{}}}}`, nil, ExitValidation, "validation"},
		{"bad json", 200, okBody, nil, `{not json`, nil, ExitUsage, "usage"},
		{"empty stdin", 200, okBody, nil, ``, nil, ExitUsage, "usage"},
		{"file with flags", 200, okBody, nil, ``, []string{"-f", "x.json", "--noul", "a=b", "--state", "s"}, ExitUsage, "usage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := serve(t, tt.status, tt.body, tt.headers)
			args := append([]string{"ask"}, baseArgs(srv.URL)...)
			args = append(args, tt.extra...)
			code, out, errOut := run(t, tt.stdin, args...)
			if code != tt.code {
				t.Fatalf("exit %d want %d; stdout %s stderr %s", code, tt.code, out, errOut)
			}
			var payload struct {
				Error struct {
					Kind   string `json:"kind"`
					Status int    `json:"status"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(out), &payload); err != nil {
				t.Fatalf("stdout not error JSON: %s", out)
			}
			if payload.Error.Kind != tt.kind {
				t.Fatalf("kind %q want %q", payload.Error.Kind, tt.kind)
			}
			if tt.status >= 400 && payload.Error.Status != tt.status {
				t.Fatalf("status %d want %d", payload.Error.Status, tt.status)
			}
			if !strings.HasPrefix(errOut, "jev: ") {
				t.Fatalf("stderr %q", errOut)
			}
		})
	}
}

func TestAskConnectionError(t *testing.T) {
	srv, _ := serve(t, 200, okBody, nil)
	url := srv.URL
	srv.Close()
	code, out, _ := run(t, requestJSON, append([]string{"ask"}, baseArgs(url)...)...)
	if code != ExitConnection || !strings.Contains(out, `"kind":"connection"`) {
		t.Fatalf("exit %d out %s", code, out)
	}
}

func TestAskMissingAPIKey(t *testing.T) {
	srv, _ := serve(t, 200, okBody, nil)
	code, out, _ := run(t, requestJSON, "ask", "--base-url", srv.URL)
	if code != ExitUsage || !strings.Contains(out, `"kind":"usage"`) {
		t.Fatalf("exit %d out %s", code, out)
	}
}

func TestModels(t *testing.T) {
	srv, rec := serve(t, 200, modelsBody, nil)
	code, out, errOut := run(t, "", append([]string{"models"}, baseArgs(srv.URL)...)...)
	if code != ExitOK {
		t.Fatalf("exit %d stderr %s", code, errOut)
	}
	if rec.paths[0] != "/v1/models" {
		t.Fatalf("path %s", rec.paths[0])
	}
	var res struct {
		Models []map[string]string `json:"models"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil || len(res.Models) != 1 || res.Models[0]["name"] != "jev-latest" {
		t.Fatalf("out %s err %v", out, err)
	}
}

func TestVersion(t *testing.T) {
	code, out, _ := run(t, "", "version")
	if code != ExitOK || !strings.Contains(out, `"version"`) || !strings.Contains(out, `"go"`) {
		t.Fatalf("exit %d out %s", code, out)
	}
}

func TestUnknownCommandIsUsage(t *testing.T) {
	code, _, errOut := run(t, "", "bogus")
	if code != ExitUsage || !strings.Contains(errOut, "jev --help") {
		t.Fatalf("exit %d stderr %s", code, errOut)
	}
}

func TestAskTerminalStdinIsUsageError(t *testing.T) {
	saved := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = saved })
	stdinIsTerminal = func(io.Reader) bool { return true }

	srv, rec := serve(t, 200, okBody, nil)
	code, out, errOut := run(t, "", append([]string{"ask"}, baseArgs(srv.URL)...)...)
	if code != ExitUsage {
		t.Fatalf("exit %d want %d; stdout %s stderr %s", code, ExitUsage, out, errOut)
	}
	if !strings.Contains(out, `"kind":"usage"`) || !strings.Contains(out, "no request given") {
		t.Fatalf("stdout %s", out)
	}
	rec.mu.Lock()
	calls := rec.calls
	rec.mu.Unlock()
	if calls != 0 {
		t.Fatalf("a terminal stdin must not reach the API, got %d calls", calls)
	}
}
