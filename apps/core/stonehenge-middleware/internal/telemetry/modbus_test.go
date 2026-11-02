package telemetry

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"stonehenge-middleware/internal/arbitration"
	"stonehenge-middleware/internal/autocontrol"
	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/plc"
	"stonehenge-middleware/internal/safety"
	"stonehenge-middleware/internal/targetfeed"
)

func TestAutoStateResultAndErrorCodeMappings(t *testing.T) {
	if autocontrol.StateDisabled.Code() != 0 ||
		autocontrol.StateIdle.Code() != 1 ||
		autocontrol.StateTargetQueued.Code() != 2 ||
		autocontrol.StateEngaging.Code() != 3 ||
		autocontrol.StateWaitingForCompletion.Code() != 4 ||
		autocontrol.StateSuspendedFault.Code() != 5 ||
		autocontrol.StateSuspendedUnavailable.Code() != 6 {
		t.Fatal("unexpected auto state code mapping")
	}
	if autocontrol.ResultCode("") != 0 ||
		autocontrol.ResultCode("accepted") != 1 ||
		autocontrol.ResultCode("completed_fired") != 2 ||
		autocontrol.ResultCode("completed_no_fire") != 3 ||
		autocontrol.ResultCode("faulted") != 4 ||
		autocontrol.ResultCode("completion_timeout") != 5 ||
		autocontrol.ResultCode("protocol_state") != 6 {
		t.Fatal("unexpected lifecycle result code mapping")
	}
	tests := map[string]uint16{
		"":                             autocontrol.ErrorCodeNone,
		"plc connection refused":       autocontrol.ErrorCodePLCUnreachable,
		"gun not ready":                autocontrol.ErrorCodeGunNotReady,
		"plc fault active":             autocontrol.ErrorCodeGunFaulted,
		"safety fire permissive false": autocontrol.ErrorCodeSafetyNotPermissive,
		"command rejected":             autocontrol.ErrorCodeCommandRejected,
		"completion timeout":           autocontrol.ErrorCodeLifecycleTimeout,
		"protocol regression":          autocontrol.ErrorCodeProtocolState,
		"audit update failed":          autocontrol.ErrorCodePersistenceError,
		"controller unavailable":       autocontrol.ErrorCodeControllerUnavailable,
		"something else":               autocontrol.ErrorCodeOther,
	}
	for message, want := range tests {
		if got := autocontrol.ClassifyError(message); got != want {
			t.Fatalf("ClassifyError(%q) = %d, want %d", message, got, want)
		}
	}
}

