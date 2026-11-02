package autocontrol

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"stonehenge-middleware/internal/arbitration"
	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/history"
	"stonehenge-middleware/internal/model"
	"stonehenge-middleware/internal/plc"
)

func TestTargetValidation(t *testing.T) {
	tests := []Target{
		{Azimuth: 10, Elevation: 10, Priority: 1},
		{TargetID: 1, Azimuth: -1, Elevation: 10, Priority: 1},
		{TargetID: 1, Azimuth: 360, Elevation: 10, Priority: 1},
		{TargetID: 1, Azimuth: 10, Elevation: 4.9, Priority: 1},
		{TargetID: 1, Azimuth: 10, Elevation: 86, Priority: 1},
		{TargetID: 1, Azimuth: 10, Elevation: 10, Priority: 101},
	}
	for _, tt := range tests {
		if err := ValidateTarget(tt); err == nil {
			t.Fatalf("ValidateTarget(%+v) returned nil", tt)
		}
	}
	if err := ValidateTarget(validTarget(1)); err != nil {
		t.Fatalf("valid target rejected: %v", err)
	}
}

func TestTargetAcceptedWhileDisabledButNotExecuted(t *testing.T) {
	cmds := newFakeCommands(statusReady(0))
	controller := newTestController(cmds, newFakeLifecycle(), newAutoAudit(), false)
	defer controller.Close()

	status, err := controller.SubmitTarget(context.Background(), validTarget(10))
	if err != nil {
		t.Fatalf("SubmitTarget returned error: %v", err)
	}
	if status.State != StateDisabled || status.QueuedTarget == nil {
		t.Fatalf("status = %+v, want disabled with queued target", status)
	}
	time.Sleep(10 * time.Millisecond)
	if cmds.submitCount() != 0 {
		t.Fatalf("submit count = %d, want 0", cmds.submitCount())
	}
}

func TestEnablingAutoWithQueuedTargetStartsEngagement(t *testing.T) {
	cmds := newFakeCommands(statusReady(0))
	lifecycle := newFakeLifecycle()
	audit := newAutoAudit()
	controller := newTestController(cmds, lifecycle, audit, true)
	defer controller.Close()

	if _, err := controller.SubmitTarget(context.Background(), validTarget(11)); err != nil {
		t.Fatalf("SubmitTarget returned error: %v", err)
	}
	controller.SetEnabled(true)
	req := lifecycle.waitStart(t)
	if req.Command.Mode != plc.ModeAuto || !req.Command.FireRequest || req.Command.FragmentID == nil || *req.Command.FragmentID != 11 {
		t.Fatalf("lifecycle request command = %+v", req.Command)
	}
	if req.Source != plc.CommandSourceAutoController {
		t.Fatalf("source = %q, want auto_controller", req.Source)
	}
	lifecycle.complete(req, "completed_fired", statusReadyWithShot(req.SubmittedSequence, 1))
	status := waitAutoState(t, controller, StateIdle)
	if status.ActiveTarget != nil || status.LastResult != "completed_fired" {
		t.Fatalf("status = %+v", status)
	}
	if audit.command.Source != plc.CommandSourceAutoController {
		t.Fatalf("audit source = %q", audit.command.Source)
	}
	link := audit.findLinkedUpdate()
	if link == nil || link.CommandAuditID == nil || *link.CommandAuditID != audit.commandID {
		t.Fatalf("engagement updates did not link command audit: %+v", audit.engagementUpdates)
	}
	if link.SubmittedSequence == nil || *link.SubmittedSequence != req.SubmittedSequence {
		t.Fatalf("engagement sequence update = %+v", link.SubmittedSequence)
	}
}

func TestTargetExecutesImmediatelyWhenEnabledAndReady(t *testing.T) {
	cmds := newFakeCommands(statusReady(0))
	lifecycle := newFakeLifecycle()
	controller := newTestController(cmds, lifecycle, newAutoAudit(), true)
	defer controller.Close()
	controller.SetEnabled(true)
	if _, err := controller.SubmitTarget(context.Background(), validTarget(12)); err != nil {
		t.Fatalf("SubmitTarget returned error: %v", err)
	}
	req := lifecycle.waitStart(t)
	if req.SubmittedSequence != 1 {
		t.Fatalf("submitted sequence = %d, want 1", req.SubmittedSequence)
	}
	lifecycle.complete(req, "completed_fired", statusReadyWithShot(req.SubmittedSequence, 1))
}

