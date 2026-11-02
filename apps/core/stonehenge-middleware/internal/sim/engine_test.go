package sim

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/fragments"
	"stonehenge-middleware/internal/history"
	"stonehenge-middleware/internal/model"
	"stonehenge-middleware/internal/plc"
	"stonehenge-middleware/internal/safety"
	"stonehenge-middleware/internal/store"
)

func testEngine() (*Engine, *store.MemoryStore) {
	engine, st, _ := testEngineWithHistory()
	return engine, st
}

func testEngineWithHistory() (*Engine, *store.MemoryStore, *history.InMemorySink) {
	cfg := config.Defaults()
	cfg.Simulation.AutoStart = false
	cfg.Fragments.Source = "ingest"
	cfg.Fragments.InternalRNG.Enabled = false
	cfg.Simulation.MinSlewDuration = time.Second
	cfg.Simulation.MaxSlewDuration = 12 * time.Second
	cfg.Simulation.SlewDegreesPerSecond = 10
	st := store.NewMemoryStore(100)
	historySink := history.NewInMemorySink(100)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	source := fragments.NewExternalIngestSource(cfg.Fragments.Ingest)
	engine := NewEngine(cfg, st, plc.NewSimulatedAdapter(), safety.NewSimulatedController(cfg.Safety), source, historySink, logger)
	return engine, st, historySink
}

