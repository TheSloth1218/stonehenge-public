package history

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/model"
)

func TestPostgresEventMappingIntegration(t *testing.T) {
	dsn := os.Getenv("STONEHENGE_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set STONEHENGE_POSTGRES_TEST_DSN to run postgres integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()
	if err := RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE gun_runtime_summary, shot_records, gun_events, fragment_tracks, schema_migrations RESTART IDENTITY`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if err := RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("rerun migrations: %v", err)
	}

	sink := &PostgresHistorySink{
		cfg: config.HistoryConfig{
			RetryInterval:  time.Millisecond,
			WriteQueueSize: 10,
		},
		pool:     pool,
		queue:    make(chan model.OperationalEvent, 10),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		logger:   slog.Default(),
		migrated: true,
	}

	fragmentID := "ULY-0001"
	externalID := "USEA-TRACKING-NET-000001"
	now := time.Now().UTC().Truncate(time.Microsecond)
	received := model.OperationalEvent{
		ID:                 1,
		EventUUID:          deterministicUUID("test-event", "received"),
		Timestamp:          now,
		EventType:          model.EventFragmentReceived,
		Severity:           model.SeverityInfo,
		FragmentID:         &fragmentID,
		ExternalFragmentID: &externalID,
		Azimuth:            floatPtr(247.3),
		Elevation:          floatPtr(41.8),
		Details: map[string]interface{}{
			"map_x":          34.2,
			"map_y":          52.7,
			"priority":       model.PriorityHigh,
			"terrain_type":   "LAND",
			"strategic_zone": "SAN_SALVACION",
			"observed_at":    now.Add(-time.Second),
			"impact_at":      now.Add(75 * time.Second),
			"status":         model.FragmentActive,
		},
	}
	if err := sink.writeEvent(ctx, received); err != nil {
		t.Fatalf("write received: %v", err)
	}
	gunID := 3
	assigned := received
	assigned.ID = 2
	assigned.EventUUID = deterministicUUID("test-event", "assigned")
	assigned.EventType = model.EventTargetAssigned
	assigned.GunID = &gunID
	if err := sink.writeEvent(ctx, assigned); err != nil {
		t.Fatalf("write assigned: %v", err)
	}
	shell := model.ShellStandard
	shotSequence := uint64(1)
	fired := model.OperationalEvent{
		ID:                 3,
		EventUUID:          deterministicUUID("test-event", "fired"),
		Timestamp:          now.Add(2 * time.Second),
		EventType:          model.EventGunFired,
		Severity:           model.SeverityInfo,
		GunID:              &gunID,
		FragmentID:         &fragmentID,
		ExternalFragmentID: &externalID,
		Azimuth:            floatPtr(247.3),
		Elevation:          floatPtr(41.8),
		ShellType:          &shell,
		ShotSequence:       &shotSequence,
	}
	if err := sink.writeEvent(ctx, fired); err != nil {
		t.Fatalf("write fired: %v", err)
	}
	if err := sink.writeEvent(ctx, fired); err != nil {
		t.Fatalf("retry fired: %v", err)
	}
	intercept := fired
	intercept.ID = 4
	intercept.EventUUID = deterministicUUID("test-event", "intercept")
	intercept.EventType = model.EventInterceptRecorded
	if err := sink.writeEvent(ctx, intercept); err != nil {
		t.Fatalf("write intercept: %v", err)
	}

	fragments, err := sink.ListFragments(ctx, FragmentFilter{Limit: 10, Status: string(model.FragmentIntercepted)})
	if err != nil {
		t.Fatalf("list fragments: %v", err)
	}
	if len(fragments) != 1 || fragments[0].AssignedGunID == nil || *fragments[0].AssignedGunID != gunID {
		t.Fatalf("unexpected fragments: %+v", fragments)
	}
	shots, err := sink.ListShots(ctx, ShotFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list shots: %v", err)
	}
	if len(shots) != 1 {
		t.Fatalf("shots len = %d, want 1", len(shots))
	}
	summary, found, err := sink.GetGunSummary(ctx, gunID)
	if err != nil || !found {
		t.Fatalf("summary found=%v err=%v", found, err)
	}
	if summary.TotalShots != 1 || summary.StandardShellsFired != 1 {
		t.Fatalf("summary double counted: %+v", summary)
	}
	events, err := sink.ListGunEvents(ctx, gunID, EventFilter{Limit: 10, EventType: string(model.EventGunFired)})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
}

func floatPtr(value float64) *float64 {
	return &value
}