func TestCompletedFireRecordsOperationalHistory(t *testing.T) {
	cmds := newFakeCommands(statusReady(0))
	lifecycle := newFakeLifecycle()
	audit := newAutoAudit()
	events := history.NewInMemorySink(10)
	controller := newTestControllerWithHistory(cmds, lifecycle, audit, events, true)
	defer controller.Close()
	controller.SetEnabled(true)
	target := validTarget(212)
	target.Metadata = map[string]interface{}{"external_id": "USEA-TRACKING-NET-000212"}
	if _, err := controller.SubmitTarget(context.Background(), target); err != nil {
		t.Fatalf("SubmitTarget returned error: %v", err)
	}
	req := lifecycle.waitStart(t)
	lifecycle.complete(req, "completed_fired", statusReadyWithShot(req.SubmittedSequence, 1))
	waitAutoState(t, controller, StateIdle)

	recorded := events.Events()
	if len(recorded) != 2 {
		t.Fatalf("operational events = %d, want fired and intercepted", len(recorded))
	}
	fired := recorded[0]
	if fired.EventType != model.EventGunFired || fired.GunID == nil || *fired.GunID != plc.Gun1ID {
		t.Fatalf("fired event = %+v", fired)
	}
	if fired.ShotSequence == nil || *fired.ShotSequence != 1 || fired.ExternalFragmentID == nil || *fired.ExternalFragmentID != "USEA-TRACKING-NET-000212" {
		t.Fatalf("fired event evidence = %+v", fired)
	}
	if recorded[1].EventType != model.EventInterceptRecorded {
		t.Fatalf("second event type = %s, want %s", recorded[1].EventType, model.EventInterceptRecorded)
	}
}

func TestTargetWaitsWhileGunUnavailableStates(t *testing.T) {
	cmds := newFakeCommands(
		statusWithState(0, 2, false, false, false),
		statusWithState(0, 3, false, false, false),
		statusWithState(0, 5, false, true, false),
		statusReady(0),
	)
	lifecycle := newFakeLifecycle()
	controller := newTestController(cmds, lifecycle, newAutoAudit(), true)
	defer controller.Close()
	controller.SetEnabled(true)
	if _, err := controller.SubmitTarget(context.Background(), validTarget(13)); err != nil {
		t.Fatalf("SubmitTarget returned error: %v", err)
	}
	req := lifecycle.waitStart(t)
	if cmds.readCount() < 4 {
		t.Fatalf("read count = %d, want at least 4", cmds.readCount())
	}
	lifecycle.complete(req, "completed_fired", statusReadyWithShot(req.SubmittedSequence, 1))
}

func TestTargetDoesNotExecuteWhenFaultActive(t *testing.T) {
	cmds := newFakeCommands(statusFault(0, 2))
	controller := newTestController(cmds, newFakeLifecycle(), newAutoAudit(), true)
	defer controller.Close()
	controller.SetEnabled(true)
	if _, err := controller.SubmitTarget(context.Background(), validTarget(14)); err != nil {
		t.Fatalf("SubmitTarget returned error: %v", err)
	}
	status := waitAutoState(t, controller, StateSuspendedFault)
	if status.ActiveTarget == nil || cmds.submitCount() != 0 {
		t.Fatalf("status = %+v submit=%d", status, cmds.submitCount())
	}
}

func TestSecondTargetRejectedWhileQueuedOrActive(t *testing.T) {
	cmds := newFakeCommands(statusReady(0))
	controller := newTestController(cmds, newFakeLifecycle(), newAutoAudit(), true)
	defer controller.Close()
	if _, err := controller.SubmitTarget(context.Background(), validTarget(15)); err != nil {
		t.Fatalf("first SubmitTarget returned error: %v", err)
	}
	if _, err := controller.SubmitTarget(context.Background(), validTarget(16)); !errors.Is(err, ErrConflict) {
		t.Fatalf("second queued target error = %v, want ErrConflict", err)
	}

	lifecycle := newFakeLifecycle()
	controller2 := newTestController(newFakeCommands(statusReady(0)), lifecycle, newAutoAudit(), true)
	defer controller2.Close()
	controller2.SetEnabled(true)
	if _, err := controller2.SubmitTarget(context.Background(), validTarget(17)); err != nil {
		t.Fatalf("active first SubmitTarget returned error: %v", err)
	}
	_ = lifecycle.waitStart(t)
	if _, err := controller2.SubmitTarget(context.Background(), validTarget(18)); !errors.Is(err, ErrConflict) {
		t.Fatalf("second active target error = %v, want ErrConflict", err)
	}
}

