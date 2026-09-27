// Command selfreview asks Jev, through the jev CLI, whether each unit's tests
// cover the behaviors its spec section requires and whether the
// implementation contradicts the spec. It writes selfreview-report.json,
// prints a Markdown summary, and exits 1 when any unit is flagged and not
// accepted in units.json.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type unit struct {
	Name           string   `json:"name"`
	SpecHeading    string   `json:"spec_heading"`
	Implementation []string `json:"implementation"`
	Tests          []string `json:"tests"`
	Behaviors      []string `json:"behaviors"`
	// Accepted lists question ids whose flags are acknowledged and do not
	// fail the run, for example "covers_03".
	Accepted []string `json:"accepted,omitempty"`
	// Notes records why each acceptance was granted. Notes are copied into
	// the report so an accepted flag always carries its rationale.
	Notes []string `json:"notes,omitempty"`
}

type config struct {
	Spec  string `json:"spec"`
	Units []unit `json:"units"`
}

type thresholds struct {
	minCover      float64
	maxContradict float64
	minThorough   float64
}

type behaviorResult struct {
	ID       string  `json:"id"`
	Behavior string  `json:"behavior"`
	Covered  float64 `json:"covered"`
}

type unitReport struct {
	Name         string           `json:"name"`
	RequestID    string           `json:"request_id,omitempty"`
	Model        string           `json:"model,omitempty"`
	InputTokens  int              `json:"input_tokens,omitempty"`
	Truncated    bool             `json:"truncated"`
	Behaviors    []behaviorResult `json:"behaviors"`
	Contradicts  float64          `json:"contradicts_spec"`
	Thoroughness float64          `json:"thoroughness"`
	ThoroughConf float64          `json:"thoroughness_confidence"`
	Weakest      string           `json:"weakest_area"`
	WeakestConf  float64          `json:"weakest_confidence"`
	Flags        []string         `json:"flags"`
	Notes        []string         `json:"notes,omitempty"`
	// StaleAcceptances lists accepted entries whose flag did not fire, so an
	// acceptance that is no longer needed does not sit unnoticed.
	StaleAcceptances []string `json:"stale_acceptances,omitempty"`
	Failing          bool     `json:"failing"`
	Error            string   `json:"error,omitempty"`
}

const truncatedMarker = "\n// ...(truncated)\n"

func main() {
	unitsPath := flag.String("units", "tools/selfreview/units.json", "units file")
	jevPath := flag.String("jev", "./bin/jev", "path to the jev binary")
	reportPath := flag.String("report", "selfreview-report.json", "where to write the JSON report")
	only := flag.String("only", "", "run a single unit by name")
	maxBytes := flag.Int("max-state-bytes", 100000, "byte cap for implementation and tests together")
	minCover := flag.Float64("min-cover", 0.6, "flag a behavior whose coverage probability is below this")
	maxContradict := flag.Float64("max-contradict", 0.4, "flag a unit whose contradiction probability is above this")
	minThorough := flag.Float64("min-thorough", 2.0, "flag a unit whose thoroughness score is below this")
	unitTimeout := flag.Duration("unit-timeout", 2*time.Minute, "time allowed for one jev ask call")
	flag.Parse()

	if os.Getenv("TYPESAFE_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "selfreview: TYPESAFE_API_KEY is not set")
		os.Exit(2)
	}
	cfgBytes, err := os.ReadFile(*unitsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "selfreview:", err)
		os.Exit(2)
	}
	var cfg config
	if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
		fmt.Fprintln(os.Stderr, "selfreview: units.json:", err)
		os.Exit(2)
	}
	if err := validateUnits(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "selfreview: units.json:", err)
		os.Exit(2)
	}
	specBytes, err := os.ReadFile(cfg.Spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, "selfreview:", err)
		os.Exit(2)
	}
	th := thresholds{minCover: *minCover, maxContradict: *maxContradict, minThorough: *minThorough}

	var reports []unitReport
	failing := false
	for _, u := range cfg.Units {
		if *only != "" && u.Name != *only {
			continue
		}
		r := runUnit(u, string(specBytes), *jevPath, *maxBytes, *unitTimeout, th)
		if r.Failing {
			failing = true
		}
		reports = append(reports, r)
	}
	if len(reports) == 0 {
		fmt.Fprintln(os.Stderr, "selfreview: no units matched")
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "selfreview: ran %d units\n", len(reports))
	out, _ := json.MarshalIndent(reports, "", "  ")
	if err := os.WriteFile(*reportPath, out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "selfreview: write report:", err)
		os.Exit(2)
	}
	fmt.Print(markdown(reports, th))
	if failing {
		os.Exit(1)
	}
}

