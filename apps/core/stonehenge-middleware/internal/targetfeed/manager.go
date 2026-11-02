package targetfeed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"stonehenge-middleware/internal/autocontrol"
)

const (
	SourceExternalGenerator = "external_target_generator"
	SourceManualAPI         = "manual_api"
	maxRetainedRecords      = 500
	maxRetainedTargetBytes  = 16 << 10
)

type State string

const (
	StatePending   State = "pending"
	StateSubmitted State = "submitted"
	StateCompleted State = "completed"
	StateRejected  State = "rejected"
	StateExpired   State = "expired"
)

const (
	ErrorCodeNone uint16 = iota
	ErrorCodeValidation
	ErrorCodeDuplicate
	ErrorCodeQueueFull
	ErrorCodeStale
	ErrorCodeAutoUnavailable
	ErrorCodeAutoSuspended
)

const ErrorCodeOther uint16 = 255

var (
	ErrValidation = errors.New("target feed validation failed")
	ErrQueueFull  = errors.New("target feed pending queue is full")
	ErrStale      = errors.New("target feed target is stale")
)

type Target struct {
	Source      string                 `json:"source"`
	TargetID    uint32                 `json:"target_id"`
	Azimuth     float64                `json:"azimuth"`
	Elevation   float64                `json:"elevation"`
	Priority    int                    `json:"priority"`
	TargetClass string                 `json:"target_class"`
	DetectedAt  time.Time              `json:"detected_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type IngestResult struct {
	Target    Target `json:"target"`
	State     State  `json:"state"`
	Duplicate bool   `json:"duplicate"`
	Updated   bool   `json:"updated"`
	Message   string `json:"message,omitempty"`
}

type Record struct {
	Target      Target     `json:"target"`
	State       State      `json:"state"`
	ReceivedAt  time.Time  `json:"received_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
}

