package plc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/goburrow/modbus"

	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/testcontrol"
)

const Gun1ID = 1

const (
	CommandSourceManualAPI          = "manual_api"
	CommandSourceAutoController     = "auto_controller"
	CommandSourceOperatorFaultReset = "operator_fault_reset"
)

const (
	gun1CommandBase uint16 = 1024
	gun1StatusBase  uint16 = 20
)

const (
	RegisterCommandSequenceLow uint16 = iota
	RegisterCommandSequenceHigh
	RegisterMode
	RegisterTargetAzimuth
	RegisterTargetElevation
	RegisterFragmentLow
	RegisterFragmentHigh
	RegisterFireRequest
	RegisterMaintenance
	RegisterAdminOverride
	RegisterFaultReset
	RegisterSafetyFirePermissive
	RegisterSafetyExtendedPermissive
)

const (
	RegisterAcceptedSequenceLow uint16 = gun1StatusBase + iota
	RegisterAcceptedSequenceHigh
	RegisterState
	RegisterCurrentAzimuth
	RegisterCurrentElevation
	RegisterActiveFragmentLow
	RegisterActiveFragmentHigh
	RegisterTargetAccepted
	RegisterOnTarget
	RegisterSettling
	RegisterFiring
	RegisterLockout
	RegisterReady
	RegisterFaultActive
	RegisterFaultCode
	RegisterShotSequenceLow
	RegisterShotSequenceHigh
	RegisterSettlingSeconds
	RegisterLockoutSeconds
)

const (
	statusStartRegister uint16 = gun1StatusBase
	statusRegisterCount uint16 = RegisterLockoutSeconds - RegisterAcceptedSequenceLow + 1
)

type CommandMode uint16

const (
	ModeAuto CommandMode = iota
	ModeManual
	ModeMaintenance
)

type GunCommand struct {
	Mode                     CommandMode `json:"mode"`
	TargetAzimuth            float64     `json:"target_azimuth"`
	TargetElevation          float64     `json:"target_elevation"`
	FragmentID               *uint32     `json:"fragment_id,omitempty"`
	FragmentLow              *uint16     `json:"fragment_low,omitempty"`
	FragmentHigh             *uint16     `json:"fragment_high,omitempty"`
	FireRequest              bool        `json:"fire_request"`
	Maintenance              bool        `json:"maintenance"`
	AdminOverride            bool        `json:"admin_override"`
	FaultReset               bool        `json:"fault_reset"`
	SafetyFirePermissive     bool        `json:"safety_fire_permissive"`
	SafetyExtendedPermissive bool        `json:"safety_extended_permissive"`
}

type GunStatus struct {
	RawRegisters           []uint16 `json:"raw_registers,omitempty"`
	AcceptedSequence       uint32   `json:"accepted_sequence"`
	AcceptedSequenceLow    uint16   `json:"accepted_sequence_low"`
	AcceptedSequenceHigh   uint16   `json:"accepted_sequence_high"`
	State                  uint16   `json:"state"`
	StateName              string   `json:"state_name"`
	CurrentAzimuthTenths   int16    `json:"current_azimuth_tenths"`
	CurrentElevationTenths int16    `json:"current_elevation_tenths"`
	CurrentAzimuth         float64  `json:"current_azimuth"`
	CurrentElevation       float64  `json:"current_elevation"`
	ActiveFragment         uint32   `json:"active_fragment"`
	TargetAccepted         bool     `json:"target_accepted"`
	OnTarget               bool     `json:"on_target"`
	Settling               bool     `json:"settling"`
	Firing                 bool     `json:"firing"`
	Lockout                bool     `json:"lockout"`
	Ready                  bool     `json:"ready"`
	FaultActive            bool     `json:"fault_active"`
	FaultCode              uint16   `json:"fault_code"`
	FaultName              string   `json:"fault_name"`
	ShotSequence           uint32   `json:"shot_sequence"`
	SettlingSeconds        uint16   `json:"settling_seconds"`
	LockoutSeconds         uint16   `json:"lockout_seconds"`
}

type CommandResult struct {
	SubmittedSequence uint32     `json:"submitted_sequence"`
	AcceptedSequence  uint32     `json:"accepted_sequence"`
	Accepted          bool       `json:"accepted"`
	CurrentState      uint16     `json:"current_state"`
	StateName         string     `json:"state_name"`
	FaultActive       bool       `json:"fault_active"`
	FaultCode         uint16     `json:"fault_code"`
	FaultName         string     `json:"fault_name"`
	ShotSequence      uint32     `json:"shot_sequence"`
	TargetAccepted    bool       `json:"target_accepted"`
	OnTarget          bool       `json:"on_target"`
	Settling          bool       `json:"settling"`
	Firing            bool       `json:"firing"`
	Lockout           bool       `json:"lockout"`
	Ready             bool       `json:"ready"`
	TimedOut          bool       `json:"timed_out"`
	ErrorType         string     `json:"error_type,omitempty"`
	Error             string     `json:"error,omitempty"`
	Status            *GunStatus `json:"status,omitempty"`
	BaselineStatus    *GunStatus `json:"-"`
}

