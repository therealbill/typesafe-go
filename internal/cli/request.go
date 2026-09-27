package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/therealbill/typesafe-go"
)

// askOptions are the flags of `jev ask`.
type askOptions struct {
	file    string
	state   string
	nouls   []string
	choices []string
	scores  []string
	raw     bool
}

// askRequest is a parsed request ready for the client.
type askRequest struct {
	State     any
	Questions typesafe.Questions
	Model     string
	Extra     map[string]any
}

func (o *askOptions) hasQuestionFlags() bool {
	return o.state != "" || len(o.nouls)+len(o.choices)+len(o.scores) > 0
}

// buildRequest chooses flag mode or JSON mode. JSON comes from --file, or
// from stdin when --file is empty and no question flags were given.
func buildRequest(o *askOptions, stdin io.Reader) (*askRequest, error) {
	if o.hasQuestionFlags() {
		if o.file != "" {
			return nil, errors.New("--file cannot be combined with --state, --noul, --choice, or --score")
		}
		return requestFromFlags(o, stdin)
	}
	var data []byte
	var err error
	if o.file == "" || o.file == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(o.file)
	}
	if err != nil {
		return nil, fmt.Errorf("read request: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, errors.New("no request given: pass --file, pipe JSON to stdin, or use --state with --noul/--choice/--score")
	}
	return parseRequestJSON(data)
}

// parseRequestJSON decodes the HTTP request body shape:
// {"state": ..., "questions": {...}, "model": "..."} plus any extra fields.
func parseRequestJSON(data []byte) (*askRequest, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("invalid request JSON: %w", err)
	}
	req := &askRequest{Questions: typesafe.Questions{}, Extra: map[string]any{}}
	stateRaw, ok := top["state"]
	if !ok {
		return nil, errors.New(`invalid request: "state" is required`)
	}
	if err := json.Unmarshal(stateRaw, &req.State); err != nil {
		return nil, fmt.Errorf("invalid request: state: %w", err)
	}
	qRaw, ok := top["questions"]
	if !ok {
		return nil, errors.New(`invalid request: "questions" is required`)
	}
	var qs map[string]json.RawMessage
	if err := json.Unmarshal(qRaw, &qs); err != nil {
		return nil, fmt.Errorf("invalid request: questions: %w", err)
	}
	for key, raw := range qs {
		q, err := parseQuestion(key, raw)
		if err != nil {
			return nil, err
		}
		req.Questions[key] = q
	}
	if m, ok := top["model"]; ok {
		if err := json.Unmarshal(m, &req.Model); err != nil {
			return nil, fmt.Errorf("invalid request: model: %w", err)
		}
	}
	for k, v := range top {
		switch k {
		case "state", "questions", "model":
			continue
		}
		var any_ any
		if err := json.Unmarshal(v, &any_); err != nil {
			return nil, fmt.Errorf("invalid request: %s: %w", k, err)
		}
		req.Extra[k] = any_
	}
	return req, nil
}

func parseQuestion(key string, raw json.RawMessage) (typesafe.Question, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, fmt.Errorf("invalid request: questions.%s: must be an object with a \"type\"", key)
	}
	if head.Type == "" {
		return nil, fmt.Errorf("invalid request: questions.%s.type: is required", key)
	}
	switch head.Type {
	case "noul":
		var w struct {
			Instructions any `json:"instructions"`
			Criteria     *struct {
				True  any `json:"true"`
				False any `json:"false"`
			} `json:"criteria"`
		}
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("invalid request: questions.%s: %w", key, err)
		}
		q := typesafe.Noul{Instructions: w.Instructions}
		if w.Criteria != nil {
			q.Criteria = &typesafe.NoulCriteria{True: w.Criteria.True, False: w.Criteria.False}
		}
		return q, nil
	case "choice":
		var w struct {
			Instructions any                             `json:"instructions"`
			Criteria     map[string]typesafe.JSONContent `json:"criteria"`
		}
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("invalid request: questions.%s: %w", key, err)
		}
		return typesafe.Choice{Instructions: w.Instructions, Criteria: w.Criteria}, nil
	case "score":
		var w struct {
			Instructions any                    `json:"instructions"`
			Criteria     []typesafe.JSONContent `json:"criteria"`
		}
		if err := json.Unmarshal(raw, &w); err != nil {
			return nil, fmt.Errorf("invalid request: questions.%s: %w", key, err)
		}
		return typesafe.Score{Instructions: w.Instructions, Criteria: w.Criteria}, nil
	default:
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("invalid request: questions.%s: %w", key, err)
		}
		return typesafe.RawQuestion(m), nil
	}
}

