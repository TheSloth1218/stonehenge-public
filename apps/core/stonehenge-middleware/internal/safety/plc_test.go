package safety

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"stonehenge-middleware/internal/config"
)

func TestSafetyCodeNames(t *testing.T) {
	if StateReady.Name() != "ready" || State(99).Name() != "unknown" {
		t.Fatal("unexpected state names")
	}
	if GlobalInhibitEmergencyStop.Name() != "emergency_stop_active" || GlobalInhibitReason(99).Name() != "unknown" {
		t.Fatal("unexpected global inhibit names")
	}
	if GunInhibitExternal.Name() != "gun_external_inhibit" || GunInhibitReason(99).Name() != "unknown" {
		t.Fatal("unexpected gun inhibit names")
	}
	if FaultHeartbeatTiming.Name() != "heartbeat_internal_timing_fault" || FaultCode(99).Name() != "unknown" {
		t.Fatal("unexpected fault names")
	}
}

func TestDecodeSnapshotInputContract(t *testing.T) {
	words := permittedWords()
	snap, err := DecodeSnapshot(words, testSafetyConfig(), time.Now())
	if err != nil {
		t.Fatalf("DecodeSnapshot: %v", err)
	}
	if !snap.SchemaSupported || snap.SchemaVersion != 1 || !snap.Ready || !snap.GlobalFirePermissive {
		t.Fatalf("decoded global status = %+v", snap)
	}
	if len(snap.Guns) != 8 {
		t.Fatalf("guns = %d, want 8", len(snap.Guns))
	}
	if !snap.Guns[0].FirePermissive || snap.Guns[0].InhibitActive || !snap.Guns[0].EnabledStatus {
		t.Fatalf("gun 1 status = %+v", snap.Guns[0])
	}
}

func TestSafetyStatusReadsStartAtRawOffsetZero(t *testing.T) {
	cfg := testSafetyConfig()
	transport := &fakeSafetyTransport{input: permittedWords()}
	service := NewServiceWithTransport(cfg, transport, nil, nil)
	service.pollOnce()
	if len(transport.inputReadAt) != 1 {
		t.Fatalf("input reads = %+v, want one read", transport.inputReadAt)
	}
	if got := transport.inputReadAt[0]; got.address != 0 || got.quantity != StatusRegisterCount {
		t.Fatalf("status read = %+v, want address 0 quantity %d", got, StatusRegisterCount)
	}

	cfg.StatusRegisterType = "holding"
	transport = &fakeSafetyTransport{holding: permittedWords()}
	service = NewServiceWithTransport(cfg, transport, nil, nil)
	service.pollOnce()
	if len(transport.holdingReadAt) != 1 {
		t.Fatalf("holding reads = %+v, want one read", transport.holdingReadAt)
	}
	if got := transport.holdingReadAt[0]; got.address != 0 || got.quantity != StatusRegisterCount {
		t.Fatalf("holding status read = %+v, want address 0 quantity %d", got, StatusRegisterCount)
	}
}