type CommandAudit struct {
	Timestamp         time.Time              `json:"timestamp"`
	GunID             int                    `json:"gun_id"`
	Source            string                 `json:"source"`
	SubmittedSequence uint32                 `json:"submitted_sequence"`
	Payload           map[string]interface{} `json:"payload"`
	Result            string                 `json:"result"`
	ResultingState    uint16                 `json:"resulting_state"`
	FaultCode         uint16                 `json:"fault_code"`
	ShotSequence      uint32                 `json:"shot_sequence"`
	Error             string                 `json:"error,omitempty"`
}

type CommandAuditCompletion struct {
	CompletionTime time.Time `json:"completion_time"`
	Result         string    `json:"result"`
	ResultingState uint16    `json:"resulting_state"`
	FaultCode      uint16    `json:"fault_code"`
	ShotSequence   *uint32   `json:"shot_sequence,omitempty"`
	Error          string    `json:"error,omitempty"`
}

type CommandLifecycleEvidence struct {
	BaselineAcceptedSequence     *uint32   `json:"baseline_accepted_sequence,omitempty"`
	ConfirmedAcceptedSequence    *uint32   `json:"confirmed_accepted_sequence,omitempty"`
	BaselineShotSequence         *uint32   `json:"baseline_shot_sequence,omitempty"`
	LastObservedShotSequence     *uint32   `json:"last_observed_shot_sequence,omitempty"`
	ShotObserved                 *bool     `json:"shot_observed,omitempty"`
	LastObservedState            *uint16   `json:"last_observed_state,omitempty"`
	FiringObserved               *bool     `json:"firing_observed,omitempty"`
	LockoutObserved              *bool     `json:"lockout_observed,omitempty"`
	ReadyAfterCommandObserved    *bool     `json:"ready_after_command_observed,omitempty"`
	CommandCommittedAt           time.Time `json:"command_committed_at,omitempty"`
	LastLifecycleObservationTime time.Time `json:"last_lifecycle_observation_time,omitempty"`
	UncertaintyReason            string    `json:"uncertainty_reason,omitempty"`
}

type CommandLifecycleEvent struct {
	AuditID           int64                  `json:"audit_id"`
	GunID             int                    `json:"gun_id"`
	SubmittedSequence uint32                 `json:"submitted_sequence"`
	Completion        CommandAuditCompletion `json:"completion"`
	UpdateError       string                 `json:"update_error,omitempty"`
}

type StatusReader interface {
	ReadStatus(ctx context.Context) (GunStatus, error)
}

type CommandAuditUpdater interface {
	UpdateGunCommandCompletion(ctx context.Context, auditID int64, completion CommandAuditCompletion) error
}

type CommandLifecycleEvidenceUpdater interface {
	UpdateGunCommandLifecycleEvidence(ctx context.Context, auditID int64, evidence CommandLifecycleEvidence) error
}

type CommandLifecycleTracker struct {
	cfg     config.PLCConfig
	reader  StatusReader
	auditor CommandAuditUpdater
	logger  *slog.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

type CommandLifecycleRequest struct {
	AuditID           int64
	GunID             int
	Command           GunCommand
	SubmittedSequence uint32
	BaselineStatus    GunStatus
	AckStatus         GunStatus
	Source            string
	Timeout           time.Duration
	Completion        chan<- CommandLifecycleEvent
}

type RegisterTransport interface {
	ReadHoldingRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error)
	WriteMultipleRegisters(ctx context.Context, address uint16, values []uint16) error
	WriteSingleRegister(ctx context.Context, address, value uint16) error
}

type GunCommandService struct {
	cfg                   config.PLCConfig
	transport             RegisterTransport
	logger                *slog.Logger
	gunID                 int
	commandBase           uint16
	statusBase            uint16
	mu                    sync.Mutex
	lastAllocatedSequence uint32
	hasLastAllocated      bool
}

func NewGunCommandService(cfg config.PLCConfig, logger *slog.Logger) *GunCommandService {
	if logger == nil {
		logger = slog.Default()
	}
	return NewGunCommandServiceWithTransport(cfg, NewModbusTCPTransport(cfg), logger)
}

func NewGunCommandServiceWithTransport(cfg config.PLCConfig, transport RegisterTransport, logger *slog.Logger) *GunCommandService {
	return NewGunCommandServiceForGunWithTransport(Gun1ID, cfg, transport, logger)
}

