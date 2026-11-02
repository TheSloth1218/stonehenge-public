package plc

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"stonehenge-middleware/internal/config"
)

func TestWordConversions(t *testing.T) {
	if got := Int16FromUint16(Uint16FromInt16(-300)); got != -300 {
		t.Fatalf("signed conversion = %d, want -300", got)
	}
	low, high := SplitUint32(0x12345678)
	if low != 0x5678 || high != 0x1234 {
		t.Fatalf("split = %#x %#x", low, high)
	}
	if got := CombineUint32(low, high); got != 0x12345678 {
		t.Fatalf("combine = %#x", got)
	}
}

func TestNextSequenceRollover(t *testing.T) {
	if got := NextSequence(41); got != 42 {
		t.Fatalf("next = %d, want 42", got)
	}
	if got := NextSequence(^uint32(0)); got != 0 {
		t.Fatalf("rollover = %d, want 0", got)
	}
	if !sequenceAfter(0, ^uint32(0)) {
		t.Fatal("0 must compare after max uint32 for rollover")
	}
}

func TestBuildPayloadOrdering(t *testing.T) {
	fragment := uint32(0x01020304)
	payload := BuildPayload(0x11223344, GunCommand{
		Mode:                     ModeManual,
		TargetAzimuth:            120.4,
		TargetElevation:          -12.5,
		FragmentID:               &fragment,
		FireRequest:              true,
		AdminOverride:            true,
		SafetyExtendedPermissive: true,
	})
	if payload[RegisterCommandSequenceLow] != 0x3344 || payload[RegisterCommandSequenceHigh] != 0x1122 {
		t.Fatalf("sequence words = %#v", payload[:2])
	}
	if payload[RegisterTargetAzimuth] != 1204 {
		t.Fatalf("azimuth word = %d", payload[RegisterTargetAzimuth])
	}
	if Int16FromUint16(payload[RegisterTargetElevation]) != -125 {
		t.Fatalf("elevation word = %d", payload[RegisterTargetElevation])
	}
	if payload[RegisterFragmentLow] != 0x0304 || payload[RegisterFragmentHigh] != 0x0102 {
		t.Fatalf("fragment words = %#x %#x", payload[RegisterFragmentLow], payload[RegisterFragmentHigh])
	}
}

func TestCommandAndStatusEffectiveAddresses(t *testing.T) {
	if gun1CommandBase != 1024 {
		t.Fatalf("gun1CommandBase = %d, want 1024", gun1CommandBase)
	}
	if gun1StatusBase != 20 {
		t.Fatalf("gun1StatusBase = %d, want 20", gun1StatusBase)
	}
	tests := []struct {
		name     string
		register uint16
		want     uint16
	}{
		{name: "MW0 commit", register: RegisterCommandSequenceLow, want: 1024},
		{name: "MW1 high", register: RegisterCommandSequenceHigh, want: 1025},
		{name: "MW12 extended permissive", register: RegisterSafetyExtendedPermissive, want: 1036},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commandAddress(tt.register); got != tt.want {
				t.Fatalf("commandAddress(%d) = %d, want %d", tt.register, got, tt.want)
			}
		})
	}
	if RegisterAcceptedSequenceLow != 20 || RegisterLockoutSeconds != 38 {
		t.Fatalf("status register span = %d..%d, want 20..38", RegisterAcceptedSequenceLow, RegisterLockoutSeconds)
	}
}

func TestSubmitWritesCommandSequenceLowLastAndAcknowledges(t *testing.T) {
	transport := newMockTransport()
	transport.status.AcceptedSequence = 9
	service := NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default())
	result, err := service.Submit(context.Background(), Gun1ID, validCommand())
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if !result.Accepted || result.SubmittedSequence != 10 || result.AcceptedSequence != 10 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(transport.writes) != 2 {
		t.Fatalf("writes len = %d, want 2", len(transport.writes))
	}
	first, second := transport.writes[0], transport.writes[1]
	if first.address != 1025 || len(first.values) != 10 {
		t.Fatalf("first write = %+v", first)
	}
	if first.address+uint16(len(first.values))-1 != 1034 {
		t.Fatalf("payload write spans %d..%d, want 1025..1034", first.address, first.address+uint16(len(first.values))-1)
	}
	if second.address != 1024 || len(second.values) != 1 || second.values[0] != 10 {
		t.Fatalf("commit write = %+v", second)
	}
	for _, write := range transport.writes {
		for _, register := range []uint16{RegisterSafetyFirePermissive, RegisterSafetyExtendedPermissive} {
			if write.address <= commandAddress(register) && commandAddress(register) < write.address+uint16(len(write.values)) {
				t.Fatalf("command submission overwrote live safety lease register %d: %+v", register, write)
			}
		}
	}
}

