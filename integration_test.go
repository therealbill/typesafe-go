package typesafe

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestIntegrationLive talks to the real API. It is skipped unless
// TYPESAFE_API_KEY is set. Run with: go test -run Integration -v .
func TestIntegrationLive(t *testing.T) {
	if os.Getenv(EnvAPIKey) == "" {
		t.Skip("TYPESAFE_API_KEY not set")
	}
	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := c.SystemOne(ctx, fixtureState, fixtureQuestions)
	if err != nil {
		t.Fatalf("SystemOne: %v", err)
	}
	if res.RequestID == "" || res.Model == "" {
		t.Fatalf("missing metadata: %+v", res)
	}
	if len(res.Nouls()) != 1 || len(res.Choices()) != 1 || len(res.Scores()) != 1 {
		t.Fatalf("unexpected answer partition: %+v", res.Answers)
	}
	if p := res.Nouls()["billing"].Noul; p < 0.5 {
		t.Errorf("billing noul %.2f, expected a clear yes", p)
	}
	if ch := res.Choices()["tone"]; ch.Choice != "angry" {
		t.Errorf("tone %q, expected angry", ch.Choice)
	}
	if s := res.Scores()["urgency"]; s.Score < 1.0 || len(s.Legend) != 3 {
		t.Errorf("urgency %+v", s)
	}
	if res.Usage.InputTokens == nil || *res.Usage.InputTokens == 0 {
		t.Errorf("usage %+v", res.Usage)
	}

	models, err := c.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models.Models) == 0 {
		t.Fatal("no models returned")
	}
}
