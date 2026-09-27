package cli

import (
	"github.com/spf13/cobra"

	"github.com/therealbill/typesafe-go"
)

func newModelsCmd(g *globals, streams IO, getenv func(string) string) *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List the models available to the account as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			changed := cmd.Flags().Changed
			if err := g.validateFlags(changed); err != nil {
				return fail(streams, g.pretty, err)
			}
			ctx := cmd.Context()
			shutdown, inst := setupTelemetry(ctx, g, streams, getenv)
			defer shutdown()
			client, err := typesafe.NewClient(g.clientOptions(streams, inst, changed)...)
			if err != nil {
				return fail(streams, g.pretty, err)
			}
			defer func() { _ = client.Close() }()
			res, err := client.ListModels(ctx)
			if err != nil {
				return fail(streams, g.pretty, err)
			}
			return writeJSON(streams.Out, res, g.pretty)
		},
	}
}