func TestSafetyDecisionEnforcement(t *testing.T) {
	cfg := testSafetyConfig()
	words := permittedWords()
	snap, err := DecodeSnapshot(words, cfg, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	snap.Connected = true
	snap.Fresh = true
	if got := EvaluateFireDecision(snap, 1); !got.Allowed {
		t.Fatalf("permitted decision denied: %+v", got)
	}
	snap.GlobalInhibitActive = true
	if got := EvaluateFireDecision(snap, 1); got.Allowed || got.ReasonCode != DecisionGlobalInhibit {
		t.Fatalf("global inhibit decision = %+v", got)
	}
	snap.GlobalInhibitActive = false
	snap.Guns[0].InhibitActive = true
	if got := EvaluateFireDecision(snap, 1); got.Allowed || got.ReasonCode != DecisionGunInhibit {
		t.Fatalf("gun inhibit decision = %+v", got)
	}
	snap.EnforcementEnabled = false
	if got := EvaluateFireDecision(snap, 1); !got.Allowed || got.ReasonCode != DecisionEnforcementDisabled {
		t.Fatalf("disabled enforcement decision = %+v", got)
	}
}

func TestReadFailuresMarkUnavailable(t *testing.T) {
	cfg := testSafetyConfig()
	transport := &fakeSafetyTransport{readErr: errors.New("connection refused")}
	service := NewServiceWithTransport(cfg, transport, nil, nil)
	service.pollOnce()
	snap := service.Snapshot()
	if snap.Connected || snap.Fresh || snap.LastError == "" {
		t.Fatalf("snapshot = %+v, want unavailable with error", snap)
	}
}

func TestHoldingRegisterStatusConfigured(t *testing.T) {
	cfg := testSafetyConfig()
	cfg.StatusRegisterType = "holding"
	transport := &fakeSafetyTransport{holding: permittedWords()}
	service := NewServiceWithTransport(cfg, transport, nil, nil)
	service.pollOnce()
	if snap := service.Snapshot(); !snap.Connected || !snap.SchemaSupported || !snap.Ready {
		t.Fatalf("snapshot = %+v, want holding-register status decoded", snap)
	}
	if transport.inputReads != 0 || transport.holdingReads != 1 {
		t.Fatalf("reads input=%d holding=%d, want holding only", transport.inputReads, transport.holdingReads)
	}
}

func TestHeartbeatStalenessAndRecovery(t *testing.T) {
	cfg := testSafetyConfig()
	cfg.StaleAfter = 5 * time.Millisecond
	transport := &fakeSafetyTransport{input: permittedWords()}
	service := NewServiceWithTransport(cfg, transport, nil, nil)
	service.pollOnce()
	if snap := service.Snapshot(); !snap.Fresh {
		t.Fatalf("fresh snapshot = %+v", snap)
	}
	time.Sleep(10 * time.Millisecond)
	if snap := service.Snapshot(); snap.Fresh {
		t.Fatalf("stale snapshot = %+v", snap)
	}
	transport.input[1]++
	service.pollOnce()
	if snap := service.Snapshot(); !snap.Fresh || snap.HeartbeatCounter != 45 {
		t.Fatalf("recovered snapshot = %+v", snap)
	}
}

func TestFrozenHeartbeatDeniesFireUntilHeartbeatChanges(t *testing.T) {
	cfg := testSafetyConfig()
	cfg.StaleAfter = 15 * time.Millisecond
	transport := &fakeSafetyTransport{input: permittedWords()}
	service := NewServiceWithTransport(cfg, transport, nil, nil)
	service.pollOnce()
	if got := service.CanFire(1); !got.Allowed {
		t.Fatalf("initial decision = %+v, want allowed", got)
	}
	time.Sleep(20 * time.Millisecond)
	service.pollOnce()
	snap := service.Snapshot()
	if snap.Fresh {
		t.Fatalf("snapshot = %+v, want frozen heartbeat stale", snap)
	}
	if got := service.CanFire(1); got.Allowed || got.ReasonCode != DecisionStale {
		t.Fatalf("frozen heartbeat decision = %+v, want stale denial", got)
	}
	transport.mu.Lock()
	transport.input[1]++
	transport.mu.Unlock()
	service.pollOnce()
	if got := service.CanFire(1); !got.Allowed {
		t.Fatalf("recovered decision = %+v, want allowed", got)
	}
}

func TestSafetyControlWrites(t *testing.T) {
	cfg := testSafetyConfig()
	transport := &fakeSafetyTransport{input: permittedWords()}
	service := NewServiceWithTransport(cfg, transport, nil, nil)
	service.pollOnce()
	enable := true
	stop := false
	maintenance := true
	if err := service.SetGlobal(context.Background(), GlobalControl{GlobalEnable: &enable, EmergencyStop: &stop, GlobalMaintenanceMode: &maintenance}); err != nil {
		t.Fatalf("SetGlobal: %v", err)
	}
	if err := service.SetGun(context.Background(), 1, GunControl{Enabled: &enable, MaintenanceInhibit: &maintenance, ExternalInhibit: &stop}); err != nil {
		t.Fatalf("SetGun: %v", err)
	}
	if err := service.PulseReset(context.Background()); err != nil {
		t.Fatalf("PulseReset: %v", err)
	}
	want := []write{
		{81, 1}, // logical MW1 global_enable
		{82, 0}, // logical MW2 emergency_stop
		{83, 1}, // logical MW3 global_maintenance_mode
		{96, 1}, // logical MW16 Gun 1 enabled
		{97, 1}, // logical MW17 Gun 1 maintenance inhibit
		{98, 0}, // logical MW18 Gun 1 external inhibit
		{84, 1}, // logical MW4 safety_reset_request
		{84, 0}, // reset pulse clears logical MW4
	}
	if len(transport.writes) != len(want) {
		t.Fatalf("writes = %+v, want %+v", transport.writes, want)
	}
	for i := range want {
		if transport.writes[i] != want[i] {
			t.Fatalf("write %d = %+v, want %+v", i, transport.writes[i], want[i])
		}
		if transport.writes[i].address < cfg.MWRegisterBase {
			t.Fatalf("write %d targeted raw status/control overlap address %d", i, transport.writes[i].address)
		}
	}
}

func TestPulseResetHoldsConfiguredDurationAndRecordsAfterRelease(t *testing.T) {
	cfg := testSafetyConfig()
	cfg.ResetPulseDuration = 100 * time.Millisecond
	transport := &fakeSafetyTransport{input: permittedWords()}
	auditor := &fakeSafetyAuditor{}
	service := NewServiceWithTransport(cfg, transport, auditor, nil)
	service.pollOnce()
	start := time.Now()
	if err := service.PulseReset(context.Background()); err != nil {
		t.Fatalf("PulseReset: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < cfg.ResetPulseDuration {
		t.Fatalf("pulse elapsed %s, want at least %s", elapsed, cfg.ResetPulseDuration)
	}
	want := []write{{84, 1}, {84, 0}}
	if got := transport.writeSnapshot(); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("writes = %+v, want %+v", got, want)
	}
	events := auditor.eventsSnapshot()
	if got := countSafetyEvents(events, "reset_requested"); got != 1 {
		t.Fatalf("events = %+v, want one reset_requested after release", events)
	}
}

func TestPulseResetCanceledAfterAssertStillAttemptsRelease(t *testing.T) {
	cfg := testSafetyConfig()
	cfg.ResetPulseDuration = 5 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	transport := &fakeSafetyTransport{input: permittedWords()}
	transport.afterWrite = func(w write) {
		if w.address == 84 && w.value == 1 {
			cancel()
		}
	}
	auditor := &fakeSafetyAuditor{}
	service := NewServiceWithTransport(cfg, transport, auditor, nil)
	service.pollOnce()
	if err := service.PulseReset(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("PulseReset error = %v, want context.Canceled", err)
	}
	want := []write{{84, 1}, {84, 0}}
	if got := transport.writeSnapshot(); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("writes = %+v, want cleanup release %+v", got, want)
	}
	if events := auditor.eventsSnapshot(); countSafetyEvents(events, "reset_requested") != 0 {
		t.Fatalf("events = %+v, want no reset_requested for canceled pulse", events)
	}
}