func NewGunCommandServiceForGun(gunID int, cfg config.PLCConfig, logger *slog.Logger) *GunCommandService {
	if logger == nil {
		logger = slog.Default()
	}
	return NewGunCommandServiceForGunWithTransport(gunID, cfg, NewModbusTCPTransport(cfg), logger)
}

func NewGunCommandServiceForGunWithTransport(gunID int, cfg config.PLCConfig, transport RegisterTransport, logger *slog.Logger) *GunCommandService {
	if logger == nil {
		logger = slog.Default()
	}
	if gunID <= 0 {
		gunID = Gun1ID
	}
	return &GunCommandService{
		cfg:         cfg,
		transport:   transport,
		logger:      logger,
		gunID:       gunID,
		commandBase: gun1CommandBase,
		statusBase:  gun1StatusBase,
	}
}

func NewCommandLifecycleTracker(cfg config.PLCConfig, reader StatusReader, auditor CommandAuditUpdater, logger *slog.Logger) *CommandLifecycleTracker {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &CommandLifecycleTracker{cfg: cfg, reader: reader, auditor: auditor, logger: logger, ctx: ctx, cancel: cancel}
}

func (t *CommandLifecycleTracker) Start(req CommandLifecycleRequest) error {
	if t == nil {
		return errors.New("plc lifecycle tracker is unavailable")
	}
	if t.reader == nil {
		return errors.New("plc lifecycle status reader is unavailable")
	}
	if t.auditor == nil {
		return errors.New("plc lifecycle audit updater is unavailable")
	}
	if req.AuditID <= 0 {
		return errors.New("plc lifecycle audit id is required")
	}
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.track(req)
	}()
	return nil
}

func (t *CommandLifecycleTracker) Close() {
	if t == nil {
		return
	}
	t.cancel()
	t.wg.Wait()
}