func TestWriteSafetyLeasesWritesBothLiveRegisters(t *testing.T) {
	transport := newMockTransport()
	service := NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default())
	if err := service.WriteSafetyLeases(context.Background(), 42); err != nil {
		t.Fatalf("WriteSafetyLeases returned error: %v", err)
	}
	if len(transport.writes) != 1 || transport.writes[0].address != 1035 || len(transport.writes[0].values) != 2 || transport.writes[0].values[0] != 42 || transport.writes[0].values[1] != 42 {
		t.Fatalf("safety lease write = %+v", transport.writes)
	}
}

func TestSubmissionSequenceDoesNotRegressWhenPLCReportsOlderAcceptedSequence(t *testing.T) {
	transport := newMockTransport()
	transport.status.AcceptedSequence = 1
	service := NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default())

	first, err := service.Submit(context.Background(), Gun1ID, validCommand())
	if err != nil {
		t.Fatalf("first Submit returned error: %v", err)
	}
	if first.SubmittedSequence != 2 || !first.Accepted {
		t.Fatalf("first result = %+v, want submitted/accepted sequence 2", first)
	}

	transport.mu.Lock()
	transport.status.AcceptedSequence = 1
	transport.mu.Unlock()

	second, err := service.Submit(context.Background(), Gun1ID, validCommand())
	if err != nil {
		t.Fatalf("second Submit returned error: %v", err)
	}
	if second.SubmittedSequence != 3 || !second.Accepted {
		t.Fatalf("second result = %+v, want submitted/accepted sequence 3", second)
	}
	if got := transport.committedSequences(); len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("committed sequences = %v, want [2 3]", got)
	}
}

func TestSeedSequenceFloorKeepsSubmissionMonotonicWhenPLCResets(t *testing.T) {
	transport := newMockTransport()
	transport.status.AcceptedSequence = 0
	service := NewGunCommandServiceForGunWithTransport(2, testPLCConfig(), transport, slog.Default())
	service.SeedSequenceFloor(9)

	result, err := service.Submit(context.Background(), 2, validCommand())
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if !result.Accepted || result.SubmittedSequence != 10 || result.AcceptedSequence != 10 {
		t.Fatalf("result = %+v, want submitted/accepted sequence 10", result)
	}
	if got := transport.committedSequences(); len(got) != 1 || got[0] != 10 {
		t.Fatalf("committed sequences = %v, want [10]", got)
	}
}

func TestSeedSequenceFloorDoesNotLowerProcessLocalFloor(t *testing.T) {
	transport := newMockTransport()
	service := NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default())
	service.SeedSequenceFloor(9)
	service.SeedSequenceFloor(4)

	result, err := service.Submit(context.Background(), Gun1ID, validCommand())
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if !result.Accepted || result.SubmittedSequence != 10 {
		t.Fatalf("result = %+v, want sequence 10", result)
	}
}

func TestSeedSequenceFloorUsesPLCHigherThanPersistedFloor(t *testing.T) {
	transport := newMockTransport()
	transport.status.AcceptedSequence = 12
	service := NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default())
	service.SeedSequenceFloor(9)

	result, err := service.Submit(context.Background(), Gun1ID, validCommand())
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if !result.Accepted || result.SubmittedSequence != 13 {
		t.Fatalf("result = %+v, want sequence 13", result)
	}
}

func TestSeededGunCommandServicesRemainIndependent(t *testing.T) {
	transportA := newMockTransport()
	transportB := newMockTransport()
	gunA := NewGunCommandServiceForGunWithTransport(1, testPLCConfig(), transportA, slog.Default())
	gunB := NewGunCommandServiceForGunWithTransport(2, testPLCConfig(), transportB, slog.Default())
	gunA.SeedSequenceFloor(30)
	gunB.SeedSequenceFloor(7)

	resultA, err := gunA.Submit(context.Background(), 1, validCommand())
	if err != nil {
		t.Fatalf("gun A Submit: %v", err)
	}
	resultB, err := gunB.Submit(context.Background(), 2, validCommand())
	if err != nil {
		t.Fatalf("gun B Submit: %v", err)
	}
	if resultA.SubmittedSequence != 31 || resultB.SubmittedSequence != 8 {
		t.Fatalf("submitted sequences A=%d B=%d, want 31 and 8", resultA.SubmittedSequence, resultB.SubmittedSequence)
	}
}

