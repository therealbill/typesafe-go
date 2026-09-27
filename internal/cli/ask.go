package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/therealbill/typesafe-go"
)

func newAskCmd(g *globals, streams IO, getenv func(string) string) *cobra.Command {
	o := &askOptions{}
	cmd := &cobra.Command{
		Use:   "ask",
		Short: "Send a state and typed questions, print the answers as JSON",
		Long: `Send one System One request and print the response as JSON.

Two input modes:

  JSON   jev ask -f request.json        (or pipe the JSON to stdin)
         The document is the HTTP body shape:
         {"state": ..., "questions": {"id": {"type": "noul", ...}}, "model": "..."}

  Flags  jev ask --state "text" --noul billing="Is this about billing?" \
             --choice tone="What is the tone?:calm,angry" \
             --score urgency="How urgent?:low|medium|high"

Output: {"model": ..., "answers": {...}, "usage": {...}, "request_id": ...}`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAsk(cmd, g, o, streams, getenv)
		},
	}
	f := cmd.Flags()
	f.StringVarP(&o.file, "file", "f", "", "request JSON file; '-' or omitted reads stdin")
	f.StringVar(&o.state, "state", "", "state text, @path to read a file, or '-' for stdin")
	f.StringArrayVar(&o.nouls, "noul", nil, "key=instructions (repeatable)")
	f.StringArrayVar(&o.choices, "choice", nil, "key=instructions:label1,label2,... (repeatable)")
	f.StringArrayVar(&o.scores, "score", nil, "key=instructions:level0|level1|... (repeatable)")
	f.BoolVar(&o.raw, "raw", false, "print the server response body unchanged")
	return cmd
}

func runAsk(cmd *cobra.Command, g *globals, o *askOptions, streams IO, getenv func(string) string) error {
	req, err := buildRequest(o, streams.In)
	if err != nil {
		return fail(streams, g.pretty, &usageError{err})
	}
	ctx := cmd.Context()
	shutdown, inst := setupTelemetry(ctx, g, streams, getenv)
	defer shutdown()

	client, err := typesafe.NewClient(g.clientOptions(streams, inst)...)
	if err != nil {
		return fail(streams, g.pretty, err)
	}
	defer client.Close()

	var ropts []typesafe.RequestOption
	if req.Model != "" && !cmd.Flags().Changed("model") {
		ropts = append(ropts, typesafe.WithRequestModel(req.Model))
	}
	if len(req.Extra) > 0 {
		ropts = append(ropts, typesafe.WithExtraBody(req.Extra))
	}
	res, err := client.SystemOne(ctx, req.State, req.Questions, ropts...)
	if err != nil {
		return fail(streams, g.pretty, err)
	}
	if o.raw {
		if _, err := streams.Out.Write(res.Raw.Body); err != nil {
			return err
		}
		_, err = fmt.Fprintln(streams.Out)
		return err
	}
	return writeJSON(streams.Out, res, g.pretty)
}
