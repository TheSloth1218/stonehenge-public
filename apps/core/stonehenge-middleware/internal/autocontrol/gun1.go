package autocontrol

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"stonehenge-middleware/internal/arbitration"
	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/history"
	"stonehenge-middleware/internal/model"
	"stonehenge-middleware/internal/plc"
	"stonehenge-middleware/internal/testcontrol"
)

const commandSource = plc.CommandSourceAutoController

type State string

const (
	StateDisabled             State = "disabled"
	StateIdle                 State = "idle"
	StateTargetQueued         State = "target_queued"
	StateEngaging             State = "engaging"
	StateWaitingForCompletion State = "waiting_for_completion"
	StateWaitingForSafety     State = "waiting_for_safety"
	StateSuspendedFault       State = "suspended_fault"
	StateSuspendedUnavailable State = "suspended_unavailable"
)

var (
	ErrConflict   = errors.New("gun 1 auto target conflict")
	ErrValidation = errors.New("gun 1 auto target validation failed")
)

type Target struct {
	TargetID    uint32                 `json:"target_id"`
	Azimuth     float64                `json:"azimuth"`
	Elevation   float64                `json:"elevation"`
	Priority    int                    `json:"priority"`
	TargetClass string                 `json:"target_class"`
	DetectedAt  time.Time              `json:"detected_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type Assignment struct {
	ID           int64  `json:"id"`
	Generation   int    `json:"generation"`
	GunID        int    `json:"gun_id"`
	EngagementID *int64 `json:"engagement_id,omitempty"`
	Target       Target `json:"target"`
}

type AssignmentStateName string

const (
	AssignmentStateAccepted         AssignmentStateName = "accepted_by_gun"
	AssignmentStateWaitingForSafety AssignmentStateName = "waiting_for_safety"
	AssignmentStateCommandCommitted AssignmentStateName = "command_committed"
	AssignmentStateCompleted        AssignmentStateName = "completed"
	AssignmentStateFailed           AssignmentStateName = "terminal_failed"
	AssignmentStateUncertain        AssignmentStateName = "uncertain"
)

type AssignmentState struct {
	State           AssignmentStateName `json:"state"`
	CommandSequence *uint32             `json:"command_sequence,omitempty"`
	CommandAuditID  *int64              `json:"command_audit_id,omitempty"`
	EngagementID    *int64              `json:"engagement_id,omitempty"`
	Result          string              `json:"result,omitempty"`
	Error           string              `json:"error,omitempty"`
}

type AssignmentReporter interface {
	AssignmentStateChanged(ctx context.Context, assignment Assignment, state AssignmentState)
}

type Status struct {
	Enabled            bool           `json:"enabled"`
	State              State          `json:"state"`
	QueuedTarget       *Target        `json:"queued_target,omitempty"`
	ActiveTarget       *Target        `json:"active_target,omitempty"`
	ActiveSequence     *uint32        `json:"active_sequence,omitempty"`
	LastResult         string         `json:"last_result,omitempty"`
	LastError          string         `json:"last_error,omitempty"`
	LastStateChangedAt time.Time      `json:"last_state_changed_at"`
	PLC                *plc.GunStatus `json:"plc,omitempty"`
	PLCError           string         `json:"plc_error,omitempty"`
	Telemetry          interface{}    `json:"telemetry,omitempty"`
}

type CommandService interface {
	ReadStatus(ctx context.Context) (plc.GunStatus, error)
	Submit(ctx context.Context, gunID int, command plc.GunCommand) (plc.CommandResult, error)
}

type LifecycleStarter interface {
	Start(req plc.CommandLifecycleRequest) error
}

type SafetyDecision struct {
	Allowed bool
	Reason  string
}

type SafetyDecisionProvider interface {
	CanFire(gunID int) SafetyDecision
}

type Controller struct {
	gunID       int
	cfg         config.PLCConfig
	commands    CommandService
	lifecycle   LifecycleStarter
	commandsLog history.CommandAuditor
	engagements history.EngagementAuditor
	events      history.Sink
	arbitrator  *arbitration.Service
	safety      SafetyDecisionProvider
	logger      *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	wake   chan struct{}

	mu                 sync.Mutex
	enabled            bool
	state              State
	queued             *Target
	active             *Target
	assignment         *Assignment
	reporter           AssignmentReporter
	activeSequence     *uint32
	activeEngagementID int64
	lastResult         string
	lastError          string
	lastStateChangedAt time.Time
	running            bool
}

func NewController(cfg config.PLCConfig, commands CommandService, lifecycle LifecycleStarter, auditor history.CommandAuditor, engagements history.EngagementAuditor, events history.Sink, logger *slog.Logger) *Controller {
	return NewControllerForGun(plc.Gun1ID, cfg, commands, lifecycle, auditor, engagements, events, logger)
}

func NewControllerForGun(gunID int, cfg config.PLCConfig, commands CommandService, lifecycle LifecycleStarter, auditor history.CommandAuditor, engagements history.EngagementAuditor, events history.Sink, logger *slog.Logger) *Controller {
	if logger == nil {
		logger = slog.Default()
	}
	if gunID <= 0 {
		gunID = plc.Gun1ID
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Controller{
		gunID:              gunID,
		cfg:                cfg,
		commands:           commands,
		lifecycle:          lifecycle,
		commandsLog:        auditor,
		engagements:        engagements,
		events:             events,
		logger:             logger,
		ctx:                ctx,
		cancel:             cancel,
		wake:               make(chan struct{}, 1),
		state:              StateDisabled,
		lastStateChangedAt: time.Now().UTC(),
	}
}

func (c *Controller) SetArbitrator(arbitrator *arbitration.Service) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.arbitrator = arbitrator
}

func (c *Controller) SetSafetyDecisionProvider(provider SafetyDecisionProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.safety = provider
}

func (c *Controller) Start() {
	if c == nil {
		return
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.loop()
	}()
}

func (c *Controller) Close() {
	if c == nil {
		return
	}
	c.cancel()
	c.signal()
	c.wg.Wait()
}

func (c *Controller) SetEnabled(enabled bool) Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.enabled == enabled {
		return c.statusLocked(nil, "")
	}
	c.enabled = enabled
	if enabled {
		if c.queued != nil {
			c.setStateLocked(StateTargetQueued, "")
		} else if c.active == nil {
			c.setStateLocked(StateIdle, "")
		}
		c.logger.Info("gun auto enabled", "gun_id", c.gunID)
	} else {
		if c.active == nil {
			c.setStateLocked(StateDisabled, "")
		}
		c.logger.Info("gun auto disabled", "gun_id", c.gunID)
	}
	c.signal()
	return c.statusLocked(nil, "")
}

func (c *Controller) SubmitTarget(ctx context.Context, target Target) (Status, error) {
	return c.offer(ctx, Assignment{GunID: c.gunID, Target: target}, nil)
}

func (c *Controller) OfferAssignment(ctx context.Context, assignment Assignment, reporter AssignmentReporter) (Status, error) {
	return c.offer(ctx, assignment, reporter)
}

func (c *Controller) offer(ctx context.Context, assignment Assignment, reporter AssignmentReporter) (Status, error) {
	target := assignment.Target
	if assignment.GunID != c.gunID {
		return c.Status(ctx), fmt.Errorf("%w: assignment gun %d cannot be accepted by gun %d", ErrValidation, assignment.GunID, c.gunID)
	}
	if err := ValidateTarget(target); err != nil {
		return c.Status(ctx), fmt.Errorf("%w: %v", ErrValidation, err)
	}
	if target.DetectedAt.IsZero() {
		target.DetectedAt = time.Now().UTC()
	} else {
		target.DetectedAt = target.DetectedAt.UTC()
	}
	c.mu.Lock()
	if c.queued != nil || c.active != nil {
		status := c.statusLocked(nil, "")
		c.mu.Unlock()
		return status, fmt.Errorf("%w: gun %d already has a queued or active auto target", ErrConflict, c.gunID)
	}
	c.queued = cloneTarget(&target)
	c.assignment = &assignment
	c.reporter = reporter
	if assignment.EngagementID != nil && *assignment.EngagementID > 0 {
		c.activeEngagementID = *assignment.EngagementID
	} else {
		c.activeEngagementID = c.recordEngagement(ctx, target, StateTargetQueued, "")
	}
	if reporter != nil {
		reporter.AssignmentStateChanged(ctx, assignment, AssignmentState{State: AssignmentStateAccepted, EngagementID: optionalInt64(c.activeEngagementID, c.activeEngagementID > 0)})
	}
	if c.enabled {
		c.setStateLocked(StateTargetQueued, "")
	} else {
		c.setStateLocked(StateDisabled, "")
	}
	status := c.statusLocked(nil, "")
	c.mu.Unlock()
	c.logger.Info("gun auto target queued", "gun_id", c.gunID, "target_id", target.TargetID, "enabled", status.Enabled, "state", status.State)
	c.signal()
	return status, nil
}

func (c *Controller) Status(ctx context.Context) Status {
	var plcStatus *plc.GunStatus
	var plcErr string
	if c.commands != nil {
		readCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
		status, err := c.commands.ReadStatus(readCtx)
		cancel()
		if err != nil {
			plcErr = err.Error()
		} else {
			plcStatus = &status
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.statusLocked(plcStatus, plcErr)
}

func (c *Controller) TelemetrySnapshot(plcStatus *plc.GunStatus, lastPLCUpdateAt *time.Time, plcStale bool) TelemetrySnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	snapshot := TelemetrySnapshot{
		SchemaVersion:      TelemetrySchemaVersion,
		Healthy:            c.running && !plcStale,
		ControllerRunning:  c.running,
		Enabled:            c.enabled,
		State:              c.state,
		QueuedTargetID:     targetID(c.queued),
		ActiveTargetID:     targetID(c.active),
		ActiveSequence:     cloneUint32(c.activeSequence),
		LastResult:         c.lastResult,
		LastErrorCode:      ClassifyError(c.lastError),
		LastError:          c.lastError,
		LastStateChangedAt: c.lastStateChangedAt,
		LastPLCUpdateAt:    cloneTime(lastPLCUpdateAt),
		PLCStale:           plcStale,
	}
	target := c.active
	if target == nil {
		target = c.queued
	}
	if target != nil {
		snapshot.TargetAzimuthTenths = int16(math.Round(target.Azimuth * 10))
		snapshot.TargetElevationTenths = int16(math.Round(target.Elevation * 10))
		if target.Priority > 0 {
			snapshot.TargetPriority = uint16(target.Priority)
		}
	}
	if plcStatus != nil {
		status := *plcStatus
		snapshot.PLC = &status
	}
	if c.arbitrator != nil {
		status := c.arbitrator.Status()
		snapshot.Arbitration = &status
	}
	return snapshot
}

func ValidateTarget(target Target) error {
	if target.TargetID == 0 {
		return errors.New("target_id must be nonzero")
	}
	if !finite(target.Azimuth) || target.Azimuth < 0 || target.Azimuth >= 360 {
		return errors.New("azimuth must be finite and >= 0 and < 360")
	}
	if !finite(target.Elevation) || target.Elevation < 5 || target.Elevation > 85 {
		return errors.New("elevation must be finite and between 5 and 85 degrees")
	}
	if target.Priority < 0 || target.Priority > 100 {
		return errors.New("priority must be between 0 and 100")
	}
	return nil
}

func (c *Controller) loop() {
	c.mu.Lock()
	c.running = true
	c.mu.Unlock()
	c.logger.Info("gun auto controller started", "gun_id", c.gunID)
	defer func() {
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
		c.logger.Info("gun auto controller stopped", "gun_id", c.gunID)
	}()
	for {
		if !c.waitForWork() {
			return
		}
		c.processQueued(c.ctx)
	}
}

func (c *Controller) waitForWork() bool {
	for {
		if c.ctx.Err() != nil {
			return false
		}
		c.mu.Lock()
		ready := c.enabled && c.queued != nil && c.active == nil && !isSuspended(c.state)
		c.mu.Unlock()
		if ready {
			return true
		}
		select {
		case <-c.ctx.Done():
			return false
		case <-c.wake:
		}
	}
}

func (c *Controller) processQueued(ctx context.Context) {
	pollInterval := c.cfg.AutoReadinessPollInterval
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		target, ok := c.currentQueuedTarget()
		if !ok {
			return
		}
		status, err := c.commands.ReadStatus(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.setLastError(fmt.Sprintf("plc status read failed while waiting for auto readiness: %v", err))
			c.logger.Warn("gun auto readiness status read failed", "gun_id", c.gunID, "target_id", target.TargetID, "error", err)
		} else if status.FaultActive || status.State == 7 {
			c.suspend(StateSuspendedFault, fmt.Sprintf("plc fault active: %s", status.FaultName), &target, nil, status)
			return
		} else if readyForAuto(status) {
			if c.lifecycle == nil || c.commandsLog == nil {
				c.suspend(StateSuspendedUnavailable, "auto lifecycle tracking and command audit are required before AUTO command submission", &target, nil, status)
				return
			}
			permissive, reason := c.safetyPermissiveLocked()
			if !permissive {
				c.mu.Lock()
				wasWaiting := c.state == StateWaitingForSafety
				c.setStateLocked(StateWaitingForSafety, reason)
				assignment, reporter := c.assignmentReporterLocked()
				c.mu.Unlock()
				c.setLastError(reason)
				if reporter != nil {
					reporter.AssignmentStateChanged(ctx, assignment, AssignmentState{State: AssignmentStateWaitingForSafety, Error: reason})
				}
				if !wasWaiting {
					c.logger.Warn("gun auto waiting for safety permissive", "gun_id", c.gunID, "target_id", target.TargetID, "reason", reason)
				}
			} else if !c.cfg.AutoSafetyFirePermissive && c.safety == nil {
				c.suspend(StateSuspendedUnavailable, "auto safety fire permissive is false; configure authoritative safety permissive before AUTO fire", &target, nil, status)
				return
			} else {
				c.startEngagement(ctx, target, permissive)
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-c.wake:
		}
	}
}

func (c *Controller) startEngagement(ctx context.Context, target Target, safetyPermissive bool) {
	now := time.Now().UTC()
	var lease *arbitration.Lease
	var err error
	if c.arbitrator != nil {
		lease, _, err = c.arbitrator.AcquireAuto(ctx)
		if err != nil {
			c.setLastError("auto arbitration blocked command submission: " + err.Error())
			c.logger.Warn("gun auto waiting on arbitration", "gun_id", c.gunID, "target_id", target.TargetID, "error", err)
			return
		}
	}
	c.mu.Lock()
	if !c.enabled || c.queued == nil || c.active != nil || c.queued.TargetID != target.TargetID {
		c.mu.Unlock()
		if lease != nil {
			lease.Release()
		}
		return
	}
	c.active = cloneTarget(&target)
	c.queued = nil
	c.activeSequence = nil
	engagementID := c.activeEngagementID
	c.setStateLocked(StateEngaging, "")
	c.mu.Unlock()
	c.updateEngagement(ctx, engagementID, history.EngagementUpdate{State: string(StateEngaging), StartedAt: &now})

	fragmentID := target.TargetID
	command := plc.GunCommand{
		Mode:                     plc.ModeAuto,
		TargetAzimuth:            target.Azimuth,
		TargetElevation:          target.Elevation,
		FragmentID:               &fragmentID,
		FireRequest:              true,
		Maintenance:              false,
		AdminOverride:            false,
		FaultReset:               false,
		SafetyFirePermissive:     safetyPermissive,
		SafetyExtendedPermissive: false,
	}
	c.logger.Info("gun auto engagement started", "gun_id", c.gunID, "target_id", target.TargetID)
	c.mu.Lock()
	assignment, _ := c.assignmentReporterLocked()
	c.mu.Unlock()
	testcontrol.Wait(ctx, testcontrol.PointBeforeAutoSubmit, target.TargetID, assignment.ID, assignment.Generation)
	result, err := c.commands.Submit(ctx, c.gunID, command)
	auditID, auditRecorded := c.recordCommandAudit(ctx, command, result)
	c.updateEngagement(ctx, engagementID, history.EngagementUpdate{
		CommandAuditID:    optionalInt64(auditID, auditRecorded),
		SubmittedSequence: optionalUint32(result.SubmittedSequence, result.SubmittedSequence != 0 || result.Accepted),
		State:             string(StateEngaging),
	})
	if err != nil {
		if lease != nil {
			lease.Release()
		}
		c.completeSubmitFailure(ctx, engagementID, target, result, err)
		return
	}
	if !result.Accepted {
		if lease != nil {
			lease.Release()
		}
		c.completeSubmitFailure(ctx, engagementID, target, result, errors.New("plc command was not accepted"))
		return
	}
	c.mu.Lock()
	c.activeSequence = uint32Ptr(result.SubmittedSequence)
	assignment, reporter := c.assignmentReporterLocked()
	c.setStateLocked(StateWaitingForCompletion, "")
	c.mu.Unlock()
	if reporter != nil {
		reporter.AssignmentStateChanged(ctx, assignment, AssignmentState{State: AssignmentStateCommandCommitted, CommandSequence: uint32Ptr(result.SubmittedSequence), CommandAuditID: optionalInt64(auditID, auditRecorded), EngagementID: optionalInt64(engagementID, engagementID > 0)})
	}
	testcontrol.Wait(ctx, testcontrol.PointAfterCommit, target.TargetID, assignment.ID, assignment.Generation)
	c.updateEngagement(ctx, engagementID, history.EngagementUpdate{
		CommandAuditID:    optionalInt64(auditID, auditRecorded),
		SubmittedSequence: optionalUint32(result.SubmittedSequence, true),
		State:             string(StateWaitingForCompletion),
	})
	if !auditRecorded || c.lifecycle == nil {
		message := "auto lifecycle tracking unavailable"
		if !auditRecorded {
			message = "auto command audit row was not recorded; lifecycle tracking cannot target an audit row"
		}
		if lease != nil {
			lease.Release()
		}
		c.completeLifecycleStartFailure(ctx, engagementID, target, result, message)
		return
	}
	completions := make(chan plc.CommandLifecycleEvent, 1)
	if err := c.lifecycle.Start(plc.CommandLifecycleRequest{
		AuditID:           auditID,
		GunID:             c.gunID,
		Command:           command,
		SubmittedSequence: result.SubmittedSequence,
		BaselineStatus:    derefStatus(result.BaselineStatus),
		AckStatus:         derefStatus(result.Status),
		Source:            commandSource,
		Completion:        completions,
	}); err != nil {
		if lease != nil {
			lease.Release()
		}
		c.completeLifecycleStartFailure(ctx, engagementID, target, result, err.Error())
		return
	}
	c.logger.Info("gun auto waiting for lifecycle completion", "gun_id", c.gunID, "target_id", target.TargetID, "submitted_sequence", result.SubmittedSequence, "command_audit_id", auditID)
	select {
	case <-ctx.Done():
		if lease != nil {
			lease.Release()
		}
		return
	case event := <-completions:
		if lease != nil {
			lease.Release()
		}
		c.applyLifecycleResult(ctx, engagementID, target, event)
	}
}

func (c *Controller) applyLifecycleResult(ctx context.Context, engagementID int64, target Target, event plc.CommandLifecycleEvent) {
	completion := event.Completion
	result := completion.Result
	errorMessage := completion.Error
	if event.UpdateError != "" {
		errorMessage = stringsJoin(errorMessage, "audit update failed: "+event.UpdateError)
	}
	state := StateIdle
	if !c.isEnabled() {
		state = StateDisabled
	}
	switch completion.Result {
	case "completed_fired":
		c.reportAssignment(ctx, AssignmentState{State: AssignmentStateCompleted, Result: result})
		c.recordCompletedFire(ctx, engagementID, target, completion)
		c.finishActive(state, result, "", true)
	case "completed_no_fire":
		result = "completed_no_fire_abnormal"
		errorMessage = stringsJoin(errorMessage, "AUTO command requested fire but lifecycle completed without a shot")
		c.reportAssignment(ctx, AssignmentState{State: AssignmentStateCompleted, Result: result, Error: errorMessage})
		c.finishActive(state, result, errorMessage, true)
	case "faulted":
		state = StateSuspendedFault
		c.reportAssignment(ctx, AssignmentState{State: AssignmentStateFailed, Result: result, Error: errorMessage})
		c.finishActive(state, result, errorMessage, false)
		if c.arbitrator != nil {
			c.arbitrator.SuspendAutoFault(errorMessage)
		}
	case "completion_timeout", "protocol_state":
		state = StateSuspendedUnavailable
		c.reportAssignment(ctx, AssignmentState{State: AssignmentStateUncertain, Result: "committed_result_unknown", Error: errorMessage})
		c.finishActive(state, result, errorMessage, false)
	default:
		state = StateSuspendedUnavailable
		result = "unknown_lifecycle_result"
		errorMessage = stringsJoin(errorMessage, "unexpected lifecycle result: "+completion.Result)
		c.reportAssignment(ctx, AssignmentState{State: AssignmentStateFailed, Result: result, Error: errorMessage})
		c.finishActive(state, result, errorMessage, false)
	}
	c.updateEngagement(ctx, engagementID, history.EngagementUpdate{
		State:             string(state),
		Result:            result,
		CompletedAt:       &completion.CompletionTime,
		Error:             errorMessage,
		FinalShotSequence: completion.ShotSequence,
		FaultCode:         &completion.FaultCode,
	})
	c.logger.Info("gun auto lifecycle completed", "gun_id", c.gunID, "target_id", target.TargetID, "submitted_sequence", event.SubmittedSequence, "result", result, "state", state, "error", errorMessage)
	c.signal()
}

func (c *Controller) recordCompletedFire(ctx context.Context, engagementID int64, target Target, completion plc.CommandAuditCompletion) {
	if c.events == nil || completion.ShotSequence == nil {
		return
	}
	completedAt := completion.CompletionTime.UTC()
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	gunID := c.gunID
	azimuth := target.Azimuth
	elevation := target.Elevation
	shellType := model.ShellStandard
	shotSequence := uint64(*completion.ShotSequence)
	var externalFragmentID *string
	if value, ok := target.Metadata["external_id"].(string); ok && value != "" {
		externalFragmentID = &value
	}
	details := map[string]interface{}{
		"engagement_id": engagementID,
		"result":        "FIRED",
		"target_id":     target.TargetID,
	}
	events := []model.OperationalEvent{
		{
			Timestamp:          completedAt,
			EventType:          model.EventGunFired,
			Severity:           model.SeverityInfo,
			GunID:              &gunID,
			ExternalFragmentID: externalFragmentID,
			Azimuth:            &azimuth,
			Elevation:          &elevation,
			ShellType:          &shellType,
			ShotSequence:       &shotSequence,
			Message:            "gun fired",
			Details:            details,
		},
		{
			Timestamp:          completedAt,
			EventType:          model.EventInterceptRecorded,
			Severity:           model.SeverityInfo,
			GunID:              &gunID,
			ExternalFragmentID: externalFragmentID,
			Message:            "fragment intercept recorded",
			Details:            details,
		},
	}
	historyCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for _, event := range events {
		if err := c.events.WriteEvent(historyCtx, event); err != nil {
			c.logger.Warn("gun auto operational history write failed", "gun_id", c.gunID, "target_id", target.TargetID, "event_type", event.EventType, "error", err)
		}
	}
}

func (c *Controller) ResumeAfterFaultReset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = nil
	c.assignment = nil
	c.reporter = nil
	c.activeSequence = nil
	c.activeEngagementID = 0
	c.lastError = ""
	c.setStateLocked(StateIdle, "")
	c.signal()
}

func (c *Controller) ClearResolvedAssignment(assignmentID int64, generation int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.assignment == nil || c.assignment.ID != assignmentID || c.assignment.Generation != generation {
		return false
	}
	c.active = nil
	c.queued = nil
	c.assignment = nil
	c.reporter = nil
	c.activeSequence = nil
	c.activeEngagementID = 0
	c.lastError = ""
	if c.enabled {
		c.setStateLocked(StateIdle, "")
	} else {
		c.setStateLocked(StateDisabled, "")
	}
	c.signal()
	return true
}

func (c *Controller) completeSubmitFailure(ctx context.Context, engagementID int64, target Target, result plc.CommandResult, err error) {
	state := StateSuspendedUnavailable
	if result.ErrorType == "plc_fault" {
		state = StateSuspendedFault
	}
	message := err.Error()
	now := time.Now().UTC()
	faultCode := result.FaultCode
	shotSequence := result.ShotSequence
	c.reportAssignment(ctx, AssignmentState{State: AssignmentStateFailed, Result: result.ErrorType, Error: message})
	c.finishActive(state, result.ErrorType, message, false)
	c.updateEngagement(ctx, engagementID, history.EngagementUpdate{
		SubmittedSequence: optionalUint32(result.SubmittedSequence, result.SubmittedSequence != 0),
		State:             string(state),
		Result:            result.ErrorType,
		CompletedAt:       &now,
		Error:             message,
		FinalShotSequence: &shotSequence,
		FaultCode:         &faultCode,
	})
	c.logger.Warn("gun auto command submission failed", "gun_id", c.gunID, "target_id", target.TargetID, "result", result.ErrorType, "error", err)
}

func (c *Controller) completeLifecycleStartFailure(ctx context.Context, engagementID int64, target Target, result plc.CommandResult, message string) {
	now := time.Now().UTC()
	state := StateSuspendedUnavailable
	faultCode := result.FaultCode
	shotSequence := result.ShotSequence
	c.reportAssignment(ctx, AssignmentState{State: AssignmentStateFailed, Result: "protocol_state", Error: message})
	c.finishActive(state, "protocol_state", message, false)
	c.updateEngagement(ctx, engagementID, history.EngagementUpdate{
		SubmittedSequence: optionalUint32(result.SubmittedSequence, true),
		State:             string(state),
		Result:            "protocol_state",
		CompletedAt:       &now,
		Error:             message,
		FinalShotSequence: &shotSequence,
		FaultCode:         &faultCode,
	})
	c.logger.Warn("gun auto lifecycle tracking failed to start", "gun_id", c.gunID, "target_id", target.TargetID, "submitted_sequence", result.SubmittedSequence, "error", message)
}

func (c *Controller) recordCommandAudit(ctx context.Context, command plc.GunCommand, result plc.CommandResult) (int64, bool) {
	if c.commandsLog == nil {
		return 0, false
	}
	auditCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	id, err := c.commandsLog.RecordGunCommand(auditCtx, plc.NewCommandAudit(c.gunID, commandSource, command, result))
	if err != nil {
		c.logger.Warn("gun auto command audit write failed", "gun_id", c.gunID, "sequence", result.SubmittedSequence, "error", err)
		return 0, false
	}
	return id, true
}

func (c *Controller) recordEngagement(ctx context.Context, target Target, state State, message string) int64 {
	if c.engagements == nil {
		return 0
	}
	auditCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	id, err := c.engagements.RecordGunEngagement(auditCtx, history.EngagementRecord{
		TargetID:      target.TargetID,
		GunID:         c.gunID,
		Source:        commandSource,
		TargetPayload: targetPayload(target),
		State:         string(state),
		CreatedAt:     time.Now().UTC(),
		Error:         message,
	})
	if err != nil {
		c.logger.Warn("gun auto engagement audit write failed", "gun_id", c.gunID, "target_id", target.TargetID, "error", err)
		return 0
	}
	return id
}

func (c *Controller) updateEngagement(ctx context.Context, id int64, update history.EngagementUpdate) {
	if c.engagements == nil || id <= 0 {
		return
	}
	auditCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.engagements.UpdateGunEngagement(auditCtx, id, update); err != nil {
		c.logger.Warn("gun auto engagement audit update failed", "gun_id", c.gunID, "engagement_id", id, "state", update.State, "error", err)
	}
}

func (c *Controller) currentQueuedTarget() (Target, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.enabled || c.queued == nil || c.active != nil || isSuspended(c.state) {
		return Target{}, false
	}
	return *cloneTarget(c.queued), true
}

func (c *Controller) suspend(state State, message string, target *Target, result *string, status plc.GunStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = message
	if result != nil {
		c.lastResult = *result
	}
	if target != nil && c.active == nil {
		c.active = cloneTarget(target)
		c.queued = nil
	}
	c.setStateLocked(state, message)
	c.logger.Warn("gun auto suspended", "gun_id", c.gunID, "state", state, "error", message, "plc_state", status.StateName, "fault", status.FaultName)
}

func (c *Controller) finishActive(state State, result, message string, clearActive bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastResult = result
	c.lastError = message
	if clearActive {
		c.active = nil
		c.assignment = nil
		c.reporter = nil
		c.activeSequence = nil
		c.activeEngagementID = 0
	}
	c.setStateLocked(state, message)
}

func (c *Controller) assignmentReporterLocked() (Assignment, AssignmentReporter) {
	if c.assignment == nil || c.reporter == nil {
		return Assignment{}, nil
	}
	return *c.assignment, c.reporter
}

func (c *Controller) reportAssignment(ctx context.Context, state AssignmentState) {
	c.mu.Lock()
	assignment, reporter := c.assignmentReporterLocked()
	c.mu.Unlock()
	if reporter != nil {
		reporter.AssignmentStateChanged(ctx, assignment, state)
	}
}

func (c *Controller) setLastError(message string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = message
}

func (c *Controller) safetyPermissiveLocked() (bool, string) {
	provider := c.safety
	if provider == nil {
		return c.cfg.AutoSafetyFirePermissive, "auto safety fire permissive is false; configure authoritative safety permissive before AUTO fire"
	}
	decision := provider.CanFire(c.gunID)
	if decision.Allowed {
		return true, ""
	}
	if decision.Reason == "" {
		return false, "safety fire denied"
	}
	return false, decision.Reason
}

func (c *Controller) isEnabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enabled
}

func (c *Controller) statusLocked(plcStatus *plc.GunStatus, plcErr string) Status {
	return Status{
		Enabled:            c.enabled,
		State:              c.state,
		QueuedTarget:       cloneTarget(c.queued),
		ActiveTarget:       cloneTarget(c.active),
		ActiveSequence:     cloneUint32(c.activeSequence),
		LastResult:         c.lastResult,
		LastError:          c.lastError,
		LastStateChangedAt: c.lastStateChangedAt,
		PLC:                plcStatus,
		PLCError:           plcErr,
	}
}

func (c *Controller) setStateLocked(state State, message string) {
	if c.state != state {
		c.state = state
		c.lastStateChangedAt = time.Now().UTC()
	}
	if message != "" {
		c.lastError = message
	}
}

func (c *Controller) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func readyForAuto(status plc.GunStatus) bool {
	return status.State == 1 && status.Ready && !status.FaultActive && !status.Lockout && !status.Firing
}

func isSuspended(state State) bool {
	return state == StateSuspendedFault || state == StateSuspendedUnavailable
}

func targetPayload(target Target) map[string]interface{} {
	return map[string]interface{}{
		"target_id":    target.TargetID,
		"azimuth":      target.Azimuth,
		"elevation":    target.Elevation,
		"priority":     target.Priority,
		"target_class": target.TargetClass,
		"detected_at":  target.DetectedAt,
		"metadata":     target.Metadata,
	}
}

func derefStatus(status *plc.GunStatus) plc.GunStatus {
	if status == nil {
		return plc.GunStatus{}
	}
	return *status
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func cloneTarget(target *Target) *Target {
	if target == nil {
		return nil
	}
	out := *target
	if target.Metadata != nil {
		out.Metadata = make(map[string]interface{}, len(target.Metadata))
		for k, v := range target.Metadata {
			out.Metadata[k] = v
		}
	}
	return &out
}

func cloneUint32(value *uint32) *uint32 {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

func uint32Ptr(value uint32) *uint32 {
	return &value
}

func targetID(target *Target) *uint32 {
	if target == nil {
		return nil
	}
	return uint32Ptr(target.TargetID)
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

func optionalUint32(value uint32, ok bool) *uint32 {
	if !ok {
		return nil
	}
	return &value
}

func optionalInt64(value int64, ok bool) *int64 {
	if !ok {
		return nil
	}
	return &value
}

func stringsJoin(left, right string) string {
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	return left + "; " + right
}
