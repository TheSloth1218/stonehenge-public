package dispatcher

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"stonehenge-middleware/internal/arbitration"
	"stonehenge-middleware/internal/autocontrol"
	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/guns"
	"stonehenge-middleware/internal/history"
	"stonehenge-middleware/internal/safety"
	"stonehenge-middleware/internal/targetfeed"
)

const ScoringVersion = "dispatcher-v1-angular-elevation-gunid"

const (
	StateAssigned          = "assigned"
	StateAcceptedByGun     = "accepted_by_gun"
	StateWaitingForSafety  = "waiting_for_safety"
	StateReconciling       = "reconciling"
	StateUncertain         = "uncertain"
	StateCommandCommitted  = "command_committed"
	StateCompleted         = "completed"
	StateReleased          = "released"
	StateRequeued          = "requeued"
	StateTerminalFailed    = "terminal_failed"
	StateTerminalUncertain = "terminal_uncertain"
)

const (
	LeaseOwnerDispatcher = "dispatcher"
	LeaseOwnerExecutor   = "executor"
)

const (
	ReconciliationAlreadyFinalized      = "already_finalized"
	ReconciliationFinalizedCommandAudit = "finalized_from_command_audit"
	ReconciliationFinalizedEngagement   = "finalized_from_engagement"
	ReconciliationFinalizedAssignment   = "finalized_from_assignment"
	ReconciliationFinalizedFeed         = "finalized_feed_from_assignment"
	ReconciliationConflictTerminal      = "uncertain_conflicting_terminal_evidence"
	ReconciliationCommittedUnknown      = "committed_result_unknown"
	ReconciliationMissingEvidence       = "protected_missing_terminal_evidence"
	ReconciliationProtectedEvidence     = "protected_command_evidence"
	ReconciliationResumedNoSequence     = "resumed_executor_no_sequence"
	ReconciliationSafeRequeueNoEvidence = "safe_requeue_no_command_evidence"
	ReconciliationSafeRequeueExpired    = "safe_requeue_expired_uncommitted"
	ReconciliationExpiredProtected      = "expired_but_protected"
)

type Reason string

const (
	ReasonDispatcherDisabled   Reason = "dispatcher_disabled"
	ReasonDispatcherIneligible Reason = "dispatcher_ineligible"
	ReasonAutoExecutorDisabled Reason = "auto_executor_disabled"
	ReasonGunDisconnected      Reason = "gun_disconnected"
	ReasonGunStale             Reason = "gun_stale"
	ReasonGunFaulted           Reason = "gun_faulted"
	ReasonGunMaintenance       Reason = "gun_maintenance"
	ReasonGunBusy              Reason = "gun_busy"
	ReasonArbitrationBusy      Reason = "arbitration_busy"
	ReasonActiveAssignment     Reason = "active_assignment"
	ReasonUncertainAssignment  Reason = "uncertain_assignment"
	ReasonSafetyUnavailable    Reason = "safety_unavailable"
	ReasonSafetyStale          Reason = "safety_stale"
	ReasonSafetyGlobalInhibit  Reason = "safety_global_inhibit"
	ReasonSafetyGunInhibit     Reason = "safety_gun_inhibit"
	ReasonTargetExpired        Reason = "target_expired"
	ReasonAzimuthOutOfRange    Reason = "azimuth_out_of_range"
	ReasonElevationOutOfRange  Reason = "elevation_out_of_range"
	ReasonRecoveryIncomplete   Reason = "recovery_incomplete"
	ReasonExecutorNotRunning   Reason = "executor_not_running"
	ReasonNoEligibleGun        Reason = "no_eligible_gun"
	ReasonTargetQueueFull      Reason = "target_queue_full"
)

type Store interface {
	RecordDispatcherDecision(ctx context.Context, decision history.DispatcherDecisionRecord) (int64, error)
	ListDispatcherDecisions(ctx context.Context, filter history.DispatcherDecisionFilter) ([]history.DispatcherDecisionRecord, error)
	GetDispatcherDecision(ctx context.Context, id int64) (history.DispatcherDecisionRecord, bool, error)
	CreateTargetAssignment(ctx context.Context, assignment history.TargetAssignmentRecord) (history.TargetAssignmentRecord, error)
	UpdateTargetAssignment(ctx context.Context, id int64, generation int, update history.TargetAssignmentUpdate) error
	RenewTargetAssignmentLease(ctx context.Context, id int64, generation int, owner string, expiresAt time.Time) (history.TargetAssignmentRecord, error)
	ListTargetAssignments(ctx context.Context, filter history.TargetAssignmentFilter) ([]history.TargetAssignmentRecord, error)
	GetTargetAssignment(ctx context.Context, id int64) (history.TargetAssignmentRecord, bool, error)
	ActiveAssignmentForTarget(ctx context.Context, targetID uint32, source string) (history.TargetAssignmentRecord, bool, error)
	ActiveAssignmentForGun(ctx context.Context, gunID int) (history.TargetAssignmentRecord, bool, error)
}

type evidenceStore interface {
	history.CommandAuditReader
	history.EngagementReader
}

type Manager struct {
	cfg      config.DispatcherConfig
	feed     *targetfeed.Manager
	registry *guns.Registry
	safety   *safety.Service
	store    Store
	logger   *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	wake   chan struct{}

	mu                   sync.RWMutex
	ready                bool
	ownershipReconciled  bool
	lastDecision         *history.DispatcherDecisionRecord
	lastError            string
	noEligibleGunReason  string
	lastPollAt           *time.Time
	targetIngestAllowed  bool
	decisionFingerprints map[string]time.Time
}

