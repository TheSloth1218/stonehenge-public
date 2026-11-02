package arbitration

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"stonehenge-middleware/internal/plc"
)

type Mode string

const (
	ModeManual        Mode = "manual"
	ModeAuto          Mode = "auto"
	ModeMaintenance   Mode = "maintenance"
	ModeAutoSuspended Mode = "auto_suspended"
)

type Operation string

const (
	OperationNone           Operation = "none"
	OperationManualCommand  Operation = "manual_command"
	OperationAutoEngagement Operation = "auto_engagement"
	OperationFaultReset     Operation = "fault_reset"
)

type ErrorCode uint16

const (
	ErrorNone ErrorCode = iota
	ErrorWrongMode
	ErrorManualActive
	ErrorAutoActive
	ErrorFaultResetActive
	ErrorMaintenanceActive
	ErrorPLCNotReady
	ErrorPLCFaulted
	ErrorAutoSuspended
	ErrorTransitionConflict
	ErrorResetNotAllowed
)

const ErrorOther ErrorCode = 255

type FaultResetResult string

const (
	ResetResultNone                 FaultResetResult = ""
	ResetResultSucceeded            FaultResetResult = "succeeded"
	ResetResultRejected             FaultResetResult = "rejected"
	ResetResultAcknowledgmentFailed FaultResetResult = "acknowledgment_failed"
	ResetResultFaultPersisted       FaultResetResult = "fault_persisted"
	ResetResultLifecycleTimeout     FaultResetResult = "lifecycle_timeout"
	ResetResultProtocolState        FaultResetResult = "protocol_state"
	ResetResultInternalError        FaultResetResult = "internal_error"
)

var ErrConflict = errors.New("gun 1 arbitration conflict")

type ConflictError struct {
	Reason string
	Mode   Mode
	Code   ErrorCode
}

func (e ConflictError) Error() string {
	return fmt.Sprintf("%s: %s", ErrConflict, e.Reason)
}

func (e ConflictError) Unwrap() error {
	return ErrConflict
}

type Status struct {
	Mode                        Mode       `json:"mode"`
	PreviousMode                Mode       `json:"previous_mode,omitempty"`
	RequestedMode               Mode       `json:"requested_mode,omitempty"`
	LastChangedAt               time.Time  `json:"last_changed_at"`
	Reason                      string     `json:"reason,omitempty"`
	ManualActive                bool       `json:"manual_active"`
	AutoActive                  bool       `json:"auto_active"`
	FaultResetActive            bool       `json:"fault_reset_active"`
	ActiveOperation             Operation  `json:"active_operation"`
	LastArbitrationError        string     `json:"last_arbitration_error,omitempty"`
	LastArbitrationErrorCode    ErrorCode  `json:"last_arbitration_error_code"`
	AutoEnabledBeforeSuspension bool       `json:"auto_enabled_before_suspension"`
	LastFaultResetResult        string     `json:"last_fault_reset_result,omitempty"`
	LastFaultResetSequence      *uint32    `json:"last_fault_reset_sequence,omitempty"`
	LastFaultCodeBefore         *uint16    `json:"last_fault_code_before,omitempty"`
	LastFaultCodeAfter          *uint16    `json:"last_fault_code_after,omitempty"`
	ResetStateChangedAt         *time.Time `json:"reset_state_changed_at,omitempty"`
}

type StatusReader interface {
	ReadStatus(ctx context.Context) (plc.GunStatus, error)
}

type Service struct {
	reader StatusReader
	logger *slog.Logger
	gunID  int

	mu                          sync.Mutex
	mode                        Mode
	previousMode                Mode
	requestedMode               Mode
	lastChangedAt               time.Time
	reason                      string
	activeOperation             Operation
	operationToken              uint64
	lastError                   string
	lastErrorCode               ErrorCode
	autoEnabledBeforeSuspension bool
	lastResetResult             FaultResetResult
	lastResetSequence           *uint32
	lastFaultCodeBefore         *uint16
	lastFaultCodeAfter          *uint16
	resetStateChangedAt         *time.Time
	nextToken                   uint64
}

type Lease struct {
	service *Service
	token   uint64
	op      Operation
	once    sync.Once
}

func NewService(reader StatusReader, logger *slog.Logger) *Service {
	return NewServiceForGun(plc.Gun1ID, reader, logger)
}

