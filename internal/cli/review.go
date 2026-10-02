package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/therealbill/typesafe-go"
	"github.com/therealbill/typesafe-go/internal/review"
)

type reviewOptions struct {
	units  string
	report string
	json   bool
	opts   review.Options
}

func newReviewCmd(g *globals, streams IO, getenv func(string) string) *cobra.Command {
	o := &reviewOptions{opts: review.DefaultOptions()}
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Ask Jev whether your tests cover the behaviors your spec requires",
		Long: `Review a codebase against its specification.

For each unit in the config file, jev review sends the named spec section and
the implementation and test files to Jev and asks whether the tests exercise
each listed behavior, whether the implementation contradicts the spec, how
thorough the tests are, and which area is weakest. It reports probabilities
and flags readings past the thresholds.

It does not find bugs, review style, check security, or verify that the code
is correct. Readings drift between runs on the same input.

Start with: jev review init`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runReview(cmd, g, o, streams, getenv)
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.units, "units", "jev-review.json", "config file; paths inside it resolve relative to the file")
	f.StringVar(&o.report, "report", "jev-review-report.json", "where to write the JSON report")
	f.BoolVar(&o.json, "json", false, "print the report JSON on stdout instead of the Markdown summary")
	f.StringVar(&o.opts.Only, "only", "", "run a single unit by name")
	f.Float64Var(&o.opts.MinCover, "min-cover", o.opts.MinCover, "flag a behavior whose coverage probability is below this")
	f.Float64Var(&o.opts.MaxContradict, "max-contradict", o.opts.MaxContradict, "flag a unit whose contradiction probability is above this")
	f.Float64Var(&o.opts.MinThorough, "min-thorough", o.opts.MinThorough, "flag a unit whose thoroughness score is below this")
	f.IntVar(&o.opts.MaxStateBytes, "max-state-bytes", o.opts.MaxStateBytes, "byte cap for implementation and tests together")
	f.DurationVar(&o.opts.UnitTimeout, "unit-timeout", o.opts.UnitTimeout, "time allowed for one unit's call")
	f.BoolVar(&o.opts.DryRun, "dry-run", false, "check the config, spec headings, and files without calling the API")
	cmd.AddCommand(newReviewInitCmd(streams))
	return cmd
}

func newReviewInitCmd(streams IO) *cobra.Command {
	var units string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a starter jev-review.json",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pretty, _ := cmd.Flags().GetBool("pretty")
			if _, err := os.Stat(units); err == nil {
				return fail(streams, pretty, &usageError{fmt.Errorf("%s already exists; remove it or pass --units", units)})
			}
			if err := os.WriteFile(units, review.Template(), 0o644); err != nil {
				return fail(streams, pretty, &usageError{err})
			}
			return writeJSON(streams.Out, map[string]string{"wrote": units}, pretty)
		},
	}
	cmd.Flags().StringVar(&units, "units", "jev-review.json", "path to write")
	return cmd
}

func runReview(cmd *cobra.Command, g *globals, o *reviewOptions, streams IO, getenv func(string) string) error {
	cfg, base, err := review.Load(o.units)
	if err != nil {
		return fail(streams, g.pretty, &usageError{err})
	}
	if o.opts.DryRun {
		return runDryRun(cmd.Context(), cfg, base, o, streams, g.pretty)
	}
	changed := cmd.Flags().Changed
	if err := g.validateFlags(changed); err != nil {
		return fail(streams, g.pretty, err)
	}
	ctx := cmd.Context()
	shutdown, inst := setupTelemetry(ctx, g, streams, getenv)
	defer shutdown()
	client, err := typesafe.NewClient(g.clientOptions(streams, inst, changed, getenv)...)
	if err != nil {
		return fail(streams, g.pretty, err)
	}
	defer func() { _ = client.Close() }()

	rep, err := review.Run(ctx, cfg, base, client, o.opts)
	if err != nil {
		return fail(streams, g.pretty, &usageError{err})
	}
	_, _ = fmt.Fprintf(streams.Err, "review: ran %d units\n", len(rep.Units))
	out, err := rep.JSON()
	if err != nil {
		return fail(streams, g.pretty, err)
	}
	if err := os.WriteFile(o.report, out, 0o644); err != nil {
		return fail(streams, g.pretty, &usageError{fmt.Errorf("write report: %w", err)})
	}
	if callErr := rep.FirstCallError(); callErr != nil {
		return fail(streams, g.pretty, callErr)
	}
	if o.json {
		_, _ = streams.Out.Write(out)
		_, _ = fmt.Fprintln(streams.Out)
	} else {
		_, _ = fmt.Fprint(streams.Out, rep.Markdown(o.opts))
	}
	if rep.Failing() {
		return flagged(streams, rep)
	}
	return nil
}

// runDryRun checks every unit up to the API call and prints the result. It
// builds no client, so no API key is needed, and it writes no report file.
func runDryRun(ctx context.Context, cfg *review.Config, base string, o *reviewOptions, streams IO, pretty bool) error {
	rep, err := review.Run(ctx, cfg, base, nil, o.opts)
	if err != nil {
		return fail(streams, pretty, &usageError{err})
	}
	_, _ = fmt.Fprintf(streams.Err, "review: checked %d units\n", len(rep.Units))
	if o.json {
		out, err := rep.JSON()
		if err != nil {
			return fail(streams, pretty, err)
		}
		_, _ = streams.Out.Write(out)
		_, _ = fmt.Fprintln(streams.Out)
	} else {
		_, _ = fmt.Fprint(streams.Out, rep.Markdown(o.opts))
	}
	if rep.Failing() {
		return flagged(streams, rep)
	}
	return nil
}

// flagged prints the failing count and returns the exit-8 error.
func flagged(streams IO, rep review.Report) error {
	failing := 0
	for _, u := range rep.Units {
		if u.Failing {
			failing++
		}
	}
	_, _ = fmt.Fprintf(streams.Err, "review: %d of %d units failing\n", failing, len(rep.Units))
	return &ExitError{Code: ExitFlagged, Kind: "flagged", Err: errors.New("review flagged at least one unit")}
}
