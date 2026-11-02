package autocontrol

import (
	"strings"
	"time"

	"stonehenge-middleware/internal/arbitration"
	"stonehenge-middleware/internal/plc"
)

const TelemetrySchemaVersion uint16 = 4

const (
	StateCodeDisabled uint16 = iota
	StateCodeIdle
	StateCodeTargetQueued
	StateCodeEngaging
	StateCodeWaitingForCompletion
	StateCodeSuspendedFault
	StateCodeSuspendedUnavailable
)

const StateCodeWaitingForSafety uint16 = 7
const StateCodeUnknown uint16 = 255

const (
	ResultCodeNone uint16 = iota
	ResultCodeAccepted
	ResultCodeCompletedFired
	ResultCodeCompletedNoFire
	ResultCodeFaulted
	ResultCodeCompletionTimeout
	ResultCodeProtocolState
)

const ResultCodeUnknown uint16 = 255

const (
	ErrorCodeNone uint16 = iota
	ErrorCodePLCUnreachable
	ErrorCodeGunNotReady
	ErrorCodeGunFaulted
	ErrorCodeSafetyNotPermissive
	ErrorCodeCommandRejected
	ErrorCodeLifecycleTimeout
	ErrorCodeProtocolState
	ErrorCodePersistenceError
	ErrorCodeControllerUnavailable
)

const ErrorCodeOther uint16 = 255

type TelemetrySnapshot struct {
	SchemaVersion         uint16
	Healthy               bool
	ControllerRunning     bool
	Enabled               bool
	State                 State
	QueuedTargetID        *uint32
	ActiveTargetID        *uint32
	ActiveSequence        *uint32
	LastResult            string
	LastErrorCode         uint16
	LastError             string
	LastStateChangedAt    time.Time
	TargetAzimuthTenths   int16
	TargetElevationTenths int16
	TargetPriority        uint16
	PLC                   *plc.GunStatus
	LastPLCUpdateAt       *time.Time
	PLCStale              bool
	Arbitration           *arbitration.Status
}

func (s State) Code() uint16 {
	switch s {
	case StateDisabled:
		return StateCodeDisabled
	case StateIdle:
		return StateCodeIdle
	case StateTargetQueued:
		return StateCodeTargetQueued
	case StateEngaging:
		return StateCodeEngaging
	case StateWaitingForCompletion:
		return StateCodeWaitingForCompletion
	case StateWaitingForSafety:
		return StateCodeWaitingForSafety
	case StateSuspendedFault:
		return StateCodeSuspendedFault
	case StateSuspendedUnavailable:
		return StateCodeSuspendedUnavailable
	default:
		return StateCodeUnknown
	}
}

func ResultCode(result string) uint16 {
	switch result {
	case "":
		return ResultCodeNone
	case "accepted":
		return ResultCodeAccepted
	case "completed_fired":
		return ResultCodeCompletedFired
	case "completed_no_fire", "completed_no_fire_abnormal":
		return ResultCodeCompletedNoFire
	case "faulted", "plc_fault":
		return ResultCodeFaulted
	case "completion_timeout", "timeout":
		return ResultCodeCompletionTimeout
	case "protocol_state":
		return ResultCodeProtocolState
	default:
		return ResultCodeUnknown
	}
}

func ClassifyError(message string) uint16 {
	msg := strings.ToLower(message)
	if msg == "" {
		return ErrorCodeNone
	}
	switch {
	case strings.Contains(msg, "unreachable"), strings.Contains(msg, "connection"), strings.Contains(msg, "connect"), strings.Contains(msg, "status read failed"):
		return ErrorCodePLCUnreachable
	case strings.Contains(msg, "not ready"), strings.Contains(msg, "lockout"), strings.Contains(msg, "slewing"), strings.Contains(msg, "settling"):
		return ErrorCodeGunNotReady
	case strings.Contains(msg, "fault"):
		return ErrorCodeGunFaulted
	case strings.Contains(msg, "safety") || strings.Contains(msg, "permissive"):
		return ErrorCodeSafetyNotPermissive
	case strings.Contains(msg, "not accepted"), strings.Contains(msg, "rejected"):
		return ErrorCodeCommandRejected
	case strings.Contains(msg, "timeout"):
		return ErrorCodeLifecycleTimeout
	case strings.Contains(msg, "protocol") || strings.Contains(msg, "regress"):
		return ErrorCodeProtocolState
	case strings.Contains(msg, "audit") || strings.Contains(msg, "persistence") || strings.Contains(msg, "database"):
		return ErrorCodePersistenceError
	case strings.Contains(msg, "unavailable"):
		return ErrorCodeControllerUnavailable
	default:
		return ErrorCodeOther
	}
}