func (t *CommandLifecycleTracker) track(req CommandLifecycleRequest) {
	started := time.Now().UTC()
	timeout := t.cfg.LifecycleCompletionTimeout
	if req.Timeout > 0 {
		timeout = req.Timeout
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(t.ctx, timeout)
	defer cancel()
	t.logger.Info("gun plc command lifecycle tracker started",
		"audit_id", req.AuditID,
		"gun_id", req.GunID,
		"submitted_sequence", req.SubmittedSequence,
		"baseline_shot_sequence", req.BaselineStatus.ShotSequence,
		"baseline_state", req.BaselineStatus.StateName,
	)
	completion := t.runLifecycle(ctx, req)
	duration := time.Since(started)
	event := CommandLifecycleEvent{
		AuditID:           req.AuditID,
		GunID:             req.GunID,
		SubmittedSequence: req.SubmittedSequence,
		Completion:        completion,
	}
	updateCtx, updateCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer updateCancel()
	if err := t.auditor.UpdateGunCommandCompletion(updateCtx, req.AuditID, completion); err != nil {
		event.UpdateError = err.Error()
		t.logger.Warn("gun plc command lifecycle audit update failed",
			"audit_id", req.AuditID,
			"gun_id", req.GunID,
			"submitted_sequence", req.SubmittedSequence,
			"result", completion.Result,
			"error", err,
		)
		t.publishLifecycleEvent(req, event)
		return
	}
	t.publishLifecycleEvent(req, event)
	t.logger.Info("gun plc command lifecycle tracker completed",
		"audit_id", req.AuditID,
		"gun_id", req.GunID,
		"submitted_sequence", req.SubmittedSequence,
		"result", completion.Result,
		"resulting_state", completion.ResultingState,
		"fault_code", completion.FaultCode,
		"shot_sequence", optionalUint32Value(completion.ShotSequence),
		"duration_ms", duration.Milliseconds(),
	)
}

func (t *CommandLifecycleTracker) publishLifecycleEvent(req CommandLifecycleRequest, event CommandLifecycleEvent) {
	if req.Completion == nil {
		return
	}
	select {
	case req.Completion <- event:
	default:
		t.logger.Warn("gun plc command lifecycle completion channel is full",
			"audit_id", req.AuditID,
			"gun_id", req.GunID,
			"submitted_sequence", req.SubmittedSequence,
			"result", event.Completion.Result,
		)
	}
}

func (t *CommandLifecycleTracker) runLifecycle(ctx context.Context, req CommandLifecycleRequest) CommandAuditCompletion {
	pollInterval := t.cfg.LifecyclePollInterval
	if pollInterval <= 0 {
		pollInterval = 200 * time.Millisecond
	}
	timeout := t.cfg.LifecycleCompletionTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	maxErrors := t.cfg.LifecycleMaxConsecutiveErrors
	if maxErrors <= 0 {
		maxErrors = 5
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	lastStatus := req.AckStatus
	lastStatusValid := req.AckStatus.AcceptedSequence == req.SubmittedSequence
	highestAccepted := req.SubmittedSequence
	lastState := req.AckStatus.State
	sawMeaningfulProgress := req.AckStatus.State != req.BaselineStatus.State || req.AckStatus.ShotSequence != req.BaselineStatus.ShotSequence
	shotIncremented := sequenceAfter(req.AckStatus.ShotSequence, req.BaselineStatus.ShotSequence)
	firingObserved := req.AckStatus.Firing
	lockoutObserved := req.AckStatus.Lockout
	readyAfterCommandObserved := req.AckStatus.Ready && sawMeaningfulProgress
	consecutiveErrors := 0
	t.persistLifecycleEvidence(ctx, req, req.AckStatus, shotIncremented, firingObserved, lockoutObserved, readyAfterCommandObserved, "")

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			t.persistLifecycleEvidence(context.Background(), req, lastStatus, shotIncremented, firingObserved, lockoutObserved, readyAfterCommandObserved, fmt.Sprintf("lifecycle tracking timed out: %v", err))
			return lifecycleCompletion("completion_timeout", lastStatus, lastStatusValid, fmt.Sprintf("lifecycle tracking timed out: %v", err))
		case <-ticker.C:
		}
		status, err := t.reader.ReadStatus(ctx)
		if err != nil {
			consecutiveErrors++
			t.logger.Warn("gun plc command lifecycle status read failed",
				"audit_id", req.AuditID,
				"gun_id", req.GunID,
				"submitted_sequence", req.SubmittedSequence,
				"consecutive_errors", consecutiveErrors,
				"max_consecutive_errors", maxErrors,
				"error", err,
			)
			if consecutiveErrors >= maxErrors {
				t.persistLifecycleEvidence(context.Background(), req, lastStatus, shotIncremented, firingObserved, lockoutObserved, readyAfterCommandObserved, fmt.Sprintf("lifecycle status read failed %d consecutive times: %v", consecutiveErrors, err))
				return lifecycleCompletion("completion_timeout", lastStatus, lastStatusValid, fmt.Sprintf("lifecycle status read failed %d consecutive times: %v", consecutiveErrors, err))
			}
			continue
		}
		consecutiveErrors = 0
		lastStatusValid = true
		if status.State != lastState {
			t.logger.Info("gun plc command lifecycle state changed",
				"audit_id", req.AuditID,
				"gun_id", req.GunID,
				"submitted_sequence", req.SubmittedSequence,
				"state", status.StateName,
				"state_code", status.State,
			)
			lastState = status.State
		}
		lastStatus = status
		if sequenceAfter(highestAccepted, status.AcceptedSequence) {
			return lifecycleCompletion("protocol_state", status, true, fmt.Sprintf("plc accepted sequence regressed from %d to %d while tracking submitted sequence %d", highestAccepted, status.AcceptedSequence, req.SubmittedSequence))
		}
		if status.AcceptedSequence != req.SubmittedSequence {
			return lifecycleCompletion("protocol_state", status, true, fmt.Sprintf("plc accepted sequence changed to %d while tracking submitted sequence %d", status.AcceptedSequence, req.SubmittedSequence))
		}
		if sequenceAfter(status.AcceptedSequence, highestAccepted) {
			highestAccepted = status.AcceptedSequence
		}
		if status.FaultActive || status.State == 7 {
			return lifecycleCompletion("faulted", status, true, status.FaultName)
		}
		if status.State != req.BaselineStatus.State || status.ShotSequence != req.BaselineStatus.ShotSequence || status.Lockout || status.Settling || status.Firing {
			sawMeaningfulProgress = true
		}
		if status.Firing {
			firingObserved = true
		}
		if status.Lockout {
			lockoutObserved = true
		}
		if sequenceAfter(status.ShotSequence, req.BaselineStatus.ShotSequence) {
			if !shotIncremented {
				t.logger.Info("gun plc command lifecycle shot sequence incremented",
					"audit_id", req.AuditID,
					"gun_id", req.GunID,
					"submitted_sequence", req.SubmittedSequence,
					"baseline_shot_sequence", req.BaselineStatus.ShotSequence,
					"shot_sequence", status.ShotSequence,
				)
			}
			shotIncremented = true
		}
		if status.Ready && sawMeaningfulProgress {
			readyAfterCommandObserved = true
		}
		t.persistLifecycleEvidence(ctx, req, status, shotIncremented, firingObserved, lockoutObserved, readyAfterCommandObserved, "")
		if shotIncremented {
			testcontrol.Wait(ctx, testcontrol.PointAfterShotEvidence, 0, req.AuditID, 0)
		}
		if commandLifecycleCompleted(req.Command, req.BaselineStatus, status, sawMeaningfulProgress, shotIncremented) {
			if req.Command.FireRequest && shotIncremented {
				return lifecycleCompletion("completed_fired", status, true, "")
			}
			return lifecycleCompletion("completed_no_fire", status, true, "")
		}
	}
}