func TestLifecycleTerminalOutcomes(t *testing.T) {
	tests := []struct {
		result    string
		wantState State
		wantLast  string
	}{
		{result: "completed_no_fire", wantState: StateIdle, wantLast: "completed_no_fire_abnormal"},
		{result: "faulted", wantState: StateSuspendedFault, wantLast: "faulted"},
		{result: "completion_timeout", wantState: StateSuspendedUnavailable, wantLast: "completion_timeout"},
		{result: "protocol_state", wantState: StateSuspendedUnavailable, wantLast: "protocol_state"},
	}
	for _, tt := range tests {
		t.Run(tt.result, func(t *testing.T) {
			cmds := newFakeCommands(statusReady(0))
			lifecycle := newFakeLifecycle()
			controller := newTestController(cmds, lifecycle, newAutoAudit(), true)
			defer controller.Close()
			controller.SetEnabled(true)
			if _, err := controller.SubmitTarget(context.Background(), validTarget(20)); err != nil {
				t.Fatalf("SubmitTarget returned error: %v", err)
			}
			req := lifecycle.waitStart(t)
			lifecycle.complete(req, tt.result, statusReadyWithShot(req.SubmittedSequence, 0))
			status := waitAutoState(t, controller, tt.wantState)
			if status.LastResult != tt.wantLast {
				t.Fatalf("last result = %q, want %q; status=%+v", status.LastResult, tt.wantLast, status)
			}
		})
	}
}