type Status struct {
	Enabled              bool                              `json:"enabled"`
	Mode                 config.DispatcherMode             `json:"mode"`
	Ready                bool                              `json:"ready"`
	OwnershipReconciled  bool                              `json:"ownership_reconciled"`
	Polling              bool                              `json:"polling"`
	ActiveAssignments    []history.TargetAssignmentRecord  `json:"active_assignments"`
	QueuedTargetCount    int                               `json:"queued_target_count"`
	UncertainCount       int                               `json:"uncertain_assignment_count"`
	EligibleGunCount     int                               `json:"eligible_gun_count"`
	LeaseDuration        string                            `json:"lease_duration"`
	LeaseRenewalInterval string                            `json:"lease_renewal_interval"`
	LastDecision         *history.DispatcherDecisionRecord `json:"last_decision,omitempty"`
	LastError            string                            `json:"last_error,omitempty"`
	TargetIngestAllowed  bool                              `json:"target_ingest_allowed"`
	NoEligibleGunReason  string                            `json:"no_eligible_gun_reason,omitempty"`
	LastPollAt           *time.Time                        `json:"last_poll_at,omitempty"`
}

func New(cfg config.DispatcherConfig, feed *targetfeed.Manager, registry *guns.Registry, safetyService *safety.Service, store Store, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 500 * time.Millisecond
	}
	if cfg.AssignmentLeaseDuration <= 0 {
		cfg.AssignmentLeaseDuration = 90 * time.Second
	}
	if cfg.LeaseRenewalInterval <= 0 {
		cfg.LeaseRenewalInterval = 30 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{cfg: cfg, feed: feed, registry: registry, safety: safetyService, store: store, logger: logger, ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1), decisionFingerprints: make(map[string]time.Time)}
}

func (m *Manager) Start() {
	if m == nil || !m.cfg.Enabled || m.cfg.Mode == config.DispatcherModeDisabled {
		return
	}
	m.reconcile(context.Background())
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.loop()
	}()
}

func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.cancel()
	m.signal()
	m.wg.Wait()
}

func (m *Manager) Reconcile(ctx context.Context) {
	if m == nil {
		return
	}
	m.reconcile(ctx)
}

func (m *Manager) signal() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *Manager) loop() {
	ticker := time.NewTicker(m.cfg.PollInterval)
	defer ticker.Stop()
	reconcileTicker := time.NewTicker(m.cfg.ReconciliationInterval)
	defer reconcileTicker.Stop()
	for {
		m.poll(m.ctx)
		select {
		case <-m.ctx.Done():
			return
		case <-m.wake:
		case <-ticker.C:
		case <-reconcileTicker.C:
			m.reconcile(m.ctx)
		}
	}
}

func (m *Manager) reconcile(ctx context.Context) {
	m.mu.Lock()
	m.ownershipReconciled = false
	m.ready = false
	m.mu.Unlock()
	if m.store != nil {
		active := true
		rows, err := m.store.ListTargetAssignments(ctx, history.TargetAssignmentFilter{Active: &active, Limit: 100})
		if err != nil {
			m.setError("ownership reconciliation failed: " + err.Error())
		}
		for _, row := range rows {
			m.reconcileAssignment(ctx, row)
		}
		inactive := false
		terminalRows, err := m.store.ListTargetAssignments(ctx, history.TargetAssignmentFilter{Active: &inactive, Limit: 100})
		if err != nil {
			m.setError("terminal assignment reconciliation failed: " + err.Error())
		}
		for _, row := range terminalRows {
			m.finalizeCompletedAssignment(ctx, row, time.Now().UTC())
		}
	}
	m.mu.Lock()
	m.ownershipReconciled = true
	m.ready = true
	m.mu.Unlock()
}

func (m *Manager) reconcileAssignment(ctx context.Context, row history.TargetAssignmentRecord) {
	now := time.Now().UTC()
	if m.finalizeCompletedAssignment(ctx, row, now) {
		return
	}
	if commandEvidence(row) {
		owner := LeaseOwnerExecutor
		state := row.State
		uncertaintyReason := ""
		if row.State != StateCommandCommitted {
			state = StateUncertain
			uncertaintyReason = "command_or_engagement_evidence_present"
		}
		expiry := now.Add(m.cfg.AssignmentLeaseDuration)
		_ = m.store.UpdateTargetAssignment(ctx, row.ID, row.Generation, history.TargetAssignmentUpdate{
			State:                state,
			LeaseOwner:           &owner,
			LeaseExpiresAt:       &expiry,
			ReconciliationResult: ReconciliationProtectedEvidence,
			UncertaintyReason:    uncertaintyReason,
			UpdatedAt:            now,
		})
		return
	}
	switch row.State {
	case StateAssigned:
		_ = m.store.UpdateTargetAssignment(ctx, row.ID, row.Generation, history.TargetAssignmentUpdate{State: StateRequeued, ClearLease: true, ReleaseReason: "restart_reconciled_unaccepted", ReconciliationResult: ReconciliationSafeRequeueNoEvidence, UpdatedAt: now})
	case StateAcceptedByGun, StateWaitingForSafety:
		owner := LeaseOwnerExecutor
		expiry := now.Add(m.cfg.AssignmentLeaseDuration)
		_ = m.store.UpdateTargetAssignment(ctx, row.ID, row.Generation, history.TargetAssignmentUpdate{LeaseOwner: &owner, LeaseExpiresAt: &expiry, ReconciliationResult: ReconciliationResumedNoSequence, UpdatedAt: now})
		m.resumeExecutor(ctx, row)
	default:
		owner := row.LeaseOwner
		if owner == "" {
			owner = LeaseOwnerExecutor
		}
		expiry := now.Add(m.cfg.AssignmentLeaseDuration)
		_ = m.store.UpdateTargetAssignment(ctx, row.ID, row.Generation, history.TargetAssignmentUpdate{LeaseOwner: &owner, LeaseExpiresAt: &expiry, ReconciliationResult: "active_preserved", UpdatedAt: now})
	}
}

type terminalEvidence struct {
	source string
	state  string
	result string
	err    string
}