func (t *CommandLifecycleTracker) persistLifecycleEvidence(ctx context.Context, req CommandLifecycleRequest, status GunStatus, shotObserved, firingObserved, lockoutObserved, readyAfterCommandObserved bool, uncertaintyReason string) {
	updater, ok := t.auditor.(CommandLifecycleEvidenceUpdater)
	if !ok || updater == nil {
		return
	}
	now := time.Now().UTC()
	evidence := CommandLifecycleEvidence{
		BaselineAcceptedSequence:     uint32Ptr(req.BaselineStatus.AcceptedSequence),
		ConfirmedAcceptedSequence:    uint32Ptr(status.AcceptedSequence),
		BaselineShotSequence:         uint32Ptr(req.BaselineStatus.ShotSequence),
		LastObservedShotSequence:     uint32Ptr(status.ShotSequence),
		ShotObserved:                 boolPtr(shotObserved),
		LastObservedState:            uint16Ptr(status.State),
		FiringObserved:               boolPtr(firingObserved),
		LockoutObserved:              boolPtr(lockoutObserved),
		ReadyAfterCommandObserved:    boolPtr(readyAfterCommandObserved),
		CommandCommittedAt:           now,
		LastLifecycleObservationTime: now,
		UncertaintyReason:            uncertaintyReason,
	}
	updateCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := updater.UpdateGunCommandLifecycleEvidence(updateCtx, req.AuditID, evidence); err != nil {
		t.logger.Warn("gun plc command lifecycle evidence update failed", "audit_id", req.AuditID, "gun_id", req.GunID, "submitted_sequence", req.SubmittedSequence, "error", err)
	}
}

func commandLifecycleCompleted(command GunCommand, baseline GunStatus, status GunStatus, sawMeaningfulProgress bool, shotIncremented bool) bool {
	if status.AcceptedSequence == 0 && baseline.AcceptedSequence != 0 {
		return false
	}
	if command.FireRequest {
		return shotIncremented && status.State == 1 && !status.FaultActive
	}
	if command.FaultReset {
		return status.State == 1 && !status.FaultActive && sawMeaningfulProgress
	}
	if command.Maintenance || command.Mode == ModeMaintenance {
		return status.State == 6 && !status.FaultActive
	}
	return status.State == 1 && !status.FaultActive && sawMeaningfulProgress
}

func lifecycleCompletion(result string, status GunStatus, hasStatus bool, message string) CommandAuditCompletion {
	var shotSequence *uint32
	if hasStatus {
		shotSequence = uint32Ptr(status.ShotSequence)
	}
	return CommandAuditCompletion{
		CompletionTime: time.Now().UTC(),
		Result:         result,
		ResultingState: status.State,
		FaultCode:      status.FaultCode,
		ShotSequence:   shotSequence,
		Error:          message,
	}
}

func uint32Ptr(value uint32) *uint32 {
	return &value
}

