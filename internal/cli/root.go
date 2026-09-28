// Package cli implements the jev command.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/therealbill/typesafe-go"
	"github.com/therealbill/typesafe-go/internal/version"
)

// globals are the persistent flags shared by every subcommand.
type globals struct {
	apiKey     string
	baseURL    string
	model      string
	timeout    time.Duration
	maxRetries int
	logLevel   string
	trace      bool
	noTrace    bool
	pretty     bool
}

// NewRootCmd builds the command tree. getenv is used for telemetry decisions
// so tests can control the environment.
func NewRootCmd(streams IO, getenv func(string) string) *cobra.Command {
	g := &globals{}
	root := &cobra.Command{
		Use:           "jev",
		Short:         "Ask TypeSafe's Jev model typed questions from the command line",
		Long:          "jev sends a state and a set of typed questions (noul, choice, score) to the TypeSafe System One API and prints the answers as JSON.",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	pf := root.PersistentFlags()
	pf.StringVar(&g.apiKey, "api-key", "", "TypeSafe API key (env TYPESAFE_API_KEY)")
	pf.StringVar(&g.baseURL, "base-url", "", "API base URL (env TYPESAFE_BASE_URL)")
	pf.StringVar(&g.model, "model", "", "model name (env TYPESAFE_DEFAULT_MODEL; default jev-latest)")
	pf.DurationVar(&g.timeout, "timeout", 0, "per-attempt HTTP timeout (default 10s)")
	pf.IntVar(&g.maxRetries, "max-retries", 2, "retries after the first attempt")
	pf.StringVar(&g.logLevel, "log-level", "", "debug|info|warning|error|off (env TYPESAFE_LOG_LEVEL)")
	pf.BoolVar(&g.trace, "trace", false, "force OpenTelemetry tracing on")
	pf.BoolVar(&g.noTrace, "no-trace", false, "disable tracing even when HONEYCOMB_API_KEY or OTEL_* is set")
	pf.BoolVar(&g.pretty, "pretty", false, "indent JSON output")

	root.SetIn(streams.In)
	root.SetOut(streams.Out)
	root.SetErr(streams.Err)
	root.AddCommand(newAskCmd(g, streams, getenv), newModelsCmd(g, streams, getenv), newReviewCmd(g, streams, getenv), newVersionCmd(streams))
	return root
}

// validateFlags rejects numeric flags the library cannot honor. changed
// reports whether the named flag was given explicitly.
func (g *globals) validateFlags(changed func(string) bool) error {
	if changed("timeout") && g.timeout < 0 {
		return &usageError{fmt.Errorf("--timeout must not be negative, got %s", g.timeout)}
	}
	if changed("max-retries") && g.maxRetries < 0 {
		return &usageError{fmt.Errorf("--max-retries must not be negative, got %d", g.maxRetries)}
	}
	return nil
}

// clientOptions turns the globals into client options. changed reports
// whether the named flag was given explicitly, so an explicit zero reaches
// the library instead of being mistaken for "unset". getenv supplies the
// log level when --log-level was not given.
func (g *globals) clientOptions(streams IO, inst typesafe.Instrumentation, changed func(string) bool, getenv func(string) string) []typesafe.Option {
	var opts []typesafe.Option
	if g.apiKey != "" {
		opts = append(opts, typesafe.WithAPIKey(g.apiKey))
	}
	if g.baseURL != "" {
		opts = append(opts, typesafe.WithBaseURL(g.baseURL))
	}
	if g.model != "" {
		opts = append(opts, typesafe.WithModel(g.model))
	}
	if changed("timeout") {
		opts = append(opts, typesafe.WithTimeout(g.timeout))
	}
	if changed("max-retries") {
		p := typesafe.DefaultRetryPolicy()
		p.MaxRetries = g.maxRetries
		opts = append(opts, typesafe.WithRetryPolicy(p))
	}
	// A logger is always injected so that env-driven logging lands on the
	// stream the caller supplied rather than on the process's own stderr.
	// NewLogger discards everything for an empty or unrecognized level.
	level := g.logLevel
	if level == "" {
		level = getenv(typesafe.EnvLogLevel)
	}
	opts = append(opts, typesafe.WithLogger(typesafe.NewLogger(streams.Err, level)))
	if inst != nil {
		opts = append(opts, typesafe.WithInstrumentation(inst))
	}
	return opts
}

// Main runs the command and returns the process exit code. It installs the
// signal handler and then defers to runMain.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runMain(ctx, args, stdin, stdout, stderr)
}

// runMain runs the command under ctx and returns the process exit code. It
// exists so tests can cancel the context that Main derives from signals.
func runMain(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	root := NewRootCmd(IO{In: stdin, Out: stdout, Err: stderr}, os.Getenv)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return ExitOK
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	// Cobra reported a bad flag, an unknown command, or bad arguments. Emit
	// the same error envelope a request failure would, so a caller parsing
	// stdout never has to special-case invocation mistakes.
	pretty, _ := root.PersistentFlags().GetBool("pretty")
	_ = writeJSON(stdout, map[string]any{"error": errorPayload{Kind: "usage", Message: err.Error()}}, pretty)
	_, _ = fmt.Fprintf(stderr, "jev: %s\nRun 'jev --help' for usage.\n", sanitize(err.Error()))
	return ExitUsage
}