func TestPulseResetAssertFailureDoesNotReleaseOrRecord(t *testing.T) {
	cfg := testSafetyConfig()
	transport := &fakeSafetyTransport{input: permittedWords()}
	transport.writeErr = func(w write) error {
		if w.address == 84 && w.value == 1 {
			return errors.New("assert failed")
		}
		return nil
	}
	auditor := &fakeSafetyAuditor{}
	service := NewServiceWithTransport(cfg, transport, auditor, nil)
	service.pollOnce()
	if err := service.PulseReset(context.Background()); err == nil {
		t.Fatal("PulseReset succeeded, want assert failure")
	}
	if got := transport.writeSnapshot(); len(got) != 1 || got[0] != (write{84, 1}) {
		t.Fatalf("writes = %+v, want assert only", got)
	}
	if events := auditor.eventsSnapshot(); countSafetyEvents(events, "reset_requested") != 0 {
		t.Fatalf("events = %+v, want none", events)
	}
}

func TestPulseResetReleaseFailureDoesNotRecord(t *testing.T) {
	cfg := testSafetyConfig()
	cfg.ResetPulseDuration = 100 * time.Millisecond
	transport := &fakeSafetyTransport{input: permittedWords()}
	transport.writeErr = func(w write) error {
		if w.address == 84 && w.value == 0 {
			return errors.New("release failed")
		}
		return nil
	}
	auditor := &fakeSafetyAuditor{}
	service := NewServiceWithTransport(cfg, transport, auditor, nil)
	service.pollOnce()
	if err := service.PulseReset(context.Background()); err == nil {
		t.Fatal("PulseReset succeeded, want release failure")
	}
	want := []write{{84, 1}, {84, 0}}
	if got := transport.writeSnapshot(); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("writes = %+v, want %+v", got, want)
	}
	if events := auditor.eventsSnapshot(); countSafetyEvents(events, "reset_requested") != 0 {
		t.Fatalf("events = %+v, want none", events)
	}
}

func TestSafetyControlAddressMapping(t *testing.T) {
	tests := []struct {
		name    string
		logical uint16
		raw     uint16
	}{
		{name: "MW0 reserved", logical: 0, raw: 80},
		{name: "MW1 global enable", logical: 1, raw: 81},
		{name: "MW2 emergency stop", logical: 2, raw: 82},
		{name: "MW3 maintenance", logical: 3, raw: 83},
		{name: "MW4 reset", logical: 4, raw: 84},
		{name: "MW5 test fault", logical: 5, raw: 85},
		{name: "Gun 1 MW16 enabled", logical: 16, raw: 96},
		{name: "Gun 1 MW17 maintenance inhibit", logical: 17, raw: 97},
		{name: "Gun 1 MW18 external inhibit", logical: 18, raw: 98},
		{name: "Gun 8 MW44 enabled", logical: 44, raw: 124},
		{name: "Gun 8 MW45 maintenance inhibit", logical: 45, raw: 125},
		{name: "Gun 8 MW46 external inhibit", logical: 46, raw: 126},
		{name: "MW47 max supported", logical: 47, raw: 127},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safetyControlRawAddress(testSafetyConfig().MWRegisterBase, tt.logical); got != tt.raw {
				t.Fatalf("safetyControlRawAddress(%d) = %d, want %d", tt.logical, got, tt.raw)
			}
		})
	}
}