func (m *Manager) finalizeCompletedAssignment(ctx context.Context, row history.TargetAssignmentRecord, now time.Time) bool {
	if row.State == StateTerminalUncertain {
		return true
	}
	evidence, readErr := m.terminalEvidence(ctx, row)
	if readErr != nil {
		m.setError("terminal evidence read failed: " + readErr.Error())
		return false
	}
	if len(evidence) == 0 {
		if reason := m.committedUnknownReason(ctx, row); reason != "" {
			m.markAssignmentUncertain(ctx, row, now, ReconciliationCommittedUnknown, reason)
			return true
		}
		if m.feedRecordCompleted(row) && assignmentActive(row.State) {
			m.markAssignmentUncertain(ctx, row, now, ReconciliationMissingEvidence, "feed_completed_without_authoritative_assignment_evidence")
			return true
		}
		return false
	}
	result := evidence[0].result
	terminalState := evidence[0].state
	source := evidence[0].source
	message := evidence[0].err
	for _, item := range evidence[1:] {
		if item.result != result || item.state != terminalState {
			m.markAssignmentUncertain(ctx, row, now, ReconciliationConflictTerminal, fmt.Sprintf("%s=%s/%s conflicts with %s=%s/%s", source, terminalState, result, item.source, item.state, item.result))
			return true
		}
		if message == "" {
			message = item.err
		}
	}
	if !assignmentActive(row.State) {
		if m.feed != nil && row.State == StateCompleted {
			m.feed.MarkCompleted(fromAssignment(row), result, message, now)
			if row.ReconciliationResult == "" {
				_ = m.store.UpdateTargetAssignment(ctx, row.ID, row.Generation, history.TargetAssignmentUpdate{ReconciliationResult: ReconciliationAlreadyFinalized, UpdatedAt: now})
			}
		}
		return true
	}
	reconciliationResult := ReconciliationFinalizedAssignment
	if source == "command_audit" {
		reconciliationResult = ReconciliationFinalizedCommandAudit
	}
	if source == "engagement" {
		reconciliationResult = ReconciliationFinalizedEngagement
	}
	update := history.TargetAssignmentUpdate{
		State:                terminalState,
		ClearLease:           true,
		CompletionState:      result,
		ReleaseReason:        message,
		ReconciliationResult: reconciliationResult,
		UpdatedAt:            now,
	}
	if terminalState == StateCompleted && m.feed != nil {
		m.feed.MarkCompleted(fromAssignment(row), result, message, now)
	}
	if terminalState == StateTerminalFailed && m.feed != nil {
		m.feed.MarkCompleted(fromAssignment(row), result, message, now)
	}
	if err := m.store.UpdateTargetAssignment(ctx, row.ID, row.Generation, update); err != nil {
		m.setError("assignment finalization failed: " + err.Error())
	}
	return true
}

func (m *Manager) terminalEvidence(ctx context.Context, row history.TargetAssignmentRecord) ([]terminalEvidence, error) {
	out := []terminalEvidence{}
	if state, result, ok := assignmentTerminalEvidence(row); ok {
		out = append(out, terminalEvidence{source: "assignment", state: state, result: result, err: row.ReleaseReason})
	}
	reader, ok := m.store.(evidenceStore)
	if !ok {
		return out, nil
	}
	if row.CommandAuditID != nil {
		command, found, err := reader.GetGunCommand(ctx, *row.CommandAuditID)
		if err != nil {
			return nil, err
		}
		if found {
			if state, result, ok := commandTerminalEvidence(command); ok {
				out = append(out, terminalEvidence{source: "command_audit", state: state, result: result, err: command.Error})
			}
		}
	}
	if row.EngagementID != nil {
		engagement, found, err := reader.GetGunEngagement(ctx, *row.EngagementID)
		if err != nil {
			return nil, err
		}
		if found {
			if state, result, ok := engagementTerminalEvidence(engagement); ok {
				out = append(out, terminalEvidence{source: "engagement", state: state, result: result, err: engagement.Error})
			}
		}
	}
	return out, nil
}

func (m *Manager) markAssignmentUncertain(ctx context.Context, row history.TargetAssignmentRecord, now time.Time, result, reason string) {
	owner := LeaseOwnerExecutor
	expiry := now.Add(m.cfg.AssignmentLeaseDuration)
	_ = m.store.UpdateTargetAssignment(ctx, row.ID, row.Generation, history.TargetAssignmentUpdate{
		State:                StateUncertain,
		LeaseOwner:           &owner,
		LeaseExpiresAt:       &expiry,
		CompletionState:      completionStateForUncertainty(result),
		ReconciliationResult: result,
		UncertaintyReason:    reason,
		UpdatedAt:            now,
	})
}

func completionStateForUncertainty(reconciliationResult string) string {
	if reconciliationResult == ReconciliationCommittedUnknown {
		return ReconciliationCommittedUnknown
	}
	return ""
}

func (m *Manager) feedRecordCompleted(row history.TargetAssignmentRecord) bool {
	if m.feed == nil {
		return false
	}
	target := fromAssignment(row)
	for _, rec := range m.feed.Targets() {
		if rec.Target.Source == target.Source && rec.Target.TargetID == target.TargetID && rec.State == targetfeed.StateCompleted {
			return true
		}
	}
	return false
}

func assignmentTerminalEvidence(row history.TargetAssignmentRecord) (string, string, bool) {
	switch row.State {
	case StateCompleted:
		return StateCompleted, normalizeTerminalResult(row.CompletionState, StateCompleted), true
	case StateTerminalFailed:
		return StateTerminalFailed, normalizeTerminalResult(row.CompletionState, StateTerminalFailed), true
	default:
		return "", "", false
	}
}

func commandTerminalEvidence(command history.GunCommandRecord) (string, string, bool) {
	if command.LifecycleEvidence.ShotObserved {
		return StateCompleted, "completed_fired", true
	}
	if command.CompletionTime == nil && !commandResultTerminal(command.Result) {
		return "", "", false
	}
	return terminalStateForResult(command.Result)
}

func engagementTerminalEvidence(engagement history.GunEngagementRecord) (string, string, bool) {
	if engagement.CompletedAt == nil && !commandResultTerminal(engagement.Result) {
		return "", "", false
	}
	return terminalStateForResult(engagement.Result)
}

