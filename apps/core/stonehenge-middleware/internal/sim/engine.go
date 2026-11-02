package sim

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/fragments"
	"stonehenge-middleware/internal/history"
	"stonehenge-middleware/internal/model"
	"stonehenge-middleware/internal/plc"
	"stonehenge-middleware/internal/safety"
	"stonehenge-middleware/internal/store"
)

const (
	maxExternalFragmentIDBytes = 256
	maxFragmentLabelBytes      = 128
	maxRejectedReasonBytes     = 512
)

type Engine struct {
	cfg        config.Config
	store      *store.MemoryStore
	plc        plc.Adapter
	safety     safety.Controller
	source     fragments.Source
	history    history.Sink
	logger     *slog.Logger
	mu         sync.RWMutex
	running    bool
	cancel     context.CancelFunc
	done       chan struct{}
	sourceDone chan struct{}
	nextFrag   int
}

func NewEngine(cfg config.Config, st *store.MemoryStore, plcAdapter plc.Adapter, safetyController safety.Controller, source fragments.Source, historySink history.Sink, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	if historySink == nil {
		historySink = history.NewNoOpSink()
	}
	e := &Engine{
		cfg:     cfg,
		store:   st,
		plc:     plcAdapter,
		safety:  safetyController,
		source:  source,
		history: historySink,
		logger:  logger,
	}
	e.Reset()
	return e
}

func (e *Engine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return
	}
	e.running = true
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.done = make(chan struct{})
	e.sourceDone = make(chan struct{})
	if source, ok := e.source.(*fragments.ExternalIngestSource); ok {
		source.Bind(e)
	}
	go e.loop(ctx, e.done)
	if e.source != nil {
		go func() {
			defer close(e.sourceDone)
			if err := e.source.Start(ctx, e); err != nil && !errors.Is(err, context.Canceled) {
				e.logger.Error("fragment source stopped with error", "source", e.source.Name(), "error", err)
			}
		}()
	} else {
		close(e.sourceDone)
	}
	e.record(model.EventSimulationStarted, model.SeverityInfo, nil, nil, "simulation started", nil)
}

func (e *Engine) Stop() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	cancel := e.cancel
	done := e.done
	sourceDone := e.sourceDone
	e.running = false
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	<-done
	<-sourceDone
	e.record(model.EventSimulationStopped, model.SeverityInfo, nil, nil, "simulation stopped", nil)
}

func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

func (e *Engine) Reset() {
	e.Stop()
	now := time.Now().UTC()
	guns := make([]model.GunState, 0, e.cfg.Simulation.GunCount)
	sectorWidth := 360.0 / float64(e.cfg.Simulation.GunCount)
	configuredModes := configuredGunModes(e.cfg.Guns)
	for i := 1; i <= e.cfg.Simulation.GunCount; i++ {
		start := model.NormalizeAzimuth(float64(i-1) * sectorWidth)
		end := model.NormalizeAzimuth(start + sectorWidth)
		if i == e.cfg.Simulation.GunCount {
			end = 359
		}
		mode := model.ModeManual
		state := model.GunManual
		if configuredModes[i] == model.ModeAuto {
			mode = model.ModeAuto
			state = model.GunIdle
		}
		gun := model.GunState{
			ID:                     i,
			Name:                   fmt.Sprintf("Stonehenge Turret %02d", i),
			Mode:                   mode,
			State:                  state,
			SectorStart:            start,
			SectorEnd:              end,
			CurrentAzimuth:         start + sectorWidth/2,
			CurrentElevation:       45,
			TargetAzimuth:          start + sectorWidth/2,
			TargetElevation:        45,
			SoftMinElevation:       e.cfg.Simulation.SoftMinElevation,
			SoftMaxElevation:       e.cfg.Simulation.SoftMaxElevation,
			HardLimitStatus:        "OK",
			SafetyPermissive:       true,
			ShellType:              model.ShellStandard,
			SpecialMeasuresEnabled: e.safety.GetSpecialMeasuresState().Enabled,
			AdminOverrideState:     "NOT_REQUESTED",
			AdminOverrideActive:    false,
			MaintenanceEnabled:     false,
			LastIdleTime:           now,
			EventSummary:           "ready",
		}
		guns = append(guns, gun)
		e.plc.UpsertGunState(gun)
	}
	e.store.Reset(guns)
	e.nextFrag = 0
	e.record(model.EventSimulationReset, model.SeverityInfo, nil, nil, "simulation reset", nil)
}