func NewServiceForGun(gunID int, reader StatusReader, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	now := time.Now().UTC()
	if gunID <= 0 {
		gunID = plc.Gun1ID
	}
	return &Service{reader: reader, logger: logger, gunID: gunID, mode: ModeManual, activeOperation: OperationNone, lastChangedAt: now}
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked()
}

func (s *Service) RequestMode(ctx context.Context, mode Mode) (Status, error) {
	if mode == ModeAutoSuspended {
		return s.reject("auto_suspended is controller-owned", ErrorTransitionConflict)
	}
	if mode != ModeManual && mode != ModeAuto && mode != ModeMaintenance {
		return s.reject("unsupported mode", ErrorWrongMode)
	}
	s.logger.Info("gun mode transition requested", "gun_id", s.gunID, "mode", mode)
	s.mu.Lock()
	current := s.mode
	if current == mode {
		status := s.statusLocked()
		s.mu.Unlock()
		return status, nil
	}
	if s.activeOperation != OperationNone {
		reason, code := activeConflict(s.activeOperation)
		status := s.setErrorLocked(reason, code)
		s.mu.Unlock()
		return status, ConflictError{Reason: reason, Mode: current, Code: code}
	}
	if current == ModeAuto && mode == ModeManual && s.activeOperation == OperationAutoEngagement {
		status := s.setErrorLocked("auto_engagement_active", ErrorAutoActive)
		s.mu.Unlock()
		return status, ConflictError{Reason: "auto_engagement_active", Mode: current, Code: ErrorAutoActive}
	}
	s.requestedMode = mode
	s.mu.Unlock()

	if mode == ModeAuto {
		if err := s.validateAutoEntry(ctx); err != nil {
			return s.reject(err.Error(), classifyTransitionError(err))
		}
	}

	s.mu.Lock()
	s.previousMode = s.mode
	s.mode = mode
	s.requestedMode = ""
	s.lastChangedAt = time.Now().UTC()
	s.reason = "operator_request"
	s.lastError = ""
	s.lastErrorCode = ErrorNone
	status := s.statusLocked()
	s.mu.Unlock()

	s.logger.Info("gun mode transition accepted", "gun_id", s.gunID, "mode", mode, "previous_mode", current)
	return status, nil
}

func (s *Service) AcquireManual(ctx context.Context) (*Lease, Status, error) {
	return s.acquire(ctx, OperationManualCommand, ModeManual)
}

func (s *Service) AcquireAuto(ctx context.Context) (*Lease, Status, error) {
	return s.acquire(ctx, OperationAutoEngagement, ModeAuto)
}

func (s *Service) AcquireFaultReset(ctx context.Context, status plc.GunStatus, resumeAuto bool) (*Lease, Status, error) {
	s.mu.Lock()
	if s.activeOperation != OperationNone {
		reason, code := activeConflict(s.activeOperation)
		out := s.setErrorLocked(reason, code)
		s.mu.Unlock()
		return nil, out, ConflictError{Reason: reason, Mode: s.mode, Code: code}
	}
	if s.mode == ModeMaintenance {
		out := s.setErrorLocked("maintenance_active", ErrorMaintenanceActive)
		s.mu.Unlock()
		return nil, out, ConflictError{Reason: "maintenance_active", Mode: s.mode, Code: ErrorMaintenanceActive}
	}
	if !status.FaultActive {
		out := s.setErrorLocked("reset_not_allowed: plc has no active fault", ErrorResetNotAllowed)
		s.mu.Unlock()
		return nil, out, ConflictError{Reason: "reset_not_allowed", Mode: s.mode, Code: ErrorResetNotAllowed}
	}
	if !ResettableFault(status.FaultCode) {
		out := s.setErrorLocked("reset_not_allowed: non-resettable fault", ErrorResetNotAllowed)
		s.mu.Unlock()
		return nil, out, ConflictError{Reason: "reset_not_allowed", Mode: s.mode, Code: ErrorResetNotAllowed}
	}
	s.nextToken++
	token := s.nextToken
	s.activeOperation = OperationFaultReset
	s.operationToken = token
	s.lastResetResult = ResetResultNone
	s.lastFaultCodeBefore = uint16Ptr(status.FaultCode)
	s.lastFaultCodeAfter = nil
	s.resetStateChangedAt = timePtr(time.Now().UTC())
	if s.mode == ModeAutoSuspended || resumeAuto {
		s.autoEnabledBeforeSuspension = true
	}
	out := s.statusLocked()
	s.mu.Unlock()
	s.logger.Info("gun fault reset ownership acquired", "gun_id", s.gunID, "fault_code", status.FaultCode, "resume_auto", resumeAuto)
	return &Lease{service: s, token: token, op: OperationFaultReset}, out, nil
}