func TestDisablingAutoPreventsNewEngagementAndAllowsActiveToFinish(t *testing.T) {
	controller := newTestController(newFakeCommands(statusReady(0)), newFakeLifecycle(), newAutoAudit(), true)
	defer controller.Close()
	if status := controller.SetEnabled(false); status.State != StateDisabled {
		t.Fatalf("disabled status = %+v", status)
	}
	if _, err := controller.SubmitTarget(context.Background(), validTarget(30)); err != nil {
		t.Fatalf("SubmitTarget returned error: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if status := controller.Status(context.Background()); status.QueuedTarget == nil || status.ActiveTarget != nil {
		t.Fatalf("status = %+v, want queued while disabled", status)
	}

	lifecycle := newFakeLifecycle()
	controller2 := newTestController(newFakeCommands(statusReady(0)), lifecycle, newAutoAudit(), true)
	defer controller2.Close()
	controller2.SetEnabled(true)
	if _, err := controller2.SubmitTarget(context.Background(), validTarget(31)); err != nil {
		t.Fatalf("SubmitTarget returned error: %v", err)
	}
	req := lifecycle.waitStart(t)
	controller2.SetEnabled(false)
	lifecycle.complete(req, "completed_fired", statusReadyWithShot(req.SubmittedSequence, 1))
	status := waitAutoState(t, controller2, StateDisabled)
	if status.ActiveTarget != nil || status.LastResult != "completed_fired" {
		t.Fatalf("status = %+v", status)
	}
}

func TestSafetyPermissiveDefaultWaitsBeforeSubmit(t *testing.T) {
	cmds := newFakeCommands(statusReady(0))
	controller := newTestController(cmds, newFakeLifecycle(), newAutoAudit(), false)
	defer controller.Close()
	controller.SetEnabled(true)
	if _, err := controller.SubmitTarget(context.Background(), validTarget(40)); err != nil {
		t.Fatalf("SubmitTarget returned error: %v", err)
	}
	status := waitAutoState(t, controller, StateWaitingForSafety)
	if cmds.submitCount() != 0 || status.QueuedTarget == nil || status.ActiveTarget != nil {
		t.Fatalf("status = %+v submit=%d", status, cmds.submitCount())
	}
}

func TestSafetyProviderBlocksAndResumesSameTarget(t *testing.T) {
	cmds := newFakeCommands(statusReady(0))
	lifecycle := newFakeLifecycle()
	audit := newAutoAudit()
	controller := newTestController(cmds, lifecycle, audit, false)
	defer controller.Close()
	safety := &fakeSafetyProvider{decision: SafetyDecision{Allowed: false, Reason: "global safety inhibit active"}}
	controller.SetSafetyDecisionProvider(safety)
	controller.SetEnabled(true)
	target := validTarget(41)
	if _, err := controller.SubmitTarget(context.Background(), target); err != nil {
		t.Fatalf("SubmitTarget returned error: %v", err)
	}
	status := waitAutoState(t, controller, StateWaitingForSafety)
	if status.QueuedTarget == nil || status.QueuedTarget.TargetID != target.TargetID || status.ActiveTarget != nil {
		t.Fatalf("status while blocked = %+v, want queued target retained", status)
	}
	time.Sleep(5 * time.Millisecond)
	if cmds.submitCount() != 0 {
		t.Fatalf("submit count while safety blocked = %d, want 0", cmds.submitCount())
	}

	safety.set(SafetyDecision{Allowed: true})
	controller.signal()
	req := lifecycle.waitStart(t)
	if req.Command.FragmentID == nil || *req.Command.FragmentID != target.TargetID {
		t.Fatalf("lifecycle target = %+v, want retained target %d", req.Command.FragmentID, target.TargetID)
	}
	if cmds.submitCount() != 1 {
		t.Fatalf("submit count after safety recovery = %d, want 1", cmds.submitCount())
	}
	lifecycle.complete(req, "completed_fired", statusReadyWithShot(req.SubmittedSequence, 1))
	final := waitAutoState(t, controller, StateIdle)
	if final.ActiveTarget != nil || final.QueuedTarget != nil || final.LastResult != "completed_fired" {
		t.Fatalf("final status = %+v", final)
	}
	if audit.findLinkedUpdate() == nil {
		t.Fatalf("engagement was not linked to command audit: %+v", audit.engagementUpdates)
	}
}

func TestSafetyWaitingDoesNotAcquireArbitration(t *testing.T) {
	cmds := newFakeCommands(statusReady(0))
	controller := newTestController(cmds, newFakeLifecycle(), newAutoAudit(), false)
	defer controller.Close()
	safety := &fakeSafetyProvider{decision: SafetyDecision{Allowed: false, Reason: "safety plc status stale"}}
	controller.SetSafetyDecisionProvider(safety)
	arb := arbitration.NewService(cmds, slog.Default())
	if _, err := arb.RequestMode(context.Background(), arbitration.ModeAuto); err != nil {
		t.Fatalf("RequestMode(auto): %v", err)
	}
	controller.SetArbitrator(arb)
	controller.SetEnabled(true)
	if _, err := controller.SubmitTarget(context.Background(), validTarget(42)); err != nil {
		t.Fatalf("SubmitTarget returned error: %v", err)
	}
	status := waitAutoState(t, controller, StateWaitingForSafety)
	if status.QueuedTarget == nil || cmds.submitCount() != 0 {
		t.Fatalf("status = %+v submit=%d, want queued and no submit", status, cmds.submitCount())
	}
	if arbStatus := arb.Status(); arbStatus.ActiveOperation != arbitration.OperationNone {
		t.Fatalf("arbitration = %+v, want no active operation while waiting for safety", arbStatus)
	}
}

func TestControllerShutdownCancelsGoroutine(t *testing.T) {
	controller := newTestController(newFakeCommands(statusWithState(0, 2, false, false, false)), newFakeLifecycle(), newAutoAudit(), true)
	controller.SetEnabled(true)
	if _, err := controller.SubmitTarget(context.Background(), validTarget(50)); err != nil {
		t.Fatalf("SubmitTarget returned error: %v", err)
	}
	done := make(chan struct{})
	go func() {
		controller.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("controller did not shut down")
	}
}

func TestOfferAssignmentReusesPersistedEngagement(t *testing.T) {
	cmds := newFakeCommands(statusReady(0))
	lifecycle := newFakeLifecycle()
	audit := newAutoAudit()
	controller := newTestController(cmds, lifecycle, audit, false)
	defer controller.Close()
	controller.SetEnabled(false)
	engagementID := int64(909)

	_, err := controller.OfferAssignment(context.Background(), Assignment{
		ID:           55,
		Generation:   1,
		GunID:        1,
		EngagementID: &engagementID,
		Target:       validTarget(55),
	}, nil)
	if err != nil {
		t.Fatalf("OfferAssignment returned error: %v", err)
	}
	controller.mu.Lock()
	activeEngagementID := controller.activeEngagementID
	controller.mu.Unlock()
	if activeEngagementID != engagementID {
		t.Fatalf("active engagement id = %d, want %d", activeEngagementID, engagementID)
	}
	if got := audit.engagementRecordCount(); got != 0 {
		t.Fatalf("engagement records = %d, want 0 for resumed assignment", got)
	}
}

func TestClearResolvedAssignmentRequiresMatchingAssignment(t *testing.T) {
	cmds := newFakeCommands(statusReady(0))
	lifecycle := newFakeLifecycle()
	audit := newAutoAudit()
	controller := newTestController(cmds, lifecycle, audit, false)
	defer controller.Close()
	controller.SetEnabled(false)

	_, err := controller.OfferAssignment(context.Background(), Assignment{
		ID:         77,
		Generation: 3,
		GunID:      1,
		Target:     validTarget(77),
	}, nil)
	if err != nil {
		t.Fatalf("OfferAssignment returned error: %v", err)
	}
	seq := uint32(12)
	controller.mu.Lock()
	controller.enabled = true
	controller.active = cloneTarget(controller.queued)
	controller.queued = nil
	controller.activeSequence = &seq
	controller.setStateLocked(StateSuspendedUnavailable, "operator resolution required")
	controller.mu.Unlock()

	if controller.ClearResolvedAssignment(77, 2) {
		t.Fatal("ClearResolvedAssignment cleared mismatched generation")
	}
	status := controller.Status(context.Background())
	if status.ActiveTarget == nil || status.ActiveSequence == nil || status.State != StateSuspendedUnavailable {
		t.Fatalf("status after mismatch = %+v, want assignment retained", status)
	}

	if !controller.ClearResolvedAssignment(77, 3) {
		t.Fatal("ClearResolvedAssignment did not clear matching assignment")
	}
	status = controller.Status(context.Background())
	if status.ActiveTarget != nil || status.QueuedTarget != nil || status.ActiveSequence != nil || status.State != StateIdle {
		t.Fatalf("status after clear = %+v, want idle with no assignment", status)
	}
}

func newTestController(cmds *fakeCommands, lifecycle *fakeLifecycle, audit *autoAudit, safety bool) *Controller {
	return newTestControllerWithHistory(cmds, lifecycle, audit, nil, safety)
}

func newTestControllerWithHistory(cmds *fakeCommands, lifecycle *fakeLifecycle, audit *autoAudit, events history.Sink, safety bool) *Controller {
	cfg := config.Defaults().PLC
	cfg.AutoReadinessPollInterval = time.Millisecond
	cfg.AutoSafetyFirePermissive = safety
	controller := NewController(cfg, cmds, lifecycle, audit, audit, events, slog.Default())
	controller.Start()
	return controller
}

type fakeSafetyProvider struct {
	mu       sync.Mutex
	decision SafetyDecision
}

func (f *fakeSafetyProvider) CanFire(gunID int) SafetyDecision {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.decision
}

func (f *fakeSafetyProvider) set(decision SafetyDecision) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.decision = decision
}