func configuredGunModes(guns []config.GunConfig) map[int]model.GunMode {
	modes := make(map[int]model.GunMode, len(guns))
	for _, gun := range guns {
		if gun.AutoEnabled {
			modes[gun.ID] = model.ModeAuto
		} else {
			modes[gun.ID] = model.ModeManual
		}
	}
	return modes
}

func (e *Engine) Status() model.Status {
	guns := e.store.Guns()
	fragments := e.store.Fragments()
	activeFragments := e.store.ActiveFragments()
	return model.Status{
		Running:              e.IsRunning(),
		GunCount:             len(guns),
		ActiveFragments:      len(activeFragments),
		EventCount:           e.store.EventCount(),
		FragmentSource:       e.sourceName(),
		FragmentSourceHealth: e.sourceHealth(),
		IngestCounters:       e.ingestCounters(),
		PLC:                  e.plc.Health(),
		Safety:               e.safety.Health(),
		History:              e.history.Health(),
		Guns:                 guns,
		FragmentList:         fragments,
		ActiveFragmentList:   activeFragments,
		RecentEvents:         e.store.Events(10),
		GeneratedAt:          time.Now().UTC(),
	}
}

func (e *Engine) loop(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	tick := time.NewTicker(e.cfg.Simulation.TickInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			e.Tick(now.UTC())
		}
	}
}

func (e *Engine) SubmitFragment(ctx context.Context, input fragments.Input) (model.Fragment, error) {
	if err := ctx.Err(); err != nil {
		return model.Fragment{}, err
	}
	now := input.ObservedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err := e.validateFragmentInput(input); err != nil {
		return model.Fragment{}, err
	}
	e.mu.Lock()
	e.nextFrag++
	fragmentID := fmt.Sprintf("ULY-%04d", e.nextFrag)
	e.mu.Unlock()
	fragment := model.Fragment{
		ID:                  fragmentID,
		ExternalID:          input.ExternalID,
		Azimuth:             model.NormalizeAzimuth(input.Bearing),
		Elevation:           input.Elevation,
		MapX:                input.MapX,
		MapY:                input.MapY,
		Priority:            input.Priority,
		TerrainType:         input.TerrainType,
		StrategicZone:       input.StrategicZone,
		Status:              model.FragmentActive,
		CreatedAt:           now,
		TimeToImpactSeconds: input.TimeToImpactSeconds,
		ImpactAt:            now.Add(time.Duration(input.TimeToImpactSeconds) * time.Second),
	}
	if err := e.store.SaveFragment(fragment); err != nil {
		return model.Fragment{}, err
	}
	if input.GeneratedByInternalRNG {
		e.record(model.EventTargetGenerated, model.SeverityInfo, nil, &fragment.ID, "Ulysses fragment generated", map[string]interface{}{"azimuth": fragment.Azimuth, "elevation": fragment.Elevation, "priority": fragment.Priority, "terrain_type": fragment.TerrainType, "strategic_zone": fragment.StrategicZone})
	} else {
		e.recordOperational(model.OperationalEvent{
			Timestamp:          time.Now().UTC(),
			EventType:          model.EventFragmentReceived,
			Severity:           model.SeverityInfo,
			FragmentID:         &fragment.ID,
			ExternalFragmentID: stringPtr(fragment.ExternalID),
			Azimuth:            floatPtr(fragment.Azimuth),
			Elevation:          floatPtr(fragment.Elevation),
			Message:            "external fragment received",
			Details: map[string]interface{}{
				"priority":               fragment.Priority,
				"terrain_type":           fragment.TerrainType,
				"strategic_zone":         fragment.StrategicZone,
				"map_x":                  fragment.MapX,
				"map_y":                  fragment.MapY,
				"observed_at":            fragment.CreatedAt,
				"received_at":            time.Now().UTC(),
				"impact_at":              fragment.ImpactAt,
				"time_to_impact_seconds": fragment.TimeToImpactSeconds,
				"status":                 fragment.Status,
			},
		})
	}
	return fragment, nil
}