func terminalStateForResult(result string) (string, string, bool) {
	result = normalizeTerminalResult(result, "")
	switch result {
	case "completed", "completed_fired", "completed_no_fire", "completed_no_fire_abnormal":
		return StateCompleted, result, true
	case "faulted", "terminal_failure", "failed", "failed_before_commit", "failed_after_commit_confirmed":
		return StateTerminalFailed, result, true
	default:
		return "", "", false
	}
}

func commandResultTerminal(result string) bool {
	_, _, ok := terminalStateForResult(result)
	return ok
}

func normalizeTerminalResult(result, state string) string {
	switch result {
	case "":
		if state == StateCompleted {
			return "completed"
		}
		if state == StateTerminalFailed {
			return "terminal_failure"
		}
		return ""
	case "success":
		return "completed"
	default:
		return result
	}
}

func (m *Manager) poll(ctx context.Context) {
	now := time.Now().UTC()
	m.mu.Lock()
	m.lastPollAt = &now
	m.mu.Unlock()
	m.renewActiveLeases(ctx, now)
	m.handleExpiredLeases(ctx, now)
	if !m.isReady() || m.feed == nil {
		return
	}
	pending := m.feed.PendingTargets(now)
	if len(pending) == 0 {
		m.refreshIngestAllowed(ctx)
		return
	}
	target := pending[0]
	decision := m.Decide(ctx, target.Target, now)
	if m.suppressDuplicateDecision(decision, now) {
		m.refreshIngestAllowed(ctx)
		return
	}
	decisionID, err := m.recordDecision(ctx, decision)
	if err != nil {
		m.setError(err.Error())
		return
	}
	decision.ID = decisionID
	m.setLastDecision(decision)
	if m.cfg.Mode != config.DispatcherModeActive || decision.SelectedGunID == nil {
		if decision.NoSelectionReason != "" {
			m.mu.Lock()
			m.noEligibleGunReason = decision.NoSelectionReason
			m.mu.Unlock()
		}
		m.refreshIngestAllowed(ctx)
		return
	}
	if m.store == nil {
		m.setError("dispatcher ownership store unavailable")
		return
	}
	assignment, err := m.store.CreateTargetAssignment(ctx, history.TargetAssignmentRecord{
		TargetID:             target.Target.TargetID,
		TargetSource:         target.Target.Source,
		ExternalID:           externalID(target.Target),
		GunID:                *decision.SelectedGunID,
		State:                StateAssigned,
		AssignedAt:           now,
		LeaseExpiresAt:       now.Add(m.cfg.AssignmentLeaseDuration),
		LeaseOwner:           LeaseOwnerDispatcher,
		LastLeaseRenewedAt:   &now,
		DispatcherDecisionID: decisionID,
		UpdatedAt:            now,
		TargetPayload:        targetPayload(target.Target),
	})
	if err != nil {
		m.setError("assignment failed: " + err.Error())
		return
	}
	inst, ok := m.registry.Get(assignment.GunID)
	if !ok || inst.Auto == nil {
		_ = m.store.UpdateTargetAssignment(ctx, assignment.ID, assignment.Generation, history.TargetAssignmentUpdate{State: StateRequeued, ClearLease: true, ReleaseReason: "executor_unavailable", UpdatedAt: now})
		return
	}
	if !m.feed.MarkSubmitted(target.Target, now) {
		_ = m.store.UpdateTargetAssignment(ctx, assignment.ID, assignment.Generation, history.TargetAssignmentUpdate{State: StateRequeued, ClearLease: true, ReleaseReason: "queue_state_changed", UpdatedAt: now})
		return
	}
	status, err := inst.Auto.OfferAssignment(ctx, autocontrol.Assignment{
		ID:         assignment.ID,
		Generation: assignment.Generation,
		GunID:      assignment.GunID,
		Target:     toAutoTarget(target.Target),
	}, m)
	if err != nil {
		_ = m.store.UpdateTargetAssignment(ctx, assignment.ID, assignment.Generation, history.TargetAssignmentUpdate{State: StateRequeued, ClearLease: true, ReleaseReason: err.Error(), UpdatedAt: now})
		m.feed.Requeue(target.Target, err.Error(), now)
		m.setError(err.Error())
		return
	}
	owner := LeaseOwnerExecutor
	expiry := now.Add(m.cfg.AssignmentLeaseDuration)
	_ = m.store.UpdateTargetAssignment(ctx, assignment.ID, assignment.Generation, history.TargetAssignmentUpdate{State: StateAcceptedByGun, LeaseOwner: &owner, LeaseExpiresAt: &expiry, UpdatedAt: now})
	m.logger.Info("dispatcher assigned target", "target_id", target.Target.TargetID, "gun_id", assignment.GunID, "assignment_id", assignment.ID, "generation", assignment.Generation, "auto_state", status.State)
	m.refreshIngestAllowed(ctx)
}

func (m *Manager) renewActiveLeases(ctx context.Context, now time.Time) {
	if m.store == nil {
		return
	}
	active := true
	rows, err := m.store.ListTargetAssignments(ctx, history.TargetAssignmentFilter{Active: &active, Limit: 100})
	if err != nil {
		m.setError("assignment lease scan failed: " + err.Error())
		return
	}
	for _, row := range rows {
		if row.LeaseOwner == "" || row.State == StateUncertain {
			continue
		}
		due := row.LastLeaseRenewedAt == nil || now.Sub(*row.LastLeaseRenewedAt) >= m.cfg.LeaseRenewalInterval
		nearExpiry := now.Add(m.cfg.LeaseRenewalInterval).After(row.LeaseExpiresAt)
		if !due && !nearExpiry {
			continue
		}
		if _, err := m.store.RenewTargetAssignmentLease(ctx, row.ID, row.Generation, row.LeaseOwner, now.Add(m.cfg.AssignmentLeaseDuration)); err != nil {
			m.setError("assignment lease renewal failed: " + err.Error())
			m.logger.Warn("assignment lease renewal failed", "assignment_id", row.ID, "generation", row.Generation, "owner", row.LeaseOwner, "state", row.State, "error", err)
		}
	}
}