func uint16Ptr(value uint16) *uint16 {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func optionalUint32Value(value *uint32) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func (s *GunCommandService) ReadStatus(ctx context.Context) (GunStatus, error) {
	words, err := s.transport.ReadHoldingRegisters(ctx, s.statusBase, statusRegisterCount)
	if err != nil {
		return GunStatus{}, err
	}
	return DecodeGunStatus(words)
}

func (s *GunCommandService) WriteSafetyLeases(ctx context.Context, lease uint16) error {
	return s.transport.WriteMultipleRegisters(ctx, s.commandAddress(RegisterSafetyFirePermissive), []uint16{lease, lease})
}

func (s *GunCommandService) Submit(ctx context.Context, gunID int, command GunCommand) (CommandResult, error) {
	if gunID != s.gunID {
		err := fmt.Errorf("plc command service for gun %d cannot submit for gun %d", s.gunID, gunID)
		return CommandResult{ErrorType: "validation", Error: err.Error()}, err
	}
	if err := ValidateGunCommand(command); err != nil {
		return CommandResult{ErrorType: "validation", Error: err.Error()}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	status, err := s.ReadStatus(ctx)
	if err != nil {
		return CommandResult{ErrorType: "transport", Error: err.Error()}, err
	}
	sequence := s.allocateSequence(status.AcceptedSequence)
	payload := BuildPayload(sequence, command)
	s.logger.Info("submitting gun plc command",
		"gun_id", gunID,
		"submitted_sequence", sequence,
		"plc_accepted_sequence", status.AcceptedSequence,
		"plc_accepted_sequence_low", status.AcceptedSequenceLow,
		"plc_accepted_sequence_high", status.AcceptedSequenceHigh,
		"host", s.cfg.Host,
		"port", s.cfg.Port,
	)
	if err := s.transport.WriteMultipleRegisters(ctx, s.commandAddress(RegisterCommandSequenceHigh), payload[1:RegisterSafetyFirePermissive]); err != nil {
		return CommandResult{SubmittedSequence: sequence, ErrorType: "transport", Error: err.Error()}, err
	}
	if err := s.transport.WriteSingleRegister(ctx, s.commandAddress(RegisterCommandSequenceLow), payload[0]); err != nil {
		return CommandResult{SubmittedSequence: sequence, ErrorType: "transport", Error: err.Error()}, err
	}
	result, err := s.pollAcknowledgment(ctx, sequence, status.AcceptedSequence, command)
	result.BaselineStatus = &status
	s.logger.Info("gun plc command result",
		"gun_id", gunID,
		"submitted_sequence", sequence,
		"plc_accepted_sequence", result.AcceptedSequence,
		"accepted", result.Accepted,
		"state", result.StateName,
		"fault", result.FaultName,
		"error_type", result.ErrorType,
	)
	return result, err
}

func (s *GunCommandService) SeedSequenceFloor(sequence uint32) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasLastAllocated || sequenceAfter(sequence, s.lastAllocatedSequence) {
		s.lastAllocatedSequence = sequence
		s.hasLastAllocated = true
	}
}

func (s *GunCommandService) allocateSequence(plcAccepted uint32) uint32 {
	base := plcAccepted
	if s.hasLastAllocated {
		if sequenceAfter(s.lastAllocatedSequence, plcAccepted) {
			s.logger.Warn("plc accepted sequence is behind middleware sequence floor",
				"plc_accepted_sequence", plcAccepted,
				"last_allocated_sequence", s.lastAllocatedSequence,
			)
			base = s.lastAllocatedSequence
		} else if sequenceAfter(plcAccepted, s.lastAllocatedSequence) {
			base = plcAccepted
		} else {
			base = s.lastAllocatedSequence
		}
	}
	sequence := NextSequence(base)
	s.lastAllocatedSequence = sequence
	s.hasLastAllocated = true
	return sequence
}

func (s *GunCommandService) pollAcknowledgment(ctx context.Context, sequence uint32, initialAccepted uint32, command GunCommand) (CommandResult, error) {
	timeout := s.cfg.CommandAckTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	interval := s.cfg.PollInterval
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	timer := time.NewTimer(0)
	defer timer.Stop()
	highestAccepted := initialAccepted
	lastStatus := GunStatus{AcceptedSequence: initialAccepted}
	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			result := resultFromStatus(sequence, lastStatus, false, "timeout", err)
			result.TimedOut = errors.Is(err, context.DeadlineExceeded)
			return result, err
		case <-timer.C:
		}
		status, err := s.ReadStatus(ctx)
		if err != nil {
			return resultFromStatus(sequence, status, false, "transport", err), err
		}
		lastStatus = status
		s.logger.Debug("polling gun plc command acknowledgment",
			"submitted_sequence", sequence,
			"plc_accepted_sequence", status.AcceptedSequence,
			"plc_accepted_sequence_low", status.AcceptedSequenceLow,
			"plc_accepted_sequence_high", status.AcceptedSequenceHigh,
		)
		if sequenceAfter(highestAccepted, status.AcceptedSequence) {
			err := fmt.Errorf("plc accepted sequence regressed from %d to %d while waiting for submitted sequence %d", highestAccepted, status.AcceptedSequence, sequence)
			return resultFromStatus(sequence, status, false, "protocol_state", err), err
		}
		if status.AcceptedSequence == sequence {
			if status.FaultActive && !command.FaultReset {
				err := fmt.Errorf("plc rejected command with fault %d (%s)", status.FaultCode, status.FaultName)
				return resultFromStatus(sequence, status, false, "plc_fault", err), err
			}
			return resultFromStatus(sequence, status, true, "", nil), nil
		}
		if sequenceAfter(status.AcceptedSequence, sequence) {
			err := fmt.Errorf("plc accepted sequence advanced to %d while waiting for submitted sequence %d", status.AcceptedSequence, sequence)
			return resultFromStatus(sequence, status, false, "protocol_state", err), err
		}
		if sequenceAfter(status.AcceptedSequence, highestAccepted) {
			highestAccepted = status.AcceptedSequence
		}
		timer.Reset(interval)
	}
}