func TestSubmitRequiresExactAcknowledgmentMatch(t *testing.T) {
	transport := newMockTransport()
	transport.status.AcceptedSequence = 2
	transport.ackOnCommit = false
	cfg := testPLCConfig()
	cfg.CommandAckTimeout = 20 * time.Millisecond
	cfg.PollInterval = 5 * time.Millisecond
	service := NewGunCommandServiceWithTransport(cfg, transport, slog.Default())

	result, err := service.Submit(context.Background(), Gun1ID, validCommand())
	if err == nil {
		t.Fatal("expected timeout because PLC never accepted the submitted sequence exactly")
	}
	if result.Accepted || result.SubmittedSequence != 3 || result.AcceptedSequence != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSubmitStaleLowerAcknowledgmentDoesNotAccept(t *testing.T) {
	transport := newMockTransport()
	transport.status.AcceptedSequence = 1
	transport.ackOnCommit = false
	cfg := testPLCConfig()
	cfg.CommandAckTimeout = 20 * time.Millisecond
	cfg.PollInterval = 5 * time.Millisecond
	service := NewGunCommandServiceWithTransport(cfg, transport, slog.Default())

	result, err := service.Submit(context.Background(), Gun1ID, validCommand())
	if err == nil {
		t.Fatal("expected timeout for stale lower acknowledgment")
	}
	if result.Accepted || result.SubmittedSequence != 2 || result.AcceptedSequence != 1 || result.ErrorType != "timeout" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSubmitAcknowledgmentRegression(t *testing.T) {
	transport := newMockTransport()
	transport.ackOnCommit = false
	transport.statusQueue = []GunStatus{
		{AcceptedSequence: 1},
		{AcceptedSequence: 1},
		{AcceptedSequence: 0},
	}
	service := NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default())

	result, err := service.Submit(context.Background(), Gun1ID, validCommand())
	if err == nil {
		t.Fatal("expected protocol state error for accepted sequence regression")
	}
	if result.Accepted || result.ErrorType != "protocol_state" || result.AcceptedSequence != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSubmitSequenceRolloverUsesProcessLocalFloor(t *testing.T) {
	transport := newMockTransport()
	transport.status.AcceptedSequence = ^uint32(0)
	service := NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default())

	first, err := service.Submit(context.Background(), Gun1ID, validCommand())
	if err != nil {
		t.Fatalf("first Submit returned error: %v", err)
	}
	if first.SubmittedSequence != 0 || !first.Accepted {
		t.Fatalf("first result = %+v, want rollover sequence 0", first)
	}

	transport.mu.Lock()
	transport.status.AcceptedSequence = ^uint32(0)
	transport.mu.Unlock()

	second, err := service.Submit(context.Background(), Gun1ID, validCommand())
	if err != nil {
		t.Fatalf("second Submit returned error: %v", err)
	}
	if second.SubmittedSequence != 1 || !second.Accepted {
		t.Fatalf("second result = %+v, want sequence 1", second)
	}
	if got := transport.committedSequences(); len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("committed sequences = %v, want [0 1]", got)
	}
}

func TestReadStatusUsesStatusOffsetsAndRawRegisters(t *testing.T) {
	transport := newMockTransport()
	transport.status.AcceptedSequence = 0x01020304
	transport.status.State = 1
	transport.status.ShotSequence = 0x05060708
	service := NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default())
	status, err := service.ReadStatus(context.Background())
	if err != nil {
		t.Fatalf("ReadStatus returned error: %v", err)
	}
	if len(transport.reads) != 1 {
		t.Fatalf("reads len = %d, want 1", len(transport.reads))
	}
	if transport.reads[0].address != 20 || transport.reads[0].quantity != 19 {
		t.Fatalf("status read = %+v, want address 20 quantity 19", transport.reads[0])
	}
	if status.AcceptedSequence != 0x01020304 || status.ShotSequence != 0x05060708 || status.StateName != "Ready" {
		t.Fatalf("unexpected decoded status: %+v", status)
	}
	if status.AcceptedSequenceLow != 0x0304 || status.AcceptedSequenceHigh != 0x0102 {
		t.Fatalf("accepted sequence words = %#x %#x", status.AcceptedSequenceLow, status.AcceptedSequenceHigh)
	}
	if len(status.RawRegisters) != 19 || status.RawRegisters[0] != 0x0304 || status.RawRegisters[1] != 0x0102 {
		t.Fatalf("unexpected raw registers: %#v", status.RawRegisters)
	}
}

func TestSubmitFaultResponse(t *testing.T) {
	transport := newMockTransport()
	transport.status.AcceptedSequence = 5
	transport.status.FaultActive = true
	transport.status.FaultCode = 2
	service := NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default())
	result, err := service.Submit(context.Background(), Gun1ID, validCommand())
	if err == nil {
		t.Fatal("expected fault error")
	}
	if result.ErrorType != "plc_fault" || result.FaultName != "Safety blocked" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestFaultResetWaitsForExactAcknowledgmentDespiteStaleFault(t *testing.T) {
	transport := newMockTransport()
	transport.ackOnCommit = false
	transport.statusQueue = []GunStatus{
		statusForLifecycle(24, 7, 0, true, 2),
		statusForLifecycle(24, 7, 0, true, 2),
		statusForLifecycle(24, 7, 0, true, 2),
		statusForLifecycle(25, 7, 0, true, 2),
	}
	service := NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default())
	command := validCommand()
	command.FaultReset = true
	command.SafetyFirePermissive = false

	result, err := service.Submit(context.Background(), Gun1ID, command)
	if err != nil {
		t.Fatalf("fault reset Submit returned error: %v result=%+v", err, result)
	}
	if !result.Accepted || result.SubmittedSequence != 25 || result.AcceptedSequence != 25 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !result.FaultActive || result.FaultCode != 2 {
		t.Fatalf("ack status should preserve transitional fault for lifecycle tracking: %+v", result)
	}
	if got := transport.committedSequences(); len(got) != 1 || got[0] != 25 {
		t.Fatalf("committed sequences = %v, want [25]", got)
	}
}

