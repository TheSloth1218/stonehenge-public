package dispatcher

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"stonehenge-middleware/internal/autocontrol"
	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/guns"
	"stonehenge-middleware/internal/history"
	"stonehenge-middleware/internal/plc"
	"stonehenge-middleware/internal/safety"
	"stonehenge-middleware/internal/targetfeed"
)

func TestAngularDistanceWrapsCircularAzimuth(t *testing.T) {
	if got := angularDistance(359, 1); got != 2 {
		t.Fatalf("angularDistance wrap = %v, want 2", got)
	}
	if got := angularDistance(10, 350); got != 20 {
		t.Fatalf("angularDistance reverse wrap = %v, want 20", got)
	}
}

func TestActiveDispatcherAssignsOnlyGun1AndRejectsGun2(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.Mode = config.DispatcherModeActive
	cfg.Guns = testGunConfigs()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 10, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	store := history.NewInMemorySink(100)
	registry := testRegistry(t, cfg.Guns)
	safetySvc := testSafety(t)
	mgr := New(cfg.Dispatcher, feed, registry, safetySvc, store, slog.Default())
	mgr.reconcile(context.Background())

	target := targetfeed.Target{Source: targetfeed.SourceExternalGenerator, TargetID: 44, Azimuth: 95, Elevation: 30, Priority: 50, TargetClass: "fragment", DetectedAt: time.Now().UTC(), Metadata: map[string]interface{}{"source": targetfeed.SourceExternalGenerator}}
	if _, err := feed.Ingest(context.Background(), target); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	mgr.poll(context.Background())

	active := true
	assignments, err := store.ListTargetAssignments(context.Background(), history.TargetAssignmentFilter{Active: &active})
	if err != nil {
		t.Fatalf("assignments: %v", err)
	}
	if len(assignments) != 1 || assignments[0].GunID != 1 || assignments[0].TargetID != target.TargetID {
		t.Fatalf("assignment = %+v, want target assigned to gun 1", assignments)
	}
	decisions, err := store.ListDispatcherDecisions(context.Background(), history.DispatcherDecisionFilter{Limit: 1})
	if err != nil {
		t.Fatalf("decisions: %v", err)
	}
	if len(decisions) != 1 || decisions[0].SelectedGunID == nil || *decisions[0].SelectedGunID != 1 {
		t.Fatalf("decision = %+v, want selected gun 1", decisions)
	}
	var gun2 history.DispatcherCandidate
	for _, c := range decisions[0].Candidates {
		if c.GunID == 2 {
			gun2 = c
		}
	}
	if gun2.Eligible || !hasReason(gun2.Reasons, string(ReasonDispatcherIneligible)) || !hasReason(gun2.Reasons, string(ReasonAutoExecutorDisabled)) {
		t.Fatalf("gun2 candidate = %+v, want dispatcher/aut executor rejection", gun2)
	}
}

func TestTwoGunSelectionUsesScoreAndTieBreak(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.Mode = config.DispatcherModeActive
	cfg.Guns = testTwoGunConfigs()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 10, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	store := history.NewInMemorySink(100)
	mgr := New(cfg.Dispatcher, feed, testRegistry(t, cfg.Guns), testSafety(t), store, slog.Default())
	mgr.reconcile(context.Background())
	now := time.Now().UTC()

	gun1Closest := mgr.Decide(context.Background(), targetfeed.Target{Source: targetfeed.SourceExternalGenerator, TargetID: 201, Azimuth: 10, Elevation: 20, DetectedAt: now}, now)
	if gun1Closest.SelectedGunID == nil || *gun1Closest.SelectedGunID != 1 {
		t.Fatalf("gun1 closest decision = %+v, want gun 1", gun1Closest)
	}
	gun2Closest := mgr.Decide(context.Background(), targetfeed.Target{Source: targetfeed.SourceExternalGenerator, TargetID: 202, Azimuth: 20, Elevation: 20, DetectedAt: now}, now)
	if gun2Closest.SelectedGunID == nil || *gun2Closest.SelectedGunID != 2 {
		t.Fatalf("gun2 closest decision = %+v, want gun 2", gun2Closest)
	}
	tie := mgr.Decide(context.Background(), targetfeed.Target{Source: targetfeed.SourceExternalGenerator, TargetID: 203, Azimuth: 15, Elevation: 20, DetectedAt: now}, now)
	if tie.SelectedGunID == nil || *tie.SelectedGunID != 1 {
		t.Fatalf("tie decision = %+v, want lower gun id", tie)
	}
}

func TestEightGunSelectionUsesScoreAndTieBreak(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.Mode = config.DispatcherModeActive
	cfg.Guns = testEightGunConfigs()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 20, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	store := history.NewInMemorySink(100)
	mgr := New(cfg.Dispatcher, feed, testRegistry(t, cfg.Guns), testSafety(t), store, slog.Default())
	mgr.reconcile(context.Background())
	now := time.Now().UTC()

	for gunID := 1; gunID <= 8; gunID++ {
		decision := mgr.Decide(context.Background(), targetfeed.Target{
			Source:     targetfeed.SourceExternalGenerator,
			TargetID:   uint32(400 + gunID),
			Azimuth:    float64(gunID * 10),
			Elevation:  20,
			DetectedAt: now,
		}, now)
		if decision.SelectedGunID == nil || *decision.SelectedGunID != gunID {
			t.Fatalf("gun %d closest decision = %+v, want gun %d", gunID, decision, gunID)
		}
		if decision.EligibleGunCount != 8 {
			t.Fatalf("gun %d eligible count = %d, want 8", gunID, decision.EligibleGunCount)
		}
	}

	tie := mgr.Decide(context.Background(), targetfeed.Target{
		Source: targetfeed.SourceExternalGenerator, TargetID: 499, Azimuth: 15, Elevation: 20, DetectedAt: now,
	}, now)
	if tie.SelectedGunID == nil || *tie.SelectedGunID != 1 {
		t.Fatalf("tie decision = %+v, want lower gun id", tie)
	}
}