func resultFromStatus(sequence uint32, status GunStatus, accepted bool, errorType string, err error) CommandResult {
	result := CommandResult{
		SubmittedSequence: sequence,
		AcceptedSequence:  status.AcceptedSequence,
		Accepted:          accepted,
		CurrentState:      status.State,
		StateName:         status.StateName,
		FaultActive:       status.FaultActive,
		FaultCode:         status.FaultCode,
		FaultName:         status.FaultName,
		ShotSequence:      status.ShotSequence,
		TargetAccepted:    status.TargetAccepted,
		OnTarget:          status.OnTarget,
		Settling:          status.Settling,
		Firing:            status.Firing,
		Lockout:           status.Lockout,
		Ready:             status.Ready,
		ErrorType:         errorType,
		Status:            &status,
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func NewCommandAudit(gunID int, source string, command GunCommand, result CommandResult) CommandAudit {
	outcome := "accepted"
	if !result.Accepted {
		outcome = result.ErrorType
		if outcome == "" {
			outcome = "rejected"
		}
	}
	return CommandAudit{
		Timestamp:         time.Now().UTC(),
		GunID:             gunID,
		Source:            source,
		SubmittedSequence: result.SubmittedSequence,
		Payload: map[string]interface{}{
			"mode":                       command.Mode,
			"target_azimuth":             command.TargetAzimuth,
			"target_elevation":           command.TargetElevation,
			"fragment_id":                command.FragmentID,
			"fragment_low":               command.FragmentLow,
			"fragment_high":              command.FragmentHigh,
			"fire_request":               command.FireRequest,
			"maintenance":                command.Maintenance,
			"admin_override":             command.AdminOverride,
			"fault_reset":                command.FaultReset,
			"safety_fire_permissive":     command.SafetyFirePermissive,
			"safety_extended_permissive": command.SafetyExtendedPermissive,
		},
		Result:         outcome,
		ResultingState: result.CurrentState,
		FaultCode:      result.FaultCode,
		ShotSequence:   result.ShotSequence,
		Error:          result.Error,
	}
}

func ValidateGunCommand(command GunCommand) error {
	switch command.Mode {
	case ModeAuto, ModeManual, ModeMaintenance:
	default:
		return fmt.Errorf("mode must be 0 (Auto), 1 (Manual), or 2 (Maintenance)")
	}
	if command.TargetAzimuth < 0 || command.TargetAzimuth >= 360 {
		return fmt.Errorf("target_azimuth must be >= 0 and < 360")
	}
	minElevation, maxElevation := 5.0, 85.0
	if command.AdminOverride && command.SafetyExtendedPermissive {
		minElevation, maxElevation = -30.0, 90.0
	}
	if command.TargetElevation < minElevation || command.TargetElevation > maxElevation {
		return fmt.Errorf("target_elevation must be between %.1f and %.1f degrees", minElevation, maxElevation)
	}
	if command.FragmentID != nil && (command.FragmentLow != nil || command.FragmentHigh != nil) {
		return fmt.Errorf("provide either fragment_id or fragment low/high words, not both")
	}
	return nil
}

func BuildPayload(sequence uint32, command GunCommand) []uint16 {
	low, high := SplitUint32(sequence)
	fragmentLow, fragmentHigh := uint16(0), uint16(0)
	if command.FragmentID != nil {
		fragmentLow, fragmentHigh = SplitUint32(*command.FragmentID)
	} else {
		if command.FragmentLow != nil {
			fragmentLow = *command.FragmentLow
		}
		if command.FragmentHigh != nil {
			fragmentHigh = *command.FragmentHigh
		}
	}
	return []uint16{
		low,
		high,
		uint16(command.Mode),
		uint16(math.Round(command.TargetAzimuth * 10)),
		Uint16FromInt16(int16(math.Round(command.TargetElevation * 10))),
		fragmentLow,
		fragmentHigh,
		boolWord(command.FireRequest),
		boolWord(command.Maintenance),
		boolWord(command.AdminOverride),
		boolWord(command.FaultReset),
		boolWord(command.SafetyFirePermissive),
		boolWord(command.SafetyExtendedPermissive),
	}
}

func DecodeGunStatus(words []uint16) (GunStatus, error) {
	if len(words) < int(statusRegisterCount) {
		return GunStatus{}, fmt.Errorf("status register read returned %d words, want %d", len(words), statusRegisterCount)
	}
	get := func(register uint16) uint16 {
		return words[register-statusStartRegister]
	}
	state := get(RegisterState)
	faultCode := get(RegisterFaultCode)
	currentAzimuth := Int16FromUint16(get(RegisterCurrentAzimuth))
	currentElevation := Int16FromUint16(get(RegisterCurrentElevation))
	acceptedLow := get(RegisterAcceptedSequenceLow)
	acceptedHigh := get(RegisterAcceptedSequenceHigh)
	return GunStatus{
		RawRegisters:           append([]uint16(nil), words[:statusRegisterCount]...),
		AcceptedSequence:       CombineUint32(acceptedLow, acceptedHigh),
		AcceptedSequenceLow:    acceptedLow,
		AcceptedSequenceHigh:   acceptedHigh,
		State:                  state,
		StateName:              StateName(state),
		CurrentAzimuthTenths:   currentAzimuth,
		CurrentElevationTenths: currentElevation,
		CurrentAzimuth:         float64(currentAzimuth) / 10,
		CurrentElevation:       float64(currentElevation) / 10,
		ActiveFragment:         CombineUint32(get(RegisterActiveFragmentLow), get(RegisterActiveFragmentHigh)),
		TargetAccepted:         wordBool(get(RegisterTargetAccepted)),
		OnTarget:               wordBool(get(RegisterOnTarget)),
		Settling:               wordBool(get(RegisterSettling)),
		Firing:                 wordBool(get(RegisterFiring)),
		Lockout:                wordBool(get(RegisterLockout)),
		Ready:                  wordBool(get(RegisterReady)),
		FaultActive:            wordBool(get(RegisterFaultActive)),
		FaultCode:              faultCode,
		FaultName:              FaultName(faultCode),
		ShotSequence:           CombineUint32(get(RegisterShotSequenceLow), get(RegisterShotSequenceHigh)),
		SettlingSeconds:        get(RegisterSettlingSeconds),
		LockoutSeconds:         get(RegisterLockoutSeconds),
	}, nil
}

func commandAddress(register uint16) uint16 {
	return gun1CommandBase + register
}

func (s *GunCommandService) commandAddress(register uint16) uint16 {
	if s == nil || s.commandBase == 0 {
		return commandAddress(register)
	}
	return s.commandBase + register
}

func NextSequence(accepted uint32) uint32 {
	return accepted + 1
}

func sequenceAfter(a, b uint32) bool {
	return a != b && a-b < 1<<31
}

func SplitUint32(value uint32) (low, high uint16) {
	return uint16(value & 0xffff), uint16(value >> 16)
}

func CombineUint32(low, high uint16) uint32 {
	return uint32(low) | uint32(high)<<16
}

func Uint16FromInt16(value int16) uint16 {
	return uint16(value)
}

func Int16FromUint16(value uint16) int16 {
	return int16(value)
}

func StateName(value uint16) string {
	switch value {
	case 0:
		return "Offline"
	case 1:
		return "Ready"
	case 2:
		return "Slewing"
	case 3:
		return "Settling"
	case 4:
		return "Firing"
	case 5:
		return "Reloading"
	case 6:
		return "Maintenance"
	case 7:
		return "Fault"
	default:
		return "Unknown"
	}
}

func FaultName(value uint16) string {
	switch value {
	case 0:
		return "None"
	case 1:
		return "Target range"
	case 2:
		return "Safety blocked"
	case 3:
		return "Invalid mode"
	case 4:
		return "Sequence error"
	case 5:
		return "Internal"
	default:
		return "Unknown"
	}
}

func boolWord(value bool) uint16 {
	if value {
		return 1
	}
	return 0
}

func wordBool(value uint16) bool {
	return value != 0
}

type ModbusTCPTransport struct {
	cfg config.PLCConfig
}

func NewModbusTCPTransport(cfg config.PLCConfig) *ModbusTCPTransport {
	return &ModbusTCPTransport{cfg: cfg}
}

func (t *ModbusTCPTransport) client() modbus.Client {
	handler := modbus.NewTCPClientHandler(fmt.Sprintf("%s:%d", t.cfg.Host, t.cfg.Port))
	handler.SlaveId = t.cfg.UnitID
	handler.Timeout = t.cfg.CommandAckTimeout
	if handler.Timeout <= 0 {
		handler.Timeout = 5 * time.Second
	}
	return modbus.NewClient(handler)
}

func (t *ModbusTCPTransport) ReadHoldingRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := t.client().ReadHoldingRegisters(address, quantity)
	if err != nil {
		return nil, err
	}
	if len(data) != int(quantity)*2 {
		return nil, fmt.Errorf("modbus returned %d bytes, want %d", len(data), int(quantity)*2)
	}
	words := make([]uint16, quantity)
	for i := range words {
		words[i] = uint16(data[i*2])<<8 | uint16(data[i*2+1])
	}
	return words, nil
}

func (t *ModbusTCPTransport) WriteMultipleRegisters(ctx context.Context, address uint16, values []uint16) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data := make([]byte, len(values)*2)
	for i, value := range values {
		data[i*2] = byte(value >> 8)
		data[i*2+1] = byte(value)
	}
	_, err := t.client().WriteMultipleRegisters(address, uint16(len(values)), data)
	return err
}

func (t *ModbusTCPTransport) WriteSingleRegister(ctx context.Context, address, value uint16) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := t.client().WriteSingleRegister(address, value)
	return err
}