func TestNonResetCommandIgnoresStaleFaultThenRejectsAfterExactAcknowledgment(t *testing.T) {
	transport := newMockTransport()
	transport.ackOnCommit = false
	transport.statusQueue = []GunStatus{
		statusForLifecycle(24, 1, 0, false, 0),
		statusForLifecycle(24, 7, 0, true, 2),
		statusForLifecycle(25, 7, 0, true, 2),
	}
	service := NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default())

	result, err := service.Submit(context.Background(), Gun1ID, validCommand())
	if err == nil {
		t.Fatal("expected plc fault after exact acknowledgment")
	}
	if result.Accepted || result.ErrorType != "plc_fault" || result.SubmittedSequence != 25 || result.AcceptedSequence != 25 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(transport.reads) < 3 {
		t.Fatalf("expected initial read plus stale and exact ack polls, got %d reads", len(transport.reads))
	}
}

func TestSubmitProtocolStateWhenAcceptedAdvancesPastSubmitted(t *testing.T) {
	transport := newMockTransport()
	transport.ackOnCommit = false
	transport.statusQueue = []GunStatus{
		statusForLifecycle(24, 1, 0, false, 0),
		statusForLifecycle(26, 1, 0, false, 0),
	}
	service := NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default())

	result, err := service.Submit(context.Background(), Gun1ID, validCommand())
	if err == nil {
		t.Fatal("expected protocol-state error")
	}
	if result.Accepted || result.ErrorType != "protocol_state" || result.SubmittedSequence != 25 || result.AcceptedSequence != 26 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSubmitAcknowledgmentTimeout(t *testing.T) {
	transport := newMockTransport()
	transport.ackOnCommit = false
	cfg := testPLCConfig()
	cfg.CommandAckTimeout = 20 * time.Millisecond
	cfg.PollInterval = 5 * time.Millisecond
	service := NewGunCommandServiceWithTransport(cfg, transport, slog.Default())
	result, err := service.Submit(context.Background(), Gun1ID, validCommand())
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !result.TimedOut || result.ErrorType != "timeout" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSubmitTransportError(t *testing.T) {
	transport := newMockTransport()
	transport.readErr = errors.New("dial failed")
	service := NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default())
	result, err := service.Submit(context.Background(), Gun1ID, validCommand())
	if err == nil {
		t.Fatal("expected transport error")
	}
	if result.ErrorType != "transport" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestConcurrentSubmissionsSerialized(t *testing.T) {
	transport := newMockTransport()
	service := NewGunCommandServiceWithTransport(testPLCConfig(), transport, slog.Default())
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := service.Submit(context.Background(), Gun1ID, validCommand()); err != nil {
				t.Errorf("Submit returned error: %v", err)
			}
		}()
	}
	wg.Wait()
	if transport.status.AcceptedSequence != 5 {
		t.Fatalf("accepted sequence = %d, want 5", transport.status.AcceptedSequence)
	}
}

func TestGunCommandServicesOwnIndependentSequences(t *testing.T) {
	transportA := newMockTransport()
	transportB := newMockTransport()
	transportA.status.AcceptedSequence = 10
	transportB.status.AcceptedSequence = 100
	gunA := NewGunCommandServiceForGunWithTransport(1, testPLCConfig(), transportA, slog.Default())
	gunB := NewGunCommandServiceForGunWithTransport(2, testPLCConfig(), transportB, slog.Default())

	resultA, err := gunA.Submit(context.Background(), 1, validCommand())
	if err != nil {
		t.Fatalf("gun A Submit: %v", err)
	}
	resultB, err := gunB.Submit(context.Background(), 2, validCommand())
	if err != nil {
		t.Fatalf("gun B Submit: %v", err)
	}
	if resultA.SubmittedSequence != 11 || resultB.SubmittedSequence != 101 {
		t.Fatalf("submitted sequences A=%d B=%d, want 11 and 101", resultA.SubmittedSequence, resultB.SubmittedSequence)
	}
	if _, err := gunA.Submit(context.Background(), 2, validCommand()); err == nil {
		t.Fatal("expected wrong-gun submission to be rejected")
	}
	if got := transportA.committedSequences(); len(got) != 1 || got[0] != 11 {
		t.Fatalf("gun A committed sequences = %v, want [11]", got)
	}
	if got := transportB.committedSequences(); len(got) != 1 || got[0] != 101 {
		t.Fatalf("gun B committed sequences = %v, want [101]", got)
	}
}