func (e *Engine) Tick(now time.Time) {
	e.markMisses(now)
	e.advanceGuns(now)
	e.assignTargets(now)
}

func (e *Engine) IngestFragment(ctx context.Context, input fragments.Input) (model.Fragment, error) {
	source, ok := e.source.(*fragments.ExternalIngestSource)
	if !ok {
		err := errors.New("external ingest source is not configured")
		e.recordRejected(input, err.Error())
		return model.Fragment{}, err
	}
	fragment, err := source.Submit(ctx, input)
	if err != nil {
		e.recordRejected(input, err.Error())
	}
	return fragment, err
}

func (e *Engine) validateFragmentInput(input fragments.Input) error {
	if len(input.ExternalID) > maxExternalFragmentIDBytes {
		return fragments.ValidationError("external_id is too long")
	}
	if input.Bearing < 0 || input.Bearing > 359 {
		return fragments.ValidationError("bearing must be between 0 and 359")
	}
	if input.Elevation < e.cfg.Simulation.SoftMinElevation || input.Elevation > e.cfg.Simulation.SoftMaxElevation {
		return fragments.ValidationError(fmt.Sprintf("elevation must be between %.1f and %.1f", e.cfg.Simulation.SoftMinElevation, e.cfg.Simulation.SoftMaxElevation))
	}
	if input.MapX < 0 || input.MapX > 100 || input.MapY < 0 || input.MapY > 100 {
		return fragments.ValidationError("map_x and map_y must be between 0 and 100")
	}
	if !validPriority(input.Priority) {
		return fragments.ValidationError("priority must be LOW, MEDIUM, HIGH, or CRITICAL")
	}
	if input.TimeToImpactSeconds <= 0 {
		return fragments.ValidationError("time_to_impact_seconds must be positive")
	}
	maxTTI := e.cfg.Fragments.Ingest.MaxTimeToImpactSeconds
	if input.GeneratedByInternalRNG {
		maxTTI = e.cfg.Simulation.TargetTimeToImpactMaxSeconds
	}
	if maxTTI > 0 && input.TimeToImpactSeconds > maxTTI {
		return fragments.ValidationError(fmt.Sprintf("time_to_impact_seconds must be <= %d", maxTTI))
	}
	if input.TerrainType == "" {
		return fragments.ValidationError("terrain_type is required")
	}
	if len(input.TerrainType) > maxFragmentLabelBytes {
		return fragments.ValidationError("terrain_type is too long")
	}
	if input.StrategicZone == "" {
		return fragments.ValidationError("strategic_zone is required")
	}
	if len(input.StrategicZone) > maxFragmentLabelBytes {
		return fragments.ValidationError("strategic_zone is too long")
	}
	return nil
}

func validPriority(priority model.Priority) bool {
	switch priority {
	case model.PriorityLow, model.PriorityMedium, model.PriorityHigh, model.PriorityCritical:
		return true
	default:
		return false
	}
}

func (e *Engine) recordRejected(input fragments.Input, reason string) {
	e.recordOperational(model.OperationalEvent{
		Timestamp:          time.Now().UTC(),
		EventType:          model.EventFragmentRejected,
		Severity:           model.SeverityWarning,
		ExternalFragmentID: stringPtr(truncateUTF8(input.ExternalID, maxExternalFragmentIDBytes)),
		Azimuth:            floatPtr(input.Bearing),
		Elevation:          floatPtr(input.Elevation),
		Message:            truncateUTF8(reason, maxRejectedReasonBytes),
		Details: map[string]interface{}{
			"terrain_type":   truncateUTF8(input.TerrainType, maxFragmentLabelBytes),
			"strategic_zone": truncateUTF8(input.StrategicZone, maxFragmentLabelBytes),
		},
	})
}

func truncateUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	end := maxBytes
	for !utf8.ValidString(value[:end]) {
		end--
	}
	return value[:end]
}