func TestEncodeGun1AutoRegistersCompleteMap(t *testing.T) {
	now := time.Date(2026, 6, 28, 18, 0, 10, 0, time.UTC)
	lastChange := now.Add(-10 * time.Second)
	queued := uint32(0x12345678)
	active := uint32(0x9abcdef0)
	sequence := uint32(0x11223344)
	resetSeq := uint32(0x778899aa)
	faultBefore := uint16(2)
	faultAfter := uint16(0)
	resetChanged := now.Add(-5 * time.Second)
	snapshot := autocontrol.TelemetrySnapshot{
		SchemaVersion:         autocontrol.TelemetrySchemaVersion,
		Healthy:               true,
		Enabled:               true,
		State:                 autocontrol.StateWaitingForCompletion,
		QueuedTargetID:        &queued,
		ActiveTargetID:        &active,
		ActiveSequence:        &sequence,
		LastResult:            "completed_fired",
		LastErrorCode:         autocontrol.ErrorCodeProtocolState,
		LastStateChangedAt:    lastChange,
		TargetAzimuthTenths:   2205,
		TargetElevationTenths: -30,
		TargetPriority:        50,
		PLC: &plc.GunStatus{
			State:                  1,
			Ready:                  true,
			FaultActive:            true,
			FaultCode:              2,
			ShotSequence:           0x01020304,
			AcceptedSequence:       0x05060708,
			CurrentAzimuthTenths:   123,
			CurrentElevationTenths: -45,
			Settling:               true,
			Firing:                 true,
			Lockout:                true,
			OnTarget:               true,
		},
		Arbitration: &arbitration.Status{
			Mode:                     arbitration.ModeAutoSuspended,
			LastChangedAt:            now.Add(-20 * time.Second),
			ActiveOperation:          arbitration.OperationFaultReset,
			FaultResetActive:         true,
			LastArbitrationErrorCode: arbitration.ErrorResetNotAllowed,
			LastFaultResetResult:     string(arbitration.ResetResultLifecycleTimeout),
			LastFaultResetSequence:   &resetSeq,
			LastFaultCodeBefore:      &faultBefore,
			LastFaultCodeAfter:       &faultAfter,
			ResetStateChangedAt:      &resetChanged,
		},
	}
	feedActive := uint32(0x01020304)
	feed := targetfeed.Status{
		Configured:     true,
		Healthy:        true,
		Receiving:      true,
		PendingCount:   3,
		ActiveTargetID: &feedActive,
		ReceivedCount:  0x11112222,
		AcceptedCount:  0x33334444,
		DuplicateCount: 0x55556666,
		RejectedCount:  0x77778888,
		LastErrorCode:  targetfeed.ErrorCodeQueueFull,
	}
	safetyStatus := safety.Snapshot{
		Configured:            true,
		EnforcementEnabled:    true,
		Connected:             true,
		Fresh:                 true,
		SchemaVersion:         1,
		SchemaSupported:       true,
		HeartbeatCounter:      99,
		State:                 safety.StateReady,
		Ready:                 true,
		GlobalFirePermissive:  true,
		GlobalInhibitActive:   true,
		GlobalInhibitReason:   safety.GlobalInhibitMaintenanceMode,
		FaultActive:           true,
		FaultCode:             safety.FaultResetRequired,
		ResetRequired:         true,
		EmergencyStopActive:   true,
		MaintenanceModeActive: true,
		AgeSeconds:            6,
		LastErrorCode:         safety.ErrorSafetyFault,
	}
	safetyStatus.Guns = []safety.GunStatus{{
		GunID:                    1,
		FirePermissive:           true,
		InhibitActive:            true,
		InhibitReason:            safety.GunInhibitExternal,
		EnabledStatus:            true,
		MaintenanceInhibitStatus: true,
		ExternalInhibitStatus:    true,
	}}
	registers := EncodeGun1AutoRegistersWithFeedAndSafety(snapshot, feed, safetyStatus, now)
	want := map[int]uint16{
		0:  autocontrol.TelemetrySchemaVersion,
		1:  1,
		2:  1,
		3:  4,
		4:  0x5678,
		5:  0x1234,
		6:  0xdef0,
		7:  0x9abc,
		8:  0x3344,
		9:  0x1122,
		10: 2,
		11: autocontrol.ErrorCodeProtocolState,
		12: 10,
		13: 0,
		14: 2205,
		15: signedWord(-30),
		16: 50,
		17: 1,
		18: 1,
		19: 1,
		20: 2,
		21: 0x0304,
		22: 0x0102,
		23: 0x0708,
		24: 0x0506,
		25: 123,
		26: signedWord(-45),
		27: 1,
		28: 1,
		29: 1,
		30: 1,
		31: 0,
		32: arbitration.ModeAutoSuspended.Code(),
		33: arbitration.OperationFaultReset.Code(),
		34: 1,
		35: 1,
		36: uint16(arbitration.ErrorResetNotAllowed),
		37: arbitration.ResetResultLifecycleTimeout.Code(),
		38: 0x99aa,
		39: 0x7788,
		40: 2,
		41: 0,
		42: 20,
		43: 0,
		44: 5,
		45: 0,
		46: 0,
		47: 0,
		48: 1,
		49: 1,
		50: 1,
		51: 3,
		52: 0x0304,
		53: 0x0102,
		54: 0x2222,
		55: 0x1111,
		56: 0x4444,
		57: 0x3333,
		58: 0x6666,
		59: 0x5555,
		60: 0x8888,
		61: 0x7777,
		62: targetfeed.ErrorCodeQueueFull,
		63: 0,
		64: 1,
		65: 1,
		66: 1,
		67: 1,
		68: 1,
		69: 99,
		70: uint16(safety.StateReady),
		71: 1,
		72: 1,
		73: 1,
		74: uint16(safety.GlobalInhibitMaintenanceMode),
		75: 1,
		76: uint16(safety.FaultResetRequired),
		77: 1,
		78: 1,
		79: 1,
		80: 1,
		81: 1,
		82: uint16(safety.GunInhibitExternal),
		83: 1,
		84: 1,
		85: 1,
		86: 6,
		87: 0,
		88: safety.ErrorSafetyFault,
		89: 1,
		95: 0,
	}
	for offset, expected := range want {
		if registers[offset] != expected {
			t.Fatalf("register %d = %d (0x%04x), want %d (0x%04x)", offset, registers[offset], registers[offset], expected, expected)
		}
	}
}