// coversID matches the positional question ids. They must not appear in an
// acceptance list: renumbering the behaviors would silently move the
// acceptance to a different claim.
var coversID = regexp.MustCompile(`^covers_\d+$`)

// validateUnits rejects configurations whose acceptances cannot be matched
// reliably.
func validateUnits(cfg config) error {
	for _, u := range cfg.Units {
		behaviors := map[string]bool{}
		for _, b := range u.Behaviors {
			behaviors[b] = true
		}
		for _, a := range u.Accepted {
			if coversID.MatchString(a) {
				return fmt.Errorf("unit %q accepts %q by position; name the behavior text instead", u.Name, a)
			}
			if !behaviors[a] && a != "contradicts_spec" && a != "thoroughness" {
				return fmt.Errorf("unit %q accepts %q, which is neither one of its behaviors nor a question id", u.Name, a)
			}
		}
	}
	return nil
}

func runUnit(u unit, spec, jevPath string, maxBytes int, timeout time.Duration, th thresholds) unitReport {
	r := unitReport{Name: u.Name}
	section, err := extractSection(spec, u.SpecHeading)
	if err != nil {
		r.Error = err.Error()
		r.Failing = true
		return r
	}
	files := map[string]string{}
	for _, p := range append(append([]string{}, u.Implementation...), u.Tests...) {
		b, err := os.ReadFile(p)
		if err != nil {
			r.Error = err.Error()
			r.Failing = true
			return r
		}
		files[p] = string(b)
	}
	half := maxBytes / 2
	impl, t1 := bundle(u.Implementation, files, half)
	tests, t2 := bundle(u.Tests, files, half)
	r.Truncated = t1 || t2

	req := map[string]any{
		"state":     map[string]any{"spec": section, "implementation": impl, "tests": tests},
		"questions": buildQuestions(u),
	}
	payload, _ := json.Marshal(req)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, jevPath, "ask")
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			r.Error = fmt.Sprintf("jev ask did not finish within %s", timeout)
			r.Failing = true
			return r
		}
		var ee *exec.ExitError
		code := -1
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		}
		r.Error = fmt.Sprintf("jev ask exited %d: %s %s", code, strings.TrimSpace(stderr.String()), strings.TrimSpace(stdout.String()))
		r.Failing = true
		return r
	}
	var resp struct {
		Model   string                    `json:"model"`
		Answers map[string]map[string]any `json:"answers"`
		Usage   struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		r.Error = "cannot parse jev output: " + err.Error()
		r.Failing = true
		return r
	}
	out := evaluate(u, resp.Answers, th)
	out.RequestID = resp.RequestID
	out.Model = resp.Model
	out.InputTokens = resp.Usage.InputTokens
	out.Truncated = r.Truncated
	return out
}

// extractSection returns the lines from the heading through the line before
// the next heading of the same or higher level.
func extractSection(spec, heading string) (string, error) {
	level := len(heading) - len(strings.TrimLeft(heading, "#"))
	lines := strings.Split(spec, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == heading {
			start = i
			break
		}
	}
	if start < 0 {
		return "", fmt.Errorf("heading %q not found in spec", heading)
	}
	end := len(lines)
	fenced := false
	for i := start + 1; i < len(lines); i++ {
		l := lines[i]
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fenced = !fenced
			continue
		}
		if fenced || !strings.HasPrefix(l, "#") {
			continue
		}
		hl := len(l) - len(strings.TrimLeft(l, "#"))
		if hl <= level && strings.HasPrefix(l[hl:], " ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n"), nil
}