func TestExternalIngestAcceptsValidFragment(t *testing.T) {
	engine, _ := testEngine()
	engine.Start()
	defer engine.Stop()
	fragment, err := engine.IngestFragment(context.Background(), fragments.Input{
		ExternalID:          "ULY-2026-00421",
		Bearing:             247.3,
		Elevation:           41.8,
		MapX:                34.2,
		MapY:                52.7,
		Priority:            model.PriorityHigh,
		TerrainType:         "LAND",
		StrategicZone:       "SAN_SALVACION",
		TimeToImpactSeconds: 75,
		ObservedAt:          time.Date(2026, 6, 26, 14, 30, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("IngestFragment failed: %v", err)
	}
	if fragment.ExternalID != "ULY-2026-00421" {
		t.Fatalf("external id = %q", fragment.ExternalID)
	}
}

func TestExternalIngestRejectsInvalidValues(t *testing.T) {
	engine, _ := testEngine()
	engine.Start()
	defer engine.Stop()
	_, err := engine.IngestFragment(context.Background(), fragments.Input{
		ExternalID:          "BAD-1",
		Bearing:             400,
		Elevation:           45,
		MapX:                50,
		MapY:                50,
		Priority:            model.PriorityHigh,
		TerrainType:         "LAND",
		StrategicZone:       "TEST",
		TimeToImpactSeconds: 30,
		ObservedAt:          time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected invalid bearing to be rejected")
	}
}

func TestExternalIngestRejectsOversizedRetainedFields(t *testing.T) {
	for _, input := range []fragments.Input{
		{ExternalID: strings.Repeat("x", maxExternalFragmentIDBytes+1), TerrainType: "LAND", StrategicZone: "TEST"},
		{ExternalID: "LONG-TERRAIN", TerrainType: strings.Repeat("x", maxFragmentLabelBytes+1), StrategicZone: "TEST"},
		{ExternalID: "LONG-ZONE", TerrainType: "LAND", StrategicZone: strings.Repeat("x", maxFragmentLabelBytes+1)},
	} {
		engine, retained := testEngine()
		input.Bearing = 100
		input.Elevation = 45
		input.MapX = 50
		input.MapY = 50
		input.Priority = model.PriorityHigh
		input.TimeToImpactSeconds = 30
		input.ObservedAt = time.Now().UTC()
		if _, err := engine.SubmitFragment(context.Background(), input); err == nil {
			t.Fatal("oversized retained field was accepted")
		}
		if len(retained.Fragments()) != 0 {
			t.Fatal("rejected fragment mutated live retention")
		}
	}
}

func TestRejectedInputIsRecordedOnceWithBoundedFields(t *testing.T) {
	engine, retained, historySink := testEngineWithHistory()
	engine.Start()
	defer engine.Stop()
	input := fragments.Input{
		ExternalID:          strings.Repeat("€", maxExternalFragmentIDBytes+1),
		Bearing:             100,
		Elevation:           45,
		MapX:                50,
		MapY:                50,
		Priority:            model.PriorityHigh,
		TerrainType:         strings.Repeat("€", maxFragmentLabelBytes+1),
		StrategicZone:       strings.Repeat("€", maxFragmentLabelBytes+1),
		TimeToImpactSeconds: 30,
		ObservedAt:          time.Now().UTC(),
	}
	if _, err := engine.IngestFragment(context.Background(), input); err == nil {
		t.Fatal("oversized input was accepted")
	}
	for name, events := range map[string][]model.Event{
		"live":    retained.Events(0),
		"history": historySink.Events(),
	} {
		var rejected []model.Event
		for _, event := range events {
			if event.EventType == model.EventFragmentRejected {
				rejected = append(rejected, event)
			}
		}
		if len(rejected) != 1 {
			t.Fatalf("%s rejection event count = %d", name, len(rejected))
		}
		event := rejected[0]
		if event.ExternalFragmentID == nil || len(*event.ExternalFragmentID) > maxExternalFragmentIDBytes || !utf8.ValidString(*event.ExternalFragmentID) {
			t.Fatalf("%s external ID was not safely bounded", name)
		}
		if len(event.Message) > maxRejectedReasonBytes {
			t.Fatalf("%s rejection reason length = %d", name, len(event.Message))
		}
		for _, field := range []string{"terrain_type", "strategic_zone"} {
			value, ok := event.Details[field].(string)
			if !ok || len(value) > maxFragmentLabelBytes || !utf8.ValidString(value) {
				t.Fatalf("%s %s was not safely bounded", name, field)
			}
		}
	}
}

func TestExternalIngestRejectsDuplicateExternalID(t *testing.T) {
	engine, _ := testEngine()
	engine.Start()
	defer engine.Stop()
	input := fragments.Input{
		ExternalID:          "DUP-1",
		Bearing:             100,
		Elevation:           45,
		MapX:                50,
		MapY:                50,
		Priority:            model.PriorityMedium,
		TerrainType:         "LAND",
		StrategicZone:       "TEST",
		TimeToImpactSeconds: 30,
		ObservedAt:          time.Now().UTC(),
	}
	if _, err := engine.IngestFragment(context.Background(), input); err != nil {
		t.Fatalf("first ingest failed: %v", err)
	}
	if _, err := engine.IngestFragment(context.Background(), input); err == nil {
		t.Fatal("expected duplicate external_id to be rejected")
	}
}

func TestAcceptedExternalFragmentUsesAssignmentPipeline(t *testing.T) {
	engine, st := testEngine()
	engine.Start()
	defer engine.Stop()
	fragment, err := engine.IngestFragment(context.Background(), fragments.Input{
		ExternalID:          "ASSIGN-1",
		Bearing:             20,
		Elevation:           45,
		MapX:                30,
		MapY:                50,
		Priority:            model.PriorityHigh,
		TerrainType:         "LAND",
		StrategicZone:       "STONEHENGE",
		TimeToImpactSeconds: 75,
		ObservedAt:          time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ingest failed: %v", err)
	}
	engine.Tick(time.Now().UTC())
	fragment, _ = st.Fragment(fragment.ID)
	if fragment.Status != model.FragmentAssigned || fragment.AssignedGunID == nil {
		t.Fatalf("expected fragment to be assigned, got status=%s gun=%v", fragment.Status, fragment.AssignedGunID)
	}
}

func TestTargetGeneration(t *testing.T) {
	engine, st := testEngine()
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	fragment, err := engine.SubmitFragment(context.Background(), fragments.Input{
		Bearing:                120,
		Elevation:              45,
		MapX:                   30,
		MapY:                   50,
		Priority:               model.PriorityHigh,
		TerrainType:            "LAND",
		StrategicZone:          "STONEHENGE",
		TimeToImpactSeconds:    75,
		ObservedAt:             now,
		GeneratedByInternalRNG: true,
	})
	if err != nil {
		t.Fatalf("SubmitFragment failed: %v", err)
	}
	if fragment.ID != "ULY-0001" {
		t.Fatalf("unexpected fragment id %s", fragment.ID)
	}
	if fragment.Azimuth < 0 || fragment.Azimuth > 359 {
		t.Fatalf("azimuth outside range: %f", fragment.Azimuth)
	}
	if fragment.Elevation < 5 || fragment.Elevation > 85 {
		t.Fatalf("elevation outside normal range: %f", fragment.Elevation)
	}
	if fragment.TerrainType == "" {
		t.Fatal("expected generated fragment to include terrain type")
	}
	if fragment.StrategicZone == "" {
		t.Fatal("expected generated fragment to include strategic zone")
	}
	if len(st.Fragments()) != 1 {
		t.Fatal("expected generated fragment to be stored")
	}
}

func TestLeastSlewAssignment(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	guns := []model.GunState{
		{ID: 1, Mode: model.ModeAuto, State: model.GunIdle, SectorStart: 0, SectorEnd: 180, CurrentAzimuth: 10, LastIdleTime: now.Add(-time.Minute)},
		{ID: 2, Mode: model.ModeAuto, State: model.GunIdle, SectorStart: 0, SectorEnd: 180, CurrentAzimuth: 80, LastIdleTime: now.Add(-2 * time.Minute)},
	}
	fragment := model.Fragment{Azimuth: 70}
	gun, ok := SelectGunForFragment(guns, fragment, now)
	if !ok {
		t.Fatal("expected eligible gun")
	}
	if gun.ID != 2 {
		t.Fatalf("expected gun 2 with least slew, got %d", gun.ID)
	}
}

func TestPostFireTenSecondLockout(t *testing.T) {
	engine, st := testEngine()
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	gun, _ := st.Gun(1)
	gun.State = model.GunReadyToFire
	gun.TargetElevation = 45
	gun.TargetAzimuth = 20
	st.SaveGun(gun)
	if _, err := engine.FireRequest(1); err != nil {
		t.Fatalf("fire request failed: %v", err)
	}
	engine.Tick(now)
	gun, _ = st.Gun(1)
	if gun.State != model.GunPostFireLockout {
		t.Fatalf("expected post-fire lockout, got %s", gun.State)
	}
	if gun.PostFireLockoutUntil == nil || !gun.PostFireLockoutUntil.Equal(now.Add(10*time.Second)) {
		t.Fatalf("unexpected lockout until %v", gun.PostFireLockoutUntil)
	}
	engine.Tick(now.Add(9 * time.Second))
	gun, _ = st.Gun(1)
	if gun.State != model.GunPostFireLockout {
		t.Fatalf("expected lockout before 10 seconds, got %s", gun.State)
	}
	engine.Tick(now.Add(10 * time.Second))
	gun, _ = st.Gun(1)
	if gun.State != model.GunIdle {
		t.Fatalf("expected idle after lockout, got %s", gun.State)
	}
}

func TestFiveSecondSettlingDelay(t *testing.T) {
	engine, st := testEngine()
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	gun, _ := st.Gun(1)
	gun.State = model.GunSlewing
	gun.TargetAzimuth = gun.CurrentAzimuth
	gun.TargetElevation = gun.CurrentElevation
	completeAt := now
	gun.SlewCompleteAt = &completeAt
	st.SaveGun(gun)
	engine.Tick(now)
	gun, _ = st.Gun(1)
	if gun.State != model.GunOnTargetSettling {
		t.Fatalf("expected settling, got %s", gun.State)
	}
	if gun.SettlingUntil == nil || !gun.SettlingUntil.Equal(now.Add(5*time.Second)) {
		t.Fatalf("unexpected settling until %v", gun.SettlingUntil)
	}
	engine.Tick(now.Add(4 * time.Second))
	gun, _ = st.Gun(1)
	if gun.State != model.GunOnTargetSettling {
		t.Fatalf("expected still settling, got %s", gun.State)
	}
	engine.Tick(now.Add(5 * time.Second))
	gun, _ = st.Gun(1)
	if gun.State != model.GunReadyToFire {
		t.Fatalf("expected ready to fire, got %s", gun.State)
	}
}

func TestSoftLimitRejection(t *testing.T) {
	engine, _ := testEngine()
	if _, err := engine.SetMode(1, model.ModeManual); err != nil {
		t.Fatalf("set mode failed: %v", err)
	}
	if _, err := engine.SetManualTarget(1, 20, 2); err == nil {
		t.Fatal("expected soft limit rejection")
	}
}

func TestMaintenanceGunsAreNotAutomaticallyAssigned(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	guns := []model.GunState{
		{ID: 1, Mode: model.ModeMaintenance, State: model.GunManual, SectorStart: 0, SectorEnd: 180, CurrentAzimuth: 10, LastIdleTime: now.Add(-time.Minute)},
	}
	fragment := model.Fragment{Azimuth: 20}
	if _, ok := SelectGunForFragment(guns, fragment, now); ok {
		t.Fatal("maintenance gun should not be eligible for automatic assignment")
	}
}

func TestMaintenanceDoesNotBypassSoftLimits(t *testing.T) {
	engine, _ := testEngine()
	if _, err := engine.SetMode(1, model.ModeMaintenance); err != nil {
		t.Fatalf("set maintenance failed: %v", err)
	}
	if _, err := engine.SetManualTarget(1, 20, -5); err == nil {
		t.Fatal("expected maintenance manual target below soft limit to be rejected")
	}
}

func TestAdminOverrideStateIsSeparateFromMaintenance(t *testing.T) {
	engine, _ := testEngine()
	gun, err := engine.SetMode(1, model.ModeMaintenance)
	if err != nil {
		t.Fatalf("set maintenance failed: %v", err)
	}
	if gun.AdminOverrideActive {
		t.Fatal("maintenance should not activate admin override")
	}
	gun, err = engine.SetAdminOverride(1, true, "test-approver", nil)
	if err != nil {
		t.Fatalf("set admin override failed: %v", err)
	}
	if !gun.MaintenanceEnabled || !gun.AdminOverrideActive {
		t.Fatalf("expected maintenance and admin override to be separate active fields: %+v", gun)
	}
}

func TestShotSequenceIncrementsOncePerFiringCycle(t *testing.T) {
	engine, st := testEngine()
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	gun, _ := st.Gun(1)
	gun.State = model.GunReadyToFire
	gun.TargetElevation = 45
	gun.TargetAzimuth = 20
	st.SaveGun(gun)
	if _, err := engine.FireRequest(1); err != nil {
		t.Fatalf("fire request failed: %v", err)
	}
	engine.Tick(now)
	gun, _ = st.Gun(1)
	if gun.ShotSequence != 1 {
		t.Fatalf("shot sequence = %d, want 1", gun.ShotSequence)
	}
	engine.Tick(now.Add(time.Second))
	gun, _ = st.Gun(1)
	if gun.ShotSequence != 1 {
		t.Fatalf("shot sequence double-counted to %d", gun.ShotSequence)
	}
}

func TestServiceStartsWithDefaultConfiguration(t *testing.T) {
	cfg := config.Defaults()
	cfg.Simulation.AutoStart = false
	cfg.Fragments.InternalRNG.Interval = time.Hour
	st := store.NewMemoryStore(cfg.History.MaxEvents)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	source, err := fragments.NewSource(cfg)
	if err != nil {
		t.Fatalf("NewSource failed: %v", err)
	}
	engine := NewEngine(cfg, st, plc.NewSimulatedAdapter(), safety.NewSimulatedController(cfg.Safety), source, history.NewInMemorySink(100), logger)
	engine.Start()
	engine.Stop()
}

func TestResetUsesConfiguredAutoBoundary(t *testing.T) {
	cfg := config.Defaults()
	cfg.Simulation.AutoStart = false
	cfg.Simulation.GunCount = 8
	cfg.Guns = make([]config.GunConfig, 0, 8)
	for id := 1; id <= 8; id++ {
		plcCfg := cfg.PLC
		plcCfg.Host = fmt.Sprintf("gun-%d", id)
		cfg.Guns = append(cfg.Guns, config.GunConfig{
			ID:          id,
			Name:        fmt.Sprintf("Gun %d", id),
			PLCEnabled:  true,
			SafetyGunID: id,
			AutoEnabled: id == 1,
			PLC:         plcCfg,
		})
	}
	st := store.NewMemoryStore(cfg.History.MaxEvents)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	source, err := fragments.NewSource(cfg)
	if err != nil {
		t.Fatalf("NewSource failed: %v", err)
	}
	NewEngine(cfg, st, plc.NewSimulatedAdapter(), safety.NewSimulatedController(cfg.Safety), source, history.NewInMemorySink(100), logger)

	for _, gun := range st.Guns() {
		wantMode := model.ModeManual
		wantState := model.GunManual
		if gun.ID == 1 {
			wantMode = model.ModeAuto
			wantState = model.GunIdle
		}
		if gun.Mode != wantMode || gun.State != wantState {
			t.Fatalf("gun %d mode/state = %s/%s, want %s/%s", gun.ID, gun.Mode, gun.State, wantMode, wantState)
		}
	}
}

func TestIngestModeStartsWithoutGeneratingInternalFragments(t *testing.T) {
	cfg := config.Defaults()
	cfg.Fragments.Source = "ingest"
	cfg.Fragments.InternalRNG.Enabled = false
	cfg.Fragments.InternalRNG.Interval = 10 * time.Millisecond
	cfg.Simulation.TickInterval = 10 * time.Millisecond
	st := store.NewMemoryStore(cfg.History.MaxEvents)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	source, err := fragments.NewSource(cfg)
	if err != nil {
		t.Fatalf("NewSource failed: %v", err)
	}
	engine := NewEngine(cfg, st, plc.NewSimulatedAdapter(), safety.NewSimulatedController(cfg.Safety), source, history.NewInMemorySink(100), logger)
	engine.Start()
	time.Sleep(50 * time.Millisecond)
	engine.Stop()
	if len(st.Fragments()) != 0 {
		t.Fatalf("ingest mode generated %d internal fragments", len(st.Fragments()))
	}
}