func TestModbusReadOneRegisterAndRange(t *testing.T) {
	server := newTestServer(t, knownSnapshotProvider(), newStaticReader(statusForTelemetry()))
	server.pollPLCOnce()
	resp := server.handlePDU(1, readRequest(functionReadHoldingRegisters, 0, 1))
	if !bytes.Equal(resp, []byte{functionReadHoldingRegisters, 2, 0, byte(autocontrol.TelemetrySchemaVersion)}) {
		t.Fatalf("single register response = %v", resp)
	}
	resp = server.handlePDU(1, readRequest(functionReadHoldingRegisters, 21, 4))
	if resp[0] != functionReadHoldingRegisters || resp[1] != 8 {
		t.Fatalf("range response header = %v", resp[:2])
	}
	values := decodeRegisterBytes(t, resp[2:])
	if values[0] != 0x0304 || values[1] != 0x0102 || values[2] != 0x0708 || values[3] != 0x0506 {
		t.Fatalf("range values = %#v", values)
	}
}

func TestModbusExceptions(t *testing.T) {
	server := newTestServer(t, knownSnapshotProvider(), newStaticReader(statusForTelemetry()))
	tests := []struct {
		name string
		unit byte
		pdu  []byte
		want []byte
	}{
		{name: "invalid range", unit: 1, pdu: readRequest(functionReadHoldingRegisters, 95, 2), want: []byte{0x83, exceptionIllegalAddress}},
		{name: "invalid quantity", unit: 1, pdu: readRequest(functionReadHoldingRegisters, 0, 0), want: []byte{0x83, exceptionIllegalValue}},
		{name: "unsupported unit", unit: 2, pdu: readRequest(functionReadHoldingRegisters, 0, 1), want: []byte{0x83, exceptionGatewayTarget}},
		{name: "write single register", unit: 1, pdu: []byte{functionWriteSingleRegister, 0, 0, 0, 1}, want: []byte{0x86, exceptionIllegalFunction}},
		{name: "write multiple registers", unit: 1, pdu: []byte{functionWriteMultipleRegisters, 0, 0, 0, 1, 2, 0, 1}, want: []byte{0x90, exceptionIllegalFunction}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := server.handlePDU(tt.unit, tt.pdu)
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("response = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModbusTCPReadAndShutdown(t *testing.T) {
	provider := knownSnapshotProvider()
	reader := newStaticReader(statusForTelemetry())
	server := NewModbusServer(testPLCConfig(), provider, reader, slog.Default())
	server.pollPLCOnce()
	serverConn, conn := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		server.handleConn(serverConn)
	}()
	if _, err := conn.Write(modbusTCPRequest(7, 1, readRequest(functionReadHoldingRegisters, 0, 3))); err != nil {
		t.Fatalf("write request: %v", err)
	}
	response := make([]byte, 15)
	if _, err := io.ReadFull(conn, response); err != nil {
		t.Fatalf("read response: %v", err)
	}
	if binary.BigEndian.Uint16(response[0:2]) != 7 || response[7] != functionReadHoldingRegisters || response[8] != 6 {
		t.Fatalf("unexpected tcp response: %v", response)
	}
	if _, err := conn.Write([]byte{0, 1, 0, 2, 0, 1, 1}); err != nil {
		t.Fatalf("write malformed header: %v", err)
	}
	_ = conn.Close()
	server.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleConn did not stop")
	}
}

func TestStaleAndFreshPLCHealth(t *testing.T) {
	cfg := testPLCConfig()
	cfg.AutoTelemetryStaleAfter = 10 * time.Millisecond
	reader := newStaticReader(statusForTelemetry())
	provider := knownSnapshotProvider()
	server := NewModbusServer(cfg, provider, reader, slog.Default())
	server.pollPLCOnce()
	if status := server.Status(); !status.Healthy || status.Stale {
		t.Fatalf("fresh status = %+v, want healthy non-stale", status)
	}
	time.Sleep(20 * time.Millisecond)
	if status := server.Status(); status.Healthy || !status.Stale {
		t.Fatalf("stale status = %+v, want unhealthy stale", status)
	}
	server.pollPLCOnce()
	if status := server.Status(); !status.Healthy || status.Stale {
		t.Fatalf("refreshed status = %+v, want healthy non-stale", status)
	}
}

func TestDisabledAutoCanRemainControllerHealthy(t *testing.T) {
	provider := &fakeProvider{snapshot: autocontrol.TelemetrySnapshot{
		SchemaVersion:      autocontrol.TelemetrySchemaVersion,
		ControllerRunning:  true,
		Enabled:            false,
		State:              autocontrol.StateDisabled,
		LastStateChangedAt: time.Now().UTC(),
	}}
	server := NewModbusServer(testPLCConfig(), provider, newStaticReader(statusForTelemetry()), slog.Default())
	server.pollPLCOnce()
	registers := EncodeGun1AutoRegisters(server.snapshot(), time.Now().UTC())
	if registers[1] != 1 || registers[2] != 0 || registers[3] != autocontrol.StateCodeDisabled {
		t.Fatalf("disabled healthy registers[1:4] = %v, want healthy disabled", registers[1:4])
	}
}

func TestConcurrentSnapshotUpdateAndReads(t *testing.T) {
	provider := knownSnapshotProvider()
	server := newTestServer(t, provider, newStaticReader(statusForTelemetry()))
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			provider.setResult("accepted")
			provider.setResult("completed_fired")
		}
		close(done)
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				_ = server.handlePDU(1, readRequest(functionReadHoldingRegisters, 0, 32))
			}
		}
	}()
	wg.Wait()
}