type Status struct {
	Source         string     `json:"source"`
	Configured     bool       `json:"configured"`
	Healthy        bool       `json:"healthy"`
	Receiving      bool       `json:"receiving"`
	LastReceivedAt *time.Time `json:"last_received_at,omitempty"`
	LastAcceptedAt *time.Time `json:"last_accepted_at,omitempty"`
	LastRejectedAt *time.Time `json:"last_rejected_at,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
	ReceivedCount  uint32     `json:"received_count"`
	AcceptedCount  uint32     `json:"accepted_count"`
	DuplicateCount uint32     `json:"duplicate_count"`
	RejectedCount  uint32     `json:"rejected_count"`
	PendingCount   uint32     `json:"pending_count"`
	ExpiredCount   uint32     `json:"expired_count"`
	SubmittedCount uint32     `json:"submitted_count"`
	CompletedCount uint32     `json:"completed_count"`
	ActiveTargetID *uint32    `json:"active_target_id,omitempty"`
	LastErrorCode  uint16     `json:"last_error_code"`
}

type Config struct {
	Enabled       bool
	MaxPending    int
	TargetTTL     time.Duration
	HealthyWindow time.Duration
	PollInterval  time.Duration
}

type AutoController interface {
	SubmitTarget(ctx context.Context, target autocontrol.Target) (autocontrol.Status, error)
	Status(ctx context.Context) autocontrol.Status
}

type Manager struct {
	cfg    Config
	auto   AutoController
	logger *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	wake   chan struct{}

	mu       sync.Mutex
	records  map[string]*Record
	order    []string
	inFlight string
	status   Status
}

func New(cfg Config, auto AutoController, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.MaxPending <= 0 {
		cfg.MaxPending = 25
	}
	if cfg.MaxPending > maxRetainedRecords {
		cfg.MaxPending = maxRetainedRecords
	}
	if cfg.TargetTTL <= 0 {
		cfg.TargetTTL = 2 * time.Minute
	}
	if cfg.HealthyWindow <= 0 {
		cfg.HealthyWindow = 30 * time.Second
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 500 * time.Millisecond
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		cfg:     cfg,
		auto:    auto,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
		wake:    make(chan struct{}, 1),
		records: make(map[string]*Record),
		status:  Status{Source: SourceExternalGenerator, Configured: cfg.Enabled},
	}
}

func (m *Manager) Start() {
	if m == nil || !m.cfg.Enabled {
		return
	}
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

func (m *Manager) Ingest(ctx context.Context, target Target) (IngestResult, error) {
	if m == nil || !m.cfg.Enabled {
		return IngestResult{}, fmt.Errorf("%w: target feed manager is disabled", ErrValidation)
	}
	now := time.Now().UTC()
	normalized, err := Normalize(target, now)
	if err != nil {
		m.recordReject(now, err.Error(), ErrorCodeValidation)
		return IngestResult{Target: normalized, State: StateRejected, Message: err.Error()}, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	if now.Sub(normalized.DetectedAt) > m.cfg.TargetTTL {
		m.recordReject(now, "target is stale", ErrorCodeStale)
		return IngestResult{Target: normalized, State: StateExpired, Message: "target is stale"}, ErrStale
	}
	key := keyFor(normalized)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.ReceivedCount++
	m.status.LastReceivedAt = timePtr(now)
	m.status.Receiving = true
	if existing, ok := m.records[key]; ok {
		if sameTarget(existing.Target, normalized) {
			m.status.DuplicateCount++
			m.status.LastAcceptedAt = timePtr(now)
			return IngestResult{Target: cloneTarget(existing.Target), State: existing.State, Duplicate: true, Message: "duplicate target acknowledged"}, nil
		}
		if existing.State == StatePending {
			existing.Target = normalized
			existing.UpdatedAt = now
			m.status.AcceptedCount++
			m.status.LastAcceptedAt = timePtr(now)
			m.signal()
			return IngestResult{Target: cloneTarget(normalized), State: StatePending, Updated: true, Message: "pending target updated"}, nil
		}
		m.status.DuplicateCount++
		m.status.LastAcceptedAt = timePtr(now)
		return IngestResult{Target: cloneTarget(existing.Target), State: existing.State, Duplicate: true, Message: "target already processed"}, nil
	}
	if m.pendingCountLocked() >= m.cfg.MaxPending {
		m.status.RejectedCount++
		m.status.LastRejectedAt = timePtr(now)
		m.status.LastError = "pending target queue is full"
		m.status.LastErrorCode = ErrorCodeQueueFull
		return IngestResult{Target: normalized, State: StateRejected, Message: m.status.LastError}, ErrQueueFull
	}
	if !m.makeRetentionRoomLocked() {
		m.status.RejectedCount++
		m.status.LastRejectedAt = timePtr(now)
		m.status.LastError = "target feed retention is full"
		m.status.LastErrorCode = ErrorCodeQueueFull
		return IngestResult{Target: normalized, State: StateRejected, Message: m.status.LastError}, ErrQueueFull
	}
	record := &Record{Target: normalized, State: StatePending, ReceivedAt: now, UpdatedAt: now}
	m.records[key] = record
	m.order = append(m.order, key)
	m.status.AcceptedCount++
	m.status.PendingCount = uint32(m.pendingCountLocked())
	m.status.LastAcceptedAt = timePtr(now)
	m.logger.Info("external target queued", "source", normalized.Source, "target_id", normalized.TargetID, "priority", normalized.Priority)
	m.signal()
	return IngestResult{Target: cloneTarget(normalized), State: StatePending, Message: "target queued"}, nil
}

func (m *Manager) Status() Status {
	if m == nil {
		return Status{Source: SourceExternalGenerator, Configured: false}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked(time.Now().UTC())
}

func (m *Manager) Config() Config {
	if m == nil {
		return Config{}
	}
	return m.cfg
}

func (m *Manager) Targets() []Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Record, 0, len(m.order))
	for _, key := range m.order {
		if rec, ok := m.records[key]; ok {
			out = append(out, cloneRecord(*rec))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].State != out[j].State {
			return out[i].State < out[j].State
		}
		return out[i].Target.DetectedAt.Before(out[j].Target.DetectedAt)
	})
	return out
}

func (m *Manager) PendingTargets(now time.Time) []Record {
	if m == nil {
		return nil
	}
	m.expireStale(now.UTC())
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Record, 0, len(m.order))
	for _, key := range m.order {
		rec := m.records[key]
		if rec == nil || rec.State != StatePending {
			continue
		}
		out = append(out, cloneRecord(*rec))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Target.Priority != out[j].Target.Priority {
			return out[i].Target.Priority > out[j].Target.Priority
		}
		if !out[i].Target.DetectedAt.Equal(out[j].Target.DetectedAt) {
			return out[i].Target.DetectedAt.Before(out[j].Target.DetectedAt)
		}
		if out[i].Target.Source != out[j].Target.Source {
			return out[i].Target.Source < out[j].Target.Source
		}
		return out[i].Target.TargetID < out[j].Target.TargetID
	})
	return out
}

func (m *Manager) MarkSubmitted(target Target, at time.Time) bool {
	if m == nil {
		return false
	}
	key := keyFor(target)
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := m.records[key]
	if rec == nil || rec.State != StatePending {
		return false
	}
	at = at.UTC()
	rec.State = StateSubmitted
	rec.SubmittedAt = timePtr(at)
	rec.UpdatedAt = at
	m.inFlight = key
	m.status.SubmittedCount++
	m.status.PendingCount = uint32(m.pendingCountLocked())
	return true
}

func (m *Manager) Requeue(target Target, reason string, at time.Time) bool {
	if m == nil {
		return false
	}
	key := keyFor(target)
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := m.records[key]
	if rec == nil || rec.State != StateSubmitted {
		return false
	}
	at = at.UTC()
	rec.State = StatePending
	rec.UpdatedAt = at
	rec.Error = reason
	if m.inFlight == key {
		m.inFlight = ""
	}
	m.status.PendingCount = uint32(m.pendingCountLocked())
	m.signal()
	return true
}

func (m *Manager) MarkCompleted(target Target, result, message string, at time.Time) bool {
	if m == nil {
		return false
	}
	key := keyFor(target)
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := m.records[key]
	if rec == nil || rec.State == StateCompleted {
		return false
	}
	at = at.UTC()
	rec.State = StateCompleted
	rec.CompletedAt = timePtr(at)
	rec.UpdatedAt = at
	rec.Result = result
	rec.Error = message
	if m.inFlight == key {
		m.inFlight = ""
	}
	m.status.CompletedCount++
	m.status.PendingCount = uint32(m.pendingCountLocked())
	return true
}

func (m *Manager) loop() {
	ticker := time.NewTicker(m.cfg.PollInterval)
	defer ticker.Stop()
	for {
		m.expireStale(time.Now().UTC())
		m.dispatch(m.ctx)
		select {
		case <-m.ctx.Done():
			return
		case <-m.wake:
		case <-ticker.C:
		}
	}
}

func (m *Manager) dispatch(ctx context.Context) {
	if m.auto == nil {
		return
	}
	autoStatus := m.auto.Status(ctx)
	m.mu.Lock()
	if m.inFlight != "" {
		rec := m.records[m.inFlight]
		if rec != nil && autoStatus.ActiveTarget == nil && autoStatus.QueuedTarget == nil {
			now := time.Now().UTC()
			rec.State = StateCompleted
			rec.CompletedAt = timePtr(now)
			rec.Result = autoStatus.LastResult
			rec.Error = autoStatus.LastError
			m.status.CompletedCount++
			m.inFlight = ""
		}
	}
	if autoStatus.Enabled && autoStatus.ActiveTarget == nil && autoStatus.QueuedTarget == nil && !autoSuspended(autoStatus.State) {
		key, target, ok := m.nextPendingLocked()
		if ok {
			m.inFlight = key
			m.mu.Unlock()
			status, err := m.auto.SubmitTarget(ctx, toAutoTarget(target))
			m.mu.Lock()
			rec := m.records[key]
			if err != nil {
				m.inFlight = ""
				m.status.LastError = err.Error()
				m.status.LastErrorCode = ErrorCodeAutoUnavailable
				if autoSuspended(status.State) {
					m.status.LastErrorCode = ErrorCodeAutoSuspended
				}
				m.mu.Unlock()
				return
			}
			now := time.Now().UTC()
			if rec != nil {
				rec.State = StateSubmitted
				rec.SubmittedAt = timePtr(now)
				rec.UpdatedAt = now
			}
			m.status.SubmittedCount++
			m.logger.Info("external target submitted to auto controller", "source", target.Source, "target_id", target.TargetID)
		}
	}
	m.status.PendingCount = uint32(m.pendingCountLocked())
	m.mu.Unlock()
}

func (m *Manager) expireStale(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range m.order {
		rec := m.records[key]
		if rec == nil || rec.State != StatePending {
			continue
		}
		if now.Sub(rec.Target.DetectedAt) > m.cfg.TargetTTL {
			rec.State = StateExpired
			rec.CompletedAt = timePtr(now)
			rec.Error = "target expired before AUTO submission"
			m.status.ExpiredCount++
			m.status.LastRejectedAt = timePtr(now)
			m.status.LastError = rec.Error
			m.status.LastErrorCode = ErrorCodeStale
		}
	}
	m.status.PendingCount = uint32(m.pendingCountLocked())
}

func (m *Manager) recordReject(now time.Time, message string, code uint16) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.ReceivedCount++
	m.status.RejectedCount++
	m.status.LastReceivedAt = timePtr(now)
	m.status.LastRejectedAt = timePtr(now)
	m.status.LastError = message
	m.status.LastErrorCode = code
}

func (m *Manager) statusLocked(now time.Time) Status {
	status := m.status
	status.Configured = m.cfg.Enabled
	status.PendingCount = uint32(m.pendingCountLocked())
	status.ActiveTargetID = nil
	if m.inFlight != "" {
		if rec := m.records[m.inFlight]; rec != nil {
			status.ActiveTargetID = uint32Ptr(rec.Target.TargetID)
		}
	}
	status.Healthy = false
	if status.LastAcceptedAt != nil && now.Sub(*status.LastAcceptedAt) <= m.cfg.HealthyWindow {
		status.Healthy = true
	}
	status.Receiving = false
	if status.LastReceivedAt != nil && now.Sub(*status.LastReceivedAt) <= m.cfg.HealthyWindow {
		status.Receiving = true
	}
	return status
}

func (m *Manager) pendingCountLocked() int {
	count := 0
	for _, rec := range m.records {
		if rec.State == StatePending {
			count++
		}
	}
	return count
}

func (m *Manager) makeRetentionRoomLocked() bool {
	if len(m.records) < maxRetainedRecords {
		return true
	}
	for i, key := range m.order {
		rec := m.records[key]
		if rec != nil && (rec.State == StatePending || rec.State == StateSubmitted || key == m.inFlight) {
			continue
		}
		delete(m.records, key)
		m.order = append(m.order[:i], m.order[i+1:]...)
		return true
	}
	return false
}

func (m *Manager) nextPendingLocked() (string, Target, bool) {
	var bestKey string
	var best *Record
	for _, key := range m.order {
		rec := m.records[key]
		if rec == nil || rec.State != StatePending {
			continue
		}
		if best == nil || rec.Target.Priority > best.Target.Priority || (rec.Target.Priority == best.Target.Priority && rec.Target.DetectedAt.Before(best.Target.DetectedAt)) {
			bestKey = key
			best = rec
		}
	}
	if best == nil {
		return "", Target{}, false
	}
	return bestKey, cloneTarget(best.Target), true
}

func (m *Manager) signal() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func Normalize(target Target, now time.Time) (Target, error) {
	if target.Source == "" {
		target.Source = SourceManualAPI
	}
	if target.DetectedAt.IsZero() {
		target.DetectedAt = now
	} else {
		target.DetectedAt = target.DetectedAt.UTC()
	}
	autoTarget := toAutoTarget(target)
	if err := autocontrol.ValidateTarget(autoTarget); err != nil {
		return target, err
	}
	encoded, err := json.Marshal(target)
	if err != nil {
		return target, fmt.Errorf("target metadata is not valid JSON: %w", err)
	}
	if len(encoded) > maxRetainedTargetBytes {
		return target, fmt.Errorf("target metadata exceeds %d bytes", maxRetainedTargetBytes)
	}
	return target, nil
}

func toAutoTarget(target Target) autocontrol.Target {
	return autocontrol.Target{
		TargetID:    target.TargetID,
		Azimuth:     target.Azimuth,
		Elevation:   target.Elevation,
		Priority:    target.Priority,
		TargetClass: target.TargetClass,
		DetectedAt:  target.DetectedAt,
		Metadata:    cloneMap(target.Metadata),
	}
}

func autoSuspended(state autocontrol.State) bool {
	return state == autocontrol.StateSuspendedFault || state == autocontrol.StateSuspendedUnavailable
}

func keyFor(target Target) string {
	return fmt.Sprintf("%s:%d", target.Source, target.TargetID)
}

func sameTarget(a, b Target) bool {
	return a.TargetID == b.TargetID &&
		a.Source == b.Source &&
		a.Azimuth == b.Azimuth &&
		a.Elevation == b.Elevation &&
		a.Priority == b.Priority &&
		a.TargetClass == b.TargetClass &&
		a.DetectedAt.Equal(b.DetectedAt)
}

func cloneRecord(record Record) Record {
	record.Target = cloneTarget(record.Target)
	record.ReceivedAt = record.ReceivedAt.UTC()
	record.UpdatedAt = record.UpdatedAt.UTC()
	if record.SubmittedAt != nil {
		record.SubmittedAt = timePtr(record.SubmittedAt.UTC())
	}
	if record.CompletedAt != nil {
		record.CompletedAt = timePtr(record.CompletedAt.UTC())
	}
	return record
}

func cloneTarget(target Target) Target {
	target.Metadata = cloneMap(target.Metadata)
	return target
}

func cloneMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func timePtr(value time.Time) *time.Time {
	value = value.UTC()
	return &value
}

func uint32Ptr(value uint32) *uint32 {
	return &value
}
