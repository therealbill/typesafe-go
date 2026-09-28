// Package review asks Jev whether a codebase's tests exercise the behaviors
// its specification requires and whether the implementation contradicts the
// specification. It judges only the behaviors listed in the configuration
// against the named spec section and files. It does not find bugs, review
// style, check security, or verify correctness.
package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/therealbill/typesafe-go"
)

// Config is the contents of jev-review.json. File paths, including Spec,
// resolve relative to the directory that contains the config file.
type Config struct {
	Description string `json:"description,omitempty"`
	Spec        string `json:"spec"`
	Units       []Unit `json:"units"`
}

// Unit pairs one spec section with the files that implement and test it and
// the behaviors the section requires.
type Unit struct {
	Name           string   `json:"name"`
	SpecHeading    string   `json:"spec_heading"`
	Implementation []string `json:"implementation"`
	Tests          []string `json:"tests"`
	Behaviors      []string `json:"behaviors"`
	// Accepted lists flags that are acknowledged and do not fail the run:
	// a behavior string verbatim, or one of the ids contradicts_spec,
	// thoroughness, weakest_area.
	Accepted []string `json:"accepted,omitempty"`
	// Notes records why each acceptance was granted. They are copied into
	// the report.
	Notes []string `json:"notes,omitempty"`
}

// Options controls thresholds, limits, and selection.
type Options struct {
	Only          string
	MaxStateBytes int
	MinCover      float64
	MaxContradict float64
	MinThorough   float64
	UnitTimeout   time.Duration
}

// DefaultOptions returns the thresholds and limits used when none are given.
func DefaultOptions() Options {
	return Options{MaxStateBytes: 100000, MinCover: 0.6, MaxContradict: 0.4, MinThorough: 2.0, UnitTimeout: 2 * time.Minute}
}

// Asker is the part of *typesafe.Client the review uses.
type Asker interface {
	SystemOne(ctx context.Context, state any, questions typesafe.Questions, opts ...typesafe.RequestOption) (*typesafe.SystemOneResponse, error)
}

// BehaviorResult is one behavior's coverage probability.
type BehaviorResult struct {
	ID       string  `json:"id"`
	Behavior string  `json:"behavior"`
	Covered  float64 `json:"covered"`
}

// UnitReport is the outcome for one unit.
type UnitReport struct {
	Name             string           `json:"name"`
	RequestID        string           `json:"request_id,omitempty"`
	Model            string           `json:"model,omitempty"`
	InputTokens      int              `json:"input_tokens,omitempty"`
	Truncated        bool             `json:"truncated"`
	Behaviors        []BehaviorResult `json:"behaviors"`
	Contradicts      float64          `json:"contradicts_spec"`
	Thoroughness     float64          `json:"thoroughness"`
	ThoroughConf     float64          `json:"thoroughness_confidence"`
	Weakest          string           `json:"weakest_area"`
	WeakestConf      float64          `json:"weakest_confidence"`
	Flags            []string         `json:"flags"`
	Notes            []string         `json:"notes,omitempty"`
	StaleAcceptances []string         `json:"stale_acceptances,omitempty"`
	Failing          bool             `json:"failing"`
	Error            string           `json:"error,omitempty"`
	err              error
}

// Report is the outcome of a run.
type Report struct {
	Units []UnitReport
}

// Failing reports whether any unit has an unaccepted flag or an error.
func (r Report) Failing() bool {
	for _, u := range r.Units {
		if u.Failing {
			return true
		}
	}
	return false
}

// FirstCallError returns the first API or transport error that stopped a
// unit, or nil. Callers use it to choose a process exit code.
func (r Report) FirstCallError() error {
	for _, u := range r.Units {
		if u.err != nil {
			return u.err
		}
	}
	return nil
}

// JSON encodes the units as the report document.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r.Units, "", "  ")
}

// ErrNoUnits is returned by Run when Options.Only matches no unit.
var ErrNoUnits = errors.New("review: no units matched")

const truncatedMarker = "\n// ...(truncated)\n"

var coversID = regexp.MustCompile(`^covers_\d+$`)

func coversKey(i int) string { return fmt.Sprintf("covers_%02d", i) }

