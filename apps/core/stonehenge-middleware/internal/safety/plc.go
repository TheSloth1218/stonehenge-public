package safety

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/goburrow/modbus"

	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/plc"
)

const (
	StatusRegisterCount = 80
	ControlRegisterMax  = 47
	SupportedSchema     = 1
	resetReleaseTimeout = 2 * time.Second
)

type State uint16

const (
	StateOffline State = iota
	StateInitializing
	StateReady
	StateInhibited
	StateEmergencyStop
	StateMaintenance
	StateFault
)

type GlobalInhibitReason uint16

const (
	GlobalInhibitNone GlobalInhibitReason = iota
	GlobalInhibitDisabled
	GlobalInhibitEmergencyStop
	GlobalInhibitMaintenanceMode
	GlobalInhibitSafetyFault
	GlobalInhibitInitializing
	GlobalInhibitResetRequired
)

const GlobalInhibitUnknown GlobalInhibitReason = 255

type GunInhibitReason uint16

const (
	GunInhibitNone GunInhibitReason = iota
	GunInhibitGlobal
	GunInhibitDisabled
	GunInhibitMaintenance
	GunInhibitExternal
	GunInhibitSafetyFault
	GunInhibitResetRequired
)

const GunInhibitUnknown GunInhibitReason = 255

type FaultCode uint16

const (
	FaultNone FaultCode = iota
	FaultInvalidConfiguration
	FaultResetRequired
	FaultInternalState
	FaultHeartbeatTiming
	FaultCriticalInternal
)

const FaultUnknown FaultCode = 255

type DecisionReason uint16

const (
	DecisionAllowed DecisionReason = iota
	DecisionEnforcementDisabled
	DecisionNotConfigured
	DecisionDisconnected
	DecisionStale
	DecisionUnsupportedSchema
	DecisionNotReady
	DecisionSafetyFault
	DecisionGlobalInhibit
	DecisionGunInhibit
	DecisionGunNotFound
)

type GunStatus struct {
	GunID                    int              `json:"gun_id"`
	FirePermissive           bool             `json:"fire_permissive"`
	InhibitActive            bool             `json:"inhibit_active"`
	InhibitReason            GunInhibitReason `json:"inhibit_reason"`
	InhibitReasonName        string           `json:"inhibit_reason_name"`
	EnabledStatus            bool             `json:"enabled_status"`
	MaintenanceInhibitStatus bool             `json:"maintenance_inhibit_status"`
	ExternalInhibitStatus    bool             `json:"external_inhibit_status"`
}

type Snapshot struct {
	Enabled                 bool                `json:"enabled"`
	EnforcementEnabled      bool                `json:"enforcement_enabled"`
	Configured              bool                `json:"configured"`
	Connected               bool                `json:"connected"`
	Fresh                   bool                `json:"fresh"`
	SchemaVersion           uint16              `json:"schema_version"`
	SchemaSupported         bool                `json:"schema_supported"`
	HeartbeatCounter        uint16              `json:"heartbeat_counter"`
	State                   State               `json:"state_code"`
	StateName               string              `json:"state"`
	Ready                   bool                `json:"ready"`
	GlobalFirePermissive    bool                `json:"global_fire_permissive"`
	GlobalInhibitActive     bool                `json:"global_inhibit_active"`
	GlobalInhibitReason     GlobalInhibitReason `json:"global_inhibit_reason"`
	GlobalInhibitReasonName string              `json:"global_inhibit_reason_name"`
	FaultActive             bool                `json:"safety_fault_active"`
	FaultCode               FaultCode           `json:"safety_fault_code"`
	FaultName               string              `json:"safety_fault_name"`
	ResetRequired           bool                `json:"reset_required"`
	EmergencyStopActive     bool                `json:"emergency_stop_active"`
	MaintenanceModeActive   bool                `json:"maintenance_mode_active"`
	Guns                    []GunStatus         `json:"guns"`
	LastSuccessfulReadAt    *time.Time          `json:"last_successful_read_at,omitempty"`
	LastHeartbeatChangeAt   *time.Time          `json:"last_heartbeat_change_at,omitempty"`
	LastError               string              `json:"last_error,omitempty"`
	LastErrorCode           uint16              `json:"last_error_code"`
	Age                     time.Duration       `json:"-"`
	AgeSeconds              uint32              `json:"age_seconds"`
	RawStatusRegisters      []uint16            `json:"raw_status_registers,omitempty"`
	Extra                   map[string]any      `json:"extra,omitempty"`
}

