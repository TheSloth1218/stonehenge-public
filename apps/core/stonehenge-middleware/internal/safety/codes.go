package safety

const (
	ErrorNone uint16 = iota
	ErrorPLCUnreachable
	ErrorStale
	ErrorUnsupportedSchema
	ErrorSafetyFault
	ErrorGlobalInhibit
	ErrorGunInhibit
	ErrorControlWriteFailed
	ErrorControllerUnavailable
)

const ErrorOther uint16 = 255

func LastErrorCodeFromDecision(decision Decision) uint16 {
	switch decision.ReasonCode {
	case DecisionAllowed, DecisionEnforcementDisabled:
		return ErrorNone
	case DecisionDisconnected:
		return ErrorPLCUnreachable
	case DecisionStale:
		return ErrorStale
	case DecisionUnsupportedSchema:
		return ErrorUnsupportedSchema
	case DecisionSafetyFault:
		return ErrorSafetyFault
	case DecisionGlobalInhibit:
		return ErrorGlobalInhibit
	case DecisionGunInhibit:
		return ErrorGunInhibit
	default:
		return ErrorOther
	}
}
