package history

import (
	"context"
	"errors"
	"sync"
	"time"

	"stonehenge-middleware/internal/model"
	"stonehenge-middleware/internal/plc"
)

type Sink interface {
	Name() string
	WriteEvent(ctx context.Context, event model.OperationalEvent) error
	Health() model.HistoryHealth
	Close() error
}

type Reader interface {
	ListFragments(ctx context.Context, filter FragmentFilter) ([]FragmentTrack, error)
	ListShots(ctx context.Context, filter ShotFilter) ([]ShotRecord, error)
	ListGunEvents(ctx context.Context, gunID int, filter EventFilter) ([]GunEvent, error)
	GetGunSummary(ctx context.Context, gunID int) (GunRuntimeSummary, bool, error)
	Ready(ctx context.Context) bool
}

type CommandAuditor interface {
	RecordGunCommand(ctx context.Context, audit plc.CommandAudit) (int64, error)
	UpdateGunCommandCompletion(ctx context.Context, auditID int64, completion plc.CommandAuditCompletion) error
	UpdateGunCommandLifecycleEvidence(ctx context.Context, auditID int64, evidence plc.CommandLifecycleEvidence) error
}

type CommandSequenceReader interface {
	LatestGunCommandSequence(ctx context.Context, gunID int) (uint32, bool, error)
}

type CommandAuditReader interface {
	GetGunCommand(ctx context.Context, id int64) (GunCommandRecord, bool, error)
}

type EngagementAuditor interface {
	RecordGunEngagement(ctx context.Context, record EngagementRecord) (int64, error)
	UpdateGunEngagement(ctx context.Context, id int64, update EngagementUpdate) error
}

type EngagementReader interface {
	GetGunEngagement(ctx context.Context, id int64) (GunEngagementRecord, bool, error)
}

type FaultResetAuditor interface {
	RecordGunFaultReset(ctx context.Context, record FaultResetRecord) (int64, error)
	UpdateGunFaultReset(ctx context.Context, id int64, update FaultResetUpdate) error
}

type DispatcherStore interface {
	RecordDispatcherDecision(ctx context.Context, decision DispatcherDecisionRecord) (int64, error)
	ListDispatcherDecisions(ctx context.Context, filter DispatcherDecisionFilter) ([]DispatcherDecisionRecord, error)
	GetDispatcherDecision(ctx context.Context, id int64) (DispatcherDecisionRecord, bool, error)
	CreateTargetAssignment(ctx context.Context, assignment TargetAssignmentRecord) (TargetAssignmentRecord, error)
	UpdateTargetAssignment(ctx context.Context, id int64, generation int, update TargetAssignmentUpdate) error
	RenewTargetAssignmentLease(ctx context.Context, id int64, generation int, owner string, expiresAt time.Time) (TargetAssignmentRecord, error)
	ListTargetAssignments(ctx context.Context, filter TargetAssignmentFilter) ([]TargetAssignmentRecord, error)
	GetTargetAssignment(ctx context.Context, id int64) (TargetAssignmentRecord, bool, error)
	ActiveAssignmentForTarget(ctx context.Context, targetID uint32, source string) (TargetAssignmentRecord, bool, error)
	ActiveAssignmentForGun(ctx context.Context, gunID int) (TargetAssignmentRecord, bool, error)
}

type DispatcherDecisionRecord struct {
	ID                int64                  `json:"id"`
	TargetID          uint32                 `json:"target_id"`
	TargetSource      string                 `json:"target_source"`
	ExternalID        string                 `json:"external_id,omitempty"`
	DecisionTime      time.Time              `json:"decision_time"`
	Mode              string                 `json:"mode"`
	Outcome           string                 `json:"outcome"`
	SelectedGunID     *int                   `json:"selected_gun_id,omitempty"`
	NoSelectionReason string                 `json:"no_selection_reason,omitempty"`
	EligibleGunCount  int                    `json:"eligible_gun_count"`
	ScoringVersion    string                 `json:"scoring_version"`
	Candidates        []DispatcherCandidate  `json:"candidates"`
	SelectedScore     *float64               `json:"selected_score,omitempty"`
	TieBreakReason    string                 `json:"tie_break_reason,omitempty"`
	ShadowOnly        bool                   `json:"shadow_only"`
	Details           map[string]interface{} `json:"details,omitempty"`
}