func TestTwoGunSelectionFallsBackForBusyDisconnectedAndSafety(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.Mode = config.DispatcherModeActive
	cfg.Guns = testTwoGunConfigs()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 10, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	store := history.NewInMemorySink(100)
	safetySvc := testSafety(t)
	mgr := New(cfg.Dispatcher, feed, testRegistry(t, cfg.Guns), safetySvc, store, slog.Default())
	mgr.reconcile(context.Background())
	now := time.Now().UTC()

	_, err := store.CreateTargetAssignment(context.Background(), history.TargetAssignmentRecord{
		TargetID: 204, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateWaitingForSafety,
		AssignedAt: now, LeaseExpiresAt: now.Add(time.Minute), LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create busy gun1 assignment: %v", err)
	}
	busyGun1 := mgr.Decide(context.Background(), targetfeed.Target{Source: targetfeed.SourceExternalGenerator, TargetID: 205, Azimuth: 15, Elevation: 20, DetectedAt: now}, now)
	if busyGun1.SelectedGunID == nil || *busyGun1.SelectedGunID != 2 {
		t.Fatalf("busy gun1 decision = %+v, want gun 2", busyGun1)
	}

	cfg.Guns = testTwoGunConfigs()
	cfg.Guns[1].PLCEnabled = false
	disconnectedStore := history.NewInMemorySink(100)
	disconnectedGun2 := New(cfg.Dispatcher, feed, testRegistry(t, cfg.Guns), safetySvc, disconnectedStore, slog.Default())
	disconnectedGun2.reconcile(context.Background())
	decision := disconnectedGun2.Decide(context.Background(), targetfeed.Target{Source: targetfeed.SourceExternalGenerator, TargetID: 206, Azimuth: 20, Elevation: 20, DetectedAt: now}, now)
	if decision.SelectedGunID == nil || *decision.SelectedGunID != 1 {
		t.Fatalf("disconnected gun2 decision = %+v, want gun 1 fallback", decision)
	}
}

func TestEightActiveAssignmentsLeaveNinthQueuedUntilGunFrees(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.Mode = config.DispatcherModeActive
	cfg.Guns = testEightGunConfigs()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 20, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	store := history.NewInMemorySink(100)
	mgr := New(cfg.Dispatcher, feed, testRegistry(t, cfg.Guns), testSafety(t), store, slog.Default())
	mgr.reconcile(context.Background())
	now := time.Now().UTC()

	for gunID := 1; gunID <= 8; gunID++ {
		if _, err := store.CreateTargetAssignment(context.Background(), history.TargetAssignmentRecord{
			TargetID:       uint32(500 + gunID),
			TargetSource:   targetfeed.SourceExternalGenerator,
			GunID:          gunID,
			State:          StateAcceptedByGun,
			Generation:     1,
			AssignedAt:     now,
			LeaseExpiresAt: now.Add(time.Minute),
			LeaseOwner:     LeaseOwnerExecutor,
			UpdatedAt:      now,
		}); err != nil {
			t.Fatalf("create gun %d active assignment: %v", gunID, err)
		}
	}
	ninth := targetfeed.Target{Source: targetfeed.SourceExternalGenerator, TargetID: 599, Azimuth: 10, Elevation: 20, Priority: 50, TargetClass: "fragment", DetectedAt: now}
	if _, err := feed.Ingest(context.Background(), ninth); err != nil {
		t.Fatalf("ingest ninth: %v", err)
	}
	mgr.poll(context.Background())

	active := true
	rows, err := store.ListTargetAssignments(context.Background(), history.TargetAssignmentFilter{Active: &active, Limit: 20})
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(rows) != 8 {
		t.Fatalf("active assignments = %d, want 8: %+v", len(rows), rows)
	}
	if _, found, err := store.ActiveAssignmentForTarget(context.Background(), ninth.TargetID, ninth.Source); err != nil || found {
		t.Fatalf("ninth active assignment found=%t err=%v, want queued only", found, err)
	}
	if pending := feed.PendingTargets(now); len(pending) != 1 || pending[0].Target.TargetID != ninth.TargetID {
		t.Fatalf("pending = %+v, want ninth target queued", pending)
	}

	if err := store.UpdateTargetAssignment(context.Background(), rows[0].ID, rows[0].Generation, history.TargetAssignmentUpdate{
		State:           StateCompleted,
		ClearLease:      true,
		CompletionState: "completed_fired",
		UpdatedAt:       now.Add(time.Second),
	}); err != nil {
		t.Fatalf("free one gun: %v", err)
	}
	mgr.poll(context.Background())
	assigned, found, err := store.ActiveAssignmentForTarget(context.Background(), ninth.TargetID, ninth.Source)
	if err != nil || !found {
		t.Fatalf("ninth active assignment found=%t err=%v", found, err)
	}
	if assigned.GunID != rows[0].GunID {
		t.Fatalf("ninth assigned to gun %d, want first freed gun %d", assigned.GunID, rows[0].GunID)
	}
}

func TestTwoGunSafetyInhibitHoldsBestGunButFallsBackToCloserSafeGun(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.Mode = config.DispatcherModeActive
	cfg.Guns = testTwoGunConfigs()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 10, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	store := history.NewInMemorySink(100)
	transport := &safetyTransport{words: safetyWords(true)}
	transport.words[16] = 0 // Gun 1 fire permissive false.
	transport.words[17] = 1 // Gun 1 inhibit active.
	transport.words[18] = uint16(safety.GunInhibitExternal)
	safetyCfg := config.Defaults().SafetyPLC
	safetyCfg.StaleAfter = time.Second
	safetySvc := safety.NewServiceWithTransport(safetyCfg, transport, nil, slog.Default())
	safetySvc.Start()
	t.Cleanup(safetySvc.Close)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snap := safetySvc.Snapshot()
		if snap.Connected && snap.Fresh && snap.SchemaSupported && snap.Ready {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	registry := testRegistry(t, cfg.Guns)
	mgr := New(cfg.Dispatcher, feed, registry, safetySvc, store, slog.Default())
	mgr.reconcile(context.Background())
	now := time.Now().UTC()

	held := mgr.Decide(context.Background(), targetfeed.Target{Source: targetfeed.SourceExternalGenerator, TargetID: 208, Azimuth: 10, Elevation: 20, DetectedAt: now}, now)
	if held.SelectedGunID == nil || *held.SelectedGunID != 1 {
		t.Fatalf("safety-held best gun decision = %+v, want gun 1 ownership hold", held)
	}
	if held.EligibleGunCount != 1 {
		t.Fatalf("eligible gun count = %d, want only Gun 2 command-eligible", held.EligibleGunCount)
	}

	fallback := mgr.Decide(context.Background(), targetfeed.Target{Source: targetfeed.SourceExternalGenerator, TargetID: 209, Azimuth: 20, Elevation: 20, DetectedAt: now}, now)
	if fallback.SelectedGunID == nil || *fallback.SelectedGunID != 2 {
		t.Fatalf("closer safe gun decision = %+v, want gun 2 fallback", fallback)
	}
}

func TestUncertainGunDoesNotBlockOtherEligibleGunIngest(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.Mode = config.DispatcherModeActive
	cfg.Guns = testTwoGunConfigs()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 10, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	store := history.NewInMemorySink(100)
	mgr := New(cfg.Dispatcher, feed, testRegistry(t, cfg.Guns), testSafety(t), store, slog.Default())
	mgr.reconcile(context.Background())
	now := time.Now().UTC()
	_, err := store.CreateTargetAssignment(context.Background(), history.TargetAssignmentRecord{
		TargetID: 207, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateUncertain,
		CompletionState: ReconciliationCommittedUnknown, AssignedAt: now, LeaseExpiresAt: now.Add(time.Minute), LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create uncertain gun1 assignment: %v", err)
	}

	mgr.refreshIngestAllowed(context.Background())
	status := mgr.Status(context.Background())
	if !status.TargetIngestAllowed || status.UncertainCount != 1 {
		t.Fatalf("status = %+v, want ingest allowed through gun2 while gun1 uncertain", status)
	}
}

func TestShadowDecisionDoesNotAssignOrConsumeQueue(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.Mode = config.DispatcherModeShadow
	cfg.Guns = testGunConfigs()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 10, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	store := history.NewInMemorySink(100)
	registry := testRegistry(t, cfg.Guns)
	safetySvc := testSafety(t)
	mgr := New(cfg.Dispatcher, feed, registry, safetySvc, store, slog.Default())
	mgr.reconcile(context.Background())

	target := targetfeed.Target{Source: targetfeed.SourceExternalGenerator, TargetID: 45, Azimuth: 95, Elevation: 30, Priority: 50, TargetClass: "fragment", DetectedAt: time.Now().UTC(), Metadata: map[string]interface{}{"source": targetfeed.SourceExternalGenerator}}
	if _, err := feed.Ingest(context.Background(), target); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	mgr.poll(context.Background())

	active := true
	assignments, err := store.ListTargetAssignments(context.Background(), history.TargetAssignmentFilter{Active: &active})
	if err != nil {
		t.Fatalf("assignments: %v", err)
	}
	if len(assignments) != 0 {
		t.Fatalf("shadow assignments = %+v, want none", assignments)
	}
	if pending := feed.PendingTargets(time.Now().UTC()); len(pending) != 1 || pending[0].Target.TargetID != target.TargetID {
		t.Fatalf("pending = %+v, want target retained", pending)
	}
	decisions, err := store.ListDispatcherDecisions(context.Background(), history.DispatcherDecisionFilter{Limit: 1})
	if err != nil {
		t.Fatalf("decisions: %v", err)
	}
	if len(decisions) != 1 || !decisions[0].ShadowOnly {
		t.Fatalf("decision = %+v, want shadow-only persisted decision", decisions)
	}
	mgr.poll(context.Background())
	decisions, err = store.ListDispatcherDecisions(context.Background(), history.DispatcherDecisionFilter{Limit: 10})
	if err != nil {
		t.Fatalf("decisions after duplicate poll: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("shadow duplicate decisions = %d, want 1", len(decisions))
	}
}

func TestWaitingForSafetyLeaseRenewsAndTerminalStops(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.AssignmentLeaseDuration = 200 * time.Millisecond
	cfg.Dispatcher.LeaseRenewalInterval = 50 * time.Millisecond
	store := history.NewInMemorySink(100)
	mgr := New(cfg.Dispatcher, nil, testRegistry(t, testGunConfigs()), testSafety(t), store, slog.Default())
	now := time.Now().UTC().Add(-time.Second)
	owner := LeaseOwnerExecutor
	assignment, err := store.CreateTargetAssignment(context.Background(), history.TargetAssignmentRecord{
		TargetID: 1, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateWaitingForSafety,
		AssignedAt: now, LeaseExpiresAt: now.Add(30 * time.Millisecond), LeaseOwner: owner, LastLeaseRenewedAt: &now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	mgr.renewActiveLeases(context.Background(), time.Now().UTC())
	got, _, err := store.GetTargetAssignment(context.Background(), assignment.ID)
	if err != nil {
		t.Fatalf("get assignment: %v", err)
	}
	if !got.LeaseExpiresAt.After(assignment.LeaseExpiresAt) || got.Generation != assignment.Generation || got.LastLeaseRenewedAt == nil {
		t.Fatalf("renewed assignment = %+v, want same generation with extended lease", got)
	}

	if err := store.UpdateTargetAssignment(context.Background(), assignment.ID, assignment.Generation, history.TargetAssignmentUpdate{State: StateCompleted, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("complete assignment: %v", err)
	}
	if _, err := store.RenewTargetAssignmentLease(context.Background(), assignment.ID, assignment.Generation, owner, time.Now().Add(time.Second)); err == nil {
		t.Fatal("terminal assignment renewal succeeded, want refusal")
	}
}

func TestLeaseRenewalFailureInjectionProtectsAssignment(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.AssignmentLeaseDuration = 200 * time.Millisecond
	cfg.Dispatcher.LeaseRenewalInterval = 50 * time.Millisecond
	base := history.NewInMemorySink(100)
	store := &flakyRenewStore{InMemorySink: base, failures: 2, err: errors.New("database unavailable")}
	mgr := New(cfg.Dispatcher, nil, testRegistry(t, testGunConfigs()), testSafety(t), store, slog.Default())
	now := time.Now().UTC().Add(-time.Second)
	assignment, err := store.CreateTargetAssignment(context.Background(), history.TargetAssignmentRecord{
		TargetID: 114, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateCommandCommitted,
		AssignedAt: now, LeaseExpiresAt: now.Add(30 * time.Millisecond), LeaseOwner: LeaseOwnerExecutor, LastLeaseRenewedAt: &now,
		CommandSequence: uint32Ptr(77), CommandAuditID: int64Ptr(177), EngagementID: int64Ptr(277), UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	mgr.renewActiveLeases(context.Background(), time.Now().UTC())
	mgr.renewActiveLeases(context.Background(), time.Now().UTC())
	status := mgr.Status(context.Background())
	if status.LastError == "" {
		t.Fatalf("status = %+v, want renewal error visible", status)
	}
	got, _, err := store.GetTargetAssignment(context.Background(), assignment.ID)
	if err != nil {
		t.Fatalf("get assignment: %v", err)
	}
	if got.State != StateCommandCommitted || got.CommandSequence == nil || got.CommandAuditID == nil || got.EngagementID == nil {
		t.Fatalf("assignment after renewal failures = %+v, want committed evidence protected", got)
	}

	mgr.renewActiveLeases(context.Background(), time.Now().UTC())
	recovered, _, err := store.GetTargetAssignment(context.Background(), assignment.ID)
	if err != nil {
		t.Fatalf("get recovered assignment: %v", err)
	}
	if !recovered.LeaseExpiresAt.After(got.LeaseExpiresAt) || recovered.LastLeaseRenewalError != "" {
		t.Fatalf("assignment after recovery = %+v, want lease advanced and error cleared", recovered)
	}
	if _, err := store.RenewTargetAssignmentLease(context.Background(), assignment.ID, assignment.Generation, "stale-worker", time.Now().Add(time.Second)); err == nil {
		t.Fatal("wrong owner renewal succeeded, want refusal")
	}
	mismatch, _, err := store.GetTargetAssignment(context.Background(), assignment.ID)
	if err != nil {
		t.Fatalf("get mismatch assignment: %v", err)
	}
	if mismatch.LeaseRenewalFailures == 0 || mismatch.LastLeaseRenewalError == "" {
		t.Fatalf("assignment after wrong-owner renewal = %+v, want failure counter and error", mismatch)
	}
	if _, err := store.RenewTargetAssignmentLease(context.Background(), assignment.ID, assignment.Generation+1, LeaseOwnerExecutor, time.Now().Add(time.Second)); err == nil {
		t.Fatal("stale generation renewal succeeded, want refusal")
	}
}

func TestExpiredLeaseRequeuesOnlyDefinitelyUncommittedAssignment(t *testing.T) {
	cfg := config.Defaults()
	store := history.NewInMemorySink(100)
	mgr := New(cfg.Dispatcher, nil, testRegistry(t, testGunConfigs()), testSafety(t), store, slog.Default())
	expired := time.Now().UTC().Add(-time.Second)
	assignment, err := store.CreateTargetAssignment(context.Background(), history.TargetAssignmentRecord{
		TargetID: 2, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateAssigned,
		AssignedAt: expired, LeaseExpiresAt: expired, LeaseOwner: LeaseOwnerDispatcher, UpdatedAt: expired,
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	mgr.handleExpiredLeases(context.Background(), time.Now().UTC())
	got, _, err := store.GetTargetAssignment(context.Background(), assignment.ID)
	if err != nil {
		t.Fatalf("get assignment: %v", err)
	}
	if got.State != StateRequeued || got.ReconciliationResult != "safe_requeue_expired_uncommitted" {
		t.Fatalf("expired uncommitted assignment = %+v, want safe requeue", got)
	}
}

func TestExpiredCommittedEvidenceBecomesUncertainAndStaysActive(t *testing.T) {
	cfg := config.Defaults()
	store := history.NewInMemorySink(100)
	mgr := New(cfg.Dispatcher, nil, testRegistry(t, testGunConfigs()), testSafety(t), store, slog.Default())
	expired := time.Now().UTC().Add(-time.Second)
	seq := uint32(12)
	assignment, err := store.CreateTargetAssignment(context.Background(), history.TargetAssignmentRecord{
		TargetID: 3, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateCommandCommitted,
		CommandSequence: &seq, AssignedAt: expired, LeaseExpiresAt: expired, LeaseOwner: LeaseOwnerExecutor, UpdatedAt: expired,
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	mgr.handleExpiredLeases(context.Background(), time.Now().UTC())
	got, _, err := store.GetTargetAssignment(context.Background(), assignment.ID)
	if err != nil {
		t.Fatalf("get assignment: %v", err)
	}
	if got.State != StateUncertain || got.UncertaintyReason == "" {
		t.Fatalf("expired committed assignment = %+v, want uncertain", got)
	}
	if _, found, err := store.ActiveAssignmentForTarget(context.Background(), got.TargetID, got.TargetSource); err != nil || !found {
		t.Fatalf("active target lookup found=%t err=%v, want protected active assignment", found, err)
	}
}

func TestExpiredEngagementWithoutCommandEvidenceCanRequeue(t *testing.T) {
	cfg := config.Defaults()
	store := history.NewInMemorySink(100)
	mgr := New(cfg.Dispatcher, nil, testRegistry(t, testGunConfigs()), testSafety(t), store, slog.Default())
	expired := time.Now().UTC().Add(-time.Second)
	engagementID := int64(44)
	assignment, err := store.CreateTargetAssignment(context.Background(), history.TargetAssignmentRecord{
		TargetID: 4, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateAssigned,
		EngagementID: &engagementID, AssignedAt: expired, LeaseExpiresAt: expired, LeaseOwner: LeaseOwnerExecutor, UpdatedAt: expired,
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	mgr.handleExpiredLeases(context.Background(), time.Now().UTC())
	got, _, err := store.GetTargetAssignment(context.Background(), assignment.ID)
	if err != nil {
		t.Fatalf("get assignment: %v", err)
	}
	if got.State != StateRequeued || got.ReconciliationResult != ReconciliationSafeRequeueExpired {
		t.Fatalf("expired engagement-only assignment = %+v, want safe requeue", got)
	}
}

func TestReconciliationFinalizesCompletedCommandAuditIdempotently(t *testing.T) {
	store := newEvidenceStore()
	mgr := New(config.Defaults().Dispatcher, nil, testRegistry(t, testGunConfigs()), testSafety(t), store, slog.Default())
	now := time.Now().UTC().Add(-time.Minute)
	seq := uint32(21)
	auditID := int64(501)
	store.commands[auditID] = history.GunCommandRecord{ID: auditID, GunID: 1, SubmittedSequence: seq, Result: "completed_fired", CompletionTime: timePtr(now)}
	assignment := createTestAssignment(t, store, history.TargetAssignmentRecord{
		TargetID: 101, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateCommandCommitted,
		CommandSequence: &seq, CommandAuditID: &auditID, AssignedAt: now, LeaseExpiresAt: now, LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now,
	})

	runRepeatedReconciliation(t, mgr, 3)
	got := mustAssignment(t, store, assignment.ID)
	if got.State != StateCompleted || got.CompletionState != "completed_fired" || got.Generation != assignment.Generation || got.ReconciliationResult != ReconciliationFinalizedCommandAudit || got.LeaseOwner != "" {
		t.Fatalf("assignment = %+v, want command-audit finalized completion with same generation", got)
	}
	assertAssignmentCount(t, store, 1)
}

func TestReconciliationFinalizesCompletedEngagementIdempotently(t *testing.T) {
	store := newEvidenceStore()
	mgr := New(config.Defaults().Dispatcher, nil, testRegistry(t, testGunConfigs()), testSafety(t), store, slog.Default())
	now := time.Now().UTC().Add(-time.Minute)
	engagementID := int64(601)
	store.engagements[engagementID] = history.GunEngagementRecord{ID: engagementID, TargetID: 102, GunID: 1, State: "idle", Result: "completed_fired", CompletedAt: timePtr(now)}
	assignment := createTestAssignment(t, store, history.TargetAssignmentRecord{
		TargetID: 102, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: "engaging",
		EngagementID: &engagementID, AssignedAt: now, LeaseExpiresAt: now, LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now,
	})

	runRepeatedReconciliation(t, mgr, 3)
	got := mustAssignment(t, store, assignment.ID)
	if got.State != StateCompleted || got.CompletionState != "completed_fired" || got.Generation != assignment.Generation || got.ReconciliationResult != ReconciliationFinalizedEngagement {
		t.Fatalf("assignment = %+v, want engagement finalized completion with same generation", got)
	}
	assertAssignmentCount(t, store, 1)
}

func TestReconciliationFinalizesFeedForCompletedAssignmentIdempotently(t *testing.T) {
	store := newEvidenceStore()
	cfg := config.Defaults()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 10, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	mgr := New(cfg.Dispatcher, feed, testRegistry(t, testGunConfigs()), testSafety(t), store, slog.Default())
	now := time.Now().UTC()
	target := targetfeed.Target{Source: targetfeed.SourceExternalGenerator, TargetID: 103, Azimuth: 95, Elevation: 30, Priority: 50, TargetClass: "fragment", DetectedAt: now, Metadata: map[string]interface{}{"source": targetfeed.SourceExternalGenerator}}
	if _, err := feed.Ingest(context.Background(), target); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if !feed.MarkSubmitted(target, now) {
		t.Fatal("mark submitted failed")
	}
	assignment := createTestAssignment(t, store, history.TargetAssignmentRecord{
		TargetID: 103, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateCompleted,
		CompletionState: "completed_fired", AssignedAt: now, LeaseExpiresAt: now, LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now, TargetPayload: targetPayload(target),
	})

	runRepeatedReconciliation(t, mgr, 3)
	got := mustAssignment(t, store, assignment.ID)
	if got.State != StateCompleted || got.Generation != assignment.Generation {
		t.Fatalf("assignment = %+v, want completed same generation", got)
	}
	records := feed.Targets()
	if len(records) != 1 || records[0].State != targetfeed.StateCompleted || records[0].Result != "completed_fired" {
		t.Fatalf("feed records = %+v, want completed once", records)
	}
	assertAssignmentCount(t, store, 1)
}

func TestReconciliationAlreadyCompleteIsStable(t *testing.T) {
	store := newEvidenceStore()
	mgr := New(config.Defaults().Dispatcher, nil, testRegistry(t, testGunConfigs()), testSafety(t), store, slog.Default())
	now := time.Now().UTC()
	assignment := createTestAssignment(t, store, history.TargetAssignmentRecord{
		TargetID: 104, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateCompleted,
		CompletionState: "completed_fired", ReconciliationResult: ReconciliationFinalizedCommandAudit,
		AssignedAt: now, LeaseExpiresAt: now, LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now,
	})

	runRepeatedReconciliation(t, mgr, 3)
	got := mustAssignment(t, store, assignment.ID)
	if got.State != StateCompleted || got.ReconciliationResult != ReconciliationFinalizedCommandAudit || got.Generation != assignment.Generation {
		t.Fatalf("assignment = %+v, want already-complete stable", got)
	}
	assertAssignmentCount(t, store, 1)
}

func TestReconciliationConflictingTerminalEvidenceBecomesUncertain(t *testing.T) {
	store := newEvidenceStore()
	mgr := New(config.Defaults().Dispatcher, nil, testRegistry(t, testGunConfigs()), testSafety(t), store, slog.Default())
	now := time.Now().UTC().Add(-time.Minute)
	auditID := int64(701)
	engagementID := int64(702)
	store.commands[auditID] = history.GunCommandRecord{ID: auditID, GunID: 1, Result: "completed_fired", CompletionTime: timePtr(now)}
	store.engagements[engagementID] = history.GunEngagementRecord{ID: engagementID, TargetID: 105, GunID: 1, Result: "completed_no_fire", CompletedAt: timePtr(now)}
	assignment := createTestAssignment(t, store, history.TargetAssignmentRecord{
		TargetID: 105, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateCommandCommitted,
		CommandAuditID: &auditID, EngagementID: &engagementID, AssignedAt: now, LeaseExpiresAt: now, LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now,
	})

	runRepeatedReconciliation(t, mgr, 3)
	got := mustAssignment(t, store, assignment.ID)
	if got.State != StateUncertain || got.ReconciliationResult != ReconciliationConflictTerminal || got.UncertaintyReason == "" || got.Generation != assignment.Generation {
		t.Fatalf("assignment = %+v, want uncertain conflict with same generation", got)
	}
	assertAssignmentCount(t, store, 1)
}

func TestReconciliationCommittedTimeoutBecomesUncertainNotTerminalFailure(t *testing.T) {
	store := newEvidenceStore()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 10, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	mgr := New(config.Defaults().Dispatcher, feed, testRegistry(t, testGunConfigs()), testSafety(t), store, slog.Default())
	now := time.Now().UTC().Add(-time.Minute)
	seq := uint32(31)
	auditID := int64(703)
	engagementID := int64(704)
	store.commands[auditID] = history.GunCommandRecord{
		ID: auditID, GunID: 1, SubmittedSequence: seq, Result: "completion_timeout", CompletionTime: timePtr(now),
		LifecycleEvidence: history.CommandLifecycleEvidence{ConfirmedAcceptedSequence: &seq, UncertaintyReason: "lost lifecycle observation after accepted sequence"},
	}
	store.engagements[engagementID] = history.GunEngagementRecord{ID: engagementID, TargetID: 108, GunID: 1, Result: "completion_timeout", CompletedAt: timePtr(now)}
	assignment := createTestAssignment(t, store, history.TargetAssignmentRecord{
		TargetID: 108, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateCommandCommitted,
		CommandSequence: &seq, CommandAuditID: &auditID, EngagementID: &engagementID, AssignedAt: now, LeaseExpiresAt: now, LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now,
	})

	runRepeatedReconciliation(t, mgr, 3)
	got := mustAssignment(t, store, assignment.ID)
	if got.State != StateUncertain || got.CompletionState != ReconciliationCommittedUnknown || got.ReconciliationResult != ReconciliationCommittedUnknown || got.UncertaintyReason == "" {
		t.Fatalf("assignment = %+v, want committed-result uncertainty", got)
	}
	if _, found, err := store.ActiveAssignmentForTarget(context.Background(), got.TargetID, got.TargetSource); err != nil || !found {
		t.Fatalf("active target found=%t err=%v, want uncertainty to protect target", found, err)
	}
}

func TestReconciliationCommittedTimeoutWithPersistedShotFinalizesFired(t *testing.T) {
	store := newEvidenceStore()
	mgr := New(config.Defaults().Dispatcher, nil, testRegistry(t, testGunConfigs()), testSafety(t), store, slog.Default())
	now := time.Now().UTC().Add(-time.Minute)
	seq := uint32(32)
	auditID := int64(705)
	baselineShot := uint32(4)
	finalShot := uint32(5)
	store.commands[auditID] = history.GunCommandRecord{
		ID: auditID, GunID: 1, SubmittedSequence: seq, Result: "completion_timeout", CompletionTime: timePtr(now), ShotSequence: &finalShot,
		LifecycleEvidence: history.CommandLifecycleEvidence{BaselineShotSequence: &baselineShot, LastObservedShotSequence: &finalShot, ShotObserved: true},
	}
	assignment := createTestAssignment(t, store, history.TargetAssignmentRecord{
		TargetID: 109, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateCommandCommitted,
		CommandSequence: &seq, CommandAuditID: &auditID, AssignedAt: now, LeaseExpiresAt: now, LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now,
	})

	runRepeatedReconciliation(t, mgr, 3)
	got := mustAssignment(t, store, assignment.ID)
	if got.State != StateCompleted || got.CompletionState != "completed_fired" || got.Generation != assignment.Generation || got.LeaseOwner != "" {
		t.Fatalf("assignment = %+v, want completed_fired from persisted shot evidence", got)
	}
	assertAssignmentCount(t, store, 1)
}

func TestUncertainGun1AssignmentBlocksDispatchAndReadiness(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.Mode = config.DispatcherModeActive
	cfg.Guns = testGunConfigs()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 10, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	store := history.NewInMemorySink(100)
	registry := testRegistry(t, cfg.Guns)
	safetySvc := testSafety(t)
	mgr := New(cfg.Dispatcher, feed, registry, safetySvc, store, slog.Default())
	mgr.reconcile(context.Background())
	now := time.Now().UTC()
	_, err := store.CreateTargetAssignment(context.Background(), history.TargetAssignmentRecord{
		TargetID: 110, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateUncertain,
		CompletionState: ReconciliationCommittedUnknown, UncertaintyReason: "operator required", AssignedAt: now, LeaseExpiresAt: now.Add(time.Minute), LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create uncertain assignment: %v", err)
	}
	target := targetfeed.Target{Source: targetfeed.SourceExternalGenerator, TargetID: 111, Azimuth: 95, Elevation: 30, Priority: 50, TargetClass: "fragment", DetectedAt: now, Metadata: map[string]interface{}{"source": targetfeed.SourceExternalGenerator}}
	if _, err := feed.Ingest(context.Background(), target); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	mgr.poll(context.Background())
	status := mgr.Status(context.Background())
	if status.TargetIngestAllowed || status.NoEligibleGunReason != string(ReasonUncertainAssignment) || status.UncertainCount != 1 {
		t.Fatalf("status = %+v, want uncertain Gun 1 to block ingest/dispatch", status)
	}
	active := true
	rows, err := store.ListTargetAssignments(context.Background(), history.TargetAssignmentFilter{Active: &active})
	if err != nil {
		t.Fatalf("assignments: %v", err)
	}
	if len(rows) != 1 || rows[0].TargetID != 110 {
		t.Fatalf("active assignments = %+v, want only original uncertain assignment", rows)
	}
}

func TestBusyGun1WithQueueCapacityAllowsIngestReadiness(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.Mode = config.DispatcherModeActive
	cfg.Guns = testGunConfigs()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 10, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	store := history.NewInMemorySink(100)
	registry := testRegistry(t, cfg.Guns)
	safetySvc := testSafety(t)
	mgr := New(cfg.Dispatcher, feed, registry, safetySvc, store, slog.Default())
	mgr.reconcile(context.Background())
	now := time.Now().UTC()
	_, err := store.CreateTargetAssignment(context.Background(), history.TargetAssignmentRecord{
		TargetID: 112, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateWaitingForSafety,
		AssignedAt: now, LeaseExpiresAt: now.Add(time.Minute), LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create busy assignment: %v", err)
	}

	mgr.refreshIngestAllowed(context.Background())
	status := mgr.Status(context.Background())
	if !status.TargetIngestAllowed || status.NoEligibleGunReason != "gun_busy_queue_capacity_available" {
		t.Fatalf("status = %+v, want queueable busy Gun 1 to allow ingest", status)
	}
}

func TestBusyGun1WithFullQueueBlocksIngestReadiness(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.Mode = config.DispatcherModeActive
	cfg.Guns = testGunConfigs()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 1, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	store := history.NewInMemorySink(100)
	registry := testRegistry(t, cfg.Guns)
	safetySvc := testSafety(t)
	mgr := New(cfg.Dispatcher, feed, registry, safetySvc, store, slog.Default())
	mgr.reconcile(context.Background())
	now := time.Now().UTC()
	_, err := store.CreateTargetAssignment(context.Background(), history.TargetAssignmentRecord{
		TargetID: 112, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateWaitingForSafety,
		AssignedAt: now, LeaseExpiresAt: now.Add(time.Minute), LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create busy assignment: %v", err)
	}
	if _, err := feed.Ingest(context.Background(), targetfeed.Target{
		Source: targetfeed.SourceExternalGenerator, TargetID: 113, Azimuth: 10, Elevation: 30, Priority: 50, TargetClass: "fragment", DetectedAt: now,
	}); err != nil {
		t.Fatalf("fill queue: %v", err)
	}

	mgr.refreshIngestAllowed(context.Background())
	status := mgr.Status(context.Background())
	if status.TargetIngestAllowed || status.NoEligibleGunReason != string(ReasonTargetQueueFull) {
		t.Fatalf("status = %+v, want full queue to block ingest with target_queue_full", status)
	}
}

func TestTwoGunSafetyWaitingAndPeerBusyAllowsQueueCapacityIngest(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.Mode = config.DispatcherModeActive
	cfg.Guns = testTwoGunConfigs()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 10, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	store := history.NewInMemorySink(100)
	transport := &safetyTransport{words: safetyWords(true)}
	transport.words[16] = 0
	transport.words[17] = 1
	transport.words[18] = uint16(safety.GunInhibitExternal)
	safetyCfg := config.Defaults().SafetyPLC
	safetyCfg.StaleAfter = time.Second
	safetySvc := safety.NewServiceWithTransport(safetyCfg, transport, nil, slog.Default())
	safetySvc.Start()
	t.Cleanup(safetySvc.Close)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snap := safetySvc.Snapshot()
		if snap.Connected && snap.Fresh && snap.SchemaSupported && snap.Ready {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	registry := testRegistry(t, cfg.Guns)
	mgr := New(cfg.Dispatcher, feed, registry, safetySvc, store, slog.Default())
	mgr.reconcile(context.Background())
	now := time.Now().UTC()
	for _, row := range []history.TargetAssignmentRecord{
		{TargetID: 301, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateWaitingForSafety, AssignedAt: now, LeaseExpiresAt: now.Add(time.Minute), LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now},
		{TargetID: 302, TargetSource: targetfeed.SourceExternalGenerator, GunID: 2, State: StateAcceptedByGun, AssignedAt: now, LeaseExpiresAt: now.Add(time.Minute), LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now},
	} {
		if _, err := store.CreateTargetAssignment(context.Background(), row); err != nil {
			t.Fatalf("create active assignment: %v", err)
		}
	}

	mgr.refreshIngestAllowed(context.Background())
	status := mgr.Status(context.Background())
	if !status.TargetIngestAllowed || status.NoEligibleGunReason != "gun_busy_queue_capacity_available" {
		t.Fatalf("status = %+v, want persistent queue capacity available while both guns occupied", status)
	}
}

func TestSafetyUnavailableTakesPriorityInIngestReadiness(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.Mode = config.DispatcherModeActive
	cfg.Guns = testGunConfigs()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 10, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	store := history.NewInMemorySink(100)
	registry := testRegistry(t, cfg.Guns)
	mgr := New(cfg.Dispatcher, feed, registry, nil, store, slog.Default())
	mgr.reconcile(context.Background())
	now := time.Now().UTC()
	_, err := store.CreateTargetAssignment(context.Background(), history.TargetAssignmentRecord{
		TargetID: 113, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateWaitingForSafety,
		AssignedAt: now, LeaseExpiresAt: now.Add(time.Minute), LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("create busy assignment: %v", err)
	}

	mgr.refreshIngestAllowed(context.Background())
	status := mgr.Status(context.Background())
	if status.TargetIngestAllowed || status.NoEligibleGunReason != string(ReasonSafetyUnavailable) {
		t.Fatalf("status = %+v, want safety_unavailable to block ingest", status)
	}
}

func TestUnreconciledDispatcherBlocksIngestReadiness(t *testing.T) {
	cfg := config.Defaults()
	cfg.Dispatcher.Mode = config.DispatcherModeActive
	cfg.Guns = testGunConfigs()
	feed := targetfeed.New(targetfeed.Config{Enabled: true, MaxPending: 10, TargetTTL: time.Minute, PollInterval: time.Hour}, nil, slog.Default())
	store := history.NewInMemorySink(100)
	mgr := New(cfg.Dispatcher, feed, testRegistry(t, cfg.Guns), testSafety(t), store, slog.Default())

	if mgr.TargetIngestAllowed(context.Background()) {
		t.Fatal("TargetIngestAllowed returned true before ownership reconciliation")
	}
	status := mgr.Status(context.Background())
	if status.TargetIngestAllowed || status.OwnershipReconciled || status.NoEligibleGunReason != "dispatcher_not_ready" {
		t.Fatalf("status = %+v, want unreconciled dispatcher to block ingest", status)
	}
}

func TestReconciliationFinalizesMissingEngagementFromAuthoritativeCommand(t *testing.T) {
	store := newEvidenceStore()
	mgr := New(config.Defaults().Dispatcher, nil, testRegistry(t, testGunConfigs()), testSafety(t), store, slog.Default())
	now := time.Now().UTC().Add(-time.Minute)
	auditID := int64(801)
	store.commands[auditID] = history.GunCommandRecord{ID: auditID, GunID: 1, Result: "completed_no_fire", CompletionTime: timePtr(now)}
	assignment := createTestAssignment(t, store, history.TargetAssignmentRecord{
		TargetID: 106, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateCommandCommitted,
		CommandAuditID: &auditID, AssignedAt: now, LeaseExpiresAt: now, LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now,
	})

	runRepeatedReconciliation(t, mgr, 2)
	got := mustAssignment(t, store, assignment.ID)
	if got.State != StateCompleted || got.CompletionState != "completed_no_fire" || got.EngagementID != nil || got.Generation != assignment.Generation {
		t.Fatalf("assignment = %+v, want command-only completion without creating engagement", got)
	}
	assertAssignmentCount(t, store, 1)
}

func TestReconciliationPrefersCompletedLifecycleOverStaleLease(t *testing.T) {
	store := newEvidenceStore()
	mgr := New(config.Defaults().Dispatcher, nil, testRegistry(t, testGunConfigs()), testSafety(t), store, slog.Default())
	now := time.Now().UTC().Add(-time.Minute)
	auditID := int64(901)
	store.commands[auditID] = history.GunCommandRecord{ID: auditID, GunID: 1, Result: "completed_fired", CompletionTime: timePtr(now)}
	assignment := createTestAssignment(t, store, history.TargetAssignmentRecord{
		TargetID: 107, TargetSource: targetfeed.SourceExternalGenerator, GunID: 1, State: StateCommandCommitted,
		CommandAuditID: &auditID, AssignedAt: now, LeaseExpiresAt: now.Add(-time.Minute), LeaseOwner: LeaseOwnerExecutor, UpdatedAt: now,
	})

	runRepeatedReconciliation(t, mgr, 2)
	got := mustAssignment(t, store, assignment.ID)
	if got.State != StateCompleted || got.CompletionState != "completed_fired" || got.Generation != assignment.Generation {
		t.Fatalf("assignment = %+v, want completed lifecycle finalized before stale lease handling", got)
	}
}

func testGunConfigs() []config.GunConfig {
	base := config.Defaults().PLC
	base.AutoSafetyFirePermissive = true
	return []config.GunConfig{
		{ID: 1, Name: "Gun 1", PLCEnabled: true, SafetyGunID: 1, AutoEnabled: true, DispatcherEligible: true, AutoExecutorEnabled: true, MinAzimuth: 0, MaxAzimuth: 360, MinElevation: 5, MaxElevation: 85, PLC: base},
		{ID: 2, Name: "Gun 2", PLCEnabled: true, SafetyGunID: 2, AutoEnabled: false, DispatcherEligible: false, AutoExecutorEnabled: false, MinAzimuth: 0, MaxAzimuth: 360, MinElevation: 5, MaxElevation: 85, PLC: base},
	}
}

func testTwoGunConfigs() []config.GunConfig {
	guns := testGunConfigs()
	guns[1].AutoEnabled = true
	guns[1].DispatcherEligible = true
	guns[1].AutoExecutorEnabled = true
	return guns
}

func testEightGunConfigs() []config.GunConfig {
	base := config.Defaults().PLC
	base.AutoSafetyFirePermissive = true
	guns := make([]config.GunConfig, 0, 8)
	for id := 1; id <= 8; id++ {
		guns = append(guns, config.GunConfig{
			ID:                  id,
			Name:                "Gun " + string(rune('0'+id)),
			PLCEnabled:          true,
			SafetyGunID:         id,
			AutoEnabled:         true,
			DispatcherEligible:  true,
			AutoExecutorEnabled: true,
			MinAzimuth:          0,
			MaxAzimuth:          360,
			MinElevation:        5,
			MaxElevation:        85,
			PLC:                 base,
		})
	}
	return guns
}

type dispatcherEvidenceStore struct {
	*history.InMemorySink
	commands    map[int64]history.GunCommandRecord
	engagements map[int64]history.GunEngagementRecord
}

type flakyRenewStore struct {
	*history.InMemorySink
	failures int
	err      error
}

func (s *flakyRenewStore) RenewTargetAssignmentLease(ctx context.Context, id int64, generation int, owner string, expiresAt time.Time) (history.TargetAssignmentRecord, error) {
	if s.failures > 0 {
		s.failures--
		return history.TargetAssignmentRecord{}, s.err
	}
	return s.InMemorySink.RenewTargetAssignmentLease(ctx, id, generation, owner, expiresAt)
}

func newEvidenceStore() *dispatcherEvidenceStore {
	return &dispatcherEvidenceStore{
		InMemorySink: history.NewInMemorySink(100),
		commands:     map[int64]history.GunCommandRecord{},
		engagements:  map[int64]history.GunEngagementRecord{},
	}
}

func (s *dispatcherEvidenceStore) GetGunCommand(ctx context.Context, id int64) (history.GunCommandRecord, bool, error) {
	if err := ctx.Err(); err != nil {
		return history.GunCommandRecord{}, false, err
	}
	row, ok := s.commands[id]
	return row, ok, nil
}

func (s *dispatcherEvidenceStore) GetGunEngagement(ctx context.Context, id int64) (history.GunEngagementRecord, bool, error) {
	if err := ctx.Err(); err != nil {
		return history.GunEngagementRecord{}, false, err
	}
	row, ok := s.engagements[id]
	return row, ok, nil
}

func createTestAssignment(t *testing.T, store history.DispatcherStore, assignment history.TargetAssignmentRecord) history.TargetAssignmentRecord {
	t.Helper()
	if assignment.AssignedAt.IsZero() {
		assignment.AssignedAt = time.Now().UTC()
	}
	if assignment.LeaseExpiresAt.IsZero() {
		assignment.LeaseExpiresAt = assignment.AssignedAt.Add(time.Minute)
	}
	if assignment.UpdatedAt.IsZero() {
		assignment.UpdatedAt = assignment.AssignedAt
	}
	got, err := store.CreateTargetAssignment(context.Background(), assignment)
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	return got
}

func runRepeatedReconciliation(t *testing.T, mgr *Manager, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		mgr.reconcile(context.Background())
	}
}

func mustAssignment(t *testing.T, store history.DispatcherStore, id int64) history.TargetAssignmentRecord {
	t.Helper()
	got, found, err := store.GetTargetAssignment(context.Background(), id)
	if err != nil || !found {
		t.Fatalf("get assignment found=%t err=%v", found, err)
	}
	return got
}

func assertAssignmentCount(t *testing.T, store history.DispatcherStore, want int) {
	t.Helper()
	rows, err := store.ListTargetAssignments(context.Background(), history.TargetAssignmentFilter{Limit: 100})
	if err != nil {
		t.Fatalf("list assignments: %v", err)
	}
	if len(rows) != want {
		t.Fatalf("assignment count = %d, want %d: %+v", len(rows), want, rows)
	}
}

func timePtr(value time.Time) *time.Time {
	v := value.UTC()
	return &v
}

func uint32Ptr(value uint32) *uint32 {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}

func testRegistry(t *testing.T, configs []config.GunConfig) *guns.Registry {
	t.Helper()
	instances := []*guns.Instance{}
	for _, gun := range configs {
		status := plc.GunStatus{State: 1, Ready: true, CurrentAzimuth: float64(gun.ID * 10), CurrentElevation: 20}
		commands := plc.NewGunCommandServiceForGunWithTransport(gun.ID, gun.PLC, &plcTransport{status: status}, slog.Default())
		auto := autocontrol.NewControllerForGun(gun.ID, gun.PLC, commands, nil, nil, nil, nil, slog.Default())
		auto.SetEnabled(gun.AutoEnabled)
		instances = append(instances, &guns.Instance{
			Definition: guns.Definition{
				ID: gun.ID, Name: gun.Name, PLCEnabled: gun.PLCEnabled, SafetyGunID: gun.SafetyGunID,
				AutoEnabled: gun.AutoEnabled, DispatcherEligible: gun.DispatcherEligible, AutoExecutorEnabled: gun.AutoExecutorEnabled,
				MinAzimuth: gun.MinAzimuth, MaxAzimuth: gun.MaxAzimuth, MinElevation: gun.MinElevation, MaxElevation: gun.MaxElevation,
				PLC: gun.PLC,
			},
			Commands: commands,
			Auto:     auto,
		})
	}
	registry, err := guns.NewRegistry(instances)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	return registry
}

type plcTransport struct {
	status plc.GunStatus
}

func (t *plcTransport) ReadHoldingRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error) {
	words := make([]uint16, quantity)
	words[plc.RegisterState-plc.RegisterAcceptedSequenceLow] = t.status.State
	words[plc.RegisterReady-plc.RegisterAcceptedSequenceLow] = 1
	words[plc.RegisterCurrentAzimuth-plc.RegisterAcceptedSequenceLow] = plc.Uint16FromInt16(int16(t.status.CurrentAzimuth * 10))
	words[plc.RegisterCurrentElevation-plc.RegisterAcceptedSequenceLow] = plc.Uint16FromInt16(int16(t.status.CurrentElevation * 10))
	return words, nil
}

func (t *plcTransport) WriteMultipleRegisters(ctx context.Context, address uint16, values []uint16) error {
	return nil
}

func (t *plcTransport) WriteSingleRegister(ctx context.Context, address, value uint16) error {
	t.status.AcceptedSequence = uint32(value)
	return nil
}

func testSafety(t *testing.T) *safety.Service {
	t.Helper()
	transport := &safetyTransport{words: safetyWords(true)}
	cfg := config.Defaults().SafetyPLC
	cfg.StaleAfter = time.Second
	svc := safety.NewServiceWithTransport(cfg, transport, nil, slog.Default())
	svc.Start()
	t.Cleanup(svc.Close)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snap := svc.Snapshot()
		if snap.Connected && snap.Fresh && snap.SchemaSupported && snap.Ready {
			return svc
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("safety service did not become ready")
	return nil
}

type safetyTransport struct {
	words []uint16
}

func (t *safetyTransport) ReadInputRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error) {
	return append([]uint16(nil), t.words...), nil
}

func (t *safetyTransport) ReadHoldingRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error) {
	return append([]uint16(nil), t.words...), nil
}

func (t *safetyTransport) WriteSingleRegister(ctx context.Context, address, value uint16) error {
	return nil
}

func safetyWords(permissive bool) []uint16 {
	words := make([]uint16, safety.StatusRegisterCount)
	words[0] = safety.SupportedSchema
	words[1] = 1
	words[2] = uint16(safety.StateReady)
	words[3] = 1
	words[4] = boolWord(permissive)
	for gunID := 1; gunID <= 8; gunID++ {
		base := 16 + (gunID-1)*8
		words[base] = boolWord(permissive)
		words[base+3] = 1
	}
	return words
}

func boolWord(value bool) uint16 {
	if value {
		return 1
	}
	return 0
}

func hasReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