// Load reads and validates a config file and returns it with the directory
// that relative paths resolve against.
func Load(path string) (*Config, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, "", fmt.Errorf("%s: %w", path, err)
	}
	if err := validate(&cfg); err != nil {
		return nil, "", fmt.Errorf("%s: %w", path, err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	return &cfg, filepath.Dir(abs), nil
}

func validate(cfg *Config) error {
	if strings.TrimSpace(cfg.Spec) == "" {
		return errors.New(`"spec" is required`)
	}
	if len(cfg.Units) == 0 {
		return errors.New("at least one unit is required")
	}
	seen := map[string]bool{}
	for i, u := range cfg.Units {
		where := fmt.Sprintf("units[%d]", i)
		if strings.TrimSpace(u.Name) == "" {
			return fmt.Errorf("%s: name is required", where)
		}
		if seen[u.Name] {
			return fmt.Errorf("duplicate unit name %q", u.Name)
		}
		seen[u.Name] = true
		if strings.TrimSpace(u.SpecHeading) == "" {
			return fmt.Errorf("unit %q: spec_heading is required", u.Name)
		}
		if len(u.Implementation) == 0 {
			return fmt.Errorf("unit %q: at least one implementation file is required", u.Name)
		}
		if len(u.Behaviors) == 0 {
			return fmt.Errorf("unit %q: at least one behavior is required", u.Name)
		}
		behaviors := map[string]bool{}
		for _, b := range u.Behaviors {
			behaviors[b] = true
		}
		for _, a := range u.Accepted {
			if coversID.MatchString(a) {
				return fmt.Errorf("unit %q accepts %q by position; name the behavior text instead", u.Name, a)
			}
			if !behaviors[a] && a != "contradicts_spec" && a != "thoroughness" && a != "weakest_area" {
				return fmt.Errorf("unit %q accepts %q, which is neither one of its behaviors nor a question id", u.Name, a)
			}
		}
	}
	return nil
}

func resolve(base, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}

// Run evaluates every unit (or the one named by opts.Only) and returns the
// report. Errors reading files or a spec section, API failures, and
// timeouts are recorded on the unit; Run itself fails only when no unit
// matched or the spec file cannot be read.
func Run(ctx context.Context, cfg *Config, base string, asker Asker, opts Options) (Report, error) {
	specBytes, err := os.ReadFile(resolve(base, cfg.Spec))
	if err != nil {
		return Report{}, err
	}
	var rep Report
	for _, u := range cfg.Units {
		if opts.Only != "" && u.Name != opts.Only {
			continue
		}
		rep.Units = append(rep.Units, runUnit(ctx, u, string(specBytes), base, asker, opts))
	}
	if len(rep.Units) == 0 {
		return rep, ErrNoUnits
	}
	return rep, nil
}

func runUnit(ctx context.Context, u Unit, spec, base string, asker Asker, opts Options) UnitReport {
	r := UnitReport{Name: u.Name, Notes: u.Notes}
	section, err := extractSection(spec, u.SpecHeading)
	if err != nil {
		r.Error, r.Failing = err.Error(), true
		return r
	}
	files := map[string]string{}
	for _, p := range append(append([]string{}, u.Implementation...), u.Tests...) {
		b, err := os.ReadFile(resolve(base, p))
		if err != nil {
			r.Error, r.Failing = err.Error(), true
			return r
		}
		files[p] = string(b)
	}
	half := opts.MaxStateBytes / 2
	impl, t1 := bundle(u.Implementation, files, half)
	tests, t2 := bundle(u.Tests, files, half)
	truncated := t1 || t2

	state := map[string]any{"spec": section, "implementation": impl, "tests": tests}
	questions := typesafe.Questions{}
	for k, q := range buildQuestions(u) {
		questions[k] = typesafe.RawQuestion(q)
	}
	uctx, cancel := context.WithTimeout(ctx, opts.UnitTimeout)
	defer cancel()
	res, err := asker.SystemOne(uctx, state, questions)
	if err != nil {
		if errors.Is(uctx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
			r.Error = fmt.Sprintf("the review call did not finish within %s", opts.UnitTimeout)
		} else {
			r.Error = err.Error()
			r.err = err
		}
		r.Failing = true
		return r
	}
	out := evaluate(u, res.Answers, opts)
	out.RequestID = res.RequestID
	out.Model = res.Model
	if res.Usage.InputTokens != nil {
		out.InputTokens = *res.Usage.InputTokens
	}
	out.Truncated = truncated
	return out
}

// extractSection returns the lines from the heading through the line before
// the next heading of the same or higher level, ignoring headings inside
// fenced code blocks.
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

func buildQuestions(u Unit) map[string]map[string]any {
	qs := map[string]map[string]any{}
	for i, behavior := range u.Behaviors {
		qs[coversKey(i)] = map[string]any{
			"type": "noul",
			"instructions": map[string]any{
				"question": "Do the tests in `tests` exercise the behavior below, which `spec` requires of the code in `implementation`?",
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
			"validation":    "Input checks and validation error paths.",
			"error_mapping": "Turning failures from dependencies into the error types the code exposes.",
			"retry":         "Backoff, retry limits, budgets, and cancellation.",
			"decoding":      "Turning input or response data into typed values.",
			"encoding":      "Turning typed values into output or wire formats.",
			"logging":       "What is and is not logged.",
			"none":          "No area stands out.",
		},
	}
	return qs
}

func evaluate(u Unit, answers map[string]typesafe.Answer, opts Options) UnitReport {
	r := UnitReport{Name: u.Name, Notes: u.Notes}
	fail := func(msg string) UnitReport {
		return UnitReport{Name: u.Name, Notes: u.Notes, Error: msg, Failing: true}
	}
	accepted := map[string]bool{}
	for _, a := range u.Accepted {
		accepted[a] = false
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
		id := coversKey(i)
		n, ok := answers[id].(typesafe.NoulAnswer)
		if !ok {
			return fail(missing(answers, id, "noul"))
		}
		r.Behaviors = append(r.Behaviors, BehaviorResult{ID: id, Behavior: behavior, Covered: n.Noul})
		if n.Noul < opts.MinCover {
			flag(behavior, fmt.Sprintf("%s: coverage %.2f < %.2f for %q", id, n.Noul, opts.MinCover, behavior))
		}
	}
	c, ok := answers["contradicts_spec"].(typesafe.NoulAnswer)
	if !ok {
		return fail(missing(answers, "contradicts_spec", "noul"))
	}
	r.Contradicts = c.Noul
	if r.Contradicts > opts.MaxContradict {
		flag("contradicts_spec", fmt.Sprintf("contradicts_spec: %.2f > %.2f", r.Contradicts, opts.MaxContradict))
	}
	s, ok := answers["thoroughness"].(typesafe.ScoreAnswer)
	if !ok {
		return fail(missing(answers, "thoroughness", "score"))
	}
	r.Thoroughness, r.ThoroughConf = s.Score, s.Confidence
	if r.Thoroughness < opts.MinThorough {
		flag("thoroughness", fmt.Sprintf("thoroughness: %.2f < %.2f", r.Thoroughness, opts.MinThorough))
	}
	w, ok := answers["weakest_area"].(typesafe.ChoiceAnswer)
	if !ok {
		return fail(missing(answers, "weakest_area", "choice"))
	}
	r.Weakest, r.WeakestConf = w.Choice, w.Confidence
	for _, a := range u.Accepted {
		if !accepted[a] {
			r.StaleAcceptances = append(r.StaleAcceptances, a)
		}
	}
	sort.Strings(r.StaleAcceptances)
	return r
}

func missing(answers map[string]typesafe.Answer, id, want string) string {
	a, ok := answers[id]
	if !ok {
		return fmt.Sprintf("answer %q is missing from the response", id)
	}
	return fmt.Sprintf("answer %q has type %q, want %q", id, a.AnswerType(), want)
}

// Markdown renders the summary table and the flagged units.
func (r Report) Markdown(opts Options) string {
	var b strings.Builder
	b.WriteString("# jev review\n\n| Unit | Behaviors covered | Contradicts | Thoroughness | Weakest | Flags |\n|---|---|---|---|---|---|\n")
	for _, u := range r.Units {
		covered := 0
		for _, br := range u.Behaviors {
			if br.Covered >= opts.MinCover {
				covered++
			}
		}
		status := fmt.Sprintf("%d", len(u.Flags))
		if u.Failing {
			status += " FAIL"
		}
		if u.Error != "" {
			status = "error"
		}
		fmt.Fprintf(&b, "| %s | %d/%d | %.2f | %.2f (%.2f) | %s | %s |\n", u.Name, covered, len(u.Behaviors), u.Contradicts, u.Thoroughness, u.ThoroughConf, u.Weakest, status)
	}
	b.WriteString("\n")
	for _, u := range r.Units {
		if u.Error != "" {
			fmt.Fprintf(&b, "## %s\n\nerror: %s\n\n", u.Name, u.Error)
			continue
		}
		if len(u.Flags) == 0 && len(u.StaleAcceptances) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", u.Name)
		if u.Truncated {
			b.WriteString("note: sources were truncated to fit the state budget\n\n")
		}
		for _, n := range u.Notes {
			b.WriteString("note: " + n + "\n")
		}
		if len(u.Notes) > 0 {
			b.WriteString("\n")
		}
		flags := append([]string(nil), u.Flags...)
		sort.Strings(flags)
		for _, f := range flags {
			b.WriteString("- " + f + "\n")
		}
		for _, a := range u.StaleAcceptances {
			b.WriteString("- acceptance did not fire: " + a + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}