func (e *Engine) markMisses(now time.Time) {
	for _, fragment := range e.store.ActiveFragments() {
		if now.Before(fragment.ImpactAt) {
			continue
		}
		fragment.Status = model.FragmentMissed
		fragment.MissedAt = &now
		if fragment.AssignedGunID != nil {
			if gun, ok := e.store.Gun(*fragment.AssignedGunID); ok && gun.AssignedFragmentID != nil && *gun.AssignedFragmentID == fragment.ID {
				gun.AssignedFragmentID = nil
				gun.State = model.GunIdle
				gun.LastIdleTime = now
				gun.EventSummary = "assigned fragment missed impact window"
				e.saveGun(gun)
			}
		}
		e.store.SaveFragment(fragment)
		e.record(model.EventFragmentMissed, model.SeverityWarning, fragment.AssignedGunID, &fragment.ID, "fragment missed intercept window", nil)
	}
}

func (e *Engine) advanceGuns(now time.Time) {
	for _, gun := range e.store.Guns() {
		switch gun.State {
		case model.GunTargetAssigned:
			e.startSlew(gun, now)
		case model.GunSlewing:
			if gun.SlewCompleteAt != nil && !now.Before(*gun.SlewCompleteAt) {
				gun.CurrentAzimuth = gun.TargetAzimuth
				gun.CurrentElevation = gun.TargetElevation
				until := now.Add(e.cfg.Simulation.SettlingDelay)
				gun.SettlingUntil = &until
				gun.State = model.GunOnTargetSettling
				gun.EventSummary = "on target; settling"
				e.saveGun(gun)
				e.record(model.EventGunOnTarget, model.SeverityInfo, &gun.ID, gun.AssignedFragmentID, "gun on target; settling delay started", map[string]interface{}{"settling_until": until})
			}
		case model.GunOnTargetSettling:
			if gun.SettlingUntil != nil && !now.Before(*gun.SettlingUntil) {
				gun.State = model.GunReadyToFire
				gun.EventSummary = "ready to fire"
				e.saveGun(gun)
			}
		case model.GunReadyToFire:
			_, _ = e.FireRequest(gun.ID)
		case model.GunFiring:
			e.completeFire(gun, now)
		case model.GunPostFireLockout:
			if gun.PostFireLockoutUntil != nil && !now.Before(*gun.PostFireLockoutUntil) {
				gun.State = model.GunIdle
				gun.PostFireLockoutUntil = nil
				gun.LastIdleTime = now
				gun.EventSummary = "ready"
				e.saveGun(gun)
			}
		}
	}
}

func (e *Engine) startSlew(gun model.GunState, now time.Time) {
	if gun.PostFireLockoutUntil != nil && now.Before(*gun.PostFireLockoutUntil) {
		gun.State = model.GunPostFireLockout
		e.saveGun(gun)
		return
	}
	duration := e.SlewDuration(gun.CurrentAzimuth, gun.CurrentElevation, gun.TargetAzimuth, gun.TargetElevation)
	completeAt := now.Add(duration)
	gun.SlewCompleteAt = &completeAt
	gun.State = model.GunSlewing
	gun.EventSummary = "slewing to assigned solution"
	e.saveGun(gun)
	e.record(model.EventGunSlewStarted, model.SeverityInfo, &gun.ID, gun.AssignedFragmentID, "gun slew started", map[string]interface{}{"duration_seconds": duration.Seconds()})
}

func (e *Engine) SlewDuration(currentAz, currentEl, targetAz, targetEl float64) time.Duration {
	azDelta := model.AngularDelta(currentAz, targetAz)
	elDelta := math.Abs(targetEl - currentEl)
	degrees := math.Max(azDelta, elDelta)
	seconds := degrees / e.cfg.Simulation.SlewDegreesPerSecond
	duration := time.Duration(seconds * float64(time.Second))
	if duration < e.cfg.Simulation.MinSlewDuration {
		duration = e.cfg.Simulation.MinSlewDuration
	}
	if duration > e.cfg.Simulation.MaxSlewDuration {
		duration = e.cfg.Simulation.MaxSlewDuration
	}
	return duration
}