// bundle concatenates files with a header line each, cutting the total to
// maxBytes and marking the cut.
func bundle(paths []string, files map[string]string, maxBytes int) (string, bool) {
	var b strings.Builder
	for _, p := range paths {
		b.WriteString("// file: " + filepath.ToSlash(p) + "\n")
		b.WriteString(files[p])
		b.WriteString("\n\n")
	}
	s := b.String()
	if len(s) <= maxBytes {
		return s, false
	}
	return s[:maxBytes] + truncatedMarker, true
}

func buildQuestions(u unit) map[string]any {
	qs := map[string]any{}
	for i, behavior := range u.Behaviors {
		qs[fmt.Sprintf("covers_%02d", i)] = map[string]any{
			"type": "noul",
			"instructions": map[string]any{
				"question": "Do the Go tests in `tests` exercise the behavior below, which `spec` requires of the code in `implementation`?",
				"behavior": behavior,
				"guidance": "Answer yes only if at least one test would fail if this behavior were removed or broken. A test that merely mentions the feature without asserting on it does not count.",
			},
		}
	}
	qs["contradicts_spec"] = map[string]any{
		"type":         "noul",
		"instructions": "Does the code in `implementation` contradict a requirement stated in `spec`? Count only actual conflicts such as a different default value, a different error type, a different wire field name, or opposite behavior. Ignore omissions and ignore anything the spec leaves unspecified.",
		"criteria": map[string]any{
			"true":  "At least one statement in `spec` is contradicted by `implementation`.",
			"false": "`implementation` is consistent with every statement in `spec` that it addresses.",
		},
	}
	qs["thoroughness"] = map[string]any{
		"type":         "score",
		"instructions": "How thoroughly do the tests in `tests` cover the behaviors that `spec` requires of `implementation`?",
		"criteria": []any{
			"No test exercises this section's behavior.",
			"Only the success path is tested; error cases named in `spec` are not.",
			"The success path and the error cases named in `spec` are tested.",
			"Success, error cases, and the boundary values, cancellation, or concurrency conditions named in `spec` are all tested.",
		},
	}
	qs["weakest_area"] = map[string]any{
		"type":         "choice",
		"instructions": "Which area of `implementation` is least covered by `tests` relative to what `spec` requires? Choose none if coverage is even.",
		"criteria": map[string]any{
			"validation":    "Input checks and the ValidationError paths.",
			"error_mapping": "Turning HTTP statuses and transport failures into error types.",
			"retry":         "Backoff, Retry-After, budget, and cancellation.",
			"decoding":      "Turning response JSON into answer types.",
			"encoding":      "Turning questions and state into wire JSON.",
			"logging":       "What is and is not logged.",
			"none":          "No area stands out.",
		},
	}
	return qs
}