type DispatcherCandidate struct {
	GunID    int      `json:"gun_id"`
	Eligible bool     `json:"eligible"`
	Reasons  []string `json:"reasons,omitempty"`
	Score    *float64 `json:"score,omitempty"`
}

type DispatcherDecisionFilter struct {
	Limit         int
	Offset        int
	TargetID      *uint32
	SelectedGunID *int
	Mode          string
	Outcome       string
	Start         *time.Time
	End           *time.Time
}

type TargetAssignmentRecord struct {
	ID                    int64                  `json:"id"`
	TargetID              uint32                 `json:"target_id"`
	TargetSource          string                 `json:"target_source"`
	ExternalID            string                 `json:"external_id,omitempty"`
	GunID                 int                    `json:"gun_id"`
	State                 string                 `json:"state"`
	Generation            int                    `json:"generation"`
	AssignedAt            time.Time              `json:"assigned_at"`
	LeaseExpiresAt        time.Time              `json:"lease_expires_at"`
	LeaseOwner            string                 `json:"lease_owner,omitempty"`
	LastLeaseRenewedAt    *time.Time             `json:"last_lease_renewed_at,omitempty"`
	LeaseRenewalFailures  int                    `json:"lease_renewal_failures,omitempty"`
	LastLeaseRenewalError string                 `json:"last_lease_renewal_error,omitempty"`
	DispatcherDecisionID  int64                  `json:"dispatcher_decision_id"`
	CommandSequence       *uint32                `json:"command_sequence,omitempty"`
	CommandAuditID        *int64                 `json:"command_audit_id,omitempty"`
	EngagementID          *int64                 `json:"engagement_id,omitempty"`
	CompletionState       string                 `json:"completion_state,omitempty"`
	ReleaseReason         string                 `json:"release_reason,omitempty"`
	ReconciliationResult  string                 `json:"reconciliation_result,omitempty"`
	UncertaintyReason     string                 `json:"uncertainty_reason,omitempty"`
	UpdatedAt             time.Time              `json:"updated_at"`
	TargetPayload         map[string]interface{} `json:"target_payload,omitempty"`
}

type TargetAssignmentUpdate struct {
	State                 string
	ClearLease            bool
	LeaseExpiresAt        *time.Time
	LeaseOwner            *string
	LastLeaseRenewedAt    *time.Time
	LeaseRenewalFailures  *int
	LastLeaseRenewalError *string
	CommandSequence       *uint32
	CommandAuditID        *int64
	EngagementID          *int64
	CompletionState       string
	ReleaseReason         string
	ReconciliationResult  string
	UncertaintyReason     string
	UpdatedAt             time.Time
}

type TargetAssignmentFilter struct {
	Limit    int
	Offset   int
	TargetID *uint32
	GunID    *int
	State    string
	Active   *bool
}

type GunCommandRecord struct {
	ID                int64                    `json:"id"`
	CommandTime       time.Time                `json:"command_time"`
	GunID             int                      `json:"gun_id"`
	CommandSource     string                   `json:"command_source"`
	SubmittedSequence uint32                   `json:"submitted_sequence"`
	CommandPayload    map[string]interface{}   `json:"command_payload"`
	Result            string                   `json:"result"`
	ResultingState    uint16                   `json:"resulting_state"`
	FaultCode         uint16                   `json:"fault_code"`
	ShotSequence      *uint32                  `json:"shot_sequence,omitempty"`
	Error             string                   `json:"error,omitempty"`
	CompletionTime    *time.Time               `json:"completion_time,omitempty"`
	UpdatedAt         *time.Time               `json:"updated_at,omitempty"`
	LifecycleEvidence CommandLifecycleEvidence `json:"lifecycle_evidence,omitempty"`
}