func (m *Manager) handleExpiredLeases(ctx context.Context, now time.Time) {
	if m.store == nil {
		return
	}
	active := true
	rows, err := m.store.ListTargetAssignments(ctx, history.TargetAssignmentFilter{Active: &active, Limit: 100})
	if err != nil {
		m.setError("assignment expiration scan failed: " + err.Error())
		return
	}
	for _, row := range rows {
		if row.LeaseExpiresAt.After(now) || row.State == StateUncertain {
			continue
		}
		if safeToRequeueExpired(row) {
			_ = m.store.UpdateTargetAssignment(ctx, row.ID, row.Generation, history.TargetAssignmentUpdate{State: StateRequeued, ClearLease: true, ReleaseReason: "lease_expired_before_commit", ReconciliationResult: ReconciliationSafeRequeueExpired, UpdatedAt: now})
			if m.feed != nil {
				m.feed.Requeue(fromAssignment(row), "lease_expired_before_commit", now)
			}
			continue
		}
		owner := LeaseOwnerExecutor
		expiry := now.Add(m.cfg.AssignmentLeaseDuration)
		_ = m.store.UpdateTargetAssignment(ctx, row.ID, row.Generation, history.TargetAssignmentUpdate{State: StateUncertain, LeaseOwner: &owner, LeaseExpiresAt: &expiry, ReconciliationResult: ReconciliationExpiredProtected, UncertaintyReason: "expired_after_possible_commit", UpdatedAt: now})
	}
}

func (m *Manager) Decide(ctx context.Context, target targetfeed.Target, now time.Time) history.DispatcherDecisionRecord {
	candidates := m.evaluate(ctx, target, now)
	eligible := make([]history.DispatcherCandidate, 0)
	assignable := make([]history.DispatcherCandidate, 0)
	for _, c := range candidates {
		if c.Eligible {
			eligible = append(eligible, c)
			assignable = append(assignable, c)
			continue
		}
		if safetyHoldAssignable(c) {
			assignable = append(assignable, c)
		}
	}
	sort.SliceStable(assignable, func(i, j int) bool {
		if *assignable[i].Score != *assignable[j].Score {
			return *assignable[i].Score < *assignable[j].Score
		}
		return assignable[i].GunID < assignable[j].GunID
	})
	outcome := "no_selection"
	reason := string(ReasonNoEligibleGun)
	var selected *int
	var selectedScore *float64
	if len(assignable) > 0 {
		outcome = "selected"
		reason = ""
		gunID := assignable[0].GunID
		score := *assignable[0].Score
		selected = &gunID
		selectedScore = &score
	}
	return history.DispatcherDecisionRecord{
		TargetID:          target.TargetID,
		TargetSource:      target.Source,
		ExternalID:        externalID(target),
		DecisionTime:      now,
		Mode:              string(m.cfg.Mode),
		Outcome:           outcome,
		SelectedGunID:     selected,
		NoSelectionReason: reason,
		EligibleGunCount:  len(eligible),
		ScoringVersion:    ScoringVersion,
		Candidates:        candidates,
		SelectedScore:     selectedScore,
		TieBreakReason:    "lowest_gun_id",
		ShadowOnly:        m.cfg.Mode == config.DispatcherModeShadow,
	}
}

func safetyHoldAssignable(candidate history.DispatcherCandidate) bool {
	return candidate.Score != nil && len(candidate.Reasons) == 1 && candidate.Reasons[0] == string(ReasonSafetyGunInhibit)
}

func (m *Manager) evaluate(ctx context.Context, target targetfeed.Target, now time.Time) []history.DispatcherCandidate {
	instances := m.registry.List()
	out := make([]history.DispatcherCandidate, 0, len(instances))
	safetySnapshot := safety.Snapshot{}
	if m.safety != nil {
		safetySnapshot = m.safety.Snapshot()
	}
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		reasons := m.rejectionReasons(ctx, inst, target, now, safetySnapshot)
		var score *float64
		if target.Azimuth >= inst.Definition.MinAzimuth && target.Azimuth <= inst.Definition.MaxAzimuth &&
			target.Elevation >= inst.Definition.MinElevation && target.Elevation <= inst.Definition.MaxElevation {
			if inst.Commands != nil {
				if status, err := inst.Commands.ReadStatus(ctx); err == nil {
					value := angularDistance(status.CurrentAzimuth, target.Azimuth) + math.Abs(status.CurrentElevation-target.Elevation)*m.cfg.ElevationWeight
					score = &value
				}
			}
		}
		out = append(out, history.DispatcherCandidate{GunID: inst.Definition.ID, Eligible: len(reasons) == 0, Reasons: reasons, Score: score})
	}
	return out
}