func validTarget(id uint32) Target {
	return Target{TargetID: id, Azimuth: 220.5, Elevation: 40, Priority: 50, TargetClass: "test_fragment", DetectedAt: time.Now().UTC()}
}

func waitAutoState(t *testing.T, controller *Controller, state State) Status {
	t.Helper()
	deadline := time.After(200 * time.Millisecond)
	for {
		status := controller.Status(context.Background())
		if status.State == state {
			return status
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for state %s; last=%+v", state, status)
		case <-time.After(time.Millisecond):
		}
	}
}

type fakeCommands struct {
	mu          sync.Mutex
	statuses    []plc.GunStatus
	lastStatus  plc.GunStatus
	reads       int
	submits     int
	submitError error
}

func newFakeCommands(statuses ...plc.GunStatus) *fakeCommands {
	if len(statuses) == 0 {
		statuses = []plc.GunStatus{statusReady(0)}
	}
	return &fakeCommands{statuses: statuses, lastStatus: statuses[len(statuses)-1]}
}

func (f *fakeCommands) ReadStatus(ctx context.Context) (plc.GunStatus, error) {
	if err := ctx.Err(); err != nil {
		return plc.GunStatus{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	if len(f.statuses) > 0 {
		f.lastStatus = f.statuses[0]
		f.statuses = f.statuses[1:]
	}
	return f.lastStatus, nil
}

func (f *fakeCommands) Submit(ctx context.Context, gunID int, command plc.GunCommand) (plc.CommandResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.submits++
	if f.submitError != nil {
		return plc.CommandResult{ErrorType: "transport", Error: f.submitError.Error()}, f.submitError
	}
	seq := uint32(f.submits)
	baseline := f.lastStatus
	ack := f.lastStatus
	ack.AcceptedSequence = seq
	ack.State = 2
	ack.StateName = "Slewing"
	return plc.CommandResult{
		SubmittedSequence: seq,
		AcceptedSequence:  seq,
		Accepted:          true,
		CurrentState:      ack.State,
		StateName:         ack.StateName,
		Status:            &ack,
		BaselineStatus:    &baseline,
	}, nil
}

func (f *fakeCommands) submitCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.submits
}

func (f *fakeCommands) readCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads
}