type CommandLifecycleEvidence struct {
	BaselineAcceptedSequence     *uint32    `json:"baseline_accepted_sequence,omitempty"`
	ConfirmedAcceptedSequence    *uint32    `json:"confirmed_accepted_sequence,omitempty"`
	BaselineShotSequence         *uint32    `json:"baseline_shot_sequence,omitempty"`
	LastObservedShotSequence     *uint32    `json:"last_observed_shot_sequence,omitempty"`
	ShotObserved                 bool       `json:"shot_observed"`
	LastObservedState            *uint16    `json:"last_observed_state,omitempty"`
	FiringObserved               bool       `json:"firing_observed"`
	LockoutObserved              bool       `json:"lockout_observed"`
	ReadyAfterCommandObserved    bool       `json:"ready_after_command_observed"`
	CommandCommittedAt           *time.Time `json:"command_committed_at,omitempty"`
	LastLifecycleObservationTime *time.Time `json:"last_lifecycle_observation_time,omitempty"`
	UncertaintyReason            string     `json:"uncertainty_reason,omitempty"`
}

type GunEngagementRecord struct {
	ID                int64                  `json:"id"`
	TargetID          uint32                 `json:"target_id"`
	GunID             int                    `json:"gun_id"`
	CommandAuditID    *int64                 `json:"command_audit_id,omitempty"`
	SubmittedSequence *uint32                `json:"submitted_sequence,omitempty"`
	Source            string                 `json:"source"`
	TargetPayload     map[string]interface{} `json:"target_payload"`
	State             string                 `json:"state"`
	Result            string                 `json:"result,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	StartedAt         *time.Time             `json:"started_at,omitempty"`
	CompletedAt       *time.Time             `json:"completed_at,omitempty"`
	Error             string                 `json:"error,omitempty"`
	FinalShotSequence *uint32                `json:"final_shot_sequence,omitempty"`
	FaultCode         *uint16                `json:"fault_code,omitempty"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

type EngagementRecord struct {
	TargetID      uint32                 `json:"target_id"`
	GunID         int                    `json:"gun_id"`
	Source        string                 `json:"source"`
	TargetPayload map[string]interface{} `json:"target_payload"`
	State         string                 `json:"state"`
	Result        string                 `json:"result,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	Error         string                 `json:"error,omitempty"`
}

type EngagementUpdate struct {
	CommandAuditID    *int64     `json:"command_audit_id,omitempty"`
	SubmittedSequence *uint32    `json:"submitted_sequence,omitempty"`
	State             string     `json:"state"`
	Result            string     `json:"result,omitempty"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	Error             string     `json:"error,omitempty"`
	FinalShotSequence *uint32    `json:"final_shot_sequence,omitempty"`
	FaultCode         *uint16    `json:"fault_code,omitempty"`
}

type FaultResetRecord struct {
	GunID               int       `json:"gun_id"`
	Source              string    `json:"source"`
	PriorMode           string    `json:"prior_mode"`
	RequestedResumeAuto bool      `json:"requested_resume_auto"`
	FaultCodeBefore     uint16    `json:"fault_code_before"`
	StartedAt           time.Time `json:"started_at"`
	Error               string    `json:"error,omitempty"`
}

type FaultResetUpdate struct {
	CommandAuditID    *int64     `json:"command_audit_id,omitempty"`
	SubmittedSequence *uint32    `json:"submitted_sequence,omitempty"`
	Result            string     `json:"result"`
	FaultCodeAfter    *uint16    `json:"fault_code_after,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	Error             string     `json:"error,omitempty"`
}

type FragmentFilter struct {
	Limit  int
	Offset int
	Start  *time.Time
	End    *time.Time
	Status string
}

type ShotFilter struct {
	Limit  int
	Offset int
	Start  *time.Time
	End    *time.Time
}

type EventFilter struct {
	Limit     int
	Offset    int
	Start     *time.Time
	End       *time.Time
	EventType string
}

type FragmentTrack struct {
	ID            string               `json:"id"`
	ExternalID    string               `json:"external_id,omitempty"`
	SourceID      string               `json:"source_id,omitempty"`
	ReceivedAt    time.Time            `json:"received_at"`
	ObservedAt    time.Time            `json:"observed_at"`
	Bearing       float64              `json:"bearing"`
	Elevation     float64              `json:"elevation"`
	MapX          float64              `json:"map_x"`
	MapY          float64              `json:"map_y"`
	Priority      model.Priority       `json:"priority"`
	TerrainType   string               `json:"terrain_type"`
	StrategicZone string               `json:"strategic_zone"`
	ImpactAt      time.Time            `json:"impact_at"`
	Status        model.FragmentStatus `json:"status"`
	AssignedGunID *int                 `json:"assigned_gun_id,omitempty"`
	InterceptedAt *time.Time           `json:"intercepted_at,omitempty"`
	MissedAt      *time.Time           `json:"missed_at,omitempty"`
}

type GunEvent struct {
	ID                 string                 `json:"id"`
	EventTime          time.Time              `json:"event_time"`
	EventType          model.EventType        `json:"event_type"`
	Severity           model.Severity         `json:"severity"`
	GunID              *int                   `json:"gun_id,omitempty"`
	FragmentID         *string                `json:"fragment_id,omitempty"`
	ExternalFragmentID *string                `json:"external_fragment_id,omitempty"`
	Azimuth            *float64               `json:"azimuth,omitempty"`
	Elevation          *float64               `json:"elevation,omitempty"`
	Mode               *model.GunMode         `json:"mode,omitempty"`
	ShellType          *model.ShellType       `json:"shell_type,omitempty"`
	ShotSequence       *uint64                `json:"shot_sequence,omitempty"`
	Details            map[string]interface{} `json:"details,omitempty"`
}

type ShotRecord struct {
	ID                 string          `json:"id"`
	GunID              int             `json:"gun_id"`
	FragmentID         *string         `json:"fragment_id,omitempty"`
	ExternalFragmentID *string         `json:"external_fragment_id,omitempty"`
	FiredAt            time.Time       `json:"fired_at"`
	Azimuth            float64         `json:"azimuth"`
	Elevation          float64         `json:"elevation"`
	ShellType          model.ShellType `json:"shell_type"`
	ShotSequence       uint64          `json:"shot_sequence"`
	Result             string          `json:"result"`
}

type GunRuntimeSummary struct {
	GunID               int        `json:"gun_id"`
	TotalShots          uint64     `json:"total_shots"`
	StandardShellsFired uint64     `json:"standard_shells_fired"`
	SpecialShellsFired  uint64     `json:"special_shells_fired"`
	LastFiredAt         *time.Time `json:"last_fired_at,omitempty"`
	LastServiceAt       *time.Time `json:"last_service_at,omitempty"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type NoOpSink struct{}

func NewNoOpSink() *NoOpSink {
	return &NoOpSink{}
}

func (s *NoOpSink) Name() string {
	return "noop"
}

func (s *NoOpSink) WriteEvent(ctx context.Context, event model.OperationalEvent) error {
	return ctx.Err()
}

func (s *NoOpSink) Health() model.HistoryHealth {
	return model.HistoryHealth{Name: s.Name(), Adapter: s.Name(), Healthy: true, Connected: true}
}

func (s *NoOpSink) Close() error {
	return nil
}

type InMemorySink struct {
	mu               sync.RWMutex
	events           []model.OperationalEvent
	maxEvents        int
	lastWriteAt      *time.Time
	closed           bool
	nextDecisionID   int64
	nextAssignmentID int64
	decisions        []DispatcherDecisionRecord
	assignments      []TargetAssignmentRecord
}

func NewInMemorySink(maxEvents int) *InMemorySink {
	if maxEvents <= 0 {
		maxEvents = 500
	}
	return &InMemorySink{maxEvents: maxEvents}
}

func (s *InMemorySink) Name() string {
	return "memory"
}

func (s *InMemorySink) WriteEvent(ctx context.Context, event model.OperationalEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.events = append(s.events, event)
	if len(s.events) > s.maxEvents {
		s.events = s.events[len(s.events)-s.maxEvents:]
	}
	s.lastWriteAt = &now
	return nil
}

func (s *InMemorySink) Events() []model.OperationalEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]model.OperationalEvent(nil), s.events...)
	return out
}