func (s *Service) SuspendAutoFault(reason string) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.previousMode = s.mode
	s.mode = ModeAutoSuspended
	s.lastChangedAt = time.Now().UTC()
	s.reason = reason
	s.autoEnabledBeforeSuspension = true
	s.lastError = reason
	s.lastErrorCode = ErrorPLCFaulted
	s.logger.Warn("gun auto suspended by fault", "gun_id", s.gunID, "reason", reason)
	return s.statusLocked()
}

func (s *Service) CompleteFaultReset(result FaultResetResult, final plc.GunStatus, message string) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeOperation == OperationFaultReset {
		s.releaseLocked(s.operationToken, OperationFaultReset)
	}
	s.lastResetResult = result
	s.lastFaultCodeAfter = uint16Ptr(final.FaultCode)
	if final.AcceptedSequence != 0 {
		s.lastResetSequence = uint32Ptr(final.AcceptedSequence)
	}
	s.resetStateChangedAt = timePtr(time.Now().UTC())
	if result == ResetResultSucceeded {
		resume := s.autoEnabledBeforeSuspension
		s.previousMode = s.mode
		if resume {
			s.mode = ModeAuto
			s.reason = "fault_reset_succeeded_resume_auto"
		} else {
			s.mode = ModeManual
			s.reason = "fault_reset_succeeded"
		}
		s.lastChangedAt = time.Now().UTC()
		s.lastError = ""
		s.lastErrorCode = ErrorNone
	} else {
		s.lastError = message
		s.lastErrorCode = classifyResetResult(result)
	}
	return s.statusLocked()
}

func (s *Service) acquire(ctx context.Context, op Operation, required Mode) (*Lease, Status, error) {
	if err := ctx.Err(); err != nil {
		status := s.Status()
		return nil, status, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeOperation != OperationNone {
		reason, code := activeConflict(s.activeOperation)
		status := s.setErrorLocked(reason, code)
		return nil, status, ConflictError{Reason: reason, Mode: s.mode, Code: code}
	}
	if s.mode != required {
		code := ErrorWrongMode
		reason := "wrong_mode"
		if s.mode == ModeAutoSuspended {
			code = ErrorAutoSuspended
			reason = "auto_suspended"
		} else if s.mode == ModeMaintenance {
			code = ErrorMaintenanceActive
			reason = "maintenance_active"
		}
		status := s.setErrorLocked(reason, code)
		return nil, status, ConflictError{Reason: reason, Mode: s.mode, Code: code}
	}
	s.nextToken++
	token := s.nextToken
	s.activeOperation = op
	s.operationToken = token
	s.lastError = ""
	s.lastErrorCode = ErrorNone
	status := s.statusLocked()
	s.logger.Info("gun operation ownership acquired", "gun_id", s.gunID, "operation", op, "mode", s.mode)
	return &Lease{service: s, token: token, op: op}, status, nil
}

func (l *Lease) Release() {
	if l == nil || l.service == nil {
		return
	}
	l.once.Do(func() {
		l.service.mu.Lock()
		defer l.service.mu.Unlock()
		l.service.releaseLocked(l.token, l.op)
	})
}

func (s *Service) releaseLocked(token uint64, op Operation) {
	if s.operationToken != token || s.activeOperation != op {
		return
	}
	s.logger.Info("gun operation ownership released", "gun_id", s.gunID, "operation", op, "mode", s.mode)
	s.activeOperation = OperationNone
	s.operationToken = 0
}

func (s *Service) validateAutoEntry(ctx context.Context) error {
	if s.reader == nil {
		return errors.New("auto controller unavailable")
	}
	readCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	status, err := s.reader.ReadStatus(readCtx)
	if err != nil {
		return fmt.Errorf("plc status read failed: %w", err)
	}
	if status.FaultActive {
		return fmt.Errorf("plc faulted: %s", status.FaultName)
	}
	if status.State != 1 || !status.Ready || status.Lockout || status.Firing {
		return fmt.Errorf("plc_not_ready: state=%s ready=%t lockout=%t firing=%t", status.StateName, status.Ready, status.Lockout, status.Firing)
	}
	return nil
}

func ResettableFault(code uint16) bool {
	switch code {
	case 1, 2, 3, 4:
		return true
	default:
		return false
	}
}

func (s *Service) reject(reason string, code ErrorCode) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.setErrorLocked(reason, code)
	return status, ConflictError{Reason: reason, Mode: s.mode, Code: code}
}