func newTestServer(t *testing.T, provider *fakeProvider, reader *fakeReader) *ModbusServer {
	t.Helper()
	return NewModbusServer(testPLCConfig(), provider, reader, slog.Default())
}

func testPLCConfig() config.PLCConfig {
	cfg := config.Defaults().PLC
	cfg.AutoTelemetryModbusEnabled = true
	cfg.AutoTelemetryModbusAddr = "127.0.0.1:0"
	cfg.AutoTelemetryUnitID = 1
	cfg.AutoTelemetryPollInterval = 10 * time.Millisecond
	cfg.AutoTelemetryStaleAfter = time.Second
	return cfg
}

func knownSnapshotProvider() *fakeProvider {
	now := time.Now().UTC()
	queued := uint32(0x12345678)
	active := uint32(0x9abcdef0)
	sequence := uint32(0x11223344)
	return &fakeProvider{snapshot: autocontrol.TelemetrySnapshot{
		SchemaVersion:         autocontrol.TelemetrySchemaVersion,
		ControllerRunning:     true,
		Healthy:               true,
		Enabled:               true,
		State:                 autocontrol.StateWaitingForCompletion,
		QueuedTargetID:        &queued,
		ActiveTargetID:        &active,
		ActiveSequence:        &sequence,
		LastResult:            "completed_fired",
		LastErrorCode:         autocontrol.ErrorCodeNone,
		LastStateChangedAt:    now,
		TargetAzimuthTenths:   2205,
		TargetElevationTenths: 400,
		TargetPriority:        50,
	}}
}