func evaluate(u unit, answers map[string]map[string]any, th thresholds) unitReport {
	r := unitReport{Name: u.Name, Notes: u.Notes}
	fail := func(msg string) unitReport {
		return unitReport{Name: u.Name, Notes: u.Notes, Error: msg, Failing: true}
	}
	accepted := map[string]bool{}
	for _, a := range u.Accepted {
		accepted[a] = false // false until the matching flag fires
	}
	flag := func(key, msg string) {
		if _, ok := accepted[key]; ok {
			accepted[key] = true
			r.Flags = append(r.Flags, msg+" (accepted)")
			return
		}
		r.Flags = append(r.Flags, msg)
		r.Failing = true
	}
	for i, behavior := range u.Behaviors {
		id := fmt.Sprintf("covers_%02d", i)
		p, err := numField(answers, id, "noul")
		if err != nil {
			return fail(err.Error())
		}
		r.Behaviors = append(r.Behaviors, behaviorResult{ID: id, Behavior: behavior, Covered: p})
		if p < th.minCover {
			// Coverage acceptances are keyed by behavior text, so renumbering
			// the behaviors cannot move an acceptance to another claim.
			flag(behavior, fmt.Sprintf("%s: coverage %.2f < %.2f for %q", id, p, th.minCover, behavior))
		}
	}
	contradicts, err := numField(answers, "contradicts_spec", "noul")
	if err != nil {
		return fail(err.Error())
	}
	r.Contradicts = contradicts
	if r.Contradicts > th.maxContradict {
		flag("contradicts_spec", fmt.Sprintf("contradicts_spec: %.2f > %.2f", r.Contradicts, th.maxContradict))
	}
	thorough, err := numField(answers, "thoroughness", "score")
	if err != nil {
		return fail(err.Error())
	}
	r.Thoroughness = thorough
	r.ThoroughConf = num(answers["thoroughness"]["confidence"])
	if r.Thoroughness < th.minThorough {
		flag("thoroughness", fmt.Sprintf("thoroughness: %.2f < %.2f", r.Thoroughness, th.minThorough))
	}
	weakest, ok := answers["weakest_area"]["choice"].(string)
	if !ok {
		return fail(`answer "weakest_area" is missing a string "choice" field`)
	}
	r.Weakest = weakest
	r.WeakestConf = num(answers["weakest_area"]["confidence"])

	for _, a := range u.Accepted {
		if !accepted[a] {
			r.StaleAcceptances = append(r.StaleAcceptances, a)
		}
	}
	sort.Strings(r.StaleAcceptances)
	return r
}

// numField reads a numeric field of one answer. A missing answer or a
// non-numeric value is an error rather than a silent zero, which would look
// like a confident "no".
func numField(answers map[string]map[string]any, id, field string) (float64, error) {
	a, ok := answers[id]
	if !ok {
		return 0, fmt.Errorf("answer %q is missing from the response", id)
	}
	v, ok := a[field]
	if !ok {
		return 0, fmt.Errorf("answer %q is missing its %q field", id, field)
	}
	f, ok := v.(float64)
	if !ok {
		return 0, fmt.Errorf("answer %q has a non-numeric %q field (%T)", id, field, v)
	}
	return f, nil
}

func num(v any) float64 {
	f, _ := v.(float64)
	return f
}

func markdown(reports []unitReport, th thresholds) string {
	var b strings.Builder
	b.WriteString("# Self-review\n\n| Unit | Behaviors covered | Contradicts | Thoroughness | Weakest | Flags |\n|---|---|---|---|---|---|\n")
	for _, r := range reports {
		covered := 0
		for _, br := range r.Behaviors {
			if br.Covered >= th.minCover {
				covered++
			}
		}
		status := fmt.Sprintf("%d", len(r.Flags))
		if r.Failing {
			status += " FAIL"
		}
		if r.Error != "" {
			status = "error"
		}
		fmt.Fprintf(&b, "| %s | %d/%d | %.2f | %.2f (%.2f) | %s | %s |\n", r.Name, covered, len(r.Behaviors), r.Contradicts, r.Thoroughness, r.ThoroughConf, r.Weakest, status)
	}
	b.WriteString("\n")
	for _, r := range reports {
		if r.Error != "" {
			fmt.Fprintf(&b, "## %s\n\nerror: %s\n\n", r.Name, r.Error)
			continue
		}
		if len(r.Flags) == 0 && len(r.StaleAcceptances) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", r.Name)
		if r.Truncated {
			b.WriteString("note: sources were truncated to fit the state budget\n\n")
		}
		for _, n := range r.Notes {
			b.WriteString("note: " + n + "\n")
		}
		if len(r.Notes) > 0 {
			b.WriteString("\n")
		}
		flags := append([]string(nil), r.Flags...)
		sort.Strings(flags)
		for _, f := range flags {
			b.WriteString("- " + f + "\n")
		}
		for _, a := range r.StaleAcceptances {
			b.WriteString("- acceptance did not fire: " + a + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}