func (e *Engine) assignTargets(now time.Time) {
	fragments := e.store.ActiveFragments()
	sort.SliceStable(fragments, func(i, j int) bool {
		if priorityRank(fragments[i].Priority) == priorityRank(fragments[j].Priority) {
			return fragments[i].ImpactAt.Before(fragments[j].ImpactAt)
		}
		return priorityRank(fragments[i].Priority) > priorityRank(fragments[j].Priority)
	})
	for _, fragment := range fragments {
		if fragment.Status != model.FragmentActive {
			continue
		}
		gun, ok := SelectGunForFragment(e.store.Guns(), fragment, now)
		if !ok {
			continue
		}
		fragment.Status = model.FragmentAssigned
		fragment.AssignedGunID = &gun.ID
		e.store.SaveFragment(fragment)
		id := fragment.ID
		gun.AssignedFragmentID = &id
		gun.TargetAzimuth = fragment.Azimuth
		gun.TargetElevation = fragment.Elevation
		gun.State = model.GunTargetAssigned
		gun.EventSummary = "target assigned"
		gun.SafetyFaultReason = ""
		_ = e.plc.WriteGunSetpoint(gun.ID, gun.TargetAzimuth, gun.TargetElevation)
		e.saveGun(gun)
		e.record(model.EventTargetAssigned, model.SeverityInfo, &gun.ID, &fragment.ID, "target assigned to gun", map[string]interface{}{"azimuth": fragment.Azimuth, "elevation": fragment.Elevation, "priority": fragment.Priority, "terrain_type": fragment.TerrainType, "strategic_zone": fragment.StrategicZone})
	}
}

func SelectGunForFragment(guns []model.GunState, fragment model.Fragment, now time.Time) (model.GunState, bool) {
	eligible := make([]model.GunState, 0)
	for _, gun := range guns {
		if !IsGunEligible(gun, fragment, now) {
			continue
		}
		eligible = append(eligible, gun)
	}
	if len(eligible) == 0 {
		return model.GunState{}, false
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		leftSlew := model.AngularDelta(eligible[i].CurrentAzimuth, fragment.Azimuth)
		rightSlew := model.AngularDelta(eligible[j].CurrentAzimuth, fragment.Azimuth)
		if math.Abs(leftSlew-rightSlew) > 0.0001 {
			return leftSlew < rightSlew
		}
		if !eligible[i].LastIdleTime.Equal(eligible[j].LastIdleTime) {
			return eligible[i].LastIdleTime.Before(eligible[j].LastIdleTime)
		}
		return eligible[i].ID < eligible[j].ID
	})
	return eligible[0], true
}

func IsGunEligible(gun model.GunState, fragment model.Fragment, now time.Time) bool {
	if gun.Mode != model.ModeAuto {
		return false
	}
	if gun.State != model.GunIdle && gun.State != model.GunReadyToFire {
		return false
	}
	if gun.FaultCode != "" || gun.State == model.GunFaulted {
		return false
	}
	if gun.PostFireLockoutUntil != nil && now.Before(*gun.PostFireLockoutUntil) {
		return false
	}
	if gun.AssignedFragmentID != nil {
		return false
	}
	if !model.SectorCovers(gun.SectorStart, gun.SectorEnd, fragment.Azimuth) {
		return false
	}
	return true
}

func (e *Engine) SetMode(id int, mode model.GunMode) (model.GunState, error) {
	if mode != model.ModeAuto && mode != model.ModeManual && mode != model.ModeMaintenance {
		return model.GunState{}, fmt.Errorf("invalid mode %q", mode)
	}
	gun, ok := e.store.Gun(id)
	if !ok {
		return model.GunState{}, ErrNotFound
	}
	gun.Mode = mode
	switch mode {
	case model.ModeAuto:
		gun.MaintenanceEnabled = false
		gun.MaintenanceRequestedBy = ""
		if gun.State == model.GunManual {
			gun.State = model.GunIdle
			gun.LastIdleTime = time.Now().UTC()
		}
	case model.ModeManual:
		gun.State = model.GunManual
		gun.AssignedFragmentID = nil
		gun.MaintenanceEnabled = false
	case model.ModeMaintenance:
		gun.State = model.GunManual
		gun.MaintenanceEnabled = true
		gun.MaintenanceRequestedBy = "development-api"
		gun.AssignedFragmentID = nil
	}
	_ = e.plc.WriteGunMode(id, string(mode))
	gun.EventSummary = "mode changed"
	e.saveGun(gun)
	e.record(model.EventGunModeChanged, model.SeverityInfo, &gun.ID, nil, "gun mode changed", map[string]interface{}{"mode": mode})
	if mode == model.ModeMaintenance || !gun.MaintenanceEnabled {
		e.record(model.EventGunMaintenanceChanged, model.SeverityInfo, &gun.ID, nil, "gun maintenance state changed", map[string]interface{}{"maintenance_enabled": gun.MaintenanceEnabled, "requested_by": gun.MaintenanceRequestedBy})
	}
	return gun, nil
}