func statusForTelemetry() plc.GunStatus {
	return plc.GunStatus{
		State:                  1,
		Ready:                  true,
		ShotSequence:           0x01020304,
		AcceptedSequence:       0x05060708,
		CurrentAzimuthTenths:   123,
		CurrentElevationTenths: 456,
	}
}

type fakeProvider struct {
	mu       sync.RWMutex
	snapshot autocontrol.TelemetrySnapshot
}

func (p *fakeProvider) TelemetrySnapshot(status *plc.GunStatus, updated *time.Time, stale bool) autocontrol.TelemetrySnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := p.snapshot
	out.PLC = status
	out.LastPLCUpdateAt = updated
	out.PLCStale = stale
	out.Healthy = out.ControllerRunning && !stale
	return out
}

func (p *fakeProvider) setResult(result string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.snapshot.LastResult = result
}

type fakeReader struct {
	mu     sync.Mutex
	status plc.GunStatus
	err    error
}

func newStaticReader(status plc.GunStatus) *fakeReader {
	return &fakeReader{status: status}
}

func (r *fakeReader) ReadStatus(ctx context.Context) (plc.GunStatus, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return plc.GunStatus{}, r.err
	}
	if ctx.Err() != nil {
		return plc.GunStatus{}, ctx.Err()
	}
	return r.status, nil
}

func readRequest(fn byte, address, quantity uint16) []byte {
	pdu := make([]byte, 5)
	pdu[0] = fn
	binary.BigEndian.PutUint16(pdu[1:3], address)
	binary.BigEndian.PutUint16(pdu[3:5], quantity)
	return pdu
}

func modbusTCPRequest(transaction uint16, unit byte, pdu []byte) []byte {
	request := make([]byte, 7+len(pdu))
	binary.BigEndian.PutUint16(request[0:2], transaction)
	binary.BigEndian.PutUint16(request[2:4], 0)
	binary.BigEndian.PutUint16(request[4:6], uint16(len(pdu)+1))
	request[6] = unit
	copy(request[7:], pdu)
	return request
}

func decodeRegisterBytes(t *testing.T, data []byte) []uint16 {
	t.Helper()
	if len(data)%2 != 0 {
		t.Fatalf("register data length is odd: %d", len(data))
	}
	values := make([]uint16, len(data)/2)
	for i := range values {
		values[i] = binary.BigEndian.Uint16(data[i*2:])
	}
	return values
}

func signedWord(value int16) uint16 {
	return uint16(value)
}

func TestPollReadErrorMarksStale(t *testing.T) {
	reader := &fakeReader{err: errors.New("connection refused")}
	server := NewModbusServer(testPLCConfig(), knownSnapshotProvider(), reader, slog.Default())
	server.pollPLCOnce()
	status := server.Status()
	if status.Healthy || !status.Stale {
		t.Fatalf("status = %+v, want unhealthy stale", status)
	}
	registers := EncodeGun1AutoRegisters(server.snapshot(), time.Now().UTC())
	if registers[1] != 0 || registers[11] != autocontrol.ErrorCodePLCUnreachable {
		t.Fatalf("health/error registers = %d/%d, want 0/%d", registers[1], registers[11], autocontrol.ErrorCodePLCUnreachable)
	}
}