func (m *Manager) rejectionReasons(ctx context.Context, inst *guns.Instance, target targetfeed.Target, now time.Time, safetySnapshot safety.Snapshot) []string {
	reasons := []string{}
	def := inst.Definition
	if !m.cfg.Enabled || m.cfg.Mode == config.DispatcherModeDisabled {
		reasons = append(reasons, string(ReasonDispatcherDisabled))
	}
	if !def.PLCEnabled {
		reasons = append(reasons, string(ReasonGunDisconnected))
	}
	if !def.DispatcherEligible {
		reasons = append(reasons, string(ReasonDispatcherIneligible))
	}
	if !def.AutoExecutorEnabled || !def.AutoEnabled {
		reasons = append(reasons, string(ReasonAutoExecutorDisabled))
	}
	if target.Azimuth < def.MinAzimuth || target.Azimuth > def.MaxAzimuth {
		reasons = append(reasons, string(ReasonAzimuthOutOfRange))
	}
	if target.Elevation < def.MinElevation || target.Elevation > def.MaxElevation {
		reasons = append(reasons, string(ReasonElevationOutOfRange))
	}
	if m.feed != nil && now.Sub(target.DetectedAt) > m.feed.Config().TargetTTL {
		reasons = append(reasons, string(ReasonTargetExpired))
	}
	if m.store != nil {
		if row, found, _ := m.store.ActiveAssignmentForGun(ctx, def.ID); found {
			reasons = append(reasons, string(ReasonActiveAssignment))
			if row.State == StateUncertain || row.State == StateReconciling {
				reasons = append(reasons, string(ReasonUncertainAssignment))
			}
		}
	}
	if inst.Auto == nil {
		reasons = append(reasons, string(ReasonExecutorNotRunning))
	} else {
		status := inst.Auto.Status(ctx)
		if status.ActiveTarget != nil || status.QueuedTarget != nil {
			reasons = append(reasons, string(ReasonGunBusy))
		}
		if !status.Enabled {
			reasons = append(reasons, string(ReasonAutoExecutorDisabled))
		}
	}
	if inst.Arbitration != nil {
		arb := inst.Arbitration.Status()
		if arb.ActiveOperation != arbitration.OperationNone {
			reasons = append(reasons, string(ReasonArbitrationBusy))
		}
		if arb.Mode == arbitration.ModeMaintenance {
			reasons = append(reasons, string(ReasonGunMaintenance))
		}
	}
	if inst.Commands == nil {
		reasons = append(reasons, string(ReasonGunDisconnected))
	} else {
		status, err := inst.Commands.ReadStatus(ctx)
		if err != nil {
			reasons = append(reasons, string(ReasonGunDisconnected))
		} else {
			if status.FaultActive {
				reasons = append(reasons, string(ReasonGunFaulted))
			}
			if !status.Ready || status.Firing || status.Lockout {
				reasons = append(reasons, string(ReasonGunBusy))
			}
		}
	}
	if m.safety == nil || !safetySnapshot.Configured || !safetySnapshot.Connected {
		reasons = append(reasons, string(ReasonSafetyUnavailable))
	} else {
		if !safetySnapshot.Fresh || !safetySnapshot.SchemaSupported {
			reasons = append(reasons, string(ReasonSafetyStale))
		}
		if safetySnapshot.GlobalInhibitActive || !safetySnapshot.GlobalFirePermissive {
			reasons = append(reasons, string(ReasonSafetyGlobalInhibit))
		}
		for _, gun := range safetySnapshot.Guns {
			if gun.GunID == def.SafetyGunID && (gun.InhibitActive || !gun.FirePermissive) {
				reasons = append(reasons, string(ReasonSafetyGunInhibit))
				break
			}
		}
	}
	return dedupe(reasons)
}

func (m *Manager) AssignmentStateChanged(ctx context.Context, assignment autocontrol.Assignment, state autocontrol.AssignmentState) {
	if m.store == nil {
		return
	}
	now := time.Now().UTC()
	owner := LeaseOwnerExecutor
	expiry := now.Add(m.cfg.AssignmentLeaseDuration)
	update := history.TargetAssignmentUpdate{State: string(state.State), LeaseOwner: &owner, LeaseExpiresAt: &expiry, UpdatedAt: now}
	if state.CommandSequence != nil {
		update.CommandSequence = state.CommandSequence
		update.State = StateCommandCommitted
	}
	if state.CommandAuditID != nil {
		update.CommandAuditID = state.CommandAuditID
	}
	if state.EngagementID != nil {
		update.EngagementID = state.EngagementID
	}
	if state.State == autocontrol.AssignmentStateWaitingForSafety {
		update.State = StateWaitingForSafety
	}
	if state.State == autocontrol.AssignmentStateCompleted {
		update.State = StateCompleted
		update.ClearLease = true
		update.CompletionState = state.Result
		m.feed.MarkCompleted(fromAutoTarget(assignment.Target), state.Result, state.Error, now)
	}
	if state.State == autocontrol.AssignmentStateUncertain {
		update.State = StateUncertain
		update.CompletionState = state.Result
		update.UncertaintyReason = state.Error
		update.ReconciliationResult = ReconciliationCommittedUnknown
	}
	if state.State == autocontrol.AssignmentStateFailed {
		update.State = StateTerminalFailed
		update.ClearLease = true
		update.CompletionState = state.Result
		update.ReleaseReason = state.Error
		m.feed.MarkCompleted(fromAutoTarget(assignment.Target), state.Result, state.Error, now)
	}
	if err := m.store.UpdateTargetAssignment(ctx, assignment.ID, assignment.Generation, update); err != nil {
		m.setError("assignment update failed: " + err.Error())
	}
}

func (m *Manager) Status(ctx context.Context) Status {
	pending := 0
	if m.feed != nil {
		pending = len(m.feed.PendingTargets(time.Now().UTC()))
	}
	activeRows := []history.TargetAssignmentRecord{}
	if m.store != nil {
		active := true
		rows, _ := m.store.ListTargetAssignments(ctx, history.TargetAssignmentFilter{Active: &active, Limit: 50})
		activeRows = rows
	}
	uncertain := 0
	for _, row := range activeRows {
		if row.State == StateUncertain || row.State == StateReconciling {
			uncertain++
		}
	}
	m.mu.RLock()
	status := Status{
		Enabled:              m.cfg.Enabled,
		Mode:                 m.cfg.Mode,
		Ready:                m.ready,
		OwnershipReconciled:  m.ownershipReconciled,
		Polling:              m.cfg.Enabled && m.cfg.Mode != config.DispatcherModeDisabled,
		ActiveAssignments:    activeRows,
		QueuedTargetCount:    pending,
		UncertainCount:       uncertain,
		LeaseDuration:        m.cfg.AssignmentLeaseDuration.String(),
		LeaseRenewalInterval: m.cfg.LeaseRenewalInterval.String(),
		LastDecision:         cloneDecisionPtr(m.lastDecision),
		LastError:            m.lastError,
		TargetIngestAllowed:  m.targetIngestAllowed,
		NoEligibleGunReason:  m.noEligibleGunReason,
		LastPollAt:           cloneTimePtr(m.lastPollAt),
	}
	if m.lastDecision != nil {
		status.EligibleGunCount = m.lastDecision.EligibleGunCount
	}
	m.mu.RUnlock()
	return status
}

func (m *Manager) TargetIngestAllowed(ctx context.Context) bool {
	m.refreshIngestAllowed(ctx)
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.targetIngestAllowed
}