type fakeLifecycle struct {
	starts chan plc.CommandLifecycleRequest
}

func newFakeLifecycle() *fakeLifecycle {
	return &fakeLifecycle{starts: make(chan plc.CommandLifecycleRequest, 10)}
}

func (f *fakeLifecycle) Start(req plc.CommandLifecycleRequest) error {
	f.starts <- req
	return nil
}

func (f *fakeLifecycle) waitStart(t *testing.T) plc.CommandLifecycleRequest {
	t.Helper()
	select {
	case req := <-f.starts:
		return req
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for lifecycle start")
		return plc.CommandLifecycleRequest{}
	}
}

func (f *fakeLifecycle) complete(req plc.CommandLifecycleRequest, result string, status plc.GunStatus) {
	req.Completion <- plc.CommandLifecycleEvent{
		AuditID:           req.AuditID,
		GunID:             req.GunID,
		SubmittedSequence: req.SubmittedSequence,
		Completion: plc.CommandAuditCompletion{
			CompletionTime: time.Now().UTC(),
			Result:         result,
			ResultingState: status.State,
			FaultCode:      status.FaultCode,
			ShotSequence:   uint32Ptr(status.ShotSequence),
		},
	}
}

type autoAudit struct {
	mu                sync.Mutex
	commandID         int64
	engagementID      int64
	engagementRecords int
	command           plc.CommandAudit
	engagement        history.EngagementRecord
	engagementUpdates []history.EngagementUpdate
}

func newAutoAudit() *autoAudit {
	return &autoAudit{commandID: 100, engagementID: 200}
}

func (a *autoAudit) RecordGunCommand(ctx context.Context, audit plc.CommandAudit) (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.command = audit
	return a.commandID, nil
}

func (a *autoAudit) UpdateGunCommandCompletion(ctx context.Context, auditID int64, completion plc.CommandAuditCompletion) error {
	return nil
}

func (a *autoAudit) UpdateGunCommandLifecycleEvidence(ctx context.Context, auditID int64, evidence plc.CommandLifecycleEvidence) error {
	return nil
}

func (a *autoAudit) RecordGunEngagement(ctx context.Context, record history.EngagementRecord) (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.engagementRecords++
	a.engagement = record
	return a.engagementID, nil
}

func (a *autoAudit) UpdateGunEngagement(ctx context.Context, id int64, update history.EngagementUpdate) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.engagementUpdates = append(a.engagementUpdates, update)
	return nil
}

func (a *autoAudit) findLinkedUpdate() *history.EngagementUpdate {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.engagementUpdates {
		if a.engagementUpdates[i].CommandAuditID != nil {
			update := a.engagementUpdates[i]
			return &update
		}
	}
	return nil
}

func (a *autoAudit) engagementRecordCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.engagementRecords
}

func statusReady(seq uint32) plc.GunStatus {
	return statusWithState(seq, 1, true, false, false)
}

func statusReadyWithShot(seq, shot uint32) plc.GunStatus {
	status := statusReady(seq)
	status.ShotSequence = shot
	return status
}

func statusFault(seq uint32, faultCode uint16) plc.GunStatus {
	status := statusWithState(seq, 7, false, false, true)
	status.FaultCode = faultCode
	status.FaultName = plc.FaultName(faultCode)
	return status
}

func statusWithState(seq uint32, state uint16, ready, lockout, fault bool) plc.GunStatus {
	return plc.GunStatus{
		AcceptedSequence: seq,
		State:            state,
		StateName:        plc.StateName(state),
		Ready:            ready,
		Lockout:          lockout,
		FaultActive:      fault,
		FaultName:        plc.FaultName(0),
	}
}