func (e *Engine) SetManualTarget(id int, azimuth, elevation float64) (model.GunState, error) {
	gun, ok := e.store.Gun(id)
	if !ok {
		return model.GunState{}, ErrNotFound
	}
	if gun.Mode != model.ModeManual && gun.Mode != model.ModeMaintenance {
		err := errors.New("manual target requires MANUAL or MAINTENANCE mode")
		e.record(model.EventManualTargetRejected, model.SeverityWarning, &id, nil, err.Error(), nil)
		return gun, err
	}
	if !gun.AdminOverrideActive && (elevation < gun.SoftMinElevation || elevation > gun.SoftMaxElevation) {
		err := fmt.Errorf("elevation %.2f outside soft limits %.2f..%.2f", elevation, gun.SoftMinElevation, gun.SoftMaxElevation)
		gun.EventSummary = "manual target rejected"
		e.saveGun(gun)
		e.record(model.EventManualTargetRejected, model.SeverityWarning, &id, nil, err.Error(), map[string]interface{}{"azimuth": azimuth, "elevation": elevation})
		return gun, err
	}
	gun.TargetAzimuth = model.NormalizeAzimuth(azimuth)
	gun.TargetElevation = elevation
	gun.State = model.GunTargetAssigned
	gun.EventSummary = "manual target accepted"
	_ = e.plc.WriteGunSetpoint(id, gun.TargetAzimuth, gun.TargetElevation)
	e.saveGun(gun)
	e.record(model.EventManualTargetAccepted, model.SeverityInfo, &id, nil, "manual target accepted", map[string]interface{}{"azimuth": gun.TargetAzimuth, "elevation": elevation})
	return gun, nil
}

func (e *Engine) FireRequest(id int) (model.GunState, error) {
	gun, ok := e.store.Gun(id)
	if !ok {
		return model.GunState{}, ErrNotFound
	}
	if gun.State != model.GunReadyToFire {
		return gun, fmt.Errorf("gun %d is %s, not READY_TO_FIRE", id, gun.State)
	}
	fragment := model.Fragment{
		ID:        "MANUAL-FIRE-" + strconv.Itoa(id),
		Azimuth:   gun.TargetAzimuth,
		Elevation: gun.TargetElevation,
		Status:    model.FragmentAssigned,
	}
	if gun.AssignedFragmentID != nil {
		if f, ok := e.store.Fragment(*gun.AssignedFragmentID); ok {
			fragment = f
		}
	}
	e.record(model.EventFireRequested, model.SeverityInfo, &gun.ID, gun.AssignedFragmentID, "fire requested", nil)
	decision := e.safety.EvaluateFirePermissive(gun, fragment)
	e.plc.SetFirePermissive(gun.ID, decision.Allowed, decision.Reason)
	gun.SafetyPermissive = decision.Allowed
	gun.SafetyFaultReason = decision.Reason
	gun.HardLimitStatus = decision.HardLimitStatus
	gun.SpecialMeasuresEnabled = e.safety.GetSpecialMeasuresState().Enabled
	if !decision.Allowed {
		gun.EventSummary = "fire blocked by safety permissive"
		if e.cfg.Simulation.FaultOnSafetyBlocked {
			gun.State = model.GunFaulted
			gun.FaultCode = "SAFETY_PERMISSIVE_BLOCKED"
		}
		e.saveGun(gun)
		e.record(model.EventSafetyPermissiveBlocked, model.SeverityCritical, &gun.ID, gun.AssignedFragmentID, decision.Reason, map[string]interface{}{"hard_limit_status": decision.HardLimitStatus, "special_measures_required": decision.SpecialMeasuresRequired})
		return gun, fmt.Errorf("safety permissive blocked: %s", decision.Reason)
	}
	_ = e.plc.WriteFireRequest(gun.ID)
	gun.State = model.GunFiring
	gun.EventSummary = "firing"
	e.saveGun(gun)
	e.record(model.EventSafetyPermissiveGranted, model.SeverityInfo, &gun.ID, gun.AssignedFragmentID, "safety permissive granted", nil)
	return gun, nil
}