func (m *Manager) refreshIngestAllowed(ctx context.Context) {
	allowed := false
	reason := ""
	if m.isReady() && m.cfg.Enabled && m.cfg.Mode == config.DispatcherModeActive {
		rejections := []string{}
		queueableCapacityOnly := false
		considered := false
		for _, inst := range m.registry.List() {
			if inst == nil {
				continue
			}
			def := inst.Definition
			if !def.PLCEnabled || !def.AutoEnabled || !def.AutoExecutorEnabled || !def.DispatcherEligible {
				continue
			}
			considered = true
			snapshot := safety.Snapshot{}
			if m.safety != nil {
				snapshot = m.safety.Snapshot()
			}
			candidates := m.rejectionReasons(ctx, inst, targetfeed.Target{TargetID: 1, Source: "readiness", Azimuth: inst.Definition.MinAzimuth, Elevation: inst.Definition.MinElevation, DetectedAt: time.Now().UTC()}, time.Now().UTC(), snapshot)
			if len(candidates) == 0 {
				allowed = true
				reason = ""
				break
			} else if queueableBusyReasons(candidates) {
				if m.feedQueueHasCapacity(time.Now().UTC()) {
					queueableCapacityOnly = true
					rejections = append(rejections, "gun_busy_queue_capacity_available")
				} else {
					rejections = append(rejections, string(ReasonTargetQueueFull))
				}
			} else {
				rejections = append(rejections, candidates...)
			}
		}
		if !allowed && !considered {
			reason = string(ReasonDispatcherIneligible)
		} else if !allowed {
			if queueableCapacityOnly && allQueueableCapacityReasons(rejections) {
				allowed = true
				reason = "gun_busy_queue_capacity_available"
			} else {
				reason = preferredReadinessReason(rejections)
			}
		}
	} else {
		reason = "dispatcher_not_ready"
	}
	m.mu.Lock()
	m.targetIngestAllowed = allowed
	m.noEligibleGunReason = reason
	m.mu.Unlock()
}

func (m *Manager) feedQueueHasCapacity(now time.Time) bool {
	if m.feed == nil {
		return false
	}
	cfg := m.feed.Config()
	maxPending := cfg.MaxPending
	if maxPending <= 0 {
		maxPending = 25
	}
	return len(m.feed.PendingTargets(now)) < maxPending
}

func queueableBusyReasons(reasons []string) bool {
	if len(reasons) == 0 {
		return false
	}
	hasBusy := false
	for _, reason := range reasons {
		switch reason {
		case string(ReasonActiveAssignment), string(ReasonGunBusy), string(ReasonArbitrationBusy):
			hasBusy = true
		case string(ReasonSafetyGunInhibit):
			// A safety-held gun that already owns work is queue-capacity safe:
			// the active assignment blocks new dispatch, while the queue remains persistent.
		default:
			return false
		}
	}
	return hasBusy
}

func allQueueableCapacityReasons(reasons []string) bool {
	if len(reasons) == 0 {
		return false
	}
	for _, reason := range reasons {
		if reason != "gun_busy_queue_capacity_available" {
			return false
		}
	}
	return true
}

func preferredReadinessReason(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	priority := []string{
		string(ReasonUncertainAssignment),
		string(ReasonSafetyUnavailable),
		string(ReasonSafetyStale),
		string(ReasonSafetyGlobalInhibit),
		string(ReasonSafetyGunInhibit),
		string(ReasonGunDisconnected),
		string(ReasonGunStale),
		string(ReasonDispatcherDisabled),
		string(ReasonDispatcherIneligible),
		string(ReasonAutoExecutorDisabled),
		string(ReasonExecutorNotRunning),
		string(ReasonGunMaintenance),
		string(ReasonGunFaulted),
		string(ReasonTargetQueueFull),
		string(ReasonActiveAssignment),
		string(ReasonGunBusy),
		string(ReasonArbitrationBusy),
	}
	for _, want := range priority {
		for _, reason := range reasons {
			if reason == want {
				return reason
			}
		}
	}
	return reasons[0]
}

func (m *Manager) recordDecision(ctx context.Context, decision history.DispatcherDecisionRecord) (int64, error) {
	if m.store == nil {
		return 0, nil
	}
	return m.store.RecordDispatcherDecision(ctx, decision)
}

func (m *Manager) suppressDuplicateDecision(decision history.DispatcherDecisionRecord, now time.Time) bool {
	if m.cfg.Mode != config.DispatcherModeShadow {
		return false
	}
	fingerprint := decisionFingerprint(decision)
	m.mu.Lock()
	defer m.mu.Unlock()
	if last, ok := m.decisionFingerprints[fingerprint]; ok && now.Sub(last) < m.cfg.ShadowDecisionRetention {
		return true
	}
	m.decisionFingerprints[fingerprint] = now
	for key, last := range m.decisionFingerprints {
		if now.Sub(last) > m.cfg.ShadowDecisionRetention {
			delete(m.decisionFingerprints, key)
		}
	}
	return false
}

func decisionFingerprint(decision history.DispatcherDecisionRecord) string {
	parts := []string{fmt.Sprintf("%s:%d:%s:%s", decision.TargetSource, decision.TargetID, decision.Mode, decision.Outcome)}
	if decision.SelectedGunID != nil {
		parts = append(parts, fmt.Sprintf("selected=%d", *decision.SelectedGunID))
	}
	for _, candidate := range decision.Candidates {
		parts = append(parts, fmt.Sprintf("gun=%d eligible=%t reasons=%v", candidate.GunID, candidate.Eligible, candidate.Reasons))
		if candidate.Score != nil {
			parts = append(parts, fmt.Sprintf("score=%.3f", *candidate.Score))
		}
	}
	return strings.Join(parts, "|")
}

func (m *Manager) isReady() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ready && m.ownershipReconciled
}

func (m *Manager) setError(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastError = message
}

func (m *Manager) setLastDecision(decision history.DispatcherDecisionRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastDecision = &decision
}