type Decision struct {
	Allowed             bool                `json:"allowed"`
	ReasonCode          DecisionReason      `json:"reason_code"`
	Reason              string              `json:"reason"`
	GlobalInhibitReason GlobalInhibitReason `json:"global_inhibit_reason"`
	GunInhibitReason    GunInhibitReason    `json:"gun_inhibit_reason"`
	SnapshotAge         time.Duration       `json:"-"`
	SnapshotAgeSeconds  uint32              `json:"snapshot_age_seconds"`
}

type EventAuditor interface {
	RecordSafetyEvent(ctx context.Context, event Event) error
}

type Event struct {
	EventTime time.Time      `json:"event_time"`
	EventType string         `json:"event_type"`
	GunID     *int           `json:"gun_id,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

type Transport interface {
	ReadInputRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error)
	ReadHoldingRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error)
	WriteSingleRegister(ctx context.Context, address, value uint16) error
}

type ModbusTransport struct {
	cfg config.SafetyPLCConfig
}

func NewModbusTransport(cfg config.SafetyPLCConfig) *ModbusTransport {
	return &ModbusTransport{cfg: cfg}
}

func (t *ModbusTransport) client() modbus.Client {
	handler := modbus.NewTCPClientHandler(fmt.Sprintf("%s:%d", t.cfg.Host, t.cfg.Port))
	handler.SlaveId = t.cfg.UnitID
	handler.Timeout = t.cfg.PollInterval
	if handler.Timeout <= 0 {
		handler.Timeout = time.Second
	}
	return modbus.NewClient(handler)
}

func (t *ModbusTransport) ReadInputRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := t.client().ReadInputRegisters(address, quantity)
	return decodeWords(data, quantity, err)
}

func (t *ModbusTransport) ReadHoldingRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := t.client().ReadHoldingRegisters(address, quantity)
	return decodeWords(data, quantity, err)
}

func (t *ModbusTransport) WriteSingleRegister(ctx context.Context, address, value uint16) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := t.client().WriteSingleRegister(address, value)
	return err
}

func decodeWords(data []byte, quantity uint16, err error) ([]uint16, error) {
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

type Service struct {
	cfg       config.SafetyPLCConfig
	transport Transport
	auditor   EventAuditor
	logger    *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu            sync.RWMutex
	snapshot      Snapshot
	lastHeartbeat uint16
	haveHeartbeat bool
	lastConnected bool
	lastFresh     bool
}

func NewService(cfg config.SafetyPLCConfig, auditor EventAuditor, logger *slog.Logger) *Service {
	return NewServiceWithTransport(cfg, NewModbusTransport(cfg), auditor, logger)
}

func NewServiceWithTransport(cfg config.SafetyPLCConfig, transport Transport, auditor EventAuditor, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		cfg:       cfg,
		transport: transport,
		auditor:   auditor,
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		snapshot: Snapshot{
			Enabled:            cfg.Enabled,
			EnforcementEnabled: cfg.Enforcement,
			Configured:         cfg.Enabled,
			State:              StateOffline,
			StateName:          StateOffline.Name(),
			SchemaSupported:    false,
			LastErrorCode:      ErrorPLCUnreachable,
		},
	}
	return s
}

func (s *Service) Start() {
	if s == nil || !s.cfg.Enabled {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.loop()
	}()
}

func (s *Service) Close() {
	if s == nil {
		return
	}
	s.cancel()
	s.wg.Wait()
}

func (s *Service) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{Enabled: false, Configured: false, State: StateOffline, StateName: StateOffline.Name(), LastErrorCode: ErrorControllerUnavailable}
	}
	s.updateFreshness()
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSnapshot(s.snapshot)
}

func (s *Service) CanFire(gunID int) Decision {
	snap := s.Snapshot()
	return EvaluateFireDecision(snap, gunID)
}

func (s *Service) SetGlobal(ctx context.Context, req GlobalControl) error {
	if err := s.controlAvailable(); err != nil {
		return err
	}
	writes := []controlWrite{}
	if req.GlobalEnable != nil {
		writes = append(writes, controlWrite{logicalAddress: 1, value: boolWord(*req.GlobalEnable)})
	}
	if req.EmergencyStop != nil {
		writes = append(writes, controlWrite{logicalAddress: 2, value: boolWord(*req.EmergencyStop)})
	}
	if req.GlobalMaintenanceMode != nil {
		writes = append(writes, controlWrite{logicalAddress: 3, value: boolWord(*req.GlobalMaintenanceMode)})
	}
	if req.TestFaultRequest != nil {
		writes = append(writes, controlWrite{logicalAddress: 5, value: boolWord(*req.TestFaultRequest)})
	}
	for _, write := range writes {
		if err := s.writeControlRegister(ctx, write.logicalAddress, write.value); err != nil {
			return err
		}
	}
	s.recordEvent("control_global_updated", nil, map[string]any{"writes": s.controlWriteDetails(writes)})
	return nil
}

func (s *Service) SetGun(ctx context.Context, gunID int, req GunControl) error {
	if gunID < 1 || gunID > 8 {
		return fmt.Errorf("gun_id must be between 1 and 8")
	}
	if err := s.controlAvailable(); err != nil {
		return err
	}
	base := uint16(16 + (gunID-1)*4)
	writes := []controlWrite{}
	if req.Enabled != nil {
		writes = append(writes, controlWrite{logicalAddress: base, value: boolWord(*req.Enabled)})
	}
	if req.MaintenanceInhibit != nil {
		writes = append(writes, controlWrite{logicalAddress: base + 1, value: boolWord(*req.MaintenanceInhibit)})
	}
	if req.ExternalInhibit != nil {
		writes = append(writes, controlWrite{logicalAddress: base + 2, value: boolWord(*req.ExternalInhibit)})
	}
	for _, write := range writes {
		if err := s.writeControlRegister(ctx, write.logicalAddress, write.value); err != nil {
			return err
		}
	}
	s.recordEvent("control_gun_updated", &gunID, map[string]any{"writes": s.controlWriteDetails(writes)})
	return nil
}

func (s *Service) PulseReset(ctx context.Context) error {
	if err := s.controlAvailable(); err != nil {
		return err
	}
	if err := s.writeControlRegister(ctx, 4, 1); err != nil {
		return err
	}
	timer := time.NewTimer(s.cfg.ResetPulseDuration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		if err := s.releaseResetAfterCancel(); err != nil {
			return fmt.Errorf("reset pulse canceled after assert: %w; release failed: %v", ctx.Err(), err)
		}
		return ctx.Err()
	case <-timer.C:
	}
	if err := s.writeControlRegister(ctx, 4, 0); err != nil {
		if ctx.Err() != nil {
			if releaseErr := s.releaseResetAfterCancel(); releaseErr != nil {
				return fmt.Errorf("reset pulse release failed after cancellation: %w; cleanup release failed: %v", err, releaseErr)
			}
		}
		return err
	}
	s.recordEvent("reset_requested", nil, map[string]any{"pulse_duration": s.cfg.ResetPulseDuration.String()})
	return nil
}

func (s *Service) releaseResetAfterCancel() error {
	releaseCtx, cancel := context.WithTimeout(context.Background(), resetReleaseTimeout)
	defer cancel()
	return s.writeControlRegister(releaseCtx, 4, 0)
}

type controlWrite struct {
	logicalAddress uint16
	value          uint16
}

func (s *Service) writeControlRegister(ctx context.Context, logicalAddress uint16, value uint16) error {
	if logicalAddress > ControlRegisterMax {
		return fmt.Errorf("safety control register %d is outside supported range 0..%d", logicalAddress, ControlRegisterMax)
	}
	return s.transport.WriteSingleRegister(ctx, safetyControlRawAddress(s.cfg.MWRegisterBase, logicalAddress), value)
}

func safetyControlRawAddress(base uint16, logicalAddress uint16) uint16 {
	return base + logicalAddress
}

func (s *Service) controlRawAddress(logicalAddress uint16) uint16 {
	return safetyControlRawAddress(s.cfg.MWRegisterBase, logicalAddress)
}

func (s *Service) controlWriteDetails(writes []controlWrite) []map[string]uint16 {
	details := make([]map[string]uint16, 0, len(writes))
	for _, write := range writes {
		details = append(details, map[string]uint16{
			"logical_address": write.logicalAddress,
			"raw_address":     s.controlRawAddress(write.logicalAddress),
			"value":           write.value,
		})
	}
	return details
}

func (s *Service) controlAvailable() error {
	snap := s.Snapshot()
	if !snap.Connected {
		return fmt.Errorf("safety plc unavailable: disconnected")
	}
	if !snap.Fresh {
		return fmt.Errorf("safety plc unavailable: status stale")
	}
	if !snap.SchemaSupported {
		return fmt.Errorf("safety plc unavailable: unsupported schema %d", snap.SchemaVersion)
	}
	return nil
}

type GlobalControl struct {
	GlobalEnable          *bool `json:"global_enable,omitempty"`
	EmergencyStop         *bool `json:"emergency_stop,omitempty"`
	GlobalMaintenanceMode *bool `json:"maintenance_mode,omitempty"`
	TestFaultRequest      *bool `json:"test_fault_request,omitempty"`
}

type GunControl struct {
	Enabled            *bool `json:"enabled,omitempty"`
	MaintenanceInhibit *bool `json:"maintenance_inhibit,omitempty"`
	ExternalInhibit    *bool `json:"external_inhibit,omitempty"`
}

func (s *Service) loop() {
	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()
	s.pollOnce()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.pollOnce()
		}
	}
}

func (s *Service) pollOnce() {
	ctx, cancel := context.WithTimeout(s.ctx, s.cfg.PollInterval)
	defer cancel()
	var (
		words []uint16
		err   error
	)
	if s.cfg.StatusRegisterType == "holding" {
		words, err = s.transport.ReadHoldingRegisters(ctx, 0, StatusRegisterCount)
	} else {
		words, err = s.transport.ReadInputRegisters(ctx, 0, StatusRegisterCount)
	}
	now := time.Now().UTC()
	if err != nil {
		s.setPollError(now, err)
		return
	}
	snap, err := DecodeSnapshot(words, s.cfg, now)
	if err != nil {
		s.setPollError(now, err)
		return
	}
	snap.Connected = true
	snap.Fresh = true
	snap.LastSuccessfulReadAt = &now
	if !s.haveHeartbeat || snap.HeartbeatCounter != s.lastHeartbeat {
		s.lastHeartbeat = snap.HeartbeatCounter
		s.haveHeartbeat = true
		snap.LastHeartbeatChangeAt = &now
	} else {
		prev := s.Snapshot()
		snap.LastHeartbeatChangeAt = prev.LastHeartbeatChangeAt
	}
	s.mu.Lock()
	wasConnected := s.lastConnected
	s.snapshot = snap
	s.lastConnected = true
	s.lastFresh = true
	s.mu.Unlock()
	if !wasConnected {
		s.logger.Info("safety plc connected", "host", s.cfg.Host, "port", s.cfg.Port)
		s.recordEvent("connected", nil, nil)
	}
}

func (s *Service) setPollError(now time.Time, err error) {
	s.mu.Lock()
	wasConnected := s.lastConnected
	s.snapshot.Enabled = s.cfg.Enabled
	s.snapshot.EnforcementEnabled = s.cfg.Enforcement
	s.snapshot.Configured = s.cfg.Enabled
	s.snapshot.Connected = false
	s.snapshot.Fresh = false
	s.snapshot.State = StateOffline
	s.snapshot.StateName = StateOffline.Name()
	s.snapshot.LastError = err.Error()
	s.snapshot.LastErrorCode = ErrorPLCUnreachable
	s.snapshot.AgeSeconds = 0
	s.lastConnected = false
	s.lastFresh = false
	s.mu.Unlock()
	if wasConnected {
		s.logger.Warn("safety plc disconnected", "error", err)
		s.recordEvent("disconnected", nil, map[string]any{"error": err.Error(), "time": now})
	}
}

func (s *Service) updateFreshness() {
	if s == nil {
		return
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snapshot.LastSuccessfulReadAt == nil {
		s.snapshot.Fresh = false
		return
	}
	age := now.Sub(*s.snapshot.LastSuccessfulReadAt)
	s.snapshot.Age = age
	s.snapshot.AgeSeconds = uint32(age.Seconds())
	heartbeatFresh := true
	if s.snapshot.LastHeartbeatChangeAt == nil {
		heartbeatFresh = false
	} else {
		heartbeatFresh = now.Sub(*s.snapshot.LastHeartbeatChangeAt) <= s.cfg.StaleAfter
	}
	fresh := age <= s.cfg.StaleAfter && heartbeatFresh
	if s.snapshot.Connected && s.lastFresh && !fresh {
		s.logger.Warn("safety plc status became stale", "last_successful_read_at", s.snapshot.LastSuccessfulReadAt, "last_heartbeat_change_at", s.snapshot.LastHeartbeatChangeAt)
		s.recordEvent("stale", nil, nil)
	}
	if s.snapshot.Connected && !s.lastFresh && fresh {
		s.logger.Info("safety plc status recovered", "last_successful_read_at", s.snapshot.LastSuccessfulReadAt)
		s.recordEvent("recovered", nil, nil)
	}
	s.snapshot.Fresh = fresh
	if !fresh {
		s.snapshot.LastErrorCode = ErrorStale
		if age > s.cfg.StaleAfter {
			s.snapshot.LastError = "safety plc status stale"
		} else if !heartbeatFresh {
			s.snapshot.LastError = "safety plc heartbeat stale"
		}
	} else if s.snapshot.LastErrorCode == ErrorStale {
		s.snapshot.LastErrorCode = ErrorNone
		s.snapshot.LastError = ""
	}
	s.lastFresh = fresh
}

func DecodeSnapshot(words []uint16, cfg config.SafetyPLCConfig, now time.Time) (Snapshot, error) {
	if len(words) < StatusRegisterCount {
		return Snapshot{}, fmt.Errorf("safety plc status read returned %d words, want %d", len(words), StatusRegisterCount)
	}
	schema := words[0]
	snap := Snapshot{
		Enabled:               cfg.Enabled,
		EnforcementEnabled:    cfg.Enforcement,
		Configured:            cfg.Enabled,
		SchemaVersion:         schema,
		SchemaSupported:       schema == SupportedSchema,
		HeartbeatCounter:      words[1],
		State:                 State(words[2]),
		StateName:             State(words[2]).Name(),
		Ready:                 wordBool(words[3]),
		GlobalFirePermissive:  wordBool(words[4]),
		GlobalInhibitActive:   wordBool(words[5]),
		GlobalInhibitReason:   GlobalInhibitReason(words[6]),
		FaultActive:           wordBool(words[7]),
		FaultCode:             FaultCode(words[8]),
		ResetRequired:         wordBool(words[9]),
		EmergencyStopActive:   wordBool(words[10]),
		MaintenanceModeActive: wordBool(words[11]),
		RawStatusRegisters:    append([]uint16(nil), words[:StatusRegisterCount]...),
		LastErrorCode:         ErrorNone,
	}
	snap.GlobalInhibitReasonName = snap.GlobalInhibitReason.Name()
	snap.FaultName = snap.FaultCode.Name()
	snap.Guns = make([]GunStatus, 0, 8)
	for gunID := 1; gunID <= 8; gunID++ {
		base := 16 + (gunID-1)*8
		reason := GunInhibitReason(words[base+2])
		snap.Guns = append(snap.Guns, GunStatus{
			GunID:                    gunID,
			FirePermissive:           wordBool(words[base]),
			InhibitActive:            wordBool(words[base+1]),
			InhibitReason:            reason,
			InhibitReasonName:        reason.Name(),
			EnabledStatus:            wordBool(words[base+3]),
			MaintenanceInhibitStatus: wordBool(words[base+4]),
			ExternalInhibitStatus:    wordBool(words[base+5]),
		})
	}
	if !snap.SchemaSupported {
		snap.LastErrorCode = ErrorUnsupportedSchema
	}
	_ = now
	return snap, nil
}

func EvaluateFireDecision(snap Snapshot, gunID int) Decision {
	decision := Decision{
		Allowed:             false,
		GlobalInhibitReason: snap.GlobalInhibitReason,
		SnapshotAge:         snap.Age,
		SnapshotAgeSeconds:  snap.AgeSeconds,
	}
	if !snap.EnforcementEnabled {
		decision.Allowed = true
		decision.ReasonCode = DecisionEnforcementDisabled
		decision.Reason = "safety plc enforcement disabled"
		return decision
	}
	if !snap.Configured {
		decision.ReasonCode = DecisionNotConfigured
		decision.Reason = "safety plc not configured"
		return decision
	}
	if !snap.Connected {
		decision.ReasonCode = DecisionDisconnected
		decision.Reason = "safety plc disconnected"
		return decision
	}
	if !snap.Fresh {
		decision.ReasonCode = DecisionStale
		decision.Reason = "safety plc status stale"
		return decision
	}
	if !snap.SchemaSupported {
		decision.ReasonCode = DecisionUnsupportedSchema
		decision.Reason = "safety plc schema unsupported"
		return decision
	}
	if !snap.Ready {
		decision.ReasonCode = DecisionNotReady
		decision.Reason = "safety plc not ready"
		return decision
	}
	if snap.FaultActive {
		decision.ReasonCode = DecisionSafetyFault
		decision.Reason = "safety plc fault active"
		return decision
	}
	if !snap.GlobalFirePermissive || snap.GlobalInhibitActive {
		decision.ReasonCode = DecisionGlobalInhibit
		decision.Reason = "global safety inhibit active"
		return decision
	}
	for _, gun := range snap.Guns {
		if gun.GunID != gunID {
			continue
		}
		decision.GunInhibitReason = gun.InhibitReason
		if !gun.FirePermissive || gun.InhibitActive {
			decision.ReasonCode = DecisionGunInhibit
			decision.Reason = "gun safety inhibit active"
			return decision
		}
		decision.Allowed = true
		decision.ReasonCode = DecisionAllowed
		decision.Reason = "safety permissive granted"
		return decision
	}
	decision.ReasonCode = DecisionGunNotFound
	decision.Reason = "gun safety block not found"
	return decision
}

func (s *Service) recordEvent(eventType string, gunID *int, details map[string]any) {
	if s == nil || s.auditor == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.auditor.RecordSafetyEvent(ctx, Event{EventTime: time.Now().UTC(), EventType: eventType, GunID: gunID, Details: details}); err != nil {
		s.logger.Warn("safety event audit write failed", "event_type", eventType, "error", err)
	}
}

func cloneSnapshot(in Snapshot) Snapshot {
	out := in
	out.Guns = append([]GunStatus(nil), in.Guns...)
	out.RawStatusRegisters = append([]uint16(nil), in.RawStatusRegisters...)
	if in.LastSuccessfulReadAt != nil {
		t := *in.LastSuccessfulReadAt
		out.LastSuccessfulReadAt = &t
	}
	if in.LastHeartbeatChangeAt != nil {
		t := *in.LastHeartbeatChangeAt
		out.LastHeartbeatChangeAt = &t
	}
	return out
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

func (s State) Name() string {
	switch s {
	case StateOffline:
		return "offline"
	case StateInitializing:
		return "initializing"
	case StateReady:
		return "ready"
	case StateInhibited:
		return "inhibited"
	case StateEmergencyStop:
		return "emergency_stop"
	case StateMaintenance:
		return "maintenance"
	case StateFault:
		return "fault"
	default:
		return "unknown"
	}
}

func (r GlobalInhibitReason) Name() string {
	switch r {
	case GlobalInhibitNone:
		return "none"
	case GlobalInhibitDisabled:
		return "global_disabled"
	case GlobalInhibitEmergencyStop:
		return "emergency_stop_active"
	case GlobalInhibitMaintenanceMode:
		return "global_maintenance_mode"
	case GlobalInhibitSafetyFault:
		return "safety_fault_active"
	case GlobalInhibitInitializing:
		return "initializing"
	case GlobalInhibitResetRequired:
		return "reset_required"
	default:
		return "unknown"
	}
}

func (r GunInhibitReason) Name() string {
	switch r {
	case GunInhibitNone:
		return "none"
	case GunInhibitGlobal:
		return "global_inhibit"
	case GunInhibitDisabled:
		return "gun_disabled"
	case GunInhibitMaintenance:
		return "gun_maintenance_inhibit"
	case GunInhibitExternal:
		return "gun_external_inhibit"
	case GunInhibitSafetyFault:
		return "safety_fault"
	case GunInhibitResetRequired:
		return "reset_required"
	default:
		return "unknown"
	}
}

func (c FaultCode) Name() string {
	switch c {
	case FaultNone:
		return "none"
	case FaultInvalidConfiguration:
		return "invalid_configuration"
	case FaultResetRequired:
		return "reset_required"
	case FaultInternalState:
		return "internal_state_error"
	case FaultHeartbeatTiming:
		return "heartbeat_internal_timing_fault"
	case FaultCriticalInternal:
		return "critical_internal_fault"
	default:
		return "unknown"
	}
}

func SplitUint32(value uint32) (low, high uint16) {
	return plc.SplitUint32(value)
}