// requestFromFlags builds a request from --state and repeated --noul,
// --choice, and --score flags.
func requestFromFlags(o *askOptions, stdin io.Reader) (*askRequest, error) {
	if o.state == "" {
		return nil, errors.New("--state is required when using --noul, --choice, or --score")
	}
	state, err := readStateFlag(o.state, stdin)
	if err != nil {
		return nil, err
	}
	req := &askRequest{State: state, Questions: typesafe.Questions{}, Extra: map[string]any{}}
	add := func(key string, q typesafe.Question) error {
		if _, dup := req.Questions[key]; dup {
			return fmt.Errorf("duplicate question key %q", key)
		}
		req.Questions[key] = q
		return nil
	}
	for _, s := range o.nouls {
		key, instr, err := splitKV(s, "--noul")
		if err != nil {
			return nil, err
		}
		if err := add(key, typesafe.Noul{Instructions: instr}); err != nil {
			return nil, err
		}
	}
	for _, s := range o.choices {
		key, rest, err := splitKV(s, "--choice")
		if err != nil {
			return nil, err
		}
		instr, labels, err := splitInstrLabels(rest, ",", "--choice")
		if err != nil {
			return nil, err
		}
		crit := make(map[string]typesafe.JSONContent, len(labels))
		for _, l := range labels {
			crit[l] = nil
		}
		if err := add(key, typesafe.Choice{Instructions: instr, Criteria: crit}); err != nil {
			return nil, err
		}
	}
	for _, s := range o.scores {
		key, rest, err := splitKV(s, "--score")
		if err != nil {
			return nil, err
		}
		instr, levels, err := splitInstrLabels(rest, "|", "--score")
		if err != nil {
			return nil, err
		}
		if len(levels) < 2 {
			return nil, fmt.Errorf("--score %q: needs at least two levels separated by |", s)
		}
		crit := make([]typesafe.JSONContent, len(levels))
		for i, l := range levels {
			crit[i] = l
		}
		if err := add(key, typesafe.Score{Instructions: instr, Criteria: crit}); err != nil {
			return nil, err
		}
	}
	return req, nil
}

func readStateFlag(v string, stdin io.Reader) (string, error) {
	switch {
	case v == "-":
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read state from stdin: %w", err)
		}
		return string(b), nil
	case strings.HasPrefix(v, "@"):
		b, err := os.ReadFile(v[1:])
		if err != nil {
			return "", fmt.Errorf("read state file: %w", err)
		}
		return string(b), nil
	default:
		return v, nil
	}
}

// splitKV splits "key=rest" on the first '='.
func splitKV(s, flag string) (string, string, error) {
	i := strings.Index(s, "=")
	if i <= 0 {
		return "", "", fmt.Errorf("%s %q: expected key=instructions", flag, s)
	}
	return strings.TrimSpace(s[:i]), s[i+1:], nil
}

// splitInstrLabels splits "instructions:l1<sep>l2" on the first ':' and
// then on sep, trimming each label and rejecting empties.
func splitInstrLabels(rest, sep, flag string) (string, []string, error) {
	i := strings.Index(rest, ":")
	if i < 0 {
		return "", nil, fmt.Errorf("%s %q: expected key=instructions:labels", flag, rest)
	}
	instr := strings.TrimSpace(rest[:i])
	var labels []string
	for _, l := range strings.Split(rest[i+1:], sep) {
		l = strings.TrimSpace(l)
		if l == "" {
			return "", nil, fmt.Errorf("%s %q: empty label", flag, rest)
		}
		labels = append(labels, l)
	}
	return instr, labels, nil
}
