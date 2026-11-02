package fragments

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/model"
)

type collectingSubmitter struct {
	inputs chan Input
}

func (s collectingSubmitter) SubmitFragment(ctx context.Context, input Input) (model.Fragment, error) {
	select {
	case s.inputs <- input:
		return model.Fragment{ID: "ULY-TEST"}, nil
	case <-ctx.Done():
		return model.Fragment{}, ctx.Err()
	}
}

func TestInternalRandomSourceProducesValidFragments(t *testing.T) {
	cfg := config.Defaults()
	cfg.Fragments.InternalRNG.Interval = time.Hour
	source := NewInternalRandomSource(cfg)
	submitter := collectingSubmitter{inputs: make(chan Input, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- source.Start(ctx, submitter)
	}()
	var input Input
	select {
	case input = <-submitter.inputs:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for internal RNG fragment")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("source returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("source did not stop after context cancellation")
	}
	if input.Bearing < 0 || input.Bearing > 359 {
		t.Fatalf("bearing outside range: %f", input.Bearing)
	}
	if input.Elevation < 5 || input.Elevation > 85 {
		t.Fatalf("elevation outside normal range: %f", input.Elevation)
	}
	if input.MapX < 0 || input.MapX > 100 || input.MapY < 0 || input.MapY > 100 {
		t.Fatalf("map coordinates outside range: %f/%f", input.MapX, input.MapY)
	}
	if input.Priority == "" || input.TerrainType == "" || input.StrategicZone == "" {
		t.Fatalf("expected priority, terrain, and strategic zone: %+v", input)
	}
}

func TestExternalIngestSourceGracefulShutdown(t *testing.T) {
	source := NewExternalIngestSource(config.Defaults().Fragments.Ingest)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- source.Start(ctx, collectingSubmitter{inputs: make(chan Input, 1)})
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("source returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("source did not stop after context cancellation")
	}
}

func TestExternalIDRetentionIsBounded(t *testing.T) {
	source := NewExternalIngestSource(config.Defaults().Fragments.Ingest)
	now := time.Now().UTC()
	for i := 0; i < maxRetainedExternalIDs; i++ {
		source.seen[fmt.Sprintf("seen-%d", i)] = now
	}
	_, err := source.Submit(context.Background(), Input{ExternalID: "overflow"})
	if !errors.Is(err, ErrExternalIDCapacity) {
		t.Fatalf("error = %v", err)
	}
	if len(source.seen) != maxRetainedExternalIDs {
		t.Fatalf("retained external IDs = %d", len(source.seen))
	}
}
