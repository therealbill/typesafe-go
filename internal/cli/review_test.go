package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// reviewServer answers any review request: covers_* nouls get cover, the
// contradiction noul 0.1, thoroughness 2.5, weakest_area "none". A non-200
// status returns an error body instead.
func reviewServer(t *testing.T, cover float64, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-typesafe-request-id", "req_review")
		if status != 200 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"detail":"no"}`))
			return
		}
		var req struct {
			Questions map[string]map[string]any `json:"questions"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		answers := map[string]any{}
		for id, q := range req.Questions {
			switch q["type"] {
			case "noul":
				p := 0.1
				if strings.HasPrefix(id, "covers_") {
					p = cover
				}
				answers[id] = map[string]any{"type": "noul", "noul": p}
			case "score":
				answers[id] = map[string]any{"type": "score", "score": 2.5, "confidence": 0.8, "legend": map[string]any{}, "probabilities": map[string]any{}}
			case "choice":
				answers[id] = map[string]any{"type": "choice", "choice": "none", "confidence": 0.6, "probabilities": map[string]any{}}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "jev-test", "answers": answers, "usage": map[string]int{"input_tokens": 10, "output_tokens": 1}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// reviewRepo writes a spec, an implementation file, a test file, and a config
// under a temp dir and returns the config and report paths.
func reviewRepo(t *testing.T) (cfg, report string) {
	t.Helper()
	dir := t.TempDir()
	must := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	must("spec.md", "# Spec\n\n## Retry\n\nretry text\n")
	must("retry.go", "package x\n")
	must("retry_test.go", "package x\n")
	must("jev-review.json", `{"spec":"spec.md","units":[{"name":"retry","spec_heading":"## Retry","implementation":["retry.go"],"tests":["retry_test.go"],"behaviors":["b0","b1"]}]}`)
	return filepath.Join(dir, "jev-review.json"), filepath.Join(dir, "report.json")
}

func reviewArgs(cfg, report, url string, extra ...string) []string {
	args := []string{"review", "--units", cfg, "--report", report, "--api-key", "k", "--base-url", url, "--max-retries", "0"}
	return append(args, extra...)
}

func TestReviewInitWritesAndRefusesOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jev-review.json")
	code, out, _ := run(t, "", "review", "init", "--units", path)
	if code != ExitOK || !strings.Contains(out, `"wrote"`) {
		t.Fatalf("exit %d out %s", code, out)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("template not written")
	}
	code, out, _ = run(t, "", "review", "init", "--units", path)
	if code != ExitUsage || !strings.Contains(out, `"kind":"usage"`) || !strings.Contains(out, "already exists") {
		t.Fatalf("second init: exit %d out %s", code, out)
	}
}

func TestReviewCleanRun(t *testing.T) {
	cfg, report := reviewRepo(t)
	srv := reviewServer(t, 0.9, 200)
	code, out, errOut := run(t, "", reviewArgs(cfg, report, srv.URL)...)
	if code != ExitOK {
		t.Fatalf("exit %d stdout %s stderr %s", code, out, errOut)
	}
	if !strings.Contains(out, "# jev review") || !strings.Contains(out, "| retry | 2/2 |") {
		t.Fatalf("markdown missing: %s", out)
	}
	if !strings.Contains(errOut, "review: ran 1 units") {
		t.Fatalf("stderr %q", errOut)
	}
	b, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	var units []map[string]any
	if err := json.Unmarshal(b, &units); err != nil || len(units) != 1 || units[0]["request_id"] != "req_review" {
		t.Fatalf("report %s err %v", b, err)
	}
}

func TestReviewFlaggedExits8(t *testing.T) {
	cfg, report := reviewRepo(t)
	srv := reviewServer(t, 0.2, 200)
	code, out, errOut := run(t, "", reviewArgs(cfg, report, srv.URL)...)
	if code != ExitFlagged {
		t.Fatalf("exit %d stdout %s stderr %s", code, out, errOut)
	}
	if !strings.Contains(out, "FAIL") || !strings.Contains(out, "covers_00") {
		t.Fatalf("flags missing from stdout: %s", out)
	}
	if !strings.Contains(errOut, "review: 1 of 1 units failing") {
		t.Fatalf("stderr %q", errOut)
	}
}

func TestReviewJSONOutput(t *testing.T) {
	cfg, report := reviewRepo(t)
	srv := reviewServer(t, 0.9, 200)
	code, out, _ := run(t, "", reviewArgs(cfg, report, srv.URL, "--json")...)
	var units []map[string]any
	if code != ExitOK || json.Unmarshal([]byte(out), &units) != nil || len(units) != 1 {
		t.Fatalf("exit %d out %s", code, out)
	}
}

func TestReviewUsageErrors(t *testing.T) {
	cfg, report := reviewRepo(t)
	srv := reviewServer(t, 0.9, 200)
	code, out, _ := run(t, "", reviewArgs(filepath.Join(t.TempDir(), "nope.json"), report, srv.URL)...)
	if code != ExitUsage || !strings.Contains(out, `"kind":"usage"`) {
		t.Fatalf("missing config: exit %d out %s", code, out)
	}
	code, out, _ = run(t, "", reviewArgs(cfg, report, srv.URL, "--only", "nope")...)
	if code != ExitUsage || !strings.Contains(out, "no units matched") {
		t.Fatalf("no match: exit %d out %s", code, out)
	}
}

func TestReviewAPIErrorClassified(t *testing.T) {
	cfg, report := reviewRepo(t)
	srv := reviewServer(t, 0.9, 401)
	code, out, _ := run(t, "", reviewArgs(cfg, report, srv.URL)...)
	if code != ExitAuth || !strings.Contains(out, `"kind":"auth"`) {
		t.Fatalf("exit %d out %s", code, out)
	}
	b, err := os.ReadFile(report)
	if err != nil || !strings.Contains(string(b), `"error"`) {
		t.Fatalf("report must still be written with the unit error: %v %s", err, b)
	}
}