func (e *Engine) completeFire(gun model.GunState, now time.Time) {
	gun.ShotSequence++
	e.recordOperational(model.OperationalEvent{
		Timestamp:    time.Now().UTC(),
		EventType:    model.EventGunFired,
		Severity:     model.SeverityInfo,
		GunID:        &gun.ID,
		FragmentID:   gun.AssignedFragmentID,
		Azimuth:      floatPtr(gun.TargetAzimuth),
		Elevation:    floatPtr(gun.TargetElevation),
		ShellType:    &gun.ShellType,
		ShotSequence: &gun.ShotSequence,
		Message:      "gun fired",
	})
	if gun.AssignedFragmentID != nil {
		if fragment, ok := e.store.Fragment(*gun.AssignedFragmentID); ok {
			fragment.Status = model.FragmentIntercepted
			fragment.InterceptedAt = &now
			e.store.SaveFragment(fragment)
			e.record(model.EventInterceptRecorded, model.SeverityInfo, &gun.ID, &fragment.ID, "fragment intercept recorded", nil)
		}
	}
	lockoutUntil := now.Add(e.cfg.Simulation.PostFireLockout)
	gun.State = model.GunPostFireLockout
	gun.AssignedFragmentID = nil
	gun.LastFireTime = &now
	gun.PostFireLockoutUntil = &lockoutUntil
	gun.SettlingUntil = nil
	gun.SlewCompleteAt = nil
	gun.EventSummary = "post-fire reloading"
	e.saveGun(gun)
}

func (e *Engine) FaultGun(id int, code string) (model.GunState, error) {
	gun, ok := e.store.Gun(id)
	if !ok {
		return model.GunState{}, ErrNotFound
	}
	if code == "" {
		code = "HMI_FAULT_REQUEST"
	}
	gun.State = model.GunFaulted
	gun.FaultCode = code
	gun.EventSummary = "faulted"
	e.saveGun(gun)
	e.record(model.EventGunFaulted, model.SeverityCritical, &gun.ID, gun.AssignedFragmentID, "gun faulted", map[string]interface{}{"fault_code": code})
	return gun, nil
}

func (e *Engine) ClearFault(id int) (model.GunState, error) {
	gun, ok := e.store.Gun(id)
	if !ok {
		return model.GunState{}, ErrNotFound
	}
	gun.FaultCode = ""
	gun.SafetyFaultReason = ""
	gun.HardLimitStatus = "OK"
	if gun.Mode == model.ModeAuto {
		gun.State = model.GunIdle
		gun.LastIdleTime = time.Now().UTC()
	} else if gun.Mode == model.ModeManual {
		gun.State = model.GunManual
	} else {
		gun.State = model.GunManual
	}
	gun.EventSummary = "fault cleared"
	e.saveGun(gun)
	e.record(model.EventFaultCleared, model.SeverityInfo, &gun.ID, nil, "gun fault cleared", nil)
	return gun, nil
}

func (e *Engine) saveGun(gun model.GunState) {
	gun.SpecialMeasuresEnabled = e.safety.GetSpecialMeasuresState().Enabled
	e.store.SaveGun(gun)
	e.plc.UpsertGunState(gun)
}

func (e *Engine) record(eventType model.EventType, severity model.Severity, gunID *int, fragmentID *string, message string, details map[string]interface{}) {
	e.recordOperational(model.OperationalEvent{
		Timestamp:  time.Now().UTC(),
		EventType:  eventType,
		Severity:   severity,
		GunID:      gunID,
		FragmentID: fragmentID,
		Message:    message,
		Details:    details,
	})
}

