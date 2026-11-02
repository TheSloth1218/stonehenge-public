package arbitration

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"stonehenge-middleware/internal/plc"
)

func TestTypedMappings(t *testing.T) {
	if ModeManual.Code() != 0 || ModeAuto.Code() != 1 || ModeMaintenance.Code() != 2 || ModeAutoSuspended.Code() != 3 || Mode("bad").Code() != 255 {
		t.Fatalf("unexpected mode codes")
	}
	if OperationNone.Code() != 0 || OperationManualCommand.Code() != 1 || OperationAutoEngagement.Code() != 2 || OperationFaultReset.Code() != 3 || Operation("bad").Code() != 255 {
		t.Fatalf("unexpected operation codes")
	}
	if ResetResultSucceeded.Code() != 1 || ResetResultRejected.Code() != 2 || ResetResultAcknowledgmentFailed.Code() != 3 || ResetResultFaultPersisted.Code() != 4 || ResetResultLifecycleTimeout.Code() != 5 || ResetResultProtocolState.Code() != 6 || ResetResultInternalError.Code() != 7 || FaultResetResult("bad").Code() != 255 {
		t.Fatalf("unexpected reset result codes")
	}
}

func TestManualAllowedOnlyInManualMode(t *testing.T) {
	service := NewService(&fakeReader{status: readyStatus()}, slog.Default())
	lease, status, err := service.AcquireManual(context.Background())
	if err != nil {
		t.Fatalf("AcquireManual in manual mode: %v status=%+v", err, status)
	}
	if !service.Status().ManualActive {
		t.Fatalf("manual operation not active")
	}
	lease.Release()

	if _, err := service.RequestMode(context.Background(), ModeAuto); err != nil {
		t.Fatalf("RequestMode(auto): %v", err)
	}
	_, status, err = service.AcquireManual(context.Background())
	if !errors.Is(err, ErrConflict) || status.LastArbitrationErrorCode != ErrorWrongMode {
		t.Fatalf("AcquireManual in auto err=%v status=%+v", err, status)
	}
}

func TestModeTransitionRejectedDuringActiveOperation(t *testing.T) {
	service := NewService(&fakeReader{status: readyStatus()}, slog.Default())
	lease, _, err := service.AcquireManual(context.Background())
	if err != nil {
		t.Fatalf("AcquireManual: %v", err)
	}
	status, err := service.RequestMode(context.Background(), ModeAuto)
	if !errors.Is(err, ErrConflict) || status.LastArbitrationErrorCode != ErrorManualActive {
		t.Fatalf("RequestMode during manual err=%v status=%+v", err, status)
	}
	lease.Release()
	if _, err := service.RequestMode(context.Background(), ModeAuto); err != nil {
		t.Fatalf("RequestMode after release: %v", err)
	}
}

func TestAutoReadinessValidation(t *testing.T) {
	service := NewService(&fakeReader{status: plc.GunStatus{State: 2, StateName: "Slewing"}}, slog.Default())
	_, err := service.RequestMode(context.Background(), ModeAuto)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("RequestMode(auto) with not-ready PLC err=%v", err)
	}

	service = NewService(&fakeReader{status: faultStatus(2)}, slog.Default())
	status, err := service.RequestMode(context.Background(), ModeAuto)
	if !errors.Is(err, ErrConflict) || status.LastArbitrationErrorCode != ErrorPLCFaulted {
		t.Fatalf("RequestMode(auto) fault err=%v status=%+v", err, status)
	}
}

func TestFaultResetEligibilityAndCompletion(t *testing.T) {
	service := NewService(&fakeReader{status: readyStatus()}, slog.Default())
	_, status, err := service.AcquireFaultReset(context.Background(), readyStatus(), false)
	if !errors.Is(err, ErrConflict) || status.LastArbitrationErrorCode != ErrorResetNotAllowed {
		t.Fatalf("reset without fault err=%v status=%+v", err, status)
	}
	_, status, err = service.AcquireFaultReset(context.Background(), faultStatus(5), false)
	if !errors.Is(err, ErrConflict) || status.LastArbitrationErrorCode != ErrorResetNotAllowed {
		t.Fatalf("reset non-resettable err=%v status=%+v", err, status)
	}

	lease, status, err := service.AcquireFaultReset(context.Background(), faultStatus(2), false)
	if err != nil {
		t.Fatalf("AcquireFaultReset resettable: %v status=%+v", err, status)
	}
	if !status.FaultResetActive || status.ActiveOperation != OperationFaultReset {
		t.Fatalf("reset not active: %+v", status)
	}
	lease.Release()
}

func TestSuccessfulResetResumesAutoSuspended(t *testing.T) {
	service := NewService(&fakeReader{status: readyStatus()}, slog.Default())
	service.SuspendAutoFault("faulted")
	lease, _, err := service.AcquireFaultReset(context.Background(), faultStatus(2), true)
	if err != nil {
		t.Fatalf("AcquireFaultReset: %v", err)
	}
	_ = lease
	status := service.CompleteFaultReset(ResetResultSucceeded, readyStatus(), "")
	if status.Mode != ModeAuto || status.FaultResetActive || status.LastFaultResetResult != string(ResetResultSucceeded) {
		t.Fatalf("reset completion status=%+v", status)
	}
}

func TestFailedResetLeavesAutoSuspended(t *testing.T) {
	service := NewService(&fakeReader{status: readyStatus()}, slog.Default())
	service.SuspendAutoFault("faulted")
	if _, _, err := service.AcquireFaultReset(context.Background(), faultStatus(2), true); err != nil {
		t.Fatalf("AcquireFaultReset: %v", err)
	}
	status := service.CompleteFaultReset(ResetResultFaultPersisted, faultStatus(2), "fault persisted")
	if status.Mode != ModeAutoSuspended || status.FaultResetActive || status.LastArbitrationErrorCode != ErrorPLCFaulted {
		t.Fatalf("failed reset status=%+v", status)
	}
}

func TestConcurrentAcquisitionAllowsOneOwner(t *testing.T) {
	service := NewService(&fakeReader{status: readyStatus()}, slog.Default())
	var wg sync.WaitGroup
	successes := 0
	var mu sync.Mutex
	leases := make(chan *Lease, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lease, _, err := service.AcquireManual(context.Background())
			if err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
				leases <- lease
			}
		}()
	}
	wg.Wait()
	close(leases)
	for lease := range leases {
		lease.Release()
	}
	if successes != 1 {
		t.Fatalf("successes=%d, want 1", successes)
	}
}

type fakeReader struct {
	status plc.GunStatus
	err    error
}

func (r *fakeReader) ReadStatus(ctx context.Context) (plc.GunStatus, error) {
	if r.err != nil {
		return plc.GunStatus{}, r.err
	}
	return r.status, nil
}

func readyStatus() plc.GunStatus {
	return plc.GunStatus{State: 1, StateName: "Ready", Ready: true}
}

func faultStatus(code uint16) plc.GunStatus {
	return plc.GunStatus{State: 7, StateName: "Fault", FaultActive: true, FaultCode: code, FaultName: "Fault"}
}