func TestLifecycleMovementCompletesNoFire(t *testing.T) {
	reader := newLifecycleReader([]GunStatus{
		statusForLifecycle(2, 2, 0, false, 0),
		statusForLifecycle(2, 3, 0, false, 0),
		statusForLifecycle(2, 1, 0, false, 0),
	})
	tracker := NewCommandLifecycleTracker(lifecycleTestConfig(), reader, newLifecycleAuditSink(), slog.Default())
	completion := tracker.runLifecycle(context.Background(), lifecycleRequest(false, 0, statusForLifecycle(2, 2, 0, false, 0)))
	if completion.Result != "completed_no_fire" || completion.ResultingState != 1 || completion.ShotSequence == nil || *completion.ShotSequence != 0 {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleCompletedNoFireStoresNonZeroFinalShotSequence(t *testing.T) {
	reader := newLifecycleReader([]GunStatus{
		statusForLifecycle(12, 2, 4, false, 0),
		statusForLifecycle(12, 1, 4, false, 0),
	})
	tracker := NewCommandLifecycleTracker(lifecycleTestConfig(), reader, newLifecycleAuditSink(), slog.Default())
	completion := tracker.runLifecycle(context.Background(), lifecycleRequest(false, 4, statusForLifecycle(12, 2, 4, false, 0)))
	if completion.Result != "completed_no_fire" || completion.ShotSequence == nil || *completion.ShotSequence != 4 {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleFiringCompletesAfterShotAndReady(t *testing.T) {
	reader := newLifecycleReader([]GunStatus{
		statusForLifecycle(3, 3, 0, false, 0),
		statusForLifecycle(3, 5, 1, false, 0),
		statusForLifecycle(3, 1, 1, false, 0),
	})
	tracker := NewCommandLifecycleTracker(lifecycleTestConfig(), reader, newLifecycleAuditSink(), slog.Default())
	completion := tracker.runLifecycle(context.Background(), lifecycleRequest(true, 0, statusForLifecycle(3, 2, 0, false, 0)))
	if completion.Result != "completed_fired" || completion.ResultingState != 1 || completion.ShotSequence == nil || *completion.ShotSequence != 1 {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleFiringDoesNotCompleteOnShotSequenceReset(t *testing.T) {
	cfg := lifecycleTestConfig()
	cfg.LifecycleCompletionTimeout = 15 * time.Millisecond
	reader := newLifecycleReader([]GunStatus{
		statusForLifecycle(18, 1, 0, false, 0),
	})
	tracker := NewCommandLifecycleTracker(cfg, reader, newLifecycleAuditSink(), slog.Default())
	completion := tracker.runLifecycle(context.Background(), lifecycleRequest(true, 5, statusForLifecycle(18, 2, 5, false, 0)))
	if completion.Result != "completion_timeout" || completion.ShotSequence == nil || *completion.ShotSequence != 0 {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleFiringRecognizesFirstShotAfterRestartBaseline(t *testing.T) {
	reader := newLifecycleReader([]GunStatus{
		statusForLifecycle(19, 4, 1, false, 0),
		statusForLifecycle(19, 1, 1, false, 0),
	})
	tracker := NewCommandLifecycleTracker(lifecycleTestConfig(), reader, newLifecycleAuditSink(), slog.Default())
	completion := tracker.runLifecycle(context.Background(), lifecycleRequest(true, 0, statusForLifecycle(19, 2, 0, false, 0)))
	if completion.Result != "completed_fired" || completion.ShotSequence == nil || *completion.ShotSequence != 1 {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleFiringTimeoutRetainsShotSequence(t *testing.T) {
	cfg := lifecycleTestConfig()
	cfg.LifecycleCompletionTimeout = 15 * time.Millisecond
	reader := newLifecycleReader([]GunStatus{statusForLifecycle(4, 5, 1, false, 0)})
	tracker := NewCommandLifecycleTracker(cfg, reader, newLifecycleAuditSink(), slog.Default())
	completion := tracker.runLifecycle(context.Background(), lifecycleRequest(true, 0, statusForLifecycle(4, 2, 0, false, 0)))
	if completion.Result != "completion_timeout" || completion.ShotSequence == nil || *completion.ShotSequence != 1 {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleFaulted(t *testing.T) {
	reader := newLifecycleReader([]GunStatus{statusForLifecycle(5, 7, 0, true, 2)})
	tracker := NewCommandLifecycleTracker(lifecycleTestConfig(), reader, newLifecycleAuditSink(), slog.Default())
	completion := tracker.runLifecycle(context.Background(), lifecycleRequest(false, 0, statusForLifecycle(5, 2, 0, false, 0)))
	if completion.Result != "faulted" || completion.FaultCode != 2 || completion.ShotSequence == nil || *completion.ShotSequence != 0 {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleFaultedStoresLastObservedShotSequence(t *testing.T) {
	reader := newLifecycleReader([]GunStatus{statusForLifecycle(13, 7, 6, true, 2)})
	tracker := NewCommandLifecycleTracker(lifecycleTestConfig(), reader, newLifecycleAuditSink(), slog.Default())
	completion := tracker.runLifecycle(context.Background(), lifecycleRequest(false, 6, statusForLifecycle(13, 2, 6, false, 0)))
	if completion.Result != "faulted" || completion.ShotSequence == nil || *completion.ShotSequence != 6 {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleAcceptedSequenceRegression(t *testing.T) {
	reader := newLifecycleReader([]GunStatus{statusForLifecycle(2, 2, 0, false, 0)})
	tracker := NewCommandLifecycleTracker(lifecycleTestConfig(), reader, newLifecycleAuditSink(), slog.Default())
	completion := tracker.runLifecycle(context.Background(), lifecycleRequest(false, 0, statusForLifecycle(3, 2, 0, false, 0)))
	if completion.Result != "protocol_state" || completion.ShotSequence == nil || *completion.ShotSequence != 0 {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleAcceptedSequenceChangesToAnotherCommand(t *testing.T) {
	reader := newLifecycleReader([]GunStatus{statusForLifecycle(4, 2, 0, false, 0)})
	tracker := NewCommandLifecycleTracker(lifecycleTestConfig(), reader, newLifecycleAuditSink(), slog.Default())
	completion := tracker.runLifecycle(context.Background(), lifecycleRequest(false, 0, statusForLifecycle(3, 2, 0, false, 0)))
	if completion.Result != "protocol_state" {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleTransientStatusReadErrorsRecover(t *testing.T) {
	reader := newLifecycleReader([]GunStatus{statusForLifecycle(6, 1, 0, false, 0)})
	reader.errors = []error{errors.New("temporary read 1"), errors.New("temporary read 2")}
	tracker := NewCommandLifecycleTracker(lifecycleTestConfig(), reader, newLifecycleAuditSink(), slog.Default())
	completion := tracker.runLifecycle(context.Background(), lifecycleRequest(false, 0, statusForLifecycle(6, 2, 0, false, 0)))
	if completion.Result != "completed_no_fire" {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleRepeatedStatusReadFailuresTimeout(t *testing.T) {
	cfg := lifecycleTestConfig()
	cfg.LifecycleMaxConsecutiveErrors = 2
	reader := newLifecycleReader(nil)
	reader.errors = []error{errors.New("read 1"), errors.New("read 2")}
	tracker := NewCommandLifecycleTracker(cfg, reader, newLifecycleAuditSink(), slog.Default())
	completion := tracker.runLifecycle(context.Background(), lifecycleRequest(false, 0, statusForLifecycle(7, 2, 0, false, 0)))
	if completion.Result != "completion_timeout" || completion.ShotSequence == nil || *completion.ShotSequence != 0 {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleTimeoutBeforeAnyValidStatusLeavesShotSequenceUnavailable(t *testing.T) {
	cfg := lifecycleTestConfig()
	cfg.LifecycleMaxConsecutiveErrors = 1
	reader := newLifecycleReader(nil)
	reader.errors = []error{errors.New("read failed")}
	tracker := NewCommandLifecycleTracker(cfg, reader, newLifecycleAuditSink(), slog.Default())
	req := lifecycleRequest(false, 0, statusForLifecycle(0, 0, 0, false, 0))
	req.SubmittedSequence = 14
	req.AckStatus = GunStatus{}
	completion := tracker.runLifecycle(context.Background(), req)
	if completion.Result != "completion_timeout" || completion.ShotSequence != nil {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleMovementDoesNotCompleteImmediatelyFromReadyBaseline(t *testing.T) {
	cfg := lifecycleTestConfig()
	cfg.LifecycleCompletionTimeout = 15 * time.Millisecond
	reader := newLifecycleReader([]GunStatus{statusForLifecycle(8, 1, 0, false, 0)})
	tracker := NewCommandLifecycleTracker(cfg, reader, newLifecycleAuditSink(), slog.Default())
	req := lifecycleRequest(false, 0, statusForLifecycle(8, 1, 0, false, 0))
	req.BaselineStatus = statusForLifecycle(7, 1, 0, false, 0)
	completion := tracker.runLifecycle(context.Background(), req)
	if completion.Result != "completion_timeout" {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleShotSequenceRollover(t *testing.T) {
	reader := newLifecycleReader([]GunStatus{statusForLifecycle(9, 1, 0, false, 0)})
	tracker := NewCommandLifecycleTracker(lifecycleTestConfig(), reader, newLifecycleAuditSink(), slog.Default())
	req := lifecycleRequest(true, ^uint32(0), statusForLifecycle(9, 2, ^uint32(0), false, 0))
	completion := tracker.runLifecycle(context.Background(), req)
	if completion.Result != "completed_fired" || completion.ShotSequence == nil || *completion.ShotSequence != 0 {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleMaintenanceCompletionStoresShotSequence(t *testing.T) {
	reader := newLifecycleReader([]GunStatus{statusForLifecycle(15, 6, 8, false, 0)})
	tracker := NewCommandLifecycleTracker(lifecycleTestConfig(), reader, newLifecycleAuditSink(), slog.Default())
	req := lifecycleRequest(false, 8, statusForLifecycle(15, 2, 8, false, 0))
	req.Command.Maintenance = true
	completion := tracker.runLifecycle(context.Background(), req)
	if completion.Result != "completed_no_fire" || completion.ShotSequence == nil || *completion.ShotSequence != 8 {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleFaultResetCompletionStoresShotSequence(t *testing.T) {
	reader := newLifecycleReader([]GunStatus{statusForLifecycle(16, 1, 9, false, 0)})
	tracker := NewCommandLifecycleTracker(lifecycleTestConfig(), reader, newLifecycleAuditSink(), slog.Default())
	req := lifecycleRequest(false, 9, statusForLifecycle(16, 7, 9, true, 2))
	req.Command.FaultReset = true
	completion := tracker.runLifecycle(context.Background(), req)
	if completion.Result != "completed_no_fire" || completion.ShotSequence == nil || *completion.ShotSequence != 9 {
		t.Fatalf("completion = %+v", completion)
	}
}

func TestLifecycleStartUpdatesCorrectAuditRow(t *testing.T) {
	reader := newLifecycleReader([]GunStatus{statusForLifecycle(10, 1, 0, false, 0)})
	auditor := newLifecycleAuditSink()
	tracker := NewCommandLifecycleTracker(lifecycleTestConfig(), reader, auditor, slog.Default())
	if err := tracker.Start(lifecycleRequest(false, 0, statusForLifecycle(10, 2, 0, false, 0))); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	update := auditor.wait(t)
	if update.id != 42 || update.completion.Result != "completed_no_fire" {
		t.Fatalf("update = %+v", update)
	}
	tracker.Close()
}

func TestLifecycleStartPublishesCompletionEvent(t *testing.T) {
	reader := newLifecycleReader([]GunStatus{statusForLifecycle(17, 1, 1, false, 0)})
	auditor := newLifecycleAuditSink()
	tracker := NewCommandLifecycleTracker(lifecycleTestConfig(), reader, auditor, slog.Default())
	completions := make(chan CommandLifecycleEvent, 1)
	req := lifecycleRequest(true, 0, statusForLifecycle(17, 2, 0, false, 0))
	req.AuditID = 77
	req.Completion = completions
	if err := tracker.Start(req); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	select {
	case event := <-completions:
		if event.AuditID != 77 || event.SubmittedSequence != 17 || event.Completion.Result != "completed_fired" {
			t.Fatalf("event = %+v", event)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for lifecycle completion event")
	}
	tracker.Close()
}

func TestLifecycleCloseCancelsTracker(t *testing.T) {
	reader := &blockingStatusReader{}
	auditor := newLifecycleAuditSink()
	tracker := NewCommandLifecycleTracker(lifecycleTestConfig(), reader, auditor, slog.Default())
	if err := tracker.Start(lifecycleRequest(false, 0, statusForLifecycle(11, 2, 0, false, 0))); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	done := make(chan struct{})
	go func() {
		tracker.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("tracker Close did not return")
	}
}

func validCommand() GunCommand {
	return GunCommand{
		Mode:                 ModeManual,
		TargetAzimuth:        120,
		TargetElevation:      30,
		SafetyFirePermissive: true,
	}
}

func testPLCConfig() config.PLCConfig {
	return config.PLCConfig{
		Host:                          "openplc-control-gun1",
		Port:                          502,
		UnitID:                        1,
		CommandAckTimeout:             100 * time.Millisecond,
		PollInterval:                  time.Millisecond,
		LifecyclePollInterval:         time.Millisecond,
		LifecycleCompletionTimeout:    100 * time.Millisecond,
		LifecycleMaxConsecutiveErrors: 5,
	}
}

func lifecycleTestConfig() config.PLCConfig {
	cfg := testPLCConfig()
	cfg.LifecyclePollInterval = time.Millisecond
	cfg.LifecycleCompletionTimeout = 100 * time.Millisecond
	cfg.LifecycleMaxConsecutiveErrors = 5
	return cfg
}

func lifecycleRequest(fire bool, baselineShot uint32, ack GunStatus) CommandLifecycleRequest {
	return CommandLifecycleRequest{
		AuditID:           42,
		GunID:             Gun1ID,
		Command:           GunCommand{Mode: ModeManual, TargetAzimuth: 120, TargetElevation: 30, FireRequest: fire},
		SubmittedSequence: ack.AcceptedSequence,
		BaselineStatus:    statusForLifecycle(ack.AcceptedSequence-1, 1, baselineShot, false, 0),
		AckStatus:         ack,
		Source:            "test",
	}
}

func statusForLifecycle(accepted uint32, state uint16, shot uint32, fault bool, faultCode uint16) GunStatus {
	low, high := SplitUint32(accepted)
	return GunStatus{
		AcceptedSequence:     accepted,
		AcceptedSequenceLow:  low,
		AcceptedSequenceHigh: high,
		State:                state,
		StateName:            StateName(state),
		FaultActive:          fault,
		FaultCode:            faultCode,
		FaultName:            FaultName(faultCode),
		ShotSequence:         shot,
		Lockout:              state == 5,
		Settling:             state == 3,
		Firing:               state == 4,
		Ready:                state == 1,
	}
}

type mockWrite struct {
	address uint16
	values  []uint16
}

type mockRead struct {
	address  uint16
	quantity uint16
}

type mockTransport struct {
	mu          sync.Mutex
	status      GunStatus
	statusQueue []GunStatus
	reads       []mockRead
	writes      []mockWrite
	readErr     error
	writeErr    error
	ackOnCommit bool
}

type lifecycleReader struct {
	mu       sync.Mutex
	statuses []GunStatus
	errors   []error
	last     GunStatus
}

func newLifecycleReader(statuses []GunStatus) *lifecycleReader {
	reader := &lifecycleReader{statuses: append([]GunStatus(nil), statuses...)}
	if len(statuses) > 0 {
		reader.last = statuses[len(statuses)-1]
	}
	return reader
}

func (r *lifecycleReader) ReadStatus(ctx context.Context) (GunStatus, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.errors) > 0 {
		err := r.errors[0]
		r.errors = r.errors[1:]
		return GunStatus{}, err
	}
	if len(r.statuses) > 0 {
		status := r.statuses[0]
		r.statuses = r.statuses[1:]
		r.last = status
		return status, nil
	}
	return r.last, nil
}

type blockingStatusReader struct{}

func (r *blockingStatusReader) ReadStatus(ctx context.Context) (GunStatus, error) {
	<-ctx.Done()
	return GunStatus{}, ctx.Err()
}

type lifecycleUpdate struct {
	id         int64
	completion CommandAuditCompletion
}

type lifecycleAuditSink struct {
	updates chan lifecycleUpdate
}

func newLifecycleAuditSink() *lifecycleAuditSink {
	return &lifecycleAuditSink{updates: make(chan lifecycleUpdate, 4)}
}

func (s *lifecycleAuditSink) UpdateGunCommandCompletion(ctx context.Context, id int64, completion CommandAuditCompletion) error {
	select {
	case s.updates <- lifecycleUpdate{id: id, completion: completion}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *lifecycleAuditSink) wait(t *testing.T) lifecycleUpdate {
	t.Helper()
	select {
	case update := <-s.updates:
		return update
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for lifecycle audit update")
		return lifecycleUpdate{}
	}
}

func newMockTransport() *mockTransport {
	return &mockTransport{ackOnCommit: true}
}

func (m *mockTransport) ReadHoldingRegisters(ctx context.Context, address, quantity uint16) ([]uint16, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readErr != nil {
		return nil, m.readErr
	}
	m.reads = append(m.reads, mockRead{address: address, quantity: quantity})
	if address != statusStartRegister || quantity != statusRegisterCount {
		return nil, errors.New("unexpected status read")
	}
	status := m.status
	if len(m.statusQueue) > 0 {
		status = m.statusQueue[0]
		m.statusQueue = m.statusQueue[1:]
	}
	words := make([]uint16, statusRegisterCount)
	low, high := SplitUint32(status.AcceptedSequence)
	words[RegisterAcceptedSequenceLow-statusStartRegister] = low
	words[RegisterAcceptedSequenceHigh-statusStartRegister] = high
	words[RegisterState-statusStartRegister] = status.State
	words[RegisterFaultActive-statusStartRegister] = boolWord(status.FaultActive)
	words[RegisterFaultCode-statusStartRegister] = status.FaultCode
	shotLow, shotHigh := SplitUint32(status.ShotSequence)
	words[RegisterShotSequenceLow-statusStartRegister] = shotLow
	words[RegisterShotSequenceHigh-statusStartRegister] = shotHigh
	return words, nil
}

func (m *mockTransport) WriteMultipleRegisters(ctx context.Context, address uint16, values []uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	m.writes = append(m.writes, mockWrite{address: address, values: append([]uint16(nil), values...)})
	return nil
}

func (m *mockTransport) WriteSingleRegister(ctx context.Context, address, value uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	m.writes = append(m.writes, mockWrite{address: address, values: []uint16{value}})
	if m.ackOnCommit && address == commandAddress(RegisterCommandSequenceLow) {
		high := uint16(0)
		if len(m.writes) >= 2 && len(m.writes[len(m.writes)-2].values) > 0 {
			high = m.writes[len(m.writes)-2].values[0]
		}
		m.status.AcceptedSequence = CombineUint32(value, high)
	}
	return nil
}

func (m *mockTransport) committedSequences() []uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	var sequences []uint32
	for _, write := range m.writes {
		if write.address == commandAddress(RegisterCommandSequenceLow) && len(write.values) == 1 {
			sequences = append(sequences, uint32(write.values[0]))
		}
	}
	return sequences
}