func (e *Engine) recordOperational(event model.OperationalEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	e.enrichEvent(&event)
	event = e.store.AddEvent(event)
	if err := e.history.WriteEvent(context.Background(), event); err != nil {
		e.logger.Error("history write failed", "event_type", event.EventType, "error", err)
	}
	e.logger.Info("sim event", "id", event.ID, "type", event.EventType, "severity", event.Severity, "gun_id", event.GunID, "fragment_id", event.FragmentID, "message", event.Message)
}

func (e *Engine) enrichEvent(event *model.OperationalEvent) {
	if event.Details == nil {
		event.Details = make(map[string]interface{})
	}
	if event.FragmentID == nil {
		return
	}
	fragment, ok := e.store.Fragment(*event.FragmentID)
	if !ok {
		return
	}
	if event.ExternalFragmentID == nil && fragment.ExternalID != "" {
		event.ExternalFragmentID = &fragment.ExternalID
	}
	if event.Azimuth == nil {
		event.Azimuth = &fragment.Azimuth
	}
	if event.Elevation == nil {
		event.Elevation = &fragment.Elevation
	}
	addDetailDefault(event.Details, "priority", fragment.Priority)
	addDetailDefault(event.Details, "terrain_type", fragment.TerrainType)
	addDetailDefault(event.Details, "strategic_zone", fragment.StrategicZone)
	addDetailDefault(event.Details, "map_x", fragment.MapX)
	addDetailDefault(event.Details, "map_y", fragment.MapY)
	addDetailDefault(event.Details, "observed_at", fragment.CreatedAt)
	addDetailDefault(event.Details, "impact_at", fragment.ImpactAt)
	addDetailDefault(event.Details, "status", fragment.Status)
	if fragment.AssignedGunID != nil {
		addDetailDefault(event.Details, "assigned_gun_id", *fragment.AssignedGunID)
	}
	if fragment.InterceptedAt != nil {
		addDetailDefault(event.Details, "intercepted_at", *fragment.InterceptedAt)
	}
	if fragment.MissedAt != nil {
		addDetailDefault(event.Details, "missed_at", *fragment.MissedAt)
	}
}

func addDetailDefault(details map[string]interface{}, key string, value interface{}) {
	if _, ok := details[key]; !ok {
		details[key] = value
	}
}

func (e *Engine) sourceName() string {
	if e.source == nil {
		return ""
	}
	return e.source.Name()
}

func (e *Engine) sourceHealth() model.SourceHealth {
	if e.source == nil {
		return model.SourceHealth{}
	}
	return e.source.Health()
}

func (e *Engine) ingestCounters() model.IngestCounters {
	source, ok := e.source.(*fragments.ExternalIngestSource)
	if !ok {
		return model.IngestCounters{}
	}
	return source.Counters()
}

func (e *Engine) SetAdminOverride(id int, active bool, approvedBy string, expiresAt *time.Time) (model.GunState, error) {
	gun, ok := e.store.Gun(id)
	if !ok {
		return model.GunState{}, ErrNotFound
	}
	gun.AdminOverrideActive = active
	gun.AdminOverrideApprovedBy = approvedBy
	gun.AdminOverrideExpiresAt = expiresAt
	if active {
		gun.AdminOverrideState = "ACTIVE_DEVELOPMENT_PLACEHOLDER"
	} else {
		gun.AdminOverrideState = "NOT_REQUESTED"
		gun.AdminOverrideApprovedBy = ""
		gun.AdminOverrideExpiresAt = nil
	}
	gun.EventSummary = "admin override state changed"
	e.saveGun(gun)
	e.record(model.EventAdminOverrideChanged, model.SeverityWarning, &gun.ID, nil, "admin override state changed", map[string]interface{}{"active": active, "approved_by": approvedBy, "expires_at": expiresAt})
	return gun, nil
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func floatPtr(value float64) *float64 {
	return &value
}

func priorityRank(p model.Priority) int {
	switch p {
	case model.PriorityCritical:
		return 4
	case model.PriorityHigh:
		return 3
	case model.PriorityMedium:
		return 2
	default:
		return 1
	}
}

var ErrNotFound = errors.New("not found")