func (s *InMemorySink) Health() model.HistoryHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return model.HistoryHealth{
		Name:        s.Name(),
		Adapter:     s.Name(),
		Healthy:     true,
		Connected:   true,
		EventCount:  len(s.events),
		LastWriteAt: s.lastWriteAt,
	}
}

func (s *InMemorySink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *InMemorySink) RecordDispatcherDecision(ctx context.Context, decision DispatcherDecisionRecord) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextDecisionID++
	decision.ID = s.nextDecisionID
	if decision.DecisionTime.IsZero() {
		decision.DecisionTime = time.Now().UTC()
	}
	s.decisions = append(s.decisions, cloneDecisionRecord(decision))
	return decision.ID, nil
}

func (s *InMemorySink) ListDispatcherDecisions(ctx context.Context, filter DispatcherDecisionFilter) ([]DispatcherDecisionRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []DispatcherDecisionRecord{}
	for i := len(s.decisions) - 1; i >= 0; i-- {
		row := s.decisions[i]
		if !matchDecision(row, filter) {
			continue
		}
		if filter.Offset > 0 {
			filter.Offset--
			continue
		}
		out = append(out, cloneDecisionRecord(row))
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (s *InMemorySink) GetDispatcherDecision(ctx context.Context, id int64) (DispatcherDecisionRecord, bool, error) {
	if err := ctx.Err(); err != nil {
		return DispatcherDecisionRecord{}, false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, row := range s.decisions {
		if row.ID == id {
			return cloneDecisionRecord(row), true, nil
		}
	}
	return DispatcherDecisionRecord{}, false, nil
}

func (s *InMemorySink) CreateTargetAssignment(ctx context.Context, assignment TargetAssignmentRecord) (TargetAssignmentRecord, error) {
	if err := ctx.Err(); err != nil {
		return TargetAssignmentRecord{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, row := range s.assignments {
		if row.TargetID == assignment.TargetID && row.TargetSource == assignment.TargetSource && assignmentActive(row.State) {
			return TargetAssignmentRecord{}, errors.New("target already has an active assignment")
		}
		if row.GunID == assignment.GunID && assignmentActive(row.State) {
			return TargetAssignmentRecord{}, errors.New("gun already has an active assignment")
		}
	}
	maxGeneration := 0
	for _, row := range s.assignments {
		if row.TargetID == assignment.TargetID && row.TargetSource == assignment.TargetSource && row.Generation > maxGeneration {
			maxGeneration = row.Generation
		}
	}
	s.nextAssignmentID++
	now := time.Now().UTC()
	assignment.ID = s.nextAssignmentID
	assignment.Generation = maxGeneration + 1
	if assignment.AssignedAt.IsZero() {
		assignment.AssignedAt = now
	}
	if assignment.UpdatedAt.IsZero() {
		assignment.UpdatedAt = now
	}
	s.assignments = append(s.assignments, cloneAssignmentRecord(assignment))
	return cloneAssignmentRecord(assignment), nil
}

func (s *InMemorySink) UpdateTargetAssignment(ctx context.Context, id int64, generation int, update TargetAssignmentUpdate) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.assignments {
		if s.assignments[i].ID != id || s.assignments[i].Generation != generation {
			continue
		}
		applyAssignmentUpdate(&s.assignments[i], update)
		return nil
	}
	return errors.New("target assignment not found")
}

func (s *InMemorySink) RenewTargetAssignmentLease(ctx context.Context, id int64, generation int, owner string, expiresAt time.Time) (TargetAssignmentRecord, error) {
	if err := ctx.Err(); err != nil {
		return TargetAssignmentRecord{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for i := range s.assignments {
		row := &s.assignments[i]
		if row.ID != id || row.Generation != generation {
			continue
		}
		if !assignmentActive(row.State) {
			return TargetAssignmentRecord{}, errors.New("target assignment is terminal")
		}
		if row.LeaseOwner != owner {
			row.LeaseRenewalFailures++
			row.LastLeaseRenewalError = "lease owner mismatch"
			row.UpdatedAt = now
			return TargetAssignmentRecord{}, errors.New("lease owner mismatch")
		}
		row.LeaseExpiresAt = expiresAt.UTC()
		row.LastLeaseRenewedAt = &now
		row.LastLeaseRenewalError = ""
		row.UpdatedAt = now
		return cloneAssignmentRecord(*row), nil
	}
	return TargetAssignmentRecord{}, errors.New("target assignment not found")
}

func (s *InMemorySink) ListTargetAssignments(ctx context.Context, filter TargetAssignmentFilter) ([]TargetAssignmentRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []TargetAssignmentRecord{}
	for i := len(s.assignments) - 1; i >= 0; i-- {
		row := s.assignments[i]
		if !matchAssignment(row, filter) {
			continue
		}
		if filter.Offset > 0 {
			filter.Offset--
			continue
		}
		out = append(out, cloneAssignmentRecord(row))
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (s *InMemorySink) GetTargetAssignment(ctx context.Context, id int64) (TargetAssignmentRecord, bool, error) {
	if err := ctx.Err(); err != nil {
		return TargetAssignmentRecord{}, false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, row := range s.assignments {
		if row.ID == id {
			return cloneAssignmentRecord(row), true, nil
		}
	}
	return TargetAssignmentRecord{}, false, nil
}

func (s *InMemorySink) ActiveAssignmentForTarget(ctx context.Context, targetID uint32, source string) (TargetAssignmentRecord, bool, error) {
	if err := ctx.Err(); err != nil {
		return TargetAssignmentRecord{}, false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, row := range s.assignments {
		if row.TargetID == targetID && row.TargetSource == source && assignmentActive(row.State) {
			return cloneAssignmentRecord(row), true, nil
		}
	}
	return TargetAssignmentRecord{}, false, nil
}

func (s *InMemorySink) ActiveAssignmentForGun(ctx context.Context, gunID int) (TargetAssignmentRecord, bool, error) {
	if err := ctx.Err(); err != nil {
		return TargetAssignmentRecord{}, false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, row := range s.assignments {
		if row.GunID == gunID && assignmentActive(row.State) {
			return cloneAssignmentRecord(row), true, nil
		}
	}
	return TargetAssignmentRecord{}, false, nil
}

func assignmentActive(state string) bool {
	switch state {
	case "completed", "released", "requeued", "terminal_failed", "terminal_uncertain", "expired":
		return false
	default:
		return state != ""
	}
}

func applyAssignmentUpdate(row *TargetAssignmentRecord, update TargetAssignmentUpdate) {
	if update.State != "" {
		row.State = update.State
	}
	if update.ClearLease {
		row.LeaseOwner = ""
		row.LastLeaseRenewedAt = nil
	}
	if update.LeaseExpiresAt != nil {
		row.LeaseExpiresAt = update.LeaseExpiresAt.UTC()
	}
	if update.LeaseOwner != nil {
		row.LeaseOwner = *update.LeaseOwner
	}
	if update.LastLeaseRenewedAt != nil {
		v := update.LastLeaseRenewedAt.UTC()
		row.LastLeaseRenewedAt = &v
	}
	if update.LeaseRenewalFailures != nil {
		row.LeaseRenewalFailures = *update.LeaseRenewalFailures
	}
	if update.LastLeaseRenewalError != nil {
		row.LastLeaseRenewalError = *update.LastLeaseRenewalError
	}
	if update.CommandSequence != nil {
		row.CommandSequence = cloneUint32Ptr(update.CommandSequence)
	}
	if update.CommandAuditID != nil {
		v := *update.CommandAuditID
		row.CommandAuditID = &v
	}
	if update.EngagementID != nil {
		v := *update.EngagementID
		row.EngagementID = &v
	}
	if update.CompletionState != "" {
		row.CompletionState = update.CompletionState
	}
	if update.ReleaseReason != "" {
		row.ReleaseReason = update.ReleaseReason
	}
	if update.ReconciliationResult != "" {
		row.ReconciliationResult = update.ReconciliationResult
	}
	if update.UncertaintyReason != "" {
		row.UncertaintyReason = update.UncertaintyReason
	}
	if update.UpdatedAt.IsZero() {
		row.UpdatedAt = time.Now().UTC()
	} else {
		row.UpdatedAt = update.UpdatedAt.UTC()
	}
}

func matchDecision(row DispatcherDecisionRecord, filter DispatcherDecisionFilter) bool {
	if filter.TargetID != nil && row.TargetID != *filter.TargetID {
		return false
	}
	if filter.SelectedGunID != nil {
		if row.SelectedGunID == nil || *row.SelectedGunID != *filter.SelectedGunID {
			return false
		}
	}
	if filter.Mode != "" && row.Mode != filter.Mode {
		return false
	}
	if filter.Outcome != "" && row.Outcome != filter.Outcome {
		return false
	}
	if filter.Start != nil && row.DecisionTime.Before(*filter.Start) {
		return false
	}
	if filter.End != nil && row.DecisionTime.After(*filter.End) {
		return false
	}
	return true
}

func matchAssignment(row TargetAssignmentRecord, filter TargetAssignmentFilter) bool {
	if filter.TargetID != nil && row.TargetID != *filter.TargetID {
		return false
	}
	if filter.GunID != nil && row.GunID != *filter.GunID {
		return false
	}
	if filter.State != "" && row.State != filter.State {
		return false
	}
	if filter.Active != nil && assignmentActive(row.State) != *filter.Active {
		return false
	}
	return true
}

func cloneDecisionRecord(row DispatcherDecisionRecord) DispatcherDecisionRecord {
	if row.SelectedGunID != nil {
		v := *row.SelectedGunID
		row.SelectedGunID = &v
	}
	if row.SelectedScore != nil {
		v := *row.SelectedScore
		row.SelectedScore = &v
	}
	row.Candidates = append([]DispatcherCandidate(nil), row.Candidates...)
	for i := range row.Candidates {
		row.Candidates[i].Reasons = append([]string(nil), row.Candidates[i].Reasons...)
		if row.Candidates[i].Score != nil {
			v := *row.Candidates[i].Score
			row.Candidates[i].Score = &v
		}
	}
	row.Details = cloneAnyMap(row.Details)
	return row
}

func cloneAssignmentRecord(row TargetAssignmentRecord) TargetAssignmentRecord {
	row.CommandSequence = cloneUint32Ptr(row.CommandSequence)
	if row.LastLeaseRenewedAt != nil {
		v := row.LastLeaseRenewedAt.UTC()
		row.LastLeaseRenewedAt = &v
	}
	if row.CommandAuditID != nil {
		v := *row.CommandAuditID
		row.CommandAuditID = &v
	}
	if row.EngagementID != nil {
		v := *row.EngagementID
		row.EngagementID = &v
	}
	row.TargetPayload = cloneAnyMap(row.TargetPayload)
	return row
}

func cloneUint32Ptr(in *uint32) *uint32 {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

func cloneAnyMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