func TestSafetyGun8ControlWritesUseMemoryWordBase(t *testing.T) {
	cfg := testSafetyConfig()
	transport := &fakeSafetyTransport{input: permittedWords()}
	service := NewServiceWithTransport(cfg, transport, nil, nil)
	service.pollOnce()
	yes := true
	no := false
	if err := service.SetGun(context.Background(), 8, GunControl{Enabled: &yes, MaintenanceInhibit: &no, ExternalInhibit: &yes}); err != nil {
		t.Fatalf("SetGun: %v", err)
	}
	want := []write{{124, 1}, {125, 0}, {126, 1}}
	if len(transport.writes) != len(want) {
		t.Fatalf("writes = %+v, want %+v", transport.writes, want)
	}
	for i := range want {
		if transport.writes[i] != want[i] {
			t.Fatalf("write %d = %+v, want %+v", i, transport.writes[i], want[i])
		}
	}
}

func TestUnsupportedSchemaDeniesAndBlocksControls(t *testing.T) {
	cfg := testSafetyConfig()
	words := permittedWords()
	words[0] = 99
	transport := &fakeSafetyTransport{input: words}
	service := NewServiceWithTransport(cfg, transport, nil, nil)
	service.pollOnce()
	if got := service.CanFire(1); got.Allowed || got.ReasonCode != DecisionUnsupportedSchema {
		t.Fatalf("decision = %+v", got)
	}
	if err := service.PulseReset(context.Background()); err == nil {
		t.Fatal("PulseReset succeeded with unsupported schema")
	}
}

func permittedWords() []uint16 {
	words := make([]uint16, StatusRegisterCount)
	words[0] = 1
	words[1] = 44
	words[2] = uint16(StateReady)
	words[3] = 1
	words[4] = 1
	for gunID := 1; gunID <= 8; gunID++ {
		base := 16 + (gunID-1)*8
		words[base] = 1
		words[base+3] = 1
	}
	return words
}

func testSafetyConfig() config.SafetyPLCConfig {
	cfg := config.Defaults().SafetyPLC
	cfg.Enforcement = true
	cfg.PollInterval = 10 * time.Millisecond
	cfg.StaleAfter = time.Second
	cfg.ResetPulseDuration = 100 * time.Millisecond
	return cfg
}

type write struct {
	address uint16
	value   uint16
}

type fakeSafetyTransport struct {
	mu            sync.Mutex
	input         []uint16
	holding       []uint16
	readErr       error
	writeErr      func(write) error
	afterWrite    func(write)
	writes        []write
	inputReadAt   []read
	holdingReadAt []read
	inputReads    int
	holdingReads  int
}

type read struct {
	address  uint16
	quantity uint16
}

func (f *fakeSafetyTransport) ReadInputRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error) {
	f.inputReads++
	f.inputReadAt = append(f.inputReadAt, read{address: address, quantity: quantity})
	if f.readErr != nil {
		return nil, f.readErr
	}
	return append([]uint16(nil), f.input[address:address+quantity]...), nil
}

func (f *fakeSafetyTransport) ReadHoldingRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error) {
	f.holdingReads++
	f.holdingReadAt = append(f.holdingReadAt, read{address: address, quantity: quantity})
	if f.readErr != nil {
		return nil, f.readErr
	}
	source := f.holding
	if source == nil {
		source = f.input
	}
	return append([]uint16(nil), source[address:address+quantity]...), nil
}

func (f *fakeSafetyTransport) WriteSingleRegister(ctx context.Context, address, value uint16) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	w := write{address: address, value: value}
	f.mu.Lock()
	f.writes = append(f.writes, w)
	f.mu.Unlock()
	if f.afterWrite != nil {
		f.afterWrite(w)
	}
	if f.writeErr != nil {
		return f.writeErr(w)
	}
	return nil
}

func (f *fakeSafetyTransport) writeSnapshot() []write {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]write(nil), f.writes...)
}

type fakeSafetyAuditor struct {
	mu     sync.Mutex
	events []Event
}

func (f *fakeSafetyAuditor) RecordSafetyEvent(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event)
	return nil
}

func (f *fakeSafetyAuditor) eventsSnapshot() []Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Event(nil), f.events...)
}

func countSafetyEvents(events []Event, eventType string) int {
	count := 0
	for _, event := range events {
		if event.EventType == eventType {
			count++
		}
	}
	return count
}
