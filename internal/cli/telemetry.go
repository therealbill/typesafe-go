package cli

import (
	"context"

	"github.com/therealbill/typesafe-go"
)

// setupTelemetry is completed in Task 18. This stub keeps tracing off.
func setupTelemetry(ctx context.Context, g *globals, streams IO, getenv func(string) string) (func(), typesafe.Instrumentation) {
	return func() {}, nil
}