func angularDistance(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 360)
	if d > 180 {
		return 360 - d
	}
	return d
}

func dedupe(in []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range in {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func toAutoTarget(target targetfeed.Target) autocontrol.Target {
	return autocontrol.Target{TargetID: target.TargetID, Azimuth: target.Azimuth, Elevation: target.Elevation, Priority: target.Priority, TargetClass: target.TargetClass, DetectedAt: target.DetectedAt, Metadata: target.Metadata}
}

func fromAutoTarget(target autocontrol.Target) targetfeed.Target {
	source, _ := target.Metadata["source"].(string)
	if source == "" {
		source = targetfeed.SourceExternalGenerator
	}
	return targetfeed.Target{Source: source, TargetID: target.TargetID, Azimuth: target.Azimuth, Elevation: target.Elevation, Priority: target.Priority, TargetClass: target.TargetClass, DetectedAt: target.DetectedAt, Metadata: target.Metadata}
}

func fromAssignment(row history.TargetAssignmentRecord) targetfeed.Target {
	target := targetfeed.Target{
		Source:     row.TargetSource,
		TargetID:   row.TargetID,
		DetectedAt: row.AssignedAt,
		Metadata:   map[string]interface{}{"source": row.TargetSource, "external_id": row.ExternalID},
	}
	if row.TargetPayload != nil {
		target.Source, _ = row.TargetPayload["source"].(string)
		if target.Source == "" {
			target.Source = row.TargetSource
		}
		target.Azimuth, _ = row.TargetPayload["azimuth"].(float64)
		target.Elevation, _ = row.TargetPayload["elevation"].(float64)
		if p, ok := row.TargetPayload["priority"].(float64); ok {
			target.Priority = int(p)
		}
		target.TargetClass, _ = row.TargetPayload["target_class"].(string)
		if detected, ok := row.TargetPayload["detected_at"].(time.Time); ok {
			target.DetectedAt = detected
		}
		if meta, ok := row.TargetPayload["metadata"].(map[string]interface{}); ok {
			target.Metadata = meta
		}
	}
	if target.Metadata == nil {
		target.Metadata = map[string]interface{}{"source": row.TargetSource, "external_id": row.ExternalID}
	}
	return target
}

func (m *Manager) resumeExecutor(ctx context.Context, row history.TargetAssignmentRecord) {
	inst, ok := m.registry.Get(row.GunID)
	if !ok || inst.Auto == nil {
		return
	}
	target := fromAssignment(row)
	_, err := inst.Auto.OfferAssignment(ctx, autocontrol.Assignment{
		ID:           row.ID,
		Generation:   row.Generation,
		GunID:        row.GunID,
		EngagementID: row.EngagementID,
		Target:       toAutoTarget(target),
	}, m)
	if err != nil {
		m.logger.Warn("dispatcher failed to resume assignment executor", "assignment_id", row.ID, "generation", row.Generation, "gun_id", row.GunID, "error", err)
	}
}

func commandEvidence(row history.TargetAssignmentRecord) bool {
	return row.CommandSequence != nil || row.CommandAuditID != nil || row.State == StateCommandCommitted
}

func (m *Manager) committedUnknownReason(ctx context.Context, row history.TargetAssignmentRecord) string {
	if !commandEvidence(row) {
		return ""
	}
	if row.State == StateUncertain && row.CompletionState == ReconciliationCommittedUnknown {
		if row.UncertaintyReason != "" {
			return row.UncertaintyReason
		}
		return "committed command result is unresolved"
	}
	reader, ok := m.store.(evidenceStore)
	if !ok {
		return ""
	}
	if row.CommandAuditID != nil {
		command, found, err := reader.GetGunCommand(ctx, *row.CommandAuditID)
		if err != nil {
			m.setError("command evidence read failed: " + err.Error())
			return ""
		}
		if found && command.CompletionTime != nil && (command.Result == "completion_timeout" || command.Result == "protocol_state") {
			if command.LifecycleEvidence.ShotObserved {
				return ""
			}
			if command.LifecycleEvidence.UncertaintyReason != "" {
				return command.LifecycleEvidence.UncertaintyReason
			}
			return fmt.Sprintf("post-commit lifecycle observation ended with %s without authoritative shot or no-fire evidence", command.Result)
		}
	}
	if row.EngagementID != nil {
		engagement, found, err := reader.GetGunEngagement(ctx, *row.EngagementID)
		if err != nil {
			m.setError("engagement evidence read failed: " + err.Error())
			return ""
		}
		if found && engagement.CompletedAt != nil && (engagement.Result == "completion_timeout" || engagement.Result == "protocol_state") {
			if engagement.Error != "" {
				return engagement.Error
			}
			return fmt.Sprintf("post-commit engagement ended with %s without authoritative shot or no-fire evidence", engagement.Result)
		}
	}
	return ""
}

func assignmentActive(state string) bool {
	switch state {
	case StateCompleted, StateReleased, StateRequeued, StateTerminalFailed, StateTerminalUncertain, "expired":
		return false
	default:
		return state != ""
	}
}

func safeToRequeueExpired(row history.TargetAssignmentRecord) bool {
	return row.State == StateAssigned && !commandEvidence(row)
}

func targetPayload(target targetfeed.Target) map[string]interface{} {
	return map[string]interface{}{"source": target.Source, "target_id": target.TargetID, "azimuth": target.Azimuth, "elevation": target.Elevation, "priority": target.Priority, "target_class": target.TargetClass, "detected_at": target.DetectedAt, "metadata": target.Metadata}
}

func externalID(target targetfeed.Target) string {
	if target.Metadata == nil {
		return ""
	}
	v, _ := target.Metadata["external_id"].(string)
	return v
}

func cloneDecisionPtr(in *history.DispatcherDecisionRecord) *history.DispatcherDecisionRecord {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneTimePtr(in *time.Time) *time.Time {
	if in == nil {
		return nil
	}
	v := in.UTC()
	return &v
}

func (m *Manager) String() string {
	return fmt.Sprintf("dispatcher mode=%s ready=%t", m.cfg.Mode, m.isReady())
}
