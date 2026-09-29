package cli

import (
	"runtime"

	"github.com/spf13/cobra"

	"github.com/therealbill/typesafe-go/internal/version"
)

func newVersionCmd(streams IO) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pretty, _ := cmd.Flags().GetBool("pretty")
			info := version.Get()
			return writeJSON(streams.Out, map[string]any{
				"version":  info.Version,
				"commit":   info.Commit,
				"modified": info.Modified,
				"source":   info.Source,
				"go":       runtime.Version(),
			}, pretty)
		},
	}
}
