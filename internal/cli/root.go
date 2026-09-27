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
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	pf := root.PersistentFlags()
	pf.StringVar(&g.apiKey, "api-key", "", "TypeSafe API key (env TYPESAFE_API_KEY)")
	pf.StringVar(&g.baseURL, "base-url", "", "API base URL (env TYPESAFE_BASE_URL)")
	pf.StringVar(&g.model, "model", "", "model name (env TYPESAFE_DEFAULT_MODEL; default jev-latest)")
	pf.DurationVar(&g.timeout, "timeout", 0, "per-attempt HTTP timeout (default 10s)")
	pf.IntVar(&g.maxRetries, "max-retries", -1, "retries after the first attempt (default 2)")
	pf.StringVar(&g.logLevel, "log-level", "", "debug|info|warning|error|off (env TYPESAFE_LOG_LEVEL)")
	pf.BoolVar(&g.trace, "trace", false, "force OpenTelemetry tracing on")
	pf.BoolVar(&g.noTrace, "no-trace", false, "disable tracing even when HONEYCOMB_API_KEY or OTEL_* is set")
	pf.BoolVar(&g.pretty, "pretty", false, "indent JSON output")

	root.SetIn(streams.In)
	root.SetOut(streams.Out)
	root.SetErr(streams.Err)
	root.AddCommand(newAskCmd(g, streams, getenv), newModelsCmd(g, streams, getenv), newVersionCmd(streams))
	return root
}

// clientOptions turns the globals into client options.
func (g *globals) clientOptions(streams IO, inst typesafe.Instrumentation) []typesafe.Option {
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
	if g.timeout > 0 {
		opts = append(opts, typesafe.WithTimeout(g.timeout))
	}
	if g.maxRetries >= 0 {
		p := typesafe.DefaultRetryPolicy()
		p.MaxRetries = g.maxRetries
		opts = append(opts, typesafe.WithRetryPolicy(p))
	}
	if g.logLevel != "" {
		opts = append(opts, typesafe.WithLogger(typesafe.NewLogger(streams.Err, g.logLevel)))
	}
	if inst != nil {
		opts = append(opts, typesafe.WithInstrumentation(inst))
	}
	return opts
}

// Main runs the command and returns the process exit code.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
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
	fmt.Fprintf(stderr, "jev: %v\nRun 'jev --help' for usage.\n", err)
	return ExitUsage
}