func (s *Service) setErrorLocked(reason string, code ErrorCode) Status {
	s.lastError = reason
	s.lastErrorCode = code
	s.logger.Warn("gun 1 arbitration rejected", "reason", reason, "mode", s.mode, "code", code)
	return s.statusLocked()
}

func (s *Service) statusLocked() Status {
	return Status{
		Mode:                        s.mode,
		PreviousMode:                s.previousMode,
		RequestedMode:               s.requestedMode,
		LastChangedAt:               s.lastChangedAt,
		Reason:                      s.reason,
		ManualActive:                s.activeOperation == OperationManualCommand,
		AutoActive:                  s.activeOperation == OperationAutoEngagement,
		FaultResetActive:            s.activeOperation == OperationFaultReset,
		ActiveOperation:             s.activeOperation,
		LastArbitrationError:        s.lastError,
		LastArbitrationErrorCode:    s.lastErrorCode,
		AutoEnabledBeforeSuspension: s.autoEnabledBeforeSuspension,
		LastFaultResetResult:        string(s.lastResetResult),
		LastFaultResetSequence:      cloneUint32(s.lastResetSequence),
		LastFaultCodeBefore:         cloneUint16(s.lastFaultCodeBefore),
		LastFaultCodeAfter:          cloneUint16(s.lastFaultCodeAfter),
		ResetStateChangedAt:         cloneTime(s.resetStateChangedAt),
	}
}

func activeConflict(op Operation) (string, ErrorCode) {
	switch op {
	case OperationManualCommand:
		return "manual_command_active", ErrorManualActive
	case OperationAutoEngagement:
		return "auto_engagement_active", ErrorAutoActive
	case OperationFaultReset:
		return "fault_reset_active", ErrorFaultResetActive
	default:
		return "transition_conflict", ErrorTransitionConflict
	}
}

func classifyTransitionError(err error) ErrorCode {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "fault"):
		return ErrorPLCFaulted
	case strings.Contains(msg, "not_ready"), strings.Contains(msg, "ready"), strings.Contains(msg, "lockout"), strings.Contains(msg, "firing"):
		return ErrorPLCNotReady
	case strings.Contains(msg, "unavailable"):
		return ErrorTransitionConflict
	default:
		return ErrorOther
	}
}

func classifyResetResult(result FaultResetResult) ErrorCode {
	switch result {
	case ResetResultLifecycleTimeout:
		return ErrorPLCNotReady
	case ResetResultProtocolState:
		return ErrorTransitionConflict
	case ResetResultFaultPersisted:
		return ErrorPLCFaulted
	case ResetResultRejected:
		return ErrorResetNotAllowed
	default:
		return ErrorOther
	}
}

func (m Mode) Code() uint16 {
	switch m {
	case ModeManual:
		return 0
	case ModeAuto:
		return 1
	case ModeMaintenance:
		return 2
	case ModeAutoSuspended:
		return 3
	default:
		return 255
	}
}

func (o Operation) Code() uint16 {
	switch o {
	case OperationNone:
		return 0
	case OperationManualCommand:
		return 1
	case OperationAutoEngagement:
		return 2
	case OperationFaultReset:
		return 3
	default:
		return 255
	}
}

func (r FaultResetResult) Code() uint16 {
	switch r {
	case ResetResultNone:
		return 0
	case ResetResultSucceeded:
		return 1
	case ResetResultRejected:
		return 2
	case ResetResultAcknowledgmentFailed:
		return 3
	case ResetResultFaultPersisted:
		return 4
	case ResetResultLifecycleTimeout:
		return 5
	case ResetResultProtocolState:
		return 6
	case ResetResultInternalError:
		return 7
	default:
		return 255
	}
}

func uint32Ptr(value uint32) *uint32     { return &value }
func uint16Ptr(value uint16) *uint16     { return &value }
func timePtr(value time.Time) *time.Time { return &value }

func cloneUint32(value *uint32) *uint32 {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

func cloneUint16(value *uint16) *uint16 {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}
